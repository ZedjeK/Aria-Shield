//go:build windows

package autostart

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"aria-shield/internal/sysx"
)

const (
	installDirName = "Aria Shield"
	installExeName = "Aria-Shield.exe"
)

// installedExe renvoie l'emplacement de la copie lancée par la tâche. Program
// Files est lu auprès de Windows et non dans la variable d'environnement, qu'un
// programme sans droits pourrait faire pointer ailleurs.
func installedExe() (string, error) {
	pf, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return "", fmt.Errorf("dossier Program Files introuvable : %w", err)
	}
	return filepath.Join(pf, installDirName, installExeName), nil
}

// schtasks renvoie le chemin complet de schtasks.exe (jamais cherché dans le PATH).
func schtasks() string {
	sys, err := windows.GetSystemDirectory()
	if err != nil {
		sys = `C:\Windows\System32`
	}
	return filepath.Join(sys, "schtasks.exe")
}

// Status indique si la tâche existe, et quel exe elle lance.
func Status() State {
	exe, _ := installedExe()
	_, err := sysx.Run(15*time.Second, schtasks(), "/Query", "/TN", TaskName)
	return State{Enabled: err == nil, Exe: exe}
}

// Enable copie l'exe en cours dans Program Files et crée la tâche planifiée.
func Enable(current string) error {
	dst, err := installedExe()
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	if !samePath(current, dst) {
		if err := copyExe(current, dst); err != nil {
			return fmt.Errorf("copie de l'exe dans %s : %w", filepath.Dir(dst), err)
		}
	}

	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	// Le XML est écrit dans Program Files (réservé aux administrateurs) : aucun
	// programme sans droits ne peut le modifier entre l'écriture et la lecture.
	xmlPath := filepath.Join(filepath.Dir(dst), "tache.xml")
	if err := os.WriteFile(xmlPath, utf16File(taskXML(sid, dst)), 0o644); err != nil {
		return err
	}
	defer os.Remove(xmlPath)
	if out, err := sysx.Run(30*time.Second, schtasks(), "/Create", "/TN", TaskName, "/XML", xmlPath, "/F"); err != nil {
		return fmt.Errorf("création de la tâche planifiée : %v %s", err, out)
	}
	return nil
}

// Disable supprime la tâche et la copie de Program Files. Si l'Aria en cours
// est cette copie, elle ne peut pas s'effacer elle-même : Windows la supprimera
// au prochain redémarrage.
func Disable(current string) error {
	var errs []error
	if Status().Enabled {
		if out, err := sysx.Run(30*time.Second, schtasks(), "/Delete", "/TN", TaskName, "/F"); err != nil {
			errs = append(errs, fmt.Errorf("suppression de la tâche planifiée : %v %s", err, out))
		}
	}
	// Le dossier « Aria Shield » du Planificateur de tâches, désormais vide.
	sysx.PowerShell(15*time.Second,
		`$s = New-Object -ComObject Schedule.Service; $s.Connect(); try { $s.GetFolder('\').DeleteFolder('Aria Shield', 0) } catch {}`)

	dst, err := installedExe()
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	dir := filepath.Dir(dst)
	if samePath(current, dst) {
		for _, p := range []string{dst, dir} {
			if err := deleteAtReboot(p); err != nil {
				errs = append(errs, err)
			}
		}
	} else if err := os.RemoveAll(dir); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Refresh remet à jour la copie de Program Files quand tu lances une autre
// version d'Aria, pour que la tâche démarre bien celle-ci. Renvoie true si la
// copie a été remplacée.
func Refresh(current string) (bool, error) {
	dst, err := installedExe()
	if err != nil || samePath(current, dst) || !Status().Enabled {
		return false, err
	}
	a, err := fileHash(current)
	if err != nil {
		return false, err
	}
	if b, err := fileHash(dst); err == nil && bytes.Equal(a, b) {
		return false, nil
	}
	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return false, err
	}
	return true, copyExe(current, dst)
}

// ensureDir crée le dossier d'installation. Un lien (jonction) est refusé :
// Aria y écrit un exe qui sera lancé en administrateur.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return fmt.Errorf("%s n'est pas un vrai dossier", dir)
	}
	return nil
}

// copyExe copie src vers dst en passant par un fichier temporaire du même
// dossier, puis remplace dst d'un coup : jamais d'exe à moitié écrit.
func copyExe(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "Aria-Shield-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, dst); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func fileHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func deleteAtReboot(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}

// currentUserSID renvoie le SID du compte qui fait tourner Aria : la tâche se
// déclenche à l'ouverture de sa session, et seulement de la sienne.
func currentUserSID() (string, error) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("compte Windows introuvable : %w", err)
	}
	return tu.User.Sid.String(), nil
}

// taskXML décrit la tâche : à l'ouverture de ta session, avec les privilèges
// les plus élevés, sans limite de durée (par défaut, Windows arrête une tâche
// au bout de 72 h) ni arrêt sur batterie, en priorité normale.
func taskXML(sid, exe string) string {
	esc := func(s string) string {
		var b strings.Builder
		xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Author>Aria Shield</Author>
    <Description>Lance Aria Shield dans la zone de notification à l'ouverture de ta session, et réactive ta dernière protection.</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + esc(sid) + `</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + esc(sid) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>5</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>"` + esc(exe) + `"</Command>
      <Arguments>` + SilentArg + `</Arguments>
    </Exec>
  </Actions>
</Task>
`
}

// utf16File encode le XML en UTF-16 little-endian avec BOM, ce qu'attend schtasks.
func utf16File(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 2+2*len(u))
	out[0], out[1] = 0xFF, 0xFE
	for i, c := range u {
		binary.LittleEndian.PutUint16(out[2+2*i:], c)
	}
	return out
}
