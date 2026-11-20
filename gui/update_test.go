package main

import (
	"context"
	"testing"
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
	t.Logf("latest=%s available=%v viaBrew=%v url=%s", info.Latest, info.Available, info.ViaBrew, info.URL)
	if !info.Available {
		t.Fatalf("1.0.0 should see an update, latest is %s", info.Latest)
	}
}
