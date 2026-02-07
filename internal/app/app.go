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
	drag    window.DragState
	nextWID int
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
		m.wm.SetReserved(0, 0) // will be 1,1 when menu bar and dock are added
		m.ready = true

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClick(tea.Mouse(msg))

	case tea.MouseMotionMsg:
		return m.handleMouseMotion(tea.Mouse(msg))

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease()
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keybindings (always handled regardless of focused window)
	switch key {
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit
	case "ctrl+n":
		m.openDemoWindow()
		return m, nil
	case "alt+tab":
		m.wm.CycleForward()
		return m, nil
	case "alt+shift+tab":
		m.wm.CycleBackward()
		return m, nil
	case "ctrl+w":
		if fw := m.wm.FocusedWindow(); fw != nil {
			m.wm.RemoveWindow(fw.ID)
		}
		return m, nil
	case "ctrl+left":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapLeft(fw, m.wm.WorkArea())
		}
		return m, nil
	case "ctrl+right":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapRight(fw, m.wm.WorkArea())
		}
		return m, nil
	case "ctrl+up":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.Maximize(fw, m.wm.WorkArea())
		}
		return m, nil
	case "ctrl+down":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.Restore(fw)
		}
		return m, nil
	case "ctrl+t":
		window.TileAll(m.wm.Windows(), m.wm.WorkArea())
		return m, nil
	}

	// If no window is focused, allow q to quit
	if m.wm.FocusedWindow() == nil && key == "q" {
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleMouseClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	p := geometry.Point{X: mouse.X, Y: mouse.Y}

	// Find which window was clicked
	w := m.wm.WindowAt(p)
	if w == nil {
		return m, nil
	}

	// Focus the clicked window
	m.wm.FocusWindow(w.ID)

	// Determine what was clicked
	zone := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)

	switch zone {
	case window.HitCloseButton:
		m.wm.RemoveWindow(w.ID)
		return m, nil

	case window.HitMaxButton:
		window.ToggleMaximize(w, m.wm.WorkArea())
		return m, nil

	case window.HitTitleBar, window.HitBorderN, window.HitBorderS, window.HitBorderE,
		window.HitBorderW, window.HitBorderNE, window.HitBorderNW,
		window.HitBorderSE, window.HitBorderSW:
		dragMode := window.DragModeForZone(zone)
		if dragMode != window.DragNone && (w.Draggable || zone != window.HitTitleBar) {
			m.drag = window.DragState{
				Active:     true,
				WindowID:   w.ID,
				Mode:       dragMode,
				StartMouse: p,
				StartRect:  w.Rect,
			}
		}
	}

	return m, nil
}

func (m Model) handleMouseMotion(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	if !m.drag.Active {
		return m, nil
	}

	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	w := m.wm.WindowByID(m.drag.WindowID)
	if w == nil {
		m.drag = window.DragState{}
		return m, nil
	}

	// Un-maximize on drag move
	if m.drag.Mode == window.DragMove && w.IsMaximized() {
		window.Restore(w)
		m.drag.StartRect = w.Rect
	}

	newRect := window.ApplyDrag(m.drag, p, m.wm.WorkArea())
	w.Rect = newRect

	return m, nil
}

func (m Model) handleMouseRelease() (tea.Model, tea.Cmd) {
	m.drag = window.DragState{}
	return m, nil
}

func (m *Model) openDemoWindow() {
	m.nextWID++
	id := fmt.Sprintf("win-%d", m.nextWID)
	title := fmt.Sprintf("Window %d", m.nextWID)

	wa := m.wm.WorkArea()
	offset := (m.nextWID - 1) % 10
	x := wa.X + 2 + offset*2
	y := wa.Y + 1 + offset
	w := min(60, wa.Width-x)
	h := min(15, wa.Height-y)
	if w < window.MinWindowWidth {
		w = window.MinWindowWidth
	}
	if h < window.MinWindowHeight {
		h = window.MinWindowHeight
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
