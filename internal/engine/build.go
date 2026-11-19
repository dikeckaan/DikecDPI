package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/xvzc/spoofdpi/internal/cache"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/desync"
	"github.com/xvzc/spoofdpi/internal/dns"
	"github.com/xvzc/spoofdpi/internal/logging"
	"github.com/xvzc/spoofdpi/internal/netutil"
	"github.com/xvzc/spoofdpi/internal/packet"
	"github.com/xvzc/spoofdpi/internal/rule"
	"github.com/xvzc/spoofdpi/internal/server"
	"github.com/xvzc/spoofdpi/internal/server/http"
	"github.com/xvzc/spoofdpi/internal/server/socks5"
	"github.com/xvzc/spoofdpi/internal/server/tun"
)

// This file holds the server wiring that used to live in cmd/spoofdpi so
// the CLI, the GUI and the auto-bypass scanner can share it.

func setupPcapIO(
	ctx context.Context,
	logger zerolog.Logger,
	route *netutil.Route,
	cfg *config.Config,
) (tcpSniffer, udpSniffer packet.Sniffer, tcpWriter, udpWriter packet.Writer, err error) {
	if !cfg.NeedsPcap() {
		return
	}

	resolveCtx, resolveCancel := context.WithTimeout(ctx, 10*time.Second)
	gatewayMAC, rErr := packet.ResolveGatewayMAC(
		resolveCtx,
		&route.Iface,
		route.LocalIP,
		route.Gateway,
	)
	resolveCancel()
	if rErr != nil {
		err = rErr
		return
	}

	pktLogger := logging.WithScope(logger, "pkt")
	hopCache := cache.NewLRUCache[netutil.IPKey, uint8](16, 8192, nil)

	logger.Info().Msg("network info")
	logger.Info().
		Str("name", route.Iface.Name).
		Str("mac", route.Iface.HardwareAddr.String()).
		Msg(" interface")
	logger.Info().
		Str("mac", gatewayMAC.String()).
		Msg(" gateway")

	if cfg.NeedsPcapTCP() {
		tcpHandle, hErr := packet.NewHandle(&route.Iface)
		if hErr != nil {
			err = fmt.Errorf("tcp pcap handle on %s: %w", route.Iface.Name, hErr)
			return
		}
		tcpSniffer = packet.NewTCPSniffer(
			pktLogger,
			tcpHandle,
			hopCache,
			&cfg.RuntimeConfig,
		)
		tcpWriter = packet.NewTCPWriter(pktLogger, tcpHandle, &route.Iface, gatewayMAC)
		tcpSniffer.StartCapturing()
	}

	if cfg.NeedsPcapUDP() {
		udpHandle, hErr := packet.NewHandle(&route.Iface)
		if hErr != nil {
			err = fmt.Errorf("udp pcap handle on %s: %w", route.Iface.Name, hErr)
			return
		}
		udpSniffer = packet.NewUDPSniffer(
			pktLogger,
			udpHandle,
			hopCache,
			&cfg.RuntimeConfig,
		)
		udpWriter = packet.NewUDPWriter(pktLogger, udpHandle, &route.Iface, gatewayMAC)
		udpSniffer.StartCapturing()
	}

	return
}

// CreateServer builds the proxy server described by cfg.
func CreateServer(
	appctx context.Context,
	logger zerolog.Logger,
	cfg *config.Config,
) (server.Server, error) {
	// --- Rule set ---
	ruleSet := rule.NewRuleSet()
	for _, r := range cfg.Rules {
		if err := ruleSet.Add(&r); err != nil {
			return nil, err
		}
	}

	// --- DNS resolver ---
	resolver := dns.NewClient(
		logging.WithScope(logger, "dns"),
		&cfg.RuntimeConfig,
	)

	// Clean up stale network state before route discovery so a crashed TUN
	// session does not leave the routing table in a state that obscures the
	// real default route.
	server.ResetJobs(logger, tun.StateFile)
	server.ResetJobs(logger, http.StateFile)
	server.ResetJobs(logger, socks5.StateFile)

	var defaultRoute *netutil.Route
	if cfg.NeedsRoute() {
		var err error
		defaultRoute, err = netutil.DiscoverRoute(cfg.App.Gateway)
		if err != nil {
			return nil, fmt.Errorf("failed to find default route: %w", err)
		}
	} else if cfg.App.Gateway != nil {
		logger.Warn().
			Str("gateway", cfg.App.Gateway.String()).
			Msg("gateway is ignored; only fake packets and tun mode use it")
	}

	tcpSniffer, udpSniffer, tcpWriter, udpWriter, err := setupPcapIO(
		appctx,
		logger,
		defaultRoute,
		cfg,
	)
	if err != nil {
		return nil, err
	}

	tlsDesyncer := desync.NewTLSDesyncer(
		logging.WithScope(logger, "dsn"),
		tcpWriter,
		tcpSniffer,
	)
	udpDesyncer := desync.NewUDPDesyncer(
		logging.WithScope(logger, "dsn"),
		udpWriter,
		udpSniffer,
	)

	switch cfg.App.Mode {
	case config.AppModeHTTP:
		return http.NewHTTPProxy(
			logging.WithScope(logger, "srv"),
			http.NewHTTPHandler(logging.WithScope(logger, "hnd")),
			http.NewHTTPSHandler(
				logging.WithScope(logger, "hnd"),
				tlsDesyncer,
				&cfg.RuntimeConfig,
			),
			resolver,
			ruleSet,
			cfg.App.ListenAddr,
			&cfg.RuntimeConfig,
		), nil

	case config.AppModeSOCKS5:
		udpPool := netutil.NewConnRegistry[netutil.NATKey](4096, 60*time.Second)
		udpPool.RunCleanupLoop(appctx)
		return socks5.NewSOCKS5Proxy(
			logging.WithScope(logger, "srv"),
			socks5.NewConnectHandler(
				logging.WithScope(logger, "hnd"),
				tlsDesyncer,
				cfg.App.ListenAddr,
				&cfg.RuntimeConfig,
			),
			socks5.NewBindHandler(logging.WithScope(logger, "hnd")),
			socks5.NewUdpAssociateHandler(
				logging.WithScope(logger, "hnd"),
				udpPool,
				udpDesyncer,
				&cfg.RuntimeConfig,
			),
			resolver,
			ruleSet,
			cfg.App.ListenAddr,
			&cfg.RuntimeConfig,
		), nil

	case config.AppModeTUN:
		logger.Info().
			Str("interface", defaultRoute.Iface.Name).
			Str("gateway", defaultRoute.Gateway.String()).
			Msg("determined default interface and gateway")
		sysNet, err := tun.NewTUNSystemNetwork(
			logging.WithScope(logger, "sys"),
			defaultRoute,
			cfg.App.FreebsdFIB,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create sysnet: %w", err)
		}
		return tun.NewTUNServer(
			logging.WithScope(logger, "srv"),
			tun.NewTCPHandler(
				logging.WithScope(logger, "hnd"),
				tlsDesyncer,
				ruleSet,
				&cfg.RuntimeConfig,
			),
			tun.NewUDPHandler(
				logging.WithScope(logger, "hnd"),
				udpDesyncer,
				ruleSet,
				&cfg.RuntimeConfig,
			),
			sysNet,
		), nil

	default:
		return nil, fmt.Errorf("unknown server mode: %s", cfg.App.Mode)
	}
}
