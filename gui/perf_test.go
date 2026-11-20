package main

import (
	"sync"
	"testing"
	"time"
)

// GetState must stay fast while the proxy is starting or stopping; the UI
// polls it every second and used to freeze behind Start's lock.
func TestGetStateStaysFastDuringStart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()
	a.st.Options.SystemProxy = false
	a.st.Options.ListenAddr = "127.0.0.1:18097"

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = a.Start() }()

	var worst time.Duration
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		t0 := time.Now()
		_ = a.GetState()
		if d := time.Since(t0); d > worst {
			worst = d
		}
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()
	a.Stop()
	t.Logf("slowest GetState during start: %s", worst)
	if worst > 50*time.Millisecond {
		t.Fatalf("GetState blocked for %s", worst)
	}
}
