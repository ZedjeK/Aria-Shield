//go:build windows

package sysdns

// Partie Windows : lecture des cartes et réglage de leurs DNS par PowerShell
// (les cmdlets DnsClient/NetAdapter), et accès direct au registre quand une
// carte n'est plus là pour recevoir la commande.

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
	psTimeout = 30 * time.Second
	// Stratégie « Désactiver la résolution de noms multi-cartes intelligente ».
	// Sans elle, Windows interroge en parallèle les DNS de toutes les cartes et
	// garde la réponse la plus rapide : celle du FAI pourrait passer avant Aria.
	policyKey  = `SOFTWARE\Policies\Microsoft\Windows NT\DNSClient`
	policyName = "DisableSmartNameResolution"
	tcpip4Key  = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\`
	tcpip6Key  = `SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters\Interfaces\`
)

// listScript liste les cartes actives en JSON. Pour chacune : les DNS fixés à
// la main (lus dans le registre, vides si DNS automatique) et les serveurs
// réellement utilisés à cet instant.
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

// rawAdapter est une carte telle que la décrit listScript.
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

// listAdapters renvoie les cartes réseau actives et leur configuration DNS.
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

// psList met en forme une liste PowerShell. Ne lui passer que des adresses
// validées par canonicalAddrs : le doublement de ' ne suffit pas à lui seul.
func psList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

// pointToLoopback fait utiliser à une carte le résolveur local (127.0.0.1, et
// ::1 si Unbound écoute en IPv6).
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
	// La photo vient d'un fichier : rien n'en part vers PowerShell ou le registre sans validation.
	v4, err := canonicalAddrs(a.StaticV4)
	if err != nil {
		return "", err
	}
	v6, err := canonicalAddrs(a.StaticV6)
	if err != nil {
		return "", err
	}
	static := append(append([]string{}, v4...), v6...)
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
	if !validGUID(a.GUID) {
		return "", fmt.Errorf("identifiant de carte invalide : %q", a.GUID)
	}
	if err := writeNameServer(tcpip4Key+a.GUID, strings.Join(v4, ",")); err != nil {
		return "", err
	}
	if err := writeNameServer(tcpip6Key+a.GUID, strings.Join(v6, ",")); err != nil {
		return "", err
	}
	return note + " (via le registre)", nil
}

// writeNameServer écrit la valeur NameServer d'une carte dans le registre.
// Une clé absente (carte jamais configurée pour cette famille) n'est pas une erreur.
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

// smartResolutionPolicy lit la stratégie actuelle : existe-t-elle, et avec quelle valeur.
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

// setSmartResolutionOff force Windows à n'interroger que le DNS de la carte prioritaire.
func setSmartResolutionOff() error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, policyKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue(policyName, 1)
}

// restoreSmartResolution remet la stratégie comme avant : sa valeur d'origine,
// ou son absence.
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

// flushCache vide le cache DNS de Windows, pour que le changement prenne effet
// tout de suite plutôt qu'à l'expiration des anciennes réponses.
func flushCache() {
	_, _ = sysx.PowerShell(psTimeout, `Clear-DnsClientCache`)
}

// firstLine garde la première ligne d'une sortie PowerShell (le message d'erreur utile).
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
