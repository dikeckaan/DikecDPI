package main

import (
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// DIKEC_LIVE=1 go test -run Live -v .
func TestServiceLive(t *testing.T) {
	if os.Getenv("DIKEC_LIVE") == "" {
		t.Skip("set DIKEC_LIVE=1")
	}
	t.Setenv("HOME", t.TempDir())

	a := NewApp()
	a.st.Options.SystemProxy = false // never touch the real network settings in tests
	a.st.Options.ListenAddr = "127.0.0.1:18095"
	var progress atomic.Int32
	done := make(chan ScanDone, 1)
	a.emit = func(name string, data any) {
		switch name {
		case "scan:progress":
			progress.Add(1)
		case "scan:done":
			done <- data.(ScanDone)
		}
	}

	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	pu, _ := url.Parse("http://127.0.0.1:18095")
	c := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(pu)},
	}
	resp, err := c.Get("https://discord.com/")
	if err != nil {
		t.Fatal("discord via proxy:", err)
	}
	resp.Body.Close()
	time.Sleep(300 * time.Millisecond)
	st := a.GetState()
	t.Logf(
		"running=%v profile=%s requests=%d hosts=%v",
		st.Status.Running,
		st.Selected.Label,
		st.Status.Requests,
		a.Hosts(5),
	)
	if st.Status.Requests == 0 {
		t.Error("request was not counted")
	}

	if err := a.StartScan(); err != nil {
		t.Fatal(err)
	}
	d := <-done
	if d.Error != "" {
		t.Fatal(d.Error)
	}
	t.Logf(
		"scan: %d progress events, best=%s, duration=%s",
		progress.Load(),
		d.Summary.Best.Label(),
		d.Summary.Duration,
	)

	if err := a.ApplyAuto(); err != nil {
		t.Fatal(err)
	}
	st = a.GetState()
	if st.SelectedID != "auto" || !st.Status.Running {
		t.Fatalf("auto not applied: %+v", st.SelectedID)
	}
	resp, err = c.Get("https://roblox.com/")
	if err != nil {
		t.Fatal("roblox via auto profile:", err)
	}
	resp.Body.Close()

	a.Stop()
	if a.GetState().Status.Running {
		t.Fatal("still running")
	}
	// settings persisted
	if loadState().AutoProfile == nil {
		t.Fatal("auto profile not saved")
	}
}
