// Package mobile is the Dikec DPI core for Android and iOS, exported with
// gomobile. The OS VPN API (Android VpnService, iOS NEPacketTunnelProvider)
// hands over a TUN file descriptor; this package runs the spoofdpi TUN
// server on it with a profile chosen by the app.
//
// Only gomobile-friendly types are used: string, int, bool, error and
// interfaces implemented on the Kotlin/Swift side.
package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/desync"
	"github.com/xvzc/spoofdpi/internal/engine"
	"github.com/xvzc/spoofdpi/internal/rule"
	"github.com/xvzc/spoofdpi/internal/server"
	stun "github.com/xvzc/spoofdpi/internal/server/tun"
	"golang.zx2c4.com/wireguard/tun"
)

// Logger receives log lines from the core (implemented in Kotlin/Swift).
type Logger interface {
	Log(level string, message string)
}

// ScanListener receives auto-bypass progress and the final report as JSON.
type ScanListener interface {
	Progress(json string)
	Done(json string)
}

var (
	mu      sync.Mutex
	cancel  context.CancelFunc
	device  tun.Device
	scanCtl context.CancelFunc
	started time.Time
	current engine.Profile
)

// Version of the core.
func Version() string { return version }

var version = "dev"

// Presets returns the built-in profiles as a JSON array.
func Presets() string {
	b, _ := json.Marshal(engine.Presets())
	return string(b)
}

// DefaultTargets returns the default auto-bypass test sites as JSON.
func DefaultTargets() string {
	b, _ := json.Marshal(engine.DefaultTargets)
	return string(b)
}

// Start runs the TUN server on fd (owned by the caller's VPN interface)
// with the profile given as JSON (the same shape as Presets entries).
func Start(fd int, mtu int, profileJSON string, logger Logger) error {
	var p engine.Profile
	if err := json.Unmarshal([]byte(profileJSON), &p); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	if p.FakeCount > 0 {
		// Fake packets need raw packet injection, which mobile OSes do not
		// allow to apps; fall back to the same profile without them.
		p.FakeCount = 0
	}

	mu.Lock()
	defer mu.Unlock()
	if cancel != nil {
		return errors.New("already running")
	}

	rc := config.DefaultConfig().RuntimeConfig
	if err := engine.ApplyProfile(&rc, p); err != nil {
		return err
	}

	dev, err := createTUNFromFD(fd, mtu)
	if err != nil {
		return fmt.Errorf("tun: %w", err)
	}

	zl := zerolog.New(logWriter{l: logger}).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)
	ruleSet := rule.NewRuleSet()
	tcp := stun.NewTCPHandler(zl, desync.NewTLSDesyncer(zl, nil, nil), ruleSet, &rc)
	udp := stun.NewUDPHandler(zl, desync.NewUDPDesyncer(zl, nil, nil), ruleSet, &rc)
	udp.SetDNSInterceptor(newDNSForwarder(&rc).serve)

	srv := stun.NewTUNServer(zl, tcp, udp, &mobileNet{dev: dev})
	ctx, c := context.WithCancel(context.Background())
	if err := srv.ListenAndServe(ctx); err != nil {
		c()
		_ = dev.Close()
		return err
	}
	cancel, device, started, current = c, dev, time.Now(), p
	zl.Info().Str("profile", p.Label()).Msg("Dikec DPI started")
	return nil
}

// Stop shuts the TUN server down. The caller closes the VPN interface.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if device != nil {
		_ = device.Close()
	}
	cancel, device = nil, nil
}

// IsRunning reports whether the TUN server is active.
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return cancel != nil
}

// RunningSince returns the start time in Unix seconds (0 when stopped).
func RunningSince() int64 {
	mu.Lock()
	defer mu.Unlock()
	if cancel == nil {
		return 0
	}
	return started.Unix()
}

// Scan runs auto-bypass in the background (targets as a JSON array, empty
// for the defaults). The app must exclude itself from its VPN so the scan
// measures the real network.
func Scan(targetsJSON string, l ScanListener) {
	var targets []string
	_ = json.Unmarshal([]byte(targetsJSON), &targets)

	mu.Lock()
	if scanCtl != nil {
		mu.Unlock()
		return
	}
	ctx, c := context.WithTimeout(context.Background(), 5*time.Minute)
	scanCtl = c
	mu.Unlock()

	go func() {
		defer func() {
			mu.Lock()
			scanCtl = nil
			mu.Unlock()
			c()
		}()
		rep, err := engine.Scan(
			ctx,
			engine.ScanOptions{Targets: targets, Thorough: true},
			func(p engine.ScanProgress) {
				if b, err := json.Marshal(p); err == nil {
					l.Progress(string(b))
				}
			},
		)
		out := map[string]any{"report": rep}
		if err != nil {
			out["error"] = err.Error()
		}
		b, _ := json.Marshal(out)
		l.Done(string(b))
	}()
}

// CancelScan stops a running scan.
func CancelScan() {
	mu.Lock()
	defer mu.Unlock()
	if scanCtl != nil {
		scanCtl()
	}
}

// mobileNet adapts the VPN-provided device to the TUN server. Routing and
// DNS are configured by the OS VPN API, so there are no jobs, and the app's
// own sockets already bypass the tunnel (Android: the app excludes itself;
// iOS: the extension's sockets are not routed into its own tunnel).
type mobileNet struct{ dev tun.Device }

func (m *mobileNet) TunDevice() tun.Device { return m.dev }
func (m *mobileNet) FIBID() int            { return 0 }
func (m *mobileNet) BindDialer(*net.Dialer, string, net.IP) error {
	return nil
}
func (m *mobileNet) BuildJobs() ([]server.NetworkJob, error) { return nil, nil }

type logWriter struct{ l Logger }

func (w logWriter) Write(p []byte) (int, error) {
	if w.l == nil {
		return len(p), nil
	}
	var m map[string]any
	if json.Unmarshal(p, &m) == nil {
		lvl, _ := m["level"].(string)
		msg, _ := m["message"].(string)
		delete(m, "level")
		delete(m, "message")
		delete(m, "time")
		if len(m) > 0 {
			extra, _ := json.Marshal(m)
			msg += " " + string(extra)
		}
		w.l.Log(lvl, msg)
	}
	return len(p), nil
}

// CurrentProfile returns the running profile as JSON ("" when stopped).
func CurrentProfile() string {
	mu.Lock()
	defer mu.Unlock()
	if cancel == nil {
		return ""
	}
	b, _ := json.Marshal(current)
	return string(b)
}
