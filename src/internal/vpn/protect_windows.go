//go:build windows

package vpn

import "golang.zx2c4.com/wireguard/windows/conf/dpapi"

// Chiffrement DPAPI : seul le compte Windows qui a enregistré le profil peut le relire.
const dpapiName = "Aria Shield VPN"

func protect(plain []byte) ([]byte, error)  { return dpapi.Encrypt(plain, dpapiName) }
func unprotect(data []byte) ([]byte, error) { return dpapi.Decrypt(data, dpapiName) }
