package tempdir

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate ramène les variables du dossier temporaire à leur valeur d'avant à la
// fin du test.
func isolate(t *testing.T) {
	t.Helper()
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(name, os.Getenv(name))
	}
}

func TestRedirect(t *testing.T) {
	isolate(t)
	data := t.TempDir()
	old := os.TempDir()

	previous, err := Redirect(data)
	if err != nil {
		t.Fatal(err)
	}
	if previous != old {
		t.Errorf("ancien dossier = %q, attendu %q", previous, old)
	}
	want := filepath.Join(data, "cache")
	if got := os.TempDir(); got != want {
		t.Fatalf("os.TempDir() = %q, attendu %q", got, want)
	}
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatalf("cache non créé : %v", err)
	}
	// Une deuxième fois : rien ne casse.
	if _, err := Redirect(data); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectRefusesLink(t *testing.T) {
	isolate(t)
	data := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(data, "cache")); err != nil {
		t.Skip("lien symbolique impossible ici : " + err.Error())
	}
	before := os.TempDir()
	if _, err := Redirect(data); err == nil {
		t.Fatal("un lien a été accepté comme dossier cache")
	}
	if os.TempDir() != before {
		t.Fatal("le dossier temporaire a changé malgré le refus")
	}
}

func TestRedirectWithoutDataDir(t *testing.T) {
	isolate(t)
	before := os.TempDir()
	if _, err := Redirect(""); err == nil {
		t.Fatal("dossier vide accepté")
	}
	if os.TempDir() != before {
		t.Fatal("le dossier temporaire a changé")
	}
}

func TestRemoveOld(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("Aria Shield{858af671-c949-4876-8302-ded7e9d1b0ec}.png")
	touch("Aria Shield{0F82E845-CB89-4039-BDBF-67CA33254C76}.png")
	touch("Autre appli{858af671}.png")
	touch("Aria Shield.txt")
	os.Mkdir(filepath.Join(dir, "Aria Shield{dossier}.png"), 0o755)

	if n := RemoveOld(dir, "Aria Shield{*}.png"); n != 2 {
		t.Fatalf("%d fichiers effacés, attendu 2", n)
	}
	for _, kept := range []string{"Autre appli{858af671}.png", "Aria Shield.txt", "Aria Shield{dossier}.png"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s effacé à tort", kept)
		}
	}
	if RemoveOld("", "*") != 0 {
		t.Error("dossier vide : rien ne doit partir")
	}
}
