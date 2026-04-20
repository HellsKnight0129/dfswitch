package main

import (
	"context"
	_ "embed"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/appicon.png
var trayIcon []byte

// buildTrayOnReady returns the systray onReady callback. The menu layout and
// wiring is identical on every platform; only how we start systray differs
// (see registerTray in tray_darwin.go / tray_windows.go).
func buildTrayOnReady(appCtx func() context.Context) func() {
	return func() {
		systray.SetTitle("DF")
		systray.SetIcon(trayIcon)
		systray.SetTooltip("DFSwitch")

		// energye/systray doesn't attach the menu to the status item on
		// darwin (systray_darwin.m:78 is commented out), so a bare left
		// click only fires onClick. On windows left click also just fires
		// onClick; the menu only auto-pops on right-click if onRClick is
		// unset. Routing both into ShowMenu gives consistent behaviour.
		systray.SetOnClick(func(m systray.IMenu) { _ = m.ShowMenu() })
		systray.SetOnRClick(func(m systray.IMenu) { _ = m.ShowMenu() })

		show := systray.AddMenuItem("显示窗口", "Show DFSwitch window")
		hide := systray.AddMenuItem("隐藏窗口", "Hide DFSwitch window")
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出", "Quit DFSwitch")

		show.Click(func() {
			if ctx := appCtx(); ctx != nil {
				runtime.WindowShow(ctx)
				runtime.WindowUnminimise(ctx)
			}
		})
		hide.Click(func() {
			if ctx := appCtx(); ctx != nil {
				runtime.WindowHide(ctx)
			}
		})
		quit.Click(func() {
			systray.Quit()
			if ctx := appCtx(); ctx != nil {
				runtime.Quit(ctx)
			}
		})
	}
}
