// Package browsers repère les navigateurs dont le « DNS sécurisé » contourne
// le DNS de Windows (et donc le résolveur d'Aria). Lecture seule : rien n'est modifié.
package browsers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Info décrit l'état d'un navigateur trouvé sur la machine.
type Info struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`   // texte lisible
	Bypass bool   `json:"bypass"` // true si le navigateur ignore le DNS de Windows
	Policy bool   `json:"policy"` // réglage imposé par une stratégie
}

type chromium struct {
	name       string
	localState string
	policyKey  string // sous HKLM\SOFTWARE\Policies, vide si aucune
}

func chromiumBrowsers() []chromium {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	return []chromium{
		{"Google Chrome", filepath.Join(local, `Google\Chrome\User Data\Local State`), `Google\Chrome`},
		{"Microsoft Edge", filepath.Join(local, `Microsoft\Edge\User Data\Local State`), `Microsoft\Edge`},
		{"Brave", filepath.Join(local, `BraveSoftware\Brave-Browser\User Data\Local State`), `BraveSoftware\Brave`},
		{"Vivaldi", filepath.Join(local, `Vivaldi\User Data\Local State`), ""},
		{"Opera", filepath.Join(roaming, `Opera Software\Opera Stable\Local State`), ""},
		{"Opera GX", filepath.Join(roaming, `Opera Software\Opera GX Stable\Local State`), ""},
	}
}

// chromiumMode lit le mode DNS sécurisé dans le fichier « Local State ».
func chromiumMode(data []byte) string {
	var st struct {
		DNS struct {
			Mode string `json:"mode"`
		} `json:"dns_over_https"`
	}
	if json.Unmarshal(data, &st) != nil || st.DNS.Mode == "" {
		return "automatic"
	}
	return st.DNS.Mode
}

func describeChromium(mode string) (string, bool) {
	switch mode {
	case "secure":
		return "DNS sécurisé forcé : contourne Aria", true
	case "off":
		return "DNS sécurisé désactivé : suit Windows", false
	default:
		// En automatique, Chromium ne passe en DoH que si le DNS système est un
		// fournisseur connu ; avec 127.0.0.1, il suit Windows.
		return "Automatique : suit Windows", false
	}
}

var trrMode = regexp.MustCompile(`user_pref\("(network\.trr\.mode|doh-rollout\.mode)",\s*(\d+)\)`)

// firefoxMode renvoie le mode TRR le plus contraignant trouvé dans un prefs.js.
func firefoxMode(prefs []byte) int {
	mode := 0
	for _, m := range trrMode.FindAllSubmatch(prefs, -1) {
		v, _ := strconv.Atoi(string(m[2]))
		if string(m[1]) == "network.trr.mode" {
			return v // le réglage utilisateur prime sur le déploiement automatique
		}
		mode = v
	}
	return mode
}

func describeFirefox(mode int) (string, bool) {
	switch mode {
	case 2:
		return "DNS via HTTPS en priorité : contourne Aria", true
	case 3:
		return "DNS via HTTPS uniquement : contourne Aria", true
	case 5:
		return "DNS via HTTPS désactivé : suit Windows", false
	default:
		return "Par défaut : suit Windows", false
	}
}

// Detect inspecte les navigateurs installés pour l'utilisateur courant.
func Detect() []Info {
	var found []Info

	for _, b := range chromiumBrowsers() {
		data, err := os.ReadFile(b.localState)
		if err != nil {
			continue
		}
		mode := chromiumMode(data)
		policy := false
		if pm, ok := chromiumPolicy(b.policyKey); ok {
			mode, policy = pm, true
		}
		text, bypass := describeChromium(mode)
		found = append(found, Info{Name: b.name, Mode: text, Bypass: bypass, Policy: policy})
	}

	profiles, _ := filepath.Glob(filepath.Join(os.Getenv("APPDATA"), `Mozilla\Firefox\Profiles\*\prefs.js`))
	if len(profiles) > 0 {
		worst := 0
		for _, p := range profiles {
			if data, err := os.ReadFile(p); err == nil {
				if m := firefoxMode(data); m == 2 || m == 3 {
					worst = m
				}
			}
		}
		text, bypass := describeFirefox(worst)
		policy := false
		if enabled, ok := firefoxPolicy(); ok {
			policy = true
			if enabled {
				text, bypass = "DNS via HTTPS imposé par stratégie : contourne Aria", true
			} else {
				text, bypass = "DNS via HTTPS désactivé par stratégie : suit Windows", false
			}
		}
		found = append(found, Info{Name: "Firefox", Mode: text, Bypass: bypass, Policy: policy})
	}
	return found
}
