// Package theme centralises every colour and style used by the UI.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme holds the base palette plus the styles derived from it.
type Theme struct {
	// Base palette (adaptive: first value = light terminals, second = dark).
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Text      lipgloss.AdaptiveColor
	Subtle    lipgloss.AdaptiveColor
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
		Primary:   lipgloss.AdaptiveColor{Light: "#C85B1B", Dark: "#ED8E26"},
		Secondary: lipgloss.AdaptiveColor{Light: "#E07528", Dark: "#F8C648"},
		Text:      lipgloss.AdaptiveColor{Light: "#1D1C21", Dark: "#FAFAFA"},
		Subtle:    lipgloss.AdaptiveColor{Light: "#807E86", Dark: "#A9A8AE"},
		Success:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"},
		Error:     lipgloss.AdaptiveColor{Light: "#E5332C", Dark: "#FF6E67"},
		Border:    lipgloss.AdaptiveColor{Light: "#E5E4E8", Dark: "#323137"},
		GradientA: "#F8C648",
		GradientB: "#C85B1B",
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
