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
