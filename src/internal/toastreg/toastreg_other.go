//go:build !windows

package toastreg

// Remove n'a rien à faire hors Windows.
func Remove(appName string) error { return nil }
