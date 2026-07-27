package panel

import (
	"fmt"
	"sort"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type HistoryPanel struct {
	table    table.Model
	width    int
	height   int
	sessions []model.SessionView
}

func NewHistoryPanel() HistoryPanel {
	cols := historyColumns(80)
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
	return HistoryPanel{table: t}
}

func (p *HistoryPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	cols := historyColumns(w)
	p.table.SetColumns(cols)
	p.table.SetWidth(w)
	p.table.SetHeight(h)
}

// Update sets the completed sessions to display, sorted most-recent-first
// by LastActivity. outcomes maps SessionID -> outcome label string
// ("Done" | "Failed" | "Killed" | "Warned"), computed by internal/app
// (internal/panel must never import internal/app — see style.OutcomeStyleFor).
func (p *HistoryPanel) Update(sessions []model.SessionView, outcomes map[string]string) {
	sorted := make([]model.SessionView, len(sessions))
	copy(sorted, sessions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].LastActivity.After(sorted[j].LastActivity)
	})
	p.sessions = sorted

	rows := make([]table.Row, len(sorted))
	for i, s := range sorted {
		rows[i] = historyToRow(s, outcomes[s.SessionID])
	}
	p.table.SetRows(rows)
}

func (p *HistoryPanel) Table() *table.Model {
	return &p.table
}

func (p HistoryPanel) View() string {
	return p.table.View()
}

// Summary returns aggregate stats for the footer: total sessions, total
// cost, and a chronological (oldest->newest) cost sparkline.
func (p HistoryPanel) Summary(outcomes map[string]string) string {
	if len(p.sessions) == 0 {
		return style.Subtle.Render("No completed sessions yet")
	}

	var total float64
	counts := map[string]int{}
	costsChrono := make([]float64, len(p.sessions))
	// p.sessions is most-recent-first; reverse for chronological order.
	for i, s := range p.sessions {
		total += s.TotalCost
		counts[outcomes[s.SessionID]]++
		costsChrono[len(p.sessions)-1-i] = s.TotalCost
	}

	line := fmt.Sprintf("%d sessions · $%.2f total · %d done · %d failed · %d killed · %d warned",
		len(p.sessions), total, counts["Done"], counts["Failed"], counts["Killed"], counts["Warned"])

	sparkWidth := p.width - 2
	if sparkWidth < 4 {
		sparkWidth = 4
	}
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	spark := style.Sparkline(costsChrono, sparkWidth)

	return lipgloss.JoinVertical(lipgloss.Left, line, spark)
}

func historyColumns(totalWidth int) []table.Column {
	return []table.Column{
		{Title: "Project", Width: max(totalWidth/4, 15)},
		{Title: "Model", Width: 18},
		{Title: "Ended", Width: 16},
		{Title: "Duration", Width: 9},
		{Title: "Iters", Width: 6},
		{Title: "Cost", Width: 9},
		{Title: "Outcome", Width: 10},
	}
}

func historyToRow(s model.SessionView, outcome string) table.Row {
	project := s.ProjectName
	if len(project) > 20 {
		project = project[:17] + "..."
	}
	model := s.Model
	if len(model) > 18 {
		model = model[:15] + "..."
	}

	ended := "—"
	if !s.LastActivity.IsZero() {
		ended = humanizeRelative(time.Since(s.LastActivity))
	}

	dur := formatDuration(s.Duration)
	iters := fmt.Sprintf("%d", s.IterationCount)
	if s.LTFAvailable {
		iters = fmt.Sprintf("%d", s.LTFIterationCount)
	}
	cost := style.CostStyle(s.TotalCost).Render(fmt.Sprintf("$%.2f", s.TotalCost))
	outcomeCell := style.OutcomeStyleFor(outcome).Render(outcome)

	return table.Row{project, model, ended, dur, iters, cost, outcomeCell}
}

// humanizeRelative renders a duration as a short "time ago" string:
// <1m -> "just now", <60m -> "Xm ago", <24h -> "Xh ago", else "Xd ago".
func humanizeRelative(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
