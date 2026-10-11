package main

import (
	"context"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"aria-shield/internal/browsers"
	"aria-shield/internal/dnsengine"
	"aria-shield/internal/leak"
	"aria-shield/internal/paths"
	"aria-shield/internal/sysdns"
	"aria-shield/internal/sysx"
	"aria-shield/internal/torbrowser"
	"aria-shield/internal/vpn"
)

// Step est une ligne du journal affiché dans l'interface.
type Step = sysdns.Step

// ShieldService expose au frontend les actions d'Aria Shield.
type ShieldService struct {
	dns   *dnsengine.Engine
	sys   *sysdns.Manager
	store *vpn.Store
	tun   *vpn.Tunnel
	tor   *torbrowser.Manager

	userDir  string // %LOCALAPPDATA%\AriaShield
	sysDir   string // %PROGRAMDATA%\AriaShield (réservé aux administrateurs)
	noticeMu sync.Mutex
	notice   string // message affiché une fois (migration, erreur de dossier)

	torMu     sync.Mutex
	torUpdate torbrowser.UpdateInfo // dernière vérification des mises à jour

	mu       sync.Mutex // sérialise Protect / Unprotect
	boxIPs   leak.IPs   // IP publique mesurée juste avant la protection
	lastLeak *leak.Verdict
	ownsDNS  bool // la bascule du DNS de Windows a été faite par Protect
}

// NewShieldService prépare le service et ses moteurs (rien n'est lancé).
func NewShieldService() *ShieldService {
	s := &ShieldService{tun: vpn.NewTunnel()}

	var errs []string
	userDir, err := paths.UserDir()
	if err != nil {
		errs = append(errs, "dossier utilisateur : "+err.Error())
	}
	sysDir, err := paths.SystemDir()
	if err != nil {
		errs = append(errs, "dossier système non verrouillé : "+err.Error())
	}
	s.userDir, s.sysDir = userDir, sysDir

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

	s.dns = dnsengine.New(sysDir)
	s.sys = sysdns.New(sysDir)
	s.sys.Exclude(vpn.AdapterName) // la carte VPN est configurée par le tunnel lui-même
	s.store = vpn.NewStore(userDir)
	s.tor = torbrowser.New(userDir, sysDir)

	// Filets de sécurité si Unbound s'arrête tout seul.
	s.dns.OnUnexpectedExit(func() {
		if s.tun.Stats().Full {
			// Protection active : on relance Unbound, sinon on coupe tout proprement.
			if _, err := s.dns.Start(); err == nil {
				return
			}
			s.tun.Down()
			s.sys.Disable("Unbound s'est arrêté : protection coupée et connexion d'origine remise.")
			return
		}
		if s.sys.Enabled() {
			s.sys.Disable("Unbound s'est arrêté : ton DNS d'origine a été remis automatiquement.")
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
	VPN        vpn.View      `json:"vpn"`
	Tunnel     vpn.Stats     `json:"tunnel"`
	Leak       *leak.Verdict `json:"leak"`
	Components []Component   `json:"components"`
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

// SystemTests regroupe les vérifications côté Windows.
type SystemTests struct {
	MarkerOK       bool            `json:"markerOk"`
	MarkerDetail   string          `json:"markerDetail"`
	AdaptersOK     bool            `json:"adaptersOk"`
	AdaptersDetail string          `json:"adaptersDetail"`
	Leak           *leak.Verdict   `json:"leak"`
	KillSwitch     bool            `json:"killSwitch"`
	Browsers       []browsers.Info `json:"browsers"`
}

// ServiceStartup : si Aria avait planté en laissant Windows sur le résolveur, on répare.
func (s *ShieldService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.sys.RecoverLeftover()
	return nil
}

// ServiceShutdown : à la fermeture, tout revient comme avant.
func (s *ShieldService) ServiceShutdown() error {
	s.tun.Down()
	if s.sys.Enabled() {
		s.sys.Disable("")
	}
	s.dns.Stop()
	return nil
}

// Status renvoie l'état des droits, des moteurs et de la protection.
func (s *ShieldService) Status() Status {
	running := s.dns.Running()
	tun := s.tun.Stats()
	profile, _ := s.store.Load()

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
		Tunnel:     tun,
		Leak:       lastLeak,
		Components: []Component{
			unbound,
			wg,
			tor,
		},
	}
}

// Adapters liste les cartes réseau actives et leurs serveurs DNS.
func (s *ShieldService) Adapters() []sysdns.AdapterView {
	return s.sys.Adapters()
}

// ---------- Résolveur ----------

// StartDNS démarre le résolveur embarqué puis le teste.
func (s *ShieldService) StartDNS() ActionResult {
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
func (s *ShieldService) StopDNS() SystemResult {
	if s.tun.Stats().Full {
		return SystemResult{Error: "désactive d'abord la protection : le résolveur en fait partie"}
	}
	var res SystemResult
	if s.sys.Enabled() {
		res.Steps = s.sys.Disable("")
	}
	s.dns.Stop()
	return res
}

// TestDNS relance les tests de résolution et de DNSSEC.
func (s *ShieldService) TestDNS() dnsengine.TestResult {
	return s.dns.Test()
}

// EnableSystemDNS fait utiliser le résolveur par tout Windows, après vérification.
func (s *ShieldService) EnableSystemDNS() SystemResult {
	var res SystemResult
	res.Steps, res.Error = s.enableSystemDNS()
	return res
}

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
func (s *ShieldService) DisableSystemDNS() SystemResult {
	if s.tun.Stats().Full {
		return SystemResult{Error: "désactive d'abord la protection : le DNS de Windows en fait partie"}
	}
	return SystemResult{Steps: s.sys.Disable("")}
}

// ---------- Profil VPN ----------

// VPNProfile renvoie le profil enregistré, sans jamais la clé privée.
func (s *ShieldService) VPNProfile() vpn.View {
	p, _ := s.store.Load()
	return p.View()
}

// SaveVPNProfile valide puis enregistre le profil (clé privée vide = garder l'actuelle).
func (s *ShieldService) SaveVPNProfile(in vpn.Input) SaveResult {
	if s.tun.Stats().Up {
		return SaveResult{Error: "désactive la protection avant de modifier le profil"}
	}
	current, err := s.store.Load()
	if err != nil {
		return SaveResult{Error: err.Error()}
	}
	p, err := in.Validate(current.PrivateKey)
	if err != nil {
		if fe, ok := err.(vpn.FieldErrors); ok {
			return SaveResult{Fields: fe, Error: "certains champs sont à corriger"}
		}
		return SaveResult{Error: err.Error()}
	}
	if err := s.store.Save(p); err != nil {
		return SaveResult{Error: "enregistrement impossible : " + err.Error()}
	}
	return SaveResult{Profile: p.View()}
}

// DeleteVPNProfile efface le profil enregistré.
func (s *ShieldService) DeleteVPNProfile() SystemResult {
	if s.tun.Stats().Up {
		return SystemResult{Error: "désactive la protection avant de supprimer le profil"}
	}
	if err := s.store.Delete(); err != nil {
		return SystemResult{Error: err.Error()}
	}
	return SystemResult{Steps: []Step{{Label: "Profil VPN supprimé", OK: true}}}
}

// TestVPN ouvre le tunnel quelques secondes, sans routes ni kill switch,
// pour vérifier que le serveur répond.
func (s *ShieldService) TestVPN() SystemResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res SystemResult
	if s.tun.Stats().Up {
		res.Error = "le tunnel est déjà ouvert"
		return res
	}
	p, err := s.store.Load()
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

// Protect active le mode Discret : résolveur, DNS de Windows, tunnel et kill switch.
func (s *ShieldService) Protect() SystemResult {
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
	p, err := s.store.Load()
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
func (s *ShieldService) Unprotect() SystemResult {
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
func (s *ShieldService) RestoreAll() SystemResult {
	res := s.Unprotect()
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
func (s *ShieldService) TorPrepare() SystemResult {
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
func (s *ShieldService) TorUpdate() SystemResult {
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
func (s *ShieldService) TorLaunch() SystemResult {
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
