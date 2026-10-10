//go:build windows

package browsers

import "golang.org/x/sys/windows/registry"

// chromiumPolicy lit DnsOverHttpsMode imposé par stratégie (machine puis utilisateur).
func chromiumPolicy(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, `SOFTWARE\Policies\`+key, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("DnsOverHttpsMode")
		k.Close()
		if err == nil && v != "" {
			return v, true
		}
	}
	return "", false
}

// firefoxPolicy lit DNSOverHTTPS\Enabled imposé par stratégie.
func firefoxPolicy() (enabled bool, ok bool) {
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, `SOFTWARE\Policies\Mozilla\Firefox\DNSOverHTTPS`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetIntegerValue("Enabled")
		k.Close()
		if err == nil {
			return v != 0, true
		}
	}
	return false, false
}
