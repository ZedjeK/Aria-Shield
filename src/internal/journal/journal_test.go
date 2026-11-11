package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrintfAppends(t *testing.T) {
	dir := t.TempDir()
	j, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	j.now = func() time.Time { return time.Date(2026, 11, 11, 9, 5, 29, 0, time.Local) }
	j.Printf("démarrage version %s", "0.11.0")
	j.Printf("deux\nlignes\r\nen une")
	j.Close()
	j.Printf("après fermeture : ignoré")

	// Une réouverture ajoute à la suite, sans écraser.
	j2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	j2.Printf("deuxième lancement")
	j2.Close()

	lines := strings.Split(strings.TrimSpace(read(t, j.Path())), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lignes : %q", len(lines), lines)
	}
	if lines[0] != "2026-11-11 09:05:29 démarrage version 0.11.0" {
		t.Errorf("ligne 1 : %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "deux lignes en une") {
		t.Errorf("ligne 2 : %q", lines[1])
	}
	if !strings.HasSuffix(lines[2], "deuxième lancement") {
		t.Errorf("ligne 3 : %q", lines[2])
	}
}

func TestLongLineIsCapped(t *testing.T) {
	j, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j.Printf("%s", strings.Repeat("x", 3*maxLine))
	j.Close()
	if n := len(read(t, j.Path())); n > maxLine+100 {
		t.Fatalf("ligne de %d octets non bornée", n)
	}
}

func TestRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	os.WriteFile(path, []byte(strings.Repeat("ancien\n", maxSize/7+10)), 0o644)
	j, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	j.Printf("nouveau")
	j.Close()
	if got := read(t, path); !strings.HasSuffix(got, "nouveau\n") || strings.Contains(got, "ancien") {
		t.Fatalf("le journal n'est pas reparti de zéro (%d octets)", len(got))
	}
	if !strings.Contains(read(t, filepath.Join(dir, oldName)), "ancien") {
		t.Fatal("l'ancien journal n'a pas été archivé")
	}
}

func TestNilAndBrokenJournalAreSafe(t *testing.T) {
	var j *Journal
	j.Printf("rien")
	j.Sync()
	j.Close()
	if j.Path() != "" {
		t.Fatal("chemin d'un journal nil")
	}
	// Un dossier à la place du fichier : ouverture refusée, journal muet.
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, fileName), 0o755)
	broken, err := Open(dir)
	if err == nil {
		t.Fatal("un dossier a été accepté comme journal")
	}
	broken.Printf("ne plante pas")
	broken.Close()
}
