// Package sysdns fait pointer le DNS de Windows sur le résolveur local d'Aria,
// et garantit le retour à la configuration d'origine quoi qu'il arrive.
package sysdns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	backupName    = "dns-backup.json"
	watchInterval = 10 * time.Second
	loopback4     = "127.0.0.1"
	loopback6     = "::1"
)

// Adapter est la photo d'une carte réseau avant modification.
type Adapter struct {
	Index    int      `json:"index"`
	Alias    string   `json:"alias"`
	GUID     string   `json:"guid"`
	StaticV4 []string `json:"staticV4"` // vide = DNS automatique (DHCP)
	StaticV6 []string `json:"staticV6"`
	Servers  []string `json:"servers"` // serveurs effectifs au moment de la photo
}

// Backup est tout ce qu'il faut pour revenir en arrière. Il est écrit sur disque
// AVANT toute modification, et supprimé seulement après une restauration réussie.
type Backup struct {
	Created       time.Time `json:"created"`
	IPv6          bool      `json:"ipv6"`
	Adapters      []Adapter `json:"adapters"`
	PolicyExisted bool      `json:"policyExisted"`
	PolicyValue   uint32    `json:"policyValue"`
}

// Step est une étape affichée dans le journal de l'interface.
type Step struct {
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	Note  string `json:"note"`
}

// AdapterView est l'état actuel d'une carte, pour l'affichage.
type AdapterView struct {
	Alias   string   `json:"alias"`
	Servers []string `json:"servers"`
	OnAria  bool     `json:"onAria"`
}

// Manager gère la bascule et la restauration.
type Manager struct {
	mu       sync.Mutex
	path     string
	enabled  bool
	backup   *Backup
	stop     chan struct{}
	event    string
	excluded map[string]bool
}

// Exclude écarte une carte (par son nom) : Aria configure elle-même sa carte VPN.
func (m *Manager) Exclude(alias string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.excluded == nil {
		m.excluded = map[string]bool{}
	}
	m.excluded[alias] = true
}

// filterLocked retire les cartes qu'Aria gère elle-même (m.mu doit être tenu).
func (m *Manager) filterLocked(all []Adapter) []Adapter {
	out := make([]Adapter, 0, len(all))
	for _, a := range all {
		if !m.excluded[a.Alias] {
			out = append(out, a)
		}
	}
	return out
}

// New prépare le gestionnaire ; la photo est rangée dans dataDir.
func New(dataDir string) *Manager {
	return &Manager{path: filepath.Join(dataDir, backupName)}
}

// Enabled indique si Windows utilise actuellement le résolveur d'Aria.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// LastEvent renvoie le dernier événement notable (restauration automatique…).
func (m *Manager) LastEvent() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.event
}

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.backup, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

func allowed(server string, ipv6 bool) bool {
	return server == loopback4 || (ipv6 && server == loopback6)
}

// onAria indique si une carte n'utilise que le résolveur local.
func onAria(servers []string, ipv6 bool) bool {
	if len(servers) == 0 {
		return false
	}
	for _, s := range servers {
		if !allowed(s, ipv6) {
			return false
		}
	}
	return true
}

// Enable photographie la config actuelle puis fait pointer toutes les cartes
// actives sur le résolveur local.
func (m *Manager) Enable(ipv6 bool) ([]Step, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var steps []Step
	add := func(label string, ok bool, note string) { steps = append(steps, Step{label, ok, note}) }

	if m.enabled {
		add("Windows utilise déjà le résolveur", true, "")
		return steps, nil
	}

	adapters, err := listAdapters()
	if err != nil {
		add("Lecture des cartes réseau", false, err.Error())
		return steps, err
	}
	adapters = m.filterLocked(adapters)
	if len(adapters) == 0 {
		add("Lecture des cartes réseau", false, "aucune carte active")
		return steps, errors.New("aucune carte réseau active")
	}
	add("Lecture des cartes réseau", true, fmt.Sprintf("%d active(s)", len(adapters)))

	existed, value, err := smartResolutionPolicy()
	if err != nil {
		add("Lecture de la stratégie DNS", false, err.Error())
		return steps, err
	}
	m.backup = &Backup{Created: time.Now(), IPv6: ipv6, Adapters: adapters, PolicyExisted: existed, PolicyValue: value}
	if err := m.save(); err != nil {
		add("Photo de la config d'origine", false, err.Error())
		m.backup = nil
		return steps, fmt.Errorf("photo impossible, rien n'a été modifié : %w", err)
	}
	add("Photo de la config d'origine", true, backupName)

	if err := setSmartResolutionOff(); err != nil {
		add("Résolution multi-cartes désactivée", false, err.Error())
	} else {
		add("Résolution multi-cartes désactivée", true, "")
	}

	ok := 0
	for _, a := range adapters {
		if err := pointToLoopback(a.Index, ipv6); err != nil {
			add("Carte « "+a.Alias+" »", false, err.Error())
			continue
		}
		ok++
		add("Carte « "+a.Alias+" »", true, servers(ipv6))
	}
	if ok == 0 {
		rs := m.restoreLocked("échec de la bascule")
		steps = append(steps, rs...)
		return steps, errors.New("aucune carte n'a pu être basculée, config d'origine remise")
	}

	flushCache()
	add("Cache DNS de Windows vidé", true, "")

	m.enabled = true
	m.event = ""
	m.stop = make(chan struct{})
	go m.watch(m.stop)
	return steps, nil
}

func servers(ipv6 bool) string {
	if ipv6 {
		return loopback4 + ", " + loopback6
	}
	return loopback4
}

// Disable remet la configuration d'origine.
func (m *Manager) Disable(reason string) []Step {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restoreLocked(reason)
}

func (m *Manager) restoreLocked(reason string) []Step {
	var steps []Step
	add := func(label string, ok bool, note string) { steps = append(steps, Step{label, ok, note}) }

	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	if m.backup == nil {
		m.enabled = false
		add("Rien à restaurer", true, "")
		return steps
	}

	failed := 0
	for _, a := range m.backup.Adapters {
		note, err := restoreAdapter(a)
		if err != nil {
			failed++
			add("Carte « "+a.Alias+" »", false, err.Error())
			continue
		}
		add("Carte « "+a.Alias+" »", true, note)
	}
	if err := restoreSmartResolution(m.backup.PolicyExisted, m.backup.PolicyValue); err != nil {
		failed++
		add("Stratégie multi-cartes", false, err.Error())
	} else {
		add("Stratégie multi-cartes", true, "remise comme avant")
	}
	flushCache()
	add("Cache DNS de Windows vidé", true, "")

	// La photo n'est effacée que si tout est revenu en place.
	if failed == 0 {
		os.Remove(m.path)
		m.backup = nil
	}
	m.enabled = false
	if reason != "" {
		m.event = reason
	}
	return steps
}

// RecoverLeftover restaure une photo laissée par une session qui a planté.
func (m *Manager) RecoverLeftover() (bool, []Step) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return false, nil
	}
	var b Backup
	if json.Unmarshal(data, &b) != nil || len(b.Adapters) == 0 {
		os.Remove(m.path)
		return false, nil
	}
	m.backup = &b
	steps := m.restoreLocked("Aria s'était fermée sans restaurer ton DNS : c'est corrigé.")
	return true, steps
}

// watch aligne les nouvelles cartes et corrige celles qui ont été modifiées.
func (m *Manager) watch(stop chan struct{}) {
	t := time.NewTicker(watchInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			m.realign(stop)
		}
	}
}

func (m *Manager) realign(stop chan struct{}) {
	adapters, err := listAdapters()
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled || m.stop != stop || m.backup == nil {
		return
	}
	adapters = m.filterLocked(adapters)
	known := map[int]bool{}
	for _, a := range m.backup.Adapters {
		known[a.Index] = true
	}
	changed := false
	for _, a := range adapters {
		if !known[a.Index] {
			// Nouvelle carte : on la photographie avant de la toucher.
			m.backup.Adapters = append(m.backup.Adapters, a)
			if m.save() != nil {
				m.backup.Adapters = m.backup.Adapters[:len(m.backup.Adapters)-1]
				continue
			}
			changed = true
		} else if onAria(a.Servers, m.backup.IPv6) {
			continue
		}
		if pointToLoopback(a.Index, m.backup.IPv6) == nil {
			changed = true
		}
	}
	if changed {
		flushCache()
	}
}

// Adapters renvoie l'état actuel des cartes actives.
func (m *Manager) Adapters() []AdapterView {
	adapters, err := listAdapters()
	if err != nil {
		return nil
	}
	m.mu.Lock()
	ipv6 := m.backup != nil && m.backup.IPv6
	adapters = m.filterLocked(adapters)
	m.mu.Unlock()
	views := make([]AdapterView, 0, len(adapters))
	for _, a := range adapters {
		views = append(views, AdapterView{Alias: a.Alias, Servers: a.Servers, OnAria: onAria(a.Servers, ipv6)})
	}
	return views
}

// CheckMarker demande à Windows (et non directement à Unbound) de résoudre le
// marqueur : seule notre config Unbound le connaît.
func CheckMarker(name, want string) (bool, string) {
	flushCache()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, name)
	if err != nil {
		return false, "Windows ne passe pas par le résolveur d'Aria (" + shortErr(err) + ")"
	}
	var got []string
	for _, ip := range ips {
		if ip.IP.String() == want {
			return true, "Windows passe bien par le résolveur d'Aria"
		}
		got = append(got, ip.IP.String())
	}
	return false, "réponse inattendue : " + strings.Join(got, ", ")
}

func shortErr(err error) string {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return "nom inconnu"
		}
		if dnsErr.IsTimeout {
			return "délai dépassé"
		}
	}
	return err.Error()
}
