package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fxManagerProject/cli-installer/internal/config"
	"github.com/fxManagerProject/cli-installer/internal/theme"
)

// selectorModel is a lightweight vertical list selector for one parameter.
// (Kept custom on purpose so it is trivial to restyle. If you need filtering
// or pagination for very long lists, swap this for bubbles/list.)
type selectorModel struct {
	theme  theme.Theme
	param  config.Param
	cursor int
	chosen bool // set to true once the user presses enter
	width  int
}

func newSelector(th theme.Theme, p config.Param) selectorModel {
	// Pre-select the option matching Default, if any.
	cursor := 0
	for i, o := range p.Options {
		if o.Value == p.Default {
			cursor = i
			break
		}
	}
	return selectorModel{theme: th, param: p, cursor: cursor, width: 60}
}

// Value returns the currently highlighted option's value.
func (m selectorModel) Value() string {
	n := len(m.param.Options)
	if n == 0 || m.cursor < 0 || m.cursor >= n {
		return ""
	}
	return m.param.Options[m.cursor].Value
}

func (m selectorModel) Update(msg tea.Msg) (selectorModel, tea.Cmd) {
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
