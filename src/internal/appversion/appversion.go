// Package appversion compare des numéros de version d'Aria et lit le dernier
// publié sur GitHub.
//
// Un numéro venu du réseau ou d'un autre processus n'est accepté que s'il a
// la forme d'une version (Parse) : rien d'autre ne doit pouvoir s'afficher.
package appversion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// ReleasesAPI : dernière release publiée du dépôt.
	ReleasesAPI = "https://api.github.com/repos/ZedjeK/Aria-Shield/releases/latest"
	// ReleasesURL : page à ouvrir pour télécharger la dernière version.
	ReleasesURL = "https://github.com/ZedjeK/Aria-Shield/releases/latest"
)

// « 0.10 », « v0.10.1 »… quatre nombres au plus.
var versionRe = regexp.MustCompile(`^v?\d{1,4}(\.\d{1,4}){0,3}$`)

// Parse découpe « v0.10.1 » en nombres ; ok faux si ce n'est pas une version.
func Parse(v string) (parts []int, ok bool) {
	v = strings.TrimSpace(v)
	if !versionRe.MatchString(v) {
		return nil, false
	}
	for _, p := range strings.Split(strings.TrimPrefix(v, "v"), ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

// Compare renvoie -1, 0 ou 1 ; « 0.10 » et « 0.10.0 » sont égales. ok faux si
// l'un des deux n'est pas une version.
func Compare(a, b string) (cmp int, ok bool) {
	pa, oka := Parse(a)
	pb, okb := Parse(b)
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

// LatestTag demande à GitHub le tag de la dernière release. La réponse est
// bornée, et le tag n'est accepté que s'il a la forme d'une version.
func LatestTag(ctx context.Context) (string, error) {
	return latestTagFrom(ctx, ReleasesAPI)
}

func latestTagFrom(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Aria-Shield")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("réponse %d de GitHub", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("réponse illisible : %w", err)
	}
	if _, ok := Parse(rel.TagName); !ok {
		return "", fmt.Errorf("numéro de version inattendu")
	}
	return rel.TagName, nil
}
