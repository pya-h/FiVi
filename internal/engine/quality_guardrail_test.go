package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/probe"
)

func TestQualityGuardrail_TriggersSoftFailoverToNextCandidate(t *testing.T) {
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
			return probeResultWith(60*time.Millisecond, []time.Duration{60 * time.Millisecond, 62 * time.Millisecond, 59 * time.Millisecond}) // first verify
		case 2:
			return probeResultWith(220*time.Millisecond, []time.Duration{120 * time.Millisecond, 220 * time.Millisecond, 320 * time.Millisecond}) // breach 1
		case 3:
			return probeResultWith(240*time.Millisecond, []time.Duration{130 * time.Millisecond, 240 * time.Millisecond, 350 * time.Millisecond}) // breach 2 -> soft failover
		case 4:
			return probeResultWith(40*time.Millisecond, []time.Duration{40 * time.Millisecond, 45 * time.Millisecond, 42 * time.Millisecond}) // second verify
		case 5:
			cancel()
			return probeResultWith(40*time.Millisecond, []time.Duration{40 * time.Millisecond, 44 * time.Millisecond, 41 * time.Millisecond}) // second monitor
		default:
			return probeResultWith(50*time.Millisecond, []time.Duration{50 * time.Millisecond, 51 * time.Millisecond, 49 * time.Millisecond})
		}
	}

	cfg := config.AppConfig{
		Mode:                "proxy",
		ListenSocksAddr:     "127.0.0.1:10808",
		ListenHTTPAddr:      "127.0.0.1:10809",
		Targets:             []string{"https://x.com/", "https://youtube.com", "https://cloudflare.com"},
		ProbeTimeout:        50 * time.Millisecond,
		ProbeMinSuccess:     1,
		RequiredTargets:     nil,
		MonitorInterval:     2 * time.Millisecond,
		MonitorFailLimit:    5,
		QualityLatencyFloor: 180 * time.Millisecond,
		QualityJitterFloor:  80 * time.Millisecond,
		QualityFloorStreak:  2,
		StartupTimeout:      50 * time.Millisecond,
		SpeedTarget:         "https://example.com/",
		SpeedTestBytes:      4096,
	}

	adapter := &mockAdapter{}
	ranked := []Ranked{
		{Raw: "vmess://first", Name: "first"},
		{Raw: "vmess://second", Name: "second"},
	}

	err := connectAndMonitor(ctx, cfg, ranked, nil, adapter, "mockbin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled after soft failover to second candidate, got: %v", err)
	}
	if adapter.startCalls != 2 {
		t.Fatalf("expected 2 candidate starts, got %d", adapter.startCalls)
	}
}

func probeResultWith(avg time.Duration, latencies []time.Duration) probe.Result {
	targets := make([]probe.TargetResult, 0, len(latencies))
	for i, latency := range latencies {
		targets = append(targets, probe.TargetResult{URL: "https://target-" + string(rune('a'+i)), OK: true, Latency: latency})
	}
	return probe.Result{
		Successes:   len(targets),
		TargetCount: len(targets),
		AvgLatency:  avg,
		Targets:     targets,
	}
}
