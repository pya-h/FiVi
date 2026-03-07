package engine

import (
	"context"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/engines"
	"fivi/internal/parser"
	"fivi/internal/probe"
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
