package main

// Commande de secours du verrou persistant (lot 7) : « Aria-Shield.exe
// --deverrouiller » retire le verrou sans ouvrir l'interface, par exemple
// quand le serveur VPN ne répond plus et qu'Internet reste coupé.

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"

	"aria-shield/internal/autostart"
	"aria-shield/internal/killswitch"
	"aria-shield/internal/prefs"
	"aria-shield/internal/sysx"
)

// unlockArg : l'argument qui retire le verrou et quitte.
const unlockArg = "--deverrouiller"

// runUnlock retire le verrou, le dit dans une fenêtre et renvoie le code de sortie.
func runUnlock() int {
	msg, icon, code := "Verrou retiré : Aria ne bloque plus ta connexion.", uint32(windows.MB_ICONINFORMATION), 0
	if err := killswitch.DisableLock(); err != nil {
		msg, icon, code = "Le verrou n'a pas pu être retiré : "+err.Error(), windows.MB_ICONERROR, 1
	}
	text, _ := windows.UTF16PtrFromString(msg)
	title, _ := windows.UTF16PtrFromString("Aria Shield")
	windows.MessageBox(0, text, title, windows.MB_OK|icon)
	return code
}

// ---------- Verrou au démarrage, côté service ----------

// lockCheck renvoie l'exe que le verrou laissera sortir, ou la raison pour
// laquelle le verrou n'est pas possible maintenant ("" si tout va bien).
// Interroge le Planificateur de tâches : pas en boucle.
func (s *ShieldService) lockCheck() (exe, why string) {
	if !sysx.IsAdmin() {
		return "", "Aria doit être lancée en administrateur."
	}
	st := autostart.Status()
	s.autostartOn.Store(st.Enabled)
	if !st.Enabled || st.Exe == "" {
		return "", "Active d'abord « Démarrer avec Windows » : le verrou ne laisse sortir que la copie d'Aria de Program Files, celle que lance le démarrage avec Windows."
	}
	if !s.prefs.Load().Resume {
		return "", "Active d'abord « Réactiver ma dernière protection » : sans elle, Internet resterait coupé après un redémarrage."
	}
	self, err := os.Executable()
	if err != nil || !strings.EqualFold(filepath.Clean(self), filepath.Clean(st.Exe)) {
		return "", "Aria tourne depuis " + self + " : le verrou ne laisse sortir que sa copie de Program Files (" + st.Exe + "). Quitte Aria, puis relance-la par le démarrage avec Windows."
	}
	p, err := s.store.Active()
	if err != nil || p.PrivateKey == "" {
		return "", "Enregistre d'abord un profil VPN (page VPN)."
	}
	host, _, err := net.SplitHostPort(p.Endpoint)
	if err != nil {
		host = p.Endpoint
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return "", "Le serveur de ton profil VPN est désigné par un nom (" + host + ") : sous verrou, Aria ne pourrait pas le résoudre. Choisis un profil dont le serveur est une adresse IP."
	}
	return st.Exe, ""
}

// postLock pose (ou repose) le verrou une fois le tunnel monté.
func (s *ShieldService) postLock(exe string) Step {
	if err := killswitch.EnableLock(exe); err != nil {
		return Step{Label: "Verrou au démarrage", OK: false, Note: err.Error()}
	}
	s.lockOn.Store(true)
	return Step{Label: "Verrou au démarrage posé", OK: true, Note: "hors du tunnel, rien ne sort, même après un redémarrage"}
}

// dropLock retire le verrou s'il est posé ; was dit s'il l'était.
func (s *ShieldService) dropLock(why string) (st Step, was bool) {
	if on, err := killswitch.LockInstalled(); err == nil && !on {
		s.lockOn.Store(false)
		return Step{}, false
	}
	if err := killswitch.DisableLock(); err != nil {
		return Step{Label: "Retrait du verrou", OK: false, Note: err.Error()}, true
	}
	s.lockOn.Store(false)
	return Step{Label: "Verrou retiré", OK: true, Note: why}, true
}

// lockSettingOff coupe le réglage du verrou et retire le verrou, quand un
// réglage dont il dépend vient d'être coupé.
func (s *ShieldService) lockSettingOff(why string) []Step {
	if !s.prefs.Load().Verrou {
		return nil
	}
	steps := []Step{{Label: "Verrou au démarrage désactivé", OK: true, Note: why}}
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.Verrou = false }); err != nil {
		steps[0] = Step{Label: "Verrou au démarrage", OK: false, Note: "réglage non enregistré : " + err.Error()}
	}
	killswitch.SetHardTunnel(false)
	if st, was := s.dropLock(why); was {
		steps = append(steps, st)
	}
	return steps
}

// doSetLock active ou coupe le verrou au démarrage. Activé, le verrou est
// posé à la prochaine activation du mode Discret : le kill switch doit être
// reposé avec les sorties du tunnel autorisées en dur, sinon le verrou les
// bloquerait.
func (s *ShieldService) doSetLock(on bool) SystemResult {
	if !on {
		res := SystemResult{Steps: s.lockSettingOff("réglage désactivé")}
		for _, st := range res.Steps {
			if !st.OK {
				res.Error = st.Note
			}
		}
		return res
	}
	if _, why := s.lockCheck(); why != "" {
		return SystemResult{Steps: []Step{{Label: "Verrou au démarrage", OK: false, Note: why}}, Error: why}
	}
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.Verrou = true }); err != nil {
		return SystemResult{Error: "réglage non enregistré : " + err.Error()}
	}
	note := "posé à la prochaine activation du mode Discret"
	if s.tun.Stats().Full {
		note = "posé à la prochaine activation du mode Discret : désactive puis réactive la protection pour le poser maintenant"
	}
	return SystemResult{Steps: []Step{
		{Label: "Verrou au démarrage activé", OK: true, Note: "seule la copie d'Aria de Program Files pourra sortir hors du tunnel"},
		{Label: "Pose du verrou", OK: true, Note: note},
	}}
}

// checkLockAtStart relit le verrou au lancement. Il reste posé seulement si
// Aria va remonter le tunnel (réglage actif, mode Discret à réactiver, exe
// et profil compatibles) ; sinon il est retiré, pour ne pas laisser Internet
// coupé sans raison.
func (s *ShieldService) checkLockAtStart() {
	on, err := killswitch.LockInstalled()
	if err != nil {
		s.logf("verrou : lecture impossible (%v)", err)
		return
	}
	s.lockOn.Store(on)
	if !on {
		return
	}
	p := s.prefs.Load()
	why := ""
	switch {
	case !p.Verrou:
		why = "le verrou au démarrage est désactivé"
	case !p.Resume || p.Last != prefs.Discret:
		why = "aucun mode Discret à réactiver"
	default:
		_, why = s.lockCheck()
	}
	if why == "" {
		s.logf("verrou posé : Internet coupé hors d'Aria jusqu'à la remontée du tunnel")
		return
	}
	st, _ := s.dropLock(why)
	if !st.OK {
		s.logf("verrou : retrait impossible au lancement : %s", st.Note)
		return
	}
	s.logf("verrou retiré au lancement : %s", why)
	s.addNotice("Verrou au démarrage retiré : " + why + ".")
}
