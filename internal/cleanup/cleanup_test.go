package cleanup

import (
	"testing"
)

func TestRunsInReverseOrder(t *testing.T) {
	m := New()
	var order []string
	m.Register("first", func() { order = append(order, "first") })
	m.Register("second", func() { order = append(order, "second") })
	m.Register("third", func() { order = append(order, "third") })

	m.Run()

	if len(order) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(order))
	}
	if order[0] != "third" || order[1] != "second" || order[2] != "first" {
		t.Fatalf("expected [third second first], got %v", order)
	}
}

func TestRunsOnlyOnce(t *testing.T) {
	m := New()
	count := 0
	m.Register("counter", func() { count++ })

	m.Run()
	m.Run()
	m.Run()

	if count != 1 {
		t.Fatalf("expected 1 invocation, got %d", count)
	}
}

func TestPanicRecovery(t *testing.T) {
	m := New()
	var order []string
	m.Register("before-panic", func() { order = append(order, "before") })
	m.Register("panicker", func() { panic("boom") })
	m.Register("after-panic", func() { order = append(order, "after") })

	// Should not panic.
	m.Run()

	// Both non-panicking hooks should still run (reverse order: after, panic, before).
	if len(order) != 2 {
		t.Fatalf("expected 2 hooks to run despite panic, got %d", len(order))
	}
	if order[0] != "after" || order[1] != "before" {
		t.Fatalf("expected [after before], got %v", order)
	}
}

func TestEmptyManager(t *testing.T) {
	m := New()
	m.Run() // Should not panic.
}
