//go:build !windows

package vpn

import "errors"

// Hors Windows, pas de DPAPI : on refuse d'écrire un profil en clair.
var errWindowsOnly = errors.New("disponible uniquement sous Windows")

func protect(plain []byte) ([]byte, error)  { return nil, errWindowsOnly }
func unprotect(data []byte) ([]byte, error) { return nil, errWindowsOnly }
