package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyDirName)
	user := filepath.Join(root, "user")
	sys := filepath.Join(root, "sys")
	for _, d := range []string{legacy, user, sys, filepath.Join(legacy, "TorBrowser", "Browser")} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(filepath.Join(legacy, "vpn-profile.bin"), []byte("profil"), 0o600)
	os.WriteFile(filepath.Join(legacy, "dns-backup.json"), []byte(`{"adapters":[]}`), 0o644)
	os.WriteFile(filepath.Join(legacy, "TorBrowser", "Browser", "tbb_version.json"), []byte(`{"version":"15.0.24"}`), 0o644)
	os.WriteFile(filepath.Join(legacy, "unbound.exe"), []byte("binaire"), 0o644)

	moved, err := Migrate(legacy, user, sys)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 3 {
		t.Fatalf("3 éléments attendus, déplacés : %v", moved)
	}
	for _, p := range []string{
		filepath.Join(user, "vpn-profile.bin"),
		filepath.Join(user, "TorBrowser", "Browser", "tbb_version.json"),
		filepath.Join(sys, "dns-backup.json"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s manquant après migration", p)
		}
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("l'ancien dossier doit être supprimé")
	}
	if _, err := os.Stat(filepath.Join(user, "unbound.exe")); err == nil {
		t.Error("unbound.exe ne doit pas être migré (il est réextrait dans le dossier système)")
	}
}

func TestMigrateKeepsExisting(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyDirName)
	user := filepath.Join(root, "user")
	os.MkdirAll(legacy, 0o755)
	os.MkdirAll(user, 0o755)
	os.WriteFile(filepath.Join(legacy, "vpn-profile.bin"), []byte("ancien"), 0o600)
	os.WriteFile(filepath.Join(user, "vpn-profile.bin"), []byte("récent"), 0o600)

	if _, err := Migrate(legacy, user, filepath.Join(root, "sys")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(user, "vpn-profile.bin")); string(b) != "récent" {
		t.Fatal("un profil déjà présent au nouvel emplacement ne doit pas être écrasé")
	}
}
