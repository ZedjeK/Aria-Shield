package appversion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.10", "0.10.0", 0},
		{"v0.10", "0.9.0", 1}, // 10 > 9 : pas une comparaison de texte
		{"0.9.0", "v0.10", -1},
		{"v0.10.1", "0.10.0", 1},
		{"1.0", "0.99.99", 1},
		{"0.11.0", "0.11.0", 0},
	}
	for _, c := range cases {
		got, ok := Compare(c.a, c.b)
		if !ok || got != c.want {
			t.Errorf("Compare(%q, %q) = %d, %v ; attendu %d", c.a, c.b, got, ok, c.want)
		}
	}
}

func TestParseRejectsJunk(t *testing.T) {
	for _, bad := range []string{
		"", "latest", "v", "1.2.3.4.5", "0.10-beta", "0.10\nfaux message", "99999.1",
		"<script>", "0.10 ; calc.exe", " ",
	} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) accepté", bad)
		}
		if _, ok := Compare(bad, "0.10.0"); ok {
			t.Errorf("Compare(%q, …) accepté", bad)
		}
	}
	if p, ok := Parse(" v0.10.1 "); !ok || len(p) != 3 || p[1] != 10 {
		t.Errorf("Parse(v0.10.1) = %v, %v", p, ok)
	}
}

func TestLatestTag(t *testing.T) {
	body, status := `{"tag_name":"v0.12","name":"Aria Shield v0.12"}`, http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	if tag, err := latestTagFrom(context.Background(), srv.URL); err != nil || tag != "v0.12" {
		t.Fatalf("tag = %q, %v", tag, err)
	}
	// Un tag qui n'est pas une version, une réponse cassée, une erreur HTTP : refusés.
	body = `{"tag_name":"v0.12<script>alert(1)</script>"}`
	if _, err := latestTagFrom(context.Background(), srv.URL); err == nil {
		t.Error("tag piégé accepté")
	}
	body = `pas du json`
	if _, err := latestTagFrom(context.Background(), srv.URL); err == nil {
		t.Error("réponse illisible acceptée")
	}
	body, status = `{"tag_name":"v0.12"}`, http.StatusForbidden
	if _, err := latestTagFrom(context.Background(), srv.URL); err == nil {
		t.Error("erreur HTTP acceptée")
	}
}
