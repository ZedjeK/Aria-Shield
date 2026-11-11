package main

// Versions d'Aria : prévenir quand une autre version est lancée pendant que
// celle-ci tourne, et quand une version plus récente est publiée.
//
//	otherVersionLaunched   un autre exe a été lancé : on le dit au lieu de
//	                       rouvrir la fenêtre en silence
//	UpdateInfo, CheckUpdate, SetUpdateCheck, checkUpdateAtStart
//	                       dernière version publiée sur GitHub
//
// La vérification interroge api.github.com (une requête, sans identifiant) :
// GitHub voit donc ton adresse IP, ou celle du VPN en mode Discret. Elle se
// coupe dans les réglages. Aria ne télécharge et n'installe rien toute seule :
// elle indique seulement où trouver la nouvelle version.

import (
	"context"
	"strings"
	"time"

	"aria-shield/internal/appversion"
	"aria-shield/internal/prefs"
)

// updateDelay : la vérification au lancement attend le réseau et la reprise.
const updateDelay = 4 * time.Minute

// UpdateState décrit la dernière vérification, pour l'interface.
type UpdateState struct {
	Enabled   bool   `json:"enabled"`   // vérification au lancement activée
	Checked   bool   `json:"checked"`   // une vérification a abouti
	Current   string `json:"current"`   // version en cours
	Latest    string `json:"latest"`    // dernière version publiée
	Available bool   `json:"available"` // Latest est plus récente que Current
	URL       string `json:"url"`
	Error     string `json:"error"`
}

// otherVersionLaunched est appelée quand un exe d'Aria est lancé alors que
// celle-ci tourne déjà. other est la version annoncée par cet exe ("" pour une
// version antérieure à la 0.11, qui ne l'annonçait pas).
func (s *ShieldService) otherVersionLaunched(other string) {
	if _, ok := appversion.Parse(other); !ok {
		other = ""
	}
	if cmp, ok := appversion.Compare(other, Version); other != "" && ok && cmp == 0 {
		s.logf("exe relancé alors qu'Aria tourne déjà (même version) : fenêtre ramenée au premier plan")
		return
	}
	what := "Une version plus ancienne d'Aria Shield"
	if other != "" {
		what = "Aria Shield " + strings.TrimPrefix(strings.TrimSpace(other), "v")
	}
	msg := what + " vient d'être lancée, mais c'est la " + Version +
		" qui tourne encore, et c'est sa fenêtre que tu vois. Pour changer de version : « Quitter » (clic droit sur l'icône, ou page réglages), puis relance l'autre exe."
	s.logf("autre version lancée (%q) alors que la %s tourne", other, Version)
	s.addNotice(msg)
	s.alert("Une autre version d'Aria tourne déjà", msg)
}

// UpdateInfo renvoie le résultat de la dernière vérification (sans réseau).
func (s *ShieldService) UpdateInfo() UpdateState {
	s.updMu.Lock()
	st := s.update
	s.updMu.Unlock()
	st.Enabled = s.prefs.Load().CheckUpdates
	st.Current = Version
	st.URL = appversion.ReleasesURL
	return st
}

// CheckUpdate interroge GitHub maintenant (bouton des réglages).
func (s *ShieldService) CheckUpdate() UpdateState {
	s.fetchLatest()
	return s.UpdateInfo()
}

// SetUpdateCheck active ou coupe la vérification au lancement.
func (s *ShieldService) SetUpdateCheck(on bool) SystemResult {
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.CheckUpdates = on }); err != nil {
		return SystemResult{Error: "réglage non enregistré : " + err.Error()}
	}
	s.logf("%s", onOff(on, "Vérification des nouvelles versions"))
	return SystemResult{}
}

// updateLatest : version plus récente connue, "" sinon (lecture rapide, pour Status).
func (s *ShieldService) updateLatest() string {
	s.updMu.Lock()
	defer s.updMu.Unlock()
	if s.update.Available {
		return s.update.Latest
	}
	return ""
}

// checkUpdateAtStart vérifie une fois, quelques minutes après le lancement, et
// prévient si une version plus récente existe. Un échec reste silencieux.
func (s *ShieldService) checkUpdateAtStart() {
	time.Sleep(updateDelay)
	if s.quitting.Load() || !s.prefs.Load().CheckUpdates {
		return
	}
	if st := s.fetchLatest(); st.Available {
		s.alert("Nouvelle version d'Aria Shield",
			"La "+st.Latest+" est disponible (tu as la "+Version+"). Téléchargement : "+appversion.ReleasesURL)
	}
}

// fetchLatest lit la dernière release publiée et retient le résultat.
func (s *ShieldService) fetchLatest() UpdateState {
	tag, err := appversion.LatestTag(context.Background())
	var st UpdateState
	if err != nil {
		st.Error = "vérification impossible : " + err.Error()
		s.logf("nouvelles versions : %s", st.Error)
	} else {
		st.Checked = true
		st.Latest = strings.TrimPrefix(tag, "v")
		cmp, _ := appversion.Compare(tag, Version)
		st.Available = cmp > 0
		s.logf("nouvelles versions : dernière publiée %s, en cours %s", st.Latest, Version)
	}
	s.updMu.Lock()
	s.update = st
	s.updMu.Unlock()
	return st
}
