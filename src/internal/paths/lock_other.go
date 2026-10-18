//go:build !windows

package paths

import "os"

// Hors Windows (développement uniquement) : dossier réservé au propriétaire.
func prepareSystemDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return "", os.Chmod(dir, 0o700)
}
