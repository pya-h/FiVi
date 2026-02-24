package engine

import (
	"context"
	"testing"
	"time"

	"fivi/internal/config"
	"fivi/internal/parser"
)

func TestEmitStageEventDoesNotBlockWhenChannelIsFull(t *testing.T) {
	events := make(chan WorkerEvent, 1)
	events <- WorkerEvent{Stage: WorkerStageDone, Status: WorkerStatusFailed}

	done := make(chan struct{})
	go func() {
		emitStageEvent(events, 1, "candidate", WorkerStageProbe, WorkerStatusRunning, 1, "", nil, time.Now())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("emitStageEvent blocked on full channel")
	}
}

func TestTestOneEmitsParseFailureEvents(t *testing.T) {
	events := make(chan WorkerEvent, 16)
	cfg := config.AppConfig{
		CandidateTimeout: 2 * time.Second,
	}

	outcome := testOne(context.Background(), cfg, parser.Candidate{
		Raw:    "not-a-valid-link",
		Scheme: "vmess",
	}, 3, events)

	if outcome.Reason != FailureReasonParseCandidate {
		t.Fatalf("unexpected outcome reason: got=%s want=%s", outcome.Reason, FailureReasonParseCandidate)
	}

	got := drainWorkerEvents(events)
	if !hasWorkerEvent(got, WorkerStageParse, WorkerStatusRunning, "") {
		t.Fatalf("missing parse running event")
	}
	if !hasWorkerEvent(got, WorkerStageParse, WorkerStatusFailed, FailureReasonParseCandidate) {
		t.Fatalf("missing parse failed event")
	}
	if !hasWorkerEvent(got, WorkerStageDone, WorkerStatusFailed, FailureReasonParseCandidate) {
		t.Fatalf("missing done failed event")
	}
}

func drainWorkerEvents(events <-chan WorkerEvent) []WorkerEvent {
	out := make([]WorkerEvent, 0, 8)
	for {
		select {
		case event := <-events:
			out = append(out, event)
		default:
			return out
		}
	}
}

func hasWorkerEvent(events []WorkerEvent, stage WorkerStage, status WorkerStatus, reason FailureReason) bool {
	for _, event := range events {
		if event.Stage != stage {
			continue
		}
		if event.Status != status {
			continue
		}
		if reason != "" && event.Reason != reason {
			continue
		}
		return true
	}
	return false
}
