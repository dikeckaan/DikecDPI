package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/xvzc/spoofdpi/internal/config"
	"github.com/xvzc/spoofdpi/internal/desync"
	"github.com/xvzc/spoofdpi/internal/dns"
	"github.com/xvzc/spoofdpi/internal/netutil"
	"github.com/xvzc/spoofdpi/internal/packet"
	"github.com/xvzc/spoofdpi/internal/rule"
	proxyhttp "github.com/xvzc/spoofdpi/internal/server/http"
)

// DefaultTargets are sites known to be blocked by DPI in Türkiye.
var DefaultTargets = []string{"discord.com", "roblox.com", "www.wattpad.com"}

// ScanOptions controls an auto-bypass scan.
type ScanOptions struct {
	Targets     []string
	Concurrency int
	Timeout     time.Duration
	// Thorough also tries the best strategies with every working resolver.
	Thorough bool
}

// TargetResult is the outcome of one site for one combination.
type TargetResult struct {
	Target  string        `json:"target"`
	OK      bool          `json:"ok"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
}

// ComboResult is the score of one profile.
type ComboResult struct {
	Profile   Profile        `json:"profile"`
	Passed    int            `json:"passed"`
	Total     int            `json:"total"`
	AvgMillis int64          `json:"avgMillis"`
	Targets   []TargetResult `json:"targets"`
}

// DNSResult is the outcome of the resolver stage.
type DNSResult struct {
	Name      string `json:"name"`
	Profile   Profile
	OK        bool   `json:"ok"`
	AvgMillis int64  `json:"avgMillis"`
	Error     string `json:"error,omitempty"`
}

// ScanProgress is reported while scanning.
type ScanProgress struct {
	Stage   string       `json:"stage"`
	Done    int          `json:"done"`
	Total   int          `json:"total"`
	Message string       `json:"message"`
	Combo   *ComboResult `json:"combo,omitempty"`
}

// ScanReport is the final result.
type ScanReport struct {
	Blocked   []string      `json:"blocked"`   // targets unreachable without help
	Reachable []string      `json:"reachable"` // targets that work directly
	DNS       []DNSResult   `json:"dns"`
	Results   []ComboResult `json:"results"` // best first
	Best      *Profile      `json:"best,omitempty"`
	Duration  time.Duration `json:"duration"`
}

type strategy struct {
	split     string
	chunk     int
	disorder  bool
	fake      int
	needsRoot bool
}

func strategies(root bool) []strategy {
	s := []strategy{
		{split: "chunk", chunk: 5},
		{split: "tlsrec"},
		{split: "first-byte"},
		{split: "chunk", chunk: 1},
		{split: "chunk", chunk: 2},
		{split: "chunk", chunk: 3},
		{split: "chunk", chunk: 8},
		{split: "chunk", chunk: 16},
		{split: "chunk", chunk: 35},
		{split: "sni"},
		{split: "random"},
		{split: "tlsrec", disorder: true},
		{split: "chunk", chunk: 1, disorder: true},
		{split: "chunk", chunk: 5, disorder: true},
		{split: "first-byte", disorder: true},
		{split: "sni", disorder: true},
		{split: "none"},
	}
	if root {
		s = append(s,
			strategy{split: "sni", fake: 3, needsRoot: true},
			strategy{split: "chunk", chunk: 5, fake: 3, needsRoot: true},
			strategy{split: "tlsrec", disorder: true, fake: 3, needsRoot: true},
		)
	}
	return s
}

func dnsCandidates() []Profile {
	return []Profile{
		{Name: "Yandex (UDP 1253)", DNSMode: "udp", DNSAddr: YandexUDPAlt},
		{Name: "Cloudflare DoH", DNSMode: "https", DoHURL: CloudflareDoH},
		{Name: "Google DoH", DNSMode: "https", DoHURL: GoogleDoH},
		{Name: "Quad9 DoH", DNSMode: "https", DoHURL: Quad9DoH},
		{Name: "Cloudflare (UDP 53)", DNSMode: "udp", DNSAddr: CloudflareUDP},
		{Name: "Sistem DNS", DNSMode: "system"},
	}
}

// Scan finds the profile that unblocks the most targets on this network.
func Scan(
	ctx context.Context,
	opts ScanOptions,
	progress func(ScanProgress),
) (*ScanReport, error) {
	start := time.Now()
	if progress == nil {
		progress = func(ScanProgress) {}
	}
	if len(opts.Targets) == 0 {
		opts.Targets = DefaultTargets
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 8 * time.Second
	}

	report := &ScanReport{}

	// Stage 1: is the internet up, and which targets are blocked at all?
	progress(
		ScanProgress{Stage: "baseline", Message: "İnternet bağlantısı kontrol ediliyor"},
	)
	if r := fetch(ctx, nil, "www.google.com", opts.Timeout); !r.OK {
		if r := fetch(ctx, nil, "www.cloudflare.com", opts.Timeout); !r.OK {
			return nil, fmt.Errorf("internet bağlantısı yok gibi görünüyor: %s", r.Error)
		}
	}
	for i, t := range opts.Targets {
		progress(
			ScanProgress{
				Stage:   "baseline",
				Done:    i,
				Total:   len(opts.Targets),
				Message: t + " doğrudan deneniyor",
			},
		)
		if r := fetch(ctx, nil, t, opts.Timeout); r.OK {
			report.Reachable = append(report.Reachable, t)
		} else {
			report.Blocked = append(report.Blocked, t)
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Stage 2: which resolvers answer for every target?
	cands := dnsCandidates()
	for i, c := range cands {
		progress(
			ScanProgress{
				Stage:   "dns",
				Done:    i,
				Total:   len(cands),
				Message: c.Name + " test ediliyor",
			},
		)
		report.DNS = append(report.DNS, testResolver(ctx, c, opts.Targets))
	}
	var goodDNS []Profile
	sorted := append([]DNSResult(nil), report.DNS...)
	sort.SliceStable(
		sorted,
		func(i, j int) bool { return sorted[i].AvgMillis < sorted[j].AvgMillis },
	)
	for _, d := range sorted {
		// The system resolver is the one most likely to be manipulated;
		// only fall back to it when nothing else works.
		if d.OK && d.Profile.DNSMode != "system" {
			goodDNS = append(goodDNS, d.Profile)
		}
	}
	if len(goodDNS) == 0 {
		for _, d := range sorted {
			if d.OK {
				goodDNS = append(goodDNS, d.Profile)
			}
		}
	}
	if len(goodDNS) == 0 {
		return nil, errors.New("hiçbir DNS sunucusu yanıt vermedi")
	}

	root := os.Geteuid() == 0
	var pcap *pcapIO
	if root {
		if p, err := newPcapIO(ctx); err == nil {
			pcap = p
		} else {
			root = false
		}
	}

	// Stage 3: every strategy with the fastest working resolver.
	var combos []Profile
	for _, s := range strategies(root) {
		combos = append(combos, comboProfile(goodDNS[0], s))
	}
	results := runCombos(ctx, "strategy", combos, opts, pcap, progress)

	// Stage 4: the best strategies with the other resolvers.
	if opts.Thorough && len(goodDNS) > 1 {
		rankResults(results)
		var more []Profile
		for _, r := range results[:min(3, len(results))] {
			if r.Passed == 0 {
				continue
			}
			for _, d := range goodDNS[1:min(4, len(goodDNS))] {
				p := r.Profile
				p.DNSMode, p.DNSAddr, p.DoHURL = d.DNSMode, d.DNSAddr, d.DoHURL
				p.Name = comboName(p)
				more = append(more, p)
			}
		}
		results = append(results, runCombos(ctx, "dns-mix", more, opts, pcap, progress)...)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	rankResults(results)
	report.Results = results
	if len(results) > 0 && results[0].Passed > 0 {
		best := results[0].Profile
		best.ID = "auto"
		best.Name = "Otomatik (bu ağ için)"
		best.Region = "Otomatik"
		best.Description = fmt.Sprintf(
			"Otomatik Bypass %s tarihinde %d/%d siteyi açan en hızlı ayarı seçti.",
			time.Now().Format("02.01.2006 15:04"),
			results[0].Passed,
			results[0].Total,
		)
		best.Verified = true
		report.Best = &best
	}
	report.Duration = time.Since(start)
	progress(ScanProgress{Stage: "done", Done: 1, Total: 1, Message: "Tarama tamamlandı"})
	return report, nil
}

func comboProfile(d Profile, s strategy) Profile {
	p := Profile{
		DNSMode: d.DNSMode, DNSAddr: d.DNSAddr, DoHURL: d.DoHURL,
		SplitMode: s.split, ChunkSize: s.chunk, Disorder: s.disorder, FakeCount: s.fake,
	}
	if p.ChunkSize == 0 {
		p.ChunkSize = 5
	}
	p.Name = comboName(p)
	return p
}

func comboName(p Profile) string { return p.Label() }

// rankResults orders by sites unblocked, then speed. The strategy list is
// ordered from gentle to aggressive, so a stable sort prefers gentle ones.
func rankResults(r []ComboResult) {
	sort.SliceStable(r, func(i, j int) bool {
		if r[i].Passed != r[j].Passed {
			return r[i].Passed > r[j].Passed
		}
		// Treat latencies within 15% as equal so we do not chase noise.
		a, b := r[i].AvgMillis, r[j].AvgMillis
		return float64(a) < float64(b)*0.85
	})
}

func runCombos(
	ctx context.Context,
	stage string,
	combos []Profile,
	opts ScanOptions,
	pcap *pcapIO,
	progress func(ScanProgress),
) []ComboResult {
	results := make([]ComboResult, len(combos))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for i, p := range combos {
		wg.Add(1)
		go func(i int, p Profile) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			res := testCombo(ctx, p, opts, pcap)
			mu.Lock()
			results[i] = res
			done++
			n := done
			mu.Unlock()
			progress(
				ScanProgress{
					Stage:   stage,
					Done:    n,
					Total:   len(combos),
					Message: p.Name,
					Combo:   &res,
				},
			)
		}(i, p)
	}
	wg.Wait()

	out := results[:0]
	for _, r := range results {
		if r.Total > 0 {
			out = append(out, r)
		}
	}
	return out
}

func testCombo(
	ctx context.Context,
	p Profile,
	opts ScanOptions,
	pcap *pcapIO,
) ComboResult {
	res := ComboResult{Profile: p, Total: len(opts.Targets)}

	addr, stop, err := startTempProxy(ctx, p, pcap)
	if err != nil {
		for _, t := range opts.Targets {
			res.Targets = append(res.Targets, TargetResult{Target: t, Error: err.Error()})
		}
		return res
	}
	defer stop()

	proxyURL := &url.URL{Scheme: "http", Host: addr}
	var sum time.Duration
	for _, t := range opts.Targets {
		r := fetch(ctx, proxyURL, t, opts.Timeout)
		if !r.OK && ctx.Err() == nil {
			r = fetch(ctx, proxyURL, t, opts.Timeout) // one retry against flakiness
		}
		if r.OK {
			res.Passed++
			sum += r.Latency
		}
		res.Targets = append(res.Targets, r)
	}
	if res.Passed > 0 {
		res.AvgMillis = (sum / time.Duration(res.Passed)).Milliseconds()
	}
	return res
}

// fetch performs a real HTTPS request with certificate verification, so a
// block page or a reset counts as failure.
func fetch(
	ctx context.Context,
	proxy *url.URL,
	host string,
	timeout time.Duration,
) TargetResult {
	tr := &nethttp.Transport{
		Proxy:               nil,
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   true,
	}
	if proxy != nil {
		tr.Proxy = nethttp.ProxyURL(proxy)
	}
	defer tr.CloseIdleConnections()

	client := &nethttp.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(*nethttp.Request, []*nethttp.Request) error {
			return nethttp.ErrUseLastResponse
		},
	}

	req, err := nethttp.NewRequestWithContext(
		ctx,
		nethttp.MethodGet,
		"https://"+host+"/",
		nil,
	)
	if err != nil {
		return TargetResult{Target: host, Error: err.Error()}
	}
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
	)

	t := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return TargetResult{Target: host, Error: shortErr(err)}
	}
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	_ = resp.Body.Close()
	return TargetResult{Target: host, OK: true, Latency: time.Since(t)}
}

func shortErr(err error) string {
	var ne net.Error
	switch {
	case errors.As(err, &ne) && ne.Timeout():
		return "zaman aşımı"
	case errors.Is(err, io.EOF):
		return "bağlantı kesildi"
	}
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

func testResolver(ctx context.Context, p Profile, targets []string) DNSResult {
	res := DNSResult{Name: p.Name, Profile: p}
	rc := config.DefaultConfig().RuntimeConfig
	if err := ApplyProfile(&rc, p); err != nil {
		res.Error = err.Error()
		return res
	}
	rc.DNS.Cache = false
	client := dns.NewClient(zerolog.Nop(), &rc)

	var sum time.Duration
	for _, t := range targets {
		c, cancel := context.WithTimeout(ctx, 4*time.Second)
		start := time.Now()
		_, err := client.Resolve(c, nil, t)
		cancel()
		if err != nil {
			res.Error = shortErr(err)
			return res
		}
		sum += time.Since(start)
	}
	res.OK = true
	res.AvgMillis = (sum / time.Duration(max(1, len(targets)))).Milliseconds()
	return res
}

// startTempProxy runs an HTTP proxy for one profile on a free local port.
// It never touches system proxy settings.
func startTempProxy(
	ctx context.Context,
	p Profile,
	pcap *pcapIO,
) (string, func(), error) {
	rc := config.DefaultConfig().RuntimeConfig
	if err := ApplyProfile(&rc, p); err != nil {
		return "", nil, err
	}
	rc.Conn.TCPTimeout = 8 * time.Second

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	addr := *l.Addr().(*net.TCPAddr)
	_ = l.Close()

	logger := zerolog.Nop()
	var writer packet.Writer
	var sniffer packet.Sniffer
	if pcap != nil && p.FakeCount > 0 {
		writer, sniffer = pcap.writer, pcap.sniffer
	}

	pctx, cancel := context.WithCancel(ctx)
	srv := proxyhttp.NewHTTPProxy(
		logger,
		proxyhttp.NewHTTPHandler(logger),
		proxyhttp.NewHTTPSHandler(
			logger,
			desync.NewTLSDesyncer(logger, writer, sniffer),
			&rc,
		),
		dns.NewClient(logger, &rc),
		rule.NewRuleSet(),
		addr,
		&rc,
	)
	if err := srv.ListenAndServe(pctx); err != nil {
		cancel()
		return "", nil, err
	}
	return addr.String(), cancel, nil
}

type pcapIO struct {
	writer  packet.Writer
	sniffer packet.Sniffer
}

func newPcapIO(ctx context.Context) (*pcapIO, error) {
	cfg := config.DefaultConfig()
	cfg.HTTPS.FakeCount = 1
	route, err := netutil.DiscoverRoute(nil)
	if err != nil {
		return nil, err
	}
	sniffer, _, writer, _, err := setupPcapIO(ctx, zerolog.Nop(), route, cfg)
	if err != nil {
		return nil, err
	}
	return &pcapIO{writer: writer, sniffer: sniffer}, nil
}
