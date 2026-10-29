package vpn

import (
	"strings"
	"testing"
)

const sampleConf = "\ufeff[Interface]\r\n" +
	"# Key for NL-FREE#12\r\n" +
	"PrivateKey = " + testPriv + "\r\n" +
	"Address = 10.2.0.2/32\r\n" +
	"Address = 2a07:b944::2:2/128\r\n" +
	"DNS = 10.2.0.1\r\n" +
	"PostUp = calc.exe\r\n" +
	"\r\n" +
	"[Peer]\r\n" +
	"# NL-FREE#12\r\n" +
	"PublicKey = " + testPub + "\r\n" +
	"AllowedIPs = 0.0.0.0/0, ::/0\r\n" +
	"Endpoint = 203.0.113.10:51820\r\n"

func TestParseConf(t *testing.T) {
	imp, err := ParseConf(sampleConf, NameFromFile(`C:\Users\x\Downloads\wg_NL-FREE-12.conf`))
	if err != nil {
		t.Fatal(err)
	}
	in := imp.Input
	if in.Name != "wg NL-FREE-12" || in.PrivateKey != testPriv || in.PublicKey != testPub ||
		in.Endpoint != "203.0.113.10:51820" || in.Addresses != "10.2.0.2/32, 2a07:b944::2:2/128" {
		t.Fatalf("lecture inattendue : %+v", in)
	}
	notes := strings.Join(imp.Notes, " | ")
	if !strings.Contains(notes, "DNS") || !strings.Contains(notes, "PostUp") || strings.Contains(notes, "AllowedIPs") {
		t.Fatalf("notes inattendues : %s", notes)
	}
	p, err := in.Validate(Profile{})
	if err != nil || p.Keepalive != 25 || !p.HasIPv6() {
		t.Fatalf("profil importé invalide : %v %+v", err, p)
	}
}

func TestParseConfPresharedAndSplitTunnel(t *testing.T) {
	psk := "Gd7zHYbcYjfn2lQ1KJpOVqvnIq3bW7a8hXc0hLdV4yY="
	text := "[Interface]\nPrivateKey=" + testPriv + "\nAddress=10.0.0.2/32\n[Peer]\nPublicKey=" + testPub +
		"\nPresharedKey=" + psk + "\nAllowedIPs=10.0.0.0/8\nEndpoint=vpn.example.net:51820\nPersistentKeepalive=15\n"
	imp, err := ParseConf(text, "x")
	if err != nil {
		t.Fatal(err)
	}
	if imp.Input.PresharedKey != psk || imp.Input.Keepalive != "15" || !strings.Contains(strings.Join(imp.Notes, " "), "AllowedIPs") {
		t.Fatalf("lecture inattendue : %+v", imp)
	}
	p, err := imp.Input.Validate(Profile{})
	if err != nil || p.PresharedKey != psk || !p.View().HasPSK {
		t.Fatalf("clé partagée perdue : %v", err)
	}
	if strings.Contains(p.View().Name+p.View().PublicKey, psk) {
		t.Fatal("la clé partagée ne doit jamais apparaître dans la vue")
	}
}

func TestParseConfRefus(t *testing.T) {
	cases := map[string]string{
		"pas de peer":     "[Interface]\nPrivateKey=" + testPriv + "\n",
		"deux peers":      "[Interface]\nPrivateKey=" + testPriv + "\n[Peer]\nPublicKey=" + testPub + "\n[Peer]\nPublicKey=" + testPub + "\n",
		"hors section":    "PrivateKey=" + testPriv + "\n",
		"section bizarre": "[Truc]\n",
		"sans égal":       "[Interface]\nPrivateKey\n",
		"trop gros":       "[Interface]\n" + strings.Repeat("#", MaxConfSize),
		"binaire":         "[Interface]\n\xff\xfe",
	}
	for name, text := range cases {
		if _, err := ParseConf(text, "x"); err == nil {
			t.Errorf("%s : refus attendu", name)
		}
	}
}
