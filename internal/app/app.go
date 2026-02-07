package app

import (
	"fmt"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

const version = "0.1.0"

// Model is the root model for the termdesk application.
type Model struct {
	width   int
	height  int
	ready   bool
	wm      *window.Manager
	theme   config.Theme
	nextWID int // counter for generating window IDs
}

// New creates a new root Model.
func New() Model {
	return Model{
		wm:    window.NewManager(80, 24),
		theme: config.RetroTheme(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.wm.SetBounds(msg.Width, msg.Height)
		m.ready = true

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+n":
			m.openDemoWindow()
		}
	}

	return m, nil
}

func (m *Model) openDemoWindow() {
	m.nextWID++
	id := fmt.Sprintf("win-%d", m.nextWID)
	title := fmt.Sprintf("Window %d", m.nextWID)

	wa := m.wm.WorkArea()
	// Cascade position: offset each new window by 2,1
	offset := (m.nextWID - 1) % 10
	x := wa.X + 2 + offset*2
	y := wa.Y + 1 + offset
	w := min(60, wa.Width-x)
	h := min(15, wa.Height-y)
	if w < 10 {
		w = 10
	}
	if h < 5 {
		h = 5
	}

	win := window.NewWindow(id, title, geometry.Rect{X: x, Y: y, Width: w, Height: h}, nil)
	m.wm.AddWindow(win)
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion

	if !m.ready {
		v.SetContent("Starting termdesk...")
		return v
	}

	buf := RenderFrame(m.wm, m.theme)
	v.SetContent(BufferToString(buf))
	return v
}
