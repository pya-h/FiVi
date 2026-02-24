package sources

import (
	"errors"
	"testing"
	"time"
)

func TestHealthTrackerObserveAndSnapshot(t *testing.T) {
	tracker := NewHealthTracker([]string{"s1"})
	now := time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC)

	tracker.Observe("s1", 10, nil, now)
	snap := tracker.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 source stat, got %d", len(snap))
	}
	stat := snap[0]
	if stat.Successes != 1 || stat.Failures != 0 {
		t.Fatalf("unexpected stat counters: %+v", stat)
	}
	if stat.TotalYield != 10 || stat.LastYield != 10 {
		t.Fatalf("unexpected yield counters: %+v", stat)
	}
	if stat.ConsecutiveFailures != 0 {
		t.Fatalf("unexpected failure streak: %d", stat.ConsecutiveFailures)
	}
}

func TestHealthTrackerCooldownAfterRepeatedFailures(t *testing.T) {
	tracker := NewHealthTracker([]string{"s1"})
	errSource := errors.New("source down")
	base := time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC)

	tracker.Observe("s1", 0, errSource, base)
	tracker.Observe("s1", 0, errSource, base.Add(1*time.Second))
	tracker.Observe("s1", 0, errSource, base.Add(2*time.Second))

	cooling, until := tracker.IsCoolingDown("s1", base.Add(3*time.Second))
	if !cooling {
		t.Fatalf("expected source to be cooling down")
	}
	if !until.After(base.Add(3 * time.Second)) {
		t.Fatalf("unexpected cooldown timestamp: %s", until)
	}

	tracker.Observe("s1", 7, nil, base.Add(5*time.Minute))
	cooling, _ = tracker.IsCoolingDown("s1", base.Add(5*time.Minute))
	if cooling {
		t.Fatalf("expected cooldown to reset on success")
	}
}
