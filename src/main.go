// Aria Shield : logiciel portable pour Windows qui protège la navigation
// (résolveur DNS Unbound, VPN WireGuard avec kill switch, Tor Browser).
//
// main.go crée l'application Wails, sa fenêtre et son icône dans la zone de
// notification. Toute la logique est dans shieldservice.go et startup.go (côté
// Go) et frontend/src (l'interface Svelte).
//
// L'exe demande les droits administrateur au lancement (manifest dans
// build/windows) : régler le DNS des cartes réseau et le pare-feu l'exige.
package main

import (
	"embed"
	"log"
	"os"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"aria-shield/internal/autostart"
	"aria-shield/internal/paths"
	"aria-shield/internal/torbrowser"
)

// Version affichée dans l'interface (garder la même dans build/windows/info.json).
const Version = "0.11.0"

// L'interface compilée par Vite est embarquée dans l'exe : rien n'est chargé
// depuis internet ni depuis le disque.
//
//go:embed all:frontend/dist
var assets embed.FS

// main démarre Aria. Fermer la fenêtre la range dans la zone de notification :
// la protection continue. « Quitter » (menu de l'icône ou barre latérale)
// arrête Aria, et ServiceShutdown remet la connexion de Windows comme avant.
func main() {
	// Assistant Tor : le même exe, relancé par Aria sans droits administrateur
	// pour installer ou mettre à jour Tor Browser (voir internal/torbrowser).
	if len(os.Args) > 1 && os.Args[1] == torbrowser.HelperArg {
		os.Exit(torbrowser.RunHelper(os.Args[2:]))
	}
	// Lancée par la tâche planifiée à l'ouverture de session : pas de fenêtre.
	silent := slices.Contains(os.Args[1:], autostart.SilentArg)

	svc := NewShieldService()
	svc.logStart(silent)
	svc.notify = newNotifier(svc.prefs)
	var window *application.WebviewWindow
	showWindow := func() {
		if window != nil {
			window.Show().Focus()
			window.UnMinimise()
		}
	}

	app := application.New(application.Options{
		Name:        "Aria Shield",
		Description: "Navigation discrète : DNS local Unbound, VPN et Tor",
		Services: []application.Service{
			application.NewService(svc.notify),
			application.NewService(svc),
		},
		// Une seule Aria à la fois : deux instances se disputeraient le DNS de
		// Windows. Relancer l'exe ramène simplement la fenêtre existante.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.zedjek.aria-shield",
			// L'exe relancé annonce sa version : si elle diffère, Aria le dit
			// au lieu de rouvrir sa fenêtre en silence.
			AdditionalData: map[string]string{"version": Version},
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				go svc.otherVersionLaunched(data.AdditionalData["version"])
				application.InvokeAsync(showWindow)
			},
		},
		Windows: application.WindowsOptions{
			// Profil WebView2 rangé avec les données d'Aria, et non dans %APPDATA%.
			WebviewUserDataPath: paths.WebviewDir(svc.userDir),
			// La fenêtre fermée, Aria continue dans la zone de notification.
			DisableQuitOnLastWindowClosed: true,
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Aria Shield",
		Width:     980,
		Height:    640,
		MinWidth:  420, // en dessous de 760 px, la barre latérale se replie en icônes
		MinHeight: 520,
		Hidden:    silent,
		// Fond sombre dès l'ouverture : pas d'éclair blanc avant que l'interface s'affiche.
		BackgroundColour: application.NewRGB(15, 18, 22),
		URL:              "/",
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})
	// La croix range la fenêtre au lieu de quitter.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if svc.quitting.Load() {
			return // Aria quitte vraiment : on laisse la fenêtre se fermer
		}
		window.Hide()
		e.Cancel()
		go svc.notify.trayHint() // « Aria continue en arrière-plan »
	})

	// Icône de la zone de notification (celle de l'exe) : clic gauche pour
	// ouvrir, clic droit pour le menu.
	menu := application.NewMenu()
	menu.Add("Ouvrir Aria Shield").OnClick(func(*application.Context) { showWindow() })
	menu.AddSeparator()
	menu.Add("Quitter (rétablit ta connexion d'origine)").OnClick(func(*application.Context) { go svc.quit() })
	tray := app.SystemTray.New()
	tray.SetTooltip("Aria Shield")
	tray.SetMenu(menu)
	tray.OnClick(showWindow)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
