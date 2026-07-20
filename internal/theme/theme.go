// Package theme centralises every colour and style used by the UI.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme holds the base palette plus the styles derived from it.
type Theme struct {
	// Base palette (adaptive: first value = light terminals, second = dark).
	Primary   lipgloss.AdaptiveColor // accent / current selection
	Secondary lipgloss.AdaptiveColor // secondary accent / descriptions
	Text      lipgloss.AdaptiveColor // default foreground
	Subtle    lipgloss.AdaptiveColor // dimmed foreground (hints, pending items)
	Success   lipgloss.AdaptiveColor
	Error     lipgloss.AdaptiveColor
	Border    lipgloss.AdaptiveColor

	// Progress-bar gradient. These must be plain hex strings because the
	// bubbles/progress API does not accept adaptive colours.
	GradientA string
	GradientB string

	// Derived styles (built once by build()).
	Title        lipgloss.Style
	Heading      lipgloss.Style
	Item         lipgloss.Style
	ItemDesc     lipgloss.Style
	Selected     lipgloss.Style
	SelectedDesc lipgloss.Style
	Cursor       lipgloss.Style
	Hint         lipgloss.Style
	SuccessTxt   lipgloss.Style
	ErrorTxt     lipgloss.Style
	Box          lipgloss.Style
}

// Default returns the built-in theme.
func Default() Theme {
	t := Theme{
		Primary:   lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
		Secondary: lipgloss.AdaptiveColor{Light: "#DB2777", Dark: "#F472B6"},
		Text:      lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"},
		Subtle:    lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"},
		Success:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"},
		Error:     lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"},
		Border:    lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"},
		GradientA: "#7C3AED",
		GradientB: "#EC4899",
	}
	return t.build()
}

// build populates the derived styles from the base palette.
func (t Theme) build() Theme {
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.Heading = lipgloss.NewStyle().Bold(true).Foreground(t.Text)
	t.Item = lipgloss.NewStyle().Foreground(t.Text)
	t.ItemDesc = lipgloss.NewStyle().Foreground(t.Subtle)
	t.Selected = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.SelectedDesc = lipgloss.NewStyle().Foreground(t.Secondary)
	t.Cursor = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	t.Hint = lipgloss.NewStyle().Foreground(t.Subtle)
	t.SuccessTxt = lipgloss.NewStyle().Bold(true).Foreground(t.Success)
	t.ErrorTxt = lipgloss.NewStyle().Bold(true).Foreground(t.Error)
	t.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)
	return t
}
