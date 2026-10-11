//go:build windows

package vpn

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/conf"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"

	"aria-shield/internal/killswitch"
	driver "aria-shield/internal/wgnt"
)

const (
	tunnelType = "Aria Shield"
	tunnelMTU  = 1420
)

// GUID fixe : Windows reconnaît la carte « Aria » d'une session à l'autre.
var adapterGUID = windows.GUID{Data1: 0x41524941, Data2: 0x5348, Data3: 0x4c44, Data4: [8]byte{0x8a, 0x1e, 0x57, 0x0c, 0x3d, 0x62, 0x91, 0x04}}

// Tunnel pilote la carte WireGuard d'Aria.
type Tunnel struct {
	mu       sync.Mutex
	adapter  *driver.Adapter
	luid     winipcfg.LUID
	full     bool
	firewall bool
	endpoint string
}

// NewTunnel prépare le pilote du tunnel (rien n'est lancé).
func NewTunnel() *Tunnel { return &Tunnel{} }

// buildConfig traduit le profil en configuration WireGuard, tunnel complet imposé.
func buildConfig(p Profile) (*conf.Config, error) {
	priv, err := conf.NewPrivateKeyFromString(p.PrivateKey)
	if err != nil {
		return nil, errors.New("clé privée invalide")
	}
	pub, err := conf.NewPrivateKeyFromString(p.PublicKey)
	if err != nil {
		return nil, errors.New("clé publique du serveur invalide")
	}
	host, portStr, err := net.SplitHostPort(p.Endpoint)
	if err != nil {
		return nil, errors.New("point d'accès invalide")
	}
	port, _ := strconv.Atoi(portStr)

	var addrs []netip.Prefix
	for _, a := range p.Addresses {
		pre, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("adresse invalide : %s", a)
		}
		addrs = append(addrs, pre)
	}

	cfg := &conf.Config{
		Name: AdapterName,
		Interface: conf.Interface{
			PrivateKey: *priv,
			Addresses:  addrs,
			MTU:        tunnelMTU,
		},
		Peers: []conf.Peer{{
			PublicKey:           *pub,
			AllowedIPs:          []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")},
			Endpoint:            conf.Endpoint{Host: host, Port: uint16(port)},
			PersistentKeepalive: uint16(p.Keepalive),
		}},
	}
	if err := cfg.ResolveEndpoints(); err != nil {
		return nil, fmt.Errorf("point d'accès introuvable : %w", err)
	}
	return cfg, nil
}

// Up ouvre le tunnel. Avec full, tout le trafic y passe, son DNS pointe sur le
// résolveur local (dns4/dns6) et le kill switch bloque tout le reste.
func (t *Tunnel) Up(p Profile, full bool, dns4, dns6 []netip.Addr) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.adapter != nil {
		return errors.New("le tunnel est déjà ouvert")
	}
	// Le paquet driver panique si wireguard.dll ne se charge pas : on le transforme en erreur.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pilote WireGuard indisponible : %v", r)
			t.downLocked()
		}
	}()
	cfg, err := buildConfig(p)
	if err != nil {
		return err
	}

	adapter, err := driver.CreateAdapter(AdapterName, tunnelType, &adapterGUID)
	if err != nil {
		// Une carte « Aria » orpheline (session précédente) : on la ferme et on recommence.
		if old, oerr := driver.OpenAdapter(AdapterName); oerr == nil {
			old.Close()
			adapter, err = driver.CreateAdapter(AdapterName, tunnelType, &adapterGUID)
		}
		if err != nil {
			return fmt.Errorf("création de la carte « %s » impossible : %w", AdapterName, err)
		}
	}
	t.adapter = adapter
	defer func() {
		if err != nil {
			t.downLocked()
		}
	}()

	drv, size := toDriverConfiguration(cfg)
	if err = adapter.SetConfiguration(drv, size); err != nil {
		return fmt.Errorf("configuration du tunnel refusée : %w", err)
	}

	t.luid = adapter.LUID()
	if err = configureFamily(t.luid, windows.AF_INET, cfg, full, dns4); err != nil {
		return err
	}
	if err = configureFamily(t.luid, windows.AF_INET6, cfg, full, dns6); err != nil {
		return err
	}

	if err = adapter.SetAdapterState(driver.AdapterStateUp); err != nil {
		return fmt.Errorf("activation de la carte impossible : %w", err)
	}

	if full {
		if err = killswitch.EnableFirewall(uint64(t.luid), false, nil); err != nil {
			return fmt.Errorf("kill switch impossible à activer : %w", err)
		}
		t.firewall = true
	}
	t.full = full
	t.endpoint = p.Endpoint
	return nil
}

// configureFamily pose adresses, routes, métrique, MTU et DNS pour une famille IP.
func configureFamily(luid winipcfg.LUID, family winipcfg.AddressFamily, cfg *conf.Config, full bool, dns []netip.Addr) error {
	var addrs []netip.Prefix
	for _, a := range cfg.Interface.Addresses {
		if (family == windows.AF_INET) == a.Addr().Is4() {
			addrs = append(addrs, a)
		}
	}

	ipif, err := luid.IPInterface(family)
	if err != nil {
		if family == windows.AF_INET6 {
			return nil // IPv6 désactivé sur la machine : rien à configurer
		}
		return fmt.Errorf("lecture de la carte : %w", err)
	}

	if full {
		def := netip.MustParsePrefix("0.0.0.0/0")
		hop := netip.IPv4Unspecified()
		if family == windows.AF_INET6 {
			def, hop = netip.MustParsePrefix("::/0"), netip.IPv6Unspecified()
		}
		// Même sans adresse IPv6 dans le tunnel, la route IPv6 y part : l'IPv6
		// est alors avalé par le tunnel au lieu de fuir par la box.
		if err := luid.SetRoutesForFamily(family, []*winipcfg.RouteData{{Destination: def, NextHop: hop, Metric: 0}}); err != nil {
			return fmt.Errorf("routes impossibles à poser : %w", err)
		}
	}
	if len(addrs) > 0 {
		if err := luid.SetIPAddressesForFamily(family, addrs); err != nil {
			return fmt.Errorf("adresses impossibles à poser : %w", err)
		}
	}

	ipif.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
	ipif.DadTransmits = 0
	ipif.ManagedAddressConfigurationSupported = false
	ipif.OtherStatefulConfigurationSupported = false
	ipif.NLMTU = tunnelMTU
	if family == windows.AF_INET6 && ipif.NLMTU < 1280 {
		ipif.NLMTU = 1280
	}
	if full {
		ipif.UseAutomaticMetric = false
		ipif.Metric = 0
	}
	if err := ipif.Set(); err != nil {
		return fmt.Errorf("métrique et MTU impossibles à poser : %w", err)
	}

	if full && len(dns) > 0 {
		if err := luid.SetDNS(family, dns, nil); err != nil {
			return fmt.Errorf("DNS du tunnel impossible à poser : %w", err)
		}
	}
	return nil
}

// Down ferme le tunnel, retire le kill switch et désinstalle le pilote s'il ne sert plus.
func (t *Tunnel) Down() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.downLocked()
}

func (t *Tunnel) downLocked() {
	if t.firewall {
		killswitch.DisableFirewall()
		t.firewall = false
	}
	if t.adapter != nil {
		_ = t.adapter.SetAdapterState(driver.AdapterStateDown)
		_ = t.adapter.Close()
		t.adapter = nil
		// Rien ne doit rester installé : le pilote est retiré s'il ne sert plus.
		_ = driver.Uninstall()
	}
	t.full = false
	t.endpoint = ""
}

// Stats renvoie l'état du tunnel.
func (t *Tunnel) Stats() Stats {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := Stats{HandshakeAgo: -1}
	if t.adapter == nil {
		return s
	}
	s.Up, s.Full, s.KillSwitch, s.Endpoint = true, t.full, t.firewall, t.endpoint
	iface, err := t.adapter.Configuration()
	if err != nil || iface.PeerCount == 0 {
		return s
	}
	peer := iface.FirstPeer()
	s.RxBytes, s.TxBytes = peer.RxBytes, peer.TxBytes
	if peer.LastHandshake != 0 {
		// FILETIME : centaines de nanosecondes depuis le 1er janvier 1601.
		const epochDiff = 116444736000000000
		hs := time.Unix(0, int64(peer.LastHandshake-epochDiff)*100)
		s.HandshakeAgo = int64(time.Since(hs).Seconds())
		if s.HandshakeAgo < 0 {
			s.HandshakeAgo = 0
		}
	}
	return s
}

// WaitHandshake attend la première poignée de main avec le serveur.
func (t *Tunnel) WaitHandshake(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s := t.Stats(); s.Up && s.HandshakeAgo >= 0 {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// toDriverConfiguration traduit la configuration pour WireGuardNT
// (reprise de conf.ToDriverConfiguration, avec les types du paquet wgnt).
func toDriverConfiguration(config *conf.Config) (*driver.Interface, uint32) {
	preallocation := unsafe.Sizeof(driver.Interface{}) + uintptr(len(config.Peers))*unsafe.Sizeof(driver.Peer{})
	for i := range config.Peers {
		preallocation += uintptr(len(config.Peers[i].AllowedIPs)) * unsafe.Sizeof(driver.AllowedIP{})
	}
	var c driver.ConfigBuilder
	c.Preallocate(uint32(preallocation))
	c.AppendInterface(&driver.Interface{
		Flags:      driver.InterfaceHasPrivateKey | driver.InterfaceHasListenPort,
		ListenPort: config.Interface.ListenPort,
		PrivateKey: config.Interface.PrivateKey,
		PeerCount:  uint32(len(config.Peers)),
	})
	for i := range config.Peers {
		flags := driver.PeerHasPublicKey | driver.PeerHasPersistentKeepalive
		if !config.Peers[i].PresharedKey.IsZero() {
			flags |= driver.PeerHasPresharedKey
		}
		var endpoint winipcfg.RawSockaddrInet
		if !config.Peers[i].Endpoint.IsEmpty() {
			if addr, err := netip.ParseAddr(config.Peers[i].Endpoint.Host); err == nil {
				flags |= driver.PeerHasEndpoint
				endpoint.SetAddrPort(netip.AddrPortFrom(addr, config.Peers[i].Endpoint.Port))
			}
		}
		c.AppendPeer(&driver.Peer{
			Flags:               flags,
			PublicKey:           config.Peers[i].PublicKey,
			PresharedKey:        config.Peers[i].PresharedKey,
			PersistentKeepalive: config.Peers[i].PersistentKeepalive,
			Endpoint:            endpoint,
			AllowedIPsCount:     uint32(len(config.Peers[i].AllowedIPs)),
		})
		for j := range config.Peers[i].AllowedIPs {
			a := &driver.AllowedIP{Cidr: uint8(config.Peers[i].AllowedIPs[j].Bits())}
			copy(a.Address[:], config.Peers[i].AllowedIPs[j].Addr().AsSlice())
			if config.Peers[i].AllowedIPs[j].Addr().Is4() {
				a.AddressFamily = windows.AF_INET
			} else if config.Peers[i].AllowedIPs[j].Addr().Is6() {
				a.AddressFamily = windows.AF_INET6
			}
			c.AppendAllowedIP(a)
		}
	}
	return c.Interface()
}
