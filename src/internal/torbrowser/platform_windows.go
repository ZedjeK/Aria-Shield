//go:build windows

package torbrowser

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runInstaller extrait l'installeur NSIS portable en mode silencieux.
// /D= doit être le dernier argument et ne jamais être entre guillemets.
func runInstaller(installer, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       fmt.Sprintf(`"%s" /S /D=%s`, installer, target),
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return errors.New("délai dépassé")
		}
		return err
	}
	return nil
}

// running indique si un firefox.exe du dossier Tor Browser d'Aria tourne.
func running(dir string) bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)

	prefix := strings.ToLower(filepath.Clean(dir)) + `\`
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "firefox.exe") {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID)
		if err != nil {
			continue
		}
		buf := make([]uint16, windows.MAX_LONG_PATH)
		n := uint32(len(buf))
		err = windows.QueryFullProcessImageName(h, 0, &buf[0], &n)
		windows.CloseHandle(h)
		if err == nil && strings.HasPrefix(strings.ToLower(windows.UTF16ToString(buf[:n])), prefix) {
			return true
		}
	}
	return false
}

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procGetShellWindow      = user32.NewProc("GetShellWindow")
	advapi32                = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTW = advapi32.NewProc("CreateProcessWithTokenW")
)

// launchUnelevated lance exe avec le jeton de l'Explorateur Windows, donc avec les
// droits normaux de l'utilisateur : un navigateur ne doit jamais tourner en admin.
func launchUnelevated(exe, dir string) error {
	err := launchWithShellToken(exe, dir)
	if err == nil {
		return nil
	}
	// Repli : l'Explorateur ouvre lui-même le programme, sans élévation.
	if err2 := exec.Command("explorer.exe", exe).Start(); err2 != nil {
		return fmt.Errorf("lancement impossible : %v / %v", err, err2)
	}
	return nil
}

func launchWithShellToken(exe, dir string) error {
	hwnd, _, _ := procGetShellWindow.Call()
	if hwnd == 0 {
		return errors.New("bureau Windows introuvable")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil {
		return err
	}
	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(proc)

	var shellToken windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_DUPLICATE, &shellToken); err != nil {
		return err
	}
	defer shellToken.Close()

	var token windows.Token
	access := uint32(windows.TOKEN_QUERY | windows.TOKEN_ASSIGN_PRIMARY | windows.TOKEN_DUPLICATE |
		windows.TOKEN_ADJUST_DEFAULT | windows.TOKEN_ADJUST_SESSIONID)
	if err := windows.DuplicateTokenEx(shellToken, access, nil, windows.SecurityImpersonation, windows.TokenPrimary, &token); err != nil {
		return err
	}
	defer token.Close()

	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cmdLine, err := windows.UTF16PtrFromString(`"` + exe + `"`)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var pi windows.ProcessInformation
	r, _, e := procCreateProcessWithTW.Call(
		uintptr(token), 0,
		uintptr(unsafe.Pointer(app)), uintptr(unsafe.Pointer(cmdLine)),
		windows.CREATE_UNICODE_ENVIRONMENT, 0,
		uintptr(unsafe.Pointer(cwd)),
		uintptr(unsafe.Pointer(&si)), uintptr(unsafe.Pointer(&pi)))
	if r == 0 {
		return e
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}
