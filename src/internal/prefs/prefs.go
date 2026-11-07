// Package prefs garde les réglages d'Aria et la dernière protection choisie,
// dans reglages.json (dossier .Aria-Shield de l'utilisateur).
//
// Ce fichier vit dans un dossier modifiable sans droits admin, alors qu'Aria
// le lit et l'écrit en administrateur. D'où trois précautions :
//   - lecture bornée (64 Ko) et valeurs inconnues ramenées à « aucune » ;
//   - écriture dans un fichier temporaire au nom tiré au sort (créé en
//     exclusif, donc jamais à travers un lien posé d'avance), puis renommage,
//     qui remplace un éventuel lien au lieu de le suivre ;
//   - le pire qu'un programme puisse faire en modifiant ce fichier est de
//     demander à Aria d'activer ou non une protection au lancement.
package prefs

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	fileName = "reglages.json"
	maxSize  = 64 << 10
)

// Dernière protection active, réactivée au lancement suivant.
const (
	None    = "aucune"  // rien à réactiver
	DNS     = "dns"     // résolveur + DNS de Windows
	Discret = "discret" // résolveur, DNS de Windows, tunnel et kill switch
)

// Prefs est le contenu de reglages.json.
type Prefs struct {
	// Resume : réactiver au lancement la dernière protection choisie.
	Resume bool `json:"reprise"`
	// Last : la dernière protection choisie (None, DNS ou Discret).
	Last string `json:"derniere"`
	// Notify : notifications Windows (incidents, sortie d'Aria).
	Notify bool `json:"notifications"`
}

// defaults : reprise et notifications activées, rien à reprendre au premier lancement.
func defaults() Prefs { return Prefs{Resume: true, Last: None, Notify: true} }

// Store lit et écrit reglages.json.
type Store struct {
	mu   sync.Mutex
	path string
}

// New prépare le fichier de réglages dans dir.
func New(dir string) *Store {
	return &Store{path: filepath.Join(dir, fileName)}
}

// Load renvoie les réglages, ou les valeurs par défaut si le fichier est
// absent ou illisible.
func (s *Store) Load() Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() Prefs {
	p := defaults()
	f, err := os.Open(s.path)
	if err != nil {
		return p
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil || len(data) > maxSize {
		return defaults()
	}
	if json.Unmarshal(data, &p) != nil {
		return defaults()
	}
	switch p.Last {
	case None, DNS, Discret:
	default:
		p.Last = None
	}
	return p
}

// Update applique fn aux réglages puis les enregistre.
func (s *Store) Update(fn func(*Prefs)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.loadLocked()
	fn(&p)
	return s.saveLocked(p)
}

// SetLast retient la dernière protection choisie par l'utilisateur.
func (s *Store) SetLast(last string) error {
	return s.Update(func(p *Prefs) { p.Last = last })
}

func (s *Store) saveLocked(p Prefs) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "reglages-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
