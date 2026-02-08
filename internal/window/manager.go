package window

import (
	"github.com/icex/termdesk/pkg/geometry"
)

// Manager manages all windows with z-ordering, focus, and hit testing.
type Manager struct {
	windows    []*Window // ordered by z-index (back to front)
	focused    string    // ID of focused window
	nextZIndex int
	bounds     geometry.Rect // available screen area
	reservedT  int           // reserved rows at top (menu bar)
	reservedB  int           // reserved rows at bottom (dock)
}

// NewManager creates a new window manager with the given screen bounds.
func NewManager(width, height int) *Manager {
	return &Manager{
		bounds: geometry.Rect{X: 0, Y: 0, Width: width, Height: height},
	}
}

// SetBounds updates the screen size.
func (m *Manager) SetBounds(width, height int) {
	m.bounds = geometry.Rect{X: 0, Y: 0, Width: width, Height: height}
}

// SetReserved sets the number of reserved rows at top and bottom.
func (m *Manager) SetReserved(top, bottom int) {
	m.reservedT = top
	m.reservedB = bottom
}

// ReservedBottom returns the number of reserved rows at the bottom.
func (m *Manager) ReservedBottom() int {
	return m.reservedB
}

// WorkArea returns the usable area after reserved space.
func (m *Manager) WorkArea() geometry.Rect {
	return geometry.Rect{
		X:      0,
		Y:      m.reservedT,
		Width:  m.bounds.Width,
		Height: m.bounds.Height - m.reservedT - m.reservedB,
	}
}

// AddWindow adds a window to the manager and brings it to front.
func (m *Manager) AddWindow(w *Window) {
	w.ZIndex = m.nextZIndex
	m.nextZIndex++
	m.windows = append(m.windows, w)
	m.FocusWindow(w.ID)
}

// RemoveWindow removes a window by ID.
func (m *Manager) RemoveWindow(id string) {
	for i, w := range m.windows {
		if w.ID == id {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			if m.focused == id {
				m.focused = ""
				// Focus the topmost remaining window
				if len(m.windows) > 0 {
					m.focused = m.windows[len(m.windows)-1].ID
					m.windows[len(m.windows)-1].Focused = true
				}
			}
			return
		}
	}
}

// FocusWindow brings a window to the front and sets it as focused.
func (m *Manager) FocusWindow(id string) {
	var target *Window
	targetIdx := -1

	for i, w := range m.windows {
		if w.ID == id {
			target = w
			targetIdx = i
		}
		w.Focused = false
	}

	if target == nil {
		return
	}

	// Move to end of slice (top of z-stack)
	if targetIdx < len(m.windows)-1 {
		m.windows = append(m.windows[:targetIdx], m.windows[targetIdx+1:]...)
		m.windows = append(m.windows, target)
	}

	target.Focused = true
	target.HasNotification = false
	target.ZIndex = m.nextZIndex
	m.nextZIndex++
	m.focused = id
}

// FocusNextVisible focuses the topmost visible, non-minimized window
// excluding the given ID. If none exist, all windows are unfocused.
func (m *Manager) FocusNextVisible(excludeID string) {
	// Unfocus all first
	for _, w := range m.windows {
		w.Focused = false
	}
	m.focused = ""
	// Walk back-to-front (topmost = last) to find the best candidate
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.ID != excludeID && w.Visible && !w.Minimized {
			m.FocusWindow(w.ID)
			return
		}
	}
}

// FocusedWindow returns the currently focused window, or nil.
func (m *Manager) FocusedWindow() *Window {
	for _, w := range m.windows {
		if w.ID == m.focused {
			return w
		}
	}
	return nil
}

// WindowAt returns the topmost visible window at the given point, or nil.
// Iterates front-to-back (end of slice first).
func (m *Manager) WindowAt(p geometry.Point) *Window {
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.Visible && !w.Minimized && w.Rect.Contains(p) {
			return w
		}
	}
	return nil
}

// Windows returns all windows in z-order (back to front) for rendering.
func (m *Manager) Windows() []*Window {
	return m.windows
}

// WindowByID returns a window by its ID, or nil.
func (m *Manager) WindowByID(id string) *Window {
	for _, w := range m.windows {
		if w.ID == id {
			return w
		}
	}
	return nil
}

// Count returns the number of windows.
func (m *Manager) Count() int {
	return len(m.windows)
}

// VisibleCount returns the number of visible, non-minimized windows.
func (m *Manager) VisibleCount() int {
	count := 0
	for _, w := range m.windows {
		if w.Visible && !w.Minimized {
			count++
		}
	}
	return count
}

// CycleForward focuses the next window in z-order.
func (m *Manager) CycleForward() {
	visible := m.visibleWindows()
	if len(visible) < 2 {
		return
	}

	currentIdx := -1
	for i, w := range visible {
		if w.ID == m.focused {
			currentIdx = i
			break
		}
	}

	nextIdx := 0
	if currentIdx >= 0 {
		nextIdx = (currentIdx + 1) % len(visible)
	}
	m.FocusWindow(visible[nextIdx].ID)
}

// CycleBackward focuses the previous window in z-order.
func (m *Manager) CycleBackward() {
	visible := m.visibleWindows()
	if len(visible) < 2 {
		return
	}

	currentIdx := -1
	for i, w := range visible {
		if w.ID == m.focused {
			currentIdx = i
			break
		}
	}

	prevIdx := len(visible) - 1
	if currentIdx >= 0 {
		prevIdx = (currentIdx - 1 + len(visible)) % len(visible)
	}
	m.FocusWindow(visible[prevIdx].ID)
}

func (m *Manager) visibleWindows() []*Window {
	var result []*Window
	for _, w := range m.windows {
		if w.Visible && !w.Minimized {
			result = append(result, w)
		}
	}
	return result
}
