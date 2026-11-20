package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xvzc/spoofdpi/internal/engine"
)

// TUN mode needs root to create the utun interface and rewrite the routing
// table, so the GUI (which runs as the user) starts the bundled CLI core
// through the standard macOS administrator prompt. The password is entered
// in the system dialog; the app never sees it.
//
// The root process is wrapped in a small watchdog that stops the core (which
// then restores the routes) when either
//   - the stop file appears (the user pressed Stop: no second password), or
//   - the GUI process is gone (quit or crash).

type tunRunner struct {
	mu       sync.Mutex
	starting bool
	pid      int
	since    time.Time
	profile  engine.Profile
	dir      string
}

func newTunRunner() *tunRunner {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return &tunRunner{dir: filepath.Join(dir, "DikecDPI", "tun")}
}

func (t *tunRunner) paths() (pidFile, stopFile, logFile string) {
	return filepath.Join(t.dir, "core.pid"), filepath.Join(t.dir, "stop"), filepath.Join(t.dir, "core.log")
}

// corePath finds the CLI core: inside the .app bundle, else on PATH (dev).
func corePath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "dikecdpi-core")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("dikecdpi"); err == nil {
		return p, nil
	}
	return "", errors.New("TUN çekirdeği (dikecdpi-core) bulunamadı")
}

// coreArgs translates a profile into CLI flags for the core.
func coreArgs(p engine.Profile, o engine.Options) []string {
	args := []string{
		"--app-mode", "tun", "--no-tui", "--clean", "--log-level", "info",
		"--auto-configure-network",
		"--dns-mode", orStr(p.DNSMode, "udp"),
		"--https-split-mode", orStr(p.SplitMode, "tlsrec"),
		fmt.Sprintf("--dns-cache=%t", o.DNSCache),
		fmt.Sprintf("--https-disorder=%t", p.Disorder),
	}
	if p.DNSAddr != "" {
		args = append(args, "--dns-addr", p.DNSAddr)
	}
	if p.DoHURL != "" {
		args = append(args, "--dns-https-url", p.DoHURL)
	}
	if p.ChunkSize > 0 {
		args = append(args, "--https-chunk-size", strconv.Itoa(p.ChunkSize))
	}
	if p.FakeCount > 0 {
		args = append(args, "--https-fake-count", strconv.Itoa(p.FakeCount))
	}
	if o.IPv6 {
		args = append(args, "--dns-qtype", "all")
	}
	return args
}

func orStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// shQuote quotes s for /bin/sh.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// asString renders s as an AppleScript string literal.
func asString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func (t *tunRunner) Start(p engine.Profile, o engine.Options) error {
	if runtime.GOOS != "darwin" {
		return errors.New("TUN modu masaüstü uygulamasında şimdilik yalnızca macOS'ta destekleniyor; Linux'ta 'sudo dikecdpi --app-mode tun' kullanın")
	}
	t.mu.Lock()
	if t.aliveLocked() || t.starting {
		t.mu.Unlock()
		return nil
	}
	t.starting = true
	t.mu.Unlock()
	// The password dialog can stay open for minutes: never hold the lock
	// while it is up, so Status/Running stay instant.
	defer func() {
		t.mu.Lock()
		t.starting = false
		t.mu.Unlock()
	}()

	core, err := corePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(t.dir, 0o700); err != nil {
		return err
	}
	pidFile, stopFile, logFile := t.paths()
	_ = os.Remove(stopFile)
	_ = os.Remove(pidFile)
	_ = os.Remove(logFile)

	quoted := make([]string, 0, 16)
	quoted = append(quoted, shQuote(core))
	for _, a := range coreArgs(p, o) {
		quoted = append(quoted, shQuote(a))
	}

	// The watchdog loop runs as root. It only reads files in the user's own
	// directory; it never executes them.
	script := fmt.Sprintf(
		`%s >%s 2>&1 & CP=$!; echo $CP >%s; `+
			`while kill -0 $CP 2>/dev/null && kill -0 %d 2>/dev/null && [ ! -e %s ]; do sleep 0.3; done; `+
			`kill -INT $CP 2>/dev/null; wait $CP; rm -f %s %s`,
		strings.Join(quoted, " "), shQuote(logFile), shQuote(pidFile),
		os.Getpid(), shQuote(stopFile), shQuote(pidFile), shQuote(stopFile),
	)
	apple := fmt.Sprintf(
		`do shell script "/bin/sh -c " & quoted form of %s & " >/dev/null 2>&1 &" with administrator privileges with prompt %s`,
		asString(script),
		asString("Dikec DPI, TUN modunda tüm sistem trafiğini yönlendirmek için yönetici izni istiyor."),
	)
	out, err := exec.Command("/usr/bin/osascript", "-e", apple).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "-128") {
			return errors.New("yönetici izni verilmedi")
		}
		return fmt.Errorf("TUN başlatılamadı: %s", strings.TrimSpace(string(out)))
	}

	// Wait for the core to come up (or fail).
	pid := 0
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if n := readPid(pidFile); n > 0 && processAlive(n) {
			pid = n
			if strings.Contains(readFile(logFile), "server started") {
				break
			}
		} else if n > 0 {
			break // started and already exited
		}
		time.Sleep(150 * time.Millisecond)
	}
	if pid == 0 || !processAlive(pid) {
		return fmt.Errorf("TUN çekirdeği çalışmadı: %s", lastError(readFile(logFile)))
	}
	t.mu.Lock()
	t.pid, t.since, t.profile = pid, time.Now(), p
	t.mu.Unlock()
	return nil
}

func (t *tunRunner) Stop() {
	t.mu.Lock()
	pid := t.pid
	t.mu.Unlock()
	if pid == 0 {
		return
	}
	_, stopFile, _ := t.paths()
	_ = os.WriteFile(stopFile, []byte("stop\n"), 0o600)
	// The watchdog polls every 0.3 s; the core then restores the routes.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && processAlive(pid) {
		time.Sleep(150 * time.Millisecond)
	}
	t.mu.Lock()
	if t.pid == pid {
		t.pid = 0
	}
	t.mu.Unlock()
}

func (t *tunRunner) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.aliveLocked()
}

func (t *tunRunner) aliveLocked() bool {
	if t.pid == 0 {
		return false
	}
	if !processAlive(t.pid) {
		t.pid = 0
		return false
	}
	return true
}

func (t *tunRunner) Status(o engine.Options) engine.Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.aliveLocked() {
		return engine.Status{Mode: "tun"}
	}
	p := t.profile
	return engine.Status{
		Running:   true,
		Addr:      "utun · tüm sistem",
		Mode:      "tun",
		SplitMode: p.SplitMode,
		ChunkSize: p.ChunkSize,
		Disorder:  p.Disorder,
		DNS:       p.Label(),
		Since:     t.since,
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Logs returns the core's log lines after line number `after`.
func (t *tunRunner) Logs(after uint64) []engine.LogEntry {
	_, _, logFile := t.paths()
	f, err := os.Open(logFile)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []engine.LogEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var n uint64
	for sc.Scan() {
		n++
		if n <= after {
			continue
		}
		line := strings.TrimSpace(ansi.ReplaceAllString(sc.Text(), ""))
		if line == "" {
			continue
		}
		level := "info"
		switch {
		case strings.HasPrefix(line, "ERR"), strings.HasPrefix(line, "FTL"):
			level = "error"
		case strings.HasPrefix(line, "WRN"):
			level = "warn"
		case strings.HasPrefix(line, "DBG"):
			level = "debug"
		}
		out = append(out, engine.LogEntry{Seq: n, Time: time.Now(), Level: level, Scope: "tun", Msg: line})
	}
	if n < after {
		// The log was recreated by a new start: send it from the beginning.
		return t.Logs(0)
	}
	return out
}

func readPid(path string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(readFile(path)))
	return n
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

// processAlive works for root-owned processes too: EPERM means "exists".
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func lastError(log string) string {
	log = ansi.ReplaceAllString(log, "")
	lines := strings.Split(strings.TrimSpace(log), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "ERR") || strings.Contains(lines[i], "error") || strings.Contains(lines[i], "failed") {
			return strings.TrimSpace(lines[i])
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		return lines[len(lines)-1]
	}
	return "bilinmeyen hata (kayıt boş)"
}
