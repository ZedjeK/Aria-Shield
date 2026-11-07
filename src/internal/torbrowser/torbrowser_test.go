package torbrowser

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedInstallerSignature(t *testing.T) {
	if err := Verify(bytes.NewReader(embeddedInstaller), embeddedSig); err != nil {
		t.Fatalf("l'installeur embarqué doit être signé par les développeurs de Tor Browser : %v", err)
	}
}

func TestTamperedInstallerRejected(t *testing.T) {
	bad := append([]byte{}, embeddedInstaller[:1<<20]...)
	bad[1000] ^= 0xFF
	if err := Verify(bytes.NewReader(bad), embeddedSig); err == nil {
		t.Fatal("un fichier modifié doit être refusé")
	}
}

func TestWrongKeyRejected(t *testing.T) {
	if _, err := loadKeyring([]byte("pas une clé")); err == nil {
		t.Fatal("une clé illisible doit être refusée")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"15.0.24", "15.0.24", 0},
		{"15.0.9", "15.0.24", -1},
		{"15.1", "15.0.24", 1},
		{"16.0", "15.9.99", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, attendu %d", c.a, c.b, got, c.want)
		}
	}
}

func TestKeepUserData(t *testing.T) {
	oldDir, newDir := t.TempDir(), t.TempDir()
	write := func(base, rel, content string) {
		p := filepath.Join(base, "Browser", "TorBrowser", "Data", rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	write(oldDir, "Browser/profile.default/places.sqlite", "favoris")
	write(oldDir, "Tor/torrc", "Bridge obfs4 …")
	write(oldDir, "Tor/geoip", "ancienne base")
	write(newDir, "Tor/geoip", "nouvelle base")

	if err := keepUserData(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(newDir, "Browser", "TorBrowser", "Data", rel))
		return string(b)
	}
	if read("Browser/profile.default/places.sqlite") != "favoris" || read("Tor/torrc") != "Bridge obfs4 …" {
		t.Fatal("le profil et les ponts doivent être conservés")
	}
	if read("Tor/geoip") != "nouvelle base" {
		t.Fatal("les fichiers fournis par la nouvelle version ne doivent pas être écrasés")
	}
}

func TestInstalledVersion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Browser"), 0o755)
	os.WriteFile(filepath.Join(dir, "Browser", "tbb_version.json"),
		[]byte(`{"version":"15.0.24","architecture":"windows-x86_64","channel":"release"}`), 0o644)
	if v := installedVersion(dir); v != "15.0.24" {
		t.Fatalf("version lue : %q", v)
	}
}

func TestHelperRefusesBadArguments(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		nil,
		{"-op", "inconnue", "-dossier", dir, "-jeton", "j"},
		{"-op", opInstall, "-dossier", "relatif", "-jeton", "j"},
		{"-op", opInstall, "-dossier", dir},
		{"-op", opUpdate, "-dossier", dir, "-jeton", "j"}, // version manquante
	} {
		if code := RunHelper(args); code != 2 {
			t.Errorf("RunHelper(%q) = %d, attendu 2", args, code)
		}
	}
}

func TestHelperReportsFailureWithNonce(t *testing.T) {
	dir := t.TempDir()
	// Hors Windows, l'installeur ne peut pas tourner : l'assistant doit le dire
	// dans son fichier d'avancement, avec le bon jeton, et sortir en échec.
	if code := RunHelper([]string{"-op", opInstall, "-dossier", dir, "-jeton", "abc"}); code != 1 {
		t.Fatalf("code = %d, attendu 1", code)
	}
	st, ok := readStatus(filepath.Join(dir, statusName), "abc")
	if !ok || st.Error == "" {
		t.Fatalf("avancement = %+v (lu : %v), une erreur était attendue", st, ok)
	}
	if _, ok := readStatus(filepath.Join(dir, statusName), "autre-jeton"); ok {
		t.Error("un fichier d'un autre lancement ne doit pas être retenu")
	}
	// Le dossier de travail est effacé, même en cas d'échec.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tor-travail-") {
			t.Errorf("dossier de travail laissé : %s", e.Name())
		}
	}
}

func TestReadStatusTruncates(t *testing.T) {
	path := filepath.Join(t.TempDir(), statusName)
	long := strings.Repeat("x", 5000)
	data, _ := json.Marshal(helperStatus{Nonce: "j", Error: long, Progress: Progress{Phase: long}})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	st, ok := readStatus(path, "j")
	if !ok || len(st.Error) > 510 || len(st.Progress.Phase) > 210 {
		t.Errorf("texte non tronqué : erreur %d, phase %d", len(st.Error), len(st.Progress.Phase))
	}
}
