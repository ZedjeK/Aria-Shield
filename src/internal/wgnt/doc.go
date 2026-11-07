// Package wgnt est le paquet driver de wireguard-windows (licence MIT, voir
// COPYING-wireguard-windows.txt dans internal/killswitch), qui pilote WireGuardNT.
// Seule modification : wireguard.dll est chargé en mémoire depuis l'exe
// (dll_embed_windows.go) au lieu d'un fichier à côté de l'exe.
//
// Le reste est volontairement laissé tel que l'écrit WireGuard (commentaires
// anglais compris) : plus facile à comparer avec l'original et à mettre à jour.
// wireguard.dll est la version officielle signée de WireGuardNT ; son empreinte
// est vérifiée avant chargement.
//
// Windows uniquement.
package wgnt
