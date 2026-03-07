package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/engines"
	"fivi/internal/parser"
	"fivi/internal/probe"
	"fivi/internal/sources"
)

type captureAdapter struct {
	lastOpts engines.ConfigBuildOpts
}

func (a *captureAdapter) Name() string                   { return "capture" }
func (a *captureAdapter) EnsureBinary(path string) error { return nil }
func (a *captureAdapter) BuildConfig(opts engines.ConfigBuildOpts) ([]byte, engines.Parsed, error) {
	a.lastOpts = opts
	return []byte(`{}`), engines.Parsed{Name: "node", Protocol: "vmess"}, nil
}
func (a *captureAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (engines.Instance, error) {
	return captureInstance{}, nil
}

type captureInstance struct{}

func (captureInstance) PID() int { return 0 }
func (captureInstance) Stop()    {}

func TestTestOne_UsesTunnelModeWhenConfigured(t *testing.T) {
	origCheck := checkThroughSOCKSFn
	origWait := waitForProxyReadyFn
	origReserve := reserveLocalAddrsFn
	t.Cleanup(func() {
		checkThroughSOCKSFn = origCheck
		waitForProxyReadyFn = origWait
		reserveLocalAddrsFn = origReserve
	})

	checkThroughSOCKSFn = func(ctx context.Context, socksAddr string, targets []string, timeout time.Duration) probe.Result {
		return probe.Result{
			Successes:   1,
			TargetCount: 1,
			AvgLatency:  10 * time.Millisecond,
			Targets: []probe.TargetResult{
				{URL: "https://x.com/", OK: true},
			},
		}
	}
	waitForProxyReadyFn = func(ctx context.Context, socksAddr, httpAddr string, timeout time.Duration) error {
		return nil
	}
	reserveLocalAddrsFn = func() (socksAddr, httpAddr string, err error) {
		return "127.0.0.1:10080", "127.0.0.1:10081", nil
	}

	adapter := &captureAdapter{}
	cfg := config.AppConfig{
		Mode:             "tunnel",
		Targets:          []string{"https://x.com/"},
		ProbeTimeout:     200 * time.Millisecond,
		ProbeMinSuccess:  1,
		RequiredTargets:  nil,
		CandidateTimeout: 2 * time.Second,
		StartupTimeout:   100 * time.Millisecond,
	}

	outcome := testOne(context.Background(), cfg, parser.Candidate{Raw: "vmess://example", Scheme: "vmess"}, 1, nil, adapter, "mockbin")
	if outcome.Ranked == nil {
		t.Fatalf("expected ranked outcome, got=%+v", outcome)
	}
	if !adapter.lastOpts.TunnelMode {
		t.Fatalf("expected BuildConfig TunnelMode=true in tunnel mode")
	}
}

func TestRankedFromProbableSortsBySuccessThenLatency(t *testing.T) {
	probable := []ProbableCandidate{
		{Raw: "a", Scheme: "vmess", Successes: 0, AvgLatency: 20 * time.Millisecond},
		{Raw: "b", Scheme: "vmess", Successes: 1, AvgLatency: 100 * time.Millisecond},
		{Raw: "c", Scheme: "vmess", Successes: 1, AvgLatency: 50 * time.Millisecond},
	}
	ranked := rankedFromProbable(probable)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked, got %d", len(ranked))
	}
	if ranked[0].Raw != "c" || ranked[1].Raw != "b" || ranked[2].Raw != "a" {
		t.Fatalf("unexpected order: %s, %s, %s", ranked[0].Raw, ranked[1].Raw, ranked[2].Raw)
	}
}

func TestShouldPromoteLatencyProbable(t *testing.T) {
	cfg := config.AppConfig{ProbableLatencyThreshold: 450 * time.Millisecond}
	if !shouldPromoteLatencyProbable(cfg, 120*time.Millisecond) {
		t.Fatalf("expected latency below threshold to be promotable")
	}
	if shouldPromoteLatencyProbable(cfg, 700*time.Millisecond) {
		t.Fatalf("expected latency above threshold to be non-promotable")
	}
	cfg.ProbableLatencyThreshold = 0
	if shouldPromoteLatencyProbable(cfg, 100*time.Millisecond) {
		t.Fatalf("expected disabled threshold to prevent promotion")
	}
}

func TestReorderProbeTargets_PinsRequiredTargets(t *testing.T) {
	current := []string{
		"https://www.cloudflare.com/cdn-cgi/trace",
		"https://x.com/",
		"https://www.youtube.com/",
		"https://example.org/health",
	}
	quality := map[string]targetQuality{
		"https://www.cloudflare.com/cdn-cgi/trace": {GoodTotal: 2, GoodPass: 2, BadTotal: 3, BadPass: 3}, // poor separator
		"https://x.com/":             {GoodTotal: 5, GoodPass: 5, BadTotal: 5, BadPass: 0}, // strong separator
		"https://www.youtube.com/":   {GoodTotal: 5, GoodPass: 4, BadTotal: 5, BadPass: 1},
		"https://example.org/health": {GoodTotal: 5, GoodPass: 5, BadTotal: 5, BadPass: 5}, // neutral
	}
	required := []string{"x.com", "youtube.com"}
	reordered := reorderProbeTargets(current, required, quality)

	if len(reordered) != len(current) {
		t.Fatalf("unexpected reordered length: %d", len(reordered))
	}
	if reordered[0] != "https://x.com/" {
		t.Fatalf("expected x.com required target first, got %s", reordered[0])
	}
	if reordered[1] != "https://www.youtube.com/" {
		t.Fatalf("expected youtube required target second, got %s", reordered[1])
	}
}

func TestTargetSeparabilityScore(t *testing.T) {
	good := targetSeparabilityScore(targetQuality{GoodTotal: 10, GoodPass: 9, BadTotal: 10, BadPass: 1})
	poor := targetSeparabilityScore(targetQuality{GoodTotal: 10, GoodPass: 6, BadTotal: 10, BadPass: 5})
	if good <= poor {
		t.Fatalf("expected stronger separator to have higher score: good=%.3f poor=%.3f", good, poor)
	}
}

func TestSummarizeFailureReasons(t *testing.T) {
	recent := []FailureReason{
		FailureReasonProbeFailed,
		"",
		FailureReasonProbePartial,
		FailureReasonProbeFailed,
		FailureReasonProbeFailed,
	}
	top := summarizeFailureReasons(recent, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top reasons, got %d", len(top))
	}
	if !strings.Contains(top[0], "probe_failed=3") {
		t.Fatalf("unexpected top[0]=%q", top[0])
	}
	if !strings.Contains(top[1], "probe_partial=1") {
		t.Fatalf("unexpected top[1]=%q", top[1])
	}
}

func TestSummarizeSourceHealthTop(t *testing.T) {
	now := time.Now()
	stats := []sources.HealthStat{
		{Source: "a", Successes: 5, Failures: 1, ConsecutiveFailures: 0, TotalYield: 80},
		{Source: "b", Successes: 0, Failures: 4, ConsecutiveFailures: 3, TotalYield: 5, CooldownUntil: now.Add(30 * time.Second)},
		{Source: "c", Successes: 1, Failures: 3, ConsecutiveFailures: 2, TotalYield: 10},
	}
	top := summarizeSourceHealthTop(stats, now, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2 source top items, got %d", len(top))
	}
	if !strings.HasPrefix(top[0], "b ") {
		t.Fatalf("expected source b as top offender, got %q", top[0])
	}
	if !strings.Contains(top[0], "yield=5") {
		t.Fatalf("expected yield in top item, got %q", top[0])
	}
}

func TestProbeJitter(t *testing.T) {
	result := probeJitter([]probe.TargetResult{
		{URL: "a", OK: true, Latency: 100 * time.Millisecond},
		{URL: "b", OK: true, Latency: 200 * time.Millisecond},
		{URL: "c", OK: true, Latency: 300 * time.Millisecond},
	})
	if result < 80*time.Millisecond || result > 82*time.Millisecond {
		t.Fatalf("unexpected jitter=%s", result)
	}
}

func TestQualityGuardrailBreaches(t *testing.T) {
	cfg := config.AppConfig{
		QualityLatencyFloor: 180 * time.Millisecond,
		QualityJitterFloor:  50 * time.Millisecond,
		QualityFloorStreak:  2,
	}
	breaches := qualityGuardrailBreaches(cfg, 220*time.Millisecond, 65*time.Millisecond)
	if len(breaches) != 2 {
		t.Fatalf("expected 2 breaches, got %d (%v)", len(breaches), breaches)
	}
	if effectiveQualityFloorStreak(cfg) != 2 {
		t.Fatalf("unexpected streak=%d", effectiveQualityFloorStreak(cfg))
	}
	cfg.QualityFloorStreak = 0
	if effectiveQualityFloorStreak(cfg) != 1 {
		t.Fatalf("expected clamped streak=1, got %d", effectiveQualityFloorStreak(cfg))
	}
}
