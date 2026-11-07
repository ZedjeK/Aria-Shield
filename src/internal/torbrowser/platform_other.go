//go:build !windows

package torbrowser

import "errors"

// Hors Windows (développement et tests) : ni installeur ni lancement.
var errWindowsOnly = errors.New("disponible uniquement sous Windows")

func runInstaller(installer, target string) error { return errWindowsOnly }
func running(dir string) bool                     { return false }
func launchUnelevated(exe, dir string) error      { return errWindowsOnly }
