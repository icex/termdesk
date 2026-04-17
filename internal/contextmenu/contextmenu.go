package contextmenu

import "unicode/utf8"

// KeyBindings holds the keybinding strings needed for context menu shortcuts.
type KeyBindings struct {
	NewTerminal string
	CloseWindow string
	Minimize    string
	Maximize    string
	SnapLeft    string
	SnapRight   string
	Center      string
	TileAll     string
	Cascade     string
	Settings    string
}

// Menu represents a right-click context menu.
type Menu struct {
	X, Y       int
	Items      []Item
	HoverIndex int
	Visible    bool
}

// Item is a single context menu entry.
type Item struct {
	Label    string
	Action   string
	Shortcut string
	Disabled bool // separator if Label == "─"
}

// InnerWidth returns the inner content width (max label + shortcut + padding, no border).
func (m *Menu) InnerWidth() int {
	maxW := 0
	for _, item := range m.Items {
		w := utf8.RuneCountInString(item.Label) + 2 // left padding
		if item.Shortcut != "" {
			w += utf8.RuneCountInString(item.Shortcut) + 2 // gap + shortcut
		}
		if w > maxW {
			maxW = w
		}
	}
	return maxW + 2 // right padding
}

// Width returns the total width including border.
func (m *Menu) Width() int {
	return m.InnerWidth() + 2
}

// Height returns the total height including border.
func (m *Menu) Height() int {
	return len(m.Items) + 2
}

// Contains reports whether the screen position (x, y) is inside the menu.
func (m *Menu) Contains(x, y int) bool {
	return x >= m.X && x < m.X+m.Width() &&
		y >= m.Y && y < m.Y+m.Height()
}

// DesktopMenu returns a context menu for the desktop background.
func DesktopMenu(x, y int, kb KeyBindings) *Menu {
	return &Menu{
		X: x, Y: y,
		Visible: true,
		Items: []Item{
			{Label: "\uf120  New Terminal", Shortcut: kb.NewTerminal, Action: "new_terminal"},
			{Label: "─", Disabled: true},
			{Label: "\uf009  Tile All", Shortcut: kb.TileAll, Action: "tile_all"},
			{Label: "\uf2d2  Cascade", Shortcut: kb.Cascade, Action: "cascade"},
			{Label: "─", Disabled: true},
			{Label: "\uf013  Settings", Shortcut: kb.Settings, Action: "settings"},
			{Label: "\uf05a  About", Action: "about"},
		},
	}
}

// TitleBarMenu returns a context menu for a window title bar.
// Items that require resizing are disabled when resizable is false.
func TitleBarMenu(x, y int, resizable, isSplit bool, kb KeyBindings) *Menu {
	items := []Item{
		{Label: "\uf00d  Close", Shortcut: kb.CloseWindow, Action: "close_window"},
		{Label: "\uf2d1  Minimize", Shortcut: kb.Minimize, Action: "minimize"},
		{Label: "\uf2d0  Maximize", Shortcut: kb.Maximize, Action: "maximize", Disabled: !resizable},
		{Label: "─", Disabled: true},
		{Label: "\ueb56  Split Horizontal", Shortcut: "Pfx+%", Action: "split_horizontal"},
		{Label: "\ueb57  Split Vertical", Shortcut: "Pfx+\"", Action: "split_vertical"},
	}
	if isSplit {
		items = append(items, Item{Label: "\uf00d  Close Pane", Shortcut: "Pfx+x", Action: "close_pane"})
	}
	items = append(items,
		Item{Label: "─", Disabled: true},
		Item{Label: "\uf104  Snap Left", Shortcut: kb.SnapLeft, Action: "snap_left", Disabled: !resizable},
		Item{Label: "\uf105  Snap Right", Shortcut: kb.SnapRight, Action: "snap_right", Disabled: !resizable},
		Item{Label: "\uf05b  Center", Shortcut: kb.Center, Action: "center", Disabled: !resizable},
	)
	return &Menu{
		X: x, Y: y,
		Visible: true,
		Items: items,
	}
}

// CopyModeMenu returns a context menu for copy mode (text selection).
func CopyModeMenu(x, y int) *Menu {
	return &Menu{
		X: x, Y: y,
		Visible: true,
		Items: []Item{
			{Label: "\uf0c5  Copy", Shortcut: "y", Action: "copy_selection"},
			{Label: "\uf0ea  Paste", Shortcut: "p", Action: "paste"},
			{Label: "─", Disabled: true},
			{Label: "\uf0b2  Select All", Action: "select_all"},
			{Label: "\uf12d  Clear Selection", Shortcut: "Esc", Action: "clear_selection"},
		},
	}
}

// DockItemMenu returns a context menu for a dock item (running/minimized window).
func DockItemMenu(x, y int) *Menu {
	return &Menu{
		X: x, Y: y,
		Visible: true,
		Items: []Item{
			{Label: "\uf06e  Focus", Action: "dock_focus_window"},
			{Label: "\uf2d1  Minimize", Action: "minimize"},
			{Label: "\uf2d0  Maximize", Action: "maximize"},
			{Label: "─", Disabled: true},
			{Label: "\uf00d  Close", Action: "close_window"},
		},
	}
}

// Clamp adjusts the menu position to fit within the given screen bounds.
func (m *Menu) Clamp(screenW, screenH int) {
	w := m.Width()
	h := m.Height()
	if m.X+w > screenW {
		m.X = screenW - w
	}
	if m.Y+h > screenH {
		m.Y = screenH - h
	}
	if m.X < 0 {
		m.X = 0
	}
	if m.Y < 1 {
		m.Y = 1
	}
}

// Hide dismisses the menu.
func (m *Menu) Hide() {
	m.Visible = false
}

// MoveHover moves the hover index by delta, skipping disabled items.
func (m *Menu) MoveHover(delta int) {
	if len(m.Items) == 0 {
		return
	}
	n := len(m.Items)
	for range n {
		m.HoverIndex += delta
		if m.HoverIndex < 0 {
			m.HoverIndex = n - 1
		}
		if m.HoverIndex >= n {
			m.HoverIndex = 0
		}
		if !m.Items[m.HoverIndex].Disabled {
			return
		}
	}
}

// SelectedAction returns the action of the currently hovered item, or "".
func (m *Menu) SelectedAction() string {
	if m.HoverIndex < 0 || m.HoverIndex >= len(m.Items) {
		return ""
	}
	item := m.Items[m.HoverIndex]
	if item.Disabled {
		return ""
	}
	return item.Action
}

// ItemAtY returns the item index at a relative Y offset, or -1.
func (m *Menu) ItemAtY(relY int) int {
	if relY < 0 || relY >= len(m.Items) {
		return -1
	}
	return relY
}
