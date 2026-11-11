package blockstats

import (
	"os"
	"path/filepath"
	"testing"

	"aria-shield/internal/blocklist"
)

const prefix = `11/11/2026 12:45:48 C:\ProgramData\AriaShield\unbound.exe[9832:0] info: `

func TestParseLine(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{prefix + "pub.exemple-pub.net. always_null 127.0.0.1@53112 sous.pub.exemple-pub.net. A IN", "pub.exemple-pub.net"},
		{prefix + "Tracker.IO. always_null ::1@4000 tracker.io. AAAA IN", "tracker.io"},
		{prefix + "ok.exemple-pub.net. transparent 127.0.0.1@55 ok.exemple-pub.net. A IN", ""},
		{prefix + "aria-shield.test. static 127.0.0.1@55415 check.aria-shield.test. A IN", ""},
		{prefix + blocklist.Canary + ". always_null 127.0.0.1@1 " + blocklist.Canary + ". A IN", ""},
		{prefix + "start of service (unbound 1.26.1).", ""},
		{`pas un domaine" always_null 127.0.0.1@1 x. A IN`, ""},
		{" always_null ", ""},
	}
	for _, c := range cases {
		got, ok := ParseLine(c.line)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("ParseLine(%q) = %q, %v ; attendu %q", c.line, got, ok, c.want)
		}
	}
}

func blocked(domain string) string {
	return prefix + domain + ". always_null 127.0.0.1@5000 " + domain + ". A IN\n"
}

func TestCounterFollowsLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unbound.log")
	c := New(path)
	c.Poll() // journal absent : rien, sans erreur
	if c.Total() != 0 {
		t.Fatal("total non nul sans journal")
	}

	os.WriteFile(path, []byte(prefix+"start of service\n"+blocked("a.net")+blocked("b.net")+blocked("a.net")), 0o644)
	c.Poll()
	c.Poll() // rien de neuf : pas de double compte
	if c.Total() != 3 {
		t.Fatalf("total = %d, attendu 3", c.Total())
	}

	// Une ligne en cours d'écriture n'est comptée qu'une fois terminée.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	half := blocked("c.net")
	f.WriteString(half[:len(half)-10])
	c.Poll()
	if c.Total() != 3 {
		t.Fatalf("ligne incomplète comptée : %d", c.Total())
	}
	f.WriteString(half[len(half)-10:])
	f.Close()
	c.Poll()
	if c.Total() != 4 {
		t.Fatalf("total = %d, attendu 4", c.Total())
	}

	// Unbound redémarre : journal neuf, plus court. Le total continue.
	os.WriteFile(path, []byte(blocked("a.net")), 0o644)
	c.Poll()
	st := c.Snapshot(2)
	if st.Total != 5 || len(st.Top) != 2 || st.Top[0] != (Entry{"a.net", 3}) {
		t.Fatalf("après redémarrage : %+v", st)
	}
	if st.Top[1].Count != 1 || st.Top[1].Domain != "b.net" {
		t.Fatalf("classement : %+v", st.Top)
	}
}
