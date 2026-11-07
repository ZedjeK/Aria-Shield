// Package paths décide où Aria range ses fichiers :
//   - %LOCALAPPDATA%\AriaShield : données de l'utilisateur (profils VPN chiffrés, Tor Browser) ;
//   - %PROGRAMDATA%\AriaShield : ce qu'Aria exécute en administrateur (Unbound)
//     et l'état système (photo du DNS), dans un dossier où seuls les administrateurs
//     et le système peuvent écrire.
package paths

import (
	"os"
	"path/filepath"
)

// appDir est le nom du dossier d'Aria dans LOCALAPPDATA et PROGRAMDATA.
const appDir = "AriaShield"

// LegacyDirName est l'ancien dossier, créé à côté de l'exe jusqu'à la v0.6.1.
const LegacyDirName = "AriaShield-data"

// envOr lit une variable d'environnement, avec une valeur de repli (tests hors Windows).
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// UserDir renvoie %LOCALAPPDATA%\AriaShield (créé si besoin).
func UserDir() (string, error) {
	dir := filepath.Join(envOr("LOCALAPPDATA", filepath.Join(os.TempDir(), "LocalAppData")), appDir)
	return dir, os.MkdirAll(dir, 0o755)
}

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
