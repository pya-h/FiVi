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
	model := newDashboardModel(context.Background(), nil, nil)
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
	row, ok := got.workers[4]
	if !ok {
		t.Fatalf("missing worker row")
	}
	if row.Stage != engine.WorkerStageDone || row.Status != engine.WorkerStatusOK {
		t.Fatalf("unexpected worker row stage/status: %+v", row)
	}
}

func TestDashboardViewIncludesWorkerTable(t *testing.T) {
	model := newDashboardModel(context.Background(), nil, nil)
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
	if !strings.Contains(view, "FiVi") {
		t.Fatalf("missing dashboard title in view")
	}
	if !strings.Contains(view, "vless://example") {
		t.Fatalf("missing candidate in workers table")
	}
}

func TestDashboardQuitKey(t *testing.T) {
	model := newDashboardModel(context.Background(), nil, nil)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
}

func TestDashboardPhaseTransition(t *testing.T) {
	model := newDashboardModel(context.Background(), nil, nil)
	if model.phase != engine.PhaseDiscovering {
		t.Fatalf("initial phase=%s want=discovering", model.phase)
	}

	updated, _ := model.Update(statusUpdateMsg(engine.StatusUpdate{
		Phase:      engine.PhaseMonitoring,
		NodeName:   "test-node",
		Protocol:   "vmess",
		SocksAddr:  "127.0.0.1:10808",
		HTTPAddr:   "127.0.0.1:10809",
		Ping:       50 * time.Millisecond,
		ProbeOK:    3,
		ProbeTotal: 3,
	}))
	got := updated.(dashboardModel)
	if got.phase != engine.PhaseMonitoring {
		t.Fatalf("phase=%s want=monitoring", got.phase)
	}
	if got.connectedNode != "test-node" {
		t.Fatalf("connectedNode=%s want=test-node", got.connectedNode)
	}
	if got.socksAddr != "127.0.0.1:10808" {
		t.Fatalf("socksAddr=%s want=127.0.0.1:10808", got.socksAddr)
	}
	if len(got.healthHistory) != 1 {
		t.Fatalf("healthHistory len=%d want=1", len(got.healthHistory))
	}
}

func TestDashboardMonitoringView(t *testing.T) {
	model := newDashboardModel(context.Background(), nil, nil)
	model.phase = engine.PhaseMonitoring
	model.connectedNode = "my-node"
	model.connProtocol = "vless"
	model.connectedAt = time.Now()
	model.socksAddr = "127.0.0.1:10808"
	model.httpAddr = "127.0.0.1:10809"
	model.lastPing = 42 * time.Millisecond
	model.lastProbeOK = 2
	model.lastProbeTotal = 2

	view := model.View()
	if !strings.Contains(view, "my-node") {
		t.Fatalf("monitoring view missing node name")
	}
	if !strings.Contains(view, "vless") {
		t.Fatalf("monitoring view missing protocol")
	}
	if !strings.Contains(view, "127.0.0.1:10808") {
		t.Fatalf("monitoring view missing socks addr")
	}
}

func TestRenderProgressBar(t *testing.T) {
	bar := renderProgressBar(50, 100, 20)
	if !strings.Contains(bar, "50%") {
		t.Fatalf("progress bar missing percentage: %s", bar)
	}
	if !strings.Contains(bar, "50/100") {
		t.Fatalf("progress bar missing count: %s", bar)
	}
}

func TestFormatElapsed(t *testing.T) {
	if got := formatElapsed(500); got != "500ms" {
		t.Fatalf("formatElapsed(500)=%s want=500ms", got)
	}
	if got := formatElapsed(2500); got != "2.5s" {
		t.Fatalf("formatElapsed(2500)=%s want=2.5s", got)
	}
}
