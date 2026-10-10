//go:build !windows

package browsers

func chromiumPolicy(key string) (string, bool) { return "", false }
func firefoxPolicy() (bool, bool)              { return false, false }
