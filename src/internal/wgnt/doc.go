// Package wgnt est le paquet driver de wireguard-windows (licence MIT, voir
// COPYING-wireguard-windows.txt dans internal/killswitch), qui pilote WireGuardNT.
// Seule modification : wireguard.dll est chargé en mémoire depuis l'exe
// (dll_embed_windows.go) au lieu d'un fichier à côté de l'exe.
//
// Windows uniquement.
package wgnt
