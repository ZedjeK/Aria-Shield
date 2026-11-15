//go:build windows

package killswitch

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// Les clés de filtre du verrou sont toutes distinctes, différentes de celles
// du fournisseur et de la sous-couche, et leur nombre est borné.
func TestLockKeys(t *testing.T) {
	next := lockKeys()
	seen := map[windows.GUID]bool{lockProviderKey: true, lockSublayerKey: true}
	for i := 0; i < maxLockFilters; i++ {
		k, err := next()
		if err != nil {
			t.Fatalf("clé %d : %v", i, err)
		}
		if seen[k] {
			t.Fatalf("clé %d en double : %v", i, k)
		}
		seen[k] = true
	}
	if _, err := next(); err == nil {
		t.Fatal("une clé de plus que maxLockFilters a été distribuée")
	}
}

func requireAdmin(t *testing.T) {
	t.Helper()
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("droits administrateur requis")
	}
	if on, err := LockInstalled(); err != nil {
		t.Fatal(err)
	} else if on {
		t.Skip("un verrou est déjà posé sur cette machine : on n'y touche pas")
	}
}

// Windows accepte toutes les règles du verrou. La transaction est annulée :
// rien n'est jamais appliqué, le réseau de la machine n'est pas bloqué.
func TestLockRulesAcceptedByWFP(t *testing.T) {
	requireAdmin(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	session, err := openLockSession()
	if err != nil {
		t.Fatal(err)
	}
	defer fwpmEngineClose0(session)
	if err := fwpmTransactionBegin0(session, 0); err != nil {
		t.Fatal(err)
	}
	defer fwpmTransactionAbort0(session)
	if err := installLock(session, exe); err != nil {
		t.Fatal(err)
	}
}

// Retirer un verrou absent ne fait rien et ne renvoie pas d'erreur.
func TestDisableLockWithoutLock(t *testing.T) {
	requireAdmin(t)
	if err := DisableLock(); err != nil {
		t.Fatal(err)
	}
	if on, err := LockInstalled(); err != nil || on {
		t.Fatalf("après retrait : posé=%v, erreur=%v", on, err)
	}
}
