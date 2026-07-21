package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

// askRequestMsg is sent from a task goroutine to ask the running program
// to display a sub-model until it's Done(), then hand its final state back.
type askRequestMsg struct {
	model confirmModel
	reply chan confirmModel
}

// sender bridges task goroutines and the running program. It starts with a nil
// function and is wired to program.Send once the program exists.
type sender struct {
	fn func(tea.Msg)
}

func (s *sender) send(msg tea.Msg) {
	if s.fn != nil {
		s.fn(msg)
	}
}

// ask blocks the calling goroutine (a task's Run) until the single running
// Program has driven m to completion.
func (s *sender) ask(m confirmModel) confirmModel {
	reply := make(chan confirmModel, 1)
	s.send(askRequestMsg{model: m, reply: reply})
	return <-reply
}

// Run launches the interactive installer: it prompts for any unresolved
// parameters, then executes the tasks returned by build, showing progress.
//
// It renders inline (no alt screen) so the completion summary stays in the
// terminal scrollback after the program exits.
func Run(th theme.Theme, res config.Result, build BuildTasks) error {
	s := &sender{}
	m := newAppModel(th, res, build, s.send)
	p := tea.NewProgram(m)

	// Wire the sender before Run so task goroutines can Report immediately.
	s.fn = p.Send

	final, err := p.Run()
	if err != nil {
		return err
	}
	if am, ok := final.(appModel); ok && am.err != nil {
		return am.err
	}
	return nil
}
