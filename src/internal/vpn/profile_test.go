package vpn

import (
	"strings"
	"testing"
)

// Clés de test générées pour l'occasion, sans rapport avec un vrai compte.
const (
	testPriv = "YHfSyDAbVM4xsZHBH4dLR3Brd5n9uiHkMv6BVf0xkGw="
	testPub  = "Wd3XQ0cpdYv1JNVbQMjX/T5U0sB7Z8zKzKc9t9ZPHXc="
)

func validInput() Input {
	return Input{
		Name:       "Proton NL-FREE",
		PrivateKey: testPriv,
		Addresses:  "10.2.0.2/32, 2a07:b944::2:2/128",
		PublicKey:  testPub,
		Endpoint:   "89.39.107.156:51820",
		Keepalive:  "25",
	}
}

func TestValidateOK(t *testing.T) {
	p, err := validInput().Validate("")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Addresses) != 2 || !p.HasIPv6() || p.Keepalive != 25 || p.Endpoint != "89.39.107.156:51820" {
		t.Fatalf("profil inattendu : %+v", p)
	}
	v := p.View()
	if !v.Configured || strings.Contains(strings.Join([]string{v.Name, v.PublicKey, v.Endpoint}, " "), testPriv) {
		t.Fatal("la vue ne doit jamais contenir la clé privée")
	}
}

func TestValidateKeepsExistingKey(t *testing.T) {
	in := validInput()
	in.PrivateKey = ""
	p, err := in.Validate(testPriv)
	if err != nil || p.PrivateKey != testPriv {
		t.Fatalf("la clé existante doit être conservée : %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	in := Input{
		PrivateKey: "pas-une-cle",
		Addresses:  "10.2.0.300/32",
		PublicKey:  testPriv,
		Endpoint:   "89.39.107.156",
		Keepalive:  "-3",
	}
	_, err := in.Validate("")
	fe, ok := err.(FieldErrors)
	if !ok {
		t.Fatalf("FieldErrors attendu, reçu %v", err)
	}
	for _, field := range []string{"privateKey", "addresses", "endpoint", "keepalive"} {
		if fe[field] == "" {
			t.Errorf("erreur attendue sur %s, reçu %v", field, fe)
		}
	}

	in2 := validInput()
	in2.PublicKey = in2.PrivateKey
	if _, err := in2.Validate(""); err == nil || err.(FieldErrors)["publicKey"] == "" {
		t.Error("une clé publique identique à la clé privée doit être refusée")
	}
}

func TestEndpointIPv6AndDefaults(t *testing.T) {
	in := validInput()
	in.Endpoint = "[2a00:7c80:0:3b6::10]:51820"
	in.Keepalive = ""
	in.Addresses = "10.2.0.2"
	p, err := in.Validate("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Endpoint != "[2a00:7c80:0:3b6::10]:51820" || p.Keepalive != 25 || p.Addresses[0] != "10.2.0.2/32" {
		t.Fatalf("profil inattendu : %+v", p)
	}
}
