package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	api := &API{}

	err := wails.Run(&options.App{
		Title:  "tokensforthepeople",
		Width:  720,
		Height: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: api.startup,
		Bind: []interface{}{
			api,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
