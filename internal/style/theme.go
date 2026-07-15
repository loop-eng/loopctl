package style

import "charm.land/lipgloss/v2"

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
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
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
