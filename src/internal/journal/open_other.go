//go:build !windows

package journal

import (
	"fmt"
	"os"
)

// openNoFollow, hors Windows (tests) : refuse un lien, puis ouvre en ajout.
func openNoFollow(path string) (*os.File, error) {
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s n'est pas un fichier ordinaire (lien ?) : journal désactivé", path)
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
