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
	"fivi/internal/netstats"
)

type workerEventMsg engine.WorkerEvent

type statusUpdateMsg engine.StatusUpdate

type streamClosedMsg struct{}

type dashboardContextDoneMsg struct{}

type clockTickMsg struct{}

type netStatsMsg netstats.Stats

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

type healthCheckEntry struct {
	Timestamp  time.Time
	Ping       time.Duration
	Speed      float64
	ProbeOK    int
	ProbeTotal int
	Failures   int
}

type dashboardModel struct {
	ctx    context.Context
	events <-chan engine.WorkerEvent
	status <-chan engine.StatusUpdate
	nsChan <-chan netstats.Stats

	startedAt   time.Time
	lastEventAt time.Time
	width       int
	height      int
	closed      bool

	// Phase tracking.
	phase      engine.Phase
	totalCands int

	// Connection state (populated during connecting/monitoring).
	connectedNode  string
	connProtocol   string
	connectedAt    time.Time
	lastPing       time.Duration
	lastSpeed      float64
	lastProbeOK    int
	lastProbeTotal int
	connFailures   int
	socksAddr      string
	httpAddr       string

	// Health check history (last N checks).
	healthHistory []healthCheckEntry

	// Network statistics (optional, nil when --stats is off).
	lastNS *netstats.Stats

	progress progressState
	workers  map[int]workerRow
}

func RunDashboard(ctx context.Context, events <-chan engine.WorkerEvent, status <-chan engine.StatusUpdate, ns <-chan netstats.Stats) error {
	program := tea.NewProgram(
		newDashboardModel(ctx, events, status, ns),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}

func newDashboardModel(ctx context.Context, events <-chan engine.WorkerEvent, status <-chan engine.StatusUpdate, ns <-chan netstats.Stats) dashboardModel {
	if ctx == nil {
		ctx = context.Background()
	}
	return dashboardModel{
		ctx:           ctx,
		events:        events,
		status:        status,
		nsChan:        ns,
		startedAt:     time.Now(),
		phase:         engine.PhaseDiscovering,
		progress:      newProgressState(),
		workers:       make(map[int]workerRow),
		healthHistory: make([]healthCheckEntry, 0, 20),
	}
}

func (m dashboardModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		waitWorkerEventCmd(m.ctx, m.events),
		waitStatusUpdateCmd(m.ctx, m.status),
		dashboardTickCmd(),
	}
	if m.nsChan != nil {
		cmds = append(cmds, waitNetStatsCmd(m.ctx, m.nsChan))
	}
	return tea.Batch(cmds...)
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
		return m, waitWorkerEventCmd(m.ctx, m.events)
	case statusUpdateMsg:
		update := engine.StatusUpdate(msg)
		m.phase = update.Phase
		if update.TotalCands > 0 {
			m.totalCands = update.TotalCands
		}
		if update.NodeName != "" {
			m.connectedNode = update.NodeName
		}
		if update.Protocol != "" {
			m.connProtocol = update.Protocol
		}
		if !update.ConnectedAt.IsZero() {
			m.connectedAt = update.ConnectedAt
		}
		if update.SocksAddr != "" {
			m.socksAddr = update.SocksAddr
		}
		if update.HTTPAddr != "" {
			m.httpAddr = update.HTTPAddr
		}
		if update.Phase == engine.PhaseMonitoring {
			m.lastPing = update.Ping
			m.lastSpeed = update.Speed
			m.lastProbeOK = update.ProbeOK
			m.lastProbeTotal = update.ProbeTotal
			m.connFailures = update.Failures
			m.healthHistory = append(m.healthHistory, healthCheckEntry{
				Timestamp:  update.Timestamp,
				Ping:       update.Ping,
				Speed:      update.Speed,
				ProbeOK:    update.ProbeOK,
				ProbeTotal: update.ProbeTotal,
				Failures:   update.Failures,
			})
			if len(m.healthHistory) > 20 {
				m.healthHistory = m.healthHistory[len(m.healthHistory)-20:]
			}
		}
		return m, waitStatusUpdateCmd(m.ctx, m.status)
	case streamClosedMsg:
		m.closed = true
		return m, tea.Quit
	case dashboardContextDoneMsg:
		m.closed = true
		return m, tea.Quit
	case netStatsMsg:
		s := netstats.Stats(msg)
		m.lastNS = &s
		return m, waitNetStatsCmd(m.ctx, m.nsChan)
	case clockTickMsg:
		if m.closed {
			return m, nil
		}
		return m, dashboardTickCmd()
	}
	return m, nil
}

// --- Styles ---

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// --- View ---

func (m dashboardModel) View() string {
	var lines []string

	phaseLabel := string(m.phase)
	if phaseLabel == "" {
		phaseLabel = "starting"
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf(" FiVi --- %s ", phaseLabel)))
	lines = append(lines, "")

	switch m.phase {
	case engine.PhaseMonitoring:
		lines = append(lines, m.viewMonitoring()...)
	default:
		lines = append(lines, m.viewDiscovery()...)
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(" q quit"))
	return strings.Join(lines, "\n")
}

func (m dashboardModel) viewDiscovery() []string {
	var lines []string

	// Info line.
	lines = append(lines, fmt.Sprintf(" %s %s",
		labelStyle.Render("uptime:"), time.Since(m.startedAt).Round(time.Second)))

	// Progress bar.
	if m.totalCands > 0 {
		lines = append(lines, "")
		lines = append(lines, " "+renderProgressBar(m.progress.processed, m.totalCands, 40))
	}
	lines = append(lines, "")

	// Stats with color.
	running := len(m.progress.active)
	lines = append(lines, fmt.Sprintf(" %s %s  |  %s %s  |  %s %s  |  %s %s",
		labelStyle.Render("Working:"), greenStyle.Render(fmt.Sprintf("%d", m.progress.working)),
		labelStyle.Render("Probable:"), yellowStyle.Render(fmt.Sprintf("%d", m.progress.probable)),
		labelStyle.Render("Failed:"), redStyle.Render(fmt.Sprintf("%d", m.progress.failed)),
		labelStyle.Render("Running:"), fmt.Sprintf("%d", running),
	))
	lines = append(lines, fmt.Sprintf(" %s %s",
		labelStyle.Render("Stages:"), formatActiveStages(m.progress.active)))

	// Connecting node.
	if m.phase == engine.PhaseConnecting || m.phase == engine.PhaseFailover {
		label := "Connecting:"
		if m.phase == engine.PhaseFailover {
			label = "Failover:"
		}
		lines = append(lines, fmt.Sprintf(" %s %s",
			yellowStyle.Render(label), m.connectedNode))
	}
	lines = append(lines, "")

	// Worker table.
	lines = append(lines, m.renderColorWorkerTable(tableRowLimit(m.height)))
	return lines
}

func (m dashboardModel) viewMonitoring() []string {
	var lines []string

	// Node info.
	uptime := "n/a"
	if !m.connectedAt.IsZero() {
		uptime = time.Since(m.connectedAt).Round(time.Second).String()
	}
	lines = append(lines, fmt.Sprintf(" %s %s  |  %s  |  uptime: %s",
		labelStyle.Render("Node:"), m.connectedNode, m.connProtocol, uptime))

	// Connection metrics.
	pingStr := m.lastPing.Round(time.Millisecond).String()
	speedStr := "n/a"
	if m.lastSpeed > 0 {
		speedStr = fmt.Sprintf("%.1f Mbps", m.lastSpeed)
	}
	statusStr := greenStyle.Render("OK")
	if m.connFailures > 0 {
		statusStr = redStyle.Render(fmt.Sprintf("DEGRADED(%d)", m.connFailures))
	}
	probeStr := fmt.Sprintf("%d/%d", m.lastProbeOK, m.lastProbeTotal)
	lines = append(lines, fmt.Sprintf(" %s %s  |  %s %s  |  %s %s  |  %s %s",
		labelStyle.Render("Ping:"), pingStr,
		labelStyle.Render("Speed:"), speedStr,
		labelStyle.Render("Probes:"), probeStr,
		labelStyle.Render("Status:"), statusStr))
	lines = append(lines, "")

	// Proxy addresses.
	lines = append(lines, fmt.Sprintf(" %s %s    %s %s",
		labelStyle.Render("SOCKS5:"), m.socksAddr,
		labelStyle.Render("HTTP:"), m.httpAddr))

	// Network statistics (when --stats is enabled).
	if m.lastNS != nil {
		lines = append(lines, fmt.Sprintf(" %s %s %s (%s)  %s %s (%s)  |  %s %s",
			labelStyle.Render("Net:"),
			greenStyle.Render("\u25bc"), netstats.FormatSpeed(m.lastNS.RecvSpeed), netstats.FormatTotal(m.lastNS.TotalRecv),
			greenStyle.Render("\u25b2"), netstats.FormatSpeed(m.lastNS.SendSpeed), netstats.FormatTotal(m.lastNS.TotalSent),
			labelStyle.Render("Iface:"), m.lastNS.Interface,
		))
	}
	lines = append(lines, "")

	// Health check history.
	if len(m.healthHistory) > 0 {
		lines = append(lines, labelStyle.Render(" Recent Health Checks"))
		maxShow := 10
		if m.height > 0 {
			maxShow = m.height - 12
			if maxShow < 3 {
				maxShow = 3
			}
			if maxShow > 20 {
				maxShow = 20
			}
		}
		start := len(m.healthHistory) - maxShow
		if start < 0 {
			start = 0
		}
		for i := len(m.healthHistory) - 1; i >= start; i-- {
			h := m.healthHistory[i]
			ts := h.Timestamp.Format("15:04:05")
			st := greenStyle.Render("OK  ")
			if h.Failures > 0 {
				st = redStyle.Render("FAIL")
			}
			sp := "n/a"
			if h.Speed > 0 {
				sp = fmt.Sprintf("%.1fMbps", h.Speed)
			}
			lines = append(lines, fmt.Sprintf("  %s  %s  ping=%-8s  speed=%-10s  %d/%d",
				ts, st, h.Ping.Round(time.Millisecond), sp, h.ProbeOK, h.ProbeTotal))
		}
	}
	return lines
}

// --- Worker table ---

func (m dashboardModel) renderColorWorkerTable(limit int) string {
	if len(m.workers) == 0 {
		return dimStyle.Render(" workers: waiting for events...")
	}
	if limit < 1 {
		limit = 1
	}

	rows := make([]workerRow, 0, len(m.workers))
	for _, row := range m.workers {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		iActive := rows[i].Status == engine.WorkerStatusRunning
		jActive := rows[j].Status == engine.WorkerStatusRunning
		if iActive != jActive {
			return iActive
		}
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].WorkerID < rows[j].WorkerID
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render(fmt.Sprintf(" %-5s %-7s   %-9s %-5s %-8s %s",
		"#", "stage", "status", "att", "elapsed", "candidate")))
	b.WriteString("\n")
	for _, row := range rows {
		var icon, statusStyled string
		switch row.Status {
		case engine.WorkerStatusRunning:
			statusStyled = yellowStyle.Render("running")
			icon = yellowStyle.Render("*")
		case engine.WorkerStatusOK:
			statusStyled = greenStyle.Render("ok")
			icon = greenStyle.Render("+")
		case engine.WorkerStatusFailed:
			statusStyled = redStyle.Render("failed")
			icon = redStyle.Render("x")
		case engine.WorkerStatusProbable:
			statusStyled = yellowStyle.Render("probable")
			icon = yellowStyle.Render("~")
		default:
			statusStyled = string(row.Status)
			icon = " "
		}
		b.WriteString(fmt.Sprintf(" #%-4d %-7s %s %-9s [%d]  %-8s %s\n",
			row.WorkerID,
			row.Stage,
			icon,
			statusStyled,
			row.Attempt,
			formatElapsed(row.ElapsedMs),
			row.Candidate,
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- Helpers ---

func renderProgressBar(current, total, width int) string {
	if total <= 0 {
		return ""
	}
	pct := float64(current) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("\u2593", filled) + strings.Repeat("\u2591", width-filled)
	return fmt.Sprintf("%s %d/%d (%d%%)", bar, current, total, int(pct*100))
}

func formatElapsed(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
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

func waitStatusUpdateCmd(ctx context.Context, status <-chan engine.StatusUpdate) tea.Cmd {
	return func() tea.Msg {
		if status == nil {
			<-ctx.Done()
			return dashboardContextDoneMsg{}
		}
		select {
		case <-ctx.Done():
			return dashboardContextDoneMsg{}
		case update, ok := <-status:
			if !ok {
				return streamClosedMsg{}
			}
			return statusUpdateMsg(update)
		}
	}
}

func waitNetStatsCmd(ctx context.Context, ch <-chan netstats.Stats) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			<-ctx.Done()
			return dashboardContextDoneMsg{}
		}
		select {
		case <-ctx.Done():
			return dashboardContextDoneMsg{}
		case s, ok := <-ch:
			if !ok {
				return streamClosedMsg{}
			}
			return netStatsMsg(s)
		}
	}
}

func dashboardTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return clockTickMsg{}
	})
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
