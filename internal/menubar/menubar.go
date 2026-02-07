package menubar

import (
	"fmt"
	"strings"
	"time"
)

// MenuItem represents a single item in a dropdown menu.
type MenuItem struct {
	Label    string
	Shortcut string // e.g., "Ctrl+N"
	Action   string // action identifier
	Disabled bool
}

// Menu represents a top-level menu with a dropdown.
type Menu struct {
	Label string
	Items []MenuItem
}

// MenuBar represents the top menu bar of the desktop.
type MenuBar struct {
	Menus      []Menu
	OpenIndex  int  // -1 = no menu open
	HoverIndex int  // highlighted item in open menu, -1 = none
	Width      int  // total width available
	ShowClock  bool
	ShowCPU    bool
	ShowMemory bool
}

// New creates a menu bar with default menus.
func New(width int) *MenuBar {
	return &MenuBar{
		Menus: []Menu{
			{Label: "File", Items: []MenuItem{
				{Label: "New Terminal", Shortcut: "Ctrl+N", Action: "new_terminal"},
				{Label: "Quit", Shortcut: "Ctrl+Q", Action: "quit"},
			}},
			{Label: "Edit", Items: []MenuItem{
				{Label: "Copy", Shortcut: "Ctrl+C", Action: "copy", Disabled: true},
				{Label: "Paste", Shortcut: "Ctrl+V", Action: "paste", Disabled: true},
			}},
			{Label: "View", Items: []MenuItem{
				{Label: "Tile All", Shortcut: "Ctrl+T", Action: "tile_all"},
				{Label: "Snap Left", Shortcut: "Ctrl+←", Action: "snap_left"},
				{Label: "Snap Right", Shortcut: "Ctrl+→", Action: "snap_right"},
			}},
			{Label: "Help", Items: []MenuItem{
				{Label: "Keybindings", Shortcut: "F1", Action: "help_keys"},
				{Label: "About", Action: "about"},
			}},
		},
		OpenIndex:  -1,
		HoverIndex: -1,
		Width:      width,
		ShowClock:  true,
		ShowCPU:    true,
		ShowMemory: true,
	}
}

// SetWidth updates the menu bar width.
func (mb *MenuBar) SetWidth(w int) {
	mb.Width = w
}

// OpenMenu opens a menu by index.
func (mb *MenuBar) OpenMenu(idx int) {
	if idx >= 0 && idx < len(mb.Menus) {
		mb.OpenIndex = idx
		mb.HoverIndex = 0
	}
}

// CloseMenu closes any open menu.
func (mb *MenuBar) CloseMenu() {
	mb.OpenIndex = -1
	mb.HoverIndex = -1
}

// IsOpen returns whether any menu is open.
func (mb *MenuBar) IsOpen() bool {
	return mb.OpenIndex >= 0
}

// MoveHover moves the hover in the open dropdown. Returns new hover index.
func (mb *MenuBar) MoveHover(delta int) int {
	if mb.OpenIndex < 0 {
		return -1
	}
	items := mb.Menus[mb.OpenIndex].Items
	if len(items) == 0 {
		return -1
	}
	mb.HoverIndex += delta
	if mb.HoverIndex < 0 {
		mb.HoverIndex = len(items) - 1
	}
	if mb.HoverIndex >= len(items) {
		mb.HoverIndex = 0
	}
	return mb.HoverIndex
}

// MoveMenu moves to adjacent menu (left/right).
func (mb *MenuBar) MoveMenu(delta int) {
	if len(mb.Menus) == 0 {
		return
	}
	mb.OpenIndex += delta
	if mb.OpenIndex < 0 {
		mb.OpenIndex = len(mb.Menus) - 1
	}
	if mb.OpenIndex >= len(mb.Menus) {
		mb.OpenIndex = 0
	}
	mb.HoverIndex = 0
}

// SelectedAction returns the action of the currently hovered item, or "".
func (mb *MenuBar) SelectedAction() string {
	if mb.OpenIndex < 0 || mb.HoverIndex < 0 {
		return ""
	}
	items := mb.Menus[mb.OpenIndex].Items
	if mb.HoverIndex >= len(items) {
		return ""
	}
	item := items[mb.HoverIndex]
	if item.Disabled {
		return ""
	}
	return item.Action
}

// MenuXPositions returns the X position of each menu label.
func (mb *MenuBar) MenuXPositions() []int {
	positions := make([]int, len(mb.Menus))
	x := 1
	for i, m := range mb.Menus {
		positions[i] = x
		x += len(m.Label) + 3 // " Label "
	}
	return positions
}

// MenuAtX returns the menu index at the given X position, or -1.
func (mb *MenuBar) MenuAtX(x int) int {
	positions := mb.MenuXPositions()
	for i, pos := range positions {
		labelW := len(mb.Menus[i].Label) + 2 // " Label "
		if x >= pos && x < pos+labelW {
			return i
		}
	}
	return -1
}

// DropdownItemAtY returns the item index at the given Y offset (0-based from dropdown top).
func (mb *MenuBar) DropdownItemAtY(y int) int {
	if mb.OpenIndex < 0 {
		return -1
	}
	items := mb.Menus[mb.OpenIndex].Items
	if y >= 0 && y < len(items) {
		return y
	}
	return -1
}

// Render returns the menu bar as a string for the given width.
func (mb *MenuBar) Render(width int) string {
	var sb strings.Builder

	// Render menu labels
	for i, m := range mb.Menus {
		if i == mb.OpenIndex {
			sb.WriteString("[" + m.Label + "]")
		} else {
			sb.WriteString(" " + m.Label + " ")
		}
	}

	left := sb.String()

	// Right side: indicators + clock
	right := mb.renderRight()

	// Pad the middle
	padding := width - len([]rune(left)) - len([]rune(right))
	if padding < 0 {
		padding = 0
	}

	return left + strings.Repeat(" ", padding) + right
}

// RenderDropdown returns the dropdown menu as lines (without the bar itself).
func (mb *MenuBar) RenderDropdown() []string {
	if mb.OpenIndex < 0 {
		return nil
	}
	menu := mb.Menus[mb.OpenIndex]
	if len(menu.Items) == 0 {
		return nil
	}

	// Calculate dropdown width
	maxW := 0
	for _, item := range menu.Items {
		w := len(item.Label) + 2 // padding
		if item.Shortcut != "" {
			w += len(item.Shortcut) + 2
		}
		if w > maxW {
			maxW = w
		}
	}
	maxW += 2 // borders

	var lines []string
	// Top border
	lines = append(lines, "┌"+strings.Repeat("─", maxW-2)+"┐")

	for i, item := range menu.Items {
		label := item.Label
		shortcut := item.Shortcut
		// Pad label to fill
		innerW := maxW - 2
		content := " " + label
		if shortcut != "" {
			gap := innerW - len(content) - len(shortcut) - 1
			if gap < 1 {
				gap = 1
			}
			content += strings.Repeat(" ", gap) + shortcut
		}
		// Pad to inner width
		for len([]rune(content)) < innerW {
			content += " "
		}

		prefix := "│"
		suffix := "│"
		if i == mb.HoverIndex {
			prefix = "│"
			suffix = "│"
			// Mark hovered items with > indicator
			content = ">" + content[1:]
		}
		if item.Disabled {
			// Dim disabled items
			content = " " + strings.Repeat("·", len(content)-1)
			content = content[:innerW]
		}

		lines = append(lines, prefix+content+suffix)
	}

	// Bottom border
	lines = append(lines, "└"+strings.Repeat("─", maxW-2)+"┘")

	return lines
}

func (mb *MenuBar) renderRight() string {
	var parts []string

	if mb.ShowCPU {
		parts = append(parts, "CPU:--")
	}
	if mb.ShowMemory {
		parts = append(parts, "MEM:--")
	}
	if mb.ShowClock {
		parts = append(parts, time.Now().Format("03:04 PM"))
	}

	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ") + " "
}

// ClockString returns the current time formatted for the menu bar.
func ClockString() string {
	return time.Now().Format("03:04 PM")
}

// FormatCPU formats a CPU percentage for the menu bar.
func FormatCPU(pct float64) string {
	return fmt.Sprintf("CPU:%2.0f%%", pct)
}

// FormatMemory formats memory usage for the menu bar.
func FormatMemory(usedGB float64) string {
	return fmt.Sprintf("MEM:%.1fG", usedGB)
}
