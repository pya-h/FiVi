package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/engines"
	"fivi/internal/probe"
	"fivi/internal/routing"
)

type mockAdapter struct {
	startCalls int
	stopCalls  int
}

func (m *mockAdapter) Name() string {
	return "mock"
}

func (m *mockAdapter) EnsureBinary(path string) error {
	return nil
}

func (m *mockAdapter) BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr string, tunnelMode bool, _ []routing.Rule) ([]byte, engines.Parsed, error) {
	return []byte(`{}`), engines.Parsed{Name: rawURI, Protocol: "vmess"}, nil
}

func (m *mockAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (engines.Instance, error) {
	m.startCalls++
	return &mockInstance{
		pid:       1000 + m.startCalls,
		stopCalls: &m.stopCalls,
	}, nil
}

type mockInstance struct {
	pid       int
	stopCalls *int
}

func (i *mockInstance) PID() int {
	return i.pid
}

func (i *mockInstance) Stop() {
	if i.stopCalls != nil {
		*i.stopCalls++
	}
}

func TestConnectAndMonitorFailoverToNextCandidate(t *testing.T) {
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
			return probeResultOK()
		case 2:
			return probeResultFail()
		case 3:
			return probeResultOK()
		case 4:
			cancel()
			return probeResultOK()
		default:
			return probeResultOK()
		}
	}

	cfg := config.AppConfig{
		ListenSocksAddr:  "127.0.0.1:10808",
		ListenHTTPAddr:   "127.0.0.1:10809",
		Targets:          []string{"https://x.com/"},
		ProbeTimeout:     50 * time.Millisecond,
		ProbeMinSuccess:  1,
		RequiredTargets:  nil,
		MonitorInterval:  2 * time.Millisecond,
		MonitorFailLimit: 1,
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
		t.Fatalf("expected context canceled after failover to second candidate, got: %v", err)
	}
	if adapter.startCalls != 2 {
		t.Fatalf("expected 2 candidate starts (failover), got %d", adapter.startCalls)
	}
	if adapter.stopCalls < 2 {
		t.Fatalf("expected at least 2 stops, got %d", adapter.stopCalls)
	}
}

func probeResultOK() probe.Result {
	return probe.Result{
		Successes:   1,
		TargetCount: 1,
		AvgLatency:  20 * time.Millisecond,
		Targets: []probe.TargetResult{
			{URL: "https://x.com/", OK: true},
		},
	}
}

func probeResultFail() probe.Result {
	return probe.Result{
		Successes:   0,
		TargetCount: 1,
		AvgLatency:  200 * time.Millisecond,
		Targets: []probe.TargetResult{
			{URL: "https://x.com/", OK: false},
		},
	}
}
