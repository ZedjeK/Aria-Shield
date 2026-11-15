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
	maxAllow = 200 // exceptions au blocage des pubs
)

// Dernière protection active, réactivée au lancement suivant.
const (
	None    = "aucune"  // rien à réactiver
	DNS     = "dns"     // résolveur + DNS de Windows
	Discret = "discret" // résolveur, DNS de Windows, tunnel et kill switch
)

// Vitesses du fond animé.
const (
	Lente   = "lente"
	Normale = "normale"
	Rapide  = "rapide"
)

// Bornes du nombre de points du fond animé (curseurs de la page Réglages).
const (
	FondMinLow, FondMinHigh = 4, 40
	FondMaxLow, FondMaxHigh = 10, 80
)

// Fond : réglages du fond animé (clés fond.actif, fond.vitesse, fond.minimum,
// fond.maximum du contrat des applis Aria).
type Fond struct {
	// Actif : le fond animé est dessiné (sinon la zone principale reste unie).
	Actif bool `json:"actif"`
	// Vitesse : Lente, Normale ou Rapide.
	Vitesse string `json:"vitesse"`
	// Minimum et Maximum : nombre de points à l'écran, au plus bas et au plus haut.
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

// DefaultFond : le fond tourne, à vitesse normale, entre 10 et 26 points.
func DefaultFond() Fond { return Fond{Actif: true, Vitesse: Normale, Minimum: 10, Maximum: 26} }

// Normalize ramène le fond à des valeurs permises : vitesse inconnue = normale,
// nombres bornés, maximum jamais sous le minimum. Le fichier est modifiable
// sans droits admin : rien de ce qu'il contient n'est pris tel quel.
func (f Fond) Normalize() Fond {
	switch f.Vitesse {
	case Lente, Normale, Rapide:
	default:
		f.Vitesse = Normale
	}
	f.Minimum = clamp(f.Minimum, FondMinLow, FondMinHigh)
	f.Maximum = clamp(f.Maximum, FondMaxLow, FondMaxHigh)
	if f.Maximum < f.Minimum {
		f.Maximum = f.Minimum
	}
	return f
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Prefs est le contenu de reglages.json.
type Prefs struct {
	// Resume : réactiver au lancement la dernière protection choisie.
	Resume bool `json:"reprise"`
	// Last : la dernière protection choisie (None, DNS ou Discret).
	Last string `json:"derniere"`
	// Notify : notifications Windows (incidents, sortie d'Aria).
	Notify bool `json:"notifications"`
	// CheckUpdates : vérifier au lancement si une version plus récente est publiée.
	CheckUpdates bool `json:"verifier-versions"`
	// BlockAds : blocage des pubs et des traceurs par le résolveur.
	BlockAds bool `json:"blocage-pubs"`
	// List : liste de blocage choisie (« small » ou « big » ; autre valeur : small).
	List string `json:"liste-blocage,omitempty"`
	// Allow : domaines que l'utilisateur ne veut pas voir bloqués. Revalidés
	// par le paquet blocklist avant tout usage.
	Allow []string `json:"exceptions,omitempty"`
	// Fond : fond animé de la zone principale de la fenêtre.
	Fond Fond `json:"fond"`
}

// defaults : reprise et notifications activées, rien à reprendre au premier lancement.
func defaults() Prefs {
	return Prefs{Resume: true, Last: None, Notify: true, CheckUpdates: true, Fond: DefaultFond()}
}

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
	if len(p.Allow) > maxAllow {
		p.Allow = p.Allow[:maxAllow]
	}
	p.Fond = p.Fond.Normalize()
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
	p.Fond = p.Fond.Normalize()
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
