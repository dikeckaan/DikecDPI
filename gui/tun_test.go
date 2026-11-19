package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/xvzc/spoofdpi/internal/engine"
)

func TestCoreArgsAreAcceptedByCLI(t *testing.T) {
	for _, p := range engine.Presets() {
		args := coreArgs(p, engine.DefaultOptions())
		joined := strings.Join(args, " ")
		for _, want := range []string{"--app-mode tun", "--https-split-mode " + p.SplitMode, "--auto-configure-network"} {
			if !strings.Contains(joined, want) {
				t.Errorf("%s: missing %q in %s", p.ID, want, joined)
			}
		}
	}
}

// The quoting must survive the AppleScript -> sh round trip unchanged.
func TestShellQuoting(t *testing.T) {
	tricky := `/Applications/Dikec DPI.app/x'y"z\w $HOME`
	out, err := exec.Command("/bin/sh", "-c", "printf %s "+shQuote(tricky)).Output()
	if err != nil || string(out) != tricky {
		t.Fatalf("shQuote round trip: %q %v", out, err)
	}
	script := "printf %s " + shQuote(tricky)
	apple := `do shell script "/bin/sh -c " & quoted form of ` + asString(script)
	out, err = exec.Command("/usr/bin/osascript", "-e", apple).Output()
	if err != nil || strings.TrimSpace(string(out)) != tricky {
		t.Fatalf("applescript round trip: %q %v", out, err)
	}
}
