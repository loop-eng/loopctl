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
	specs := []colSpec{
		{title: "Status", min: 7, ideal: 9, priority: 5},
		{title: "Project", min: 12, ideal: max(totalWidth/4, 15), priority: 6},
		{title: "Model", min: 10, ideal: 18, priority: 3},
		{title: "Duration", min: 7, ideal: 9, priority: 2},
		{title: "Iters", min: 5, ideal: 6, priority: 1},
		{title: "Cost", min: 7, ideal: 9, priority: 7},
		{title: "Context", min: 7, ideal: 10, priority: 4},
		{title: "Tool/min", min: 6, ideal: 8, priority: 0},
	}
	return fitColumns(totalWidth, specs)
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
	if s.LTFAvailable {
		iters = fmt.Sprintf("%d", s.LTFIterationCount)
	}
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
	if s.HasWarnings && s.Active {
		return style.StatusPaused.Render("⚠ Warn")
	}
	if s.Active {
		return style.StatusActive.Render("● Run")
	}
	if !s.Active && s.TerminationReason != "" {
		if label, styled := terminationLabel(s.TerminationReason); label != "" {
			return styled
		}
	}
	if s.ErrorCount > 0 {
		return style.StatusFailed.Render("✗ Fail")
	}
	return style.StatusComplete.Render("○ Done")
}

// terminationLabel maps an LTF loop_summary termination_reason to a
// reason-aware status label. Returns ("", "") for reasons with no special
// rendering, so the caller falls through to the generic Done/Fail logic.
func terminationLabel(reason string) (label string, styled string) {
	switch reason {
	case "goal_met":
		return reason, style.StatusActive.Render("✓ Goal met")
	case "budget_exhausted":
		return reason, style.StatusFailed.Render("✗ Budget")
	case "max_iterations":
		return reason, style.StatusFailed.Render("✗ Max iter")
	case "user_cancelled":
		return reason, style.StatusComplete.Render("○ Cancelled")
	case "spin_detected":
		return reason, style.StatusSpin.Render("⊘ Spin")
	case "stall_detected":
		return reason, style.StatusPaused.Render("⊘ Stall")
	case "error":
		return reason, style.StatusFailed.Render("✗ Error")
	default:
		return "", ""
	}
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
