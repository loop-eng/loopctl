package panel

import (
	"fmt"
	"strings"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type AlertPanel struct {
	width  int
	height int
	alerts []model.Alert
}

func NewAlertPanel() AlertPanel {
	return AlertPanel{}
}

func (p *AlertPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *AlertPanel) Update(alerts []model.Alert) {
	p.alerts = alerts
}

func (p AlertPanel) View(focused bool) string {
	var b strings.Builder

	b.WriteString(style.Bold.Render("Alerts"))
	b.WriteString("\n\n")

	if len(p.alerts) == 0 {
		b.WriteString(style.Subtle.Render("  No alerts"))
		b.WriteString("\n")
	} else {
		maxShow := p.height - 4
		if maxShow < 1 {
			maxShow = 1
		}
		for i, a := range p.alerts {
			if i >= maxShow {
				remaining := len(p.alerts) - maxShow
				b.WriteString(style.Subtle.Render(fmt.Sprintf("  +%d more", remaining)))
				b.WriteString("\n")
				break
			}
			icon := "⚠"
			s := style.AlertWarning
			if a.Severity == "critical" {
				icon = "✗"
				s = style.AlertCritical
			}
			line := fmt.Sprintf(" %s %s", icon, a.Message)
			if len(line) > p.width-4 {
				line = line[:p.width-7] + "..."
			}
			b.WriteString(s.Render(line))
			b.WriteString("\n")
		}
	}

	border := style.PanelBorder
	if focused {
		border = style.FocusedBorder
	}
	return border.Width(p.width - 2).Height(p.height - 2).Render(b.String())
}
