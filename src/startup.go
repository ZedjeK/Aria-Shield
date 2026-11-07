package main

// Démarrage avec Windows, mémoire de la dernière protection et sortie d'Aria.
//
//	Settings, SetAutostart, SetResume   réglages affichés dans l'interface
//	resume                              réactivation au lancement
//	refreshAutostart                    copie de Program Files à jour
//	Quit                                quitter vraiment (tout est restauré)
//
// La dernière protection choisie (rien, DNS, mode Discret) est retenue à chaque
// action de l'utilisateur (shieldservice.go, remember). Au lancement suivant,
// resume la réactive : à l'ouverture de session, le réseau n'est pas toujours
// prêt, d'où plusieurs essais pendant 3 minutes. Toute action de l'utilisateur
// pendant ce temps arrête la reprise (cancelResume) : c'est lui qui décide.

import (
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"aria-shield/internal/autostart"
	"aria-shield/internal/prefs"
	"aria-shield/internal/sysx"
)

const (
	resumeWindow = 3 * time.Minute  // durée maximale des essais de reprise
	resumeRetry  = 15 * time.Second // pause entre deux essais
)

// Settings décrit les réglages de démarrage, pour l'interface.
type Settings struct {
	Autostart autostart.State `json:"autostart"`
	Resume    bool            `json:"resume"`
	Last      string          `json:"last"` // aucune, dns ou discret
}

// Settings lit les réglages (interroge le Planificateur de tâches : à
// n'appeler qu'à l'ouverture de la carte, pas toutes les 5 s).
func (s *ShieldService) Settings() Settings {
	p := s.prefs.Load()
	return Settings{Autostart: autostart.Status(), Resume: p.Resume, Last: p.Last}
}

// SetAutostart crée ou supprime la tâche qui lance Aria à l'ouverture de session.
func (s *ShieldService) SetAutostart(on bool) SystemResult {
	exe, err := os.Executable()
	if err != nil {
		return SystemResult{Error: "emplacement de l'exe introuvable : " + err.Error()}
	}
	if on {
		if err := autostart.Enable(exe); err != nil {
			return SystemResult{
				Steps: []Step{{Label: "Démarrage avec Windows", OK: false, Note: err.Error()}},
				Error: err.Error(),
			}
		}
		st := autostart.Status()
		return SystemResult{Steps: []Step{
			{Label: "Copie protégée de l'exe", OK: true, Note: st.Exe},
			{Label: "Tâche « " + autostart.TaskName + " » créée", OK: true, Note: "à l'ouverture de ta session, sans fenêtre UAC"},
		}}
	}
	if err := autostart.Disable(exe); err != nil {
		return SystemResult{
			Steps: []Step{{Label: "Arrêt du démarrage avec Windows", OK: false, Note: err.Error()}},
			Error: err.Error(),
		}
	}
	return SystemResult{Steps: []Step{
		{Label: "Tâche planifiée supprimée", OK: true},
		{Label: "Copie de Program Files retirée", OK: true},
	}}
}

// SetResume active ou non la réactivation de la dernière protection au lancement.
func (s *ShieldService) SetResume(on bool) SystemResult {
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.Resume = on }); err != nil {
		return SystemResult{Error: "réglage non enregistré : " + err.Error()}
	}
	return SystemResult{}
}

// Quit ferme vraiment Aria : ServiceShutdown remet ta connexion comme avant.
// Lancé à part pour que l'appel de l'interface puisse d'abord répondre.
func (s *ShieldService) Quit() {
	go application.Get().Quit()
}

// remember retient la dernière protection choisie par l'utilisateur.
func (s *ShieldService) remember(last string) {
	s.prefs.SetLast(last) // échec sans gravité : seule la reprise en dépend
}

// refreshAutostart remet à jour la copie lancée par la tâche planifiée, si tu
// as lancé une autre version d'Aria que celle de Program Files.
func (s *ShieldService) refreshAutostart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if _, err := autostart.Refresh(exe); err != nil {
		s.addNotice("La copie d'Aria lancée au démarrage de Windows n'a pas pu être mise à jour : " + err.Error())
	}
}

// resume réactive la dernière protection choisie, si le réglage le permet.
func (s *ShieldService) resume() {
	p := s.prefs.Load()
	if !p.Resume || p.Last == prefs.None || !sysx.IsAdmin() {
		return
	}
	what := "le DNS"
	if p.Last == prefs.Discret {
		what = "le mode Discret"
	}
	stop := s.beginResume()
	defer s.endResume(stop)

	deadline := time.Now().Add(resumeWindow)
	for attempt := 1; ; attempt++ {
		s.setResuming(fmt.Sprintf("Réactivation de %s (essai %d)…", what, attempt))
		msg := s.resumeOnce(p.Last)
		select {
		case <-stop:
			return // l'utilisateur a repris la main entre-temps
		default:
		}
		if msg == "" {
			s.addNotice("Ta dernière protection est réactivée : " + what + ".")
			return
		}
		if time.Now().After(deadline) {
			s.addNotice("Impossible de réactiver " + what + " au lancement : " + msg)
			return
		}
		select {
		case <-stop:
			return
		case <-time.After(resumeRetry):
		}
	}
}

// resumeOnce fait un essai de reprise. Renvoie "" en cas de succès.
func (s *ShieldService) resumeOnce(last string) string {
	if last == prefs.Discret {
		res := s.protect()
		if s.tun.Stats().Full {
			return "" // protection en place (un avertissement de fuite reste affiché)
		}
		return res.Error
	}
	if !s.dns.Running() {
		if _, err := s.dns.Start(); err != nil {
			return err.Error()
		}
	}
	if s.sys.Enabled() {
		return ""
	}
	_, msg := s.enableSystemDNS()
	return msg
}

// beginResume ouvre une reprise ; cancelResume la referme.
func (s *ShieldService) beginResume() chan struct{} {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	s.resumeStop = make(chan struct{})
	return s.resumeStop
}

func (s *ShieldService) endResume(stop chan struct{}) {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	if s.resumeStop == stop {
		s.resumeStop = nil
	}
	s.resuming = ""
}

// cancelResume arrête une reprise en cours : appelée par chaque action de
// l'utilisateur et à la fermeture d'Aria.
func (s *ShieldService) cancelResume() {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	if s.resumeStop != nil {
		close(s.resumeStop)
		s.resumeStop = nil
	}
	s.resuming = ""
}

func (s *ShieldService) setResuming(text string) {
	s.resumeMu.Lock()
	s.resuming = text
	s.resumeMu.Unlock()
}

func (s *ShieldService) resumingText() string {
	s.resumeMu.Lock()
	defer s.resumeMu.Unlock()
	return s.resuming
}

// addNotice ajoute un message au bandeau d'information.
func (s *ShieldService) addNotice(text string) {
	s.noticeMu.Lock()
	defer s.noticeMu.Unlock()
	if s.notice != "" {
		s.notice += " " + text
	} else {
		s.notice = text
	}
}
