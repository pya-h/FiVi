package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"fivi/internal/engine"
)

func TestProgressStateTracksDoneOutcomes(t *testing.T) {
	state := newProgressState()
	state.observe(engine.WorkerEvent{
		WorkerID: 1,
		Stage:    engine.WorkerStageParse,
		Status:   engine.WorkerStatusRunning,
	})
	state.observe(engine.WorkerEvent{
		WorkerID: 2,
		Stage:    engine.WorkerStageProbe,
		Status:   engine.WorkerStatusRunning,
	})
	state.observe(engine.WorkerEvent{
		WorkerID: 1,
		Stage:    engine.WorkerStageDone,
		Status:   engine.WorkerStatusOK,
	})
	state.observe(engine.WorkerEvent{
		WorkerID: 2,
		Stage:    engine.WorkerStageDone,
		Status:   engine.WorkerStatusFailed,
	})

	got := state.snapshot()
	wantTokens := []string{
		"processed=2",
		"working=1",
		"probable=0",
		"failed=1",
		"running=0",
		"stages=-",
	}
	for _, token := range wantTokens {
		if !strings.Contains(got, token) {
			t.Fatalf("snapshot missing %q in %q", token, got)
		}
	}
}

func TestRunLogProgressReporterEmitsSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan engine.WorkerEvent, 4)
	logs := make([]string, 0, 4)
	runDone := make(chan struct{})
	go func() {
		runLogProgressReporter(ctx, events, nil, 10*time.Millisecond, nil, func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		})
		close(runDone)
	}()

	events <- engine.WorkerEvent{
		WorkerID: 1,
		Stage:    engine.WorkerStageParse,
		Status:   engine.WorkerStatusRunning,
	}
	events <- engine.WorkerEvent{
		WorkerID: 1,
		Stage:    engine.WorkerStageDone,
		Status:   engine.WorkerStatusOK,
	}
	time.Sleep(25 * time.Millisecond)
	close(events)

	select {
	case <-runDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("reporter did not exit")
	}
	if len(logs) == 0 {
		t.Fatalf("expected at least one snapshot log")
	}
}
