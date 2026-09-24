package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Permite que o WebView2 execute áudio automaticamente (WebRTC) sem exigir clique prévio e habilita IPs diretos na LAN
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--autoplay-policy=no-user-gesture-required --disable-features=WebRtcHideLocalIpsWithMdns")

	// Cria uma instância da aplicação
	app := NewApp()

	// Configura e executa a aplicação Wails
	err := wails.Run(&options.App{
		Title:             "Zyron Client - Suporte Remoto P2P",
		Width:             1100,
		Height:            780,
		MinWidth:          340,
		MinHeight:         220,
		Frameless:         true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
		},
	})

	if err != nil {
		println("Erro na inicialização do Wails:", err.Error())
	}
}
