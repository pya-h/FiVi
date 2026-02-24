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
	"strings"
	"sync"
	"time"

	"fivi/internal/config"
	"fivi/internal/fetch"
	"fivi/internal/logx"
	"fivi/internal/parser"
	"fivi/internal/probe"
	"fivi/internal/sources"
	"fivi/internal/xray"
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

func Run(ctx context.Context, cfg config.AppConfig) error {
	return RunWithEvents(ctx, cfg, nil)
}

func RunWithEvents(ctx context.Context, cfg config.AppConfig, events chan<- WorkerEvent) error {
	if err := xray.EnsureBinary(cfg.XRAYBin); err != nil {
		return err
	}
	health := sources.NewHealthTracker(cfg.BootstrapSources)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ranked, err := discoverAndRank(ctx, cfg, health, events)
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

func discoverAndRank(ctx context.Context, cfg config.AppConfig, health *sources.HealthTracker, events chan<- WorkerEvent) ([]Ranked, error) {
	roundSources := selectSourcesForRound(cfg.BootstrapSources, health, time.Now())
	logx.Infof("fetching subscriptions from %d/%d sources", len(roundSources), len(cfg.BootstrapSources))

	sourceResults := fetch.FetchAll(ctx, roundSources, 20*time.Second)
	all := make([]string, 0, 4096)
	for _, r := range sourceResults {
		if health != nil {
			health.Observe(r.Source, len(r.Links), r.Err, time.Now())
		}
		if r.Err != nil {
			logx.Warnf("source failed %s: %v", r.Source, r.Err)
			continue
		}
		logx.Infof("source ok %s (%d links)", r.Source, len(r.Links))
		all = append(all, r.Links...)
	}
	if health != nil {
		stats := health.Snapshot()
		printSourceHealthSummary(stats)
		if err := writeSourceReport(stats); err != nil {
			logx.Warnf("failed to write source report: %v", err)
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no links fetched")
	}

	cands := parser.Normalize(all, cfg.OnlySchemes, cfg.MaxCandidates)
	if len(cands) == 0 {
		return nil, fmt.Errorf("no candidates after normalize")
	}
	logx.Infof("testing %d candidates with %d workers", len(cands), cfg.Workers)

	validation := validateCandidates(ctx, cfg, cands, events)
	if len(validation.Working) == 0 {
		if err := writeCandidateBuckets(nil, validation.Probable); err != nil {
			logx.Warnf("failed to write candidate buckets: %v", err)
		}
		return nil, nil
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
	return ranked, nil
}

func validateCandidates(ctx context.Context, cfg config.AppConfig, cands []parser.Candidate, events chan<- WorkerEvent) validationSummary {
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
					outcome := testOne(ctx, cfg, c, workerID, events)
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

func testOne(ctx context.Context, cfg config.AppConfig, c parser.Candidate, workerID int, events chan<- WorkerEvent) candidateOutcome {
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
		socksAddr, httpAddr, err := reserveLocalAddrs()
		if err != nil {
			emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusFailed, attempt, FailureReasonReserveAddr, err, parseStarted)
			emitDone(WorkerStatusFailed, FailureReasonReserveAddr, err, attempt)
			return candidateOutcome{Raw: c.Raw, Reason: FailureReasonReserveAddr}
		}
		configJSON, parsed, err := xray.BuildRuntimeConfigFromURI(c.Raw, socksAddr, httpAddr)
		if err != nil {
			emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusFailed, attempt, FailureReasonParseCandidate, err, parseStarted)
			emitDone(WorkerStatusFailed, FailureReasonParseCandidate, err, attempt)
			return candidateOutcome{Raw: c.Raw, Reason: FailureReasonParseCandidate}
		}
		emitStageEvent(events, workerID, c.Raw, WorkerStageParse, WorkerStatusOK, attempt, "", nil, parseStarted)

		spawnStarted := time.Now()
		emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusRunning, attempt, "", nil, spawnStarted)
		inst, err := xray.Start(candidateCtx, cfg.XRAYBin, configJSON, parsed.Name)
		if err != nil {
			lastReason = FailureReasonEngineStart
			emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusFailed, attempt, FailureReasonEngineStart, err, spawnStarted)
			continue
		}
		if err := waitForProxyReady(candidateCtx, socksAddr, httpAddr, cfg.StartupTimeout); err != nil {
			inst.Stop()
			lastReason = FailureReasonEngineNotReady
			emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusFailed, attempt, FailureReasonEngineNotReady, err, spawnStarted)
			continue
		}
		emitStageEvent(events, workerID, c.Raw, WorkerStageSpawn, WorkerStatusOK, attempt, "", nil, spawnStarted)

		probeStarted := time.Now()
		emitStageEvent(events, workerID, c.Raw, WorkerStageProbe, WorkerStatusRunning, attempt, "", nil, probeStarted)
		res := probe.CheckThroughSOCKS(candidateCtx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
		inst.Stop()
		if isProbeQualified(cfg, res) {
			latMs := float64(max(int(res.AvgLatency.Milliseconds()), 1))
			score := float64(res.Successes*1000) - latMs
			ranked := Ranked{
				Raw:         c.Raw,
				Scheme:      c.Scheme,
				Name:        parsed.Name,
				Successes:   res.Successes,
				TargetCount: res.TargetCount,
				AvgLatency:  res.AvgLatency,
				BannedOK:    hasRequiredBannedTargetsOK(res.Targets, cfg.Targets),
				Score:       score,
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
	// Keep a sane fallback if flag/env wiring changes.
	return 24 * time.Second
}

func waitForProxyReady(ctx context.Context, socksAddr, httpAddr string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(120 * time.Millisecond)
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
	conn, err := net.DialTimeout("tcp", addr, 160*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func connectAndMonitor(ctx context.Context, cfg config.AppConfig, ranked []Ranked) error {
	for i := 0; i < len(ranked); i++ {
		cand := ranked[i]
		inst, initialProbe, initialSpeed, err := startCandidate(ctx, cfg, cand.Raw, cfg.ListenSocksAddr, cfg.ListenHTTPAddr)
		if err != nil {
			logx.Warnf("connect failed for %s: %v", displayName(cand), err)
			continue
		}

		connectedAt := time.Now()
		printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, inst.PID(), connectedAt, initialProbe, initialSpeed, 0)
		monitorErr := monitorConnection(ctx, cfg, cand, connectedAt)
		inst.Stop()
		if monitorErr == nil || errors.Is(monitorErr, context.Canceled) {
			return monitorErr
		}
		logx.Warnf("connection degraded for %s: %v", displayName(cand), monitorErr)
	}
	return fmt.Errorf("exhausted %d ranked nodes", len(ranked))
}

func monitorConnection(ctx context.Context, cfg config.AppConfig, cand Ranked, connectedAt time.Time) error {
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
			res := probe.CheckThroughSOCKS(ctx, cfg.ListenSocksAddr, cfg.Targets, cfg.ProbeTimeout)
			if isProbeQualified(cfg, res) {
				failures = 0
				speed := 0.0
				if tick%3 == 0 {
					s, err := probe.MeasureDownloadSpeedMbps(ctx, cfg.ListenSocksAddr, cfg.SpeedTarget, cfg.ProbeTimeout, cfg.SpeedTestBytes)
					if err == nil {
						speed = s
					}
				}
				printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, 0, connectedAt, res, speed, failures)
				continue
			}
			failures++
			printConnectionStatus(cand, cfg.ListenSocksAddr, cfg.ListenHTTPAddr, 0, connectedAt, res, 0, failures)
			if failures >= cfg.MonitorFailLimit {
				return fmt.Errorf("health checks failed %d times", failures)
			}
		}
	}
}

func startCandidate(ctx context.Context, cfg config.AppConfig, rawURI, socksAddr, httpAddr string) (*xray.Instance, probe.Result, float64, error) {
	cfgJSON, _, err := xray.BuildRuntimeConfigFromURI(rawURI, socksAddr, httpAddr)
	if err != nil {
		return nil, probe.Result{}, 0, err
	}
	inst, err := xray.Start(ctx, cfg.XRAYBin, cfgJSON, "active")
	if err != nil {
		return nil, probe.Result{}, 0, err
	}
	if err := waitForProxyReady(ctx, socksAddr, httpAddr, cfg.StartupTimeout); err != nil {
		inst.Stop()
		return nil, probe.Result{}, 0, err
	}
	res := probe.CheckThroughSOCKS(ctx, socksAddr, cfg.Targets, cfg.ProbeTimeout)
	if !isProbeQualified(cfg, res) {
		inst.Stop()
		return nil, probe.Result{}, 0, fmt.Errorf("initial probe failed success=%d/%d", res.Successes, len(cfg.Targets))
	}
	speed, _ := probe.MeasureDownloadSpeedMbps(ctx, socksAddr, cfg.SpeedTarget, cfg.ProbeTimeout, cfg.SpeedTestBytes)
	return inst, res, speed, nil
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
	return hasRequiredBannedTargetsOK(res.Targets, cfg.Targets)
}

func sleepWithCancel(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
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

func hasRequiredBannedTargetsOK(results []probe.TargetResult, expectedTargets []string) bool {
	requireX := hasExpected(expectedTargets, "x.com")
	requireY := hasExpected(expectedTargets, "youtube.com")
	if !requireX && !requireY {
		return true
	}
	var xOK, yOK bool
	for _, r := range results {
		if strings.Contains(r.URL, "x.com") && r.OK {
			xOK = true
		}
		if strings.Contains(r.URL, "youtube.com") && r.OK {
			yOK = true
		}
	}
	if requireX && !xOK {
		return false
	}
	if requireY && !yOK {
		return false
	}
	return true
}

func hasExpected(targets []string, token string) bool {
	for _, t := range targets {
		if strings.Contains(t, token) {
			return true
		}
	}
	return false
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
	if err := writeLinkBucket(filepath.Join("output", "probable_configs.txt"), "Probable candidates", probableLinks); err != nil {
		return err
	}
	return nil
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
