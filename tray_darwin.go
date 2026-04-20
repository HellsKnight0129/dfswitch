//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <dispatch/dispatch.h>

// trampoline defined in Go
extern void trayStartTrampoline(void);

static void dispatch_tray_start(void) {
    dispatch_async_f(dispatch_get_main_queue(), NULL, (dispatch_function_t)trayStartTrampoline);
}
*/
import "C"

import (
	"context"
	"sync"

	"github.com/energye/systray"
)

var (
	trayStartFn func()
	trayStartMu sync.Mutex
)

//export trayStartTrampoline
func trayStartTrampoline() {
	trayStartMu.Lock()
	fn := trayStartFn
	trayStartMu.Unlock()
	if fn != nil {
		fn()
	}
}

// registerTray hooks a status-bar menu into Wails' Cocoa runloop.
//
// Two constraints collide here:
//  1. systray.Register on macOS > 10.14 only installs an NSApplicationDelegate
//     and never posts NSApplicationDidFinishLaunching, so NSStatusItem is
//     never created. Only RunWithExternalLoop → nativeStart posts the launch
//     notification manually.
//  2. nativeStart creates NSStatusBarWindow, which Cocoa insists must happen
//     on the main thread. Wails dispatches OnStartup on a background goroutine,
//     so calling start() directly from here panics with
//     "NSWindow should only be instantiated on the main thread!".
//
// Fix: hop to the main queue via dispatch_async before calling start().
// appCtx is a getter because Wails hands us ctx inside Startup, after this
// function has already returned.
func registerTray(appCtx func() context.Context) {
	start, _ := systray.RunWithExternalLoop(buildTrayOnReady(appCtx), func() {})

	trayStartMu.Lock()
	trayStartFn = start
	trayStartMu.Unlock()

	C.dispatch_tray_start()
}
