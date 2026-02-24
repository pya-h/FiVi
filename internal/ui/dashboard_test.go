package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"fivi/internal/engine"
)

func TestDashboardUpdateTracksProgressAndCurrentNode(t *testing.T) {
	model := newDashboardModel(context.Background(), nil)
	now := time.Now()

	updated, _ := model.Update(workerEventMsg(engine.WorkerEvent{
		WorkerID:  4,
		Candidate: "vmess://example",
		Stage:     engine.WorkerStageDone,
		Status:    engine.WorkerStatusOK,
		Attempt:   1,
		Timestamp: now,
	}))
	got := updated.(dashboardModel)

	if got.progress.processed != 1 {
		t.Fatalf("processed=%d want=1", got.progress.processed)
	}
	if got.progress.working != 1 {
		t.Fatalf("working=%d want=1", got.progress.working)
	}
	if got.currentNode == "" {
		t.Fatalf("expected current node to be set")
	}
	row, ok := got.workers[4]
	if !ok {
		t.Fatalf("missing worker row")
	}
	if row.Stage != engine.WorkerStageDone || row.Status != engine.WorkerStatusOK {
		t.Fatalf("unexpected worker row stage/status: %+v", row)
	}
}

func TestDashboardViewIncludesWorkerTable(t *testing.T) {
	model := newDashboardModel(context.Background(), nil)
	model.workers[1] = workerRow{
		WorkerID:  1,
		Stage:     engine.WorkerStageProbe,
		Status:    engine.WorkerStatusRunning,
		Attempt:   2,
		ElapsedMs: 123,
		Candidate: "vless://example",
		UpdatedAt: time.Now(),
	}

	view := model.View()
	if !strings.Contains(view, "FiVi Dashboard") {
		t.Fatalf("missing dashboard title in view")
	}
	if !strings.Contains(view, "workers:") {
		t.Fatalf("missing workers table in view")
	}
	if !strings.Contains(view, "vless://example") {
		t.Fatalf("missing candidate in workers table")
	}
}

func TestDashboardQuitKey(t *testing.T) {
	model := newDashboardModel(context.Background(), nil)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
}
