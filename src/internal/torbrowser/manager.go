package torbrowser

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tor Browser portable officiel, embarqué tel quel avec sa signature.
//
//go:embed assets/tor-browser-portable.exe
var embeddedInstaller []byte

//go:embed assets/tor-browser-portable.exe.asc
var embeddedSig []byte

// EmbeddedVersion est la version de Tor Browser contenue dans l'exe.
const EmbeddedVersion = "15.0.24"

// Index officiel des versions publiées.
const indexURL = "https://aus1.torproject.org/torbrowser/update_3/release/downloads.json"

const dirName = "TorBrowser"

// Progress décrit l'opération en cours, pour la barre de progression.
type Progress struct {
	Active bool   `json:"active"`
	Phase  string `json:"phase"`
	Done   int64  `json:"done"`
	Total  int64  `json:"total"`
}

// State est l'état de Tor Browser affiché dans l'interface.
type State struct {
	Installed bool     `json:"installed"`
	Version   string   `json:"version"`
	Running   bool     `json:"running"`
	Dir       string   `json:"dir"`
	Progress  Progress `json:"progress"`
}

// UpdateInfo est le résultat de la vérification des mises à jour.
type UpdateInfo struct {
	Checked   bool   `json:"checked"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Error     string `json:"error"`
	binURL    string
	sigURL    string
}

// Manager pilote le dossier Tor Browser d'Aria.
type Manager struct {
	dir  string     // Tor Browser, dans les données de l'utilisateur
	work string     // installeurs téléchargés, exécutés en admin : dossier réservé aux administrateurs
	mu   sync.Mutex // une seule installation ou mise à jour à la fois

	pmu      sync.Mutex
	progress Progress
}

// New prépare le gestionnaire : Tor Browser vivra dans userDir\TorBrowser, et les
// installeurs (lancés en administrateur) passeront par workDir, réservé aux admins.
func New(userDir, workDir string) *Manager {
	return &Manager{dir: filepath.Join(userDir, dirName), work: workDir}
}

func (m *Manager) setProgress(phase string, done, total int64) {
	m.pmu.Lock()
	m.progress = Progress{Active: phase != "", Phase: phase, Done: done, Total: total}
	m.pmu.Unlock()
}

func (m *Manager) firefox() string { return filepath.Join(m.dir, "Browser", "firefox.exe") }

// installedVersion lit Browser\tbb_version.json.
func installedVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "Browser", "tbb_version.json"))
	if err != nil {
		return ""
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	return v.Version
}

// State renvoie l'état courant (lecture rapide, sans réseau).
func (m *Manager) State() State {
	m.pmu.Lock()
	p := m.progress
	m.pmu.Unlock()
	v := installedVersion(m.dir)
	return State{
		Installed: v != "",
		Version:   v,
		Running:   v != "" && running(m.dir),
		Dir:       m.dir,
		Progress:  p,
	}
}

// compareVersions renvoie -1, 0 ou 1 (« 15.0.9 » < « 15.0.24 »).
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// CheckUpdate interroge l'index officiel de Tor.
func (m *Manager) CheckUpdate() UpdateInfo {
	info := UpdateInfo{Current: installedVersion(m.dir)}
	if info.Current == "" {
		info.Current = EmbeddedVersion
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		info.Error = "index de Tor injoignable"
		return info
	}
	defer resp.Body.Close()
	var idx struct {
		Version   string `json:"version"`
		Downloads map[string]map[string]struct {
			Binary string `json:"binary"`
			Sig    string `json:"sig"`
		} `json:"downloads"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&idx) != nil {
		info.Error = "index de Tor illisible"
		return info
	}
	dl, ok := idx.Downloads["win64"]["ALL"]
	if !ok || idx.Version == "" {
		info.Error = "aucune version Windows dans l'index"
		return info
	}
	// Seuls les fichiers du site de distribution officiel sont acceptés.
	for _, u := range []string{dl.Binary, dl.Sig} {
		if !strings.HasPrefix(u, "https://dist.torproject.org/") && !strings.HasPrefix(u, "https://archive.torproject.org/") {
			info.Error = "adresse de téléchargement inattendue dans l'index"
			return info
		}
	}
	info.Checked = true
	info.Latest = idx.Version
	info.Available = compareVersions(idx.Version, info.Current) > 0
	info.binURL, info.sigURL = dl.Binary, dl.Sig
	return info
}

// EnsureInstalled installe la version embarquée si Tor Browser est absent.
func (m *Manager) EnsureInstalled() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if installedVersion(m.dir) != "" {
		return nil
	}
	defer m.setProgress("", 0, 0)

	if err := os.MkdirAll(filepath.Dir(m.dir), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(m.work, "tor-browser-setup.exe")
	m.setProgress("Vérification de la signature", 0, 0)
	if err := Verify(bytes.NewReader(embeddedInstaller), embeddedSig); err != nil {
		return fmt.Errorf("Tor Browser embarqué : %w", err)
	}
	if err := os.WriteFile(tmp, embeddedInstaller, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	return m.installFrom(tmp, embeddedSig, m.dir)
}

// installFrom revérifie l'installeur sur disque puis l'extrait dans target.
func (m *Manager) installFrom(installer string, sig []byte, target string) error {
	f, err := os.Open(installer)
	if err != nil {
		return err
	}
	m.setProgress("Vérification de la signature", 0, 0)
	err = Verify(f, sig)
	f.Close()
	if err != nil {
		return err
	}
	m.setProgress("Installation de Tor Browser", 0, 0)
	os.RemoveAll(target)
	if err := runInstaller(installer, target); err != nil {
		os.RemoveAll(target)
		return fmt.Errorf("installation échouée : %w", err)
	}
	if installedVersion(target) == "" {
		os.RemoveAll(target)
		return errors.New("installation incomplète : Tor Browser introuvable après extraction")
	}
	return nil
}

// download récupère url dans dst en mettant à jour la progression.
func (m *Manager) download(url, dst, phase string) error {
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			done += int64(n)
			m.setProgress(phase, done, total)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return f.Sync()
}

// Update télécharge, vérifie et installe la dernière version, en gardant
// favoris, réglages et ponts. L'ancienne version reste en place en cas d'échec.
func (m *Manager) Update(info UpdateInfo) error {
	if !info.Checked || info.binURL == "" {
		return errors.New("vérifie d'abord les mises à jour")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.setProgress("", 0, 0)

	if running(m.dir) {
		return errors.New("ferme Tor Browser avant la mise à jour")
	}
	base := m.work
	bin := filepath.Join(base, "tor-browser-update.exe")
	sigPath := bin + ".asc"
	defer os.Remove(bin)
	defer os.Remove(sigPath)

	if err := m.download(info.sigURL, sigPath, "Téléchargement de la signature"); err != nil {
		return fmt.Errorf("signature : %w", err)
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return err
	}
	if err := m.download(info.binURL, bin, "Téléchargement de Tor Browser "+info.Latest); err != nil {
		return fmt.Errorf("téléchargement : %w", err)
	}

	fresh := m.dir + ".new"
	if err := m.installFrom(bin, sig, fresh); err != nil {
		return err
	}

	m.setProgress("Conservation de tes réglages", 0, 0)
	if installedVersion(m.dir) != "" {
		if err := keepUserData(m.dir, fresh); err != nil {
			os.RemoveAll(fresh)
			return fmt.Errorf("réglages non copiés, mise à jour annulée : %w", err)
		}
	}

	old := m.dir + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(m.dir); err == nil {
		if err := os.Rename(m.dir, old); err != nil {
			os.RemoveAll(fresh)
			return fmt.Errorf("Tor Browser semble encore ouvert : %w", err)
		}
	}
	if err := os.Rename(fresh, m.dir); err != nil {
		os.Rename(old, m.dir) // on remet l'ancienne version
		return err
	}
	os.RemoveAll(old)
	return nil
}

// Fichiers fournis par Tor Browser lui-même : jamais repris de l'ancienne version.
var shippedData = map[string]bool{"geoip": true, "geoip6": true, "torrc-defaults": true}

// keepUserData recopie le dossier Data (profil, favoris, ponts) de l'ancienne version.
func keepUserData(oldDir, newDir string) error {
	src := filepath.Join(oldDir, "Browser", "TorBrowser", "Data")
	dst := filepath.Join(newDir, "Browser", "TorBrowser", "Data")
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if shippedData[info.Name()] || !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Launch démarre Tor Browser avec les droits normaux de l'utilisateur.
func (m *Manager) Launch() error {
	if installedVersion(m.dir) == "" {
		return errors.New("Tor Browser n'est pas installé")
	}
	if running(m.dir) {
		return errors.New("Tor Browser est déjà ouvert")
	}
	return launchUnelevated(m.firefox(), filepath.Dir(m.firefox()))
}
