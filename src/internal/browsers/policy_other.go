//go:build !windows

package browsers

// Hors Windows : pas de stratégies d'entreprise dans le registre.
func chromiumPolicy(key string) (string, bool) { return "", false }
func firefoxPolicy() (bool, bool)              { return false, false }
