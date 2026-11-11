package main

// ShieldService est le pont entre l'interface et les moteurs d'Aria.
//
// Chaque méthode exportée est appelable depuis le JavaScript : Wails génère
// pour elle une fonction du même nom (frontend/bindings, à la compilation).
// Les méthodes renvoient toujours un résultat affichable (journal d'étapes et
// message d'erreur en français) plutôt qu'une erreur Go brute.
//
// Organisation du fichier :
//
//	démarrage        NewShieldService, ServiceStartup, ServiceShutdown, Status
//	Résolveur        StartDNS, StopDNS, TestDNS, Enable/DisableSystemDNS
//	Profils VPN      VPNProfile, SaveVPNProfile, ImportVPNConf, DeleteVPNProfile,
//	                 UseVPNProfile (bascule à chaud), TestVPN
//	Protection       Protect, Unprotect, RestoreAll
//	Tests            TestSystem
//	Mode Fantôme     TorState, TorPrepare, TorCheckUpdate, TorUpdate, TorLaunch
//
// Verrous : s.mu sérialise tout ce qui ouvre, bascule ou ferme le tunnel, pour
// qu'un double clic ne lance jamais deux séquences en même temps.

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"aria-shield/internal/blocklist"
	"aria-shield/internal/browsers"
	"aria-shield/internal/dnsengine"
	"aria-shield/internal/journal"
	"aria-shield/internal/leak"
	"aria-shield/internal/paths"
	"aria-shield/internal/prefs"
	"aria-shield/internal/sysdns"
	"aria-shield/internal/sysx"
	"aria-shield/internal/torbrowser"
	"aria-shield/internal/vpn"
)

// Step est une ligne du journal affiché dans l'interface.
type Step = sysdns.Step

// ShieldService expose au frontend les actions d'Aria Shield.
type ShieldService struct {
	dns   *dnsengine.Engine   // résolveur Unbound
	sys   *sysdns.Manager     // DNS des cartes réseau de Windows
	store *vpn.Store          // carnet de profils VPN chiffré
	tun   *vpn.Tunnel         // tunnel WireGuard et kill switch
	tor   *torbrowser.Manager // Tor Browser portable

	userDir  string // %USERPROFILE%\.Aria-Shield
	sysDir   string // %PROGRAMDATA%\AriaShield (réservé aux administrateurs)
	noticeMu sync.Mutex
	notice   string // message affiché une fois (migration, erreur de dossier)

	torMu     sync.Mutex
	torUpdate torbrowser.UpdateInfo // dernière vérification des mises à jour

	mu       sync.Mutex // sérialise Protect / Unprotect
	boxIPs   leak.IPs   // IP publique mesurée juste avant la protection
	lastLeak *leak.Verdict
	ownsDNS  bool // la bascule du DNS de Windows a été faite par Protect

	prefs      *prefs.Store  // réglages et dernière protection (startup.go)
	resumeMu   sync.Mutex    // protège resumeStop et resuming
	resumeStop chan struct{} // fermé quand l'utilisateur reprend la main
	resuming   string        // réactivation en cours, affichée dans l'interface
	restarts   []time.Time   // relances automatiques d'Unbound (sous resumeMu)
	notify     *notifier     // notifications Windows (notify.go), posé par main
	quitting   atomic.Bool   // Aria est en train de quitter

	log  *journal.Journal // aria.log (journal.go)
	// silent : lancée sans fenêtre par la tâche planifiée (posé par main).
	silent bool

	updMu  sync.Mutex  // protège update
	update UpdateState // dernière vérification des versions (update.go)

	ads  *blocklist.Store // liste de blocage des pubs (adblock.go)
	adMu sync.Mutex       // une seule opération sur la liste à la fois

	pauseMu    sync.Mutex  // protège adPauseEnd et adPauseTimer
	adPauseEnd time.Time   // fin de la suspension du blocage, zéro si aucune
	adPauseTimer *time.Timer // reprise automatique
}

// NewShieldService prépare le service et ses moteurs (rien n'est lancé).
func NewShieldService() *ShieldService {
	s := &ShieldService{tun: vpn.NewTunnel()}

	var errs []string
	userDir, movedHome, err := paths.UserDir()
	if err != nil {
		errs = append(errs, "déplacement de tes données vers .Aria-Shield, réessayé au prochain lancement"+
			" (ferme Tor Browser avant) : "+err.Error())
	}
	if movedHome {
		s.notice = "Tes données (profils VPN, Tor Browser) sont maintenant dans " + userDir +
			", comme pour les autres applis Aria. L'ancien cache de l'interface, %APPDATA%\\Aria-Shield.exe," +
			" peut être supprimé."
	}
	sysDir, aside, err := paths.SystemDir()
	sysLocked := err == nil
	if err != nil {
		errs = append(errs, "dossier système non verrouillé, Unbound ne sera pas lancé : "+err.Error())
	}
	if aside != "" {
		errs = append(errs, "un dossier AriaShield qu'Aria n'avait pas créé (piège possible) a été écarté dans "+
			aside+" : examine-le avec un antivirus avant de le supprimer")
	}
	s.userDir, s.sysDir = userDir, sysDir
	var logErr error
	if s.log, logErr = journal.Open(userDir); logErr != nil {
		errs = append(errs, "journal aria.log non ouvert : "+logErr.Error())
	}

	// Les versions jusqu'à la v0.6.1 rangeaient tout à côté de l'exe.
	if legacy := paths.LegacyDir(); legacy != "" {
		moved, err := paths.Migrate(legacy, userDir, sysDir)
		switch {
		case err != nil:
			errs = append(errs, "déplacement de l'ancien dossier AriaShield-data : "+err.Error())
		case len(moved) > 0:
			s.notice = "Tes données ont été déplacées : profil VPN et Tor Browser dans " + userDir +
				", fichiers système dans " + sysDir + ". L'ancien dossier AriaShield-data a été supprimé."
		}
	}
	paths.CleanOldDLL()
	if len(errs) > 0 {
		s.notice = "Attention : " + strings.Join(errs, " ; ")
	}

	// Unbound tourne en administrateur : jamais depuis un dossier non verrouillé.
	unboundDir := ""
	if sysLocked {
		unboundDir = sysDir
	}
	s.dns = dnsengine.New(unboundDir)
	s.sys = sysdns.New(sysDir)
	s.sys.Exclude(vpn.AdapterName) // la carte VPN est configurée par le tunnel lui-même
	s.store = vpn.NewStore(userDir)
	s.tor = torbrowser.New(userDir)
	s.prefs = prefs.New(userDir)
	// La liste de blocage vit avec Unbound, dans le dossier verrouillé.
	s.ads = blocklist.NewStore(unboundDir)
	if s.prefs.Load().BlockAds {
		s.dns.SetInclude(s.ads.ConfPath())
	}

	// Filets de sécurité si Unbound s'arrête tout seul.
	s.dns.OnUnexpectedExit(func() {
		// Windows ou le tunnel comptent sur Unbound : on le relance d'abord (3 fois
		// en 10 min au plus, pour ne pas boucler sur un Unbound qui plante en série).
		s.logf("Unbound s'est arrêté tout seul")
		if (s.tun.Stats().Full || s.sys.Enabled()) && s.allowRestart() {
			if _, err := s.dns.Start(); err == nil {
				s.logf("Unbound relancé automatiquement")
				return
			}
		}
		if s.tun.Stats().Full {
			// Relance impossible : on coupe tout proprement.
			s.tun.Down()
			s.sys.Disable("Unbound s'est arrêté : protection coupée et connexion d'origine remise.")
			s.alert("Protection coupée", "Le résolveur Unbound s'est arrêté et n'a pas pu être relancé : tunnel fermé, connexion d'origine remise.")
			return
		}
		if s.sys.Enabled() {
			s.sys.Disable("Unbound s'est arrêté : ton DNS d'origine a été remis automatiquement.")
			s.alert("DNS d'origine remis", "Le résolveur Unbound s'est arrêté et n'a pas pu être relancé : Windows utilise de nouveau le DNS de ta box.")
		}
	})
	return s
}

// Component décrit un moteur affiché dans la carte « Moteurs ».
type Component struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	State  string `json:"state"` // "active", "ready" ou "planned"
}

// Status décrit l'état général de l'appli.
type Status struct {
	Admin      bool          `json:"admin"`
	DNSRunning bool          `json:"dnsRunning"`
	SystemDNS  bool          `json:"systemDns"`
	Protected  bool          `json:"protected"`
	Event      string        `json:"event"`
	DataDir    string        `json:"dataDir"`
	UserDir    string        `json:"userDir"`
	VPN        vpn.View      `json:"vpn"`      // profil actif
	Profiles   []vpn.View    `json:"profiles"` // tous les profils, sans clé
	Tunnel     vpn.Stats     `json:"tunnel"`
	Leak       *leak.Verdict `json:"leak"`
	Components []Component   `json:"components"`
	Resuming   string        `json:"resuming"` // réactivation automatique en cours
	LogPath    string        `json:"logPath"`  // journal aria.log
	NewVersion string        `json:"newVersion"` // version plus récente publiée, "" sinon
	AdBlock    bool          `json:"adBlock"`  // pubs bloquées par le résolveur en cours
	AdPaused   bool          `json:"adPaused"` // blocage suspendu pour quelques minutes
	Version    string        `json:"version"`
}

// ActionResult regroupe le journal d'une action et le test qui la suit.
type ActionResult struct {
	Steps []dnsengine.Step     `json:"steps"`
	Test  dnsengine.TestResult `json:"test"`
	Error string               `json:"error"`
}

// SystemResult est le journal d'une action système.
type SystemResult struct {
	Steps []Step `json:"steps"`
	Error string `json:"error"`
}

// SaveResult renvoie le profil enregistré, ou les erreurs par champ.
type SaveResult struct {
	Profile vpn.View          `json:"profile"`
	Fields  map[string]string `json:"fields"`
	Error   string            `json:"error"`
}

// ImportResult résume l'import de fichiers .conf.
type ImportResult struct {
	Imported []vpn.View `json:"imported"`
	Steps    []Step     `json:"steps"`
	Error    string     `json:"error"`
	Canceled bool       `json:"canceled"`
}

// SystemTests regroupe les vérifications côté Windows.
type SystemTests struct {
	MarkerOK       bool            `json:"markerOk"`
	MarkerDetail   string          `json:"markerDetail"`
	AdaptersOK     bool            `json:"adaptersOk"`
	AdaptersDetail string          `json:"adaptersDetail"`
	Leak           *leak.Verdict   `json:"leak"`
	KillSwitch     bool            `json:"killSwitch"`
	Browsers       []browsers.Info `json:"browsers"`
	AdBlockOn      bool            `json:"adBlockOn"` // blocage des pubs demandé
	AdBlockOK      bool            `json:"adBlockOk"`
	AdBlockDetail  string          `json:"adBlockDetail"`
}

// ServiceStartup : si Aria avait planté en laissant Windows sur le résolveur, on
// répare ; puis, en arrière-plan, la dernière protection est réactivée et la
// copie lancée au démarrage de Windows remise à jour (startup.go).
func (s *ShieldService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// Noté ici et non dans main : un exe relancé alors qu'Aria tourne déjà
	// s'efface avant d'arriver jusqu'ici, et n'écrit donc pas « démarre ».
	s.logStart(s.silent)
	if recovered, steps := s.sys.RecoverLeftover(); recovered {
		s.logResult("DNS laissé sur Aria par une session interrompue : restauration", steps, "")
	}
	go s.refreshAutostart()
	go s.resume()
	go s.refreshBlocklist()
	go s.checkUpdateAtStart()
	return nil
}

// ServiceShutdown : quand Aria quitte (menu « Quitter », fin de session
// Windows), tout revient comme avant. Fermer la fenêtre ne quitte pas : Aria
// reste dans la zone de notification (main.go).
func (s *ShieldService) ServiceShutdown() error {
	s.quitting.Store(true)
	s.logf("arrêt d'Aria : tunnel fermé, DNS d'origine remis, résolveur arrêté")
	defer s.log.Close()
	s.cancelResume()
	s.tun.Down()
	if s.sys.Enabled() {
		s.sys.Disable("")
	}
	s.dns.Stop()
	return nil
}

// Status renvoie l'état des droits, des moteurs et de la protection.
// L'interface l'appelle toutes les 5 s : il doit rester rapide (aucun accès réseau).
func (s *ShieldService) Status() Status {
	running := s.dns.Running()
	tun := s.tun.Stats()
	book, _ := s.store.Load()
	profile := book.Current()

	unbound := Component{ID: "unbound", Name: "Unbound", State: "ready",
		Detail: "Intégré · v" + dnsengine.Version + " · arrêté"}
	if running {
		unbound.State = "active"
		unbound.Detail = "Intégré · v" + dnsengine.Version + " · actif"
	}
	wg := Component{ID: "wireguard", Name: "WireGuard", State: "planned", Detail: "Aucun profil VPN enregistré"}
	if profile.PrivateKey != "" {
		wg.State, wg.Detail = "ready", "Intégré · profil « "+profile.Name+" » prêt"
	}
	if tun.Up {
		wg.State, wg.Detail = "active", "Intégré · tunnel ouvert vers "+profile.Name
	}

	s.mu.Lock()
	lastLeak := s.lastLeak
	s.mu.Unlock()

	torState := s.tor.State()
	tor := Component{ID: "tor", Name: "Tor Browser", State: "ready",
		Detail: "Intégré · v" + torbrowser.EmbeddedVersion + " · extrait au premier lancement"}
	if torState.Installed {
		tor.Detail = "Intégré · v" + torState.Version
	}
	if torState.Running {
		tor.State, tor.Detail = "active", "Ouvert · v"+torState.Version
	}

	return Status{
		Admin:      sysx.IsAdmin(),
		DNSRunning: running,
		SystemDNS:  s.sys.Enabled(),
		Protected:  tun.Full,
		Event:      s.event(),
		DataDir:    s.sysDir,
		UserDir:    s.userDir,
		VPN:        profile.View(),
		Profiles:   book.Views(),
		Tunnel:     tun,
		Leak:       lastLeak,
		Components: []Component{
			unbound,
			wg,
			tor,
		},
		Resuming: s.resumingText(),
		LogPath:  s.log.Path(),
		NewVersion: s.updateLatest(),
		AdBlock:  s.adBlockActive(),
		AdPaused: s.adPaused(),
		Version:  Version,
	}
}

// Adapters liste les cartes réseau actives et leurs serveurs DNS.
func (s *ShieldService) Adapters() []sysdns.AdapterView {
	return s.sys.Adapters()
}

// ---------- Résolveur ----------

// StartDNS démarre le résolveur embarqué puis le teste.
func (s *ShieldService) doStartDNS() ActionResult {
	s.cancelResume()
	steps, err := s.dns.Start()
	res := ActionResult{Steps: steps}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Test = s.dns.Test()
	return res
}

// StopDNS rend son DNS à Windows si besoin, puis arrête le résolveur.
func (s *ShieldService) doStopDNS() SystemResult {
	s.cancelResume()
	if s.tun.Stats().Full {
		return SystemResult{Error: "désactive d'abord la protection : le résolveur en fait partie"}
	}
	var res SystemResult
	if s.sys.Enabled() {
		res.Steps = s.sys.Disable("")
	}
	s.dns.Stop()
	s.remember(prefs.None)
	return res
}

// TestDNS relance les tests de résolution et de DNSSEC.
func (s *ShieldService) TestDNS() dnsengine.TestResult {
	return s.dns.Test()
}

// EnableSystemDNS fait utiliser le résolveur par tout Windows, après vérification.
func (s *ShieldService) EnableSystemDNS() SystemResult {
	s.cancelResume()
	var res SystemResult
	res.Steps, res.Error = s.enableSystemDNS()
	if res.Error == "" {
		s.remember(prefs.DNS)
	}
	s.logResult("Bascule de Windows sur le résolveur", res.Steps, res.Error)
	return res
}

// enableSystemDNS bascule Windows sur le résolveur, en trois temps : le
// résolveur doit d'abord passer ses tests, puis les cartes sont basculées, et
// enfin Windows doit résoudre le marqueur qu'Unbound est seul à connaître.
// Si cette dernière preuve manque, tout est remis comme avant.
func (s *ShieldService) enableSystemDNS() ([]Step, string) {
	if !s.dns.Running() {
		return nil, "démarre d'abord le résolveur"
	}
	if t := s.dns.Test(); !t.ResolveOK || !t.DNSSECOK {
		return nil, "le résolveur ne passe pas ses tests : " + t.Detail
	}
	steps, err := s.sys.Enable(s.dns.IPv6())
	if err != nil {
		return steps, err.Error()
	}
	ok, detail := sysdns.CheckMarker(dnsengine.MarkerName, dnsengine.MarkerIP)
	steps = append(steps, Step{Label: "Vérification par Windows", OK: ok, Note: detail})
	if !ok {
		steps = append(steps, s.sys.Disable("")...)
		return steps, "Windows n'utilise pas le résolveur malgré la bascule : config d'origine remise"
	}
	return steps, ""
}

// DisableSystemDNS rend son DNS d'origine à Windows.
func (s *ShieldService) doDisableSystemDNS() SystemResult {
	s.cancelResume()
	if s.tun.Stats().Full {
		return SystemResult{Error: "désactive d'abord la protection : le DNS de Windows en fait partie"}
	}
	s.remember(prefs.None)
	return SystemResult{Steps: s.sys.Disable("")}
}

// ---------- Profils VPN ----------

// VPNProfile renvoie le profil actif, sans jamais la clé privée.
func (s *ShieldService) VPNProfile() vpn.View {
	p, _ := s.store.Active()
	return p.View()
}

// SaveVPNProfile valide puis enregistre un profil : id vide pour en créer un.
// Clé privée ou clé partagée laissées vides = garder celles déjà enregistrées.
func (s *ShieldService) SaveVPNProfile(id string, in vpn.Input) SaveResult {
	book, err := s.store.Load()
	if err != nil {
		return SaveResult{Error: err.Error()}
	}
	var current vpn.Profile
	if id != "" {
		var ok bool
		if current, ok = book.Get(id); !ok {
			return SaveResult{Error: vpn.ErrNotFound.Error()}
		}
		if id == book.Active && s.tun.Stats().Up {
			return SaveResult{Error: "désactive la protection avant de modifier le profil en cours d'utilisation"}
		}
	}
	p, err := in.Validate(current)
	if err != nil {
		if fe, ok := err.(vpn.FieldErrors); ok {
			return SaveResult{Fields: fe, Error: "certains champs sont à corriger"}
		}
		return SaveResult{Error: err.Error()}
	}
	p.ID = id
	if p, err = s.store.Put(p); err != nil {
		return SaveResult{Error: "enregistrement impossible : " + err.Error()}
	}
	return SaveResult{Profile: p.View()}
}

// ImportVPNConf ouvre le sélecteur de fichiers de Windows et importe les .conf
// choisis. Les fichiers sont lus ici, côté Go : les clés ne transitent jamais
// par l'interface. (Le glisser-déposer est impossible : Windows bloque les
// dépôts de l'Explorateur vers une appli lancée en administrateur.)
func (s *ShieldService) doImportVPNConf() ImportResult {
	files, err := application.Get().Dialog.OpenFile().
		SetTitle("Importer une configuration WireGuard").
		AddFilter("Configuration WireGuard (*.conf)", "*.conf").
		CanChooseFiles(true).
		PromptForMultipleSelection()
	if err != nil {
		return ImportResult{Error: "sélecteur de fichiers indisponible : " + err.Error()}
	}
	if len(files) == 0 {
		return ImportResult{Canceled: true}
	}
	return s.importFiles(files)
}

// importFiles importe chaque fichier comme un nouveau profil.
func (s *ShieldService) importFiles(files []string) ImportResult {
	var res ImportResult
	failed := 0
	for _, f := range files {
		label := "Import de « " + vpn.NameFromFile(f) + " »"
		p, notes, err := s.importOne(f)
		if err != nil {
			failed++
			res.Steps = append(res.Steps, Step{Label: label, OK: false, Note: err.Error()})
			continue
		}
		res.Imported = append(res.Imported, p.View())
		res.Steps = append(res.Steps, Step{Label: label, OK: true, Note: "profil « " + p.Name + " » ajouté"})
		for _, n := range notes {
			res.Steps = append(res.Steps, Step{Label: "  " + n, OK: true})
		}
	}
	if failed > 0 {
		res.Error = strconv.Itoa(failed) + " fichier(s) non importé(s) : voir le journal"
	}
	return res
}

// importOne lit, valide et enregistre un fichier .conf. La taille est vérifiée
// avant lecture : un fichier énorme ou spécial (périphérique, tube) est refusé.
func (s *ShieldService) importOne(path string) (vpn.Profile, []string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return vpn.Profile{}, nil, errors.New("fichier illisible")
	}
	if !fi.Mode().IsRegular() || fi.Size() > vpn.MaxConfSize {
		return vpn.Profile{}, nil, errors.New("ce n'est pas une configuration WireGuard")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return vpn.Profile{}, nil, errors.New("fichier illisible")
	}
	imp, err := vpn.ParseConf(string(data), vpn.NameFromFile(path))
	if err != nil {
		return vpn.Profile{}, nil, err
	}
	p, err := imp.Input.Validate(vpn.Profile{})
	if err != nil {
		if fe, ok := err.(vpn.FieldErrors); ok {
			return vpn.Profile{}, nil, errors.New("valeurs incorrectes : " + fe.Error())
		}
		return vpn.Profile{}, nil, err
	}
	p, err = s.store.Put(p)
	return p, imp.Notes, err
}

// DeleteVPNProfile efface un profil (pas celui qu'utilise le tunnel ouvert).
func (s *ShieldService) doDeleteVPNProfile(id string) SystemResult {
	book, err := s.store.Load()
	if err != nil {
		return SystemResult{Error: err.Error()}
	}
	p, ok := book.Get(id)
	if !ok {
		return SystemResult{Error: vpn.ErrNotFound.Error()}
	}
	if id == book.Active && s.tun.Stats().Up {
		return SystemResult{Error: "désactive la protection avant de supprimer le profil en cours d'utilisation"}
	}
	if err := s.store.Delete(id); err != nil {
		return SystemResult{Error: err.Error()}
	}
	return SystemResult{Steps: []Step{{Label: "Profil « " + p.Name + " » supprimé", OK: true}}}
}

// UseVPNProfile choisit le profil de la protection. Si la protection est
// active, Aria bascule à chaud vers ce serveur, kill switch maintenu : en cas
// d'échec, elle revient au serveur précédent.
func (s *ShieldService) doUseVPNProfile(id string) SystemResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}
	book, err := s.store.Load()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	next, ok := book.Get(id)
	if !ok {
		res.Error = vpn.ErrNotFound.Error()
		return res
	}
	prev := book.Current()
	tun := s.tun.Stats()
	switch {
	case tun.Up && !tun.Full:
		res.Error = "un test de connexion est en cours, réessaie dans quelques secondes"
		return res
	case !tun.Full || prev.ID == next.ID:
		if err := s.store.SetActive(id); err != nil {
			res.Error = err.Error()
			return res
		}
		add("Profil « "+next.Name+" » choisi", true, "utilisé à la prochaine activation")
		return res
	}

	dns4 := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	var dns6 []netip.Addr
	if s.dns.IPv6() {
		dns6 = []netip.Addr{netip.IPv6Loopback()}
	}
	add("Kill switch maintenu pendant la bascule", true, "rien ne sort en clair")
	if err := s.tun.Switch(next, dns4, dns6); err != nil {
		add("Bascule vers « "+next.Name+" »", false, err.Error())
		res.Error = "bascule impossible : " + err.Error()
		if s.tun.Switch(prev, dns4, dns6) == nil && s.tun.WaitHandshake(15*time.Second) {
			add("Retour sur « "+prev.Name+" »", true, "")
		}
		return res
	}
	add("Bascule vers « "+next.Name+" »", true, next.Endpoint)
	if !s.tun.WaitHandshake(15 * time.Second) {
		add("Poignée de main avec le serveur", false, "aucune réponse en 15 s")
		res.Error = "« " + next.Name + " » ne répond pas"
		if s.tun.Switch(prev, dns4, dns6) == nil && s.tun.WaitHandshake(15*time.Second) {
			add("Retour sur « "+prev.Name+" »", true, "")
		} else {
			add("Retour sur « "+prev.Name+" »", false, "le kill switch bloque tout : désactive puis réactive la protection")
		}
		return res
	}
	add("Poignée de main avec le serveur", true, "")
	if err := s.store.SetActive(id); err != nil {
		add("Profil actif enregistré", false, err.Error())
	}

	v := leak.Compare(s.boxIPs, leak.Public())
	s.lastLeak = masked(v)
	add("Test de fuite", v.V4OK && v.V6OK, v.Detail)
	if !(v.V4OK && v.V6OK) {
		res.Error = "bascule faite mais " + v.Detail
	}
	return res
}

// TestVPN ouvre le tunnel quelques secondes, sans routes ni kill switch,
// pour vérifier que le serveur répond.
func (s *ShieldService) doTestVPN() SystemResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res SystemResult
	if s.tun.Stats().Up {
		res.Error = "le tunnel est déjà ouvert"
		return res
	}
	p, err := s.store.Active()
	if err != nil || p.PrivateKey == "" {
		res.Error = "enregistre d'abord un profil VPN"
		return res
	}
	if err := s.tun.Up(p, false, nil, nil); err != nil {
		res.Steps = append(res.Steps, Step{Label: "Ouverture du tunnel de test", OK: false, Note: err.Error()})
		res.Error = err.Error()
		return res
	}
	res.Steps = append(res.Steps, Step{Label: "Ouverture du tunnel de test", OK: true, Note: p.Endpoint})
	ok := s.tun.WaitHandshake(10 * time.Second)
	s.tun.Down()
	if !ok {
		res.Steps = append(res.Steps, Step{Label: "Poignée de main avec le serveur", OK: false, Note: "aucune réponse en 10 s"})
		res.Error = "le serveur ne répond pas : vérifie les clés et le point d'accès"
		return res
	}
	res.Steps = append(res.Steps,
		Step{Label: "Poignée de main avec le serveur", OK: true, Note: "le serveur a reconnu ta clé"},
		Step{Label: "Tunnel de test refermé", OK: true})
	return res
}

// ---------- Protection complète ----------

// maskIP masque le milieu d'une adresse : utile si tu partages une capture.
func maskIP(ip string) string {
	if ip == "" {
		return "inconnue"
	}
	if a, err := netip.ParseAddr(ip); err == nil && a.Is4() {
		b := a.As4()
		return strconv.Itoa(int(b[0])) + ".•.•." + strconv.Itoa(int(b[3]))
	}
	parts := strings.Split(ip, ":")
	if len(parts) > 2 {
		return parts[0] + ":•:" + parts[len(parts)-1]
	}
	return ip
}

// masked renvoie un verdict affichable : l'IP de la box est masquée, celle du VPN reste lisible.
func masked(v leak.Verdict) *leak.Verdict {
	m := func(ip string) string {
		if ip == "" {
			return ""
		}
		return maskIP(ip)
	}
	v.Before = leak.IPs{V4: m(v.Before.V4), V6: m(v.Before.V6)}
	return &v
}

// Protect active le mode Discret à la demande de l'utilisateur, et le retient
// pour le réactiver au prochain lancement.
func (s *ShieldService) Protect() SystemResult {
	s.cancelResume()
	res := s.protect()
	if s.tun.Stats().Full {
		s.remember(prefs.Discret)
	}
	s.logResult("Activation de la protection", res.Steps, res.Error)
	return res
}

// protect active le mode Discret : résolveur, DNS de Windows, tunnel et kill switch.
// Chaque étape est vérifiée ; au premier échec, fail défait ce qui a été mis en
// place (rollbackLocked) pour ne jamais laisser une protection à moitié posée.
func (s *ShieldService) protect() SystemResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}
	fail := func(msg string) SystemResult {
		res.Steps = append(res.Steps, s.rollbackLocked()...)
		res.Error = msg
		return res
	}

	if s.tun.Stats().Full {
		add("Protection déjà active", true, "")
		return res
	}
	p, err := s.store.Active()
	if err != nil || p.PrivateKey == "" {
		res.Error = "enregistre d'abord un profil VPN (page VPN)"
		return res
	}

	// 1. Photo de l'IP publique, pour le test de fuite.
	s.boxIPs = leak.Public()
	add("IP de ta box mesurée", s.boxIPs.V4 != "", maskIP(s.boxIPs.V4))

	// 2. Résolveur.
	if !s.dns.Running() {
		steps, err := s.dns.Start()
		for _, st := range steps {
			add(st.Label, st.OK, st.Note)
		}
		if err != nil {
			return fail("le résolveur n'a pas démarré : " + err.Error())
		}
	} else {
		add("Résolveur déjà actif", true, "")
	}

	// 3. DNS de Windows.
	if !s.sys.Enabled() {
		steps, msg := s.enableSystemDNS()
		res.Steps = append(res.Steps, steps...)
		if msg != "" {
			return fail(msg)
		}
		s.ownsDNS = true
	} else {
		add("Windows utilise déjà le résolveur", true, "")
	}

	// 4. Tunnel complet + kill switch.
	dns4 := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	var dns6 []netip.Addr
	if s.dns.IPv6() {
		dns6 = []netip.Addr{netip.IPv6Loopback()}
	}
	if err := s.tun.Up(p, true, dns4, dns6); err != nil {
		add("Ouverture du tunnel", false, err.Error())
		return fail(err.Error())
	}
	add("Tunnel « "+vpn.AdapterName+" » ouvert", true, p.Name)
	add("Kill switch activé", true, "tout ce qui ne passe pas par le tunnel est bloqué")

	// 5. Le serveur doit répondre, sinon on annule tout.
	if !s.tun.WaitHandshake(15 * time.Second) {
		add("Poignée de main avec le serveur", false, "aucune réponse en 15 s")
		return fail("le serveur VPN ne répond pas : protection annulée, connexion d'origine remise")
	}
	add("Poignée de main avec le serveur", true, "")

	// 6. Test de fuite.
	v := leak.Compare(s.boxIPs, leak.Public())
	s.lastLeak = masked(v)
	add("Test de fuite", v.V4OK && v.V6OK, v.Detail)
	if !(v.V4OK && v.V6OK) {
		res.Error = "protection active mais " + v.Detail
	}
	return res
}

// rollbackLocked défait ce que Protect a mis en place (s.mu doit être tenu).
func (s *ShieldService) rollbackLocked() []Step {
	var steps []Step
	if s.tun.Stats().Up {
		s.tun.Down()
		steps = append(steps, Step{Label: "Tunnel fermé et kill switch retiré", OK: true})
	}
	if s.ownsDNS && s.sys.Enabled() {
		steps = append(steps, s.sys.Disable("")...)
	}
	s.ownsDNS = false
	s.lastLeak = nil
	return steps
}

// Unprotect désactive la protection et rend sa connexion d'origine à Windows.
func (s *ShieldService) doUnprotect() SystemResult {
	s.cancelResume()
	s.remember(prefs.None)
	s.mu.Lock()
	defer s.mu.Unlock()
	res := SystemResult{Steps: []Step{}}
	if s.tun.Stats().Up {
		s.tun.Down()
		res.Steps = append(res.Steps, Step{Label: "Tunnel fermé et kill switch retiré", OK: true})
	}
	if s.sys.Enabled() {
		res.Steps = append(res.Steps, s.sys.Disable("")...)
	}
	s.ownsDNS = false
	s.lastLeak = nil
	return res
}

// RestoreAll remet tout comme avant Aria : tunnel, DNS de Windows, puis arrêt d'Unbound.
func (s *ShieldService) doRestoreAll() SystemResult {
	res := s.doUnprotect()
	s.dns.Stop()
	res.Steps = append(res.Steps, Step{Label: "Résolveur arrêté", OK: true})
	return res
}

// ---------- Tests ----------

// TestSystem vérifie que Windows passe par Aria, mesure les fuites et inspecte les navigateurs.
func (s *ShieldService) TestSystem() SystemTests {
	var t SystemTests
	t.Browsers = browsers.Detect()
	tun := s.tun.Stats()
	t.KillSwitch = tun.KillSwitch

	if tun.Full {
		s.mu.Lock()
		before := s.boxIPs
		s.mu.Unlock()
		t.Leak = masked(leak.Compare(before, leak.Public()))
		s.mu.Lock()
		s.lastLeak = t.Leak
		s.mu.Unlock()
	}

	// Blocage des pubs : le nom témoin de la liste doit répondre (0.0.0.0).
	t.AdBlockOn = s.prefs.Load().BlockAds
	switch {
	case !t.AdBlockOn:
		t.AdBlockDetail = "Blocage des pubs désactivé"
	case !s.dns.Running():
		t.AdBlockDetail = "Le résolveur est arrêté"
	case s.dns.Answers(blocklist.Canary):
		t.AdBlockOK = true
		t.AdBlockDetail = "La liste est chargée : " + countNote(s.ads.Info()) + " bloqués"
	default:
		t.AdBlockDetail = "Le résolveur tourne sans la liste de blocage"
	}

	if !s.sys.Enabled() {
		t.MarkerDetail = "Windows n'utilise pas encore le résolveur"
		t.AdaptersDetail = t.MarkerDetail
		return t
	}
	t.MarkerOK, t.MarkerDetail = sysdns.CheckMarker(dnsengine.MarkerName, dnsengine.MarkerIP)

	adapters := s.sys.Adapters()
	on := 0
	for _, a := range adapters {
		if a.OnAria {
			on++
		}
	}
	t.AdaptersOK = len(adapters) > 0 && on == len(adapters)
	t.AdaptersDetail = plural(on, len(adapters))
	return t
}

// plural décrit combien de cartes réseau pointent sur le résolveur.
func plural(on, total int) string {
	if total == 0 {
		return "aucune carte active"
	}
	if total == 1 {
		if on == 1 {
			return "ta carte réseau pointe sur le résolveur"
		}
		return "ta carte réseau ne pointe pas sur le résolveur"
	}
	return strconv.Itoa(on) + " cartes sur " + strconv.Itoa(total) + " pointent sur le résolveur"
}

// ---------- Mode Fantôme (Tor Browser) ----------

// TorState renvoie l'état de Tor Browser et la progression en cours (lecture rapide).
func (s *ShieldService) TorState() torbrowser.State {
	return s.tor.State()
}

// TorPrepare extrait la version embarquée si Tor Browser n'est pas encore là.
func (s *ShieldService) doTorPrepare() SystemResult {
	if s.tor.State().Installed {
		return SystemResult{Steps: []Step{}}
	}
	if err := s.tor.EnsureInstalled(); err != nil {
		return SystemResult{Steps: []Step{{Label: "Installation de Tor Browser", OK: false, Note: err.Error()}}, Error: err.Error()}
	}
	return SystemResult{Steps: []Step{
		{Label: "Signature des développeurs de Tor vérifiée", OK: true},
		{Label: "Tor Browser " + s.tor.State().Version + " installé", OK: true, Note: s.tor.State().Dir},
	}}
}

// TorCheckUpdate compare la version installée à l'index officiel de Tor.
func (s *ShieldService) TorCheckUpdate() torbrowser.UpdateInfo {
	info := s.tor.CheckUpdate()
	s.torMu.Lock()
	s.torUpdate = info
	s.torMu.Unlock()
	return info
}

// TorUpdate installe la version trouvée par la dernière vérification.
func (s *ShieldService) doTorUpdate() SystemResult {
	s.torMu.Lock()
	info := s.torUpdate
	s.torMu.Unlock()
	if err := s.tor.Update(info); err != nil {
		return SystemResult{Steps: []Step{{Label: "Mise à jour de Tor Browser", OK: false, Note: err.Error()}}, Error: err.Error()}
	}
	return SystemResult{Steps: []Step{
		{Label: "Tor Browser " + info.Latest + " téléchargé", OK: true},
		{Label: "Signature des développeurs de Tor vérifiée", OK: true},
		{Label: "Favoris, réglages et ponts conservés", OK: true},
	}}
}

// TorLaunch démarre Tor Browser sans droits administrateur.
func (s *ShieldService) doTorLaunch() SystemResult {
	if err := s.tor.Launch(); err != nil {
		return SystemResult{Steps: []Step{{Label: "Lancement de Tor Browser", OK: false, Note: err.Error()}}, Error: err.Error()}
	}
	note := "sans droits administrateur"
	if s.tun.Stats().Full {
		note += ", à travers le VPN"
	}
	return SystemResult{Steps: []Step{{Label: "Tor Browser lancé", OK: true, Note: note}}}
}

// event renvoie le message à afficher : incident DNS en priorité, sinon la note de démarrage.
func (s *ShieldService) event() string {
	if e := s.sys.LastEvent(); e != "" {
		return e
	}
	s.noticeMu.Lock()
	defer s.noticeMu.Unlock()
	return s.notice
}

// DismissNotice masque le message affiché (incident DNS ou note de démarrage).
func (s *ShieldService) DismissNotice() {
	s.sys.ClearEvent()
	s.noticeMu.Lock()
	s.notice = ""
	s.noticeMu.Unlock()
}
