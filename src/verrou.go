package main

// Commande de secours du verrou persistant (lot 7) : « Aria-Shield.exe
// --deverrouiller » retire le verrou sans ouvrir l'interface, par exemple
// quand le serveur VPN ne répond plus et qu'Internet reste coupé.

import (
	"golang.org/x/sys/windows"

	"aria-shield/internal/killswitch"
)

// unlockArg : l'argument qui retire le verrou et quitte.
const unlockArg = "--deverrouiller"

// runUnlock retire le verrou, le dit dans une fenêtre et renvoie le code de sortie.
func runUnlock() int {
	msg, icon, code := "Verrou retiré : Aria ne bloque plus ta connexion.", uint32(windows.MB_ICONINFORMATION), 0
	if err := killswitch.DisableLock(); err != nil {
		msg, icon, code = "Le verrou n'a pas pu être retiré : "+err.Error(), windows.MB_ICONERROR, 1
	}
	text, _ := windows.UTF16PtrFromString(msg)
	title, _ := windows.UTF16PtrFromString("Aria Shield")
	windows.MessageBox(0, text, title, windows.MB_OK|icon)
	return code
}
