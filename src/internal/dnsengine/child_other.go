//go:build !windows

package dnsengine

import "os/exec"

// Hors Windows (développement uniquement) : pas de console à masquer ni de job.
func prepareChild(cmd *exec.Cmd)       {}
func bindToParent(cmd *exec.Cmd) error { return nil }
func portOwner() string                { return "" }
