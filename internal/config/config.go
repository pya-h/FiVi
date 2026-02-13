package config

import (
	"flag"
	"runtime"
	"strings"
	"time"
)

type AppConfig struct {
	XRAYBin          string
	ListenSocksAddr  string
	ListenHTTPAddr   string
	Workers          int
	MaxCandidates    int
	TopCount         int
	ProbeTimeout     time.Duration
	ProbeMinSuccess  int
	MonitorInterval  time.Duration
	MonitorFailLimit int
	BootstrapSources []string
	Targets          []string
	SpeedTarget      string
	SpeedTestBytes   int64
	RetryBackoff     time.Duration
	OnlySchemes      map[string]bool
}

func ParseFlags() AppConfig {
	var (
		xrayBin          = flag.String("xray-bin", "xray", "path to xray binary")
		socksAddr        = flag.String("socks-addr", "127.0.0.1:10808", "SOCKS listen address for active connection")
		httpAddr         = flag.String("http-addr", "127.0.0.1:10809", "HTTP listen address for active connection")
		workers          = flag.Int("workers", max(6, runtime.NumCPU()*2), "parallel workers for candidate testing")
		maxCandidates    = flag.Int("max-candidates", 250, "max unique candidates to test per round")
		topCount         = flag.Int("top", 5, "number of top ranked candidates to print")
		probeTimeout     = flag.Duration("probe-timeout", 7*time.Second, "timeout per probe request")
		minProbeSuccess  = flag.Int("min-success", 2, "minimum successful target probes required")
		monitorInterval  = flag.Duration("monitor-interval", 45*time.Second, "interval between health checks")
		monitorFailLimit = flag.Int("monitor-fail-limit", 2, "consecutive monitor failures before failover")
		sourcesCSV       = flag.String("sources", strings.Join(DefaultSources(), ","), "comma-separated subscription URLs")
		targetsCSV       = flag.String("targets", strings.Join(DefaultTargets(), ","), "comma-separated probe targets")
		speedTarget      = flag.String("speed-target", "https://www.youtube.com/", "target URL for speed sampling through proxy")
		speedBytes       = flag.Int64("speed-bytes", 262144, "max bytes to read for speed sampling")
		retryBackoff     = flag.Duration("retry-backoff", 12*time.Second, "backoff before restarting discovery when all candidates fail")
		schemesCSV       = flag.String("schemes", "vmess,vless,trojan,ss", "allowed URI schemes")
	)
	flag.Parse()

	allowed := map[string]bool{}
	for _, s := range splitCSV(*schemesCSV) {
		allowed[strings.ToLower(s)] = true
	}

	return AppConfig{
		XRAYBin:          *xrayBin,
		ListenSocksAddr:  *socksAddr,
		ListenHTTPAddr:   *httpAddr,
		Workers:          *workers,
		MaxCandidates:    *maxCandidates,
		TopCount:         *topCount,
		ProbeTimeout:     *probeTimeout,
		ProbeMinSuccess:  *minProbeSuccess,
		MonitorInterval:  *monitorInterval,
		MonitorFailLimit: *monitorFailLimit,
		BootstrapSources: splitCSV(*sourcesCSV),
		Targets:          splitCSV(*targetsCSV),
		SpeedTarget:      strings.TrimSpace(*speedTarget),
		SpeedTestBytes:   *speedBytes,
		RetryBackoff:     *retryBackoff,
		OnlySchemes:      allowed,
	}
}

func DefaultTargets() []string {
	return []string{
		"https://x.com/",
		"https://www.youtube.com/",
		"https://www.cloudflare.com/cdn-cgi/trace",
	}
}

func DefaultSources() []string {
	// Keep this list short and user-overridable because free lists change often.
	return []string{
		"https://raw.githubusercontent.com/mahdibland/V2RayAggregator/master/sub/sub_merge_base64.txt",
		"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/vless",
		"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/vmess",
		"https://raw.githubusercontent.com/soroushmirzaei/telegram-configs-collector/main/protocols/trojan",
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
