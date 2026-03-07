package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/probe"
)

func TestURLTestMode_ProactiveDropSwitchesToNextCandidate(t *testing.T) {
	origCheck := checkThroughSOCKSFn
	origMeasure := measureDownloadFn
	origWaitReady := waitForProxyReadyFn
	t.Cleanup(func() {
		checkThroughSOCKSFn = origCheck
		measureDownloadFn = origMeasure
		waitForProxyReadyFn = origWaitReady
	})

	waitForProxyReadyFn = func(ctx context.Context, socksAddr, httpAddr string, timeout time.Duration) error {
		return nil
	}
	measureDownloadFn = func(ctx context.Context, socksAddr, target string, timeout time.Duration, maxBytes int64) (float64, error) {
		return 0, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	call := 0
	checkThroughSOCKSFn = func(ctx context.Context, socksAddr string, targets []string, timeout time.Duration) probe.Result {
		call++
		switch call {
		case 1:
			return probeResultWithLatency(10 * time.Millisecond) // first candidate verify
		case 2:
			return probeResultWithLatency(10 * time.Millisecond) // first candidate monitor baseline
		case 3:
			return probeResultWithLatency(500 * time.Millisecond) // drop streak 1
		case 4:
			return probeResultWithLatency(500 * time.Millisecond) // drop streak 2 -> proactive switch
		case 5:
			return probeResultWithLatency(15 * time.Millisecond) // second candidate verify
		case 6:
			cancel()
			return probeResultWithLatency(15 * time.Millisecond) // second candidate monitor
		default:
			return probeResultWithLatency(20 * time.Millisecond)
		}
	}

	cfg := config.AppConfig{
		Mode:             "urltest",
		ListenSocksAddr:  "127.0.0.1:10808",
		ListenHTTPAddr:   "127.0.0.1:10809",
		Targets:          []string{"https://x.com/"},
		ProbeTimeout:     50 * time.Millisecond,
		ProbeMinSuccess:  1,
		RequiredTargets:  nil,
		MonitorInterval:  2 * time.Millisecond,
		MonitorFailLimit: 5,
		StartupTimeout:   50 * time.Millisecond,
		SpeedTarget:      "https://example.com/",
		SpeedTestBytes:   4096,
	}

	adapter := &mockAdapter{}
	ranked := []Ranked{
		{Raw: "vmess://first", Name: "first"},
		{Raw: "vmess://second", Name: "second"},
	}

	err := connectAndMonitor(ctx, cfg, ranked, nil, adapter, "mockbin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled after proactive switch to second candidate, got: %v", err)
	}
	if adapter.startCalls != 2 {
		t.Fatalf("expected 2 candidate starts (proactive switch), got %d", adapter.startCalls)
	}
	if adapter.stopCalls < 2 {
		t.Fatalf("expected at least 2 stops, got %d", adapter.stopCalls)
	}
}

func probeResultWithLatency(lat time.Duration) probe.Result {
	return probe.Result{
		Successes:   1,
		TargetCount: 1,
		AvgLatency:  lat,
		Targets: []probe.TargetResult{
			{URL: "https://x.com/", OK: true},
		},
	}
}
