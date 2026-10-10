package sysdns

import "testing"

func TestOnAria(t *testing.T) {
	cases := []struct {
		servers []string
		ipv6    bool
		want    bool
	}{
		{[]string{"127.0.0.1"}, false, true},
		{[]string{"127.0.0.1", "::1"}, true, true},
		{[]string{"127.0.0.1", "::1"}, false, false},
		{[]string{"127.0.0.1", "192.168.1.1"}, true, false},
		{[]string{"fe80::1"}, true, false},
		{nil, true, false},
	}
	for _, c := range cases {
		if got := onAria(c.servers, c.ipv6); got != c.want {
			t.Errorf("onAria(%v, %v) = %v, attendu %v", c.servers, c.ipv6, got, c.want)
		}
	}
}
