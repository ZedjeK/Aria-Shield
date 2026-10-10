// Package torbrowser installe, met à jour et lance Tor Browser (version portable
// officielle) dans le dossier d'Aria. Chaque fichier est vérifié par signature PGP
// avec la clé des développeurs de Tor Browser avant d'être exécuté.
package torbrowser

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Empreinte de la clé principale « Tor Browser Developers (signing key) »,
// publiée sur support.torproject.org. Seule une clé portant cette empreinte est acceptée.
const SigningKeyFingerprint = "EF6E286DDA85EA2A4BA7DE684E2C6E8793298290"

// Clé publique embarquée (avec ses sous-clés de signature au 10/10/2026).
//
//go:embed assets/torbrowser-signing-key.gpg
var embeddedKey []byte

// Annuaire WKD officiel du projet Tor : permet de récupérer les nouvelles sous-clés.
const wkdURL = "https://openpgpkey.torproject.org/.well-known/openpgpkey/torproject.org/hu/kounek7zrdx745qydx6p59t9mqjpuhdf"

// loadKeyring lit une clé binaire et vérifie qu'il s'agit bien de la clé attendue.
func loadKeyring(data []byte) (openpgp.EntityList, error) {
	ring, err := openpgp.ReadKeyRing(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var kept openpgp.EntityList
	for _, e := range ring {
		if strings.EqualFold(hex.EncodeToString(e.PrimaryKey.Fingerprint), SigningKeyFingerprint) {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return nil, errors.New("aucune clé ne correspond à l'empreinte des développeurs de Tor Browser")
	}
	return kept, nil
}

// fetchKey récupère la clé à jour depuis l'annuaire officiel (nouvelles sous-clés).
func fetchKey() (openpgp.EntityList, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(wkdURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("annuaire de clés : HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return loadKeyring(data)
}

// Verify contrôle la signature détachée sig (format ASCII armor) du fichier lu par data.
// La clé embarquée est essayée d'abord ; si la signature vient d'une sous-clé plus
// récente, la clé est rafraîchie depuis l'annuaire officiel, toujours épinglée sur
// la même empreinte principale.
func Verify(data io.ReadSeeker, sig []byte) error {
	try := func(ring openpgp.EntityList) error {
		if _, err := data.Seek(0, io.SeekStart); err != nil {
			return err
		}
		_, err := openpgp.CheckArmoredDetachedSignature(ring, data, bytes.NewReader(sig), nil)
		return err
	}

	ring, err := loadKeyring(embeddedKey)
	if err != nil {
		return fmt.Errorf("clé embarquée illisible : %w", err)
	}
	firstErr := try(ring)
	if firstErr == nil {
		return nil
	}
	fresh, err := fetchKey()
	if err != nil {
		return fmt.Errorf("signature invalide : %w", firstErr)
	}
	if err := try(fresh); err != nil {
		return fmt.Errorf("signature invalide : %w", err)
	}
	return nil
}
