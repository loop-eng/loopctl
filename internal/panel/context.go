package panel

import (
	"fmt"
	"strings"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type ContextPanel struct {
	width  int
	height int
	data   ContextData
}

type ContextData struct {
	FillPercent     float64
	CompactionCount int
	TokenEfficiency float64
	CacheHitRate    float64
}

func NewContextPanel() ContextPanel {
	return ContextPanel{}
}

func (p *ContextPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ContextPanel) Update(s model.SessionView) {
	p.data = ContextData{
		FillPercent:     s.ContextFillPct,
		CompactionCount: s.CompactionCount,
		TokenEfficiency: s.TokenEfficiency,
		CacheHitRate:    s.CacheHitRate,
	}
}

func (p ContextPanel) View(focused bool) string {
	var b strings.Builder

	b.WriteString(style.Bold.Render("Context"))
	b.WriteString("\n\n")

	fillStr := fmt.Sprintf("%.0f%%", p.data.FillPercent)
	b.WriteString("Fill     " + style.ContextStyle(p.data.FillPercent).Render(fillStr))
	b.WriteString("\n")

	barWidth := p.width - 6
	if barWidth < 8 {
		barWidth = 8
	}
	if barWidth > 20 {
		barWidth = 20
	}
	b.WriteString("         " + style.ContextBar(p.data.FillPercent, barWidth))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("Compact  %d\n", p.data.CompactionCount))
	b.WriteString(fmt.Sprintf("Effic    %.0f%%\n", p.data.TokenEfficiency))
	b.WriteString(fmt.Sprintf("Cache    %.0f%%\n", p.data.CacheHitRate))

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
	return border.Width(w).Height(h).Render(b.String())
}
