// Package engine runs the proxy in-process so that front-ends (the Dikec DPI
// desktop app, the auto-bypass scanner) can start, stop and observe it.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/server"
	"github.com/xvzc/spoofdpi/internal/server/http"
	"github.com/xvzc/spoofdpi/internal/server/socks5"
	"github.com/xvzc/spoofdpi/internal/server/tun"
	"github.com/xvzc/spoofdpi/internal/session"
)

const (
	maxLogEntries = 1500
	maxHosts      = 500
)

// LogEntry is a single structured log line.
type LogEntry struct {
	Seq    uint64            `json:"seq"`
	Time   time.Time         `json:"time"`
	Level  string            `json:"level"`
	Scope  string            `json:"scope"`
	Msg    string            `json:"msg"`
	Fields map[string]string `json:"fields,omitempty"`
}

// HostStat counts proxied requests per destination host.
type HostStat struct {
	Host     string    `json:"host"`
	Requests uint64    `json:"requests"`
	Failures uint64    `json:"failures"`
	LastSeen time.Time `json:"lastSeen"`
}

// Status describes the engine state.
type Status struct {
	Running     bool      `json:"running"`
	Starting    bool      `json:"starting"`
	Addr        string    `json:"addr"`
	Mode        string    `json:"mode"`
	SplitMode   string    `json:"splitMode"`
	ChunkSize   int       `json:"chunkSize"`
	Disorder    bool      `json:"disorder"`
	DNS         string    `json:"dns"`
	SystemProxy bool      `json:"systemProxy"`
	Since       time.Time `json:"since"`
	Requests    uint64    `json:"requests"`
	Failures    uint64    `json:"failures"`
	LastError   string    `json:"lastError"`
}

// Engine owns one running proxy server at a time.
type Engine struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	stateFile string
	status    Status

	logMu sync.Mutex
	logs  []LogEntry
	seq   uint64
	hosts map[string]*HostStat
}

func New() *Engine {
	return &Engine{hosts: make(map[string]*HostStat)}
}

// Start launches the proxy with cfg. cfg must already be finalized.
func (e *Engine) Start(cfg *config.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		return errors.New("zaten çalışıyor")
	}

	ctx, cancel := context.WithCancel(session.WithNewTraceID(context.Background()))
	logger := zerolog.New(&sink{e: e}).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	srv, err := CreateServer(ctx, logger, cfg)
	if err != nil {
		cancel()
		e.status.LastError = err.Error()
		return err
	}
	if err := srv.ListenAndServe(ctx); err != nil {
		cancel()
		e.status.LastError = err.Error()
		return err
	}

	e.cancel = cancel
	e.status = Status{
		Running:   true,
		Addr:      srv.Addr(),
		Mode:      cfg.App.Mode.String(),
		SplitMode: cfg.HTTPS.SplitMode.String(),
		ChunkSize: int(cfg.HTTPS.ChunkSize),
		Disorder:  cfg.HTTPS.Disorder,
		DNS:       DescribeDNS(cfg.DNS),
		Since:     time.Now(),
	}
	logger.Info().Str("addr", srv.Addr()).Msg("Dikec DPI başlatıldı")

	if cfg.App.AutoConfigureNetwork {
		stateFile, err := srv.SetupNetworkJobs(ctx)
		if err == nil && stateFile != "" {
			err = server.ApplyJobs(logger, stateFile)
		}
		if err != nil {
			logger.Error().Err(err).Msg("sistem proxy'si ayarlanamadı")
			e.status.LastError = "sistem proxy'si ayarlanamadı: " + err.Error()
		} else if stateFile != "" {
			e.stateFile = stateFile
			e.status.SystemProxy = true
		}
	}

	return nil
}

// Stop shuts the proxy down and restores the network settings.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

func (e *Engine) stopLocked() {
	if e.cancel == nil {
		return
	}
	logger := zerolog.New(&sink{e: e}).With().Timestamp().Logger()
	if e.stateFile != "" {
		server.ResetJobs(logger, e.stateFile)
		e.stateFile = ""
	}
	e.cancel()
	e.cancel = nil
	e.status.Running = false
	e.status.SystemProxy = false
	logger.Info().Msg("Dikec DPI durduruldu")
}

// CleanupStaleState resets network settings left behind by a crashed run.
func CleanupStaleState() {
	logger := zerolog.Nop()
	server.ResetJobs(logger, http.StateFile)
	server.ResetJobs(logger, socks5.StateFile)
	server.ResetJobs(logger, tun.StateFile)
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	st := e.status
	e.mu.Unlock()

	e.logMu.Lock()
	for _, h := range e.hosts {
		st.Requests += h.Requests
		st.Failures += h.Failures
	}
	e.logMu.Unlock()
	return st
}

// Logs returns entries with a sequence number greater than after.
func (e *Engine) Logs(after uint64) []LogEntry {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	i := sort.Search(len(e.logs), func(i int) bool { return e.logs[i].Seq > after })
	out := make([]LogEntry, len(e.logs)-i)
	copy(out, e.logs[i:])
	return out
}

// Hosts returns the most active hosts, busiest first.
func (e *Engine) Hosts(limit int) []HostStat {
	e.logMu.Lock()
	out := make([]HostStat, 0, len(e.hosts))
	for _, h := range e.hosts {
		out = append(out, *h)
	}
	e.logMu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Requests != out[j].Requests {
			return out[i].Requests > out[j].Requests
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (e *Engine) ResetStats() {
	e.logMu.Lock()
	e.hosts = make(map[string]*HostStat)
	e.logMu.Unlock()
}

// sink receives JSON log lines from zerolog.
type sink struct{ e *Engine }

func (s *sink) Write(p []byte) (int, error) {
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		return len(p), nil
	}
	s.e.record(m)
	return len(p), nil
}

func (e *Engine) record(m map[string]any) {
	entry := LogEntry{Time: time.Now(), Fields: map[string]string{}}
	for k, v := range m {
		switch k {
		case zerolog.LevelFieldName:
			entry.Level = fmt.Sprint(v)
		case zerolog.MessageFieldName:
			entry.Msg = fmt.Sprint(v)
		case zerolog.TimestampFieldName:
		case "scope":
			entry.Scope = fmt.Sprint(v)
		case "local_scope":
			entry.Fields["step"] = fmt.Sprint(v)
		default:
			entry.Fields[k] = fmt.Sprint(v)
		}
	}

	e.logMu.Lock()
	defer e.logMu.Unlock()

	// Per-host statistics are derived from the connection logs.
	if host, ok := entry.Fields["host"]; ok && entry.Msg == "new request" {
		h := e.hosts[host]
		if h == nil {
			if len(e.hosts) >= maxHosts {
				e.evictOldestHostLocked()
			}
			h = &HostStat{Host: host}
			e.hosts[host] = h
		}
		h.Requests++
		h.LastSeen = entry.Time
	}
	if entry.Level == "error" || entry.Level == "warn" {
		if host, ok := entry.Fields["host"]; ok {
			if h := e.hosts[host]; h != nil {
				h.Failures++
			}
		}
	}

	// Per-request debug noise is counted but not kept in the visible log.
	if entry.Level == "debug" || entry.Level == "trace" {
		return
	}

	e.seq++
	entry.Seq = e.seq
	e.logs = append(e.logs, entry)
	if len(e.logs) > maxLogEntries {
		e.logs = append([]LogEntry(nil), e.logs[len(e.logs)-maxLogEntries:]...)
	}
}

func (e *Engine) evictOldestHostLocked() {
	var oldest *HostStat
	for _, h := range e.hosts {
		if oldest == nil || h.LastSeen.Before(oldest.LastSeen) {
			oldest = h
		}
	}
	if oldest != nil {
		delete(e.hosts, oldest.Host)
	}
}

// DescribeDNS renders DNS options for humans.
func DescribeDNS(o config.DNSOptions) string {
	switch o.Mode {
	case config.DNSModeHTTPS:
		return "DoH " + o.HTTPSURL
	case config.DNSModeSystem:
		return "Sistem DNS"
	default:
		return "UDP " + o.Addr.String()
	}
}
