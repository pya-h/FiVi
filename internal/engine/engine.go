package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"fivi/internal/config"
	"fivi/internal/fetch"
	"fivi/internal/logx"
	"fivi/internal/parser"
	"fivi/internal/probe"
	"fivi/internal/xray"
)

type Ranked struct {
	Raw           string
	Scheme        string
	Name          string
	Successes     int
	TargetCount   int
	AvgLatency    time.Duration
	BannedOK      bool
	Score         float64
	ValidationLog string
}

func Run(ctx context.Context, cfg config.AppConfig) error {
	if err := xray.EnsureBinary(cfg.XRAYBin); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ranked, err := discoverAndRank(ctx, cfg)
		if err != nil {
			logx.Errorf("discovery failed: %v", err)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}
		if len(ranked) == 0 {
			logx.Warnf("no working nodes found, retrying in %s", cfg.RetryBackoff)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}

		if err := connectAndMonitor(ctx, cfg, ranked); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			logx.Warnf("all ranked nodes failed; rediscovering: %v", err)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}
	}
}

func discoverAndRank(ctx context.Context, cfg config.AppConfig) ([]Ranked, error) {
	logx.Infof("fetching subscriptions from %d sources", len(cfg.BootstrapSources))
	sourceResults := fetch.FetchAll(ctx, cfg.BootstrapSources, 20*time.Second)
	all := make([]string, 0, 4096)
	for _, r := range sourceResults {
		if r.Err != nil {
			logx.Warnf("source failed %s: %v", r.Source, r.Err)
			continue
		}
		logx.Infof("source ok %s (%d links)", r.Source, len(r.Links))
		all = append(all, r.Links...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no links fetched")
	}

	cands := parser.Normalize(all, cfg.OnlySchemes, cfg.MaxCandidates)
	if len(cands) == 0 {
		return nil, fmt.Errorf("no candidates after normalize")
	}
	logx.Infof("testing %d candidates with %d workers", len(cands), cfg.Workers)

	ranked := validateCandidates(ctx, cfg, cands)
	if len(ranked) == 0 {
		return nil, nil
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].BannedOK != ranked[j].BannedOK {
			return ranked[i].BannedOK
		}
		if ranked[i].Successes != ranked[j].Successes {
			return ranked[i].Successes > ranked[j].Successes
		}
		if ranked[i].AvgLatency != ranked[j].AvgLatency {
			return ranked[i].AvgLatency < ranked[j].AvgLatency
		}
		return ranked[i].Score > ranked[j].Score
	})
	if len(ranked) > 0 {
		top := ranked[0]
		logx.Infof("best candidate: %s (%s) success=%d/%d latency=%s", displayName(top), top.Scheme, top.Successes, top.TargetCount, top.AvgLatency)
	}
	return ranked, nil
}

func validateCandidates(ctx context.Context, cfg config.AppConfig, cands []parser.Candidate) []Ranked {
	jobs := make(chan parser.Candidate)
	results := make(chan Ranked, len(cands))
	wg := sync.WaitGroup{}
	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range jobs {
				r, ok := testOne(ctx, cfg, c)
				if ok {
					results <- r
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, c := range cands {
			select {
			case <-ctx.Done():
				return
			case jobs <- c:
			}
		}
	}()

	wg.Wait()
	close(results)

	out := make([]Ranked, 0, len(cands))
	for r := range results {
		out = append(out, r)
	}
	logx.Infof("working candidates: %d/%d", len(out), len(cands))
	return out
}

func testOne(ctx context.Context, cfg config.AppConfig, c parser.Candidate) (Ranked, bool) {
	socksAddr, httpAddr, err := reserveLocalAddrs()
	if err != nil {
		return Ranked{}, false
	}
	configJSON, parsed, err := xray.BuildRuntimeConfigFromURI(c.Raw, socksAddr, httpAddr)
	if err != nil {
		return Ranked{}, false
	}

	runCtx, cancel := context.WithCancel(ctx)
	inst, err := xray.Start(runCtx, cfg.XRAYBin, configJSON, parsed.Name)
	if err != nil {
		cancel()
		return Ranked{}, false
	}
	defer func() {
		inst.Stop()
		cancel()
	}()

	sleepWithCancel(ctx, 850*time.Millisecond)
	res := probe.CheckThroughSOCKS(ctx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
	if res.Successes == 0 {
		return Ranked{}, false
	}
	bannedOK := hasRequiredBannedTargetsOK(res.Targets)
	if !bannedOK {
		return Ranked{}, false
	}
	latMs := float64(max(int(res.AvgLatency.Milliseconds()), 1))
	score := float64(res.Successes*1000) - latMs

	return Ranked{
		Raw:           c.Raw,
		Scheme:        c.Scheme,
		Name:          parsed.Name,
		Successes:     res.Successes,
		TargetCount:   res.TargetCount,
		AvgLatency:    res.AvgLatency,
		BannedOK:      bannedOK,
		Score:         score,
		ValidationLog: inst.LogPath(),
	}, true
}

func connectAndMonitor(ctx context.Context, cfg config.AppConfig, ranked []Ranked) error {
	for i := 0; i < len(ranked); i++ {
		cand := ranked[i]
		inst, err := startCandidate(ctx, cfg, cand.Raw, cfg.ListenSocksAddr, cfg.ListenHTTPAddr)
		if err != nil {
			logx.Warnf("connect failed for %s: %v", displayName(cand), err)
			continue
		}

		logx.Infof("connected to %s via %s | SOCKS=%s HTTP=%s PID=%d", displayName(cand), cand.Scheme, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, inst.PID())
		monitorErr := monitorConnection(ctx, cfg, inst)
		inst.Stop()
		if monitorErr == nil || errors.Is(monitorErr, context.Canceled) {
			return monitorErr
		}
		logx.Warnf("connection degraded for %s: %v", displayName(cand), monitorErr)
	}
	return fmt.Errorf("exhausted %d ranked nodes", len(ranked))
}

func monitorConnection(ctx context.Context, cfg config.AppConfig, inst *xray.Instance) error {
	ticker := time.NewTicker(cfg.MonitorInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			res := probe.CheckThroughSOCKS(ctx, cfg.ListenSocksAddr, cfg.Targets, cfg.ProbeTimeout)
			if res.Successes == len(cfg.Targets) && hasRequiredBannedTargetsOK(res.Targets) {
				failures = 0
				logx.Infof("health check ok latency=%s", res.AvgLatency)
				continue
			}
			failures++
			logx.Warnf("health check failed %d/%d (success=%d/%d)", failures, cfg.MonitorFailLimit, res.Successes, len(cfg.Targets))
			if failures >= cfg.MonitorFailLimit {
				return fmt.Errorf("health checks failed %d times", failures)
			}
		}
	}
}

func startCandidate(ctx context.Context, cfg config.AppConfig, rawURI, socksAddr, httpAddr string) (*xray.Instance, error) {
	cfgJSON, _, err := xray.BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr)
	if err != nil {
		return nil, err
	}
	runCtx, _ := context.WithCancel(ctx)
	inst, err := xray.Start(runCtx, cfg.XRAYBin, cfgJSON, "active")
	if err != nil {
		return nil, err
	}
	sleepWithCancel(ctx, 1200*time.Millisecond)
	res := probe.CheckThroughSOCKS(ctx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
	if res.Successes == 0 || !hasRequiredBannedTargetsOK(res.Targets) {
		inst.Stop()
		return nil, fmt.Errorf("initial probe failed success=%d/%d", res.Successes, len(cfg.Targets))
	}
	return inst, nil
}

func reserveLocalAddrs() (socksAddr, httpAddr string, err error) {
	s, err := reserveAddr()
	if err != nil {
		return "", "", err
	}
	h, err := reserveAddr()
	if err != nil {
		return "", "", err
	}
	return s, h, nil
}

func reserveAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr, nil
}

func sleepWithCancel(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func hasRequiredBannedTargetsOK(results []probe.TargetResult) bool {
	var xOK, yOK bool
	for _, r := range results {
		if strings.Contains(r.URL, "x.com") && r.OK {
			xOK = true
		}
		if strings.Contains(r.URL, "youtube.com") && r.OK {
			yOK = true
		}
	}
	return xOK && yOK
}

func displayName(r Ranked) string {
	if strings.TrimSpace(r.Name) != "" {
		return r.Name
	}
	if len(r.Raw) > 28 {
		return r.Raw[:28] + "..."
	}
	return r.Raw
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
