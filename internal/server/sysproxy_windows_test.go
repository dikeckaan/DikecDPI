//go:build windows

package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"golang.org/x/sys/windows/registry"
)

func readPAC(t *testing.T) (string, bool) {
	t.Helper()
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		internetSettingsKey,
		registry.QUERY_VALUE,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()
	v, _, err := k.GetStringValue("AutoConfigURL")
	if err != nil {
		return "", false
	}
	return v, true
}

func writePAC(t *testing.T, v string) {
	t.Helper()
	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		internetSettingsKey,
		registry.SET_VALUE,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()
	if v == "" {
		_ = k.DeleteValue("AutoConfigURL")
		return
	}
	if err := k.SetStringValue("AutoConfigURL", v); err != nil {
		t.Fatal(err)
	}
}

// Turning protection on and off must leave the user's proxy settings exactly
// as they were: a corporate PAC is restored, and no value is left behind
// when there was none.
func TestSystemProxyRoundTrip(t *testing.T) {
	orig, had := readPAC(t)
	defer func() {
		if had {
			writePAC(t, orig)
		} else {
			writePAC(t, "")
		}
	}()

	for _, before := range []string{"http://corp.example/proxy.pac", ""} {
		writePAC(t, before)
		state := filepath.Join(t.TempDir(), "state")
		ctx, cancel := context.WithCancel(context.Background())

		if err := SetupSystemProxy(ctx, "PROXY 127.0.0.1:8080", state); err != nil {
			t.Fatal(err)
		}
		if err := ApplyJobs(zerolog.Nop(), state); err != nil {
			t.Fatal(err)
		}
		cur, ok := readPAC(t)
		if !ok || cur == before {
			t.Fatalf("PAC not set (before %q, now %q)", before, cur)
		}
		resp, err := http.Get(cur)
		if err != nil {
			t.Fatal("PAC server:", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if string(body) == "" {
			t.Fatal("empty PAC")
		}
		t.Logf("PAC served at %s", cur)

		ResetJobs(zerolog.Nop(), state)
		cancel()
		after, ok := readPAC(t)
		if before == "" && ok {
			t.Fatalf("AutoConfigURL left behind: %q", after)
		}
		if before != "" && after != before {
			t.Fatalf("corporate PAC not restored: %q", after)
		}
		if _, err := os.Stat(state); !os.IsNotExist(err) {
			t.Fatal("state file not removed")
		}
	}
}
