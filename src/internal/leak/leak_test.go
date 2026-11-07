package leak

import "testing"

// Adresses de documentation (RFC 5737 et 3849) : aucune n'appartient à quelqu'un.

func TestCompare(t *testing.T) {
	box := IPs{V4: "198.51.100.7", V6: "2001:db8:b0c5::1"}

	if v := Compare(box, IPs{V4: "203.0.113.10", V6: "2001:db8:7f::2"}); !v.V4OK || !v.V6OK {
		t.Errorf("tunnel complet : aucune fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "203.0.113.10"}); !v.V4OK || !v.V6OK {
		t.Errorf("IPv6 avalé par le tunnel : pas une fuite, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "198.51.100.7"}); v.V4OK {
		t.Errorf("même IPv4 que la box : fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "203.0.113.10", V6: "2001:db8:b0c5::1"}); v.V6OK {
		t.Errorf("même IPv6 que la box : fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{}); v.V4OK {
		t.Errorf("rien de mesuré : pas de verdict positif, reçu %+v", v)
	}
}
