package vpn

import (
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// MaxConfSize borne la taille d'un fichier .conf importé : un vrai fait moins d'1 Ko.
const MaxConfSize = 64 << 10

// Import est le résultat de la lecture d'un fichier .conf WireGuard.
type Import struct {
	Input Input
	Notes []string // ce qu'Aria a ignoré ou imposé, à montrer à l'utilisateur
}

// NameFromFile tire un nom de profil du nom de fichier (« wg-NL-12.conf » → « wg-NL-12 »).
func NameFromFile(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	name := strings.TrimSuffix(path, filepath.Ext(path))
	name = strings.TrimSpace(strings.NewReplacer("_", " ").Replace(name))
	if r := []rune(name); len(r) > 40 {
		name = string(r[:40])
	}
	return name
}

// ParseConf lit un fichier de configuration WireGuard (format wg-quick).
// Seules les valeurs utiles à Aria sont gardées. Les commandes (PostUp…) ne
// sont jamais exécutées : elles sont ignorées et signalées.
func ParseConf(text, name string) (Import, error) {
	if len(text) > MaxConfSize {
		return Import{}, errors.New("fichier trop gros pour une configuration WireGuard")
	}
	if !utf8.ValidString(text) {
		return Import{}, errors.New("fichier illisible : ce n'est pas un texte")
	}
	text = strings.TrimPrefix(text, "\ufeff")

	var (
		res      = Import{Input: Input{Name: name}}
		section  string
		peers    int
		addrs    []string
		ignored  = map[string]bool{}
		fullTunn = true
		sawAllow bool
	)
	note := func(format string, a ...any) { res.Notes = append(res.Notes, fmt.Sprintf(format, a...)) }

	for n, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.IndexAny(line, "#;"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			switch section {
			case "interface":
			case "peer":
				peers++
			default:
				return Import{}, fmt.Errorf("ligne %d : section [%s] inconnue", n+1, section)
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Import{}, fmt.Errorf("ligne %d : « clé = valeur » attendu", n+1)
		}
		key, val = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(val)

		switch {
		case section == "":
			return Import{}, fmt.Errorf("ligne %d : valeur hors de toute section", n+1)
		case peers > 1:
			// le refus tombe après la boucle
		case section == "interface":
			switch key {
			case "privatekey":
				res.Input.PrivateKey = val
			case "address":
				addrs = append(addrs, splitList(val)...)
			case "dns":
				ignored["DNS : Unbound résout à travers le tunnel"] = true
			case "preup", "postup", "predown", "postdown":
				ignored["commandes PreUp/PostUp… : Aria n'exécute jamais rien venant d'un fichier"] = true
			default:
				ignored[key] = true
			}
		case section == "peer":
			switch key {
			case "publickey":
				res.Input.PublicKey = val
			case "presharedkey":
				res.Input.PresharedKey = val
			case "endpoint":
				res.Input.Endpoint = val
			case "persistentkeepalive":
				res.Input.Keepalive = val
			case "allowedips":
				sawAllow = true
				if !coversEverything(val) {
					fullTunn = false
				}
			default:
				ignored[key] = true
			}
		}
	}

	if peers == 0 {
		return Import{}, errors.New("aucune section [Peer] : ce n'est pas une configuration WireGuard complète")
	}
	if peers > 1 {
		return Import{}, errors.New("plusieurs serveurs [Peer] dans le fichier : Aria n'en gère qu'un par profil")
	}
	res.Input.Addresses = strings.Join(addrs, ", ")
	if sawAllow && !fullTunn {
		note("AllowedIPs ignoré : Aria fait passer tout le trafic dans le tunnel")
	}
	keys := make([]string, 0, len(ignored))
	for k := range ignored {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Contains(k, " ") {
			note("%s", k)
		} else {
			note("%s ignoré (inutile pour Aria)", k)
		}
	}
	return res, nil
}

// coversEverything indique si la liste AllowedIPs couvre tout IPv4 (tunnel complet).
func coversEverything(list string) bool {
	for _, a := range splitList(list) {
		if p, err := netip.ParsePrefix(a); err == nil && p.Bits() == 0 && p.Addr().Is4() {
			return true
		}
	}
	return false
}
