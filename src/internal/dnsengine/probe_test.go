package dnsengine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildQuery(t *testing.T) {
	q, err := buildQuery(0xBEEF, "example.com.")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0xBE, 0xEF, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0,
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0,
		0, 1, 0, 1,
	}
	if string(q) != string(want) {
		t.Fatalf("requête inattendue :\n got %v\nwant %v", q, want)
	}
	if _, err := buildQuery(1, "bad..name"); err == nil {
		t.Fatal("un label vide aurait dû être refusé")
	}
}

func TestParseHeader(t *testing.T) {
	// Réponse SERVFAIL, 0 réponse.
	resp := []byte{0x12, 0x34, 0x81, 0x82, 0, 1, 0, 0, 0, 0, 0, 0}
	a, err := parseHeader(0x1234, resp)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rcode != RcodeServFail || a.Answers != 0 {
		t.Fatalf("lu %+v", a)
	}
	// Réponse NOERROR avec 2 enregistrements.
	resp = []byte{0x12, 0x34, 0x81, 0x80, 0, 1, 0, 2, 0, 0, 0, 0}
	if a, _ = parseHeader(0x1234, resp); a.Rcode != RcodeNoError || a.Answers != 2 {
		t.Fatalf("lu %+v", a)
	}
	if _, err := parseHeader(0x9999, resp); err == nil {
		t.Fatal("un ID différent aurait dû être refusé")
	}
}

func TestRenderConfig(t *testing.T) {
	conf := RenderConfig("/data/AriaShield-data", true)
	for _, want := range []string{
		`directory: "/data/AriaShield-data"`,
		`auto-trust-anchor-file: "/data/AriaShield-data/root.key"`,
		"interface: 127.0.0.1",
		"interface: ::1",
		"prefer-ip4: yes",
		`local-data: "check.aria-shield.test. 60 IN A 192.0.2.53"`,
		"control-enable: no",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("%q absent de la config :\n%s", want, conf)
		}
	}
	if strings.Contains(conf, "{{") || strings.Contains(conf, "forward-zone:") {
		t.Fatal("config mal générée")
	}

	// Modèle en CRLF (dépôt cloné sous Windows) : le repère doit être remplacé quand même.
	saved := confTemplate
	confTemplate = strings.ReplaceAll(saved, "\n", "\r\n")
	crlf := RenderConfig("/d", true)
	confTemplate = saved
	if strings.Contains(crlf, "{{") || !strings.Contains(crlf, "interface: ::1") {
		t.Fatalf("modèle CRLF mal rendu :\n%s", crlf)
	}

	v4 := RenderConfig("/d", false)
	if strings.Contains(v4, "::1") || !strings.Contains(v4, "do-ip6: no") {
		t.Fatalf("config IPv4 seule incorrecte :\n%s", v4)
	}
}

func TestEmbeddedUnbound(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "unbound.exe")
	if err := extractUnbound(dst); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(dst); got != unboundSHA256 {
		t.Fatalf("empreinte extraite %s", got)
	}
	if !strings.Contains(string(rootKey), "38696 8 2 683D2D0ACB8C9B712A1948B27F741219298D0A450D612C483AF444A4C0FB2B16") {
		t.Fatal("clé racine KSK-2024 absente")
	}
}
