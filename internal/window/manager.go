package window

import (
	"sort"

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
	target.HasBell = false
	target.HasActivity = false
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

// FocusWindowNoRaise focuses a window without changing z-order.
func (m *Manager) FocusWindowNoRaise(id string) {
	found := false
	for _, w := range m.windows {
		if w.ID == id {
			w.Focused = true
			w.HasNotification = false
			w.HasBell = false
			w.HasActivity = false
			found = true
		} else {
			w.Focused = false
		}
	}
	if found {
		m.focused = id
	} else {
		m.focused = ""
	}
}

// FocusNextVisibleNoRaise focuses the topmost visible, non-minimized window
// excluding the given ID, without changing z-order.
func (m *Manager) FocusNextVisibleNoRaise(excludeID string) {
	for _, w := range m.windows {
		w.Focused = false
	}
	m.focused = ""
	for i := len(m.windows) - 1; i >= 0; i-- {
		w := m.windows[i]
		if w.ID != excludeID && w.Visible && !w.Minimized {
			w.Focused = true
			w.HasNotification = false
			m.focused = w.ID
			return
		}
	}
}

// TilingSlotOf returns the 0-based slot index of a window among visible,
// non-minimized, resizable windows in current z-order. Returns -1 if missing.
func (m *Manager) TilingSlotOf(id string) int {
	slot := 0
	for _, w := range m.windows {
		if !w.Visible || w.Minimized || !w.Resizable {
			continue
		}
		if w.ID == id {
			return slot
		}
		slot++
	}
	return -1
}

// PlaceWindowAtTilingSlot moves a window to the requested slot order among
// visible, non-minimized, resizable windows without otherwise changing state.
func (m *Manager) PlaceWindowAtTilingSlot(id string, slot int) {
	if slot < 0 {
		return
	}
	cur := -1
	var target *Window
	for i, w := range m.windows {
		if w.ID == id {
			cur = i
			target = w
			break
		}
	}
	if cur < 0 || target == nil {
		return
	}

	// Remove target from current position.
	m.windows = append(m.windows[:cur], m.windows[cur+1:]...)

	// Find insertion index in full list that corresponds to desired tiling slot.
	insertAt := len(m.windows)
	cand := 0
	for i, w := range m.windows {
		if !w.Visible || w.Minimized || !w.Resizable {
			continue
		}
		if cand == slot {
			insertAt = i
			break
		}
		cand++
	}

	m.windows = append(m.windows, nil)
	copy(m.windows[insertAt+1:], m.windows[insertAt:])
	m.windows[insertAt] = target
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
// Sends the current focused window to the bottom of the z-stack,
// then focuses the new topmost visible window. This ensures full
// cycling through all windows (FocusWindow brings to top, which
// would cause oscillation between only 2 windows if used directly).
func (m *Manager) CycleBackward() {
	visible := m.visibleWindows()
	if len(visible) < 2 {
		return
	}

	// Find current focused in the full window list
	idx := -1
	for i, w := range m.windows {
		if w.ID == m.focused {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}

	// Move focused window to front of slice (bottom of z-stack)
	w := m.windows[idx]
	copy(m.windows[1:idx+1], m.windows[:idx])
	m.windows[0] = w
	w.Focused = false

	// Focus the new topmost visible window (scan from end of z-stack)
	for i := len(m.windows) - 1; i >= 0; i-- {
		if m.windows[i].Visible && !m.windows[i].Minimized {
			m.windows[i].Focused = true
			m.windows[i].HasNotification = false
			m.windows[i].HasActivity = false
			m.windows[i].ZIndex = m.nextZIndex
			m.nextZIndex++
			m.focused = m.windows[i].ID
			return
		}
	}
}

// ClampAllWindows repositions and resizes all windows to fit within the
// current work area. Called after a terminal resize to prevent windows
// from being off-screen.
func (m *Manager) ClampAllWindows() {
	wa := m.WorkArea()
	for _, w := range m.windows {
		if w.Minimized {
			continue
		}
		if w.IsMaximized() {
			w.Rect = wa
			continue
		}
		if w.Rect.Width > wa.Width {
			w.Rect.Width = wa.Width
		}
		if w.Rect.Height > wa.Height {
			w.Rect.Height = wa.Height
		}
		if w.Rect.X+w.Rect.Width > wa.X+wa.Width {
			w.Rect.X = wa.X + wa.Width - w.Rect.Width
		}
		if w.Rect.Y+w.Rect.Height > wa.Y+wa.Height {
			w.Rect.Y = wa.Y + wa.Height - w.Rect.Height
		}
		if w.Rect.X < wa.X {
			w.Rect.X = wa.X
		}
		if w.Rect.Y < wa.Y {
			w.Rect.Y = wa.Y
		}
	}
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

// VisibleWindows returns visible, non-minimized windows in z-order (back to front).
func (m *Manager) VisibleWindows() []*Window {
	return m.visibleWindows()
}

// VisibleWindowsByID returns visible, non-minimized windows sorted by ID
// for a stable iteration order that doesn't change when z-order changes.
func (m *Manager) VisibleWindowsByID() []*Window {
	visible := m.visibleWindows()
	sort.Slice(visible, func(i, j int) bool {
		return visible[i].ID < visible[j].ID
	})
	return visible
}

// FocusedIndex returns the index of the focused window in the visible windows list,
// or -1 if no window is focused.
func (m *Manager) FocusedIndex() int {
	visible := m.visibleWindows()
	for i, w := range visible {
		if w.ID == m.focused {
			return i
		}
	}
	return -1
}
