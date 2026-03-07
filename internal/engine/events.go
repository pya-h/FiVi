package engine

import (
	"time"
)

type WorkerStage string

const (
	WorkerStageParse WorkerStage = "parse"
	WorkerStageSpawn WorkerStage = "spawn"
	WorkerStageProbe WorkerStage = "probe"
	WorkerStageDone  WorkerStage = "done"
)

type WorkerStatus string

const (
	WorkerStatusRunning  WorkerStatus = "running"
	WorkerStatusOK       WorkerStatus = "ok"
	WorkerStatusFailed   WorkerStatus = "failed"
	WorkerStatusProbable WorkerStatus = "probable"
)

type WorkerEvent struct {
	WorkerID  int           `json:"worker_id"`
	Candidate string        `json:"candidate"`
	Stage     WorkerStage   `json:"stage"`
	Status    WorkerStatus  `json:"status"`
	Reason    FailureReason `json:"reason,omitempty"`
	Err       string        `json:"err,omitempty"`
	Attempt   int           `json:"attempt"`
	ElapsedMs int64         `json:"elapsed_ms"`
	Timestamp time.Time     `json:"timestamp"`
}

func emitStageEvent(
	events chan<- WorkerEvent,
	workerID int,
	candidate string,
	stage WorkerStage,
	status WorkerStatus,
	attempt int,
	reason FailureReason,
	err error,
	startedAt time.Time,
) {
	if events == nil {
		return
	}
	event := WorkerEvent{
		WorkerID:  workerID,
		Candidate: candidate,
		Stage:     stage,
		Status:    status,
		Reason:    reason,
		Attempt:   attempt,
		Timestamp: time.Now(),
	}
	if err != nil {
		event.Err = err.Error()
	}
	if !startedAt.IsZero() {
		elapsed := time.Since(startedAt).Milliseconds()
		if elapsed < 0 {
			elapsed = 0
		}
		event.ElapsedMs = elapsed
	}
	select {
	case events <- event:
	default:
	}
}

// Phase represents the engine's operational phase.
type Phase string

const (
	PhaseDiscovering  Phase = "discovering"
	PhaseConnecting   Phase = "connecting"
	PhaseMonitoring   Phase = "monitoring"
	PhaseFailover     Phase = "failover"
	PhaseWarmFailover Phase = "warm_failover"
)

// StatusUpdate carries engine phase transitions and connection health data.
type StatusUpdate struct {
	Phase       Phase
	NodeName    string
	Protocol    string
	Ping        time.Duration
	Speed       float64 // Mbps, 0 if not measured
	ProbeOK     int
	ProbeTotal  int
	Failures    int // consecutive monitor failures
	TotalCands  int // total candidates in current round
	ConnectedAt time.Time
	SocksAddr   string
	HTTPAddr    string
	FailureTop  []string // top failure reasons over recent monitoring checks
	SourceTop   []string // top source health offenders during discovery
	Timestamp   time.Time
}

func emitStatus(ch chan<- StatusUpdate, update StatusUpdate) {
	if ch == nil {
		return
	}
	if update.Timestamp.IsZero() {
		update.Timestamp = time.Now()
	}
	select {
	case ch <- update:
	default:
	}
}
