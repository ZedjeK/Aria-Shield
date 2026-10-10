package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "Aria Shield",
		Description: "Navigation discrète : DNS local Unbound, VPN et Tor",
		Services: []application.Service{
			application.NewService(NewShieldService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Aria Shield",
		Width:            980,
		Height:           640,
		MinWidth:         420,
		MinHeight:        520,
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
