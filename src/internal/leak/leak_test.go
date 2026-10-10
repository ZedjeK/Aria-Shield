package leak

import "testing"

func TestCompare(t *testing.T) {
	box := IPs{V4: "86.1.2.3", V6: "2a01:e0a::1"}

	if v := Compare(box, IPs{V4: "89.39.107.156", V6: "2a07:b944::2:2"}); !v.V4OK || !v.V6OK {
		t.Errorf("tunnel complet : aucune fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "89.39.107.156"}); !v.V4OK || !v.V6OK {
		t.Errorf("IPv6 avalé par le tunnel : pas une fuite, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "86.1.2.3"}); v.V4OK {
		t.Errorf("même IPv4 que la box : fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{V4: "89.39.107.156", V6: "2a01:e0a::1"}); v.V6OK {
		t.Errorf("même IPv6 que la box : fuite attendue, reçu %+v", v)
	}
	if v := Compare(box, IPs{}); v.V4OK {
		t.Errorf("rien de mesuré : pas de verdict positif, reçu %+v", v)
	}
}
