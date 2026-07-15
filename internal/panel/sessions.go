package panel

import (
	"fmt"
	"path/filepath"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type SessionPanel struct {
	table  table.Model
	width  int
	height int
}

func NewSessionPanel() SessionPanel {
	cols := defaultColumns(80)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).BorderBottom(true).BorderStyle(lipgloss.NormalBorder())
	s.Selected = s.Selected.Background(lipgloss.Color("236")).Foreground(lipgloss.Color("255"))
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(nil),
		table.WithStyles(s),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	return SessionPanel{table: t}
}

func (p *SessionPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	cols := defaultColumns(w)
	p.table.SetColumns(cols)
	p.table.SetWidth(w)
	p.table.SetHeight(h)
}

func (p *SessionPanel) Update(sessions []model.SessionView) {
	rows := make([]table.Row, len(sessions))
	for i, s := range sessions {
		rows[i] = sessionToRow(s)
	}
	p.table.SetRows(rows)
}

func (p *SessionPanel) Table() *table.Model {
	return &p.table
}

func (p SessionPanel) View() string {
	return p.table.View()
}

func (p SessionPanel) SelectedIndex() int {
	return p.table.Cursor()
}

func defaultColumns(totalWidth int) []table.Column {
	return []table.Column{
		{Title: "Status", Width: 9},
		{Title: "Project", Width: max(totalWidth/4, 15)},
		{Title: "Model", Width: 18},
		{Title: "Duration", Width: 9},
		{Title: "Iters", Width: 6},
		{Title: "Cost", Width: 9},
		{Title: "Context", Width: 10},
		{Title: "Tool/min", Width: 8},
	}
}

func sessionToRow(s model.SessionView) table.Row {
	status := statusIcon(s)
	project := s.ProjectName
	if project == "" {
		project = filepath.Base(s.ProjectDir)
	}
	if len(project) > 20 {
		project = project[:17] + "..."
	}

	model := s.Model
	if len(model) > 18 {
		model = model[:15] + "..."
	}

	dur := formatDuration(s.Duration)
	iters := fmt.Sprintf("%d", s.IterationCount)
	cost := style.CostStyle(s.TotalCost).Render(fmt.Sprintf("$%.2f", s.TotalCost))
	ctx := style.ContextStyle(s.ContextFillPct).Render(fmt.Sprintf("%.0f%%", s.ContextFillPct))

	toolsPerMin := float64(0)
	if s.Duration.Minutes() > 0 {
		toolsPerMin = float64(s.ToolCallCount) / s.Duration.Minutes()
	}
	tpm := fmt.Sprintf("%.1f", toolsPerMin)

	return table.Row{status, project, model, dur, iters, cost, ctx, tpm}
}

func statusIcon(s model.SessionView) string {
	if s.IsSpinning {
		return style.StatusSpin.Render("⊘ SPIN")
	}
	if s.Active {
		return style.StatusActive.Render("● Run")
	}
	if s.ErrorCount > 0 {
		return style.StatusFailed.Render("✗ Fail")
	}
	return style.StatusComplete.Render("○ Done")
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
