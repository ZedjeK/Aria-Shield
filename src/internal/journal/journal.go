// Package journal écrit le journal d'Aria, aria.log, dans le dossier
// .Aria-Shield de l'utilisateur : ce qu'Aria a fait, quand, et ce qui a échoué.
// Sans lui, impossible de savoir après coup si une sortie, une réactivation ou
// un arrêt de Windows s'est bien passé.
//
// Le fichier vit dans un dossier modifiable sans droits admin, alors qu'Aria y
// écrit en administrateur. Il est donc ouvert sans jamais suivre un lien
// (open_windows.go) : un programme sans droits ne peut pas faire écrire Aria
// dans un autre fichier du disque.
//
// Rien de secret n'y est écrit : ni clé VPN, ni contenu de profil. Les étapes
// journalisées sont celles affichées dans l'interface, où l'IP de la box est
// déjà masquée.
package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	fileName = "aria.log"
	oldName  = "aria.log.1"
	// maxSize : au-delà, au lancement, le journal est archivé (un seul
	// précédent gardé) et repart de zéro.
	maxSize = 1 << 20
	// maxLine borne une ligne, pour qu'une erreur bavarde ne gonfle pas le fichier.
	maxLine = 2000
)

// Journal écrit des lignes horodatées. Un Journal nil ou sans fichier est
// utilisable : il n'écrit simplement rien.
type Journal struct {
	mu   sync.Mutex
	file *os.File
	path string
	now  func() time.Time
}

// Open ouvre (ou crée) le journal dans dir. En cas d'échec, le journal renvoyé
// n'écrit rien : le journal ne doit jamais empêcher Aria de fonctionner.
func Open(dir string) (*Journal, error) {
	j := &Journal{path: filepath.Join(dir, fileName), now: time.Now}
	if dir == "" {
		return j, fmt.Errorf("pas de dossier pour le journal")
	}
	if st, err := os.Lstat(j.path); err == nil && st.Mode().IsRegular() && st.Size() > maxSize {
		old := filepath.Join(dir, oldName)
		os.Remove(old)
		os.Rename(j.path, old)
	}
	f, err := openNoFollow(j.path)
	if err != nil {
		return j, err
	}
	j.file = f
	return j, nil
}

// Path renvoie l'emplacement du journal.
func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// Printf écrit une ligne horodatée.
func (j *Journal) Printf(format string, args ...any) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return
	}
	line := clean(fmt.Sprintf(format, args...))
	fmt.Fprintf(j.file, "%s %s\n", j.now().Format("2006-01-02 15:04:05"), line)
}

// Close vide le journal sur le disque et le ferme.
func (j *Journal) Close() {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file != nil {
		j.file.Sync()
		j.file.Close()
		j.file = nil
	}
}

// Sync force l'écriture sur le disque (avant une sortie brutale, par exemple
// l'arrêt de Windows).
func (j *Journal) Sync() {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file != nil {
		j.file.Sync()
	}
}

// clean tient le message sur une ligne et le borne.
func clean(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
	if len(s) > maxLine {
		s = s[:maxLine] + "…"
	}
	return s
}
