package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

type BrokenPromptModel struct {
	selector selectorModel
	canceled bool
}

func NewBrokenPromptModel(artifact, reason string) BrokenPromptModel {
	param := config.Param{
		Usage:   fmt.Sprintf("⚠️  WARNING: Artifact build '%s' is flagged as BROKEN!\nReason: %s\n\nDo you wish to continue with the installation anyway?", artifact, reason),
		Default: "quit",
		Options: []config.Option{
			{
				Title: "Abort installation (Recommended)",
				Value: "quit",
				Desc:  "Cancel the setup process to prevent downloading a faulty server build.",
			},
			{
				Title: "Continue anyway",
				Value: "continue",
				Desc:  "Proceed with downloading this artifact despite reported issues.",
			},
		},
	}

	return BrokenPromptModel{
		selector: newSelector(theme.Default(), param),
	}
}

func (m BrokenPromptModel) Init() tea.Cmd {
	return m.selector.Init()
}

func (m BrokenPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit

		case "y", "Y":
			m.selector.cursor = 1
			m.selector.chosen = true
			return m, tea.Quit

		case "n", "N":
			m.selector.cursor = 0
			m.selector.chosen = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)

	if m.selector.chosen {
		return m, tea.Quit
	}

	return m, cmd
}

func (m BrokenPromptModel) View() string {
	return "\n" + m.selector.View() + "\n"
}

// PromptBrokenArtifact executes the Bubbletea list selector prompt.
func PromptBrokenArtifact(artifact, reason string) (bool, error) {
	p := tea.NewProgram(NewBrokenPromptModel(artifact, reason))
	m, err := p.Run()
	if err != nil {
		return false, fmt.Errorf("running broken artifact prompt: %w", err)
	}

	model := m.(BrokenPromptModel)
	if model.canceled {
		return false, nil
	}

	return model.selector.Value() == "continue", nil
}
