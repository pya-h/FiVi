package config

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"fivi/internal/binresolver"
	"fivi/internal/dns"
	"fivi/internal/profile"
	"fivi/internal/routing"
	"fivi/internal/sources"
)

type AppConfig struct {
	Profile                  string
	Engine                   string
	Mode                     string // "proxy" or "tunnel"
	XRAYBin                  string
	SingboxBin               string
	ListenSocksAddr          string
	ListenHTTPAddr           string
	ShareMode                bool
	NoTUI                    bool
	Workers                  int
	MaxCandidates            int
	TopCount                 int
	CandidateTimeout         time.Duration
	StartupTimeout           time.Duration
	FetchTimeout             time.Duration
	ProbeTimeout             time.Duration
	ProbeMinSuccess          int
	RequiredTargets          []string
	MonitorInterval          time.Duration
	MonitorFailLimit         int
	BootstrapSources         []string
	DirectConfigs            []string
	Targets                  []string
	SpeedTarget              string
	SpeedTestBytes           int64
	RetryBackoff             time.Duration
	LatencyCutoff            time.Duration
	ProbableLatencyThreshold time.Duration
	VerifyRetries            int
	StandbyTTL               time.Duration
	AutoReconnect            bool
	RoutingRules             []routing.Rule
	DNS                      dns.Config
	GeoBypass                string               // country code for geo-based routing (e.g. "ir")
	GeoRuleSets              *routing.GeoRuleSets // resolved at startup by engine
	NetStats                 bool                 // enable real-time network statistics
	NetStatsIface            string               // override interface for netstats (auto-detect if empty)
	CheckUpdate              bool                 // check installed engine binaries against latest releases
	OnlySchemes              map[string]bool
}

// IsTunnelMode returns true when the user selected tunnel (TUN) connection mode.
func (c AppConfig) IsTunnelMode() bool {
	return strings.EqualFold(c.Mode, "tunnel")
}

func ParseFlags() AppConfig {
	defaultProfile := profile.MustResolve("iran")

	var (
		profileName       = flag.String("profile", "iran", "operational profile name")
		engine            = flag.String("engine", "xray", "engine backend: xray|singbox")
		mode              = flag.String("mode", "proxy", "connection mode: proxy|tunnel (tunnel uses TUN interface, may require root/capabilities)")
		xrayBin           = flag.String("xray-bin", "", "override path to xray binary (auto-resolved from bins/ by default)")
		singboxBin        = flag.String("singbox-bin", "", "override path to sing-box binary (auto-resolved from bins/ by default)")
		socksAddr         = flag.String("socks-addr", "127.0.0.1:10808", "SOCKS listen address for active connection")
		httpAddr          = flag.String("http-addr", "127.0.0.1:10809", "HTTP listen address for active connection")
		shareMode         = flag.Bool("share", false, "bind proxy listeners to 0.0.0.0 for LAN sharing (proxy mode)")
		noTUI             = flag.Bool("no-tui", false, "disable TUI mode even in interactive terminals")
		workers           = flag.Int("workers", defaultWorkers(), "parallel workers for candidate testing")
		maxCandidates     = flag.Int("max-candidates", 0, "max unique candidates to test per round (0 = no limit)")
		topCount          = flag.Int("top", 5, "number of top ranked candidates to print")
		candidateTimeout  = flag.Duration("candidate-timeout", 24*time.Second, "timeout budget per candidate across all attempts")
		startupTimeout    = flag.Duration("startup-timeout", 8*time.Second, "max wait for local proxy listeners to become ready after engine start")
		fetchTimeout      = flag.Duration("fetch-timeout", 25*time.Second, "timeout per source URL fetch")
		probeTimeout      = flag.Duration("probe-timeout", defaultProfile.ProbeTimeout, "timeout per probe request")
		minProbeSuccess   = flag.Int("min-success", defaultProfile.ProbeMinOK, "minimum successful target probes required")
		monitorInterval   = flag.Duration("monitor-interval", defaultProfile.MonitorPeriod, "interval between health checks")
		monitorFailLimit  = flag.Int("monitor-fail-limit", 2, "consecutive monitor failures before failover")
		sourcesCSV        = flag.String("sources", strings.Join(DefaultSources(), ","), "comma-separated subscription URLs")
		directConfigsCSV  = flag.String("config", "", "comma-separated single proxy configs/URIs to include alongside subscriptions")
		directConfigsFile = flag.String("config-file", "", "path to file containing single proxy configs/URIs (one per line)")
		targetsCSV        = flag.String("targets", strings.Join(defaultProfile.Targets, ","), "comma-separated probe targets")
		speedTarget       = flag.String("speed-target", "https://www.youtube.com/", "target URL for speed sampling through proxy")
		speedBytes        = flag.Int64("speed-bytes", 524288, "max bytes to read for speed sampling")
		retryBackoff      = flag.Duration("retry-backoff", defaultProfile.RetryBackoff, "backoff before restarting discovery when all candidates fail")
		// latencyCutoff    = flag.Duration("latency-cutoff", 1500*time.Millisecond, "max TCP connect latency for pre-scan; 0 disables pre-scan")
		latencyCutoff            = flag.Duration("latency-cutoff", 0, "max TCP connect latency for pre-scan; 0 disables pre-scan")
		probableLatencyThreshold = flag.Duration("probable-latency-threshold", 450*time.Millisecond, "max latency for promoting probe-failed candidates to probable; 0 disables latency-based promotion")
		verifyRetries            = flag.Int("verify-retries", 5, "post-connect verification attempts before declaring connected")
		standbyTTL               = flag.Duration("standby-ttl", 10*time.Minute, "warm standby config expiry time; 0 disables warm standby")
		autoReconnect            = flag.Bool("auto-reconnect", true, "retry same config before failover on transient failures")
		routingRulesCSV          = flag.String("routing-rules", "", "comma-separated routing rules (e.g. domain:*.ir,ip:1.2.3.4,range:10.0.0.0/8)")
		routingRulesFile         = flag.String("routing-rules-file", "", "path to routing rules file (one rule per line)")
		dnsPlain                 = flag.String("dns", "", "plain DNS server for direct resolution (e.g. 223.5.5.5)")
		dnsDoH                   = flag.String("doh", "", "DNS-over-HTTPS/TLS server for proxy-routed resolution (e.g. tls://8.8.8.8, https://1.1.1.1/dns-query)")
		geoBypass                = flag.String("geo-bypass", "", "country code for geo-based routing bypass (e.g. ir, cn, ru); downloads .srs rule sets for singbox")
		netStats                 = flag.Bool("stats", false, "enable real-time network statistics in TUI (reads /proc/net/dev)")
		netStatsIface            = flag.String("stats-iface", "", "network interface for stats (auto-detected if empty)")
		checkUpdate              = flag.Bool("check-update", false, "check whether engine binaries are outdated on startup")
		schemesCSV               = flag.String("schemes", "vmess,vless,trojan,ss", "allowed URI schemes")
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

	// Normalise engine name.
	engineName := normalizeEngine(*engine)

	// Normalise connection mode.
	modeName := normalizeMode(*mode)

	// Resolve binary paths: explicit flag > env var > auto-detect from bins/.
	xrayBinPath := resolveBinary("xray", *xrayBin, "FIVI_XRAY_BIN", visited["xray-bin"])
	singboxBinPath := resolveBinary("singbox", *singboxBin, "FIVI_SINGBOX_BIN", visited["singbox-bin"])

	switch engineName {
	case "xray":
		if strings.TrimSpace(xrayBinPath) == "" {
			failFlag("could not locate xray binary; install xray and use --xray-bin /path/to/xray or place it in bins/xray/%s/", runtime.GOARCH)
		}
	case "singbox":
		if strings.TrimSpace(singboxBinPath) == "" {
			failFlag("could not locate sing-box binary; install sing-box and use --singbox-bin /path/to/sing-box or place it in bins/singbox/%s/", runtime.GOARCH)
		}
	default:
		failFlag("invalid --engine=%q (supported: xray, singbox)", *engine)
	}

	// Load routing rules: file > inline CSV > profile defaults.
	var routingRules []routing.Rule
	switch {
	case *routingRulesFile != "":
		rules, err := routing.LoadFromFile(*routingRulesFile)
		if err != nil {
			failFlag("%v", err)
		}
		routingRules = rules
	case *routingRulesCSV != "":
		rules, err := routing.ParseCSV(*routingRulesCSV)
		if err != nil {
			failFlag("--routing-rules: %v", err)
		}
		routingRules = rules
	default:
		routingRules = routing.ProfileDefaults(selectedProfile.Name)
	}

	// Build DNS config: user overrides > defaults.
	dnsConfig := dns.Default()
	if *dnsPlain != "" {
		dnsConfig.Direct = strings.TrimSpace(*dnsPlain)
	}
	if *dnsDoH != "" {
		dnsConfig.Remote = strings.TrimSpace(*dnsDoH)
	}

	allowed := map[string]bool{}
	for _, s := range splitCSV(*schemesCSV) {
		allowed[strings.ToLower(s)] = true
	}

	directConfigs := splitCSV(*directConfigsCSV)
	if strings.TrimSpace(*directConfigsFile) != "" {
		fromFile, err := readConfigLines(*directConfigsFile)
		if err != nil {
			failFlag("--config-file: %v", err)
		}
		directConfigs = append(directConfigs, fromFile...)
	}
	directConfigs = uniqueStrings(directConfigs)

	if *shareMode {
		*socksAddr = sharedListenAddr(*socksAddr)
		*httpAddr = sharedListenAddr(*httpAddr)
	}

	return AppConfig{
		Profile:                  selectedProfile.Name,
		Engine:                   engineName,
		Mode:                     modeName,
		XRAYBin:                  xrayBinPath,
		SingboxBin:               singboxBinPath,
		ListenSocksAddr:          *socksAddr,
		ListenHTTPAddr:           *httpAddr,
		ShareMode:                *shareMode,
		NoTUI:                    *noTUI,
		Workers:                  *workers,
		MaxCandidates:            *maxCandidates,
		TopCount:                 *topCount,
		CandidateTimeout:         *candidateTimeout,
		StartupTimeout:           *startupTimeout,
		FetchTimeout:             *fetchTimeout,
		ProbeTimeout:             *probeTimeout,
		ProbeMinSuccess:          *minProbeSuccess,
		RequiredTargets:          append([]string(nil), selectedProfile.RequiredTargetTokens...),
		MonitorInterval:          *monitorInterval,
		MonitorFailLimit:         *monitorFailLimit,
		BootstrapSources:         splitCSV(*sourcesCSV),
		DirectConfigs:            directConfigs,
		Targets:                  splitCSV(*targetsCSV),
		SpeedTarget:              strings.TrimSpace(*speedTarget),
		SpeedTestBytes:           *speedBytes,
		RetryBackoff:             *retryBackoff,
		LatencyCutoff:            *latencyCutoff,
		ProbableLatencyThreshold: *probableLatencyThreshold,
		VerifyRetries:            *verifyRetries,
		StandbyTTL:               *standbyTTL,
		AutoReconnect:            *autoReconnect,
		RoutingRules:             routingRules,
		DNS:                      dnsConfig,
		GeoBypass:                strings.TrimSpace(strings.ToLower(*geoBypass)),
		NetStats:                 *netStats,
		NetStatsIface:            strings.TrimSpace(*netStatsIface),
		CheckUpdate:              *checkUpdate,
		OnlySchemes:              allowed,
	}
}

func defaultWorkers() int {
	n := runtime.NumCPU() * 3
	if n < 10 {
		n = 10
	}
	return n
}

// resolveBinary determines the binary path for an engine.
// Priority: explicit flag > environment variable > auto-detect from bins/.
func resolveBinary(engine, flagValue, envKey string, flagExplicit bool) string {
	// 1. Explicit flag value.
	if flagExplicit && strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	// 2. Environment variable.
	if v, ok := os.LookupEnv(envKey); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	// 3. Auto-detect from bins/ directory.
	if resolved, err := binresolver.Resolve(engine); err == nil {
		return resolved
	}
	// 4. Empty — caller validates.
	return ""
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

func readConfigLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := make([]string, 0, 64)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		res = append(res, line)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func sharedListenAddr(addr string) string {
	_, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return addr
	}
	return net.JoinHostPort("0.0.0.0", port)
}

func normalizeEngine(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "sing-box" {
		return "singbox"
	}
	return v
}

func normalizeMode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "proxy", "tunnel":
		return v
	default:
		return "proxy"
	}
}

func failFlag(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "flag error: "+format+"\n", args...)
	os.Exit(2)
}
