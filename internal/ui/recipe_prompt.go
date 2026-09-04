package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type RecipeDetails struct {
	ServerName string
	MaxClients string
	Port       string
}

type RecipePromptModel struct {
	inputs   []textinput.Model
	focused  int
	canceled bool
	done     bool
}

func NewRecipePromptModel(defaultRecipeName string) RecipePromptModel {
	m := RecipePromptModel{
		inputs: make([]textinput.Model, 3),
	}

	// 0: serverName
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Server name"
	m.inputs[0].SetValue("A cool server")
	m.inputs[0].Prompt = "Name: "
	m.inputs[0].Focus()

	// 1: maxClients
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "00"
	m.inputs[1].SetValue("48")
	m.inputs[1].Prompt = "Max Clients: "

	// 3: tcp/udp game port
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "30120"
	m.inputs[2].SetValue("30120")
	m.inputs[2].Prompt = "Game Port: "

	return m
}

func (m RecipePromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m RecipePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *RecipePromptModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m RecipePromptModel) Done() bool { return m.canceled || m.done }

func (m RecipePromptModel) View() string {
	var b strings.Builder
	b.WriteString("\n📋Data required by Recipe\n")
	b.WriteString("Please configure following fields:\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View() + "\n")
	}

	b.WriteString("\n(Use Tab / Enter to navigate fields, Enter on last field to submit)\n")
	return b.String()
}

func (m RecipePromptModel) Details() RecipeDetails {
	clients, _ := strconv.Atoi(m.inputs[1].Value())
	if clients <= 0 {
		clients = 48
	}

	port, _ := strconv.Atoi(m.inputs[2].Value())
	if port <= 5000 {
		port = 30120
	}

	return RecipeDetails{
		ServerName: m.inputs[0].Value(),
		MaxClients: strconv.Itoa(clients),
		Port:       strconv.Itoa(port),
	}
}

func PromptRecipeDetails(ctx Context, defaultRecipe string) (RecipeDetails, bool, error) {
	final := ctx.Ask(NewRecipePromptModel(defaultRecipe))
	model := final.(RecipePromptModel)

	if model.canceled {
		return RecipeDetails{}, false, nil
	}

	return model.Details(), true, nil
}
