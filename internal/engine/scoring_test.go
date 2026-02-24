package engine

import (
	"testing"
	"time"
)

func TestScoreCandidatePrefersHigherSuccesses(t *testing.T) {
	low := scoreCandidate(1, 10*time.Millisecond)
	high := scoreCandidate(2, 900*time.Millisecond)
	if high <= low {
		t.Fatalf("expected higher successes to win: high=%f low=%f", high, low)
	}
}

func TestScoreCandidatePenalizesLatency(t *testing.T) {
	fast := scoreCandidate(2, 30*time.Millisecond)
	slow := scoreCandidate(2, 300*time.Millisecond)
	if fast <= slow {
		t.Fatalf("expected lower latency to have higher score: fast=%f slow=%f", fast, slow)
	}
}

func TestScoreCandidateLatencyFloor(t *testing.T) {
	zero := scoreCandidate(1, 0)
	oneMs := scoreCandidate(1, 1*time.Millisecond)
	if zero != oneMs {
		t.Fatalf("expected zero-latency floor to match 1ms score: zero=%f one=%f", zero, oneMs)
	}
}
