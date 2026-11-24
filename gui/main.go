// Dikec DPI desktop app: main window + menu bar popover around the
// spoofdpi engine.
package main

import (
	"embed"
	"flag"
	"fmt"
	"image/png"
	"io/fs"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/xvzc/spoofdpi/internal/engine"
)

var version = "dev"

//go:embed frontend
var frontend embed.FS

func main() {
	// Build helper: render the app icon PNG (used to make the .icns).
	renderIcon := flag.String(
		"render-icon",
		"",
		"write the app icon PNG to this path and exit",
	)
	iconSize := flag.Int("icon-size", 1024, "size of the rendered icon")
	flag.Parse()
	if *renderIcon != "" {
		f, err := os.Create(*renderIcon)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, renderAppIcon(*iconSize)); err != nil {
			log.Fatal(err)
		}
		f.Close()
		return
	}

	// Undo a system proxy left behind by a crash before anything else.
	engine.CleanupStaleState()

	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	svc := NewApp()

	var mainWin *application.WebviewWindow

	// Quit flow: warn before cutting off apps that pinned the proxy address.
	var app *application.App
	var quitting atomic.Bool
	reallyQuit := func() {
		quitting.Store(true)
		app.Quit()
	}
	var hideAll func()
	requestQuit := func() {
		n := svc.PinnedConnections()
		if n == 0 {
			reallyQuit()
			return
		}
		lang := svc.lang()
		d := app.Dialog.Question().
			SetTitle(nt(lang, "qTitle")).
			SetMessage(fmt.Sprintf(nt(lang, "qMsg"), n))
		keep := d.AddButton(nt(lang, "qKeep"))
		keep.OnClick(func() {
			if hideAll != nil {
				hideAll()
			}
		})
		d.AddButton(nt(lang, "qQuit")).OnClick(reallyQuit)
		d.SetDefaultButton(keep)
		d.SetCancelButton(keep)
		d.Show()
	}

	app = application.New(application.Options{
		// Cmd+Q / Dock "Quit" end up here; route them through the warning.
		ShouldQuit: func() bool {
			if quitting.Load() {
				return true
			}
			go requestQuit()
			return false
		},
		Name:        "Dikec DPI",
		Description: "DPI engellerini aşan, kendi kendini ayarlayan proxy",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyRegular,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.dikec.dpi",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if mainWin != nil {
					mainWin.Show()
					mainWin.Focus()
				}
			},
		},
	})

	mainWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Dikec DPI",
		Width:            980,
		Height:           660,
		MinWidth:         820,
		MinHeight:        560,
		URL:              "/index.html",
		BackgroundColour: application.NewRGB(0x0B, 0x0C, 0x0F),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 44,
			Backdrop:                application.MacBackdropNormal,
		},
	})
	// Closing the window keeps Dikec DPI running in the menu bar.
	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWin.Hide()
		e.Cancel()
	})

	popover := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "tray",
		Width:            330,
		Height:           452,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		DisableResize:    true,
		HideOnEscape:     true,
		HideOnFocusLost:  true,
		URL:              "/index.html?view=tray",
		BackgroundColour: application.NewRGBA(0x15, 0x17, 0x1C, 0xFF),
		Windows:          application.WindowsWindow{HiddenOnTaskbar: true},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTransparent,
		},
	})

	tray := app.SystemTray.New()
	setTrayState(tray, trayOff)
	tray.SetTooltip("Dikec DPI")
	tray.AttachWindow(popover).WindowOffset(6)

	menu := app.NewMenu()
	lang := svc.lang()
	toggleItem := menu.Add(nt(lang, "on"))
	toggleItem.OnClick(func(*application.Context) { _ = svc.Toggle() })
	openItem := menu.Add(nt(lang, "open"))
	openItem.OnClick(func(*application.Context) { mainWin.Show(); mainWin.Focus() })
	menu.AddSeparator()
	quitItem := menu.Add(nt(lang, "quit"))
	quitItem.OnClick(func(*application.Context) { go requestQuit() })
	tray.SetMenu(menu)

	// ---- wire the service to the shell ----
	svc.emit = func(name string, data any) { app.Event.Emit(name, data) }
	svc.openMain = func() { popover.Hide(); mainWin.Show(); mainWin.Focus() }
	svc.quit = func() { go requestQuit() }
	svc.forceQuit = reallyQuit
	hideAll = func() { popover.Hide(); mainWin.Hide() }
	svc.openURL = func(u string) { _ = app.Browser.OpenURL(u) }

	// Check for updates shortly after launch, then every 6 hours.
	go func() {
		select {
		case <-app.Context().Done():
			return
		case <-time.After(5 * time.Second):
		}
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			if info := svc.CheckUpdate(); info.Available {
				app.Event.Emit("update:available", info)
			}
			select {
			case <-app.Context().Done():
				return
			case <-t.C:
			}
		}
	}()

	var anim struct {
		sync.Mutex
		stop chan struct{}
	}
	updateTray := func() {
		st := svc.GetState()
		anim.Lock()
		defer anim.Unlock()
		if st.Scanning {
			if anim.stop == nil {
				anim.stop = make(chan struct{})
				go func(stop chan struct{}) {
					frames := []trayState{trayScan0, trayScan1, trayScan2}
					t := time.NewTicker(400 * time.Millisecond)
					defer t.Stop()
					for i := 0; ; i++ {
						select {
						case <-stop:
							return
						case <-t.C:
							setTrayState(tray, frames[i%3])
						}
					}
				}(anim.stop)
			}
			tray.SetTooltip(nt(st.Language, "tipScan"))
			return
		}
		if anim.stop != nil {
			close(anim.stop)
			anim.stop = nil
		}
		switch {
		case st.Status.Running && st.Error != "":
			setTrayState(tray, trayWarn)
		case st.Status.Running:
			setTrayState(tray, trayOn)
		default:
			setTrayState(tray, trayOff)
		}
		label := ""
		if st.Status.Running && st.ShowTrayCount && st.UnblockedNow > 0 {
			label = strconv.Itoa(st.UnblockedNow)
		}
		tray.SetLabel(label)
		lang := st.Language
		openItem.SetLabel(nt(lang, "open"))
		quitItem.SetLabel(nt(lang, "quit"))
		if st.Status.Running {
			toggleItem.SetLabel(nt(lang, "off"))
			tray.SetTooltip(fmt.Sprintf(nt(lang, "tipOn"), st.Selected.Label))
		} else {
			toggleItem.SetLabel(nt(lang, "on"))
			tray.SetTooltip(nt(lang, "tipOff"))
		}
		menu.Update()
		app.Event.Emit("state:changed", st)
	}
	svc.onChange = updateTray

	// Refresh the badge periodically: the unblocked counter changes as the
	// user browses.
	go func() {
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-app.Context().Done():
				return
			case <-t.C:
				if svc.running() {
					updateTray()
				}
			}
		}
	}()

	app.Event.OnApplicationEvent(
		events.Common.ApplicationStarted,
		func(*application.ApplicationEvent) {
			st := svc.GetState()
			if st.StartOnLaunch {
				_ = svc.Start()
			} else {
				svc.EnsureStandby()
			}
			updateTray()
		},
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setTrayState shows the state glyph as a template image on macOS; other
// systems get the colored app icon (state is in the tooltip and menu).
func setTrayState(tray *application.SystemTray, s trayState) {
	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(trayIcons[s])
		return
	}
	tray.SetIcon(trayIconColor)
}
