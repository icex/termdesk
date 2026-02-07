package app

import (
	"fmt"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

const version = "0.1.0"

// Model is the root model for the termdesk application.
type Model struct {
	width     int
	height    int
	ready     bool
	wm        *window.Manager
	theme     config.Theme
	drag      window.DragState
	nextWID   int
	terminals map[string]*terminal.Terminal
}

// New creates a new root Model.
func New() Model {
	return Model{
		wm:        window.NewManager(80, 24),
		theme:     config.RetroTheme(),
		terminals: make(map[string]*terminal.Terminal),
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
		m.resizeAllTerminals()

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClick(tea.Mouse(msg))

	case tea.MouseMotionMsg:
		return m.handleMouseMotion(tea.Mouse(msg))

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease()

	case PtyOutputMsg:
		// PTY produced output — schedule a redraw and continue reading
		return m, m.schedulePtyRead(msg.WindowID)

	case PtyClosedMsg:
		// PTY exited — optionally close the window
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keybindings (always handled regardless of focused window)
	switch key {
	case "ctrl+c", "ctrl+q":
		m.closeAllTerminals()
		return m, tea.Quit
	case "ctrl+n":
		cmd := m.openTerminalWindow()
		return m, cmd
	case "alt+tab":
		m.wm.CycleForward()
		return m, nil
	case "alt+shift+tab":
		m.wm.CycleBackward()
		return m, nil
	case "ctrl+w":
		if fw := m.wm.FocusedWindow(); fw != nil {
			m.closeTerminal(fw.ID)
			m.wm.RemoveWindow(fw.ID)
		}
		return m, nil
	case "ctrl+left":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapLeft(fw, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil
	case "ctrl+right":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapRight(fw, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil
	case "ctrl+up":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.Maximize(fw, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil
	case "ctrl+down":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.Restore(fw)
			m.resizeTerminalForWindow(fw)
		}
		return m, nil
	case "ctrl+t":
		window.TileAll(m.wm.Windows(), m.wm.WorkArea())
		m.resizeAllTerminals()
		return m, nil
	}

	// Forward keys to the focused terminal
	if fw := m.wm.FocusedWindow(); fw != nil {
		if term, ok := m.terminals[fw.ID]; ok {
			k := tea.Key(msg)
			term.SendKey(k.Code, k.Mod, k.Text)
			return m, nil
		}
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
		m.closeTerminal(w.ID)
		m.wm.RemoveWindow(w.ID)
		return m, nil

	case window.HitMaxButton:
		window.ToggleMaximize(w, m.wm.WorkArea())
		m.resizeTerminalForWindow(w)
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
	if m.drag.Active {
		// Resize terminal after drag completes
		w := m.wm.WindowByID(m.drag.WindowID)
		if w != nil {
			m.resizeTerminalForWindow(w)
		}
	}
	m.drag = window.DragState{}
	return m, nil
}

// openTerminalWindow creates a new window with an embedded terminal.
func (m *Model) openTerminalWindow() tea.Cmd {
	m.nextWID++
	id := fmt.Sprintf("win-%d", m.nextWID)
	title := fmt.Sprintf("Terminal %d", m.nextWID)

	wa := m.wm.WorkArea()
	offset := (m.nextWID - 1) % 10
	x := wa.X + 2 + offset*2
	y := wa.Y + 1 + offset
	w := min(80, wa.Width-x)
	h := min(24, wa.Height-y)
	if w < window.MinWindowWidth {
		w = window.MinWindowWidth
	}
	if h < window.MinWindowHeight {
		h = window.MinWindowHeight
	}

	win := window.NewWindow(id, title, geometry.Rect{X: x, Y: y, Width: w, Height: h}, nil)
	m.wm.AddWindow(win)

	// Content area is window minus borders (1 cell each side, 1 row top, 1 row bottom)
	contentW := w - 2
	contentH := h - 2
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	term, err := terminal.NewShell(contentW, contentH)
	if err != nil {
		// Fall back to a window with no terminal
		return nil
	}

	m.terminals[id] = term

	// Start reading PTY output in background
	return m.startPtyReader(id, term)
}

// openDemoWindow creates a demo window without a terminal (for testing).
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

// startPtyReader returns a Cmd that reads PTY output and sends messages.
func (m *Model) startPtyReader(windowID string, term *terminal.Terminal) tea.Cmd {
	return func() tea.Msg {
		err := term.ReadPtyLoop()
		return PtyClosedMsg{WindowID: windowID, Err: err}
	}
}

// schedulePtyRead returns a Cmd for a brief tick then re-triggers a PtyOutputMsg.
// This creates a render loop for terminal windows.
func (m *Model) schedulePtyRead(windowID string) tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(time.Time) tea.Msg {
		return PtyOutputMsg{WindowID: windowID}
	})
}

// closeTerminal closes and removes a terminal by window ID.
func (m *Model) closeTerminal(windowID string) {
	if term, ok := m.terminals[windowID]; ok {
		term.Close()
		delete(m.terminals, windowID)
	}
}

// closeAllTerminals closes all active terminals.
func (m *Model) closeAllTerminals() {
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

// resizeTerminalForWindow resizes the terminal to match its window's content area.
func (m *Model) resizeTerminalForWindow(w *window.Window) {
	if term, ok := m.terminals[w.ID]; ok {
		cr := w.ContentRect()
		if cr.Width > 0 && cr.Height > 0 {
			term.Resize(cr.Width, cr.Height)
		}
	}
}

// resizeAllTerminals resizes all terminals to match their windows.
func (m *Model) resizeAllTerminals() {
	for _, w := range m.wm.Windows() {
		m.resizeTerminalForWindow(w)
	}
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion

	if !m.ready {
		v.SetContent("Starting termdesk...")
		return v
	}

	buf := RenderFrame(m.wm, m.theme, m.terminals)
	v.SetContent(BufferToString(buf))
	return v
}
