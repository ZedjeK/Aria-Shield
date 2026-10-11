package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Empreinte du wireguard.dll déposé à côté de l'exe par les v0.5 à v0.6.1.
const oldWireguardDLL = "b1b85e072c45d81358be29d94c599dc76652f912be8c0f0a41e2d5d89a6461d3"

// Migrate déplace l'ancien dossier AriaShield-data vers les nouveaux emplacements :
// profil VPN et Tor Browser vers userDir, photo du DNS vers sysDir. Le reste
// (Unbound, journaux) est recréé à la demande. L'ancien dossier n'est supprimé
// que si tout a pu être déplacé ; sinon on réessaiera au prochain lancement.
func Migrate(legacy, userDir, sysDir string) (moved []string, err error) {
	if legacy == "" {
		return nil, nil
	}
	items := []struct{ name, dst string }{
		{"vpn-profile.bin", filepath.Join(userDir, "vpn-profile.bin")},
		{"TorBrowser", filepath.Join(userDir, "TorBrowser")},
		{"dns-backup.json", filepath.Join(sysDir, "dns-backup.json")},
	}
	for _, it := range items {
		src := filepath.Join(legacy, it.name)
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if _, err := os.Stat(it.dst); err == nil {
			continue // déjà présent au nouvel emplacement : on garde le plus récent
		}
		if err := move(src, it.dst); err != nil {
			return moved, fmt.Errorf("%s : %w (Tor Browser est-il ouvert ?)", it.name, err)
		}
		moved = append(moved, it.name)
	}
	if err := os.RemoveAll(legacy); err != nil {
		return moved, fmt.Errorf("ancien dossier non supprimé : %w", err)
	}
	removeOldDLL(filepath.Dir(legacy))
	return moved, nil
}

// move renomme, ou copie puis supprime si la source est sur un autre disque.
func move(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyTree(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// removeOldDLL supprime le wireguard.dll laissé à côté de l'exe par les anciennes
// versions, uniquement s'il s'agit bien de ce fichier-là.
func removeOldDLL(dir string) {
	p := filepath.Join(dir, "wireguard.dll")
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) == oldWireguardDLL {
		os.Remove(p)
	}
}

// CleanOldDLL est appelé à chaque démarrage : la DLL est désormais chargée depuis l'exe.
func CleanOldDLL() {
	if exe, err := os.Executable(); err == nil {
		removeOldDLL(filepath.Dir(exe))
	}
}
