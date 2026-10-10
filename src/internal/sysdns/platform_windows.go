//go:build windows

package sysdns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"

	"aria-shield/internal/sysx"
)

const (
	psTimeout  = 30 * time.Second
	policyKey  = `SOFTWARE\Policies\Microsoft\Windows NT\DNSClient`
	policyName = "DisableSmartNameResolution"
	tcpip4Key  = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\`
	tcpip6Key  = `SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters\Interfaces\`
)

const listScript = `
$ErrorActionPreference = 'Stop'
$out = foreach ($a in @(Get-NetAdapter | Where-Object { $_.Status -eq 'Up' })) {
  $g = [string]$a.InterfaceGuid
  $v4 = (Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue).NameServer
  $v6 = (Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters\Interfaces\$g" -Name NameServer -ErrorAction SilentlyContinue).NameServer
  $srv = @(Get-DnsClientServerAddress -InterfaceIndex $a.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { $_.ServerAddresses })
  [pscustomobject]@{ index = [int]$a.ifIndex; alias = [string]$a.Name; guid = $g; staticV4 = [string]$v4; staticV6 = [string]$v6; servers = @($srv) }
}
if ($null -eq $out) { '[]' } else { ConvertTo-Json -InputObject @($out) -Depth 4 -Compress }
`

type rawAdapter struct {
	Index    int      `json:"index"`
	Alias    string   `json:"alias"`
	GUID     string   `json:"guid"`
	StaticV4 string   `json:"staticV4"`
	StaticV6 string   `json:"staticV6"`
	Servers  []string `json:"servers"`
}

// splitServers découpe la valeur de registre NameServer (virgules ou espaces).
func splitServers(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
}

func listAdapters() ([]Adapter, error) {
	out, err := sysx.PowerShell(psTimeout, listScript)
	if err != nil {
		return nil, fmt.Errorf("%v : %s", err, firstLine(out))
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var raw []rawAdapter
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("lecture des cartes : %w", err)
	}
	adapters := make([]Adapter, 0, len(raw))
	for _, r := range raw {
		if r.Index <= 0 {
			continue
		}
		adapters = append(adapters, Adapter{
			Index: r.Index, Alias: r.Alias, GUID: r.GUID,
			StaticV4: splitServers(r.StaticV4), StaticV6: splitServers(r.StaticV6),
			Servers: r.Servers,
		})
	}
	return adapters, nil
}

func psList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

func pointToLoopback(index int, ipv6 bool) error {
	list := []string{loopback4}
	if ipv6 {
		list = append(list, loopback6)
	}
	script := fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses %s -ErrorAction Stop`, index, psList(list))
	out, err := sysx.PowerShell(psTimeout, script)
	if err != nil && ipv6 {
		// Carte sans IPv6 : on se contente de l'IPv4.
		out, err = sysx.PowerShell(psTimeout, fmt.Sprintf(
			`Set-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses %s -ErrorAction Stop`, index, psList([]string{loopback4})))
	}
	if err != nil {
		return errors.New(firstLine(out))
	}
	return nil
}

// restoreAdapter remet une carte comme sur la photo : DNS automatique, ou serveurs fixes.
func restoreAdapter(a Adapter) (string, error) {
	static := append(append([]string{}, a.StaticV4...), a.StaticV6...)
	script := fmt.Sprintf(`Set-DnsClientServerAddress -InterfaceIndex %d -ResetServerAddresses -ErrorAction Stop`, a.Index)
	if len(static) > 0 {
		script += fmt.Sprintf("\nSet-DnsClientServerAddress -InterfaceIndex %d -ServerAddresses %s -ErrorAction Stop", a.Index, psList(static))
	}
	note := "DNS automatique"
	if len(static) > 0 {
		note = "serveurs fixes : " + strings.Join(static, ", ")
	}
	if _, err := sysx.PowerShell(psTimeout, script); err == nil {
		return note, nil
	}
	// Carte débranchée ou disparue : on corrige directement le registre,
	// la carte retrouvera son DNS d'origine à son retour.
	if a.GUID == "" {
		return "", errors.New("carte introuvable")
	}
	if err := writeNameServer(tcpip4Key+a.GUID, strings.Join(a.StaticV4, ",")); err != nil {
		return "", err
	}
	if err := writeNameServer(tcpip6Key+a.GUID, strings.Join(a.StaticV6, ",")); err != nil {
		return "", err
	}
	return note + " (via le registre)", nil
}

func writeNameServer(path, value string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer k.Close()
	return k.SetStringValue("NameServer", value)
}

func smartResolutionPolicy() (bool, uint32, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, policyKey, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, 0, nil
		}
		return false, 0, err
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(policyName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return true, uint32(v), nil
}

func setSmartResolutionOff() error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, policyKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue(policyName, 1)
}

func restoreSmartResolution(existed bool, value uint32) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, policyKey, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer k.Close()
	if existed {
		return k.SetDWordValue(policyName, value)
	}
	if err := k.DeleteValue(policyName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func flushCache() {
	_, _ = sysx.PowerShell(psTimeout, `Clear-DnsClientCache`)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
