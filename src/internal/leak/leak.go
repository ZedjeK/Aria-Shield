// Package leak mesure l'adresse IP publique vue par internet, en IPv4 et en IPv6.
package leak

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// Services d'écho d'IP : api.ipify.org n'a que de l'IPv4, api6.ipify.org que de l'IPv6.
const (
	echo4   = "https://api.ipify.org"
	echo6   = "https://api6.ipify.org"
	timeout = 8 * time.Second
)

// IPs est l'adresse publique vue de l'extérieur ("" si aucune).
type IPs struct {
	V4 string `json:"v4"`
	V6 string `json:"v6"`
}

func fetch(ctx context.Context, url, network string) string {
	dialer := &net.Dialer{Timeout: timeout}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: nil, // jamais de proxy système : on veut la vraie sortie
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			TLSHandshakeTimeout: timeout,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(string(body)))
	if err != nil {
		return ""
	}
	return ip.String()
}

// Public mesure l'IP publique (IPv4 et IPv6 en parallèle).
func Public() IPs {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()
	v4 := make(chan string, 1)
	v6 := make(chan string, 1)
	go func() { v4 <- fetch(ctx, echo4, "tcp4") }()
	go func() { v6 <- fetch(ctx, echo6, "tcp6") }()
	return IPs{V4: <-v4, V6: <-v6}
}

// Verdict compare l'IP de la box (avant protection) à celle vue avec la protection.
type Verdict struct {
	Before IPs    `json:"before"`
	After  IPs    `json:"after"`
	V4OK   bool   `json:"v4Ok"`
	V6OK   bool   `json:"v6Ok"`
	Detail string `json:"detail"`
}

// Compare juge s'il y a fuite.
func Compare(before, after IPs) Verdict {
	v := Verdict{Before: before, After: after}
	switch {
	case after.V4 == "":
		v.Detail = "IP publique impossible à mesurer en IPv4"
	case before.V4 != "" && after.V4 == before.V4:
		v.Detail = "fuite IPv4 : les sites voient encore l'IP de ta box"
	default:
		v.V4OK = true
	}
	// IPv6 : soit rien ne sort, soit ce n'est plus l'adresse de la box.
	if after.V6 == "" || before.V6 == "" || after.V6 != before.V6 {
		v.V6OK = true
	} else if v.Detail == "" {
		v.Detail = "fuite IPv6 : ton adresse IPv6 d'origine est visible"
	}
	if v.V4OK && v.V6OK {
		v.Detail = "aucune fuite : les sites voient l'IP du VPN"
	}
	return v
}
