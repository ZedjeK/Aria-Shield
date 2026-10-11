//go:build !windows

package paths

import "os"

// Hors Windows (développement uniquement) : dossier réservé au propriétaire.
func lockDown(dir string) error { return os.Chmod(dir, 0o700) }
