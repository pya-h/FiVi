package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fivi/internal/cleanup"
	"fivi/internal/config"
	"fivi/internal/engines"
	"fivi/internal/fetch"
	"fivi/internal/latency"
	"fivi/internal/logx"
	"fivi/internal/pac"
	"fivi/internal/parser"
	"fivi/internal/probe"
	"fivi/internal/routing"
	"fivi/internal/session"
	"fivi/internal/sources"
	"fivi/internal/sysproxy"
)

type Ranked struct {
	Raw         string
	Scheme      string
	Name        string
	Successes   int
	TargetCount int
	AvgLatency  time.Duration
	BannedOK    bool
	Score       float64
}

type FailureReason string

const (
	FailureReasonContextCanceled  FailureReason = "context_canceled"
	FailureReasonCandidateTimeout FailureReason = "candidate_timeout"
	FailureReasonReserveAddr      FailureReason = "reserve_addr_error"
	FailureReasonParseCandidate   FailureReason = "candidate_parse_error"
	FailureReasonEngineStart      FailureReason = "engine_start_error"
	FailureReasonEngineNotReady   FailureReason = "engine_not_ready"
	FailureReasonProbeFailed      FailureReason = "probe_failed"
	FailureReasonProbePartial     FailureReason = "probe_partial"
	FailureReasonLatencyReject    FailureReason = "candidate_latency_reject"
)

type ProbableCandidate struct {
	Raw         string
	Scheme      string
	Name        string
	Successes   int
	TargetCount int
	AvgLatency  time.Duration
	Reason      FailureReason
}

type candidateOutcome struct {
	Raw      string
	Ranked   *Ranked
	Probable *ProbableCandidate
	Reason   FailureReason
}

type validationSummary struct {
	Working       []Ranked
	Probable      []ProbableCandidate
	FailureCounts map[FailureReason]int
	Processed     int
	Failed        int
}

type discoveryResult struct {
	Working  []Ranked
	Probable []ProbableCandidate
}

var (
	checkThroughSOCKSFn = probe.CheckThroughSOCKS
	measureDownloadFn   = probe.MeasureDownloadSpeedMbps
	waitForProxyReadyFn = waitForProxyReady
	reserveLocalAddrsFn = reserveLocalAddrs
	enableSystemProxyFn = sysproxy.Enable
	loadSessionFn       = session.LoadDefault
	saveSessionFn       = session.SaveDefault
	cleanupMgr          *cleanup.Manager
)

const persistedSessionMaxAge = time.Hour

// SetCleanupManager sets the global cleanup manager used by the engine to
// register safety-net cleanup hooks (proxy restore, engine kill).
func SetCleanupManager(m *cleanup.Manager) {
	cleanupMgr = m
}

func registerCleanup(name string, fn func()) {
	if cleanupMgr != nil {
		cleanupMgr.Register(name, fn)
	}
}

func splitHostPort(addr string) (string, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "127.0.0.1", 0
	}
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}

func Run(ctx context.Context, cfg config.AppConfig) error {
	return RunWithEvents(ctx, cfg, nil, nil)
}

func RunWithEvents(ctx context.Context, cfg config.AppConfig, events chan<- WorkerEvent, status chan<- StatusUpdate) error {
	adapter, err := adapterFromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWithAdapter(ctx, cfg, events, status, adapter)
}

// warmStandby holds pre-validated configs for instant failover.
type warmStandby struct {
	configs   []Ranked
	createdAt time.Time
}

func (w *warmStandby) isValid(ttl time.Duration) bool {
	if w == nil || len(w.configs) == 0 || ttl <= 0 {
		return false
	}
	return time.Since(w.createdAt) < ttl
}

const warmStandbyMaxConfigs = 3

func RunWithAdapter(ctx context.Context, cfg config.AppConfig, events chan<- WorkerEvent, status chan<- StatusUpdate, adapter engines.Adapter) error {
	if adapter == nil {
		adapter = engines.NewXrayAdapter()
	}
	if err := ensureListenAddrsAvailable(cfg.ListenSocksAddr, cfg.ListenHTTPAddr); err != nil {
		return err
	}
	engineBin := binaryForAdapter(cfg, adapter)
	if err := adapter.EnsureBinary(engineBin); err != nil {
		return err
	}

	// Resolve geo rule sets if --geo-bypass is set.
	if cfg.GeoBypass != "" && cfg.GeoRuleSets == nil {
		cacheDir, err := routing.DefaultGeoCacheDir()
		if err != nil {
			return fmt.Errorf("geo-bypass: %w", err)
		}
		logx.Infof("downloading geo rule sets for %q", cfg.GeoBypass)
		geo, err := routing.EnsureGeoRuleSets(cfg.GeoBypass, cacheDir)
		if err != nil {
			return fmt.Errorf("geo-bypass: %w", err)
		}
		cfg.GeoRuleSets = geo
		logx.Infof("geo rule sets ready: geoip=%s geosite=%s", geo.GeoIPPath, geo.GeositePath)
	}

	health := sources.NewHealthTracker(cfg.BootstrapSources)

	if persisted, ok, err := loadSessionFn(persistedSessionMaxAge, time.Now()); err != nil {
		logx.Warnf("session: failed to load persisted state: %v", err)
	} else if ok {
		name := "persisted"
		scheme := "unknown"
		if parts := strings.SplitN(strings.TrimSpace(persisted.RawURI), "://", 2); len(parts) == 2 {
			scheme = strings.ToLower(strings.TrimSpace(parts[0]))
		}
		persistedCandidate := Ranked{
			Raw:    persisted.RawURI,
			Name:   name,
			Scheme: scheme,
			Score:  persisted.Score,
		}
		logx.Infof("trying persisted session candidate first (saved_at=%s)", persisted.SavedAt.Format(time.RFC3339))
		if err := connectAndMonitor(ctx, cfg, []Ranked{persistedCandidate}, status, adapter, engineBin); err == nil || errors.Is(err, context.Canceled) {
			return err
		} else {
			logx.Warnf("persisted session candidate failed, falling back to discovery: %v", err)
		}
	}

	var standby *warmStandby

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		emitStatus(status, StatusUpdate{Phase: PhaseDiscovering})
		discovered, err := discoverAndRank(ctx, cfg, health, events, status, adapter, engineBin)
		if err != nil {
			logx.Errorf("discovery failed: %v", err)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}
		ranked := discovered.Working
		if len(ranked) == 0 {
			if len(discovered.Probable) > 0 {
				fallback := rankedFromProbable(discovered.Probable)
				logx.Warnf("no working nodes found; trying %d probable candidates as plan-B", len(fallback))
				if err := connectAndMonitor(ctx, cfg, fallback, status, adapter, engineBin); err == nil || errors.Is(err, context.Canceled) {
					return err
				}
				logx.Warnf("probable plan-B candidates failed; retrying discovery in %s", cfg.RetryBackoff)
				sleepWithCancel(ctx, cfg.RetryBackoff)
				continue
			}
			logx.Warnf("no working/probable nodes found, retrying in %s", cfg.RetryBackoff)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}

		// Save top configs as warm standbys for future failover.
		n := warmStandbyMaxConfigs
		if n > len(ranked) {
			n = len(ranked)
		}
		standby = &warmStandby{
			configs:   append([]Ranked(nil), ranked[:n]...),
			createdAt: time.Now(),
		}

		if err := connectAndMonitor(ctx, cfg, ranked, status, adapter, engineBin); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			// Try warm standby configs before full rediscovery.
			if standby.isValid(cfg.StandbyTTL) {
				logx.Infof("trying %d warm standby configs before rediscovery", len(standby.configs))
				emitStatus(status, StatusUpdate{Phase: PhaseWarmFailover})
				if warmErr := connectAndMonitor(ctx, cfg, standby.configs, status, adapter, engineBin); warmErr == nil {
					continue
				} else if errors.Is(warmErr, context.Canceled) {
					return warmErr
				}
				logx.Warnf("warm standby configs exhausted; falling back to full rediscovery")
			}
			logx.Warnf("all ranked nodes failed; rediscovering: %v", err)
			sleepWithCancel(ctx, cfg.RetryBackoff)
			continue
		}
	}
}

func discoverAndRank(ctx context.Context, cfg config.AppConfig, health *sources.HealthTracker, events chan<- WorkerEvent, status chan<- StatusUpdate, adapter engines.Adapter, engineBin string) (discoveryResult, error) {
	roundSources := selectSourcesForRound(cfg.BootstrapSources, health, time.Now())
	logx.Infof("fetching subscriptions from %d/%d sources", len(roundSources), len(cfg.BootstrapSources))
	attemptedSources := make(map[string]struct{}, len(roundSources))
	for _, src := range roundSources {
		attemptedSources[src] = struct{}{}
	}

	fetchTimeout := cfg.FetchTimeout
	if fetchTimeout <= 0 {
		fetchTimeout = 25 * time.Second
	}
	sourceResults := fetch.FetchAll(ctx, roundSources, fetchTimeout)
	all := make([]string, 0, 4096)
	for _, r := range sourceResults {
		if r.Err == nil && len(r.Links) > 0 {
			// Cache successful fetches for restricted-network resilience.
			fetch.SaveToCache(r.Source, r.Links)
		}
		if r.Err != nil || len(r.Links) == 0 {
			// Try cached data when live fetch fails or source returns no links.
			if cached, ok := fetch.LoadFromCache(r.Source); ok {
				if r.Err != nil {
					logx.Warnf("source failed %s: %v (using %d cached links)", r.Source, r.Err, len(cached))
				} else {
					logx.Warnf("source returned no links %s (using %d cached links)", r.Source, len(cached))
				}
				if health != nil {
					health.Observe(r.Source, len(cached), nil, time.Now())
				}
				all = append(all, cached...)
				continue
			}
		}
		if health != nil {
			health.Observe(r.Source, len(r.Links), r.Err, time.Now())
		}
		if r.Err != nil {
			logx.Warnf("source failed %s: %v", r.Source, r.Err)
			continue
		}
		if len(r.Links) == 0 {
			logx.Warnf("source returned no links %s", r.Source)
			continue
		}
		logx.Infof("source ok %s (%d links)", r.Source, len(r.Links))
		all = append(all, r.Links...)
	}

	// Sources skipped this round (cooldown) can still contribute cached links.
	for _, src := range cfg.BootstrapSources {
		if _, attempted := attemptedSources[src]; attempted {
			continue
		}
		if cached, ok := fetch.LoadFromCache(src); ok {
			logx.Warnf("source in cooldown %s (using %d cached links)", src, len(cached))
			all = append(all, cached...)
		}
	}
	if health != nil {
		stats := health.Snapshot()
		printSourceHealthSummary(stats)
		if err := writeSourceReport(stats); err != nil {
			logx.Warnf("failed to write source report: %v", err)
		}
	}
	if len(all) == 0 {
		if len(cfg.DirectConfigs) == 0 {
			return discoveryResult{}, fmt.Errorf("no links fetched")
		}
	}

	if len(cfg.DirectConfigs) > 0 {
		all = append(all, cfg.DirectConfigs...)
		logx.Infof("added %d direct single configs", len(cfg.DirectConfigs))
	}

	cands := parser.Normalize(all, cfg.OnlySchemes, cfg.MaxCandidates)
	if len(cands) == 0 {
		return discoveryResult{}, fmt.Errorf("no candidates after normalize")
	}
	logx.Infof("fetched %d raw links, %d unique candidates after dedup", len(all), len(cands))

	candLatency := map[string]time.Duration{}

	// Latency pre-scan: TCP connect to server addresses, reject slow/unreachable.
	if cfg.LatencyCutoff > 0 {
		logx.Infof("latency pre-scan: checking %d candidates (cutoff=%s)", len(cands), cfg.LatencyCutoff)
		dialTimeout := 2 * time.Second
		if cfg.LatencyCutoff < dialTimeout {
			dialTimeout = cfg.LatencyCutoff
		}
		var latencyRejected int
		cands, latencyRejected, candLatency = latency.ScanBatchWithLatency(ctx, cands, dialTimeout, cfg.LatencyCutoff, 50)
		if latencyRejected > 0 {
			logx.Infof("latency pre-scan: rejected %d unreachable/slow candidates, %d remaining", latencyRejected, len(cands))
		}
		if len(cands) == 0 {
			return discoveryResult{}, fmt.Errorf("no candidates after latency pre-scan")
		}
	}

	logx.Infof("testing %d candidates with %d workers", len(cands), cfg.Workers)
	emitStatus(status, StatusUpdate{Phase: PhaseDiscovering, TotalCands: len(cands)})

	// Write all fetched configs before validation.
	writeAllConfigs(cands)

	validation := validateCandidates(ctx, cfg, cands, events, adapter, engineBin, candLatency)
	if len(validation.Working) == 0 {
		if err := writeCandidateBuckets(nil, validation.Probable); err != nil {
			logx.Warnf("failed to write candidate buckets: %v", err)
		}
		return discoveryResult{Working: nil, Probable: validation.Probable}, nil
	}

	ranked := validation.Working
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
	if err := writeCandidateBuckets(ranked, validation.Probable); err != nil {
		logx.Warnf("failed to write candidate buckets: %v", err)
	}
	printTopCandidates(ranked, cfg.TopCount)
	return discoveryResult{Working: ranked, Probable: validation.Probable}, nil
}

func validateCandidates(ctx context.Context, cfg config.AppConfig, cands []parser.Candidate, events chan<- WorkerEvent, adapter engines.Adapter, engineBin string, latencies map[string]time.Duration) validationSummary {
	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan parser.Candidate)
	outcomes := make(chan candidateOutcome, workers*2)
	wg := sync.WaitGroup{}

	for i := 0; i < workers; i++ {
		workerID := i + 1
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case c, ok := <-jobs:
					if !ok {
						return
					}
					outcome := testOne(ctx, cfg, c, workerID, events, adapter, engineBin)
					if outcome.Ranked == nil && outcome.Probable == nil && outcome.Reason == FailureReasonProbeFailed {
						if lat, ok := latencies[c.Raw]; ok && shouldPromoteLatencyProbable(cfg, lat) {
							outcome.Probable = &ProbableCandidate{
								Raw:         c.Raw,
								Scheme:      c.Scheme,
								Name:        "low-latency-unverified",
								Successes:   0,
								TargetCount: len(cfg.Targets),
								AvgLatency:  lat,
								Reason:      FailureReasonProbePartial,
							}
							outcome.Reason = FailureReasonProbePartial
						}
					}
					select {
					case outcomes <- outcome:
					case <-ctx.Done():
						return
					}
				}
			}
		}(workerID)
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

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	summary := validationSummary{
		Working:       make([]Ranked, 0, len(cands)),
		Probable:      make([]ProbableCandidate, 0, len(cands)),
		FailureCounts: make(map[FailureReason]int),
	}
	for outcome := range outcomes {
		summary.Processed++
		switch {
		case outcome.Ranked != nil:
			summary.Working = append(summary.Working, *outcome.Ranked)
		case outcome.Probable != nil:
			summary.Probable = append(summary.Probable, *outcome.Probable)
		default:
			reason := outcome.Reason
			if reason == "" {
				reason = FailureReasonProbeFailed
			}
			summary.Failed++
			summary.FailureCounts[reason]++
		}
	}

	printValidationSummary(summary, len(cands))
	return summary
}

func testOne(ctx context.Context, cfg config.AppConfig, c parser.Candidate, workerID int, events chan<- WorkerEvent, adapter engines.Adapter, engineBin string) candidateOutcome {
	candidateCtx, cancel := context.WithTimeout(ctx, effectiveCandidateTimeout(cfg))
	defer cancel()

	candidateStarted := time.Now()
	emitDone := func(status WorkerStatus, reason FailureReason, err error, attempt int) {
		emitStageEvent(events, workerID, c.Raw, WorkerStageDone, status, attempt, reason, err, candidateStarted)
	}

	lastReason := FailureReasonEngineStart
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-candidateCtx.Done():
			reason := candidateFailureReasonFromCtx(candidateCtx)
			emitDone(WorkerStatusFailed, reason, candidateCtx.Err(), attempt)
			return candidateOutcome{Raw: c.Raw, Reason: reason}
		default:
		}

		parseStarted := time.Now()
		emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusRunning, attempt, "", nil, parseStarted)
		socksAddr, httpAddr, err := reserveLocalAddrsFn()
		if err != nil {
			emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusFailed, attempt, FailureReasonReserveAddr, err, parseStarted)
			emitDone(WorkerStatusFailed, FailureReasonReserveAddr, err, attempt)
			return candidateOutcome{Raw: c.Raw, Reason: FailureReasonReserveAddr}
		}
		// Candidate testing should match selected mode. Tunnel mode candidates are
		// validated with TunnelMode=true to avoid proxy-only false negatives.
		configJSON, parsed, err := adapter.BuildConfig(engines.ConfigBuildOpts{
			RawURI:     c.Raw,
			SocksAddr:  socksAddr,
			HTTPAddr:   httpAddr,
			TunnelMode: cfg.IsTunnelMode(),
			Rules:      cfg.RoutingRules,
			DNS:        cfg.DNS,
			Geo:        cfg.GeoRuleSets,
		})
		if err != nil {
			emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusFailed, attempt, FailureReasonParseCandidate, err, parseStarted)
			emitDone(WorkerStatusFailed, FailureReasonParseCandidate, err, attempt)
			return candidateOutcome{Raw: c.Raw, Reason: FailureReasonParseCandidate}
		}
		emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusOK, attempt, "", nil, parseStarted)

		spawnStarted := time.Now()
		emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusRunning, attempt, "", nil, spawnStarted)
		inst, err := adapter.Start(candidateCtx, engineBin, configJSON, parsed.Name)
		if err != nil {
			lastReason = FailureReasonEngineStart
			emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusFailed, attempt, FailureReasonEngineStart, err, spawnStarted)
			continue
		}
		if err := waitForProxyReadyFn(candidateCtx, socksAddr, httpAddr, cfg.StartupTimeout); err != nil {
			inst.Stop()
			lastReason = FailureReasonEngineNotReady
			emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusFailed, attempt, FailureReasonEngineNotReady, err, spawnStarted)
			continue
		}
		emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusOK, attempt, "", nil, spawnStarted)

		probeStarted := time.Now()
		emitStageEvent(events, workerID, c.Raw, WorkerStageProbe, WorkerStatusRunning, attempt, "", nil, probeStarted)
		res := checkThroughSOCKSFn(candidateCtx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
		inst.Stop()
		if isProbeQualified(cfg, res) {
			ranked := Ranked{
				Raw:         c.Raw,
				Scheme:      c.Scheme,
				Name:        parsed.Name,
				Successes:   res.Successes,
				TargetCount: res.TargetCount,
				AvgLatency:  res.AvgLatency,
				BannedOK:    hasRequiredTargetsOK(res.Targets, cfg.RequiredTargets),
				Score:       scoreCandidate(res.Successes, res.AvgLatency),
			}
			emitStageEvent(events, workerID, c.Raw, WorkerStageProbe, WorkerStatusOK, attempt, "", nil, probeStarted)
			emitDone(WorkerStatusOK, "", nil, attempt)
			return candidateOutcome{Raw: c.Raw, Ranked: &ranked}
		}
		if res.Successes > 0 {
			probable := ProbableCandidate{
				Raw:         c.Raw,
				Scheme:      c.Scheme,
				Name:        parsed.Name,
				Successes:   res.Successes,
				TargetCount: res.TargetCount,
				AvgLatency:  res.AvgLatency,
				Reason:      FailureReasonProbePartial,
			}
			emitStageEvent(events, workerID, c.Raw, WorkerStageProbe, WorkerStatusFailed, attempt, FailureReasonProbePartial, nil, probeStarted)
			emitDone(WorkerStatusProbable, FailureReasonProbePartial, nil, attempt)
			return candidateOutcome{Raw: c.Raw, Probable: &probable, Reason: FailureReasonProbePartial}
		}
		lastReason = FailureReasonProbeFailed
		emitStageEvent(events, workerID, c.Raw, WorkerStageProbe, WorkerStatusFailed, attempt, FailureReasonProbeFailed, nil, probeStarted)
	}

	if candidateCtx.Err() != nil {
		reason := candidateFailureReasonFromCtx(candidateCtx)
		emitDone(WorkerStatusFailed, reason, candidateCtx.Err(), 3)
		return candidateOutcome{Raw: c.Raw, Reason: reason}
	}
	emitDone(WorkerStatusFailed, lastReason, nil, 3)
	return candidateOutcome{Raw: c.Raw, Reason: lastReason}
}

func candidateFailureReasonFromCtx(ctx context.Context) FailureReason {
	if ctx == nil {
		return FailureReasonContextCanceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return FailureReasonCandidateTimeout
	}
	return FailureReasonContextCanceled
}

func effectiveCandidateTimeout(cfg config.AppConfig) time.Duration {
	if cfg.CandidateTimeout > 0 {
		return cfg.CandidateTimeout
	}
	return 24 * time.Second
}

func waitForProxyReady(ctx context.Context, socksAddr, httpAddr string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if isListeningAddr(socksAddr) && isListeningAddr(httpAddr) {
			return nil
		}
		select {
		case <-readyCtx.Done():
			return fmt.Errorf("proxy listeners not ready within %s", timeout)
		case <-ticker.C:
		}
	}
}

func isListeningAddr(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

const (
	autoReconnectMaxAttempts = 3
	autoReconnectBaseBackoff = 2 * time.Second
	autoReconnectMaxBackoff  = 30 * time.Second
)

func connectAndMonitor(ctx context.Context, cfg config.AppConfig, ranked []Ranked, status chan<- StatusUpdate, adapter engines.Adapter, engineBin string) error {
	for i := 0; i < len(ranked); i++ {
		cand := ranked[i]

		err := connectAndMonitorOne(ctx, cfg, cand, status, adapter, engineBin)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}

		// Auto-reconnect: retry the same config with exponential backoff
		// before moving to the next ranked candidate.
		if cfg.AutoReconnect {
			reconnected := false
			backoff := autoReconnectBaseBackoff
			for attempt := 1; attempt <= autoReconnectMaxAttempts; attempt++ {
				logx.Infof("auto-reconnect attempt %d/%d for %s (backoff=%s)", attempt, autoReconnectMaxAttempts, displayName(cand), backoff)
				sleepWithCancel(ctx, backoff)
				if ctx.Err() != nil {
					return ctx.Err()
				}

				retryErr := connectAndMonitorOne(ctx, cfg, cand, status, adapter, engineBin)
				if retryErr == nil || errors.Is(retryErr, context.Canceled) {
					return retryErr
				}
				logx.Warnf("auto-reconnect attempt %d failed for %s: %v", attempt, displayName(cand), retryErr)

				backoff *= 2
				if backoff > autoReconnectMaxBackoff {
					backoff = autoReconnectMaxBackoff
				}
			}
			if !reconnected {
				logx.Warnf("auto-reconnect exhausted for %s, trying next candidate", displayName(cand))
			}
		}

		emitStatus(status, StatusUpdate{
			Phase:    PhaseFailover,
			NodeName: displayName(cand),
			Protocol: cand.Scheme,
		})
		logx.Warnf("connection degraded for %s: %v", displayName(cand), err)
	}
	return fmt.Errorf("exhausted %d ranked nodes", len(ranked))
}

func connectAndMonitorOne(ctx context.Context, cfg config.AppConfig, cand Ranked, status chan<- StatusUpdate, adapter engines.Adapter, engineBin string) error {
	emitStatus(status, StatusUpdate{
		Phase:    PhaseConnecting,
		NodeName: displayName(cand),
		Protocol: cand.Scheme,
	})
	inst, initialProbe, initialSpeed, err := startCandidate(ctx, cfg, cand.Raw, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, adapter, engineBin)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	connectedAt := time.Now()
	if saveErr := saveSessionFn(session.State{
		RawURI:  cand.Raw,
		Score:   cand.Score,
		SavedAt: connectedAt,
	}); saveErr != nil {
		logx.Warnf("session: failed to persist last working candidate: %v", saveErr)
	}
	if cfg.IsTunnelMode() {
		logx.Infof("tunnel mode active: all system traffic is routed through the proxy")
	}

	// Register engine instance for safety-net cleanup on crash/signal.
	registerCleanup("stop engine process", func() { inst.Stop() })

	// In proxy mode, configure system proxy so browsers use the connection.
	var restoreProxy func()
	if !cfg.IsTunnelMode() {
		sysSocksAddr := cfg.ListenSocksAddr
		sysHTTPAddr := cfg.ListenHTTPAddr
		if cfg.ShareMode {
			sysSocksAddr = resolveSystemProxyAddr(cfg.ListenSocksAddr)
			sysHTTPAddr = resolveSystemProxyAddr(cfg.ListenHTTPAddr)
		}
		socksHost, socksPort := splitHostPort(sysSocksAddr)
		httpHost, httpPort := splitHostPort(sysHTTPAddr)
		restore, proxyErr := enableSystemProxyFn(socksHost, socksPort, httpHost, httpPort)
		if proxyErr != nil {
			logx.Warnf("system proxy setup failed: %v (browsers need manual proxy config)", proxyErr)
		} else {
			restoreProxy = restore
			registerCleanup("restore system proxy", restore)
			logx.Infof("system proxy configured (socks=%s, http=%s)", sysSocksAddr, sysHTTPAddr)
		}
	}

	// In proxy mode, start PAC script server.
	var pacSrv *pac.Server
	if !cfg.IsTunnelMode() {
		pacAddr := pac.PACAddr(cfg.ListenHTTPAddr)
		srv, pacErr := pac.Start(pacAddr, cfg.ListenSocksAddr, cfg.RoutingRules)
		if pacErr != nil {
			logx.Warnf("PAC server failed to start: %v", pacErr)
		} else {
			pacSrv = srv
			registerCleanup("stop PAC server", srv.Stop)
			logx.Infof("PAC script available at %s", srv.URL())
		}
	}

	if cfg.ShareMode && !cfg.IsTunnelMode() {
		printLANShareInfo(cfg.ListenSocksAddr, cfg.ListenHTTPAddr)
	}

	printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, inst.PID(), connectedAt, initialProbe, initialSpeed, 0)
	emitStatus(status, StatusUpdate{
		Phase:       PhaseMonitoring,
		NodeName:    displayName(cand),
		Protocol:    cand.Scheme,
		Ping:        initialProbe.AvgLatency,
		Speed:       initialSpeed,
		ProbeOK:     initialProbe.Successes,
		ProbeTotal:  initialProbe.TargetCount,
		ConnectedAt: connectedAt,
		SocksAddr:   cfg.ListenSocksAddr,
		HTTPAddr:    cfg.ListenHTTPAddr,
	})
	monitorErr := monitorConnection(ctx, cfg, cand, connectedAt, status)
	if pacSrv != nil {
		pacSrv.Stop()
		logx.Infof("PAC server stopped")
	}
	if restoreProxy != nil {
		restoreProxy()
		logx.Infof("system proxy restored")
	}
	inst.Stop()
	return monitorErr
}

func monitorConnection(ctx context.Context, cfg config.AppConfig, cand Ranked, connectedAt time.Time, status chan<- StatusUpdate) error {
	ticker := time.NewTicker(cfg.MonitorInterval)
	defer ticker.Stop()

	failures := 0
	tick := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			tick++
			res := checkThroughSOCKSFn(ctx, cfg.ListenSocksAddr, cfg.Targets, cfg.ProbeTimeout)
			if isProbeQualified(cfg, res) {
				failures = 0
				speed := 0.0
				if tick%3 == 0 {
					s, err := measureDownloadFn(ctx, cfg.ListenSocksAddr, cfg.SpeedTarget, cfg.ProbeTimeout, cfg.SpeedTestBytes)
					if err == nil {
						speed = s
					}
				}
				printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, 0, connectedAt, res, speed, failures)
				emitStatus(status, StatusUpdate{
					Phase:       PhaseMonitoring,
					NodeName:    displayName(cand),
					Protocol:    cand.Scheme,
					Ping:        res.AvgLatency,
					Speed:       speed,
					ProbeOK:     res.Successes,
					ProbeTotal:  res.TargetCount,
					Failures:    failures,
					ConnectedAt: connectedAt,
					SocksAddr:   cfg.ListenSocksAddr,
					HTTPAddr:    cfg.ListenHTTPAddr,
				})
				continue
			}
			failures++
			printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, 0, connectedAt, res, 0, failures)
			emitStatus(status, StatusUpdate{
				Phase:       PhaseMonitoring,
				NodeName:    displayName(cand),
				Protocol:    cand.Scheme,
				Ping:        res.AvgLatency,
				ProbeOK:     res.Successes,
				ProbeTotal:  res.TargetCount,
				Failures:    failures,
				ConnectedAt: connectedAt,
				SocksAddr:   cfg.ListenSocksAddr,
				HTTPAddr:    cfg.ListenHTTPAddr,
			})
			if failures >= cfg.MonitorFailLimit {
				return fmt.Errorf("health checks failed %d times", failures)
			}
		}
	}
}

func startCandidate(ctx context.Context, cfg config.AppConfig, rawURI, socksAddr, httpAddr string, adapter engines.Adapter, engineBin string) (engines.Instance, probe.Result, float64, error) {
	// Active connection uses the configured mode (tunnel adds TUN inbound).
	cfgJSON, _, err := adapter.BuildConfig(engines.ConfigBuildOpts{
		RawURI:     rawURI,
		SocksAddr:  socksAddr,
		HTTPAddr:   httpAddr,
		TunnelMode: cfg.IsTunnelMode(),
		Rules:      cfg.RoutingRules,
		DNS:        cfg.DNS,
		Geo:        cfg.GeoRuleSets,
	})
	if err != nil {
		return nil, probe.Result{}, 0, err
	}
	inst, err := adapter.Start(ctx, engineBin, cfgJSON, "active")
	if err != nil {
		return nil, probe.Result{}, 0, err
	}
	if err := waitForProxyReadyFn(ctx, socksAddr, httpAddr, cfg.StartupTimeout); err != nil {
		inst.Stop()
		return nil, probe.Result{}, 0, err
	}

	// Post-connect verification loop: retry connectivity check to catch cases
	// where the proxy port is listening but the tunnel is not yet established.
	retries := cfg.VerifyRetries
	if retries < 1 {
		retries = 1
	}
	var res probe.Result
	for attempt := 1; attempt <= retries; attempt++ {
		res = checkThroughSOCKSFn(ctx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
		if isProbeQualified(cfg, res) {
			speed, _ := measureDownloadFn(ctx, socksAddr, cfg.SpeedTarget, cfg.ProbeTimeout, cfg.SpeedTestBytes)
			return inst, res, speed, nil
		}
		if attempt < retries {
			logx.Warnf("verification attempt %d/%d failed (success=%d/%d), retrying in 2s", attempt, retries, res.Successes, res.TargetCount)
			sleepWithCancel(ctx, 2*time.Second)
			if ctx.Err() != nil {
				inst.Stop()
				return nil, probe.Result{}, 0, ctx.Err()
			}
		}
	}
	inst.Stop()
	return nil, probe.Result{}, 0, fmt.Errorf("post-connect verification failed after %d attempts (success=%d/%d)", retries, res.Successes, res.TargetCount)
}

func isProbeQualified(cfg config.AppConfig, res probe.Result) bool {
	minSuccess := cfg.ProbeMinSuccess
	if minSuccess < 1 {
		minSuccess = 1
	}
	if minSuccess > len(cfg.Targets) {
		minSuccess = len(cfg.Targets)
	}
	if res.Successes < minSuccess {
		return false
	}
	return hasRequiredTargetsOK(res.Targets, cfg.RequiredTargets)
}

func sleepWithCancel(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func ensureListenAddrsAvailable(socksAddr, httpAddr string) error {
	if err := ensureListenAddrAvailable("socks", socksAddr); err != nil {
		return err
	}
	return ensureListenAddrAvailable("http", httpAddr)
}

func ensureListenAddrAvailable(kind, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf(
			"%s listen address %s is unavailable: %w; choose another via --%s-addr or stop the process using this port",
			strings.ToUpper(kind),
			addr,
			err,
			kind,
		)
	}
	_ = ln.Close()
	return nil
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

func resolveSystemProxyAddr(listenAddr string) string {
	host, port := splitHostPort(listenAddr)
	if !isWildcardHost(host) {
		return listenAddr
	}
	resolvedHost := resolveHostForSystemProxy(localLANIPs())
	if resolvedHost == "" {
		resolvedHost = "127.0.0.1"
	}
	return net.JoinHostPort(resolvedHost, strconv.Itoa(port))
}

func isWildcardHost(host string) bool {
	host = strings.TrimSpace(host)
	return host == "" || host == "0.0.0.0" || host == "::"
}

func resolveHostForSystemProxy(lanIPs []string) string {
	return resolveHostForSystemProxyWithPreferred(lanIPs, primaryOutboundIPv4())
}

func resolveHostForSystemProxyWithPreferred(lanIPs []string, preferred string) string {
	if parsed := net.ParseIP(strings.TrimSpace(preferred)); parsed != nil {
		if v4 := parsed.To4(); v4 != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
			return v4.String()
		}
	}

	for _, ip := range lanIPs {
		parsed := net.ParseIP(strings.TrimSpace(ip))
		if parsed == nil || parsed.IsLoopback() || parsed.IsUnspecified() {
			continue
		}
		if v4 := parsed.To4(); v4 != nil {
			if isPrivateIPv4(v4) {
				return v4.String()
			}
		}
	}

	for _, ip := range lanIPs {
		if parsed := net.ParseIP(strings.TrimSpace(ip)); parsed != nil {
			if parsed.IsLoopback() || parsed.IsUnspecified() {
				continue
			}
			if v4 := parsed.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

func primaryOutboundIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr == nil || udpAddr.IP == nil {
		return ""
	}
	if v4 := udpAddr.IP.To4(); v4 != nil {
		return v4.String()
	}
	return ""
}

func isPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 10 {
		return true
	}
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

func localLANIPs() []string {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]string, 0, 8)
	for _, iface := range ifs {
		if isLikelyVirtualInterface(iface.Name) {
			continue
		}
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				out = append(out, v4.String())
			}
		}
	}
	return out
}

func isLikelyVirtualInterface(name string) bool {
	l := strings.ToLower(strings.TrimSpace(name))
	if l == "" {
		return false
	}
	virtualPrefixes := []string{"docker", "br-", "veth", "virbr", "vmnet", "zt", "tailscale", "tun", "wg"}
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

func printLANShareInfo(socksAddr, httpAddr string) {
	_, socksPort := splitHostPort(socksAddr)
	_, httpPort := splitHostPort(httpAddr)
	ips := localLANIPs()
	logx.Warnf("LAN sharing is enabled: proxy listeners are exposed to your local network")
	if len(ips) == 0 {
		logx.Warnf("could not detect LAN IPs automatically; find your local IP and use ports socks=%d http=%d", socksPort, httpPort)
		return
	}
	for _, ip := range ips {
		logx.Infof("LAN share endpoint: socks5://%s | http://%s", net.JoinHostPort(ip, strconv.Itoa(socksPort)), net.JoinHostPort(ip, strconv.Itoa(httpPort)))
	}
}

func hasRequiredTargetsOK(results []probe.TargetResult, requiredTokens []string) bool {
	if len(requiredTokens) == 0 {
		return true
	}
	for _, token := range requiredTokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		ok := false
		for _, r := range results {
			if strings.Contains(r.URL, token) && r.OK {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func displayName(r Ranked) string {
	if strings.TrimSpace(r.Name) != "" {
		return r.Name
	}
	if len(r.Raw) > 42 {
		return r.Raw[:42] + "..."
	}
	return r.Raw
}

func printTopCandidates(ranked []Ranked, topN int) {
	if topN <= 0 {
		topN = 5
	}
	if topN > len(ranked) {
		topN = len(ranked)
	}
	logx.Infof("Top %d candidates:", topN)
	for i := 0; i < topN; i++ {
		r := ranked[i]
		logx.Infof("#%d %s | %s | success=%d/%d | ping=%s | score=%.1f", i+1, displayName(r), r.Scheme, r.Successes, r.TargetCount, r.AvgLatency.Round(time.Millisecond), r.Score)
	}
}

func printConnectionStatus(c Ranked, socksAddr, httpAddr string, pid int, connectedAt time.Time, res probe.Result, speedMbps float64, failures int) {
	status := "OK"
	if failures > 0 {
		status = fmt.Sprintf("DEGRADED(%d)", failures)
	}
	pidPart := ""
	if pid > 0 {
		pidPart = fmt.Sprintf(" pid=%d", pid)
	}
	speedPart := "n/a"
	if speedMbps > 0 {
		speedPart = fmt.Sprintf("%.2fMbps", speedMbps)
	}
	logx.Infof("[%s] %s | uptime=%s | ping=%s | speed=%s | success=%d/%d | socks=%s http=%s%s",
		status,
		displayName(c),
		time.Since(connectedAt).Round(time.Second),
		res.AvgLatency.Round(time.Millisecond),
		speedPart,
		res.Successes,
		res.TargetCount,
		socksAddr,
		httpAddr,
		pidPart,
	)
}

func printValidationSummary(summary validationSummary, totalCandidates int) {
	logx.Infof(
		"candidate summary: processed=%d/%d | working=%d | probable=%d | failed=%d",
		summary.Processed,
		totalCandidates,
		len(summary.Working),
		len(summary.Probable),
		summary.Failed,
	)
	if len(summary.FailureCounts) == 0 {
		return
	}

	type reasonCount struct {
		reason FailureReason
		count  int
	}
	counts := make([]reasonCount, 0, len(summary.FailureCounts))
	for reason, count := range summary.FailureCounts {
		counts = append(counts, reasonCount{reason: reason, count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].count != counts[j].count {
			return counts[i].count > counts[j].count
		}
		return counts[i].reason < counts[j].reason
	})

	maxPrint := 6
	if maxPrint > len(counts) {
		maxPrint = len(counts)
	}
	for i := 0; i < maxPrint; i++ {
		logx.Infof("failure[%s]=%d", counts[i].reason, counts[i].count)
	}
}

func writeAllConfigs(cands []parser.Candidate) {
	links := make([]string, 0, len(cands))
	for _, c := range cands {
		links = append(links, c.Raw)
	}
	if err := writeLinkBucket(filepath.Join("output", "all_configs.txt"), "All fetched candidates", links); err != nil {
		logx.Warnf("failed to write all_configs.txt: %v", err)
	}
}

func writeCandidateBuckets(working []Ranked, probable []ProbableCandidate) error {
	workingLinks := make([]string, 0, len(working))
	for _, c := range working {
		workingLinks = append(workingLinks, c.Raw)
	}

	probableLinks := make([]string, 0, len(probable))
	for _, c := range probable {
		probableLinks = append(probableLinks, c.Raw)
	}

	if err := writeLinkBucket(filepath.Join("output", "working_configs.txt"), "Working candidates", workingLinks); err != nil {
		return err
	}
	return writeLinkBucket(filepath.Join("output", "probable_configs.txt"), "Probable candidates", probableLinks)
}

func rankedFromProbable(probable []ProbableCandidate) []Ranked {
	out := make([]Ranked, 0, len(probable))
	for _, p := range probable {
		out = append(out, Ranked{
			Raw:         p.Raw,
			Scheme:      p.Scheme,
			Name:        p.Name,
			Successes:   p.Successes,
			TargetCount: p.TargetCount,
			AvgLatency:  p.AvgLatency,
			BannedOK:    false,
			Score:       scoreCandidate(p.Successes, p.AvgLatency),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Successes != out[j].Successes {
			return out[i].Successes > out[j].Successes
		}
		if out[i].AvgLatency != out[j].AvgLatency {
			return out[i].AvgLatency < out[j].AvgLatency
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func shouldPromoteLatencyProbable(cfg config.AppConfig, latency time.Duration) bool {
	if latency <= 0 {
		return false
	}
	threshold := cfg.ProbableLatencyThreshold
	if threshold <= 0 {
		return false
	}
	return latency <= threshold
}

func writeLinkBucket(path, title string, rawLinks []string) error {
	links := uniqueSortedLinks(rawLinks)
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("# generated_at: ")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("# total: %d\n\n", len(links)))
	for _, link := range links {
		b.WriteString(link)
		b.WriteString("\n")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func uniqueSortedLinks(links []string) []string {
	seen := make(map[string]struct{}, len(links))
	out := make([]string, 0, len(links))
	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		out = append(out, link)
	}
	sort.Strings(out)
	return out
}

func selectSourcesForRound(all []string, health *sources.HealthTracker, now time.Time) []string {
	if health == nil {
		return append([]string(nil), all...)
	}
	out := make([]string, 0, len(all))
	type skippedSource struct {
		source string
		until  time.Time
	}
	skipped := make([]skippedSource, 0, len(all))

	for _, src := range all {
		cooling, until := health.IsCoolingDown(src, now)
		if cooling {
			skipped = append(skipped, skippedSource{source: src, until: until})
			continue
		}
		out = append(out, src)
	}

	if len(skipped) > 0 {
		logx.Warnf("skipping %d sources in cooldown this round", len(skipped))
		maxPrint := 5
		if maxPrint > len(skipped) {
			maxPrint = len(skipped)
		}
		for i := 0; i < maxPrint; i++ {
			remaining := time.Until(skipped[i].until).Round(time.Second)
			logx.Warnf("cooldown source %s (remaining=%s)", skipped[i].source, remaining)
		}
		if len(skipped) > maxPrint {
			logx.Warnf("...and %d more sources in cooldown", len(skipped)-maxPrint)
		}
	}

	if len(out) == 0 {
		logx.Warnf("all sources are in cooldown; bypassing cooldown for this round")
		return append([]string(nil), all...)
	}
	return out
}

func printSourceHealthSummary(stats []sources.HealthStat) {
	if len(stats) == 0 {
		return
	}

	now := time.Now()
	attempted := 0
	totalSuccess := 0
	totalFailures := 0
	totalYield := 0
	coolingDown := 0
	worst := make([]sources.HealthStat, 0, len(stats))

	for _, stat := range stats {
		if stat.Successes+stat.Failures > 0 {
			attempted++
		}
		totalSuccess += stat.Successes
		totalFailures += stat.Failures
		totalYield += stat.TotalYield
		if !stat.CooldownUntil.IsZero() && now.Before(stat.CooldownUntil) {
			coolingDown++
		}
		if stat.Failures > 0 {
			worst = append(worst, stat)
		}
	}

	logx.Infof(
		"source health: attempted=%d/%d | successes=%d | failures=%d | yield=%d | cooling_down=%d",
		attempted,
		len(stats),
		totalSuccess,
		totalFailures,
		totalYield,
		coolingDown,
	)

	sort.Slice(worst, func(i, j int) bool {
		if worst[i].ConsecutiveFailures != worst[j].ConsecutiveFailures {
			return worst[i].ConsecutiveFailures > worst[j].ConsecutiveFailures
		}
		if worst[i].Failures != worst[j].Failures {
			return worst[i].Failures > worst[j].Failures
		}
		return worst[i].Source < worst[j].Source
	})

	maxPrint := 3
	if maxPrint > len(worst) {
		maxPrint = len(worst)
	}
	for i := 0; i < maxPrint; i++ {
		stat := worst[i]
		lastOK := "never"
		if !stat.LastOK.IsZero() {
			lastOK = fmt.Sprintf("%s ago", now.Sub(stat.LastOK).Round(time.Second))
		}
		cooldown := "none"
		if !stat.CooldownUntil.IsZero() && now.Before(stat.CooldownUntil) {
			cooldown = time.Until(stat.CooldownUntil).Round(time.Second).String()
		}
		logx.Infof(
			"source health issue %d: %s | ok=%d fail=%d streak=%d yield=%d last_ok=%s cooldown=%s",
			i+1,
			stat.Source,
			stat.Successes,
			stat.Failures,
			stat.ConsecutiveFailures,
			stat.TotalYield,
			lastOK,
			cooldown,
		)
	}
}

func writeSourceReport(stats []sources.HealthStat) error {
	type sourceEntry struct {
		Source              string `json:"source"`
		Successes           int    `json:"successes"`
		Failures            int    `json:"failures"`
		ConsecutiveFailures int    `json:"consecutive_failures"`
		TotalYield          int    `json:"total_yield"`
		LastYield           int    `json:"last_yield"`
		LastOK              string `json:"last_ok,omitempty"`
		LastSeen            string `json:"last_seen,omitempty"`
		CooldownUntil       string `json:"cooldown_until,omitempty"`
	}
	type report struct {
		GeneratedAt      string        `json:"generated_at"`
		SourceCount      int           `json:"source_count"`
		AttemptedSources int           `json:"attempted_sources"`
		CoolingDown      int           `json:"cooling_down"`
		TotalSuccesses   int           `json:"total_successes"`
		TotalFailures    int           `json:"total_failures"`
		TotalYield       int           `json:"total_yield"`
		Sources          []sourceEntry `json:"sources"`
	}

	now := time.Now()
	rep := report{
		GeneratedAt: now.Format(time.RFC3339),
		SourceCount: len(stats),
		Sources:     make([]sourceEntry, 0, len(stats)),
	}
	for _, stat := range stats {
		if stat.Successes+stat.Failures > 0 {
			rep.AttemptedSources++
		}
		if !stat.CooldownUntil.IsZero() && now.Before(stat.CooldownUntil) {
			rep.CoolingDown++
		}
		rep.TotalSuccesses += stat.Successes
		rep.TotalFailures += stat.Failures
		rep.TotalYield += stat.TotalYield
		rep.Sources = append(rep.Sources, sourceEntry{
			Source:              stat.Source,
			Successes:           stat.Successes,
			Failures:            stat.Failures,
			ConsecutiveFailures: stat.ConsecutiveFailures,
			TotalYield:          stat.TotalYield,
			LastYield:           stat.LastYield,
			LastOK:              formatTime(stat.LastOK),
			LastSeen:            formatTime(stat.LastSeen),
			CooldownUntil:       formatTime(stat.CooldownUntil),
		})
	}

	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll("output", 0o755); err != nil {
		return err
	}
	finalPath := filepath.Join("output", "source_report.json")
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, finalPath)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func adapterFromConfig(cfg config.AppConfig) (engines.Adapter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Engine)) {
	case "", "xray":
		return engines.NewXrayAdapter(), nil
	case "singbox", "sing-box":
		return engines.NewSingboxAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported engine %q (supported: xray, singbox)", cfg.Engine)
	}
}

func binaryForAdapter(cfg config.AppConfig, adapter engines.Adapter) string {
	name := "xray"
	if adapter != nil {
		name = strings.ToLower(strings.TrimSpace(adapter.Name()))
	}
	switch name {
	case "singbox", "sing-box":
		return cfg.SingboxBin
	default:
		return cfg.XRAYBin
	}
}

func scoreCandidate(successes int, avgLatency time.Duration) float64 {
	latMs := avgLatency.Milliseconds()
	if latMs < 1 {
		latMs = 1
	}
	return float64(successes*1000) - float64(latMs)
}
