package main

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/xvzc/spoofdpi/internal/engine"
)

func defaultRoute(t *testing.T) string {
	out, _ := exec.Command("/sbin/route", "-n", "get", "default").Output()
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "interface:"))
		}
	}
	return ""
}

func directGet(url string) error {
	c := &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Asks for the admin password in the macOS dialog.
// DIKEC_TUN_LIVE=1 DIKEC_CORE=<path> go test -run TunLive -v .
func TestTunLive(t *testing.T) {
	if os.Getenv("DIKEC_TUN_LIVE") == "" {
		t.Skip("set DIKEC_TUN_LIVE=1")
	}
	t.Setenv("HOME", t.TempDir())
	before := defaultRoute(t)
	t.Logf("default route before: %s", before)
	if err := directGet("https://discord.com/"); err == nil {
		t.Log("note: discord.com is reachable without bypass on this network")
	} else {
		t.Logf("without TUN, discord.com: %v (blocked, as expected)", err)
	}

	r := newTunRunner()
	p, _ := engine.PresetByID("tr")
	if err := r.Start(p, engine.DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	t.Logf("TUN core running, pid %d, default route now: %s", r.pid, defaultRoute(t))

	time.Sleep(2 * time.Second)
	for _, u := range []string{"https://discord.com/", "https://roblox.com/", "https://www.google.com/"} {
		if err := directGet(u); err != nil {
			t.Errorf("through TUN, %s failed: %v", u, err)
		} else {
			t.Logf("through TUN, %s OK (no proxy configured)", u)
		}
	}
	logs := r.Logs(0)
	t.Logf("core wrote %d log lines", len(logs))

	r.Stop()
	time.Sleep(1500 * time.Millisecond)
	after := defaultRoute(t)
	t.Logf("default route after stop: %s", after)
	if r.Running() {
		t.Error("core still running after Stop")
	}
	if after != before {
		t.Errorf("routes not restored: before %s, after %s", before, after)
	}
	if err := directGet("https://www.google.com/"); err != nil {
		t.Errorf("internet broken after stop: %v", err)
	}
}
