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
			return m, nil
		case "y", "Y":
			m.selector.cursor, m.selector.chosen = 1, true
			return m, nil
		case "n", "N":
			m.selector.cursor, m.selector.chosen = 0, true
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)
	return m, cmd
}

func (m BrokenPromptModel) Done() bool { return m.canceled || m.selector.chosen }

func (m BrokenPromptModel) View() string { return "\n" + m.selector.View() + "\n" }

// PromptBrokenArtifact asks the already-running program to take over the
// screen with this prompt and blocks the calling task goroutine until the
// user answers. No second Program, no second stdin reader.
func PromptBrokenArtifact(ctx Context, artifact, reason string) (bool, error) {
	final := ctx.Ask(NewBrokenPromptModel(artifact, reason))
	model := final.(BrokenPromptModel)
	if model.canceled {
		return false, nil
	}
	return model.selector.Value() == "continue", nil
}
