//go:build !windows

package sysx

import (
	"os"
	"os/exec"
)

// hideWindow : pas de console à masquer hors Windows.
func hideWindow(cmd *exec.Cmd) {}

// IsAdmin : hors Windows (dev uniquement), on regarde si on est root.
func IsAdmin() bool {
	return os.Geteuid() == 0
}
