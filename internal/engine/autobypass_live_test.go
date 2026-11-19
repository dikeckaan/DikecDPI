package engine

import (
	"context"
	"os"
	"testing"
	"time"
)

// Run with DIKEC_LIVE=1 go test ./internal/engine -run Live -v
func TestScanLive(t *testing.T) {
	if os.Getenv("DIKEC_LIVE") == "" {
		t.Skip("set DIKEC_LIVE=1 to scan the real network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	rep, err := Scan(ctx, ScanOptions{Thorough: true}, func(p ScanProgress) {
		if p.Combo != nil {
			t.Logf(
				"[%s %d/%d] %-45s %d/%d %dms",
				p.Stage,
				p.Done,
				p.Total,
				p.Combo.Profile.Name,
				p.Combo.Passed,
				p.Combo.Total,
				p.Combo.AvgMillis,
			)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("blocked=%v reachable=%v", rep.Blocked, rep.Reachable)
	for _, d := range rep.DNS {
		t.Logf("dns %-22s ok=%v %dms %s", d.Name, d.OK, d.AvgMillis, d.Error)
	}
	if rep.Best == nil {
		t.Fatal("no working profile")
	}
	t.Logf("BEST: %s (took %s)", rep.Best.Label(), rep.Duration.Round(time.Second))
}
