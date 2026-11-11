// Package blockstats compte les requêtes bloquées en lisant le journal
// d'Unbound.
//
// Avec « log-local-actions: yes » (ajouté par la liste de blocage), Unbound
// écrit une ligne par requête répondue par une zone locale :
//
//	… info: pub.exemple.net. always_null 127.0.0.1@53112 sous.pub.exemple.net. A IN
//
// Aria relit régulièrement la fin de ce fichier. C'est ce qui permet de
// compter sans rouvrir le canal de commande d'Unbound, fermé exprès : rien ne
// peut piloter le résolveur par ce biais, on ne fait que lire un fichier du
// dossier réservé aux administrateurs.
//
// Les noms comptés ne sont gardés qu'en mémoire, le temps qu'Aria tourne.
package blockstats

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"aria-shield/internal/blocklist"
)

const (
	marker = " always_null "
	// maxRead borne ce qui est lu à chaque passage.
	maxRead = 4 << 20
	// maxZones borne les domaines suivis ; au-delà, seul le total augmente.
	maxZones = 5000
)

// Entry est un domaine de la liste et le nombre de requêtes qu'il a bloquées.
type Entry struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// Stats est l'état du compteur, pour l'interface.
type Stats struct {
	Total int       `json:"total"`
	Since time.Time `json:"since"`
	Top   []Entry   `json:"top"`
}

// ParseLine renvoie le domaine de la liste qui a bloqué la requête décrite par
// line, ou ok faux si la ligne n'est pas un blocage. Le nom témoin d'Aria
// (page Tests) n'est pas compté.
func ParseLine(line string) (domain string, ok bool) {
	i := strings.Index(line, marker)
	if i < 0 {
		return "", false
	}
	head := strings.TrimRight(line[:i], " ")
	if j := strings.LastIndexByte(head, ' '); j >= 0 {
		head = head[j+1:]
	}
	d, valid := blocklist.ValidDomain(head)
	if !valid || d == blocklist.Canary {
		return "", false
	}
	return d, true
}

// Counter suit le journal d'Unbound et cumule les blocages.
type Counter struct {
	mu     sync.Mutex
	path   string
	offset int64
	total  int
	zones  map[string]int
	since  time.Time
}

// New prépare un compteur sur le journal path. Rien n'est lu avant Poll.
func New(path string) *Counter {
	return &Counter{path: path, zones: map[string]int{}, since: time.Now()}
}

// Size renvoie la taille actuelle du journal (0 s'il est absent).
func (c *Counter) Size() int64 {
	st, err := os.Stat(c.path)
	if err != nil {
		return 0
	}
	return st.Size()
}

// Poll lit ce qui a été ajouté au journal depuis le dernier passage. Un
// journal plus court qu'avant est un nouveau journal (Unbound a redémarré) : on
// le relit depuis le début, sans perdre le total.
func (c *Counter) Poll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.Open(c.path)
	if err != nil {
		c.offset = 0
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return
	}
	if st.Size() < c.offset {
		c.offset = 0
	}
	if st.Size() == c.offset {
		return
	}
	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		return
	}
	// Seules les lignes complètes sont comptées : la dernière peut être en
	// cours d'écriture.
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		if len(data) == maxRead {
			c.offset += int64(len(data)) // ligne démesurée : on la saute
		}
		return
	}
	c.offset += int64(end + 1)
	sc := bufio.NewScanner(bytes.NewReader(data[:end+1]))
	sc.Buffer(make([]byte, 0, 4096), 64<<10)
	for sc.Scan() {
		d, ok := ParseLine(sc.Text())
		if !ok {
			continue
		}
		c.total++
		if _, known := c.zones[d]; known || len(c.zones) < maxZones {
			c.zones[d]++
		}
	}
}

// Snapshot renvoie le total et les n domaines qui ont le plus bloqué.
func (c *Counter) Snapshot(n int) Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	top := make([]Entry, 0, len(c.zones))
	for d, k := range c.zones {
		top = append(top, Entry{Domain: d, Count: k})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].Count != top[j].Count {
			return top[i].Count > top[j].Count
		}
		return top[i].Domain < top[j].Domain
	})
	if len(top) > n {
		top = top[:n]
	}
	return Stats{Total: c.total, Since: c.since, Top: top}
}

// Total renvoie le nombre de requêtes bloquées depuis le lancement d'Aria.
func (c *Counter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}
