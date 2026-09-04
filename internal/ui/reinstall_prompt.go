package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

type ReinstallPromptModel struct {
	selector selectorModel
	canceled bool
}

func NewReinstallPromptModel(installedVersion, latestVersion string) ReinstallPromptModel {
	param := config.Param{
		Usage: fmt.Sprintf(
			"ℹ️  Webpanel version '%s' is already installed (latest stable is '%s').\n\nDo you want to reinstall/overwrite with the latest stable version?",
			installedVersion,
			latestVersion,
		),
		Default: "skip",
		Options: []config.Option{
			{
				Title: "Skip update (Keep current version)",
				Value: "skip",
				Desc:  "Leave the currently installed webpanel intact.",
			},
			{
				Title: "Reinstall latest stable version",
				Value: "reinstall",
				Desc:  "Download and overwrite with the latest stable release.",
			},
		},
	}

	return ReinstallPromptModel{
		selector: newSelector(theme.Default(), param),
	}
}

func (m ReinstallPromptModel) Init() tea.Cmd {
	return m.selector.Init()
}

func (m ReinstallPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ReinstallPromptModel) Done() bool { return m.canceled || m.selector.chosen }

func (m ReinstallPromptModel) View() string { return "\n" + m.selector.View() + "\n" }

// PromptReinstallWebpanel prompts the user when the installed version is equal to
// or newer than the latest release, asking if they wish to reinstall anyway.
func PromptReinstallWebpanel(ctx Context, installedVersion, latestVersion string) (bool, error) {
	final := ctx.Ask(NewReinstallPromptModel(installedVersion, latestVersion))
	model := final.(ReinstallPromptModel)
	if model.canceled {
		return false, nil
	}
	return model.selector.Value() == "reinstall", nil
}
