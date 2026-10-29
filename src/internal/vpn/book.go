package vpn

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	booksFile  = "vpn-profiles.bin"
	legacyFile = "vpn-profile.bin" // un seul profil, jusqu'à la v0.8
	// MaxProfiles borne le carnet : au-delà, l'interface deviendrait illisible.
	MaxProfiles = 20
)

// ErrNotFound signale un identifiant de profil inconnu.
var ErrNotFound = errors.New("profil introuvable")

// Book est le carnet de profils VPN, chiffré d'un bloc sur le disque.
type Book struct {
	Active   string    `json:"active"`
	Profiles []Profile `json:"profiles"`
}

// Get renvoie le profil d'identifiant id.
func (b Book) Get(id string) (Profile, bool) {
	for _, p := range b.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Current renvoie le profil actif (vide s'il n'y en a pas).
func (b Book) Current() Profile {
	p, _ := b.Get(b.Active)
	return p
}

// Views renvoie la liste affichable, sans aucune clé privée.
func (b Book) Views() []View {
	out := make([]View, 0, len(b.Profiles))
	for _, p := range b.Profiles {
		v := p.View()
		v.Active = p.ID == b.Active
		out = append(out, v)
	}
	return out
}

// uniqueName ajoute « (2) », « (3) »… si le nom est déjà pris par un autre profil.
func (b Book) uniqueName(name, selfID string) string {
	taken := func(n string) bool {
		for _, p := range b.Profiles {
			if p.ID != selfID && strings.EqualFold(p.Name, n) {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s (%d)", name, i)
		if !taken(n) {
			return n
		}
	}
}

func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand ne doit pas échouer
	}
	return hex.EncodeToString(b)
}

// Store range le carnet chiffré (DPAPI) dans le dossier de l'utilisateur.
type Store struct {
	dir  string
	seal func([]byte) ([]byte, error)
	open func([]byte) ([]byte, error)
}

// NewStore prépare le stockage des profils dans dataDir.
func NewStore(dataDir string) *Store {
	return &Store{dir: dataDir, seal: protect, open: unprotect}
}

func (s *Store) path() string { return filepath.Join(s.dir, booksFile) }

func (s *Store) read(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	plain, err := s.open(data)
	if err != nil {
		return nil, fmt.Errorf("profils illisibles (créés par un autre compte Windows ?) : %w", err)
	}
	return plain, nil
}

// Load lit le carnet. Un ancien profil unique (v0.8 et avant) est repris
// comme premier profil, puis l'ancien fichier est effacé.
func (s *Store) Load() (Book, error) {
	plain, err := s.read(booksFile)
	if err == nil {
		var b Book
		if err := json.Unmarshal(plain, &b); err != nil {
			return Book{}, err
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Book{}, err
	}

	plain, err = s.read(legacyFile)
	if errors.Is(err, os.ErrNotExist) {
		return Book{}, nil
	}
	if err != nil {
		return Book{}, err
	}
	var p Profile
	if err := json.Unmarshal(plain, &p); err != nil {
		return Book{}, err
	}
	if p.PrivateKey == "" {
		return Book{}, nil
	}
	p.ID = newID()
	b := Book{Active: p.ID, Profiles: []Profile{p}}
	if err := s.save(b); err != nil {
		return b, nil // migration réessayée au prochain lancement
	}
	os.Remove(filepath.Join(s.dir, legacyFile))
	return b, nil
}

func (s *Store) save(b Book) error {
	plain, err := json.Marshal(b)
	if err != nil {
		return err
	}
	data, err := s.seal(plain)
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

// Active renvoie le profil actif (vide si aucun).
func (s *Store) Active() (Profile, error) {
	b, err := s.Load()
	return b.Current(), err
}

// Put ajoute (ID vide) ou remplace un profil. Le premier profil devient actif.
func (s *Store) Put(p Profile) (Profile, error) {
	b, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	p.Name = b.uniqueName(p.Name, p.ID)
	if p.ID == "" {
		if len(b.Profiles) >= MaxProfiles {
			return Profile{}, fmt.Errorf("%d profils au maximum : supprimes-en un d'abord", MaxProfiles)
		}
		p.ID = newID()
		b.Profiles = append(b.Profiles, p)
	} else {
		found := false
		for i := range b.Profiles {
			if b.Profiles[i].ID == p.ID {
				b.Profiles[i], found = p, true
			}
		}
		if !found {
			return Profile{}, ErrNotFound
		}
	}
	if b.Current().ID == "" {
		b.Active = p.ID
	}
	return p, s.save(b)
}

// Delete efface un profil ; si c'était l'actif, le premier restant prend sa place.
func (s *Store) Delete(id string) error {
	b, err := s.Load()
	if err != nil {
		return err
	}
	kept := b.Profiles[:0]
	found := false
	for _, p := range b.Profiles {
		if p.ID == id {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return ErrNotFound
	}
	b.Profiles = kept
	if b.Active == id {
		b.Active = ""
		if len(kept) > 0 {
			b.Active = kept[0].ID
		}
	}
	return s.save(b)
}

// SetActive choisit le profil utilisé par la protection.
func (s *Store) SetActive(id string) error {
	b, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := b.Get(id); !ok {
		return ErrNotFound
	}
	b.Active = id
	return s.save(b)
}
