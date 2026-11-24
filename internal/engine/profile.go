package engine

import (
	"fmt"
	"net"
	"os"
	"runtime"

	"github.com/xvzc/spoofdpi/internal/config"
)

// Profile is a complete, serialisable bypass setting: DNS + ClientHello
// fragmentation. Presets, auto-bypass results and user edits all use it.
type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Description string `json:"description"`

	DNSMode string `json:"dnsMode"` // udp | https | system
	DNSAddr string `json:"dnsAddr"` // host:port for udp
	DoHURL  string `json:"dohUrl"`  // for https

	SplitMode string `json:"splitMode"`
	ChunkSize int    `json:"chunkSize"`
	Disorder  bool   `json:"disorder"`
	FakeCount int    `json:"fakeCount"` // >0 needs root + pcap

	// Verified marks presets that were tested on a real network in that
	// region. Others are educated starting points; run auto-bypass.
	Verified bool `json:"verified"`
}

// NeedsRoot reports whether the profile uses raw packet injection.
func (p Profile) NeedsRoot() bool { return p.FakeCount > 0 }

// Label is a short technical summary, e.g. "chunk/5 · UDP 77.88.8.8:1253".
func (p Profile) Label() string {
	s := p.SplitMode
	if p.SplitMode == "chunk" {
		s = fmt.Sprintf("chunk/%d", p.ChunkSize)
	}
	if p.Disorder {
		s += "+disorder"
	}
	if p.FakeCount > 0 {
		s += fmt.Sprintf("+fake×%d", p.FakeCount)
	}
	return s + " · " + p.dnsLabel()
}

func (p Profile) dnsLabel() string {
	switch p.DNSMode {
	case "https":
		return "DoH " + p.DoHURL
	case "system":
		return "Sistem DNS"
	default:
		return "UDP " + p.DNSAddr
	}
}

// Options are app-level settings independent of the bypass profile.
type Options struct {
	ListenAddr  string `json:"listenAddr"`  // default 127.0.0.1:8080
	AppMode     string `json:"appMode"`     // http | socks5
	SystemProxy bool   `json:"systemProxy"` // macOS only
	IPv6        bool   `json:"ipv6"`
	DNSCache    bool   `json:"dnsCache"`
}

func DefaultOptions() Options {
	return Options{
		ListenAddr:  "127.0.0.1:8080",
		AppMode:     "http",
		SystemProxy: runtime.GOOS == "darwin" || runtime.GOOS == "windows",
		DNSCache:    true,
	}
}

// BuildConfig turns a profile + options into a finalized, validated config.
func BuildConfig(p Profile, o Options) (*config.Config, error) {
	cfg := config.DefaultConfig()

	if err := ApplyProfile(&cfg.RuntimeConfig, p); err != nil {
		return nil, err
	}

	switch o.AppMode {
	case "", "http":
		cfg.App.Mode = config.AppModeHTTP
	case "socks5":
		cfg.App.Mode = config.AppModeSOCKS5
	default:
		return nil, fmt.Errorf("desteklenmeyen mod %q", o.AppMode)
	}

	if o.ListenAddr != "" {
		addr, err := net.ResolveTCPAddr("tcp", o.ListenAddr)
		if err != nil {
			return nil, fmt.Errorf("dinleme adresi: %w", err)
		}
		cfg.App.ListenAddr = *addr
	}
	cfg.App.AutoConfigureNetwork = o.SystemProxy &&
		(runtime.GOOS == "darwin" || runtime.GOOS == "windows")
	cfg.App.NoTUI = true
	cfg.DNS.Cache = o.DNSCache
	if o.IPv6 {
		cfg.DNS.QType = config.DNSQueryAll
	}

	if p.NeedsRoot() && os.Geteuid() != 0 {
		return nil, fmt.Errorf(
			"%q profili sahte paket kullanıyor ve yönetici (root) yetkisi gerektiriyor",
			p.Name,
		)
	}

	if err := cfg.Finalize(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ApplyProfile writes the profile's DNS and HTTPS settings into rc.
func ApplyProfile(rc *config.RuntimeConfig, p Profile) error {
	mode, err := config.ParseDNSMode(orDefault(p.DNSMode, "udp"))
	if err != nil {
		return err
	}
	rc.DNS.Mode = mode
	if p.DNSAddr != "" {
		addr, err := net.ResolveTCPAddr("tcp", p.DNSAddr)
		if err != nil {
			return fmt.Errorf("dns adresi: %w", err)
		}
		rc.DNS.Addr = *addr
	}
	if p.DoHURL != "" {
		rc.DNS.HTTPSURL = p.DoHURL
	}

	split, err := config.ParseHTTPSSplitMode(orDefault(p.SplitMode, "chunk"))
	if err != nil {
		return err
	}
	rc.HTTPS.SplitMode = split
	if p.ChunkSize > 0 {
		rc.HTTPS.ChunkSize = uint8(min(p.ChunkSize, 255))
	}
	rc.HTTPS.Disorder = p.Disorder
	rc.HTTPS.FakeCount = uint8(min(max(p.FakeCount, 0), 255))
	return nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// Well-known resolvers. IP-based DoH URLs avoid a bootstrap lookup that
// could itself be poisoned.
const (
	YandexUDPAlt   = "77.88.8.8:1253"
	CloudflareUDP  = "1.1.1.1:53"
	CloudflareDoH  = "https://1.1.1.1/dns-query"
	GoogleDoH      = "https://8.8.8.8/dns-query"
	Quad9DoH       = "https://9.9.9.9/dns-query"
	AdGuardDoH     = "https://dns.adguard-dns.com/dns-query"
	regionTR       = "Türkiye"
	regionGlobal   = "Genel"
	regionEurasia  = "Avrasya"
	regionMidEast  = "Orta Doğu"
	regionAsia     = "Asya"
	unverifiedNote = " Bu bölgede doğrulanmadı: Otomatik Bypass ile kendi ağınızda test edin."
)

// Presets returns the built-in profiles. The first one is the default.
func Presets() []Profile {
	return []Profile{
		{
			ID:          "tr",
			Name:        "Türkiye (Önerilen)",
			Region:      regionTR,
			Description: "ClientHello'yu SNI'den TLS kayıtlarına böler + Yandex DNS alternatif portu. Canlı testte engelli sitelerin hepsini açtı.",
			DNSMode:     "udp",
			DNSAddr:     YandexUDPAlt,
			SplitMode:   "tlsrec",
			ChunkSize:   5,
			Verified:    true,
		},
		{
			ID:          "tr-chunk",
			Name:        "Türkiye – Klasik Parçalama",
			Region:      regionTR,
			Description: "Eski SpoofDPI-Türkiye ayarı: 5 baytlık TCP parçaları. Bazı operatörlerde bazı siteleri açmayabilir.",
			DNSMode:     "udp",
			DNSAddr:     YandexUDPAlt,
			SplitMode:   "chunk",
			ChunkSize:   5,
			Verified:    true,
		},
		{
			ID:          "tr-doh",
			Name:        "Türkiye – DoH (Cloudflare)",
			Region:      regionTR,
			Description: "DNS sorgularını şifreli HTTPS üzerinden yapar; DNS manipülasyonuna karşı en güvenli seçenek.",
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "chunk",
			ChunkSize:   5,
		},
		{
			ID:          "tr-aggressive",
			Name:        "Türkiye – Agresif",
			Region:      regionTR,
			Description: "Bayt bayt parçalama + sıra karıştırma. Diğerleri çalışmazsa deneyin; biraz daha yavaştır.",
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "chunk",
			ChunkSize:   1,
			Disorder:    true,
		},
		{
			ID: "ru", Name: "Rusya (TSPU)", Region: regionEurasia,
			Description: "TLS kayıt bölme + sıra karıştırma, şifreli DNS." + unverifiedNote,
			DNSMode:     "https", DoHURL: CloudflareDoH, SplitMode: "tlsrec", Disorder: true,
		},
		{
			ID:          "ru-fake",
			Name:        "Rusya – Sahte Paket (root)",
			Region:      regionEurasia,
			Description: "Düşük TTL'li sahte ClientHello paketleri gönderir. Yönetici yetkisi ister." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "sni",
			Disorder:    true,
			FakeCount:   3,
		},
		{
			ID:          "kz",
			Name:        "Kazakistan",
			Region:      regionEurasia,
			Description: "İlk bayt bölme + sıra karıştırma, şifreli DNS." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "first-byte",
			Disorder:    true,
		},
		{
			ID:          "ir",
			Name:        "İran",
			Region:      regionMidEast,
			Description: "Bayt bayt parçalama + sıra karıştırma. IP engellemesi olan sitelerde DPI atlatma tek başına yetmez." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      Quad9DoH,
			SplitMode:   "chunk",
			ChunkSize:   1,
			Disorder:    true,
		},
		{
			ID:          "mideast",
			Name:        "Körfez, Mısır, Pakistan",
			Region:      regionMidEast,
			Description: "SNI bölme + şifreli DNS; SNI tabanlı filtrelere karşı başlangıç noktası." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "sni",
		},
		{
			ID:          "in-id",
			Name:        "Hindistan, Endonezya",
			Region:      regionAsia,
			Description: "Bu ülkelerde engellerin çoğu DNS düzeyinde; şifreli DNS + hafif bölme genelde yeterli." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      GoogleDoH,
			SplitMode:   "sni",
		},
		{
			ID:          "cn",
			Name:        "Çin (GFW) – sınırlı",
			Region:      regionAsia,
			Description: "GFW IP engelleme ve aktif inceleme yapar; parçalama çoğu sitede yetmez. Yalnızca SNI filtresine takılan siteler için." + unverifiedNote,
			DNSMode:     "https",
			DoHURL:      Quad9DoH,
			SplitMode:   "tlsrec",
			Disorder:    true,
		},
		{
			ID:          "dns-only",
			Name:        "Yalnızca DNS Engeli",
			Region:      regionGlobal,
			Description: "Paket bölmeden yalnızca şifreli DNS kullanır. En hızlısı; yalnızca DNS engellerini aşar.",
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "none",
		},
		{
			ID:          "max",
			Name:        "Maksimum Uyumluluk",
			Region:      regionGlobal,
			Description: "Tek bayt parçalama + sıra karıştırma + şifreli DNS. Hız yerine başarı önceliklidir.",
			DNSMode:     "https",
			DoHURL:      CloudflareDoH,
			SplitMode:   "chunk",
			ChunkSize:   1,
			Disorder:    true,
		},
	}
}

// PresetByID finds a built-in profile.
func PresetByID(id string) (Profile, bool) {
	for _, p := range Presets() {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}
