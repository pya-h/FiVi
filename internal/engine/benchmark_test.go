package engine

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/engines"
	"fivi/internal/logx"
	"fivi/internal/parser"
	"fivi/internal/probe"
)

type benchmarkAdapter struct{}

func (a benchmarkAdapter) Name() string {
	return "benchmark"
}

func (a benchmarkAdapter) EnsureBinary(path string) error {
	return nil
}

func (a benchmarkAdapter) BuildConfig(opts engines.ConfigBuildOpts) ([]byte, engines.Parsed, error) {
	return []byte(`{}`), engines.Parsed{Name: opts.RawURI, Protocol: "vmess"}, nil
}

func (a benchmarkAdapter) Start(ctx context.Context, bin string, cfgJSON []byte, name string) (engines.Instance, error) {
	return benchmarkInstance{}, nil
}

type benchmarkInstance struct{}

func (i benchmarkInstance) PID() int { return 0 }
func (i benchmarkInstance) Stop()    {}

func BenchmarkValidateCandidatesThroughput(b *testing.B) {
	origCheck := checkThroughSOCKSFn
	origMeasure := measureDownloadFn
	origWaitReady := waitForProxyReadyFn
	origReserve := reserveLocalAddrsFn
	b.Cleanup(func() {
		checkThroughSOCKSFn = origCheck
		measureDownloadFn = origMeasure
		waitForProxyReadyFn = origWaitReady
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
	measureDownloadFn = func(ctx context.Context, socksAddr, target string, timeout time.Duration, maxBytes int64) (float64, error) {
		return 0, nil
	}
	waitForProxyReadyFn = func(ctx context.Context, socksAddr, httpAddr string, timeout time.Duration) error {
		return nil
	}
	reserveLocalAddrsFn = func() (socksAddr, httpAddr string, err error) {
		return "127.0.0.1:10080", "127.0.0.1:10081", nil
	}

	cfg := config.AppConfig{
		Workers:          24,
		CandidateTimeout: 2 * time.Second,
		StartupTimeout:   200 * time.Millisecond,
		ProbeTimeout:     200 * time.Millisecond,
		ProbeMinSuccess:  1,
		Targets:          []string{"https://x.com/"},
		RequiredTargets:  []string{"x.com"},
	}

	cands := make([]parser.Candidate, 0, 120)
	for i := 0; i < 120; i++ {
		cands = append(cands, parser.Candidate{
			Raw:    fmt.Sprintf("vmess://bench-%d", i),
			Scheme: "vmess",
		})
	}

	ctx := context.Background()
	adapter := benchmarkAdapter{}
	restoreLogs := logx.SetOutput(io.Discard)
	b.Cleanup(restoreLogs)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		summary := validateCandidates(ctx, cfg, cands, nil, adapter, "bench-bin", nil)
		if len(summary.Working) == 0 {
			b.Fatalf("expected working candidates")
		}
	}
}
