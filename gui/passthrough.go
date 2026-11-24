package main

import (
	"time"

	"github.com/xvzc/spoofdpi/internal/engine"
)

// Some apps read the system proxy once and pin it. Claude Desktop, for
// example, starts its sessions with HTTPS_PROXY=http://127.0.0.1:8080 in the
// environment and keeps using that address after the system proxy is turned
// off. If the port simply disappeared, those apps would lose the internet.
//
// So while the app runs and protection is stopped, the listen address stays
// open as a plain pass-through proxy: no fragmentation, system DNS, no system
// proxy settings. It is not "protection": it only keeps pinned clients alive.

var directProfile = engine.Profile{
	ID: "direct", Name: "direct", DNSMode: "system", SplitMode: "none",
}

// startStandbyLocked opens the pass-through listener (HTTP/SOCKS modes only).
func (a *App) startStandbyLocked() {
	if a.isTunLocked() || a.quitting || a.standby.Status().Running {
		return
	}
	o := a.st.Options
	o.SystemProxy = false
	cfg, err := engine.BuildConfig(directProfile, o)
	if err != nil {
		return
	}
	// The previous listener closes asynchronously; retry briefly.
	for i := 0; i < 20; i++ {
		if err = a.standby.Start(cfg); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (a *App) stopStandby() {
	a.standby.Stop()
}

func (a *App) PinnedConnections() int {
	a.mu.Lock()
	listen := a.st.Options.ListenAddr
	tun := a.isTunLocked()
	a.mu.Unlock()
	if tun {
		return 0
	}
	return pinnedConnections(listen)
}

// EnsureStandby opens the pass-through listener when protection is off.
func (a *App) EnsureStandby() {
	a.mu.Lock()
	if !a.eng.Status().Running && !a.tun.Running() {
		a.startStandbyLocked()
	}
	a.mu.Unlock()
	a.changed()
}
