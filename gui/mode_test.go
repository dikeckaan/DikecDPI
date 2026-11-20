package main

import (
	"testing"

	"github.com/xvzc/spoofdpi/internal/engine"
)

// Choosing TUN must persist, and survive a reload (the reported bug was the
// UI reverting to proxy).
func TestSaveTunModePersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()
	o := a.GetState().Options
	o.AppMode = "tun"
	if err := a.SaveOptions(o); err != nil {
		t.Fatal(err)
	}
	if got := a.GetState().Options.AppMode; got != "tun" {
		t.Fatalf("in memory: %q", got)
	}
	if got := loadState().Options.AppMode; got != "tun" {
		t.Fatalf("on disk: %q", got)
	}
	// switching back works too
	o.AppMode = "http"
	if err := a.SaveOptions(o); err != nil {
		t.Fatal(err)
	}
	if got := loadState().Options.AppMode; got != "http" {
		t.Fatalf("back to http: %q", got)
	}
	_ = engine.DefaultOptions()
}
