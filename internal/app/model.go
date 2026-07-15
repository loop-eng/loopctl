package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"charm.land/bubbles/v2/help"
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

	focusedPanel int
	showHelp     bool

	sessions []model.SessionView
	alerts   []model.Alert
	daily    float64

	help help.Model
	keys KeyMap

	collector Collector
	ready     bool
}

func New(collector Collector) Model {
	return Model{
		sessionPanel: panel.NewSessionPanel(),
		costPanel:    panel.NewCostPanel(),
		contextPanel: panel.NewContextPanel(),
		alertPanel:   panel.NewAlertPanel(),
		keys:         DefaultKeyMap(),
		help:         help.New(),
		collector:    collector,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
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

	case model.TickMsg:
		data := m.collector.Snapshot()
		m.sessions = data.Sessions
		m.alerts = data.Alerts
		m.daily = data.DailyTotal
		m.updatePanels()
		return m, tickCmd()
	}

	var cmd tea.Cmd
	t := *m.sessionPanel.Table()
	t, cmd = t.Update(msg)
	sp := m.sessionPanel
	*sp.Table() = t
	m.sessionPanel = sp

	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "tab":
		m.focusedPanel = (m.focusedPanel + 1) % numPanels
		if m.focusedPanel == panelSessions {
			m.sessionPanel.Table().Focus()
		} else {
			m.sessionPanel.Table().Blur()
		}
		return m, nil
	case "escape":
		m.showHelp = false
		return m, nil
	case "K":
		return m.handleKill()
	case "e":
		return m.handleExport()
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

func (m Model) handleKill() (tea.Model, tea.Cmd) {
	idx := m.sessionPanel.SelectedIndex()
	if idx < 0 || idx >= len(m.sessions) {
		return m, nil
	}
	s := m.sessions[idx]
	if s.PID <= 0 || !s.Active {
		return m, nil
	}
	pid := s.PID
	return m, func() tea.Msg {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil
		}
		proc.Signal(syscall.SIGTERM)
		return nil
	}
}

func (m Model) handleExport() (tea.Model, tea.Cmd) {
	idx := m.sessionPanel.SelectedIndex()
	if idx < 0 || idx >= len(m.sessions) {
		return m, nil
	}
	s := m.sessions[idx]
	return m, func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		dir := filepath.Join(home, ".config", "loopctl", "exports")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return model.ExportDoneMsg{Err: err}
		}
		path := filepath.Join(dir, s.SessionID+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
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
}

func (m *Model) updatePanels() {
	m.sessionPanel.Update(m.sessions)

	var selected model.SessionView
	idx := m.sessionPanel.SelectedIndex()
	if idx >= 0 && idx < len(m.sessions) {
		selected = m.sessions[idx]
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
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if !m.ready {
		v.SetContent("Loading...")
		return v
	}

	if m.showHelp {
		v.SetContent(m.renderHelp())
		return v
	}

	v.SetContent(m.renderDashboard())
	return v
}

func (m Model) renderDashboard() string {
	header := style.Title.Render("LoopCtl") + style.Subtle.Render(" — htop for AI agents")

	sessionTable := m.sessionPanel.View()

	bottom := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.costPanel.View(m.focusedPanel == panelCost),
		m.contextPanel.View(m.focusedPanel == panelContext),
		m.alertPanel.View(m.focusedPanel == panelAlerts),
	)

	helpBar := style.HelpBar.Render(m.help.View(m.keys))

	return lipgloss.JoinVertical(lipgloss.Left, header, sessionTable, bottom, helpBar)
}

func (m Model) renderHelp() string {
	content := style.Bold.Render("LoopCtl Keyboard Shortcuts") + "\n\n"
	content += m.help.FullHelpView(m.keys.FullHelp())
	content += "\n\n" + style.Subtle.Render("Press ? or ESC to close")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
