//go:build !windows

package sysdns

import "errors"

var errWindowsOnly = errors.New("disponible uniquement sous Windows")

func listAdapters() ([]Adapter, error)                    { return nil, errWindowsOnly }
func pointToLoopback(index int, ipv6 bool) error          { return errWindowsOnly }
func restoreAdapter(a Adapter) (string, error)            { return "", errWindowsOnly }
func smartResolutionPolicy() (bool, uint32, error)        { return false, 0, errWindowsOnly }
func setSmartResolutionOff() error                        { return errWindowsOnly }
func restoreSmartResolution(existed bool, v uint32) error { return errWindowsOnly }
func flushCache()                                         {}
