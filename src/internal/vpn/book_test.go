package vpn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testStore range le carnet en clair : DPAPI n'existe que sous Windows.
func testStore(t *testing.T) *Store {
	t.Helper()
	id := func(b []byte) ([]byte, error) { return b, nil }
	return &Store{dir: t.TempDir(), seal: id, open: id}
}

func mustProfile(t *testing.T, name string) Profile {
	t.Helper()
	in := validInput()
	in.Name = name
	p, err := in.Validate(Profile{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBookPutActiveDelete(t *testing.T) {
	s := testStore(t)
	if b, err := s.Load(); err != nil || len(b.Profiles) != 0 {
		t.Fatalf("carnet vide attendu : %v", err)
	}
	nl, err := s.Put(mustProfile(t, "Pays-Bas"))
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := s.Put(mustProfile(t, "Suisse"))
	dup, _ := s.Put(mustProfile(t, "pays-bas"))
	if dup.Name != "pays-bas (2)" {
		t.Fatalf("nom en double non renommé : %q", dup.Name)
	}
	if a, _ := s.Active(); a.ID != nl.ID {
		t.Fatal("le premier profil doit devenir actif")
	}
	if err := s.SetActive(ch.ID); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Load()
	for _, v := range b.Views() {
		if v.Active != (v.ID == ch.ID) {
			t.Fatalf("vue active incorrecte : %+v", v)
		}
	}
	if err := s.Delete(ch.ID); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Active(); a.ID != nl.ID {
		t.Fatal("après suppression de l'actif, le premier restant doit prendre sa place")
	}
	if s.SetActive("inconnu") != ErrNotFound || s.Delete("inconnu") != ErrNotFound {
		t.Fatal("identifiant inconnu non signalé")
	}
	// Modification : même ID, nom gardé même s'il est identique à lui-même.
	nl.Endpoint = "203.0.113.20:51820"
	if p, err := s.Put(nl); err != nil || p.Name != "Pays-Bas" {
		t.Fatalf("modification ratée : %v %q", err, p.Name)
	}
}

func TestBookMigrationDepuisV08(t *testing.T) {
	s := testStore(t)
	old := mustProfile(t, "Proton NL-FREE")
	data, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(s.dir, legacyFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := s.Load()
	if err != nil || len(b.Profiles) != 1 || b.Active == "" || b.Current().PrivateKey != testPriv {
		t.Fatalf("migration ratée : %v %+v", err, b)
	}
	if _, err := os.Stat(filepath.Join(s.dir, legacyFile)); !os.IsNotExist(err) {
		t.Fatal("l'ancien fichier doit être effacé après migration")
	}
	if _, err := os.Stat(filepath.Join(s.dir, booksFile)); err != nil {
		t.Fatal("le carnet doit être écrit")
	}
}

func TestBookLimite(t *testing.T) {
	s := testStore(t)
	for i := 0; i < MaxProfiles; i++ {
		if _, err := s.Put(mustProfile(t, "P")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Put(mustProfile(t, "P")); err == nil {
		t.Fatal("la limite de profils doit être respectée")
	}
}
