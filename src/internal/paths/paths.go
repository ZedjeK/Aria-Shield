// Package paths décide où Aria range ses fichiers :
//   - %USERPROFILE%\.Aria-Shield : données de l'utilisateur (profils VPN chiffrés,
//     Tor Browser, cache de l'interface), comme les autres applis Aria ;
//   - %PROGRAMDATA%\AriaShield : ce qu'Aria exécute en administrateur (Unbound)
//     et l'état système (photo du DNS), dans un dossier où seuls les administrateurs
//     et le système peuvent écrire.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// appDir est le nom du dossier d'Aria dans PROGRAMDATA (et dans LOCALAPPDATA jusqu'à la v0.8).
const appDir = "AriaShield"

// homeDir est le dossier des données dans le profil de l'utilisateur.
const homeDir = ".Aria-Shield"

// LegacyDirName est l'ancien dossier, créé à côté de l'exe jusqu'à la v0.6.1.
const LegacyDirName = "AriaShield-data"

// envOr lit une variable d'environnement, avec une valeur de repli (tests hors Windows).
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// UserDir renvoie %USERPROFILE%\.Aria-Shield (créé si besoin). Au premier
// lancement, l'ancien dossier %LOCALAPPDATA%\AriaShield y est déplacé (moved).
// Si le déplacement échoue (Tor Browser ouvert, par exemple), ou si
// .Aria-Shield est un lien plutôt qu'un vrai dossier, l'ancien dossier sert
// encore pour cette fois et l'erreur est renvoyée.
func UserDir() (dir string, moved bool, err error) {
	home, herr := os.UserHomeDir()
	if herr != nil {
		home = filepath.Join(os.TempDir(), "Profil")
	}
	dir = filepath.Join(home, homeDir)
	old := filepath.Join(envOr("LOCALAPPDATA", filepath.Join(os.TempDir(), "LocalAppData")), appDir)
	if moved, err = migrateUserDir(old, dir); err != nil {
		return old, false, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return old, false, errors.Join(err, os.MkdirAll(old, 0o755))
	}
	// Aria écrit ici en administrateur : jamais à travers un lien (jonction)
	// qu'un autre programme aurait posé pour la rediriger ailleurs.
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return old, false, errors.Join(fmt.Errorf("%s n'est pas un vrai dossier (lien ?) : supprime-le puis relance Aria", dir),
			os.MkdirAll(old, 0o755))
	}
	return dir, moved, nil
}

// migrateUserDir renomme old en dir si old est un vrai dossier et que dir
// n'existe pas encore (si les deux existent, dir est gardé tel quel). Un simple
// renommage : rien n'est copié, donc rien ne peut rester à moitié déplacé.
func migrateUserDir(old, dir string) (bool, error) {
	st, err := os.Lstat(old)
	if err != nil || !st.IsDir() { // absent, ou lien : rien à déplacer
		return false, nil
	}
	if _, err := os.Lstat(dir); err == nil {
		return false, nil
	}
	if err := os.Rename(old, dir); err != nil {
		return false, err
	}
	return true, nil
}

// WebviewDir est le profil WebView2 de l'interface, rangé avec les données.
func WebviewDir(userDir string) string { return filepath.Join(userDir, "webview") }

// SystemDir renvoie %PROGRAMDATA%\AriaShield, créé si besoin et verrouillé
// (écriture réservée aux administrateurs et au système). aside est le nom d'un
// dossier suspect, qu'Aria n'avait pas créé, mis de côté à la place ("" sinon).
func SystemDir() (dir, aside string, err error) {
	dir = filepath.Join(envOr("ProgramData", filepath.Join(os.TempDir(), "ProgramData")), appDir)
	aside, err = prepareSystemDir(dir)
	return dir, aside, err
}

// LegacyDir renvoie l'ancien dossier à côté de l'exe, s'il existe.
func LegacyDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Join(filepath.Dir(exe), LegacyDirName)
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}
