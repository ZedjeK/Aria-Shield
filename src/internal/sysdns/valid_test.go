package sysdns

import (
	"slices"
	"testing"
)

func TestCanonicalAddrs(t *testing.T) {
	got, err := canonicalAddrs([]string{"1.1.1.1", "2606:4700:4700:0000:0000:0000:0000:1111"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1.1.1.1", "2606:4700:4700::1111"}; !slices.Equal(got, want) {
		t.Errorf("canonicalAddrs = %v, attendu %v", got, want)
	}
	for _, bad := range []string{
		"1.1.1.1'; Remove-Item C:\\ -Recurse; '",
		"1.1.1.1’; calc; ‘",
		"fe80::1%’; calc; ‘",
		"dns.exemple.org",
		"",
	} {
		if _, err := canonicalAddrs([]string{bad}); err == nil {
			t.Errorf("%q accepté, aurait dû être refusé", bad)
		}
	}
}

func TestValidGUID(t *testing.T) {
	if !validGUID("{4D36E972-E325-11CE-BFC1-08002BE10318}") {
		t.Error("GUID valide refusé")
	}
	for _, bad := range []string{"", `{4D36E972-E325-11CE-BFC1-08002BE10318}\..\x`, "4D36E972-E325-11CE-BFC1-08002BE10318"} {
		if validGUID(bad) {
			t.Errorf("%q accepté", bad)
		}
	}
}
