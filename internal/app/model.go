package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/panel"
	"github.com/loop-eng/loopctl/internal/style"
)

const (
	panelSessions = iota
	panelCost
	panelContext
	panelAlerts
	numPanels
)

type viewMode int

const (
	modeDashboard viewMode = iota
	modeHelp
	modeDetail
	modeFilter
	modeHistory
)

type Collector interface {
	Snapshot() model.DataMsg
}

type Model struct {
	width  int
	height int

	sessionPanel panel.SessionPanel
	costPanel    panel.CostPanel
	contextPanel panel.ContextPanel
	alertPanel   panel.AlertPanel
	detailPanel  panel.DetailPanel
	historyPanel panel.HistoryPanel

	focusedPanel int
	mode         viewMode

	detailSessionID string

	filterInput textinput.Model
	filterQuery string

	killedSessions map[string]bool

	sessions []model.SessionView
	alerts   []model.Alert
	daily    float64

	help help.Model
	keys KeyMap

	collector   Collector
	refreshRate time.Duration
	ready       bool
}

func New(collector Collector, refreshRate time.Duration) Model {
	if refreshRate <= 0 {
		refreshRate = time.Second
	}

	fi := textinput.New()
	fi.Placeholder = "project name…"
	fi.Prompt = "/ "
	fi.CharLimit = 100

	return Model{
		sessionPanel:   panel.NewSessionPanel(),
		costPanel:      panel.NewCostPanel(),
		contextPanel:   panel.NewContextPanel(),
		alertPanel:     panel.NewAlertPanel(),
		detailPanel:    panel.NewDetailPanel(),
		historyPanel:   panel.NewHistoryPanel(),
		filterInput:    fi,
		keys:           DefaultKeyMap(),
		help:           help.New(),
		collector:      collector,
		refreshRate:    refreshRate,
		killedSessions: make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return m.tickCmd()
}

func (m Model) tickCmd() tea.Cmd {
	rate := m.refreshRate
	return tea.Tick(rate, func(t time.Time) tea.Msg {
		return model.TickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		m.updatePanels()
		m.ready = true
		return m, nil

	case model.ExportDoneMsg:
		return m, nil

	case model.TickMsg:
		data := m.collector.Snapshot()
		m.sessions = data.Sessions
		m.alerts = data.Alerts
		m.daily = data.DailyTotal
		m.updatePanels()
		return m, m.tickCmd()
	}

	var cmd tea.Cmd
	switch m.mode {
	case modeFilter:
		m.filterInput, cmd = m.filterInput.Update(msg)
	case modeDetail:
		m.detailPanel, cmd = m.detailPanel.Update(msg)
	case modeHistory:
		t := *m.historyPanel.Table()
		t, cmd = t.Update(msg)
		*m.historyPanel.Table() = t
	default:
		t := *m.sessionPanel.Table()
		t, cmd = t.Update(msg)
		*m.sessionPanel.Table() = t
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeDetail:
		return m.handleDetailKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeHistory:
		return m.handleHistoryKey(msg)
	}
	return m.handleDashboardKey(msg)
}

func (m Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "escape", "q":
		m.mode = modeDashboard
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "escape", "enter", "q":
		m.mode = modeDashboard
		return m, nil
	case "K":
		return m.handleKill()
	case "e":
		return m.handleExport()
	}
	var cmd tea.Cmd
	m.detailPanel, cmd = m.detailPanel.Update(msg)
	return m, cmd
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filterInput.Blur()
		m.mode = modeDashboard
		m.updatePanels()
		return m, nil
	case "escape":
		m.filterInput.SetValue("")
		m.filterQuery = ""
		m.filterInput.Blur()
		m.mode = modeDashboard
		m.updatePanels()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterQuery = m.filterInput.Value()
	m.updatePanels()
	return m, cmd
}

func (m Model) handleHistoryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "escape", "h", "q":
		m.mode = modeDashboard
		return m, nil
	}
	t := *m.historyPanel.Table()
	var cmd tea.Cmd
	t, cmd = t.Update(msg)
	*m.historyPanel.Table() = t
	return m, cmd
}

func (m Model) handleDashboardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
		return m, nil
	case "tab":
		m.focusedPanel = (m.focusedPanel + 1) % numPanels
		if m.focusedPanel == panelSessions {
			m.sessionPanel.Table().Focus()
		} else {
			m.sessionPanel.Table().Blur()
		}
		return m, nil
	case "K":
		return m.handleKill()
	case "e":
		return m.handleExport()
	case "enter":
		if m.focusedPanel == panelSessions {
			return m.openDetail()
		}
		return m, nil
	case "/":
		m.filterInput.SetValue(m.filterQuery)
		m.filterInput.CursorEnd()
		cmd := m.filterInput.Focus()
		m.mode = modeFilter
		return m, cmd
	case "h":
		m.mode = modeHistory
		return m, nil
	}

	if m.focusedPanel == panelSessions {
		t := *m.sessionPanel.Table()
		var cmd tea.Cmd
		t, cmd = t.Update(msg)
		*m.sessionPanel.Table() = t
		m.updatePanels()
		return m, cmd
	}

	return m, nil
}

// visibleLiveSessions returns the active sessions currently shown in the
// live table — i.e. after the active-only split and any committed filter.
func (m Model) visibleLiveSessions() []model.SessionView {
	return applyFilter(filterActive(m.sessions), m.filterQuery)
}

func (m Model) openDetail() (tea.Model, tea.Cmd) {
	visible := m.visibleLiveSessions()
	idx := m.sessionPanel.SelectedIndex()
	if idx < 0 || idx >= len(visible) {
		return m, nil
	}
	m.detailSessionID = visible[idx].SessionID
	m.detailPanel.SetSize(m.width, m.height-1)
	m.detailPanel.SetSession(visible[idx])
	m.mode = modeDetail
	return m, nil
}

// resolveTarget returns the session that K/e should act on: the
// detail-mode session when Detail is open, otherwise the selected row in
// the live table. K and e behave identically from either place.
func (m Model) resolveTarget() (model.SessionView, bool) {
	if m.mode == modeDetail {
		for _, s := range m.sessions {
			if s.SessionID == m.detailSessionID {
				return s, true
			}
		}
		return model.SessionView{}, false
	}
	visible := m.visibleLiveSessions()
	idx := m.sessionPanel.SelectedIndex()
	if idx < 0 || idx >= len(visible) {
		return model.SessionView{}, false
	}
	return visible[idx], true
}

func (m Model) handleKill() (tea.Model, tea.Cmd) {
	s, ok := m.resolveTarget()
	if !ok || s.PID <= 1 || !s.Active {
		return m, nil
	}
	pid := s.PID
	// Best-effort labeling for the History view: records that loopctl sent
	// SIGTERM, not that the process actually died as a result of it.
	m.killedSessions[s.SessionID] = true
	return m, func() tea.Msg {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil
		}
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return nil
		}
		return nil
	}
}

func (m Model) handleExport() (tea.Model, tea.Cmd) {
	s, ok := m.resolveTarget()
	if !ok {
		return m, nil
	}
	return m, func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		dir := filepath.Join(home, ".config", "loopctl", "exports")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		safeID := filepath.Base(s.SessionID)
		path := filepath.Join(dir, safeID+".json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		return model.ExportDoneMsg{Path: path}
	}
}

func (m *Model) updateLayout() {
	tableHeight := m.height*60/100 - 2
	if tableHeight < 5 {
		tableHeight = 5
	}
	panelHeight := m.height - tableHeight - 3
	if panelHeight < 5 {
		panelHeight = 5
	}
	panelWidth := m.width / 3
	if panelWidth < 15 {
		panelWidth = 15
	}

	m.sessionPanel.SetSize(m.width, tableHeight)
	m.costPanel.SetSize(panelWidth, panelHeight)
	m.contextPanel.SetSize(panelWidth, panelHeight)
	alertWidth := m.width - 2*panelWidth
	if alertWidth < 10 {
		alertWidth = 10
	}
	m.alertPanel.SetSize(alertWidth, panelHeight)
	m.help.SetWidth(m.width)

	m.filterInput.SetWidth(max(m.width-4, 10))

	m.detailPanel.SetSize(m.width, m.height-1)

	historyTableHeight := m.height - 5
	if historyTableHeight < 5 {
		historyTableHeight = 5
	}
	m.historyPanel.SetSize(m.width, historyTableHeight)
}

func (m *Model) updatePanels() {
	live := filterActive(m.sessions)
	filtered := applyFilter(live, m.filterQuery)
	m.sessionPanel.Update(filtered)

	var selected model.SessionView
	idx := m.sessionPanel.SelectedIndex()
	if idx >= 0 && idx < len(filtered) {
		selected = filtered[idx]
	}

	sorted := make([]model.SessionView, len(m.sessions))
	copy(sorted, m.sessions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalCost > sorted[j].TotalCost
	})
	var topSessions []model.SessionView
	for i := 0; i < len(sorted) && i < 3; i++ {
		topSessions = append(topSessions, sorted[i])
	}

	m.costPanel.Update(panel.CostData{
		SelectedCost: selected.TotalCost,
		BurnRate:     selected.BurnRate,
		DailyTotal:   m.daily,
		TopSessions:  topSessions,
	})

	m.contextPanel.Update(selected)
	m.alertPanel.Update(m.alerts)

	completed := filterCompleted(m.sessions)
	outcomes := make(map[string]string, len(completed))
	for _, s := range completed {
		outcomes[s.SessionID] = string(classifyOutcome(s, m.killedSessions))
	}
	m.historyPanel.Update(completed, outcomes)

	if m.mode == modeDetail {
		m.refreshDetailSession()
	}
}

// refreshDetailSession re-resolves the Detail view's session against the
// latest snapshot. If the session has aged out of the collector (its
// underlying file fell outside the discovery window), Detail falls back
// to the dashboard rather than showing stale data.
func (m *Model) refreshDetailSession() {
	for _, s := range m.sessions {
		if s.SessionID == m.detailSessionID {
			m.detailPanel.SetSession(s)
			return
		}
	}
	m.mode = modeDashboard
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if !m.ready {
		v.SetContent("Loading...")
		return v
	}

	switch m.mode {
	case modeHelp:
		v.SetContent(m.renderHelp())
	case modeDetail:
		v.SetContent(m.renderDetail())
	case modeHistory:
		v.SetContent(m.renderHistory())
	default:
		v.SetContent(m.renderDashboard())
	}
	return v
}

func (m Model) renderDashboard() string {
	header := style.Title.Render("LoopCtl") + style.Subtle.Render(" — htop for AI agents")

	filtered := m.visibleLiveSessions()
	if m.filterQuery != "" && m.mode != modeFilter {
		header += style.Subtle.Render(fmt.Sprintf(" [filter: %q, %d shown]", m.filterQuery, len(filtered)))
	}

	sessionTable := m.sessionPanel.View()

	parts := []string{header, sessionTable}

	if len(filtered) == 0 && m.filterQuery != "" {
		parts = append(parts, style.Subtle.Render(fmt.Sprintf("No sessions match %q — press / to change, esc to clear", m.filterQuery)))
	}

	bottom := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.costPanel.View(m.focusedPanel == panelCost),
		m.contextPanel.View(m.focusedPanel == panelContext),
		m.alertPanel.View(m.focusedPanel == panelAlerts),
	)
	parts = append(parts, bottom)

	var footer string
	if m.mode == modeFilter {
		footer = style.HelpBar.Render(m.filterInput.View())
	} else {
		footer = style.HelpBar.Render(m.help.View(m.keys))
	}
	parts = append(parts, footer)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderHelp() string {
	content := style.Bold.Render("LoopCtl Keyboard Shortcuts") + "\n\n"
	content += m.help.FullHelpView(m.keys.FullHelp())
	content += "\n\n" + style.Subtle.Render("Press ? or ESC to close")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderDetail() string {
	footer := style.HelpBar.Render("[esc] back  [↑/↓] scroll  [K] kill  [e] export")
	return lipgloss.JoinVertical(lipgloss.Left, m.detailPanel.View(), footer)
}

func (m Model) renderHistory() string {
	completed := filterCompleted(m.sessions)
	outcomes := make(map[string]string, len(completed))
	for _, s := range completed {
		outcomes[s.SessionID] = string(classifyOutcome(s, m.killedSessions))
	}

	header := style.Title.Render("Session History") + style.Subtle.Render(" — completed sessions")
	tbl := m.historyPanel.View()
	summary := m.historyPanel.Summary(outcomes)
	footer := style.HelpBar.Render("[esc/h] back  [↑/↓] navigate")

	return lipgloss.JoinVertical(lipgloss.Left, header, tbl, summary, footer)
}
