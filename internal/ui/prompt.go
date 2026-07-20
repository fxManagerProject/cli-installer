package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

// selectorModel is a lightweight vertical list selector for one parameter.
// (Kept custom on purpose so it is trivial to restyle. If you need filtering
// or pagination for very long lists, swap this for bubbles/list.)
type selectorModel struct {
	theme theme.Theme
	param config.Param

	// List selector state
	cursor int

	// Text input state
	input textinput.Model

	chosen bool // set to true once the user submits their choice/input with enter
	width  int
}

func newSelector(th theme.Theme, p config.Param) selectorModel {
	m := selectorModel{
		theme: th,
		param: p,
		width: 60,
	}

	if p.IsInput() {
		ti := textinput.New()
		ti.Placeholder = p.Default
		ti.SetValue(p.Default)
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = 50
		m.input = ti
	} else if p.IsList() {
		// Pre-select option matching Default, if any.
		for i, o := range p.Options {
			if o.Value == p.Default {
				m.cursor = i
				break
			}
		}
	}

	return m
}

// Init starts component commands, such as cursor blinking for text inputs.
func (m selectorModel) Init() tea.Cmd {
	if m.param.IsInput() {
		return textinput.Blink
	}
	return nil
}

// Value returns the resolved parameter string (either typed text or selected list value).
func (m selectorModel) Value() string {
	if m.param.IsInput() {
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			return m.param.Default
		}
		return val
	}

	n := len(m.param.Options)
	if n == 0 || m.cursor < 0 || m.cursor >= n {
		return ""
	}
	return m.param.Options[m.cursor].Value
}

func (m selectorModel) Update(msg tea.Msg) (selectorModel, tea.Cmd) {
	// 1. Text Input Update Loop
	if m.param.IsInput() {
		if key, ok := msg.(tea.KeyMsg); ok {
			if key.String() == "enter" {
				m.chosen = true
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// 2. List Selector Update Loop
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	n := len(m.param.Options)
	switch key.String() {
	case "up", "k":
		if n > 0 {
			m.cursor = (m.cursor - 1 + n) % n
		}
	case "down", "j":
		if n > 0 {
			m.cursor = (m.cursor + 1) % n
		}
	case "enter":
		m.chosen = true
	}
	return m, nil
}

func (m selectorModel) View() string {
	th := m.theme
	var b strings.Builder

	b.WriteString(th.Heading.Render(m.param.Usage))
	b.WriteString("\n\n")

	// Render Text Input
	if m.param.IsInput() {
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString(th.Hint.Render("enter confirm · esc cancel"))
		return b.String()
	}

	// Render List Selector
	for i, o := range m.param.Options {
		title := o.Title
		if title == "" {
			title = o.Value
		}
		if i == m.cursor {
			b.WriteString(th.Cursor.Render("❯ ") + th.Selected.Render(title) + "\n")
			if o.Desc != "" {
				b.WriteString("    " + th.SelectedDesc.Render(o.Desc) + "\n")
			}
		} else {
			b.WriteString("  " + th.Item.Render(title) + "\n")
			if o.Desc != "" {
				b.WriteString("    " + th.ItemDesc.Render(o.Desc) + "\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(th.Hint.Render("↑/↓ move · enter select · esc cancel"))
	return b.String()
}
