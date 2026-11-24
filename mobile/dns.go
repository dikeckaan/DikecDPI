package mobile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/netutil"
)

// dnsForwarder answers DNS queries that the OS sends into the tunnel by
// relaying the raw message to the profile's resolver: UDP on any port (e.g.
// Yandex 77.88.8.8:1253, which ISPs do not intercept) or DNS-over-HTTPS.
// With the "system" resolver the query goes to its original destination.
type dnsForwarder struct {
	cfg  *config.RuntimeConfig
	http *http.Client
}

func newDNSForwarder(cfg *config.RuntimeConfig) *dnsForwarder {
	return &dnsForwarder{cfg: cfg, http: &http.Client{Timeout: 5 * time.Second}}
}

const dnsIdle = 15 * time.Second

func (f *dnsForwarder) serve(
	ctx context.Context,
	lConn net.Conn,
	dst *netutil.Destination,
) {
	defer func() { _ = lConn.Close() }()
	buf := make([]byte, 65535)
	for {
		_ = lConn.SetReadDeadline(time.Now().Add(dnsIdle))
		n, err := lConn.Read(buf)
		if err != nil || ctx.Err() != nil {
			return
		}
		query := append([]byte(nil), buf[:n]...)
		go func() {
			if reply, err := f.resolve(ctx, query, dst); err == nil {
				_, _ = lConn.Write(reply)
			}
		}()
	}
}

func (f *dnsForwarder) resolve(
	ctx context.Context,
	query []byte,
	dst *netutil.Destination,
) ([]byte, error) {
	switch f.cfg.DNS.Mode {
	case config.DNSModeHTTPS:
		return f.doh(ctx, query)
	case config.DNSModeSystem:
		if len(dst.Addrs) == 0 {
			return nil, errors.New("no destination")
		}
		return udpExchange(
			ctx,
			net.JoinHostPort(dst.Addrs[0].String(), strconv.Itoa(dst.Port)),
			query,
		)
	default:
		return udpExchange(ctx, f.cfg.DNS.Addr.String(), query)
	}
}

func udpExchange(ctx context.Context, addr string, query []byte) ([]byte, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	c, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(4 * time.Second))
	if _, err := c.Write(query); err != nil {
		return nil, err
	}
	buf := make([]byte, 65535)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (f *dnsForwarder) doh(ctx context.Context, query []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		f.cfg.DNS.HTTPSURL,
		bytes.NewReader(query),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("doh: " + resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 65535))
}
