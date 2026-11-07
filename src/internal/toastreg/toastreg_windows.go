//go:build windows

package toastreg

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

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
	del(aumidPath + appName)
	del(appRootPath + appName + `\NotificationCategories`)
	del(appRootPath + appName)
	return errors.Join(errs...)
}
