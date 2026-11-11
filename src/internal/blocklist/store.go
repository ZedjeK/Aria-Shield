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
	// SourceName et SourceURL : la liste utilisée. « small » vise la pub et les
	// traceurs avec peu de faux positifs. Format « domainswild2 » : un domaine
	// par ligne, sous-domaines compris.
	SourceName = "OISD small"
	SourceURL  = "https://small.oisd.nl/domainswild2"

	confName = "blocage.conf" // règles lues par Unbound
	listName = "blocage.txt"  // domaines retenus, pour régénérer sans retélécharger
	metaName = "blocage.json" // date et taille de la liste

	maxDownload = 20 << 20 // 20 Mo : la liste en fait environ 1,2
	maxMeta     = 64 << 10
	// MaxAge : au-delà, Aria retélécharge la liste au lancement.
	MaxAge = 7 * 24 * time.Hour
)

// Info décrit la liste installée.
type Info struct {
	Present bool      `json:"present"`
	Count   int       `json:"count"`   // domaines bloqués, exceptions déduites
	Updated time.Time `json:"updated"` // dernier téléchargement réussi
	Source  string    `json:"source"`
}

// Store range la liste dans le dossier système verrouillé : Unbound, lancé en
// administrateur, ne lit que des fichiers qu'un programme sans droits ne peut
// pas modifier.
type Store struct {
	mu  sync.Mutex
	dir string
	// fetch télécharge la liste brute (remplacé dans les tests).
	fetch func(ctx context.Context) (io.ReadCloser, error)
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

// Stale indique si la liste est absente ou vieille de plus de MaxAge.
func (s *Store) Stale() bool {
	info := s.Info()
	return !info.Present || time.Since(info.Updated) > MaxAge
}

// Update télécharge la liste, n'en garde que les domaines valides et réécrit
// les fichiers. En cas d'échec, la liste déjà installée reste en place.
func (s *Store) Update(ctx context.Context, allow []string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return Info{}, fmt.Errorf("dossier système non verrouillé : la liste n'est pas installée")
	}
	body, err := s.fetch(ctx)
	if err != nil {
		return s.infoLocked(), fmt.Errorf("téléchargement de la liste : %w", err)
	}
	defer body.Close()
	domains, err := Parse(io.LimitReader(body, maxDownload))
	if err != nil {
		return s.infoLocked(), err
	}
	return s.writeLocked(domains, allow, time.Now())
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
	defer f.Close()
	domains, err := Parse(io.LimitReader(f, maxDownload))
	if err != nil {
		return s.infoLocked(), err
	}
	return s.writeLocked(domains, allow, s.infoLocked().Updated)
}

// writeLocked écrit la liste retenue, les règles puis la fiche, chacune par
// fichier temporaire puis renommage : jamais de fichier à moitié écrit.
func (s *Store) writeLocked(domains, allow []string, updated time.Time) (Info, error) {
	conf, n := Render(domains, allow)
	list := []byte{}
	for _, d := range domains {
		list = append(list, d...)
		list = append(list, '\n')
	}
	info := Info{Present: true, Count: n, Updated: updated, Source: SourceName}
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
func download(ctx context.Context) (io.ReadCloser, error) {
	client := &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) > 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SourceURL, nil)
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
