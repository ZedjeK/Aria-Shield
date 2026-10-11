package wgnt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/driver/memmod"
)

// Pilote officiel WireGuardNT 1.1 (wireguard-nt-1.1.zip de download.wireguard.com),
// signé « WireGuard LLC ». Aria le charge directement en mémoire depuis l'exe :
// aucun wireguard.dll n'est déposé sur le disque.
//
//go:embed wireguard.dll
var embeddedDLL []byte

// DLLSHA256 est l'empreinte attendue de wireguard.dll.
const DLLSHA256 = "b1b85e072c45d81358be29d94c599dc76652f912be8c0f0a41e2d5d89a6461d3"

// lazyDLL remplace le chargement depuis un fichier (paquet d'origine) par un
// chargement en mémoire, comme le fait wireguard-windows avec ses ressources.
type lazyDLL struct {
	Name   string
	Base   windows.Handle
	mu     sync.Mutex
	module *memmod.Module
	onLoad func(d *lazyDLL)
}

func (d *lazyDLL) Load() error {
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&d.module))) != nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.module != nil {
		return nil
	}

	sum := sha256.Sum256(embeddedDLL)
	if hex.EncodeToString(sum[:]) != DLLSHA256 {
		return fmt.Errorf("wireguard.dll embarqué altéré")
	}
	module, err := memmod.LoadLibrary(embeddedDLL)
	if err != nil {
		return fmt.Errorf("chargement de WireGuardNT impossible : %w", err)
	}
	d.Base = windows.Handle(module.BaseAddr())

	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&d.module)), unsafe.Pointer(module))
	if d.onLoad != nil {
		d.onLoad(d)
	}
	return nil
}

func (p *lazyProc) nameToAddr() (uintptr, error) {
	return p.dll.module.ProcAddressByName(p.Name)
}
