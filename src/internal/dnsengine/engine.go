// Package dnsengine embarque Unbound (version portable officielle) et le fait
// tourner comme processus enfant d'Aria Shield : rien n'est installé sur la machine.
package dnsengine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Unbound 1.26.1 portable, zip officiel NLnet Labs vérifié par signature PGP
// (clé « NLnet Labs releases signing key G2 », 2310 1869 0C4D 903E F419 146A A144 323D EAAC DF45).
const (
	Version       = "1.26.1"
	unboundSHA256 = "8fd7f2d2ad66261ea09e345dd0ffcde7428c404f474fcf459ef6c3d05470fa24"
)

//go:embed assets/unbound.exe.gz
var unboundGz []byte

// Ancre DNSSEC racine (KSK 20326 et 38696), identique à la publication IANA.
//
//go:embed assets/root.key
var rootKey []byte

//go:embed unbound.conf.tmpl
var confTemplate string

// Marqueur servi uniquement par notre Unbound : si Windows le résout,
// c'est qu'il passe bien par le résolveur d'Aria.
const (
	MarkerName = "check.aria-shield.test"
	MarkerIP   = "192.0.2.53"
)

const (
	listenAddr   = "127.0.0.1:53"
	listenAddr6  = "[::1]:53"
	dataDirName  = "AriaShield-data"
	startTimeout = 12 * time.Second
	probeTimeout = 3 * time.Second
	testDomain   = "example.com"
	brokenDNSSEC = "dnssec-failed.org"
)

// Step est une étape affichée dans le journal de l'interface.
type Step struct {
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	Note  string `json:"note"`
}

// TestResult est le résultat des deux tests de validation.
type TestResult struct {
	ResolveOK bool   `json:"resolveOk"`
	DNSSECOK  bool   `json:"dnssecOk"`
	Detail    string `json:"detail"`
}

// Engine pilote le processus unbound.exe.
type Engine struct {
	mu       sync.Mutex
	dataDir  string
	cmd      *exec.Cmd
	done     chan struct{}
	ipv6     bool
	stopping bool
	onExit   func()
}

// OnUnexpectedExit enregistre une fonction appelée si Unbound s'arrête
// sans qu'on le lui ait demandé (plantage, processus tué…).
func (e *Engine) OnUnexpectedExit(f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onExit = f
}

// IPv6 indique si Unbound écoute aussi sur ::1.
func (e *Engine) IPv6() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ipv6 && e.runningLocked()
}

// New crée le moteur ; rien n'est lancé tant que Start n'est pas appelé.
func New() *Engine { return &Engine{} }

// DataDir renvoie le dossier de travail : à côté de l'exe si possible,
// sinon dans %LOCALAPPDATA%.
func (e *Engine) DataDir() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dataDirLocked()
}

func (e *Engine) dataDirLocked() string {
	if e.dataDir != "" {
		return e.dataDir
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), dataDirName))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "AriaShield"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "AriaShield"))

	for _, dir := range candidates {
		if writable(dir) {
			e.dataDir = dir
			return dir
		}
	}
	e.dataDir = candidates[len(candidates)-1]
	return e.dataDir
}

func writable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// Running indique si unbound.exe tourne.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runningLocked()
}

func (e *Engine) runningLocked() bool {
	if e.done == nil {
		return false
	}
	select {
	case <-e.done:
		return false
	default:
		return true
	}
}

// RenderConfig produit la config Unbound pour un dossier de travail donné.
// Avec ipv6, Unbound écoute aussi sur ::1 (sinon Windows passerait par l'IPv6 de la box)
// tout en préférant l'IPv4 pour ses propres requêtes.
func RenderConfig(dir string, ipv6 bool) string {
	v6 := "    do-ip6: no\n"
	if ipv6 {
		v6 = "    interface: ::1\n    access-control: ::1 allow\n    do-ip6: yes\n    prefer-ip4: yes\n"
	}
	conf := strings.ReplaceAll(confTemplate, "{{IPV6}}\n", v6)
	return strings.ReplaceAll(conf, "{{DIR}}", filepath.ToSlash(dir))
}

// ipv6Loopback vérifie que ::1 est utilisable sur cette machine.
func ipv6Loopback() bool {
	c, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// prepare extrait unbound.exe (si absent ou altéré), la clé racine et la config.
func (e *Engine) prepare(add func(string, bool, string)) (exePath, confPath string, err error) {
	dir := e.dataDirLocked()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		add("Dossier de travail", false, err.Error())
		return "", "", err
	}
	add("Dossier de travail", true, dir)

	exePath = filepath.Join(dir, "unbound.exe")
	if fileSHA256(exePath) != unboundSHA256 {
		if err := extractUnbound(exePath); err != nil {
			add("Extraction d'Unbound "+Version, false, err.Error())
			return "", "", err
		}
		add("Extraction d'Unbound "+Version, true, "empreinte vérifiée")
	} else {
		add("Extraction d'Unbound "+Version, true, "déjà présent et intact")
	}

	// Unbound met lui-même la clé à jour (RFC 5011) : on ne l'écrase jamais.
	keyPath := filepath.Join(dir, "root.key")
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(keyPath, rootKey, 0o644); err != nil {
			add("Clé racine DNSSEC", false, err.Error())
			return "", "", err
		}
		add("Clé racine DNSSEC", true, "installée")
	} else {
		add("Clé racine DNSSEC", true, "déjà présente")
	}

	e.ipv6 = ipv6Loopback()
	confPath = filepath.Join(dir, "unbound.conf")
	if err := os.WriteFile(confPath, []byte(RenderConfig(dir, e.ipv6)), 0o644); err != nil {
		add("Configuration", false, err.Error())
		return "", "", err
	}
	if e.ipv6 {
		add("Configuration", true, "écoute sur 127.0.0.1 et ::1")
	} else {
		add("Configuration", true, "écoute sur 127.0.0.1 (IPv6 indisponible)")
	}
	return exePath, confPath, nil
}

func extractUnbound(dst string) error {
	zr, err := gzip.NewReader(bytes.NewReader(unboundGz))
	if err != nil {
		return err
	}
	defer zr.Close()

	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), zr); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != unboundSHA256 {
		os.Remove(tmp)
		return fmt.Errorf("empreinte inattendue (%s)", got[:12])
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// portFree vérifie que personne n'écoute déjà sur le port 53 de la boucle locale.
func portFree(ipv6 bool) error {
	addrs := [][2]string{{"udp4", listenAddr}, {"tcp4", listenAddr}}
	if ipv6 {
		addrs = append(addrs, [2]string{"udp6", listenAddr6}, [2]string{"tcp6", listenAddr6})
	}
	for _, a := range addrs {
		if a[0][:3] == "udp" {
			c, err := net.ListenPacket(a[0], a[1])
			if err != nil {
				return err
			}
			c.Close()
		} else {
			l, err := net.Listen(a[0], a[1])
			if err != nil {
				return err
			}
			l.Close()
		}
	}
	return nil
}

// Start lance Unbound et attend qu'il réponde.
func (e *Engine) Start() ([]Step, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var steps []Step
	add := func(label string, ok bool, note string) { steps = append(steps, Step{label, ok, note}) }

	if e.runningLocked() {
		add("Unbound déjà actif", true, "")
		return steps, nil
	}

	exePath, confPath, err := e.prepare(add)
	if err != nil {
		return steps, err
	}

	if err := portFree(e.ipv6); err != nil {
		owner := portOwner()
		note := "occupé"
		if owner != "" {
			note = "occupé par " + owner
		}
		add("Port 53 disponible", false, note)
		return steps, fmt.Errorf("le port 53 est déjà utilisé (%s) : un autre service DNS tourne sur la machine", note)
	}
	add("Port 53 disponible", true, "")

	logPath := filepath.Join(e.dataDirLocked(), "unbound-console.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		add("Lancement d'Unbound", false, err.Error())
		return steps, err
	}
	// Repart d'un journal propre à chaque démarrage.
	os.Remove(filepath.Join(e.dataDirLocked(), "unbound.log"))

	cmd := exec.Command(exePath, "-d", "-c", confPath)
	cmd.Dir = e.dataDirLocked()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	prepareChild(cmd)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		add("Lancement d'Unbound", false, err.Error())
		return steps, err
	}
	// Unbound est rattaché à Aria : si Aria se ferme ou plante, Windows l'arrête aussi.
	if err := bindToParent(cmd); err != nil {
		add("Liaison au processus Aria", false, err.Error())
	}

	done := make(chan struct{})
	e.stopping = false
	go func() {
		cmd.Wait()
		logFile.Close()
		close(done)

		e.mu.Lock()
		unexpected := !e.stopping && e.cmd == cmd
		cb := e.onExit
		e.mu.Unlock()
		if unexpected && cb != nil {
			cb()
		}
	}()
	e.cmd, e.done = cmd, done

	// Attend que le port réponde, ou que le processus s'arrête en erreur.
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			e.stopping = true // échec de démarrage : pas un plantage à signaler
			reason := e.lastLogLine()
			add("Lancement d'Unbound", false, reason)
			return steps, fmt.Errorf("Unbound s'est arrêté au démarrage : %s", reason)
		default:
		}
		if _, err := Query(listenAddr, testDomain, probeTimeout); err == nil {
			add("Lancement d'Unbound", true, fmt.Sprintf("PID %d", cmd.Process.Pid))
			return steps, nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	add("Lancement d'Unbound", false, "aucune réponse après 12 s")
	e.stopLocked()
	return steps, errors.New("Unbound ne répond pas : vérifie ta connexion internet")
}

// lastLogLine renvoie la dernière ligne utile des journaux d'Unbound.
func (e *Engine) lastLogLine() string {
	for _, name := range []string{"unbound.log", "unbound-console.log"} {
		data, err := os.ReadFile(filepath.Join(e.dataDirLocked(), name))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if l := strings.TrimSpace(lines[i]); l != "" {
				return l
			}
		}
	}
	return "voir unbound.log dans le dossier de travail"
}

// Stop arrête Unbound s'il tourne.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

func (e *Engine) stopLocked() {
	if !e.runningLocked() {
		return
	}
	e.stopping = true
	_ = e.cmd.Process.Kill()
	select {
	case <-e.done:
	case <-time.After(5 * time.Second):
	}
}

// Test vérifie que le résolveur répond et qu'il valide bien DNSSEC.
func (e *Engine) Test() TestResult {
	var res TestResult
	if !e.Running() {
		res.Detail = "Unbound n'est pas démarré"
		return res
	}

	ans, err := Query(listenAddr, testDomain, probeTimeout)
	if err != nil {
		res.Detail = "Unbound ne répond pas sur 127.0.0.1 : " + err.Error()
		return res
	}
	res.ResolveOK = ans.Rcode == RcodeNoError && ans.Answers > 0
	if !res.ResolveOK {
		res.Detail = fmt.Sprintf("%s ne se résout pas (code %d)", testDomain, ans.Rcode)
		return res
	}

	bad, err := Query(listenAddr, brokenDNSSEC, probeTimeout*2)
	switch {
	case err != nil:
		res.Detail = "test DNSSEC impossible : " + err.Error()
	case bad.Rcode == RcodeServFail:
		res.DNSSECOK = true
		res.Detail = "Résolution OK, signature DNSSEC invalide bien rejetée"
	default:
		res.Detail = fmt.Sprintf("%s a été résolu alors que sa signature est cassée : DNSSEC n'est pas actif", brokenDNSSEC)
	}
	return res
}
