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
	if !p.Resume || p.Last != None {
		t.Fatalf("défauts inattendus : %+v", p)
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
