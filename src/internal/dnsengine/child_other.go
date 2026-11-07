//go:build !windows

package dnsengine

import "os/exec"

// Hors Windows (développement uniquement) : pas de console à masquer, pas de
// job pour rattacher Unbound, et personne à nommer sur le port 53.
func prepareChild(cmd *exec.Cmd)       {}
func bindToParent(cmd *exec.Cmd) error { return nil }
func portOwner() string                { return "" }
