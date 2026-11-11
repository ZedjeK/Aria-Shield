package blocklist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeList fabrique une liste de n domaines valides, plus des lignes d'en-tête.
func fakeList(n int, extra ...string) string {
	var b strings.Builder
	b.WriteString("# Title: liste de test\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "pub%d.exemple-pub.net\n", i)
	}
	for _, e := range extra {
		b.WriteString(e + "\n")
	}
	return b.String()
}

func TestValidDomain(t *testing.T) {
	good := map[string]string{
		"Ads.Example.NET":  "ads.example.net",
		"*.tracker.io":     "tracker.io",
		"a_b.exemple.com.": "a_b.exemple.com",
	}
	for in, want := range good {
		if got, ok := ValidDomain(in); !ok || got != want {
			t.Errorf("ValidDomain(%q) = %q, %v ; attendu %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{
		"", "com", "localhost", "a..b.com", "-a.com", "a-.com", "exa mple.com",
		`a.com" always_transparent`, "a.com\nlocal-data: x", "ünï.com", "a.com/chemin",
		strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 130) + "com",
	} {
		if _, ok := ValidDomain(bad); ok {
			t.Errorf("ValidDomain(%q) accepté", bad)
		}
	}
}

func TestParse(t *testing.T) {
	list := fakeList(MinDomains,
		"*.pub0.exemple-pub.net",           // doublon, avec joker
		"0.0.0.0 hosts-format.net",         // autre format : ignoré
		`evil.net" redirect`,               // tentative d'injection : ignorée
		"example.com", "www.example.com",   // protégés
		"org", "dnssec-failed.org",         // protégés
		"check.aria-shield.test", "ipify.org", "api.ipify.org",
	)
	got, err := Parse(strings.NewReader(list))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MinDomains {
		t.Fatalf("%d domaines retenus, attendu %d", len(got), MinDomains)
	}
	for _, d := range got {
		if !strings.HasSuffix(d, ".exemple-pub.net") {
			t.Fatalf("domaine inattendu retenu : %q", d)
		}
	}
}

func TestParseRejectsSmallList(t *testing.T) {
	if _, err := Parse(strings.NewReader(fakeList(10))); err == nil {
		t.Fatal("liste de 10 domaines acceptée")
	}
	if _, err := Parse(strings.NewReader("<html>erreur 503</html>")); err == nil {
		t.Fatal("page d'erreur acceptée")
	}
}

func TestRender(t *testing.T) {
	domains := []string{"ads.net", "colis.ads.net", "suivi.poste.fr", "tracker.io", "example.com", `x" y.net`}
	conf, n := Render(domains, []string{"Colis.ads.net", "poste.fr", "pas valide"})
	text := string(conf)
	if n != 2 {
		t.Fatalf("%d domaines bloqués, attendu 2 :\n%s", n, text)
	}
	for _, want := range []string{
		"server:\n",
		`local-zone: "` + Canary + `." always_null`,
		`local-zone: "ads.net." always_null`,
		`local-zone: "tracker.io." always_null`,
		`local-zone: "colis.ads.net." transparent`,
		`local-zone: "poste.fr." transparent`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("règle absente : %s", want)
		}
	}
	for _, bad := range []string{`"suivi.poste.fr."`, `"colis.ads.net." always_null`, "example.com", "pas valide", `x" y`} {
		if strings.Contains(text, bad) {
			t.Errorf("présent à tort : %s", bad)
		}
	}
	// Chaque ligne est un commentaire, « server: » ou une zone : rien d'autre.
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		switch {
		case strings.HasPrefix(line, "#"), line == "server:":
		case strings.HasPrefix(line, `local-zone: "`) &&
			(strings.HasSuffix(line, `." always_null`) || strings.HasSuffix(line, `." transparent`)):
			if seen[line] {
				t.Errorf("zone en double : %s", line)
			}
			seen[line] = true
		default:
			t.Errorf("ligne inattendue : %q", line)
		}
	}
}

func storeWith(t *testing.T, body string, err error) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	s.fetch = func(context.Context) (io.ReadCloser, error) {
		if err != nil {
			return nil, err
		}
		return io.NopCloser(strings.NewReader(body)), nil
	}
	return s
}

func TestStoreUpdateAndRebuild(t *testing.T) {
	s := storeWith(t, fakeList(MinDomains+5), nil)
	if s.ConfPath() != "" || !s.Stale() {
		t.Fatal("liste annoncée présente avant tout téléchargement")
	}
	info, err := s.Update(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Present || info.Count != MinDomains+5 || info.Source != SourceName {
		t.Fatalf("fiche inattendue : %+v", info)
	}
	if s.ConfPath() == "" || s.Stale() {
		t.Fatal("liste absente ou périmée juste après le téléchargement")
	}
	// Une exception retire un domaine sans retélécharger, et garde la date.
	again, err := s.Rebuild([]string{"pub3.exemple-pub.net"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Count != info.Count-1 || !again.Updated.Equal(info.Updated) {
		t.Fatalf("après exception : %+v (avant : %+v)", again, info)
	}
	if got := s.Info(); got.Count != again.Count {
		t.Fatalf("fiche relue : %+v", got)
	}
	// Aucun fichier temporaire ne traîne.
	entries, _ := os.ReadDir(s.dir)
	if len(entries) != 3 {
		t.Fatalf("fichiers présents : %v", entries)
	}
}

func TestStoreKeepsOldListOnFailure(t *testing.T) {
	s := storeWith(t, fakeList(MinDomains), nil)
	if _, err := s.Update(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.dir, confName))

	s.fetch = func(context.Context) (io.ReadCloser, error) { return nil, errors.New("réseau coupé") }
	if _, err := s.Update(context.Background(), nil); err == nil {
		t.Fatal("échec de téléchargement non signalé")
	}
	s.fetch = func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("<html>503</html>")), nil
	}
	if _, err := s.Update(context.Background(), nil); err == nil {
		t.Fatal("page d'erreur acceptée comme liste")
	}
	after, _ := os.ReadFile(filepath.Join(s.dir, confName))
	if string(before) != string(after) {
		t.Fatal("la liste installée a été modifiée par un téléchargement raté")
	}
}

func TestStoreWithoutLockedDir(t *testing.T) {
	s := NewStore("")
	if _, err := s.Update(context.Background(), nil); err == nil {
		t.Fatal("installation acceptée sans dossier verrouillé")
	}
	if s.ConfPath() != "" || s.Info().Present {
		t.Fatal("liste annoncée présente sans dossier")
	}
}
