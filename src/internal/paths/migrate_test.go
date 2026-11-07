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

func TestUserDirMovesOldFolder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "profil"))
	t.Setenv("USERPROFILE", filepath.Join(root, "profil"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	os.MkdirAll(filepath.Join(root, "profil"), 0o755)
	old := filepath.Join(root, "local", appDir)
	os.MkdirAll(filepath.Join(old, "TorBrowser"), 0o755)
	os.WriteFile(filepath.Join(old, "vpn.json"), []byte("carnet"), 0o600)

	dir, moved, err := UserDir()
	if err != nil || !moved {
		t.Fatalf("UserDir = %q, %v, %v", dir, moved, err)
	}
	if dir != filepath.Join(root, "profil", homeDir) {
		t.Errorf("dossier = %s", dir)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "vpn.json")); string(b) != "carnet" {
		t.Error("carnet VPN non déplacé")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("l'ancien dossier doit avoir disparu")
	}
	// Deuxième lancement : rien à déplacer.
	if _, moved, err := UserDir(); err != nil || moved {
		t.Errorf("second lancement : moved=%v err=%v", moved, err)
	}
}

func TestUserDirKeepsExistingNewFolder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "profil"))
	t.Setenv("USERPROFILE", filepath.Join(root, "profil"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	os.MkdirAll(filepath.Join(root, "profil", homeDir), 0o755)
	os.WriteFile(filepath.Join(root, "profil", homeDir, "vpn.json"), []byte("récent"), 0o600)
	os.MkdirAll(filepath.Join(root, "local", appDir), 0o755)
	os.WriteFile(filepath.Join(root, "local", appDir, "vpn.json"), []byte("ancien"), 0o600)

	dir, moved, err := UserDir()
	if err != nil || moved {
		t.Fatalf("moved=%v err=%v", moved, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "vpn.json")); string(b) != "récent" {
		t.Error("le dossier .Aria-Shield existant ne doit pas être écrasé")
	}
}

func TestUserDirRefusesLink(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "profil"))
	t.Setenv("USERPROFILE", filepath.Join(root, "profil"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	os.MkdirAll(filepath.Join(root, "profil"), 0o755)
	os.MkdirAll(filepath.Join(root, "ailleurs"), 0o755)
	if err := os.Symlink(filepath.Join(root, "ailleurs"), filepath.Join(root, "profil", homeDir)); err != nil {
		t.Skip("liens symboliques indisponibles :", err)
	}
	dir, _, err := UserDir()
	if err == nil {
		t.Fatal("un .Aria-Shield qui est un lien doit être refusé")
	}
	if dir != filepath.Join(root, "local", appDir) {
		t.Errorf("repli attendu sur l'ancien dossier, obtenu %s", dir)
	}
}
