package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/theme"
)

// Context is handed to every task's Run function. Use it to report progress.
type Context struct {
	send func(tea.Msg)
	idx  int
}

// Report updates the progress bar for a determinate task. fraction is clamped
// to [0, 1]. It is safe to call from the task's own goroutine (it ultimately
// calls tea.Program.Send, which is goroutine-safe).
//
// For an Indeterminate task (spinner), Report is a no-op — just do your work.
func (c Context) Report(fraction float64) {
	if c.send == nil {
		return
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	c.send(progressMsg{index: c.idx, fraction: fraction})
}

// Task is one step of the install/update flow. This is the type you plug your
// own logic into: give it a Title and a Run function.
//
//   - Indeterminate == false -> a progress bar is shown; call ctx.Report(f).
//   - Indeterminate == true  -> a spinner is shown; Report is ignored.
//
// Returning a non-nil error aborts the run and shows the error on the summary.
type Task struct {
	Title         string
	Indeterminate bool
	Run           func(ctx Context) error
}

// --- messages -------------------------------------------------------------

type (
	progressMsg struct {
		index    int
		fraction float64
	}
	taskDoneMsg struct {
		index int
		err   error
	}
	startMsg   struct{}
	allDoneMsg struct{ err error }
)

// --- status ---------------------------------------------------------------

type taskStatus int

const (
	statusPending taskStatus = iota
	statusRunning
	statusDone
	statusFailed
)

// --- model ----------------------------------------------------------------

type runnerModel struct {
	theme    theme.Theme
	tasks    []Task
	status   []taskStatus
	current  int // index of the running task, -1 before the first starts
	progress progress.Model
	spinner  spinner.Model
	send     func(tea.Msg)
	width    int
	done     bool
	err      error
}

func newRunner(th theme.Theme, tasks []Task, send func(tea.Msg)) runnerModel {
	p := progress.New(progress.WithGradient(th.GradientA, th.GradientB))
	p.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = th.Cursor

	return runnerModel{
		theme:    th,
		tasks:    tasks,
		status:   make([]taskStatus, len(tasks)),
		current:  -1,
		progress: p,
		spinner:  s,
		send:     send,
		width:    60,
	}
}

// Init starts the spinner ticking and emits a startMsg so the first task is
// launched from Update (Init must not mutate the model).
func (m runnerModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return startMsg{} },
	)
}

// startNext advances to the next task and launches it. Pointer receiver: it
// mutates the model, so callers must use it on an addressable value.
func (m *runnerModel) startNext() tea.Cmd {
	next := m.current + 1
	if next >= len(m.tasks) {
		m.current = len(m.tasks)
		m.done = true
		return func() tea.Msg { return allDoneMsg{err: m.err} }
	}

	m.current = next
	m.status[next] = statusRunning
	task := m.tasks[next]
	send := m.send
	ctx := Context{send: send, idx: next}

	// Run the task in its own goroutine so the UI stays responsive. It reports
	// its result back through the program via Send. We capture send (not the
	// model) so the goroutine holds no reference to this runnerModel.
	runCmd := func() tea.Msg {
		go func() {
			var err error
			if task.Run != nil {
				err = task.Run(ctx)
			}
			if send != nil {
				send(taskDoneMsg{index: next, err: err})
			}
		}()
		return nil
	}

	// Reset the bar to 0 for the new task and kick off the worker.
	return tea.Batch(m.progress.SetPercent(0), runCmd)
}

func (m runnerModel) Update(msg tea.Msg) (runnerModel, tea.Cmd) {
	switch msg := msg.(type) {

	case startMsg:
		// Sequence the mutation before returning m: in `return m, m.startNext()`
		// the copy of m and the pointer-receiver mutation are evaluated in an
		// unspecified order, so the mutation could be lost.
		cmd := m.startNext()
		return m, cmd

	case progressMsg:
		if msg.index == m.current {
			cmd := m.progress.SetPercent(msg.fraction)
			return m, cmd
		}
		return m, nil

	case taskDoneMsg:
		// Ignore stale completions from a task that is no longer current.
		if msg.index != m.current {
			return m, nil
		}
		if msg.err != nil {
			m.status[msg.index] = statusFailed
			m.err = msg.err
			m.done = true
			return m, func() tea.Msg { return allDoneMsg{err: msg.err} }
		}
		m.status[msg.index] = statusDone
		// Move on. The next task resets the bar to 0 itself.
		cmd := m.startNext()
		return m, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m runnerModel) View() string {
	th := m.theme
	var b strings.Builder

	for i, t := range m.tasks {
		switch m.status[i] {
		case statusDone:
			b.WriteString(th.SuccessTxt.Render("✓ ") + th.Item.Render(t.Title) + "\n")

		case statusFailed:
			b.WriteString(th.ErrorTxt.Render("✗ ") + th.Item.Render(t.Title) + "\n")

		case statusRunning:
			if t.Indeterminate {
				b.WriteString(m.spinner.View() + " " + th.Heading.Render(t.Title) + "\n")
			} else {
				b.WriteString(th.Cursor.Render("▸ ") + th.Heading.Render(t.Title) + "\n")
				b.WriteString("   " + m.progress.View() + "\n")
			}

		default: // statusPending
			b.WriteString(th.Hint.Render("  "+t.Title) + "\n")
		}
	}

	return b.String()
}
