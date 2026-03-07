package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fivi/internal/engine"
	"fivi/internal/logx"
	"fivi/internal/netstats"
)

type progressLogger func(format string, args ...any)

func RunLogProgressReporter(ctx context.Context, events <-chan engine.WorkerEvent, status <-chan engine.StatusUpdate, interval time.Duration, ns <-chan netstats.Stats) {
	runLogProgressReporter(ctx, events, status, interval, ns, logx.Infof)
}

func runLogProgressReporter(ctx context.Context, events <-chan engine.WorkerEvent, status <-chan engine.StatusUpdate, interval time.Duration, ns <-chan netstats.Stats, logger progressLogger) {
	if logger == nil {
		return
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	state := newProgressState()
	var lastPhase engine.Phase
	var lastNS *netstats.Stats
	for {
		select {
		case <-ctx.Done():
			if state.eventCount > 0 {
				logger("progress snapshot: %s", state.snapshot())
			}
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				if status == nil {
					if state.eventCount > 0 {
						logger("progress snapshot: %s", state.snapshot())
					}
					return
				}
				continue
			}
			state.observe(event)
		case update, ok := <-status:
			if !ok {
				status = nil
				if events == nil {
					if state.eventCount > 0 {
						logger("progress snapshot: %s", state.snapshot())
					}
					return
				}
				continue
			}
			if update.Phase != lastPhase {
				lastPhase = update.Phase
				switch update.Phase {
				case engine.PhaseDiscovering:
					logger("phase: discovering (%d candidates)", update.TotalCands)
				case engine.PhaseConnecting:
					logger("phase: connecting to %s [%s]", update.NodeName, update.Protocol)
				case engine.PhaseMonitoring:
					logger("phase: monitoring %s (socks=%s http=%s)", update.NodeName, update.SocksAddr, update.HTTPAddr)
				case engine.PhaseFailover:
					logger("phase: failover triggered")
				case engine.PhaseWarmFailover:
					logger("phase: warm failover (trying pre-validated standbys)")
				}
			}
			if update.Phase == engine.PhaseMonitoring && update.Ping > 0 {
				logger("health: ping=%s probes=%d/%d failures=%d", update.Ping.Round(time.Millisecond), update.ProbeOK, update.ProbeTotal, update.Failures)
			}
		case s, ok := <-ns:
			if !ok {
				ns = nil
				continue
			}
			lastNS = &s
		case <-ticker.C:
			if state.eventCount == 0 {
				continue
			}
			snap := state.snapshot()
			if lastNS != nil {
				snap += fmt.Sprintf(" net=[%s recv=%s send=%s]",
					lastNS.Interface,
					netstats.FormatSpeed(lastNS.RecvSpeed),
					netstats.FormatSpeed(lastNS.SendSpeed))
			}
			logger("progress snapshot: %s", snap)
		}
	}
}

type progressState struct {
	eventCount int
	processed  int
	working    int
	probable   int
	failed     int
	active     map[int]engine.WorkerStage
}

func newProgressState() progressState {
	return progressState{
		active: make(map[int]engine.WorkerStage),
	}
}

func (s *progressState) observe(event engine.WorkerEvent) {
	s.eventCount++
	switch event.Status {
	case engine.WorkerStatusRunning:
		s.active[event.WorkerID] = event.Stage
	case engine.WorkerStatusOK, engine.WorkerStatusFailed, engine.WorkerStatusProbable:
		if event.Stage != engine.WorkerStageDone {
			return
		}
		delete(s.active, event.WorkerID)
		s.processed++
		switch event.Status {
		case engine.WorkerStatusOK:
			s.working++
		case engine.WorkerStatusProbable:
			s.probable++
		case engine.WorkerStatusFailed:
			s.failed++
		}
	}
}

func (s progressState) snapshot() string {
	return fmt.Sprintf(
		"processed=%d working=%d probable=%d failed=%d running=%d stages=%s",
		s.processed,
		s.working,
		s.probable,
		s.failed,
		len(s.active),
		formatActiveStages(s.active),
	)
}

func formatActiveStages(active map[int]engine.WorkerStage) string {
	if len(active) == 0 {
		return "-"
	}

	counts := map[engine.WorkerStage]int{
		engine.WorkerStageParse: 0,
		engine.WorkerStageSpawn: 0,
		engine.WorkerStageProbe: 0,
		engine.WorkerStageDone:  0,
	}
	for _, stage := range active {
		counts[stage]++
	}

	order := []engine.WorkerStage{
		engine.WorkerStageParse,
		engine.WorkerStageSpawn,
		engine.WorkerStageProbe,
		engine.WorkerStageDone,
	}
	parts := make([]string, 0, len(order))
	for _, stage := range order {
		if counts[stage] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%d", stage, counts[stage]))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}
