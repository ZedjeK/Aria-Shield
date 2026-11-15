package main

// Zone de notification : l'icône, son infobulle et son menu suivent l'état de
// la protection, pour tout voir et tout faire sans ouvrir la fenêtre.
//
//	icône      délavée (rien ne protège), pleine (DNS) ; une seule pastille à la
//	           fois, par priorité : orange (fuite, à regarder), cyan (mode
//	           Discret, l'appli agit), bleue (nouvelle version publiée)
//	infobulle  « Aria Shield · DNS protégé · pubs bloquées »
//	menu       ouvrir · état · agir sans ouvrir (DNS, mode Discret, changer de
//	           serveur, pause des pubs) · aller à une page · voir la mise à jour
//	           (si elle existe) · démarrer avec Windows · quitter
//
// watch relit l'état toutes les 3 s et ne touche à l'icône ou au menu que s'il
// a changé. Une action du menu s'exécute hors du fil de l'interface ; pendant
// qu'elle tourne, les autres clics sont ignorés, et un échec arrive par
// notification (la fenêtre peut être fermée).

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Icônes 64 px générées à partir de build/appicon.png.
var (
	//go:embed build/tray/off.png
	iconOff []byte
	//go:embed build/tray/off-info.png
	iconOffInfo []byte
	//go:embed build/tray/dns.png
	iconDNS []byte
	//go:embed build/tray/dns-info.png
	iconDNSInfo []byte
	//go:embed build/tray/discret.png
	iconDiscret []byte
	//go:embed build/tray/alerte.png
	iconAlert []byte
)

const (
	trayInterval = 3 * time.Second
	// pauseMinutes : durée de la suspension du blocage des pubs depuis le menu.
	pauseMinutes = 10
)

// trayProfile est un profil VPN proposé dans « Changer de serveur ».
type trayProfile struct{ ID, Name string }

// trayState est ce que l'icône et le menu ont besoin de savoir.
type trayState struct {
	DNS       bool // Windows utilise le résolveur d'Aria
	Protected bool // mode Discret actif
	Leak      bool // fuite détectée pendant la protection
	VPNReady  bool // un profil VPN est enregistré
	Ads       bool // pubs bloquées en ce moment
	AdsOn     bool // blocage demandé dans les réglages
	AdsPaused bool
	Active    string // nom du profil VPN actif
	Others    []trayProfile
	Update    string // version plus récente publiée, "" sinon
	Autostart bool   // démarrage avec Windows activé
}

// trayPage est une page de l'interface où le menu peut mener.
type trayPage struct{ ID, Label string }

var trayPages = []trayPage{
	{"home", "Accueil"},
	{"dns", "Résolveur DNS"},
	{"vpn", "VPN"},
	{"ghost", "Mode Fantôme"},
	{"tests", "Tests"},
	{"settings", "Réglages"},
}

// signature résume l'état : l'icône et le menu ne sont refaits que si elle change.
func (st trayState) signature() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v|%v|%v|%v|%v|%v|%v|%s|%s|%v", st.DNS, st.Protected, st.Leak, st.VPNReady, st.Ads, st.AdsOn, st.AdsPaused, st.Active, st.Update, st.Autostart)
	for _, p := range st.Others {
		b.WriteString("|" + p.ID + "=" + p.Name)
	}
	return b.String()
}

// summary est le texte de l'infobulle et de la première ligne du menu.
func (st trayState) summary() string {
	var s string
	switch {
	case st.Protected && st.Leak:
		s = "ALERTE : fuite détectée"
	case st.Protected:
		s = "protégé (mode Discret)"
		if st.Active != "" {
			s += " via " + st.Active
		}
	case st.DNS:
		s = "DNS protégé"
	default:
		s = "non protégé"
	}
	switch {
	case st.Ads:
		s += " · pubs bloquées"
	case st.AdsPaused:
		s += " · blocage des pubs en pause"
	}
	if st.Update != "" {
		s += " · version " + st.Update + " disponible"
	}
	return s
}

// icon choisit le dessin : orange (à regarder) avant cyan (l'appli agit) avant
// bleu (information).
func (st trayState) icon() []byte {
	switch {
	case st.Protected && st.Leak:
		return iconAlert
	case st.Protected:
		return iconDiscret
	case st.DNS && st.Update != "":
		return iconDNSInfo
	case st.DNS:
		return iconDNS
	case st.Update != "":
		return iconOffInfo
	}
	return iconOff
}

// trayState lit l'état courant, sans accès réseau.
func (s *ShieldService) trayState() trayState {
	tun := s.tun.Stats()
	st := trayState{
		DNS:       s.sys.Enabled(),
		Protected: tun.Full,
		Ads:       s.adBlockActive(),
		AdsOn:     s.prefs.Load().BlockAds,
		AdsPaused: s.adPaused(),
		Update:    s.updateLatest(),
		Autostart: s.autostartOn.Load(),
	}
	s.mu.Lock()
	if s.lastLeak != nil {
		st.Leak = !(s.lastLeak.V4OK && s.lastLeak.V6OK)
	}
	s.mu.Unlock()
	if book, err := s.store.Load(); err == nil {
		cur := book.Current()
		st.VPNReady = cur.PrivateKey != ""
		st.Active = cur.Name
		for _, v := range book.Views() {
			if !v.Active {
				st.Others = append(st.Others, trayProfile{ID: v.ID, Name: v.Name})
			}
		}
	}
	return st
}

// trayUI tient l'icône à jour.
type trayUI struct {
	svc  *ShieldService
	tray *application.SystemTray
	show func()
	goTo func(page string) // ouvre la fenêtre sur une page
	mu   sync.Mutex        // refresh vient de watch et de la fin d'une action
	last string
	busy atomic.Bool
}

// watch tient l'icône à jour jusqu'à la sortie d'Aria.
func (t *trayUI) watch() {
	for !t.svc.quitting.Load() {
		time.Sleep(trayInterval)
		t.refresh()
	}
}

// refresh refait l'icône, l'infobulle et le menu si l'état a changé.
func (t *trayUI) refresh() {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := t.svc.trayState()
	sig := st.signature()
	if sig == t.last {
		return
	}
	t.last = sig
	t.tray.SetIcon(st.icon())
	t.tray.SetTooltip("Aria Shield · " + st.summary())
	t.tray.SetMenu(t.menu(st))
}

// menu construit le menu pour l'état donné.
func (t *trayUI) menu(st trayState) *application.Menu {
	m := application.NewMenu()
	m.Add("Ouvrir Aria Shield").OnClick(func(*application.Context) { t.show() })
	m.AddSeparator()
	m.Add("État : " + st.summary()).SetEnabled(false)
	m.AddSeparator()

	switch {
	case st.Protected:
		t.action(m, "Désactiver le mode Discret", func() string { return t.svc.Unprotect().Error })
		if len(st.Others) > 0 {
			sub := m.AddSubmenu("Changer de serveur")
			for _, p := range st.Others {
				id := p.ID
				t.action(sub, p.Name, func() string { return t.svc.UseVPNProfile(id).Error })
			}
		}
	default:
		if st.DNS {
			t.action(m, "Couper le DNS d'Aria", func() string { return t.svc.StopDNS().Error })
		} else {
			t.action(m, "Activer le DNS d'Aria", t.svc.enableDNSFromTray)
		}
		if st.VPNReady {
			t.action(m, "Activer le mode Discret", func() string {
				res := t.svc.Protect()
				if t.svc.tun.Stats().Full {
					return "" // en place ; un avertissement de fuite se voit sur l'icône
				}
				return res.Error
			})
		}
	}
	switch {
	case st.AdsPaused:
		t.action(m, "Reprendre le blocage des pubs", func() string { return t.svc.ResumeAdBlock().Error })
	case st.AdsOn && st.Ads:
		t.action(m, fmt.Sprintf("Suspendre le blocage des pubs %d min", pauseMinutes),
			func() string { return t.svc.PauseAdBlock(pauseMinutes).Error })
	}

	m.AddSeparator()
	nav := m.AddSubmenu("Aller à")
	for _, p := range trayPages {
		id := p.ID
		nav.Add(p.Label).OnClick(func(*application.Context) { t.goTo(id) })
	}
	if st.Update != "" {
		m.Add("Voir la mise à jour (" + st.Update + ")").OnClick(func(*application.Context) { t.goTo("settings") })
	}

	m.AddSeparator()
	on := st.Autostart
	m.AddCheckbox("Démarrer avec Windows", on).OnClick(t.handler("Démarrer avec Windows",
		func() string { return t.svc.SetAutostart(!on).Error }))
	m.Add("Quitter (rétablit ta connexion d'origine)").OnClick(func(*application.Context) { go t.svc.quit() })
	return m
}

// action ajoute une entrée qui lance do hors du fil de l'interface.
func (t *trayUI) action(m *application.Menu, label string, do func() string) {
	m.Add(label).OnClick(t.handler(label, do))
}

// handler lance do hors du fil de l'interface. Une seule action à la fois ; un
// échec est signalé par notification, la fenêtre pouvant être fermée.
func (t *trayUI) handler(label string, do func() string) func(*application.Context) {
	return func(*application.Context) {
		if !t.busy.CompareAndSwap(false, true) {
			return
		}
		go func() {
			defer t.busy.Store(false)
			if msg := do(); msg != "" {
				t.svc.alert("Aria Shield : "+label, msg)
			}
			t.refresh()
		}()
	}
}

// enableDNSFromTray démarre le résolveur puis le fait utiliser par Windows.
// Renvoie "" en cas de succès.
func (s *ShieldService) enableDNSFromTray() string {
	if !s.dns.Running() {
		if res := s.StartDNS(); res.Error != "" {
			return res.Error
		}
	}
	return s.EnableSystemDNS().Error
}
