package main

import (
	"fyne.io/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// startTray runs the menu-bar/system-tray icon in the background. Pachyderm
// has no Dock/taskbar presence (see LSUIElement in build/darwin/Info*.plist
// and StartHidden in main.go) — the tray icon is the only way in and out.
func (a *App) startTray() {
	go systray.Run(a.onTrayReady, func() {})
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Pachyderm")

	// On Linux, a click with no handler registered is a no-op (unlike
	// Windows/macOS, which fall back to showing the menu) — so without
	// these, clicking the tray icon does nothing. Registering both means
	// every click reliably does something, regardless of whether this
	// desktop's tray host renders the Show/Quit menu on its own.
	systray.SetOnTapped(func() {
		wailsruntime.WindowShow(a.ctx)
	})
	systray.SetOnSecondaryTapped(func() {
		wailsruntime.WindowShow(a.ctx)
	})

	show := systray.AddMenuItem("Show Pachyderm", "Open the Pachyderm window")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit Pachyderm", "Quit Pachyderm")

	for {
		select {
		case <-show.ClickedCh:
			wailsruntime.WindowShow(a.ctx)
		case <-quit.ClickedCh:
			wailsruntime.Quit(a.ctx)
			return
		}
	}
}
