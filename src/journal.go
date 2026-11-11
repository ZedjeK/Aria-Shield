package main

// Journal d'Aria (aria.log, dans .Aria-Shield) : chaque action demandée depuis
// l'interface y laisse son résultat et ses étapes, ainsi que ce qu'Aria fait
// d'elle-même (réactivation au lancement, relance d'Unbound, notifications,
// arrêt).
//
// Les méthodes appelées par l'interface sont ici de simples enveloppes : elles
// appellent le vrai travail (doStopDNS, doUnprotect…, dans shieldservice.go,
// startup.go et adblock.go) puis notent son résultat. Protect et
// EnableSystemDNS, qui avaient déjà leur enveloppe, se journalisent sur place.
//
// Rien de secret n'est écrit : les étapes sont celles affichées à l'écran, où
// l'IP de la box est déjà masquée, et aucune clé ne passe par elles.

import (
	"os"

	"aria-shield/internal/sysx"
)

// logf écrit une ligne dans le journal.
func (s *ShieldService) logf(format string, args ...any) {
	s.log.Printf(format, args...)
}

// logResult note le résultat d'une action, puis ses étapes.
func (s *ShieldService) logResult(title string, steps []Step, errMsg string) {
	if errMsg != "" {
		s.logf("%s : ÉCHEC : %s", title, errMsg)
	} else {
		s.logf("%s : fait", title)
	}
	for _, st := range steps {
		mark := "ok"
		if !st.OK {
			mark = "ÉCHEC"
		}
		if st.Note != "" {
			s.logf("    [%s] %s · %s", mark, st.Label, st.Note)
		} else {
			s.logf("    [%s] %s", mark, st.Label)
		}
	}
}

// logStart note le lancement : version, exe, droits, mode silencieux.
func (s *ShieldService) logStart(silent bool) {
	exe, _ := os.Executable()
	mode := "fenêtre ouverte"
	if silent {
		mode = "lancement silencieux (ouverture de session)"
	}
	s.logf("=== Aria Shield %s démarre : %s ; administrateur : %v ; %s", Version, exe, sysx.IsAdmin(), mode)
}

// onOff : « activer » ou « couper », pour les titres du journal.
func onOff(on bool, what string) string {
	if on {
		return what + " : activation"
	}
	return what + " : arrêt"
}

// ---------- Résolveur ----------

func (s *ShieldService) StartDNS() ActionResult {
	res := s.doStartDNS()
	steps := make([]Step, len(res.Steps))
	for i, st := range res.Steps {
		steps[i] = Step{Label: st.Label, OK: st.OK, Note: st.Note}
	}
	s.logResult("Démarrage du résolveur", steps, res.Error)
	return res
}

func (s *ShieldService) StopDNS() SystemResult {
	res := s.doStopDNS()
	s.logResult("Arrêt du résolveur", res.Steps, res.Error)
	return res
}

func (s *ShieldService) DisableSystemDNS() SystemResult {
	res := s.doDisableSystemDNS()
	s.logResult("Retour au DNS d'origine", res.Steps, res.Error)
	return res
}

// ---------- VPN ----------

func (s *ShieldService) ImportVPNConf() ImportResult {
	res := s.doImportVPNConf()
	if !res.Canceled {
		s.logResult("Import de configurations WireGuard", res.Steps, res.Error)
	}
	return res
}

func (s *ShieldService) DeleteVPNProfile(id string) SystemResult {
	res := s.doDeleteVPNProfile(id)
	s.logResult("Suppression d'un profil VPN", res.Steps, res.Error)
	return res
}

func (s *ShieldService) UseVPNProfile(id string) SystemResult {
	res := s.doUseVPNProfile(id)
	s.logResult("Choix ou bascule de profil VPN", res.Steps, res.Error)
	return res
}

func (s *ShieldService) TestVPN() SystemResult {
	res := s.doTestVPN()
	s.logResult("Test de connexion au serveur VPN", res.Steps, res.Error)
	return res
}

// ---------- Protection ----------

func (s *ShieldService) Unprotect() SystemResult {
	res := s.doUnprotect()
	s.logResult("Désactivation de la protection", res.Steps, res.Error)
	return res
}

func (s *ShieldService) RestoreAll() SystemResult {
	res := s.doRestoreAll()
	s.logResult("Restauration complète", res.Steps, res.Error)
	return res
}

// ---------- Tor Browser ----------

func (s *ShieldService) TorPrepare() SystemResult {
	res := s.doTorPrepare()
	if len(res.Steps) > 0 || res.Error != "" {
		s.logResult("Installation de Tor Browser", res.Steps, res.Error)
	}
	return res
}

func (s *ShieldService) TorUpdate() SystemResult {
	res := s.doTorUpdate()
	s.logResult("Mise à jour de Tor Browser", res.Steps, res.Error)
	return res
}

func (s *ShieldService) TorLaunch() SystemResult {
	res := s.doTorLaunch()
	s.logResult("Lancement de Tor Browser", res.Steps, res.Error)
	return res
}

// ---------- Réglages ----------

func (s *ShieldService) SetAutostart(on bool) SystemResult {
	res := s.doSetAutostart(on)
	s.logResult(onOff(on, "Démarrage avec Windows"), res.Steps, res.Error)
	return res
}

func (s *ShieldService) SetResume(on bool) SystemResult {
	res := s.doSetResume(on)
	s.logResult(onOff(on, "Réactivation au lancement"), res.Steps, res.Error)
	return res
}

func (s *ShieldService) SetNotifications(on bool) SystemResult {
	res := s.doSetNotifications(on)
	s.logResult(onOff(on, "Notifications Windows"), res.Steps, res.Error)
	return res
}

// ---------- Blocage des pubs ----------

func (s *ShieldService) SetAdBlock(on bool) SystemResult {
	res := s.doSetAdBlock(on)
	s.logResult(onOff(on, "Blocage des pubs"), res.Steps, res.Error)
	return res
}

func (s *ShieldService) UpdateBlocklist() SystemResult {
	res := s.doUpdateBlocklist()
	s.logResult("Mise à jour de la liste de blocage", res.Steps, res.Error)
	return res
}

func (s *ShieldService) AddAllow(domain string) SystemResult {
	res := s.doAddAllow(domain)
	s.logResult("Exception au blocage", res.Steps, res.Error)
	return res
}

func (s *ShieldService) RemoveAllow(domain string) SystemResult {
	res := s.doRemoveAllow(domain)
	s.logResult("Exception au blocage (retrait)", res.Steps, res.Error)
	return res
}
