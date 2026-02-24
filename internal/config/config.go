package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"fivi/internal/profile"
	"fivi/internal/sources"
)

type AppConfig struct {
	Profile          string
	Engine           string
	XRAYBin          string
	SingboxBin       string
	ListenSocksAddr  string
	ListenHTTPAddr   string
	NoTUI            bool
	Workers          int
	MaxCandidates    int
	TopCount         int
	CandidateTimeout time.Duration
	StartupTimeout   time.Duration
	ProbeTimeout     time.Duration
	ProbeMinSuccess  int
	RequiredTargets  []string
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
	defaultProfile := profile.MustResolve("iran")

	var (
		profileName      = flag.String("profile", "iran", "operational profile name")
		engine           = flag.String("engine", "xray", "engine backend: xray|singbox")
		xrayBin          = flag.String("xray-bin", defaultXrayBin(), "path to xray binary (or set FIVI_XRAY_BIN)")
		singboxBin       = flag.String("singbox-bin", defaultSingboxBin(), "path to sing-box binary (or set FIVI_SINGBOX_BIN)")
		socksAddr        = flag.String("socks-addr", "127.0.0.1:10808", "SOCKS listen address for active connection")
		httpAddr         = flag.String("http-addr", "127.0.0.1:10809", "HTTP listen address for active connection")
		noTUI            = flag.Bool("no-tui", false, "disable TUI mode even in interactive terminals")
		workers          = flag.Int("workers", max(6, runtime.NumCPU()*2), "parallel workers for candidate testing")
		maxCandidates    = flag.Int("max-candidates", 250, "max unique candidates to test per round")
		topCount         = flag.Int("top", 5, "number of top ranked candidates to print")
		candidateTimeout = flag.Duration("candidate-timeout", 24*time.Second, "timeout budget per candidate across all attempts")
		startupTimeout   = flag.Duration("startup-timeout", 4*time.Second, "max wait for local proxy listeners to become ready after engine start")
		probeTimeout     = flag.Duration("probe-timeout", defaultProfile.ProbeTimeout, "timeout per probe request")
		minProbeSuccess  = flag.Int("min-success", defaultProfile.ProbeMinOK, "minimum successful target probes required")
		monitorInterval  = flag.Duration("monitor-interval", defaultProfile.MonitorPeriod, "interval between health checks")
		monitorFailLimit = flag.Int("monitor-fail-limit", 2, "consecutive monitor failures before failover")
		sourcesCSV       = flag.String("sources", strings.Join(DefaultSources(), ","), "comma-separated subscription URLs")
		targetsCSV       = flag.String("targets", strings.Join(defaultProfile.Targets, ","), "comma-separated probe targets")
		speedTarget      = flag.String("speed-target", "https://www.youtube.com/", "target URL for speed sampling through proxy")
		speedBytes       = flag.Int64("speed-bytes", 262144, "max bytes to read for speed sampling")
		retryBackoff     = flag.Duration("retry-backoff", defaultProfile.RetryBackoff, "backoff before restarting discovery when all candidates fail")
		schemesCSV       = flag.String("schemes", "vmess,vless,trojan,ss", "allowed URI schemes")
	)
	flag.Parse()
	selectedProfile, err := profile.Resolve(*profileName)
	if err != nil {
		failFlag("%v", err)
	}

	visited := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	if !visited["targets"] {
		*targetsCSV = strings.Join(selectedProfile.Targets, ",")
	}
	if !visited["probe-timeout"] {
		*probeTimeout = selectedProfile.ProbeTimeout
	}
	if !visited["min-success"] {
		*minProbeSuccess = selectedProfile.ProbeMinOK
	}
	if !visited["retry-backoff"] {
		*retryBackoff = selectedProfile.RetryBackoff
	}
	if !visited["monitor-interval"] {
		*monitorInterval = selectedProfile.MonitorPeriod
	}

	engineName := normalizeEngine(*engine)
	switch engineName {
	case "xray":
		if strings.TrimSpace(*xrayBin) == "" {
			failFlag("--xray-bin cannot be empty when --engine=xray")
		}
	case "singbox":
		if strings.TrimSpace(*singboxBin) == "" {
			failFlag("--singbox-bin cannot be empty when --engine=singbox")
		}
	default:
		failFlag("invalid --engine=%q (supported: xray, singbox)", *engine)
	}

	allowed := map[string]bool{}
	for _, s := range splitCSV(*schemesCSV) {
		allowed[strings.ToLower(s)] = true
	}

	return AppConfig{
		Profile:          selectedProfile.Name,
		Engine:           engineName,
		XRAYBin:          *xrayBin,
		SingboxBin:       *singboxBin,
		ListenSocksAddr:  *socksAddr,
		ListenHTTPAddr:   *httpAddr,
		NoTUI:            *noTUI,
		Workers:          *workers,
		MaxCandidates:    *maxCandidates,
		TopCount:         *topCount,
		CandidateTimeout: *candidateTimeout,
		StartupTimeout:   *startupTimeout,
		ProbeTimeout:     *probeTimeout,
		ProbeMinSuccess:  *minProbeSuccess,
		RequiredTargets:  append([]string(nil), selectedProfile.RequiredTargetTokens...),
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

func defaultXrayBin() string {
	if v, ok := os.LookupEnv("FIVI_XRAY_BIN"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "xray"
}

func defaultSingboxBin() string {
	if v, ok := os.LookupEnv("FIVI_SINGBOX_BIN"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "sing-box"
}

func DefaultTargets() []string {
	return profile.MustResolve("iran").Targets
}

func DefaultSources() []string {
	return sources.DefaultURLs()
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

func normalizeEngine(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "sing-box" {
		return "singbox"
	}
	return v
}

func failFlag(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "flag error: "+format+"\n", args...)
	os.Exit(2)
}
