//go:build !windows

package vpn

import (
	"net/netip"
	"time"
)

// Tunnel : hors Windows, rien n'est disponible (développement uniquement).
type Tunnel struct{}

func NewTunnel() *Tunnel { return &Tunnel{} }

func (t *Tunnel) Up(p Profile, full bool, dns4, dns6 []netip.Addr) error { return errWindowsOnly }
func (t *Tunnel) Down()                                                  {}
func (t *Tunnel) Stats() Stats                                           { return Stats{HandshakeAgo: -1} }
func (t *Tunnel) WaitHandshake(timeout time.Duration) bool               { return false }
