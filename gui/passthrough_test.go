package main

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// A client that pinned the proxy address (like a Claude Desktop session with
// HTTPS_PROXY set) must keep working after protection is stopped, and
// protection must be able to take the port back.
// DIKEC_LIVE=1 go test -run PinnedClient -v .
func TestPinnedClientSurvivesStop(t *testing.T) {
	if os.Getenv("DIKEC_LIVE") == "" {
		t.Skip("set DIKEC_LIVE=1")
	}
	t.Setenv("HOME", t.TempDir())
	a := NewApp()
	a.st.Options.SystemProxy = false
	a.st.Options.ListenAddr = "127.0.0.1:18099"

	pu, _ := url.Parse("http://127.0.0.1:18099")
	get := func() error {
		c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), DisableKeepAlives: true}}
		r, err := c.Get("https://www.google.com/")
		if err == nil {
			_ = r.Body.Close()
		}
		return err
	}

	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	if err := get(); err != nil {
		t.Fatal("through protection:", err)
	}
	a.Stop()
	st := a.GetState()
	if st.Status.Running || !st.Standby {
		t.Fatalf("after stop: running=%v standby=%v", st.Status.Running, st.Standby)
	}
	if err := get(); err != nil {
		t.Fatal("pinned client after stop (pass-through):", err)
	}
	if n := a.PinnedConnections(); n < 0 {
		t.Fatal("bad count")
	}
	if err := a.Start(); err != nil {
		t.Fatal("restart after standby:", err)
	}
	if a.GetState().Standby {
		t.Fatal("standby still on while protection runs")
	}
	if err := get(); err != nil {
		t.Fatal("through protection again:", err)
	}
	_ = a.ServiceShutdown()
	if err := get(); err == nil {
		t.Fatal("port should be closed after shutdown")
	}
	t.Log("pinned client: protected → pass-through → protected → closed on quit: OK")
}
