package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/engine"
	"github.com/xvzc/spoofdpi/internal/logging"
	"github.com/xvzc/spoofdpi/internal/server"
	"github.com/xvzc/spoofdpi/internal/session"
)

// Version and commit are set at build time.
var (
	version = "dev"
	commit  = "unknown"
	build   = "unknown"
)

// SwitchableWriter lets the log destination be swapped while connection
// goroutines are already logging, so access to target is synchronized.
type SwitchableWriter struct {
	mu     sync.RWMutex
	target io.Writer
}

func (sw *SwitchableWriter) SetWriter(w io.Writer) {
	sw.mu.Lock()
	sw.target = w
	sw.mu.Unlock()
}

func (sw *SwitchableWriter) Write(p []byte) (n int, err error) {
	sw.mu.RLock()
	w := sw.target
	sw.mu.RUnlock()
	return w.Write(p)
}

type DelayedWriter struct {
	writer io.Writer
	delay  time.Duration
}

// DelayedWriter is stateless, so value receiver is technically fine,
// but pointer receiver is preferred for consistency in Go.
func (dw *DelayedWriter) Write(p []byte) (n int, err error) {
	if dw.delay > 0 {
		time.Sleep(dw.delay)
	}
	return dw.writer.Write(p)
}

func main() {
	cmd := config.CreateCommand(runApp, version, commit, build)
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println("application failed to start")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runApp(mainctx context.Context, configDir string, cfg *config.Config) (err error) {
	appctx, cancel := signal.NotifyContext(
		session.WithNewTraceID(mainctx),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP,
	)
	defer cancel()

	var writer io.Writer
	// Channel to capture critical TUI execution failures
	if !cfg.App.NoTUI {
		if err := startTUI(cancel); err != nil {
			return fmt.Errorf("failed to start tui: %w", err)
		}
		writer = TUIWriter{}
	} else {
		writer = os.Stdout
	}

	dw := &DelayedWriter{
		writer: writer,
		delay:  29 * time.Millisecond,
	}
	sw := &SwitchableWriter{target: dw}

	logging.SetGlobalLogger(appctx, cfg.App.LogLevel, sw)
	logger := log.Logger.With().Ctx(appctx).Logger()

	// In TUI mode the alt-screen has already been claimed by startTUI.
	// Letting a setup error bubble up via `return err` would land in
	// main's fmt.Println + os.Exit(1) path, which tears down the alt
	// screen before the user can read what went wrong.
	//
	// Catch the error here instead: log it through the configured logger
	// so it shows up in the TUI, park on appctx until the user dismisses
	// with Ctrl+C, and clear `err` so main exits cleanly.
	//
	// Headless mode keeps the original behavior — the error propagates
	// to main, which prints "application failed to start" to stderr and
	// exits with status 1.
	defer func() {
		if err == nil || cfg.App.NoTUI {
			return
		}
		logger.Error().Err(err).Msg("application failed to start")
		<-appctx.Done()
		err = nil
	}()

	logger.Info().Str("version", version).Msg("dikecdpi")
	if configDir != "" {
		logger.Info().
			Str("dir", configDir).
			Msgf("loaded config file")
	} else {
		logger.Warn().
			Msg("config file not found")
		logger.Warn().
			Msg(" please try 'sudo -E spoofdpi' if you expect a configuration to be loaded")
	}

	for _, m := range config.WarnMsgs {
		logger.Warn().Msg(m)
	}

	logger.Info().Str("mode", cfg.App.Mode.String()).Msgf("app")

	srv, err := engine.CreateServer(appctx, logger, cfg)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	logger.Info().Msg("https info")
	logger.Info().
		Str("split-mode", cfg.HTTPS.SplitMode.String()).
		Uint8("chunk-size", uint8(cfg.HTTPS.ChunkSize)).
		Bool("disorder", cfg.HTTPS.Disorder).
		Msg(" split")

	logger.Info().
		Uint8("count", uint8(cfg.HTTPS.FakeCount)).
		Msg(" fake")

	if cfg.Conn.DNSTimeout > 0 {
		logger.Info().
			Str("value", fmt.Sprintf("%dms", cfg.Conn.DNSTimeout.Milliseconds())).
			Msgf("dns connection timeout")
	}
	if cfg.Conn.TCPTimeout > 0 {
		logger.Info().
			Str("value", fmt.Sprintf("%dms", cfg.Conn.TCPTimeout.Milliseconds())).
			Msgf("tcp connection timeout")
	}
	if cfg.Conn.UDPIdleTimeout > 0 {
		logger.Info().
			Str("value", fmt.Sprintf("%dms", cfg.Conn.UDPIdleTimeout.Milliseconds())).
			Msgf("udp idle timeout")
	}

	time.Sleep(300 * time.Millisecond)
	if err := srv.ListenAndServe(appctx); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}
	logger.Info().Msgf("server started on %s", srv.Addr())
	if cfg.App.AutoConfigureNetwork {
		if stateFile, acErr := srv.SetupNetworkJobs(appctx); acErr != nil {
			// Non-fatal: server is running, just couldn't auto-set
			// system proxy. Log and continue rather than tearing down.
			logger.Error().Err(acErr).Msg("failed to set system network config")
		} else if acErr := server.ApplyJobs(logger, stateFile); acErr != nil {
			logger.Error().Err(acErr).Msg("failed to apply network config")
		} else {
			defer server.ResetJobs(logger, stateFile)
		}
	}

	sw.SetWriter(writer)

	<-appctx.Done()

	return nil
}
