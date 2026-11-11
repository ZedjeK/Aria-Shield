package main

// Blocage des pubs et des traceurs par le résolveur.
//
//	AdBlock                      état affiché sur la page Résolveur DNS
//	SetAdBlock, UpdateBlocklist  activer, couper, retélécharger la liste
//	AddAllow, RemoveAllow        exceptions de l'utilisateur
//	refreshBlocklist             mise à jour hebdomadaire, au lancement
//
// La liste est téléchargée et filtrée par internal/blocklist, rangée dans le
// dossier système verrouillé, puis chargée par Unbound comme fichier inclus.
// Le canal de commande d'Unbound étant fermé, appliquer une liste le redémarre
// (une à deux secondes sans résolution). Si Unbound refuse de démarrer avec la
// liste, Aria la retire et le relance sans elle : le blocage ne doit jamais
// coûter le résolveur.

import (
	"context"
	"strconv"
	"time"

	"aria-shield/internal/blocklist"
	"aria-shield/internal/prefs"
)

// refreshDelay : la mise à jour hebdomadaire attend que le réseau et la
// reprise de la protection soient en place.
const refreshDelay = 3 * time.Minute

// AdBlock décrit le blocage, pour l'interface.
type AdBlock struct {
	Enabled bool           `json:"enabled"` // réglage de l'utilisateur
	Active  bool           `json:"active"`  // liste chargée par l'Unbound en cours
	List    blocklist.Info `json:"list"`
	Allow   []string       `json:"allow"`
	Source  string         `json:"source"`
	URL     string         `json:"url"`
}

// AdBlock renvoie l'état du blocage.
func (s *ShieldService) AdBlock() AdBlock {
	p := s.prefs.Load()
	allow := blocklist.CleanAllow(p.Allow)
	if allow == nil {
		allow = []string{}
	}
	return AdBlock{
		Enabled: p.BlockAds,
		Active:  s.adBlockActive(),
		List:    s.ads.Info(),
		Allow:   allow,
		Source:  blocklist.SourceName,
		URL:     blocklist.SourceURL,
	}
}

// adBlockActive : Unbound tourne-t-il avec la liste ? (lecture rapide, pour Status)
func (s *ShieldService) adBlockActive() bool {
	return s.dns.Running() && s.dns.Include() != ""
}

// SetAdBlock active ou coupe le blocage. À l'activation, la liste est
// téléchargée si elle manque ou date de plus d'une semaine ; si le
// téléchargement échoue et qu'aucune liste n'est installée, rien n'est activé.
func (s *ShieldService) doSetAdBlock(on bool) SystemResult {
	s.adMu.Lock()
	defer s.adMu.Unlock()
	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}

	if on && s.ads.Stale() {
		info, err := s.ads.Update(context.Background(), s.prefs.Load().Allow)
		switch {
		case err == nil:
			add("Liste "+blocklist.SourceName+" téléchargée et filtrée", true, countNote(info))
		case info.Present:
			add("Mise à jour de la liste", false, err.Error()+" : la liste déjà installée est gardée")
		default:
			add("Téléchargement de la liste", false, err.Error())
			res.Error = "blocage non activé : " + err.Error()
			return res
		}
	}
	if err := s.prefs.Update(func(p *prefs.Prefs) { p.BlockAds = on }); err != nil {
		res.Error = "réglage non enregistré : " + err.Error()
		return res
	}
	if on {
		add("Blocage des pubs activé", true, countNote(s.ads.Info()))
	} else {
		add("Blocage des pubs coupé", true, "")
	}
	s.applyBlocklist(on, add, &res)
	return res
}

// UpdateBlocklist retélécharge la liste maintenant.
func (s *ShieldService) doUpdateBlocklist() SystemResult {
	s.adMu.Lock()
	defer s.adMu.Unlock()
	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}
	p := s.prefs.Load()
	info, err := s.ads.Update(context.Background(), p.Allow)
	if err != nil {
		add("Mise à jour de la liste", false, err.Error())
		res.Error = err.Error()
		return res
	}
	add("Liste "+blocklist.SourceName+" téléchargée et filtrée", true, countNote(info))
	s.applyBlocklist(p.BlockAds, add, &res)
	return res
}

// AddAllow ajoute une exception : ce domaine et ses sous-domaines ne sont plus bloqués.
func (s *ShieldService) doAddAllow(domain string) SystemResult {
	d, ok := blocklist.ValidDomain(domain)
	if !ok {
		return SystemResult{Error: "ce n'est pas un nom de domaine (exemple : suivi.exemple.fr)"}
	}
	return s.changeAllow("Exception ajoutée : "+d, func(allow []string) []string {
		return append(allow, d)
	})
}

// RemoveAllow retire une exception.
func (s *ShieldService) doRemoveAllow(domain string) SystemResult {
	d, _ := blocklist.ValidDomain(domain)
	return s.changeAllow("Exception retirée : "+d, func(allow []string) []string {
		out := allow[:0:0]
		for _, a := range allow {
			if a != d {
				out = append(out, a)
			}
		}
		return out
	})
}

// changeAllow enregistre les exceptions, régénère les règles et les applique.
func (s *ShieldService) changeAllow(label string, change func([]string) []string) SystemResult {
	s.adMu.Lock()
	defer s.adMu.Unlock()
	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}
	var allow []string
	var enabled bool
	err := s.prefs.Update(func(p *prefs.Prefs) {
		p.Allow = blocklist.CleanAllow(change(blocklist.CleanAllow(p.Allow)))
		allow, enabled = p.Allow, p.BlockAds
	})
	if err != nil {
		res.Error = "exception non enregistrée : " + err.Error()
		return res
	}
	add(label, true, "")
	info, err := s.ads.Rebuild(allow)
	if err != nil {
		// Pas encore de liste : l'exception servira au premier téléchargement.
		return res
	}
	add("Règles régénérées", true, countNote(info))
	s.applyBlocklist(enabled, add, &res)
	return res
}

// applyBlocklist dit à Unbound de charger (ou non) la liste, et le redémarre
// s'il tourne. s.mu est tenu pendant le redémarrage : ni activation ni bascule
// de la protection au même moment.
func (s *ShieldService) applyBlocklist(on bool, add func(string, bool, string), res *SystemResult) {
	path := ""
	if on {
		path = s.ads.ConfPath()
	}
	s.dns.SetInclude(path)
	if !s.dns.Running() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dns.Stop()
	_, err := s.dns.Start()
	if err == nil {
		add("Résolveur redémarré", true, "")
		return
	}
	if path != "" {
		// Unbound ne démarre plus avec la liste : on la retire plutôt que de
		// laisser Windows sans résolveur.
		s.dns.SetInclude("")
		if _, err2 := s.dns.Start(); err2 == nil {
			s.prefs.Update(func(p *prefs.Prefs) { p.BlockAds = false })
			add("Résolveur redémarré sans la liste", false, err.Error())
			res.Error = "Unbound a refusé la liste : blocage coupé, résolveur relancé sans elle"
			return
		}
	}
	add("Redémarrage du résolveur", false, err.Error())
	res.Error = "le résolveur n'a pas redémarré : " + err.Error()
	if s.tun.Stats().Full {
		s.tun.Down()
	}
	if s.sys.Enabled() {
		res.Steps = append(res.Steps, s.sys.Disable("Le résolveur n'a pas redémarré : ton DNS d'origine a été remis.")...)
	}
	s.alert("DNS d'origine remis", "Le résolveur Unbound n'a pas redémarré après le changement de liste de blocage.")
}

// refreshBlocklist met la liste à jour une fois par semaine, au lancement,
// sans rien dire si ça échoue : la liste installée continue de servir.
func (s *ShieldService) refreshBlocklist() {
	time.Sleep(refreshDelay)
	if s.quitting.Load() {
		return
	}
	s.adMu.Lock()
	defer s.adMu.Unlock()
	p := s.prefs.Load()
	if !p.BlockAds || !s.ads.Stale() {
		return
	}
	info, err := s.ads.Update(context.Background(), p.Allow)
	if err != nil {
		s.logf("mise à jour hebdomadaire de la liste de blocage : échec : %v", err)
		return
	}
	var res SystemResult
	add := func(label string, ok bool, note string) {
		res.Steps = append(res.Steps, Step{Label: label, OK: ok, Note: note})
	}
	add("Liste "+blocklist.SourceName+" téléchargée et filtrée", true, countNote(info))
	s.applyBlocklist(true, add, &res)
	s.logResult("Mise à jour hebdomadaire de la liste de blocage", res.Steps, res.Error)
}

// countNote : « 60 312 domaines », pour le journal.
func countNote(info blocklist.Info) string {
	n := strconv.Itoa(info.Count)
	for i := len(n) - 3; i > 0; i -= 3 {
		n = n[:i] + " " + n[i:]
	}
	return n + " domaines"
}
