package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

// BuildTasks turns the fully-resolved parameter values into the ordered list
// of tasks to execute. This is where the caller wires in real install logic.
type BuildTasks func(values map[string]string) []Task

type phase int

const (
	phasePrompt phase = iota
	phaseRun
	phaseDone
)

type appModel struct {
	theme theme.Theme
	phase phase

	// prompt phase
	prompts   []config.Param
	promptIdx int
	selector  selectorModel

	// resolved values fed to BuildTasks
	values map[string]string
	build  BuildTasks

	// run phase
	runner runnerModel
	send   func(tea.Msg)

	width  int
	height int
	err    error
	quit   bool
}

func newAppModel(th theme.Theme, res config.Result, build BuildTasks, send func(tea.Msg)) appModel {
	m := appModel{
		theme:   th,
		prompts: res.Prompts,
		values:  res.Values,
		build:   build,
		send:    send,
	}
	if m.values == nil {
		m.values = map[string]string{}
	}

	if len(res.Prompts) == 0 {
		// Nothing to ask: build the runner now (constructor has everything it
		// needs) so Init can just start it — Init must not mutate the model.
		m.phase = phaseRun
		m.runner = newRunner(th, build(m.values), send)
	} else {
		m.phase = phasePrompt
		m.selector = newSelector(th, res.Prompts[0])
	}
	return m
}

func (m appModel) Init() tea.Cmd {
	if m.phase == phaseRun {
		return m.runner.Init()
	}
	if m.phase == phasePrompt {
		return m.selector.Init()
	}
	return nil
}

// enterRun builds the runner from the collected values and switches phase.
// Pointer receiver: mutates the model. Returns the runner's Init command.
func (m *appModel) enterRun() tea.Cmd {
	m.phase = phaseRun
	m.runner = newRunner(m.theme, m.build(m.values), m.send)
	return m.runner.Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.selector.width = msg.Width
		m.runner.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quit = true
			return m, tea.Quit
		}
		if m.phase == phaseDone {
			// Any key exits once we are done.
			m.quit = true
			return m, tea.Quit
		}
	}

	switch m.phase {
	case phasePrompt:
		return m.updatePrompt(msg)
	case phaseRun:
		return m.updateRun(msg)
	default:
		return m, nil
	}
}

func (m appModel) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.selector, cmd = m.selector.Update(msg)

	if m.selector.chosen {
		p := m.prompts[m.promptIdx]
		chosenVal := m.selector.Value()
		m.values[p.Key] = chosenVal

		// Dynamically filter remaining prompts if the user just selected the "action"
		if p.Key == "action" {
			remaining := m.prompts[m.promptIdx+1:]
			m.prompts = append(m.prompts[:m.promptIdx+1], config.FilterPrompts(remaining, chosenVal)...)
		}

		m.promptIdx++

		if m.promptIdx < len(m.prompts) {
			m.selector = newSelector(m.theme, m.prompts[m.promptIdx])
			m.selector.width = m.width
			return m, m.selector.Init() // Kicks off cursor blink for input prompts
		}
		// All prompts answered -> run. Sequence the mutation before returning m.
		cmd := m.enterRun()
		return m, cmd
	}
	return m, cmd
}

func (m appModel) updateRun(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(allDoneMsg); ok {
		m.err = done.err
		m.phase = phaseDone
		return m, nil
	}
	var cmd tea.Cmd
	m.runner, cmd = m.runner.Update(msg)
	return m, cmd
}

func (m appModel) View() string {
	th := m.theme
	out := "\n" + th.Title.Render("fxManager Installer") + "\n\n"

	switch m.phase {
	case phasePrompt:
		out += m.selector.View() + "\n"
	case phaseRun:
		out += m.runner.View()
	case phaseDone:
		out += m.doneView()
	}
	return out
}

func (m appModel) doneView() string {
	th := m.theme
	out := m.runner.View() + "\n"

	if m.err != nil {
		out += th.ErrorTxt.Render("✗ Failed: "+m.err.Error()) + "\n"
	} else {
		out += th.SuccessTxt.Render("✓ All steps completed successfully.") + "\n"
	}

	// Keep the final frame on screen until the user acknowledges it. When they
	// press a key we set quit and this prompt disappears from the last frame.
	if !m.quit {
		out += "\n" + th.Hint.Render("Press enter to exit.") + "\n"
	}
	return out
}
