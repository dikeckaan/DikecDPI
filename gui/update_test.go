package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNewerThan(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.2.0", "1.1.1", true},
		{"1.10.0", "1.9.3", true},
		{"v2.0.0", "1.99.99", true},
		{"1.1.1", "1.1.1", false},
		{"1.1.0", "1.1.1", false},
		{"1.2.0", "dev", false},
		{"garbage", "1.0.0", false},
		{"1.2.0-beta", "1.1.0", true},
	}
	for _, c := range cases {
		if got := newerThan(c.latest, c.current); got != c.want {
			t.Errorf("newerThan(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// Hits the real GitHub API; run with -run LiveUpdate.
func TestLiveUpdateCheck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	old := version
	version = "1.0.0"
	defer func() { version = old }()
	u := newUpdater()
	info := u.check(context.Background())
	if info.Error != "" {
		t.Skipf("network: %s", info.Error)
	}
	t.Logf(
		"latest=%s available=%v viaBrew=%v url=%s",
		info.Latest,
		info.Available,
		info.ViaBrew,
		info.URL,
	)
	if !info.Available {
		t.Fatalf("1.0.0 should see an update, latest is %s", info.Latest)
	}
}

// The update script must wait for the app to exit before running brew, then
// reopen the app. Uses stand-ins for brew and open.
func TestUpdateScriptOrdering(t *testing.T) {
	dir := t.TempDir()
	trace := dir + "/trace"
	brew := dir + "/brew"
	_ = os.WriteFile(
		brew,
		[]byte("#!/bin/sh\necho \"brew $1 $(date +%s%N)\" >> "+shQuote(trace)+"\n"),
		0o755,
	)
	app := exec.Command("/bin/sleep", "30")
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	openCmd := "echo open >> " + shQuote(trace)
	cmd := exec.Command(
		"/bin/sh",
		"-c",
		updateScript(brew, openCmd, dir+"/log", app.Process.Pid),
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if b, _ := os.ReadFile(trace); len(b) != 0 {
		t.Fatalf("brew ran while the app was still running: %s", b)
	}
	_ = app.Process.Kill()
	_ = app.Wait()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(trace)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "brew update") ||
		!strings.HasPrefix(lines[1], "brew upgrade") ||
		lines[2] != "open" {
		t.Fatalf("unexpected order:\n%s", b)
	}
}
