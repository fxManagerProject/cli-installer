package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type DBCredentials struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
}

type DBPromptModel struct {
	inputs   []textinput.Model
	focused  int
	canceled bool
	done     bool
}

func NewDBPromptModel(defaultDBName string) DBPromptModel {
	m := DBPromptModel{
		inputs: make([]textinput.Model, 5),
	}

	// 0: Host
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "127.0.0.1"
	m.inputs[0].SetValue("127.0.0.1")
	m.inputs[0].Prompt = "Host: "
	m.inputs[0].Focus()

	// 1: Port
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "3306"
	m.inputs[1].SetValue("3306")
	m.inputs[1].Prompt = "Port: "

	// 2: Username
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "root"
	m.inputs[2].SetValue("root")
	m.inputs[2].Prompt = "Username: "

	// 3: Password
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "(none)"
	m.inputs[3].EchoMode = textinput.EchoPassword
	m.inputs[3].Prompt = "Password: "

	// 4: Database Name
	m.inputs[4] = textinput.New()
	m.inputs[4].Placeholder = defaultDBName
	m.inputs[4].SetValue(defaultDBName)
	m.inputs[4].Prompt = "Database Name: "

	return m
}

func (m DBPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m DBPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			return m, nil

		case "tab", "shift+tab", "down", "up":
			if key.String() == "up" || key.String() == "shift+tab" {
				m.focused--
				if m.focused < 0 {
					m.focused = len(m.inputs) - 1
				}
			} else {
				m.focused++
				if m.focused >= len(m.inputs) {
					m.focused = 0
				}
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focused {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)

		case "enter":
			if m.focused == len(m.inputs)-1 {
				m.done = true
				return m, nil
			}
			m.focused++
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i < len(m.inputs); i++ {
				if i == m.focused {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *DBPromptModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m DBPromptModel) Done() bool { return m.canceled || m.done }

func (m DBPromptModel) View() string {
	var b strings.Builder
	b.WriteString("\n📊 Database Setup Required by Recipe\n")
	b.WriteString("Please configure your MySQL/MariaDB connection:\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}

	b.WriteString("\n(Use Tab / Enter to navigate fields, Enter on last field to submit)\n")
	return b.String()
}

func (m DBPromptModel) Credentials() DBCredentials {
	port, _ := strconv.Atoi(m.inputs[1].Value())
	if port <= 0 {
		port = 3306
	}

	return DBCredentials{
		Host:     m.inputs[0].Value(),
		Port:     strconv.Itoa(port),
		Username: m.inputs[2].Value(),
		Password: m.inputs[3].Value(),
		Database: m.inputs[4].Value(),
	}
}

// PromptDatabaseCredentials pauses the runner screen and opens the DB config form.
func PromptDatabaseCredentials(ctx Context, defaultDB string) (DBCredentials, bool, error) {
	final := ctx.Ask(NewDBPromptModel(defaultDB))
	model := final.(DBPromptModel)

	if model.canceled {
		return DBCredentials{}, false, nil
	}

	return model.Credentials(), true, nil
}
