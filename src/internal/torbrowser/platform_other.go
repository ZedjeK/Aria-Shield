//go:build !windows

package torbrowser

import "errors"

var errWindowsOnly = errors.New("disponible uniquement sous Windows")

func runInstaller(installer, target string) error { return errWindowsOnly }
func running(dir string) bool                     { return false }
func launchUnelevated(exe, dir string) error      { return errWindowsOnly }
