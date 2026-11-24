package mobile

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/engine"
	"github.com/xvzc/spoofdpi/internal/netutil"
)

// DIKEC_LIVE=1 go test -run DNSForwarder -v ./mobile/
func TestDNSForwarderLive(t *testing.T) {
	if os.Getenv("DIKEC_LIVE") == "" {
		t.Skip("set DIKEC_LIVE=1")
	}
	profiles := map[string]engine.Profile{
		"yandex-1253": {
			DNSMode:   "udp",
			DNSAddr:   engine.YandexUDPAlt,
			SplitMode: "none",
		},
		"cloudflare-doh": {
			DNSMode:   "https",
			DoHURL:    engine.CloudflareDoH,
			SplitMode: "none",
		},
		"system(original dst)": {DNSMode: "system", SplitMode: "none"},
	}
	for name, p := range profiles {
		rc := config.DefaultConfig().RuntimeConfig
		if err := engine.ApplyProfile(&rc, p); err != nil {
			t.Fatal(err)
		}
		app, tunSide := net.Pipe()
		dst := &netutil.Destination{Addrs: []net.IP{net.ParseIP("1.1.1.1")}, Port: 53}
		go newDNSForwarder(&rc).serve(context.Background(), tunSide, dst)

		q := new(dns.Msg)
		q.SetQuestion("discord.com.", dns.TypeA)
		raw, _ := q.Pack()
		_ = app.SetDeadline(time.Now().Add(8 * time.Second))
		if _, err := app.Write(raw); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4096)
		n, err := app.Read(buf)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		r := new(dns.Msg)
		if err := r.Unpack(buf[:n]); err != nil || r.Id != q.Id || len(r.Answer) == 0 {
			t.Fatalf("%s: bad reply (%v, %d answers)", name, err, len(r.Answer))
		}
		t.Logf("%-22s discord.com -> %s", name, r.Answer[0].(*dns.A).A)
		_ = app.Close()
	}
}
