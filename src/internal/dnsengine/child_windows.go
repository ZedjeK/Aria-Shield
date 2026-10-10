//go:build windows

package dnsengine

import (
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"aria-shield/internal/sysx"
)

var (
	jobOnce sync.Once
	job     windows.Handle
	jobErr  error
)

// prepareChild masque la console d'unbound.exe.
func prepareChild(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// killOnCloseJob crée (une seule fois) un job Windows qui tue ses processus
// quand Aria se termine, même brutalement.
func killOnCloseJob() (windows.Handle, error) {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = err
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			jobErr = err
			return
		}
		job = h
	})
	return job, jobErr
}

// bindToParent rattache unbound.exe au job d'Aria.
func bindToParent(cmd *exec.Cmd) error {
	j, err := killOnCloseJob()
	if err != nil {
		return err
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(p)
	return windows.AssignProcessToJobObject(j, p)
}

// portOwner tente de nommer le programme qui occupe déjà le port 53.
func portOwner() string {
	out, err := sysx.PowerShell(10*time.Second,
		`$p = @(Get-NetUDPEndpoint -LocalPort 53 -ErrorAction SilentlyContinue) + @(Get-NetTCPConnection -LocalPort 53 -State Listen -ErrorAction SilentlyContinue) | `+
			`Select-Object -First 1 -ExpandProperty OwningProcess; `+
			`if ($p) { $proc = Get-Process -Id $p -ErrorAction SilentlyContinue; "$($proc.ProcessName) (PID $p)" }`)
	if err != nil {
		return ""
	}
	out = strings.TrimSpace(out)
	if out == "" || strings.HasPrefix(out, " (PID") {
		return ""
	}
	return out
}
