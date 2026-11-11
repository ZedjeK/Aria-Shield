package blocklist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// Small et Big : les deux listes proposées (voir Sources).
	Small = "small"
	Big   = "big"

	confName = "blocage.conf" // règles lues par Unbound
	listName = "blocage.txt"  // domaines retenus, pour régénérer sans retélécharger
	metaName = "blocage.json" // date et taille de la liste

	maxDownload = 20 << 20 // 20 Mo : la liste en fait environ 1,2
	maxMeta     = 64 << 10
	// MaxAge : au-delà, Aria retélécharge la liste au lancement.
	MaxAge = 7 * 24 * time.Hour
)

// Source décrit une liste proposée. Format « domainswild2 » : un domaine par
// ligne, sous-domaines compris.
type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Note string `json:"note"`
}

// Sources : les listes OISD. Les adresses sont fixées ici : l'utilisateur
// choisit entre elles, il ne peut pas en saisir une autre.
var Sources = []Source{
	{Small, "OISD small", "https://small.oisd.nl/domainswild2",
		"Pubs et traceurs, peu de faux positifs. Environ 60 000 domaines, 15 Mo de mémoire en plus."},
	{Big, "OISD big", "https://big.oisd.nl/domainswild2",
		"Bloque plus large (pubs, traceurs, hameçonnage, logiciels malveillants). Environ 225 000 domaines, 70 Mo de mémoire en plus, et plus de sites qui cassent."},
}

// SourceByID renvoie la liste demandée, ou « small » si l'identifiant est inconnu.
func SourceByID(id string) Source {
	for _, s := range Sources {
		if s.ID == id {
			return s
		}
	}
	return Sources[0]
}

// Info décrit la liste installée.
type Info struct {
	Present bool      `json:"present"`
	Count   int       `json:"count"`   // domaines bloqués, exceptions déduites
	Updated time.Time `json:"updated"` // dernier téléchargement réussi
	Source  string    `json:"source"`   // nom affiché
	ID      string    `json:"sourceId"` // Small ou Big
}

// Store range la liste dans le dossier système verrouillé : Unbound, lancé en
// administrateur, ne lit que des fichiers qu'un programme sans droits ne peut
// pas modifier.
type Store struct {
	mu  sync.Mutex
	dir string
	// fetch télécharge la liste brute (remplacé dans les tests).
	fetch func(ctx context.Context, url string) (io.ReadCloser, error)
}

// NewStore prépare le rangement dans dir ("" : dossier non verrouillé, tout échoue).
func NewStore(dir string) *Store {
	return &Store{dir: dir, fetch: download}
}

// ConfPath renvoie le fichier à inclure dans la config d'Unbound, ou "" s'il
// n'existe pas encore.
func (s *Store) ConfPath() string {
	if s.dir == "" {
		return ""
	}
	p := filepath.Join(s.dir, confName)
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		return ""
	}
	return p
}

// Info lit la date et la taille de la liste installée.
func (s *Store) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoLocked()
}

func (s *Store) infoLocked() Info {
	var info Info
	if s.dir == "" {
		return info
	}
	f, err := os.Open(filepath.Join(s.dir, metaName))
	if err != nil {
		return info
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMeta))
	if err != nil || json.Unmarshal(data, &info) != nil {
		return Info{}
	}
	info.Present = s.ConfPath() != ""
	return info
}

// Stale indique si la liste installée est absente, vieille de plus de MaxAge,
// ou n'est pas celle demandée (id).
func (s *Store) Stale(id string) bool {
	info := s.Info()
	return !info.Present || time.Since(info.Updated) > MaxAge || info.ID != SourceByID(id).ID
}

// Update télécharge la liste, n'en garde que les domaines valides et réécrit
// les fichiers. En cas d'échec, la liste déjà installée reste en place.
func (s *Store) Update(ctx context.Context, id string, allow []string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return Info{}, fmt.Errorf("dossier système non verrouillé : la liste n'est pas installée")
	}
	src := SourceByID(id)
	body, err := s.fetch(ctx, src.URL)
	if err != nil {
		return s.infoLocked(), fmt.Errorf("téléchargement de la liste : %w", err)
	}
	defer body.Close()
	domains, err := Parse(io.LimitReader(body, maxDownload))
	if err != nil {
		return s.infoLocked(), err
	}
	return s.writeLocked(domains, allow, time.Now(), src)
}

// Rebuild régénère les règles à partir de la liste déjà téléchargée, après un
// changement d'exceptions.
func (s *Store) Rebuild(allow []string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return Info{}, ErrEmpty
	}
	f, err := os.Open(filepath.Join(s.dir, listName))
	if err != nil {
		return s.infoLocked(), ErrEmpty
	}
	domains, err := Parse(io.LimitReader(f, maxDownload))
	// Fermé avant de réécrire : Windows refuse de remplacer un fichier ouvert.
	f.Close()
	if err != nil {
		return s.infoLocked(), err
	}
	old := s.infoLocked()
	return s.writeLocked(domains, allow, old.Updated, SourceByID(old.ID))
}

// writeLocked écrit la liste retenue, les règles puis la fiche, chacune par
// fichier temporaire puis renommage : jamais de fichier à moitié écrit.
func (s *Store) writeLocked(domains, allow []string, updated time.Time, src Source) (Info, error) {
	conf, n := Render(domains, allow)
	list := []byte{}
	for _, d := range domains {
		list = append(list, d...)
		list = append(list, '\n')
	}
	info := Info{Present: true, Count: n, Updated: updated, Source: src.Name, ID: src.ID}
	meta, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return s.infoLocked(), err
	}
	for _, f := range []struct {
		name string
		data []byte
	}{{listName, list}, {confName, conf}, {metaName, meta}} {
		if err := writeAtomic(filepath.Join(s.dir, f.name), f.data); err != nil {
			return s.infoLocked(), fmt.Errorf("écriture de %s : %w", f.name, err)
		}
	}
	return info, nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
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
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// download récupère la liste en HTTPS. Les redirections restent sur HTTPS.
func download(ctx context.Context, url string) (io.ReadCloser, error) {
	client := &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) > 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Aria-Shield")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("réponse %d du serveur", resp.StatusCode)
	}
	return resp.Body, nil
}
