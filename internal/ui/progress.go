package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fivi/internal/engine"
	"fivi/internal/logx"
)

type progressLogger func(format string, args ...any)

func RunLogProgressReporter(ctx context.Context, events <-chan engine.WorkerEvent, interval time.Duration) {
	runLogProgressReporter(ctx, events, interval, logx.Infof)
}

func runLogProgressReporter(ctx context.Context, events <-chan engine.WorkerEvent, interval time.Duration, logger progressLogger) {
	if events == nil || logger == nil {
		return
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	state := newProgressState()
	for {
		select {
		case <-ctx.Done():
			if state.eventCount > 0 {
				logger("progress snapshot: %s", state.snapshot())
			}
			return
		case event, ok := <-events:
			if !ok {
				if state.eventCount > 0 {
					logger("progress snapshot: %s", state.snapshot())
				}
				return
			}
			state.observe(event)
		case <-ticker.C:
			if state.eventCount == 0 {
				continue
			}
			logger("progress snapshot: %s", state.snapshot())
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
