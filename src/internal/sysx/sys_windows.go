//go:build windows

package sysx

import (
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	createNoWindow = 0x08000000
	tokenElevation = 20 // TOKEN_INFORMATION_CLASS.TokenElevation
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// IsAdmin indique si le processus tourne avec des droits élevés.
func IsAdmin() bool {
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}
	var token syscall.Token
	if err := syscall.OpenProcessToken(proc, syscall.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var elevated uint32
	var size uint32
	err = syscall.GetTokenInformation(token, tokenElevation,
		(*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &size)
	return err == nil && elevated != 0
}
