// Package blocklist fabrique la liste de blocage des pubs et des traceurs
// qu'Unbound charge en plus de sa configuration.
//
// La liste vient d'un tiers (OISD « small », téléchargée en HTTPS, sans
// signature). Elle n'est donc JAMAIS donnée telle quelle à Unbound : un fichier
// de configuration piégé pourrait rediriger un domaine (ta banque) vers une
// autre adresse. Aria n'en retient que des noms de domaine valides (Parse) et
// écrit elle-même les règles (Render), qui ne savent faire qu'une chose :
// répondre 0.0.0.0. Le pire qu'une liste piégée puisse obtenir est donc de
// bloquer un site de trop, et les domaines dont Aria a besoin pour fonctionner
// ne sont jamais bloqués (protected).
//
// La liste n'est pas embarquée dans l'exe : OISD est sous licence GPLv3, Aria
// sous MIT. Elle est téléchargée sur ta machine, à ta demande.
package blocklist

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	// MaxDomains plafonne la liste : au-delà, ce n'est plus la liste attendue.
	MaxDomains = 500_000
	// MinDomains : en dessous, le téléchargement est tronqué ou ce n'est pas la bonne page.
	MinDomains = 1_000
	// MaxAllow plafonne les exceptions de l'utilisateur.
	MaxAllow = 200

	// Canary est un nom bloqué par toute liste générée : s'il répond, c'est que
	// la liste est bien chargée par Unbound (page Tests).
	Canary = "blocage.aria-shield-canari.test"
)

// protected : domaines qu'une liste ne doit jamais bloquer, ni leurs
// sous-domaines ni leurs parents, parce qu'Aria s'en sert pour démarrer, se
// tester ou se mettre à jour.
var protected = []string{
	"aria-shield.test",  // marqueur « Windows passe par Aria »
	"example.com",       // test de résolution au démarrage d'Unbound
	"dnssec-failed.org", // test DNSSEC : doit rester en échec de validation
	"ipify.org",         // mesure de l'IP publique (test de fuite)
	"torproject.org",    // mises à jour de Tor Browser
	"oisd.nl",           // la liste elle-même
	"github.com",        // releases d'Aria
}

// ValidDomain normalise un nom (minuscules, sans point final ni « *. ») et dit
// s'il est acceptable : au moins deux labels, lettres, chiffres, tirets et
// tirets bas uniquement, 253 caractères au plus. Rien d'autre ne peut donc
// atteindre le fichier lu par Unbound (ni guillemet, ni espace, ni retour ligne).
func ValidDomain(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "*.")
	s = strings.TrimSuffix(s, ".")
	if len(s) < 3 || len(s) > 253 {
		return "", false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, l := range labels {
		if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return "", false
		}
		for i := 0; i < len(l); i++ {
			c := l[i]
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return "", false
			}
		}
	}
	return s, true
}

// within indique si name est domain ou l'un de ses sous-domaines.
func within(name, domain string) bool {
	return name == domain || strings.HasSuffix(name, "."+domain)
}

// isProtected : name touche-t-il un domaine protégé (lui, un sous-domaine, ou
// un parent qui l'engloberait) ?
func isProtected(name string) bool {
	for _, p := range protected {
		if within(name, p) || within(p, name) {
			return true
		}
	}
	return false
}

// Parse lit une liste « un domaine par ligne » (commentaires en #, « *. »
// toléré) et renvoie les noms valides, triés et sans doublon. Les lignes
// invalides sont ignorées ; une liste trop petite ou trop grosse est refusée.
func Parse(r io.Reader) ([]string, error) {
	seen := make(map[string]struct{}, 70_000)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 4096) // une ligne plus longue n'est pas un domaine
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		d, ok := ValidDomain(line)
		if !ok || isProtected(d) {
			continue
		}
		seen[d] = struct{}{}
		if len(seen) > MaxDomains {
			return nil, fmt.Errorf("liste trop grosse (plus de %d domaines)", MaxDomains)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("liste illisible : %w", err)
	}
	if len(seen) < MinDomains {
		return nil, fmt.Errorf("liste trop petite (%d domaines) : téléchargement incomplet ?", len(seen))
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out, nil
}

// CleanAllow valide les exceptions de l'utilisateur : noms normalisés, sans
// doublon, MaxAllow au plus.
func CleanAllow(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		d, ok := ValidDomain(s)
		if !ok {
			continue
		}
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
		if len(out) == MaxAllow {
			break
		}
	}
	sort.Strings(out)
	return out
}

// Render écrit le fichier inclus par Unbound. Chaque domaine bloqué devient
// une zone « always_null » (0.0.0.0 pour lui et ses sous-domaines). Une
// exception retire les entrées qu'elle couvre et pose une zone « transparent »,
// qui l'emporte sur un parent bloqué. Renvoie le fichier et le nombre de
// domaines réellement bloqués.
func Render(domains, allow []string) ([]byte, int) {
	allow = CleanAllow(allow)
	var b bytes.Buffer
	b.WriteString("# Généré par Aria Shield : ne pas modifier, le fichier est réécrit à chaque mise à jour.\n")
	b.WriteString("server:\n")
	// Une ligne de journal par requête bloquée : c'est ce que lit le compteur
	// (internal/blockstats). Actif seulement quand la liste est chargée.
	b.WriteString("log-local-actions: yes\n")
	fmt.Fprintf(&b, "local-zone: \"%s.\" always_null\n", Canary)
	for _, a := range allow {
		fmt.Fprintf(&b, "local-zone: \"%s.\" transparent\n", a)
	}
	n := 0
	for _, d := range domains {
		if _, ok := ValidDomain(d); !ok || isProtected(d) || allowed(d, allow) {
			continue
		}
		fmt.Fprintf(&b, "local-zone: \"%s.\" always_null\n", d)
		n++
	}
	return b.Bytes(), n
}

// allowed : d est-il couvert par une exception (elle-même ou un parent) ?
func allowed(d string, allow []string) bool {
	for _, a := range allow {
		if within(d, a) {
			return true
		}
	}
	return false
}

// ErrEmpty est renvoyé quand aucune liste n'a encore été téléchargée.
var ErrEmpty = errors.New("aucune liste téléchargée")
