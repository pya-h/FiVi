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

func TestRunLogProgressReporterLogsWarmFailoverPhase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan engine.WorkerEvent)
	status := make(chan engine.StatusUpdate, 1)
	logs := make([]string, 0, 2)
	runDone := make(chan struct{})

	go func() {
		runLogProgressReporter(ctx, events, status, 100*time.Millisecond, nil, func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		})
		close(runDone)
	}()

	status <- engine.StatusUpdate{Phase: engine.PhaseWarmFailover}
	close(status)
	close(events)

	select {
	case <-runDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("reporter did not exit")
	}

	found := false
	for _, line := range logs {
		if strings.Contains(line, "warm failover") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warm failover log line, got=%v", logs)
	}
}

func TestRunLogProgressReporterMirrorsInsightRollups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan engine.WorkerEvent, 2)
	status := make(chan engine.StatusUpdate, 3)
	logs := make([]string, 0, 8)
	runDone := make(chan struct{})

	go func() {
		runLogProgressReporter(ctx, events, status, 10*time.Millisecond, nil, func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		})
		close(runDone)
	}()

	status <- engine.StatusUpdate{Phase: engine.PhaseDiscovering, SourceTop: []string{"src-a ok=1 fail=4 yield=7 cd=20s"}}
	status <- engine.StatusUpdate{Phase: engine.PhaseMonitoring, FailureTop: []string{"probe_failed=2"}, Ping: 120 * time.Millisecond}
	events <- engine.WorkerEvent{WorkerID: 1, Stage: engine.WorkerStageDone, Status: engine.WorkerStatusFailed}
	time.Sleep(30 * time.Millisecond)
	close(status)
	close(events)

	select {
	case <-runDone:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("reporter did not exit")
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "source-health(top):") {
		t.Fatalf("expected source health insight log, got=%v", logs)
	}
	if !strings.Contains(joined, "why-failed(last checks):") {
		t.Fatalf("expected failure rollup insight log, got=%v", logs)
	}
	if !strings.Contains(joined, "srcTop=[") || !strings.Contains(joined, "failTop=[") {
		t.Fatalf("expected snapshot to mirror insight payloads, got=%v", logs)
	}
}
