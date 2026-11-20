package main

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// pinnedConnections counts TCP connections that other processes currently
// hold to our listen address. (netstat prints nothing for TCP when run from
// the app, so lsof is used; our own server-side sockets are skipped.)
func pinnedConnections(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return 0
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	out, err := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP@"+host+":"+port, "-sTCP:ESTABLISHED").Output()
	if err != nil && len(out) == 0 {
		return 0
	}
	self := strconv.Itoa(os.Getpid())
	n := 0
	for i, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 2 || f[1] == self {
			continue
		}
		n++
	}
	return n
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
