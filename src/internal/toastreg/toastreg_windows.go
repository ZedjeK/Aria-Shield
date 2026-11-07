//go:build windows

package toastreg

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Mêmes chemins que le service de notifications de Wails (v3.0.0-beta.28).
const (
	aumidPath   = `Software\Classes\AppUserModelId\`
	clsidPath   = `Software\Classes\CLSID\`
	activator   = "CustomActivator"
	appRootPath = `Software\`
)

// Un GUID entre accolades : rien d'autre n'est jamais effacé sous CLSID.
var guidRe = regexp.MustCompile(`^\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}$`)

// Remove efface les clés de notification de l'appli appName. Une clé absente
// n'est pas une erreur.
func Remove(appName string) error {
	var errs []error
	del := func(path string) {
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	guid := ""
	if k, err := registry.OpenKey(registry.CURRENT_USER, aumidPath+appName, registry.QUERY_VALUE); err == nil {
		guid, _, _ = k.GetStringValue(activator)
		k.Close()
	}
	if guidRe.MatchString(guid) {
		del(clsidPath + guid + `\LocalServer32`)
		del(clsidPath + guid)
		os.Remove(filepath.Join(os.TempDir(), appName+guid+".png"))
	}
	// go-toast inscrit aussi son exe sous un GUID fixe, le même pour toutes les
	// applis qui l'utilisent (il ne tient pas compte du GUID propre à l'appli) :
	// on ne l'efface que s'il désigne bien notre exe.
	if pointsTo(clsidPath+goToastGUID+`\LocalServer32`, exeName) {
		del(clsidPath + goToastGUID + `\LocalServer32`)
		del(clsidPath + goToastGUID)
	}
	del(aumidPath + appName)
	del(appRootPath + appName + `\NotificationCategories`)
	del(appRootPath + appName)
	return errors.Join(errs...)
}

// goToastGUID : GUID par défaut de go-toast v2 (wintoast/impl.go).
const goToastGUID = "{0F82E845-CB89-4039-BDBF-67CA33254C76}"

// exeName : nom de l'exe d'Aria, quel que soit son dossier.
const exeName = "Aria-Shield.exe"

// pointsTo indique si la valeur par défaut de la clé désigne un exe nommé exe.
func pointsTo(path, exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	if err != nil {
		return false
	}
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, `"`) {
		if end := strings.Index(v[1:], `"`); end >= 0 {
			v = v[1 : 1+end]
		}
	} else if i := strings.Index(strings.ToLower(v), ".exe"); i >= 0 {
		v = v[:i+4]
	}
	return strings.EqualFold(filepath.Base(v), exe)
}
