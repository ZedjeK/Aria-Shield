package torbrowser

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// L'assistant Tor : le même exe qu'Aria, relancé avec les droits normaux de
// l'utilisateur (platform_windows.go) pour tout ce qui écrit dans le dossier de
// Tor Browser : vérification de la signature, téléchargement, installation,
// recopie du profil et échange des dossiers. Aria, en administrateur, ne fait
// que le lancer, l'attendre et lire son avancement.

// HelperArg, en premier argument, lance l'assistant au lieu de la fenêtre.
const HelperArg = "--assistant-tor"

const (
	opInstall = "installer"
	opUpdate  = "mettre-a-jour"

	// statusName : fichier d'avancement écrit par l'assistant, lu par Aria.
	statusName    = "assistant-tor.json"
	helperTimeout = 30 * time.Minute
)

// helperStatus est l'avancement publié par l'assistant. Le jeton, tiré au sort
// par Aria à chaque lancement, écarte un fichier resté d'une fois précédente.
type helperStatus struct {
	Nonce    string   `json:"jeton"`
	Progress Progress `json:"progression"`
	Error    string   `json:"erreur,omitempty"`
}

// RunHelper est le point d'entrée de l'assistant, appelé par main avant toute
// fenêtre. Renvoie le code de sortie : 0 réussite, 1 échec (message dans le
// fichier d'avancement), 2 arguments invalides.
func RunHelper(args []string) int {
	fs := flag.NewFlagSet(HelperArg, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	op := fs.String("op", "", "")
	dir := fs.String("dossier", "", "")
	nonce := fs.String("jeton", "", "")
	version := fs.String("version", "", "")
	if fs.Parse(args) != nil || (*op != opInstall && *op != opUpdate) || *dir == "" || *nonce == "" ||
		!filepath.IsAbs(*dir) || (*op == opUpdate && *version == "") {
		return 2
	}
	userDir := filepath.Clean(*dir)
	statusPath := filepath.Join(userDir, statusName)
	write := func(st helperStatus) {
		st.Nonce = *nonce
		data, err := json.Marshal(st)
		if err != nil {
			return
		}
		tmp := statusPath + ".tmp"
		if os.WriteFile(tmp, data, 0o644) == nil {
			_ = os.Rename(tmp, statusPath)
		}
	}

	m := New(userDir)
	m.report = func(p Progress) { write(helperStatus{Progress: p}) }
	if err := m.helperOp(*op, *version); err != nil {
		write(helperStatus{Error: err.Error()})
		return 1
	}
	write(helperStatus{})
	return 0
}

// helperOp fait l'opération dans le processus courant, avec un dossier de
// travail à soi, à côté de Tor Browser, effacé à la fin.
func (m *Manager) helperOp(op, version string) error {
	if err := os.MkdirAll(filepath.Dir(m.dir), 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(m.dir), ".tor-travail-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	switch op {
	case opInstall:
		return m.installHere(work)
	case opUpdate:
		return m.updateHere(work, version)
	}
	return fmt.Errorf("opération inconnue : %s", op)
}

// readStatus lit l'avancement publié par l'assistant (côté Aria). Le fichier
// est dans un dossier de l'utilisateur : lecture bornée, texte tronqué, et
// rien n'est retenu si le jeton ne correspond pas.
func readStatus(path, nonce string) (helperStatus, bool) {
	f, err := os.Open(path)
	if err != nil {
		return helperStatus{}, false
	}
	defer f.Close()
	var st helperStatus
	if json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&st) != nil || st.Nonce != nonce {
		return helperStatus{}, false
	}
	st.Error = truncate(st.Error, 500)
	st.Progress.Phase = truncate(st.Progress.Phase, 200)
	return st, true
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// newNonce tire un jeton au sort pour un lancement de l'assistant.
func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
