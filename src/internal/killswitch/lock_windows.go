//go:build windows

package killswitch

// Verrou persistant (Aria Shield, lot 7, variante A).
//
// Le kill switch de rules.go vit dans une session WFP dynamique : Windows le
// retire dès qu'Aria s'arrête, plantage compris. Le verrou est son pendant
// persistant : des filtres gardés par Windows d'un démarrage à l'autre, qui
// bloquent tout sauf
//
//   - l'exe d'Aria désigné par son chemin (la copie de Program Files, que seul
//     un administrateur peut modifier), pour joindre le serveur VPN ;
//   - la boucle locale, DHCP et NDP, pour que la carte réseau reste configurée.
//
// Il ne laisse pas passer la carte « Aria » : elle est recréée à chaque
// ouverture du tunnel, son identifiant change. Quand le tunnel est monté, le
// kill switch dynamique autorise les sorties par le tunnel « en dur »
// (FWPM_FILTER_FLAG_CLEAR_ACTION_RIGHT, voir SetHardTunnel), dans une
// sous-couche de poids plus élevé : le blocage du verrou ne peut pas les
// annuler. Si Aria s'arrête sans retirer le verrou (plantage, coupure,
// redémarrage), tout reste bloqué jusqu'à ce qu'elle remonte le tunnel.
//
// Fournisseur, sous-couche et filtres ont des clés fixes : DisableLock les
// retrouve et les retire même sans savoir ce qui a été posé.

import (
	"errors"
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	lockProviderKey = windows.GUID{Data1: 0xf77dc011, Data2: 0x7581, Data3: 0x4d38, Data4: [8]byte{0xbd, 0xe6, 0xcc, 0xe4, 0x78, 0xbb, 0x9d, 0xfb}}
	lockSublayerKey = windows.GUID{Data1: 0xfef14eb2, Data2: 0x878e, Data3: 0x456c, Data4: [8]byte{0xa1, 0x2e, 0x9f, 0xf7, 0x1d, 0x12, 0x54, 0xc5}}
	// lockFilterBase : la clé du filtre n° i est cette clé, dernier octet = i.
	lockFilterBase = windows.GUID{Data1: 0xdbc272ca, Data2: 0xe113, Data3: 0x4671, Data4: [8]byte{0xbd, 0x73, 0xbb, 0xa4, 0xaf, 0x07, 0xc3, 0x00}}
)

// maxLockFilters : nombre de clés de filtre réservées au verrou (il en pose
// une vingtaine) ; DisableLock les essaie toutes.
const maxLockFilters = 64

// Poids de la sous-couche du verrou : juste sous celle du kill switch
// dynamique (^uint16(0)), pour que ses autorisations « dures » passent devant.
const lockSublayerWeight = ^uint16(0) - 1

const cFWPM_PROVIDER_FLAG_PERSISTENT = 0x00000001

// Erreurs WFP « objet absent » (fwpmu.h), ignorées au retrait du verrou.
const (
	errFwpFilterNotFound   syscall.Errno = 0x80320003
	errFwpProviderNotFound syscall.Errno = 0x80320005
	errFwpSublayerNotFound syscall.Errno = 0x80320007
)

// hardTunnelPermits : les prochains EnableFirewall autorisent les sorties par
// le tunnel en dur. Sans verrou, c'est inutile : on garde le comportement de
// WireGuard, où le pare-feu de Windows peut encore bloquer une appli dans le tunnel.
var hardTunnelPermits atomic.Bool

// SetHardTunnel règle les autorisations du tunnel pour les prochains
// EnableFirewall : en dur quand le verrou est voulu, simples sinon.
func SetHardTunnel(on bool) { hardTunnelPermits.Store(on) }

// lockFilterKey renvoie la clé du filtre n° i du verrou.
func lockFilterKey(i int) windows.GUID {
	k := lockFilterBase
	k.Data4[7] = byte(i)
	return k
}

// lockKeys distribue les clés de filtre du verrou, dans l'ordre.
func lockKeys() func() (windows.GUID, error) {
	next := 0
	return func() (windows.GUID, error) {
		if next >= maxLockFilters {
			return windows.GUID{}, fmt.Errorf("verrou : plus de %d filtres", maxLockFilters)
		}
		k := lockFilterKey(next)
		next++
		return k, nil
	}
}

// openLockSession ouvre une session WFP non dynamique : ce qu'elle pose avec
// le drapeau « persistant » reste en place après sa fermeture et au redémarrage.
func openLockSession() (uintptr, error) {
	displayData, err := createWtFwpmDisplayData0("Aria Shield", "Verrou persistant d'Aria Shield")
	if err != nil {
		return 0, err
	}
	session := wtFwpmSession0{
		displayData:          *displayData,
		txnWaitTimeoutInMSec: windows.INFINITE,
	}
	handle := uintptr(0)
	if err := fwpmEngineOpen0(nil, cRPC_C_AUTHN_WINNT, nil, &session, unsafe.Pointer(&handle)); err != nil {
		return 0, fmt.Errorf("ouverture du pare-feu (WFP) impossible : %w", err)
	}
	return handle, nil
}

// EnableLock pose (ou remplace) le verrou persistant. appPath est l'exe qui
// pourra sortir sous verrou : il doit être dans un dossier que seul un
// administrateur peut modifier. Retrait et pose se font dans une seule
// transaction : pas d'instant sans règle.
func EnableLock(appPath string) error {
	session, err := openLockSession()
	if err != nil {
		return err
	}
	defer fwpmEngineClose0(session)
	return runTransaction(session, func(s uintptr) error {
		if err := removeLock(s); err != nil {
			return err
		}
		return installLock(s, appPath)
	})
}

// DisableLock retire le verrou persistant. Sans verrou posé, ne fait rien.
func DisableLock() error {
	session, err := openLockSession()
	if err != nil {
		return err
	}
	defer fwpmEngineClose0(session)
	return runTransaction(session, removeLock)
}

// LockInstalled indique si le verrou persistant est posé.
func LockInstalled() (bool, error) {
	session, err := openLockSession()
	if err != nil {
		return false, err
	}
	defer fwpmEngineClose0(session)
	var provider *wtFwpmProvider0
	err = fwpmProviderGetByKey0(session, &lockProviderKey, &provider)
	if errors.Is(err, errFwpProviderNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lecture du verrou impossible : %w", err)
	}
	fwpmFreeMemory0(unsafe.Pointer(&provider))
	return true, nil
}

// removeLock retire filtres, sous-couche et fournisseur du verrou, dans cet
// ordre (Windows refuse de retirer un objet encore utilisé). Un objet absent
// n'est pas une erreur.
func removeLock(session uintptr) error {
	for i := 0; i < maxLockFilters; i++ {
		key := lockFilterKey(i)
		if err := fwpmFilterDeleteByKey0(session, &key); err != nil && !errors.Is(err, errFwpFilterNotFound) {
			return fmt.Errorf("retrait du filtre %d du verrou : %w", i, err)
		}
	}
	if err := fwpmSubLayerDeleteByKey0(session, &lockSublayerKey); err != nil && !errors.Is(err, errFwpSublayerNotFound) {
		return fmt.Errorf("retrait de la sous-couche du verrou : %w", err)
	}
	if err := fwpmProviderDeleteByKey0(session, &lockProviderKey); err != nil && !errors.Is(err, errFwpProviderNotFound) {
		return fmt.Errorf("retrait du fournisseur du verrou : %w", err)
	}
	return nil
}

// installLock pose fournisseur, sous-couche et filtres du verrou, tous
// persistants. À appeler dans une transaction.
func installLock(session uintptr, appPath string) error {
	{
		displayData, err := createWtFwpmDisplayData0("Aria Shield", "Verrou persistant d'Aria Shield")
		if err != nil {
			return err
		}
		provider := wtFwpmProvider0{
			providerKey: lockProviderKey,
			displayData: *displayData,
			flags:       cFWPM_PROVIDER_FLAG_PERSISTENT,
		}
		if err := fwpmProviderAdd0(session, &provider, 0); err != nil {
			return fmt.Errorf("fournisseur du verrou refusé : %w", err)
		}
	}
	{
		displayData, err := createWtFwpmDisplayData0("Aria Shield, verrou", "Bloque tout sauf Aria, la boucle locale, DHCP et NDP")
		if err != nil {
			return err
		}
		sublayer := wtFwpmSublayer0{
			subLayerKey: lockSublayerKey,
			displayData: *displayData,
			flags:       cFWPM_SUBLAYER_FLAG_PERSISTENT,
			providerKey: &lockProviderKey,
			weight:      lockSublayerWeight,
		}
		if err := fwpmSubLayerAdd0(session, &sublayer, 0); err != nil {
			return fmt.Errorf("sous-couche du verrou refusée : %w", err)
		}
	}

	bo := &baseObjects{
		provider:    lockProviderKey,
		filters:     lockSublayerKey,
		filterFlags: cFWPM_FILTER_FLAG_PERSISTENT,
		nextKey:     lockKeys(),
	}
	steps := []struct {
		name string
		add  func() error
	}{
		{"Aria", func() error { return permitAppPath(session, bo, 15, appPath) }},
		{"boucle locale", func() error { return permitLoopback(session, bo, 13) }},
		{"DHCP IPv4", func() error { return permitDHCPIPv4(session, bo, 12) }},
		{"DHCP IPv6", func() error { return permitDHCPIPv6(session, bo, 12) }},
		{"NDP", func() error { return permitNdp(session, bo, 12) }},
		{"blocage", func() error { return blockAll(session, bo, 0) }},
	}
	for _, st := range steps {
		if err := st.add(); err != nil {
			return fmt.Errorf("verrou, règle « %s » refusée : %w", st.name, err)
		}
	}
	return nil
}

// permitAppPath autorise tout le trafic d'un exe désigné par son chemin. Pas
// de condition d'utilisateur comme dans permitWireGuardService : le verrou
// doit valoir avant l'ouverture de session.
func permitAppPath(session uintptr, bo *baseObjects, weight uint8, appPath string) error {
	path, err := windows.UTF16PtrFromString(appPath)
	if err != nil {
		return err
	}
	var appID *wtFwpByteBlob
	if err := fwpmGetAppIdFromFileName0(path, unsafe.Pointer(&appID)); err != nil {
		return fmt.Errorf("identifiant de %s introuvable : %w", appPath, err)
	}
	defer fwpmFreeMemory0(unsafe.Pointer(&appID))

	condition := wtFwpmFilterCondition0{
		fieldKey:  cFWPM_CONDITION_ALE_APP_ID,
		matchType: cFWP_MATCH_EQUAL,
		conditionValue: wtFwpConditionValue0{
			_type: cFWP_BYTE_BLOB_TYPE,
			value: uintptr(unsafe.Pointer(appID)),
		},
	}
	filter := wtFwpmFilter0{
		providerKey:         &bo.provider,
		subLayerKey:         bo.filters,
		weight:              filterWeight(weight),
		numFilterConditions: 1,
		filterCondition:     &condition,
		action:              wtFwpmAction0{_type: cFWP_ACTION_PERMIT},
	}
	layers := []struct {
		name  string
		layer windows.GUID
	}{
		{"Aria Shield, verrou : sorties d'Aria (IPv4)", cFWPM_LAYER_ALE_AUTH_CONNECT_V4},
		{"Aria Shield, verrou : entrées d'Aria (IPv4)", cFWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V4},
		{"Aria Shield, verrou : sorties d'Aria (IPv6)", cFWPM_LAYER_ALE_AUTH_CONNECT_V6},
		{"Aria Shield, verrou : entrées d'Aria (IPv6)", cFWPM_LAYER_ALE_AUTH_RECV_ACCEPT_V6},
	}
	filterID := uint64(0)
	for _, l := range layers {
		displayData, err := createWtFwpmDisplayData0(l.name, "")
		if err != nil {
			return err
		}
		filter.displayData = *displayData
		filter.layerKey = l.layer
		if err := addFilter(session, bo, &filter, &filterID); err != nil {
			return err
		}
	}
	return nil
}

// Appels WFP du verrou, écrits à la main (zsyscall_windows.go est généré).
// Ces fonctions renvoient le code d'erreur WFP lui-même, pour reconnaître
// « objet absent ».
var (
	procFwpmFilterDeleteByKey0   = modfwpuclnt.NewProc("FwpmFilterDeleteByKey0")
	procFwpmSubLayerDeleteByKey0 = modfwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmProviderDeleteByKey0 = modfwpuclnt.NewProc("FwpmProviderDeleteByKey0")
	procFwpmProviderGetByKey0    = modfwpuclnt.NewProc("FwpmProviderGetByKey0")
)

func wfpCall(proc *windows.LazyProc, args ...uintptr) error {
	if err := proc.Find(); err != nil {
		return err
	}
	r1, _, _ := syscall.SyscallN(proc.Addr(), args...)
	if r1 != 0 {
		return syscall.Errno(r1)
	}
	return nil
}

func fwpmFilterDeleteByKey0(session uintptr, key *windows.GUID) error {
	return wfpCall(procFwpmFilterDeleteByKey0, session, uintptr(unsafe.Pointer(key)))
}

func fwpmSubLayerDeleteByKey0(session uintptr, key *windows.GUID) error {
	return wfpCall(procFwpmSubLayerDeleteByKey0, session, uintptr(unsafe.Pointer(key)))
}

func fwpmProviderDeleteByKey0(session uintptr, key *windows.GUID) error {
	return wfpCall(procFwpmProviderDeleteByKey0, session, uintptr(unsafe.Pointer(key)))
}

func fwpmProviderGetByKey0(session uintptr, key *windows.GUID, provider **wtFwpmProvider0) error {
	return wfpCall(procFwpmProviderGetByKey0, session, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(provider)))
}
