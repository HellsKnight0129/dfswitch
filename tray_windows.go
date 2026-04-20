//go:build windows

package main

import (
	"context"

	"github.com/energye/systray"
)

// registerTray boots the systray on Windows.
//
// Unlike darwin, systray on Windows creates its own hidden message-only
// window (pCreateWindowEx at systray_windows.go:479) and spawns a dedicated
// goroutine for the GetMessage/DispatchMessage loop inside nativeStart. That
// runs independently of Wails' own WebView2 message pump, so there's no
// main-thread requirement and no dispatch hop needed — just call start().
func registerTray(appCtx func() context.Context) {
	start, _ := systray.RunWithExternalLoop(buildTrayOnReady(appCtx), func() {})
	start()
}
