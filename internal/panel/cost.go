package panel

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type CostPanel struct {
	width  int
	height int
	data   CostData
}

type CostData struct {
	SelectedCost float64
	BurnRate     float64
	DailyTotal   float64
	TopSessions  []model.SessionView
}

func NewCostPanel() CostPanel {
	return CostPanel{}
}

func (p *CostPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *CostPanel) Update(data CostData) {
	p.data = data
}

func (p CostPanel) View(focused bool) string {
	var b strings.Builder

	b.WriteString(style.Bold.Render("Cost"))
	b.WriteString("\n\n")

	costStr := fmt.Sprintf("$%.2f", p.data.SelectedCost)
	b.WriteString("Session  " + style.CostStyle(p.data.SelectedCost).Render(costStr))
	b.WriteString("\n")

	rateStr := fmt.Sprintf("$%.3f/min", p.data.BurnRate)
	b.WriteString("Rate     " + rateStr)
	b.WriteString("\n")

	dailyStr := fmt.Sprintf("$%.2f", p.data.DailyTotal)
	b.WriteString("Today    " + style.CostStyle(p.data.DailyTotal).Render(dailyStr))
	b.WriteString("\n")

	if len(p.data.TopSessions) > 0 {
		b.WriteString("\n" + style.Subtle.Render("Top sessions"))
		b.WriteString("\n")
		for i, s := range p.data.TopSessions {
			if i >= 3 {
				break
			}
			name := filepath.Base(s.ProjectDir)
			if len(name) > 12 {
				name = name[:9] + "..."
			}
			b.WriteString(fmt.Sprintf(" %s $%.2f\n", name, s.TotalCost))
		}
	}

	content := b.String()
	border := style.PanelBorder
	if focused {
		border = style.FocusedBorder
	}
	w := p.width - 2
	if w < 1 {
		w = 1
	}
	h := p.height - 2
	if h < 1 {
		h = 1
	}
	return border.Width(w).Height(h).Render(content)
}

