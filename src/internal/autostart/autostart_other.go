//go:build !windows

package autostart

import "errors"

// Hors Windows (tests), le démarrage automatique n'existe pas.

var errUnsupported = errors.New("démarrage automatique réservé à Windows")

func Status() State { return State{} }

func Enable(current string) error { return errUnsupported }

func Disable(current string) error { return errUnsupported }

func Refresh(current string) (bool, error) { return false, nil }
