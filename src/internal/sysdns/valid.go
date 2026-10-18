package sysdns

import (
	"fmt"
	"net/netip"
	"regexp"
)

// canonicalAddrs n'accepte que des adresses IP littérales et les réécrit sous
// leur forme canonique. Les valeurs finissent dans un script PowerShell lancé en
// administrateur et dans le registre : une adresse ne contient ni guillemet ni
// espace, donc rien ne peut en sortir (PowerShell prend aussi ‘ ’ ‚ ‛ pour des
// apostrophes, que le simple doublement de ' ne neutralise pas).
func canonicalAddrs(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, s := range items {
		a, err := netip.ParseAddr(s)
		// Une zone IPv6 (« fe80::1%texte ») accepte n'importe quel caractère : refusée.
		if err != nil || a.Zone() != "" {
			return nil, fmt.Errorf("adresse DNS invalide : %q", s)
		}
		out = append(out, a.String())
	}
	return out, nil
}

var guidRe = regexp.MustCompile(`^\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}$`)

// validGUID vérifie l'identifiant de carte avant de l'accoler à un chemin de registre.
func validGUID(s string) bool { return guidRe.MatchString(s) }
