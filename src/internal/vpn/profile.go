// Package vpn gère le profil WireGuard saisi par l'utilisateur et le tunnel
// (pilote officiel WireGuardNT), avec le kill switch.
package vpn

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// Profile est la configuration WireGuard saisie à la main.
// AllowedIPs n'en fait pas partie : Aria impose le tunnel complet.
type Profile struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	PrivateKey   string   `json:"privateKey"`
	PresharedKey string   `json:"presharedKey,omitempty"`
	Addresses    []string `json:"addresses"`
	PublicKey    string   `json:"publicKey"`
	Endpoint     string   `json:"endpoint"`
	Keepalive    int      `json:"keepalive"`
}

// View est ce que l'interface a le droit de voir : jamais la clé privée.
type View struct {
	ID         string   `json:"id"`
	Active     bool     `json:"active"`
	Configured bool     `json:"configured"`
	Name       string   `json:"name"`
	Addresses  []string `json:"addresses"`
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	Keepalive  int      `json:"keepalive"`
	HasIPv6    bool     `json:"hasIpv6"`
	HasPSK     bool     `json:"hasPsk"` // clé partagée présente (jamais affichée)
}

// Input est ce que l'interface envoie à l'enregistrement.
// Adresses en texte libre (séparées par des virgules) ; clé privée ou clé
// partagée vides = garder celles déjà enregistrées.
type Input struct {
	Name         string `json:"name"`
	PrivateKey   string `json:"privateKey"`
	PresharedKey string `json:"presharedKey"`
	Addresses    string `json:"addresses"`
	PublicKey    string `json:"publicKey"`
	Endpoint     string `json:"endpoint"`
	Keepalive    string `json:"keepalive"`
}

// FieldErrors associe un champ du formulaire à un message en français.
type FieldErrors map[string]string

func (f FieldErrors) Error() string {
	parts := make([]string, 0, len(f))
	for k, v := range f {
		parts = append(parts, k+" : "+v)
	}
	return strings.Join(parts, " ; ")
}

func checkKey(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("obligatoire")
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) != 32 {
		return errors.New("clé invalide : 44 caractères en base64, terminés par « = »")
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseEndpoint accepte « IP:port », « [IPv6]:port » ou « nom:port ».
func parseEndpoint(s string) (host string, port int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, errors.New("obligatoire")
	}
	h, p, err := net.SplitHostPort(s)
	if err != nil || h == "" {
		return "", 0, errors.New("format attendu : IP:port, par exemple 203.0.113.10:51820")
	}
	port, err = strconv.Atoi(p)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, errors.New("port invalide (1 à 65535)")
	}
	return h, port, nil
}

// Validate vérifie la saisie et renvoie un profil propre. existing est le profil
// déjà enregistré : ses clés servent quand les champs correspondants sont vides.
func (in Input) Validate(existing Profile) (Profile, error) {
	errs := FieldErrors{}
	p := Profile{Name: strings.TrimSpace(in.Name)}

	if p.Name == "" {
		p.Name = "Mon VPN"
	}
	if len([]rune(p.Name)) > 40 {
		errs["name"] = "40 caractères maximum"
	}

	p.PrivateKey = strings.TrimSpace(in.PrivateKey)
	if p.PrivateKey == "" {
		p.PrivateKey = existing.PrivateKey
	}
	if err := checkKey(p.PrivateKey); err != nil {
		errs["privateKey"] = err.Error()
	}

	p.PresharedKey = strings.TrimSpace(in.PresharedKey)
	if p.PresharedKey == "" {
		p.PresharedKey = existing.PresharedKey
	}
	if p.PresharedKey != "" {
		if err := checkKey(p.PresharedKey); err != nil {
			errs["presharedKey"] = err.Error()
		}
	}

	p.PublicKey = strings.TrimSpace(in.PublicKey)
	if err := checkKey(p.PublicKey); err != nil {
		errs["publicKey"] = err.Error()
	} else if p.PublicKey == p.PrivateKey {
		errs["publicKey"] = "c'est la clé publique du serveur, pas ta clé privée"
	}

	addrs := splitList(in.Addresses)
	if len(addrs) == 0 {
		errs["addresses"] = "obligatoire"
	}
	for _, a := range addrs {
		pre, err := netip.ParsePrefix(a)
		if err != nil {
			ip, err2 := netip.ParseAddr(a)
			if err2 != nil {
				errs["addresses"] = fmt.Sprintf("« %s » n'est pas une adresse valide (ex. 10.2.0.2/32)", a)
				break
			}
			pre = netip.PrefixFrom(ip, ip.BitLen())
		}
		p.Addresses = append(p.Addresses, pre.String())
	}

	if host, port, err := parseEndpoint(in.Endpoint); err != nil {
		errs["endpoint"] = err.Error()
	} else {
		p.Endpoint = net.JoinHostPort(host, strconv.Itoa(port))
	}

	ka := strings.TrimSpace(in.Keepalive)
	if ka == "" {
		p.Keepalive = 25
	} else if v, err := strconv.Atoi(ka); err != nil || v < 0 || v > 65535 {
		errs["keepalive"] = "nombre de secondes entre 0 et 65535"
	} else {
		p.Keepalive = v
	}

	if len(errs) > 0 {
		return Profile{}, errs
	}
	return p, nil
}

// HasIPv6 indique si le profil donne une adresse IPv6 dans le tunnel.
func (p Profile) HasIPv6() bool {
	for _, a := range p.Addresses {
		if pre, err := netip.ParsePrefix(a); err == nil && pre.Addr().Is6() {
			return true
		}
	}
	return false
}

// View renvoie la version affichable du profil.
func (p Profile) View() View {
	return View{
		ID:         p.ID,
		Configured: p.PrivateKey != "",
		Name:       p.Name,
		Addresses:  p.Addresses,
		PublicKey:  p.PublicKey,
		Endpoint:   p.Endpoint,
		Keepalive:  p.Keepalive,
		HasIPv6:    p.HasIPv6(),
		HasPSK:     p.PresharedKey != "",
	}
}
