package browsers

import "testing"

func TestChromiumMode(t *testing.T) {
	cases := map[string]string{
		`{"dns_over_https":{"mode":"secure","templates":"https://dns.example/dns-query"}}`: "secure",
		`{"dns_over_https":{"mode":"off"}}`:                                                "off",
		`{"browser":{}}`:                                                                   "automatic",
		`pas du json`:                                                                      "automatic",
	}
	for in, want := range cases {
		if got := chromiumMode([]byte(in)); got != want {
			t.Errorf("chromiumMode(%s) = %q, attendu %q", in, got, want)
		}
	}
	if _, bypass := describeChromium("secure"); !bypass {
		t.Error("le mode secure doit être signalé comme contournant Aria")
	}
	if _, bypass := describeChromium("automatic"); bypass {
		t.Error("le mode automatique suit Windows")
	}
}

func TestFirefoxMode(t *testing.T) {
	prefs := []byte(`user_pref("browser.startup.page", 3);
user_pref("doh-rollout.mode", 2);
user_pref("network.trr.mode", 5);`)
	if got := firefoxMode(prefs); got != 5 {
		t.Fatalf("le réglage utilisateur doit primer, lu %d", got)
	}
	if got := firefoxMode([]byte(`user_pref("doh-rollout.mode", 2);`)); got != 2 {
		t.Fatalf("déploiement automatique non détecté, lu %d", got)
	}
	if got := firefoxMode([]byte(`user_pref("browser.startup.page", 3);`)); got != 0 {
		t.Fatalf("mode par défaut attendu, lu %d", got)
	}
}
