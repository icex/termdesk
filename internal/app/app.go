package app

import (
	tea "charm.land/bubbletea/v2"
)

const version = "0.1.0"

// Model is the root model for the termdesk application.
type Model struct {
	width  int
	height int
	ready  bool
}

// New creates a new root Model.
func New() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion

	if !m.ready {
		v.SetContent("Starting termdesk...")
		return v
	}

	v.SetContent("")
	return v
}
