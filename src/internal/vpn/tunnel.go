package vpn

// Types communs à toutes les plateformes ; le tunnel lui-même est dans
// tunnel_windows.go (et un bouchon dans tunnel_other.go).

import "time"

// AdapterName est le nom de la carte virtuelle créée par Aria.
const AdapterName = "Aria"

// Stats décrit l'état du tunnel pour l'interface.
type Stats struct {
	Up           bool   `json:"up"`   // la carte « Aria » existe
	Full         bool   `json:"full"` // routes + kill switch (sinon : simple test de connexion)
	KillSwitch   bool   `json:"killSwitch"`
	Endpoint     string `json:"endpoint"`
	HandshakeAgo int64  `json:"handshakeAgo"` // secondes, -1 si jamais
	RxBytes      uint64 `json:"rxBytes"`
	TxBytes      uint64 `json:"txBytes"`
}

// Fresh indique si la dernière poignée de main est récente (WireGuard la
// renouvelle toutes les 2 minutes tant que le tunnel sert).
func (s Stats) Fresh() bool {
	return s.Up && s.HandshakeAgo >= 0 && time.Duration(s.HandshakeAgo)*time.Second < 3*time.Minute
}
