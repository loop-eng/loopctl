package style

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	Subtle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	Highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	Bold      = lipgloss.NewStyle().Bold(true)

	StatusActive   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	StatusPaused   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	StatusComplete = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	StatusFailed   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StatusSpin     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	CostLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	CostMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	CostHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	ContextLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	ContextMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	ContextHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	AlertWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	AlertCritical = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	OutcomeDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	OutcomeFailed = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	OutcomeKilled = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	OutcomeWarned = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	PanelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))

	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("212"))

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))

	HelpBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255"))
)

func CostStyle(cost float64) lipgloss.Style {
	switch {
	case cost >= 15.0:
		return CostHigh
	case cost >= 5.0:
		return CostMedium
	default:
		return CostLow
	}
}

func ContextStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 85.0:
		return ContextHigh
	case pct >= 60.0:
		return ContextMedium
	default:
		return ContextLow
	}
}

func ContextBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	return ContextStyle(pct).Render(bar)
}

// OutcomeStyleFor returns the style for a session-history outcome label
// ("Done" | "Failed" | "Killed" | "Warned"). Takes a plain string rather
// than a typed enum so internal/panel doesn't need to import internal/app
// (which would create an import cycle — app already imports panel).
func OutcomeStyleFor(outcome string) lipgloss.Style {
	switch outcome {
	case "Failed":
		return OutcomeFailed
	case "Killed":
		return OutcomeKilled
	case "Warned":
		return OutcomeWarned
	default:
		return OutcomeDone
	}
}

var sparkGlyphs = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders values as a row of block glyphs, most-recent value on
// the right. If there are more values than width, it downsamples by
// bucket-averaging into exactly width buckets. Never panics on nil/empty
// input or an all-zero series.
func Sparkline(values []float64, width int) string {
	if width <= 0 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat(" ", width)
	}

	var bucketed []float64
	if len(values) > width {
		bucketed = make([]float64, width)
		chunkSize := len(values) / width
		if chunkSize < 1 {
			chunkSize = 1
		}
		for i := 0; i < width; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if i == width-1 || end > len(values) {
				end = len(values)
			}
			if start >= len(values) {
				bucketed[i] = bucketed[i-1]
				continue
			}
			sum := 0.0
			for _, v := range values[start:end] {
				sum += v
			}
			bucketed[i] = sum / float64(end-start)
		}
	} else {
		bucketed = values
	}

	max := 0.0
	for _, v := range bucketed {
		if v > max {
			max = v
		}
	}

	pad := width - len(bucketed)
	var b strings.Builder
	if pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	for _, v := range bucketed {
		level := 0
		if max > 0 {
			level = int(v / max * 7)
			if level > 7 {
				level = 7
			}
			if level < 0 {
				level = 0
			}
		}
		b.WriteRune(sparkGlyphs[level])
	}
	return Subtle.Render(b.String())
}
