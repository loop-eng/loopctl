package panel

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/loop-eng/loopctl/internal/model"
	"github.com/loop-eng/loopctl/internal/style"
)

type DetailPanel struct {
	viewport   viewport.Model
	width      int
	height     int
	session    model.SessionView
	hasSession bool
}

func NewDetailPanel() DetailPanel {
	return DetailPanel{viewport: viewport.New()}
}

func (p *DetailPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	// -2 for the border, -4 for the header block rendered above the viewport.
	vpHeight := h - 6
	if vpHeight < 3 {
		vpHeight = 3
	}
	vpWidth := w - 2
	if vpWidth < 10 {
		vpWidth = 10
	}
	p.viewport.SetWidth(vpWidth)
	p.viewport.SetHeight(vpHeight)
	if p.hasSession {
		p.viewport.SetContent(renderDetailBody(p.session, vpWidth))
	}
}

func (p *DetailPanel) SetSession(s model.SessionView) {
	p.session = s
	p.hasSession = true
	vpWidth := p.width - 2
	if vpWidth < 10 {
		vpWidth = 10
	}
	p.viewport.SetContent(renderDetailBody(s, vpWidth))
}

func (p *DetailPanel) Clear() {
	p.hasSession = false
}

func (p DetailPanel) Update(msg tea.Msg) (DetailPanel, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p DetailPanel) View() string {
	header := renderDetailHeader(p.session, p.width-2)
	body := style.FocusedBorder.Width(max(p.width-2, 1)).Height(max(p.height-4, 1)).Render(p.viewport.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func renderDetailHeader(s model.SessionView, width int) string {
	name := s.ProjectName
	if name == "" {
		name = "(unknown project)"
	}
	title := lipgloss.NewStyle().Bold(true).Width(width).Render(name)
	dir := style.Subtle.Width(width).Render(s.ProjectDir)

	sid := s.SessionID
	if len(sid) > 12 {
		sid = sid[:12] + "…"
	}
	pidStr := "—"
	if s.PID > 0 {
		pidStr = fmt.Sprintf("%d", s.PID)
	}
	meta := fmt.Sprintf("%s | %s | Session %s | PID %s | %s", s.Agent, s.Model, sid, pidStr, statusIcon(s))

	return lipgloss.JoinVertical(lipgloss.Left, title, dir, meta)
}

func renderDetailBody(s model.SessionView, width int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n", style.Bold.Render("Timing"))
	if !s.StartedAt.IsZero() {
		fmt.Fprintf(&b, "Started       %s\n", s.StartedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Fprintf(&b, "Started       —\n")
	}
	fmt.Fprintf(&b, "Duration      %s\n", formatDuration(s.Duration))
	if s.Active {
		fmt.Fprintf(&b, "Last active   active now\n")
	} else if !s.LastActivity.IsZero() {
		fmt.Fprintf(&b, "Last active   %s ago\n", time.Since(s.LastActivity).Round(time.Second))
	} else {
		fmt.Fprintf(&b, "Last active   —\n")
	}

	fmt.Fprintf(&b, "\n%s\n", style.Bold.Render("Cost"))
	fmt.Fprintf(&b, "Total cost    %s\n", style.CostStyle(s.TotalCost).Render(fmt.Sprintf("$%.4f", s.TotalCost)))
	fmt.Fprintf(&b, "Burn rate     $%.4f/min\n", s.BurnRate)
	fmt.Fprintf(&b, "Input tokens        %s\n", formatInt(s.TotalInput))
	fmt.Fprintf(&b, "Output tokens       %s\n", formatInt(s.TotalOutput))
	fmt.Fprintf(&b, "Cache read tokens   %s\n", formatInt(s.TotalCacheRead))
	fmt.Fprintf(&b, "Cache write tokens  %s\n", formatInt(s.TotalCacheWrite))
	fmt.Fprintf(&b, "Total tokens        %s\n", formatInt(s.TotalInput+s.TotalOutput+s.TotalCacheRead+s.TotalCacheWrite))

	fmt.Fprintf(&b, "\n%s\n", style.Bold.Render("Context Health"))
	barWidth := width - 10
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 8 {
		barWidth = 8
	}
	fmt.Fprintf(&b, "Fill          %s %s\n", fmt.Sprintf("%.0f%%", s.ContextFillPct), style.ContextBar(s.ContextFillPct, barWidth))
	fmt.Fprintf(&b, "Compactions   %d\n", s.CompactionCount)
	fmt.Fprintf(&b, "Efficiency    %.0f%%\n", s.TokenEfficiency)
	fmt.Fprintf(&b, "Cache hit     %.0f%%\n", s.CacheHitRate)

	iters := s.IterationCount
	iterLabel := "Iterations (tool calls)"
	if s.LTFAvailable {
		iters = s.LTFIterationCount
		iterLabel = "Iterations (LTF-verified)"
	}
	fmt.Fprintf(&b, "%s  %d\n", iterLabel, iters)
	if s.TerminationReason != "" {
		fmt.Fprintf(&b, "Termination   %s\n", s.TerminationReason)
	}

	fmt.Fprintf(&b, "\n%s\n", style.Bold.Render("Spin / Warnings"))
	if len(s.SpinReasons) == 0 {
		fmt.Fprintf(&b, "%s\n", style.Subtle.Render("No spin or stall warnings"))
	} else {
		lineStyle := style.AlertWarning
		if s.IsSpinning {
			lineStyle = style.AlertCritical
		}
		for _, r := range s.SpinReasons {
			fmt.Fprintf(&b, "%s\n", lineStyle.Render("• "+r))
		}
	}

	fmt.Fprintf(&b, "\n%s (%d total)\n", style.Bold.Render("Files Changed"), s.FilesChanged)
	if len(s.FilesChangedList) == 0 {
		fmt.Fprintf(&b, "%s\n", style.Subtle.Render("No files changed"))
	} else {
		for _, f := range s.FilesChangedList {
			fmt.Fprintf(&b, "• %s\n", f)
		}
		if len(s.FilesChangedList) == 10000 {
			fmt.Fprintf(&b, "%s\n", style.Subtle.Render("showing first 10,000 (cap reached)"))
		}
	}

	fmt.Fprintf(&b, "\n%s (%d total)\n", style.Bold.Render("Errors"), s.ErrorCount)
	if len(s.ErrorMessages) == 0 {
		fmt.Fprintf(&b, "%s\n", style.Subtle.Render("No errors"))
	} else {
		for i := len(s.ErrorMessages) - 1; i >= 0; i-- {
			msg := s.ErrorMessages[i]
			if len(msg) > width && width > 1 {
				msg = msg[:width-1] + "…"
			}
			fmt.Fprintf(&b, "%s\n", style.AlertCritical.Render("• "+msg))
		}
	}

	return b.String()
}

// formatInt renders n with thousands separators, e.g. 1234567 -> "1,234,567".
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
