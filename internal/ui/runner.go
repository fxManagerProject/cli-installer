package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/theme"
)

type confirmModel interface {
	tea.Model
	Done() bool
}

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

// Ask hands control of the screen to m and blocks the calling task goroutine
// until the user has answered it. Safe to call from the task's own goroutine,
// same as Report.
func (c Context) Ask(m confirmModel) confirmModel {
	if c.send == nil {
		return m
	}
	reply := make(chan confirmModel, 1)
	c.send(askRequestMsg{model: m, reply: reply})
	return <-reply
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

	// subTasksInitMsg is sent once a task discovers its sub-steps at runtime
	// (e.g. after parsing a recipe's yaml). It replaces any prior sub-task
	// list for that task.
	subTasksInitMsg struct {
		taskIndex int
		titles    []string
	}
	subTaskStartMsg struct {
		taskIndex int
		subIndex  int
	}
	subTaskProgressMsg struct {
		taskIndex int
		subIndex  int
		fraction  float64
	}
	subTaskDoneMsg struct {
		taskIndex int
		subIndex  int
		err       error
	}
)

// --- sub tasks ------------------------------------------------------------

// SetSubTasks declares the sub-steps of the current task once they're known
// (e.g. after a recipe's yaml has been parsed). Call this before reporting
// any sub-task progress. Safe to call from the task's own goroutine.
func (c Context) SetSubTasks(titles []string) {
	if c.send == nil {
		return
	}
	c.send(subTasksInitMsg{taskIndex: c.idx, titles: titles})
}

// SubTaskStarted marks sub-task i as running and resets its progress bar.
func (c Context) SubTaskStarted(i int) {
	if c.send == nil {
		return
	}
	c.send(subTaskStartMsg{taskIndex: c.idx, subIndex: i})
}

// SubTaskProgress updates the progress bar for the currently running
// sub-task. fraction is clamped to [0, 1]. For a step with no measurable
// progress (a move, a query, etc.), just skip calling this.
func (c Context) SubTaskProgress(i int, fraction float64) {
	if c.send == nil {
		return
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	c.send(subTaskProgressMsg{taskIndex: c.idx, subIndex: i, fraction: fraction})
}

// SubTaskDone marks sub-task i finished. Pass a non-nil err to mark it failed.
func (c Context) SubTaskDone(i int, err error) {
	if c.send == nil {
		return
	}
	c.send(subTaskDoneMsg{taskIndex: c.idx, subIndex: i, err: err})
}

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
	current  int
	progress progress.Model
	spinner  spinner.Model
	send     func(tea.Msg)
	width    int
	done     bool
	err      error

	// sub-task state, keyed by top-level task index. subTitles[i] is nil
	// until that task calls SetSubTasks; tasks that never do have no
	// sub-task rendering at all.
	subTitles   [][]string
	subStatus   [][]taskStatus
	subCurrent  []int // running sub-index per task, -1 if none running
	subProgress progress.Model
}

func newRunner(th theme.Theme, tasks []Task, send func(tea.Msg)) runnerModel {
	p := progress.New(progress.WithGradient(th.GradientA, th.GradientB))
	p.Width = 40

	sp := progress.New(progress.WithGradient(th.GradientA, th.GradientB))
	sp.Width = 30

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = th.Cursor

	subCurrent := make([]int, len(tasks))
	for i := range subCurrent {
		subCurrent[i] = -1
	}

	return runnerModel{
		theme:       th,
		tasks:       tasks,
		status:      make([]taskStatus, len(tasks)),
		current:     -1,
		progress:    p,
		spinner:     s,
		send:        send,
		width:       60,
		subTitles:   make([][]string, len(tasks)),
		subStatus:   make([][]taskStatus, len(tasks)),
		subCurrent:  subCurrent,
		subProgress: sp,
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

		spm, spCmd := m.subProgress.Update(msg)
		m.subProgress = spm.(progress.Model)

		return m, tea.Batch(cmd, spCmd)

	case subTasksInitMsg:
		if msg.taskIndex != m.current {
			return m, nil
		}
		m.subTitles[msg.taskIndex] = msg.titles
		m.subStatus[msg.taskIndex] = make([]taskStatus, len(msg.titles))
		return m, nil

	case subTaskStartMsg:
		if msg.taskIndex != m.current {
			return m, nil
		}
		if msg.subIndex < 0 || msg.subIndex >= len(m.subStatus[msg.taskIndex]) {
			return m, nil
		}
		m.subStatus[msg.taskIndex][msg.subIndex] = statusRunning
		m.subCurrent[msg.taskIndex] = msg.subIndex
		cmd := m.subProgress.SetPercent(0)
		return m, cmd

	case subTaskProgressMsg:
		if msg.taskIndex != m.current || msg.subIndex != m.subCurrent[msg.taskIndex] {
			return m, nil
		}
		cmd := m.subProgress.SetPercent(msg.fraction)
		return m, cmd

	case subTaskDoneMsg:
		if msg.taskIndex != m.current {
			return m, nil
		}
		if msg.subIndex < 0 || msg.subIndex >= len(m.subStatus[msg.taskIndex]) {
			return m, nil
		}
		if msg.err != nil {
			m.subStatus[msg.taskIndex][msg.subIndex] = statusFailed
		} else {
			m.subStatus[msg.taskIndex][msg.subIndex] = statusDone
		}
		if m.subCurrent[msg.taskIndex] == msg.subIndex {
			m.subCurrent[msg.taskIndex] = -1
		}
		return m, nil
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
			m.writeSubTasks(&b, th, i)

		default: // statusPending
			b.WriteString(th.Hint.Render("  "+t.Title) + "\n")
		}
	}

	return b.String()
}

// writeSubTasks renders task i's sub-steps, indented beneath it. Limits rendering
// to the active step and up to 4 upcoming steps to keep the UI compact.
func (m runnerModel) writeSubTasks(b *strings.Builder, th theme.Theme, i int) {
	titles := m.subTitles[i]
	if len(titles) == 0 {
		return
	}
	statuses := m.subStatus[i]

	startIdx := m.subCurrent[i]
	if startIdx == -1 {
		for j, status := range statuses {
			if status == statusRunning || status == statusPending {
				startIdx = j
				break
			}
		}
		if startIdx == -1 {
			startIdx = len(titles) - 1
		}
	}

	// limit window to 5 steps (active + next 4)
	const maxDisplayed = 5
	endIdx := startIdx + maxDisplayed
	if endIdx > len(titles) {
		endIdx = len(titles)
	}

	for j := startIdx; j < endIdx; j++ {
		title := titles[j]
		switch statuses[j] {
		case statusDone:
			b.WriteString("    " + th.SuccessTxt.Render("✓ ") + th.Hint.Render(title) + "\n")
		case statusFailed:
			b.WriteString("    " + th.ErrorTxt.Render("✗ ") + th.Hint.Render(title) + "\n")
		case statusRunning:
			b.WriteString("    " + m.spinner.View() + " " + th.Item.Render(title) + "\n")
		default:
			b.WriteString("    " + th.Hint.Render("  "+title) + "\n")
		}
	}

	// Show remaining indicator if there are more upcoming steps
	if remaining := len(titles) - endIdx; remaining > 0 {
		b.WriteString("    " + th.Hint.Render("  ... ("+strconv.Itoa(remaining)+" more step(s))") + "\n")
	}
}
