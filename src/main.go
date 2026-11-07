// Aria Shield : logiciel portable pour Windows qui protège la navigation
// (résolveur DNS Unbound, VPN WireGuard avec kill switch, Tor Browser).
//
// main.go crée l'application Wails et sa fenêtre. Toute la logique est dans
// shieldservice.go (côté Go) et frontend/src (l'interface Svelte).
//
// L'exe demande les droits administrateur au lancement (manifest dans
// build/windows) : régler le DNS des cartes réseau et le pare-feu l'exige.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"aria-shield/internal/paths"
	"aria-shield/internal/torbrowser"
)

// L'interface compilée par Vite est embarquée dans l'exe : rien n'est chargé
// depuis internet ni depuis le disque.
//
//go:embed all:frontend/dist
var assets embed.FS

// main démarre Aria. À la fermeture, Wails appelle ServiceShutdown, qui remet
// la connexion de Windows comme avant.
func main() {
	// Assistant Tor : le même exe, relancé par Aria sans droits administrateur
	// pour installer ou mettre à jour Tor Browser (voir internal/torbrowser).
	if len(os.Args) > 1 && os.Args[1] == torbrowser.HelperArg {
		os.Exit(torbrowser.RunHelper(os.Args[2:]))
	}

	svc := NewShieldService()
	app := application.New(application.Options{
		Name:        "Aria Shield",
		Description: "Navigation discrète : DNS local Unbound, VPN et Tor",
		Services: []application.Service{
			application.NewService(svc),
		},
		Windows: application.WindowsOptions{
			// Profil WebView2 rangé avec les données d'Aria, et non dans %APPDATA%.
			WebviewUserDataPath: paths.WebviewDir(svc.userDir),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Aria Shield",
		Width:     980,
		Height:    640,
		MinWidth:  420, // en dessous de 760 px, la barre latérale se replie en icônes
		MinHeight: 520,
		// Fond sombre dès l'ouverture : pas d'éclair blanc avant que l'interface s'affiche.
		BackgroundColour: application.NewRGB(15, 18, 22),
		URL:              "/",
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
