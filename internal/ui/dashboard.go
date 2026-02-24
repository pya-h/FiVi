package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"fivi/internal/engine"
)

type workerEventMsg engine.WorkerEvent

type streamClosedMsg struct{}

type dashboardContextDoneMsg struct{}

type clockTickMsg struct{}

type workerRow struct {
	WorkerID  int
	Candidate string
	Stage     engine.WorkerStage
	Status    engine.WorkerStatus
	Attempt   int
	Reason    engine.FailureReason
	ElapsedMs int64
	UpdatedAt time.Time
}

type dashboardModel struct {
	ctx           context.Context
	events        <-chan engine.WorkerEvent
	startedAt     time.Time
	lastEventAt   time.Time
	currentNode   string
	currentNodeAt time.Time
	width         int
	height        int
	closed        bool

	progress progressState
	workers  map[int]workerRow
}

func RunDashboard(ctx context.Context, events <-chan engine.WorkerEvent) error {
	program := tea.NewProgram(
		newDashboardModel(ctx, events),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}

func newDashboardModel(ctx context.Context, events <-chan engine.WorkerEvent) dashboardModel {
	if ctx == nil {
		ctx = context.Background()
	}
	return dashboardModel{
		ctx:       ctx,
		events:    events,
		startedAt: time.Now(),
		progress:  newProgressState(),
		workers:   make(map[int]workerRow),
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(waitWorkerEventCmd(m.ctx, m.events), dashboardTickCmd())
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case workerEventMsg:
		event := engine.WorkerEvent(msg)
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		m.lastEventAt = event.Timestamp
		m.progress.observe(event)
		m.workers[event.WorkerID] = workerRow{
			WorkerID:  event.WorkerID,
			Candidate: compactCandidate(event.Candidate),
			Stage:     event.Stage,
			Status:    event.Status,
			Attempt:   event.Attempt,
			Reason:    event.Reason,
			ElapsedMs: event.ElapsedMs,
			UpdatedAt: event.Timestamp,
		}
		if event.Stage == engine.WorkerStageDone && event.Status == engine.WorkerStatusOK {
			m.currentNode = compactCandidate(event.Candidate)
			m.currentNodeAt = event.Timestamp
		}
		return m, waitWorkerEventCmd(m.ctx, m.events)
	case streamClosedMsg:
		m.closed = true
		return m, tea.Quit
	case dashboardContextDoneMsg:
		m.closed = true
		return m, tea.Quit
	case clockTickMsg:
		if m.closed {
			return m, nil
		}
		return m, dashboardTickCmd()
	}
	return m, nil
}

func (m dashboardModel) View() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	lastEvent := "-"
	if !m.lastEventAt.IsZero() {
		lastEvent = time.Since(m.lastEventAt).Round(time.Second).String() + " ago"
	}
	node := "-"
	if m.currentNode != "" {
		node = m.currentNode
		if !m.currentNodeAt.IsZero() {
			node += fmt.Sprintf(" (%s ago)", time.Since(m.currentNodeAt).Round(time.Second))
		}
	}

	lines := []string{
		headerStyle.Render("FiVi Dashboard"),
		fmt.Sprintf("%s %s | %s %s | %s %s",
			labelStyle.Render("uptime:"),
			time.Since(m.startedAt).Round(time.Second),
			labelStyle.Render("last_event:"),
			lastEvent,
			labelStyle.Render("stream:"),
			streamState(m.closed),
		),
		fmt.Sprintf("%s processed=%d working=%d probable=%d failed=%d running=%d",
			labelStyle.Render("progress:"),
			m.progress.processed,
			m.progress.working,
			m.progress.probable,
			m.progress.failed,
			len(m.progress.active),
		),
		fmt.Sprintf("%s %s", labelStyle.Render("stages:"), formatActiveStages(m.progress.active)),
		fmt.Sprintf("%s %s", labelStyle.Render("current_node:"), node),
		"",
		renderWorkerTable(m.workers, tableRowLimit(m.height)),
		"",
		labelStyle.Render("Keys: q to quit"),
	}
	return strings.Join(lines, "\n")
}

func waitWorkerEventCmd(ctx context.Context, events <-chan engine.WorkerEvent) tea.Cmd {
	return func() tea.Msg {
		if events == nil {
			<-ctx.Done()
			return dashboardContextDoneMsg{}
		}
		select {
		case <-ctx.Done():
			return dashboardContextDoneMsg{}
		case event, ok := <-events:
			if !ok {
				return streamClosedMsg{}
			}
			return workerEventMsg(event)
		}
	}
}

func dashboardTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return clockTickMsg{}
	})
}

func streamState(closed bool) string {
	if closed {
		return "closed"
	}
	return "open"
}

func tableRowLimit(height int) int {
	if height <= 0 {
		return 10
	}
	limit := height - 11
	if limit < 5 {
		return 5
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func renderWorkerTable(workers map[int]workerRow, limit int) string {
	if len(workers) == 0 {
		return "workers: waiting for events..."
	}
	if limit < 1 {
		limit = 1
	}

	rows := make([]workerRow, 0, len(workers))
	for _, row := range workers {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].WorkerID < rows[j].WorkerID
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}

	var b strings.Builder
	b.WriteString("workers:\n")
	b.WriteString(fmt.Sprintf("%-7s %-7s %-9s %-7s %-9s %-15s %s\n", "id", "stage", "status", "attempt", "elapsed", "reason", "candidate"))
	for _, row := range rows {
		reason := "-"
		if row.Reason != "" {
			reason = string(row.Reason)
		}
		b.WriteString(fmt.Sprintf("%-7d %-7s %-9s %-7d %-9d %-15s %s\n",
			row.WorkerID,
			row.Stage,
			row.Status,
			row.Attempt,
			row.ElapsedMs,
			reason,
			row.Candidate,
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func compactCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	if len(raw) <= 72 {
		return raw
	}
	return raw[:38] + "..." + raw[len(raw)-24:]
}
