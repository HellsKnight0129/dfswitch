package main

import (
	"context"
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:web/dist
var assets embed.FS

// Version is stamped at build time via `-ldflags "-X main.Version=vX.Y.Z"`.
// Leave it as "dev" in unstamped builds so CheckUpdate just reports "dev
// -> vX.Y.Z" instead of crashing.
var Version = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "DFSwitch",
		Width:            1180,
		Height:           820,
		MinWidth:         960,
		MinHeight:        640,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 10, G: 14, B: 20, A: 255},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			registerTray(func() context.Context { return app.ctx })
		},
		// Closing the window hides it; the tray menu keeps the process alive.
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			wruntime.WindowHide(ctx)
			return true
		},
		HideWindowOnClose: true,
		Bind:              []interface{}{app},
		Menu:              buildAppMenu(),
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: false,
			About: &mac.AboutInfo{
				Title:   "DFSwitch",
				Message: "AI 工具接引司\n版本 " + Version,
			},
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}

// buildAppMenu returns the native menubar. On macOS the first submenu becomes
// the app menu (with About/Quit). Edit submenu provides the standard
// clipboard/undo shortcuts that the Webview would otherwise silently drop.
func buildAppMenu() *menu.Menu {
	m := menu.NewMenu()

	if runtime.GOOS == "darwin" {
		m.Append(menu.AppMenu())
		m.Append(menu.EditMenu())
	} else {
		fileMenu := m.AddSubmenu("文件")
		fileMenu.AddText("退出", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {})
		m.Append(menu.EditMenu())
	}

	return m
}
