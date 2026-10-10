//go:build !windows

package vpn

import "errors"

var errWindowsOnly = errors.New("disponible uniquement sous Windows")

func protect(plain []byte) ([]byte, error)  { return nil, errWindowsOnly }
func unprotect(data []byte) ([]byte, error) { return nil, errWindowsOnly }
