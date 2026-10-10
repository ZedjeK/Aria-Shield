// Package vpn gère le profil WireGuard saisi par l'utilisateur et le tunnel
// (pilote officiel WireGuardNT), avec le kill switch.
package vpn

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const profileFile = "vpn-profile.bin"

// Profile est la configuration WireGuard saisie à la main.
// AllowedIPs n'en fait pas partie : Aria impose le tunnel complet.
type Profile struct {
	Name       string   `json:"name"`
	PrivateKey string   `json:"privateKey"`
	Addresses  []string `json:"addresses"`
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	Keepalive  int      `json:"keepalive"`
}

// View est ce que l'interface a le droit de voir : jamais la clé privée.
type View struct {
	Configured bool     `json:"configured"`
	Name       string   `json:"name"`
	Addresses  []string `json:"addresses"`
	PublicKey  string   `json:"publicKey"`
	Endpoint   string   `json:"endpoint"`
	Keepalive  int      `json:"keepalive"`
	HasIPv6    bool     `json:"hasIpv6"`
}

// Input est ce que l'interface envoie à l'enregistrement.
// Adresses en texte libre (séparées par des virgules) ; clé privée vide = garder l'actuelle.
type Input struct {
	Name       string `json:"name"`
	PrivateKey string `json:"privateKey"`
	Addresses  string `json:"addresses"`
	PublicKey  string `json:"publicKey"`
	Endpoint   string `json:"endpoint"`
	Keepalive  string `json:"keepalive"`
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

// Validate vérifie la saisie et renvoie un profil propre. existingKey est la
// clé déjà enregistrée, utilisée si le champ clé privée est laissé vide.
func (in Input) Validate(existingKey string) (Profile, error) {
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
		p.PrivateKey = existingKey
	}
	if err := checkKey(p.PrivateKey); err != nil {
		errs["privateKey"] = err.Error()
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
		Configured: p.PrivateKey != "",
		Name:       p.Name,
		Addresses:  p.Addresses,
		PublicKey:  p.PublicKey,
		Endpoint:   p.Endpoint,
		Keepalive:  p.Keepalive,
		HasIPv6:    p.HasIPv6(),
	}
}

// Store range le profil chiffré dans le dossier de travail.
type Store struct {
	path string
}

// NewStore prépare le stockage du profil dans dataDir.
func NewStore(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, profileFile)}
}

// Load lit et déchiffre le profil ; un profil absent renvoie un Profile vide.
func (s *Store) Load() (Profile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Profile{}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	plain, err := unprotect(data)
	if err != nil {
		return Profile{}, fmt.Errorf("profil illisible (créé par un autre compte Windows ?) : %w", err)
	}
	var p Profile
	if err := json.Unmarshal(plain, &p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Save chiffre et enregistre le profil.
func (s *Store) Save(p Profile) error {
	plain, err := json.Marshal(p)
	if err != nil {
		return err
	}
	data, err := protect(plain)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Delete efface le profil.
func (s *Store) Delete() error {
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
