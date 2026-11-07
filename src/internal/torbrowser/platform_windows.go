//go:build windows

package torbrowser

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// Fonctions Win32 absentes du paquet golang.org/x/sys/windows, chargées à la demande.
var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procGetShellWindow      = user32.NewProc("GetShellWindow")
	advapi32                = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTW = advapi32.NewProc("CreateProcessWithTokenW")
)

// launchUnelevated lance exe avec le jeton de l'Explorateur Windows, donc avec les
// droits normaux de l'utilisateur : un navigateur ne doit jamais tourner en admin.
func launchUnelevated(exe, dir string) error {
	proc, err := startWithShellToken(exe, nil, dir, nil)
	if err == nil {
		windows.CloseHandle(proc)
		return nil
	}
	// Repli : l'Explorateur ouvre lui-même le programme, sans élévation.
	if err2 := exec.Command("explorer.exe", exe).Start(); err2 != nil {
		return fmt.Errorf("lancement impossible : %v / %v", err, err2)
	}
	return nil
}

// runHelper lance l'assistant (le même exe, avec HelperArg) au nom de
// l'utilisateur, sans élévation, l'attend et relaie son avancement. Il n'y a
// pas de repli en administrateur : sans jeton de l'Explorateur, on s'arrête.
func (m *Manager) runHelper(op, version string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	userDir := filepath.Dir(m.dir)
	nonce := newNonce()
	args := []string{HelperArg, "-op", op, "-dossier", userDir, "-jeton", nonce}
	if version != "" {
		args = append(args, "-version", version)
	}

	// L'exe demande l'élévation dans son manifeste : la couche de compatibilité
	// RunAsInvoker l'ignore pour ce lancement, qui garde le jeton non élevé.
	shellEnv := func(token windows.Token) ([]string, error) {
		env, err := token.Environ(false)
		if err != nil {
			return nil, err
		}
		return append(env, "__COMPAT_LAYER=RunAsInvoker"), nil
	}
	proc, err := startWithShellToken(exe, args, filepath.Dir(exe), shellEnv)
	if err != nil {
		return fmt.Errorf("assistant Tor impossible à lancer sans droits administrateur : %w", err)
	}
	defer windows.CloseHandle(proc)

	status := filepath.Join(userDir, statusName)
	deadline := time.Now().Add(helperTimeout)
	for {
		ev, err := windows.WaitForSingleObject(proc, 250)
		if st, ok := readStatus(status, nonce); ok && st.Progress.Active {
			m.setProgress(st.Progress.Phase, st.Progress.Done, st.Progress.Total)
		}
		if err != nil {
			return err
		}
		if ev == windows.WAIT_OBJECT_0 {
			break
		}
		if time.Now().After(deadline) {
			_ = windows.TerminateProcess(proc, 1)
			return errors.New("l'assistant Tor ne répond plus (délai dépassé)")
		}
	}

	var code uint32
	if err := windows.GetExitCodeProcess(proc, &code); err != nil {
		return err
	}
	if code == 0 {
		return nil
	}
	if st, ok := readStatus(status, nonce); ok && st.Error != "" {
		return errors.New(st.Error)
	}
	return fmt.Errorf("l'assistant Tor s'est arrêté (code %d)", code)
}

// startWithShellToken lance exe avec une copie du jeton du processus qui
// possède le bureau (l'Explorateur), donc au nom de l'utilisateur et sans
// élévation, même si Aria tourne en administrateur. C'est la méthode
// recommandée par Microsoft pour « dé-élever » un lancement. env, s'il est
// fourni, construit l'environnement du processus à partir de ce jeton.
// L'appelant ferme la poignée du processus renvoyée.
func startWithShellToken(exe string, args []string, dir string, env func(windows.Token) ([]string, error)) (windows.Handle, error) {
	hwnd, _, _ := procGetShellWindow.Call()
	if hwnd == 0 {
		return 0, errors.New("bureau Windows introuvable")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil {
		return 0, err
	}
	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(proc)

	var shellToken windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_DUPLICATE, &shellToken); err != nil {
		return 0, err
	}
	defer shellToken.Close()

	var token windows.Token
	access := uint32(windows.TOKEN_QUERY | windows.TOKEN_ASSIGN_PRIMARY | windows.TOKEN_DUPLICATE |
		windows.TOKEN_ADJUST_DEFAULT | windows.TOKEN_ADJUST_SESSIONID)
	if err := windows.DuplicateTokenEx(shellToken, access, nil, windows.SecurityImpersonation, windows.TokenPrimary, &token); err != nil {
		return 0, err
	}
	defer token.Close()

	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	line := syscall.EscapeArg(exe)
	for _, a := range args {
		line += " " + syscall.EscapeArg(a)
	}
	cmdLine, err := windows.UTF16PtrFromString(line)
	if err != nil {
		return 0, err
	}
	cwd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var envBlock *uint16
	if env != nil {
		vars, err := env(token)
		if err != nil {
			return 0, err
		}
		if envBlock, err = environmentBlock(vars); err != nil {
			return 0, err
		}
	}
	si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var pi windows.ProcessInformation
	r, _, e := procCreateProcessWithTW.Call(
		uintptr(token), 0,
		uintptr(unsafe.Pointer(app)), uintptr(unsafe.Pointer(cmdLine)),
		windows.CREATE_UNICODE_ENVIRONMENT, uintptr(unsafe.Pointer(envBlock)),
		uintptr(unsafe.Pointer(cwd)),
		uintptr(unsafe.Pointer(&si)), uintptr(unsafe.Pointer(&pi)))
	if r == 0 {
		return 0, e
	}
	windows.CloseHandle(pi.Thread)
	return pi.Process, nil
}

// environmentBlock met des variables « NOM=valeur » au format attendu par
// CreateProcess : chaînes UTF-16 terminées par zéro, puis un zéro final.
func environmentBlock(vars []string) (*uint16, error) {
	var block []uint16
	for _, v := range vars {
		u, err := windows.UTF16FromString(v)
		if err != nil {
			return nil, err
		}
		block = append(block, u...) // u se termine déjà par un zéro
	}
	block = append(block, 0)
	return &block[0], nil
}
