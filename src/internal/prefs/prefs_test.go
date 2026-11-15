package prefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	s := New(t.TempDir())
	p := s.Load()
	if !p.Resume || p.Last != None || !p.Notify || !p.CheckUpdates {
		t.Fatalf("défauts inattendus : %+v", p)
	}
}

// Un reglages.json d'avant les notifications les garde activées.
func TestMissingFieldKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, fileName), []byte(`{"reprise":true,"derniere":"dns"}`), 0o644)
	if p := New(dir).Load(); !p.Notify || p.Last != DNS {
		t.Fatalf("relu : %+v", p)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.SetLast(Discret); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(p *Prefs) { p.Resume = false }); err != nil {
		t.Fatal(err)
	}
	p := New(dir).Load()
	if p.Resume || p.Last != Discret {
		t.Fatalf("relu : %+v", p)
	}
	// Aucun fichier temporaire ne doit traîner.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("fichiers restants : %v", entries)
	}
}

func TestUnknownValueIgnored(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, fileName), []byte(`{"reprise":true,"derniere":"powershell -c calc"}`), 0o644)
	if p := New(dir).Load(); p.Last != None {
		t.Fatalf("valeur inconnue acceptée : %q", p.Last)
	}
}

func TestOversizedOrBrokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	os.WriteFile(path, []byte(`{"reprise":false,"derniere":"dns"`+strings.Repeat(" ", maxSize)+`}`), 0o644)
	if p := New(dir).Load(); !p.Resume || p.Last != None {
		t.Fatalf("fichier trop gros pris en compte : %+v", p)
	}
	os.WriteFile(path, []byte(`{pas du json`), 0o644)
	if p := New(dir).Load(); !p.Resume || p.Last != None {
		t.Fatalf("fichier cassé pris en compte : %+v", p)
	}
}

func TestAllowListCapped(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	many := make([]string, maxAllow+50)
	for i := range many {
		many[i] = "exemple.fr"
	}
	if err := s.Update(func(p *Prefs) { p.BlockAds = true; p.Allow = many }); err != nil {
		t.Fatal(err)
	}
	p := New(dir).Load()
	if !p.BlockAds || len(p.Allow) != maxAllow {
		t.Fatalf("relu : blocage=%v, %d exceptions", p.BlockAds, len(p.Allow))
	}
}

func TestFondDefaults(t *testing.T) {
	f := New(t.TempDir()).Load().Fond
	if want := DefaultFond(); f != want {
		t.Fatalf("fond par défaut : %+v, attendu %+v", f, want)
	}
}

// Un reglages.json d'avant le fond garde les valeurs par défaut ; un fond
// partiel complète seulement ce qui manque.
func TestFondMissingOrPartial(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, fileName), []byte(`{"reprise":true,"derniere":"dns"}`), 0o644)
	if f := New(dir).Load().Fond; f != DefaultFond() {
		t.Fatalf("fond absent : %+v", f)
	}
	os.WriteFile(filepath.Join(dir, fileName), []byte(`{"fond":{"actif":false,"vitesse":"rapide"}}`), 0o644)
	f := New(dir).Load().Fond
	if f.Actif || f.Vitesse != Rapide || f.Minimum != DefaultFond().Minimum || f.Maximum != DefaultFond().Maximum {
		t.Fatalf("fond partiel : %+v", f)
	}
}

// Le fichier est modifiable sans droits admin : vitesse inconnue et nombres
// hors bornes sont ramenés à des valeurs permises.
func TestFondNormalize(t *testing.T) {
	cases := []struct {
		in, want Fond
	}{
		{Fond{true, "turbo", 10, 26}, Fond{true, Normale, 10, 26}},
		{Fond{true, Lente, -5, 9999}, Fond{true, Lente, FondMinLow, FondMaxHigh}},
		{Fond{false, Rapide, 999, 999}, Fond{false, Rapide, FondMinHigh, FondMaxHigh}},
		{Fond{true, Normale, 30, 12}, Fond{true, Normale, 30, 30}},
		{Fond{true, Normale, 0, 0}, Fond{true, Normale, FondMinLow, FondMaxLow}},
	}
	for _, c := range cases {
		if got := c.in.Normalize(); got != c.want {
			t.Errorf("Normalize(%+v) = %+v, attendu %+v", c.in, got, c.want)
		}
	}
}

func TestFondRoundTripAndLoadNormalizes(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	want := Fond{Actif: false, Vitesse: Lente, Minimum: 6, Maximum: 40}
	if err := s.Update(func(p *Prefs) { p.Fond = want }); err != nil {
		t.Fatal(err)
	}
	if got := New(dir).Load().Fond; got != want {
		t.Fatalf("relu : %+v", got)
	}
	os.WriteFile(filepath.Join(dir, fileName), []byte(`{"fond":{"actif":true,"vitesse":"powershell","minimum":-1,"maximum":1000000}}`), 0o644)
	if got := New(dir).Load().Fond; got != (Fond{true, Normale, FondMinLow, FondMaxHigh}) {
		t.Fatalf("valeurs hors bornes acceptées : %+v", got)
	}
}
