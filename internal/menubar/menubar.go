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
	ShowClock   bool
	ShowCPU     bool
	ShowMemory  bool
	ShowBattery bool
	CPUPct      float64   // current CPU percentage
	CPUHistory  []float64 // rolling history for sparkline (last 20 samples)
	MemGB       float64 // current memory usage in GB
	BatPct      float64 // battery percentage
	BatCharging bool    // battery is charging
	BatPresent  bool    // battery exists on this system
	Username    string  // logged-in username
}

// New creates a menu bar with default menus.
func New(width int) *MenuBar {
	return &MenuBar{
		Menus: []Menu{
			{Label: "File", Items: []MenuItem{
				{Label: "New Terminal", Shortcut: "n", Action: "new_terminal"},
				{Label: "Minimize", Shortcut: "m", Action: "minimize"},
				{Label: "─", Disabled: true},
				{Label: "Detach", Shortcut: "F12", Action: "detach"},
				{Label: "Quit", Shortcut: "Ctrl+Q", Action: "quit"},
			}},
			{Label: "Apps", Items: []MenuItem{
				{Label: "\uf120 Terminal", Action: "launch_terminal"},
				{Label: "\ue62b nvim", Action: "launch_nvim"},
				{Label: "\uf07b Files", Action: "launch_files"},
				{Label: "\uf1ec Calc", Action: "launch_calc"},
				{Label: "\uf200 System Monitor", Action: "launch_htop"},
			}},
			{Label: "View", Items: []MenuItem{
				{Label: "Tile All", Shortcut: "t", Action: "tile_all"},
				{Label: "Snap Left", Shortcut: "h", Action: "snap_left"},
				{Label: "Snap Right", Shortcut: "l", Action: "snap_right"},
				{Label: "─", Disabled: true},
				{Label: "Dock: Icons Only", Action: "toggle_icons_only"},
				{Label: "─", Disabled: true},
				{Label: "Theme: Retro", Action: "theme_retro"},
				{Label: "Theme: Modern", Action: "theme_modern"},
				{Label: "Theme: Tokyo Night", Action: "theme_tokyonight"},
				{Label: "Theme: Catppuccin", Action: "theme_catppuccin"},
			}},
			{Label: "Help", Items: []MenuItem{
				{Label: "Keybindings", Shortcut: "F1", Action: "help_keys"},
				{Label: "About", Action: "about"},
			}},
		},
		OpenIndex:  -1,
		HoverIndex: -1,
		Width:      width,
		ShowClock:   true,
		ShowCPU:     true,
		ShowMemory:  true,
		ShowBattery: true,
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
		// Skip to first selectable item
		items := mb.Menus[idx].Items
		for mb.HoverIndex < len(items) && items[mb.HoverIndex].Disabled {
			mb.HoverIndex++
		}
		if mb.HoverIndex >= len(items) {
			mb.HoverIndex = 0
		}
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

// MoveHover moves the hover in the open dropdown, skipping separators.
// Returns new hover index.
func (mb *MenuBar) MoveHover(delta int) int {
	if mb.OpenIndex < 0 {
		return -1
	}
	items := mb.Menus[mb.OpenIndex].Items
	n := len(items)
	if n == 0 {
		return -1
	}
	start := mb.HoverIndex
	for {
		mb.HoverIndex += delta
		if mb.HoverIndex < 0 {
			mb.HoverIndex = n - 1
		}
		if mb.HoverIndex >= n {
			mb.HoverIndex = 0
		}
		if !items[mb.HoverIndex].Disabled {
			break
		}
		if mb.HoverIndex == start {
			break // all disabled, prevent infinite loop
		}
	}
	return mb.HoverIndex
}

// isSeparator returns true if the item is a visual separator line.
func isSeparator(item MenuItem) bool {
	return item.Disabled && strings.HasPrefix(item.Label, "─")
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
		x += len(m.Label) + 2 // " Label " or "[Label]" = len+2
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

	// Calculate dropdown width (exclude separators)
	maxW := 0
	for _, item := range menu.Items {
		if isSeparator(item) {
			continue
		}
		w := len([]rune(item.Label)) + 2
		if item.Shortcut != "" {
			w += len(item.Shortcut) + 2
		}
		if w > maxW {
			maxW = w
		}
	}
	maxW += 2 // borders

	var lines []string
	lines = append(lines, "┌"+strings.Repeat("─", maxW-2)+"┐")

	for i, item := range menu.Items {
		// Separator: render as connected horizontal line
		if isSeparator(item) {
			lines = append(lines, "├"+strings.Repeat("─", maxW-2)+"┤")
			continue
		}

		innerW := maxW - 2
		content := " " + item.Label
		if item.Shortcut != "" {
			gap := innerW - len([]rune(content)) - len(item.Shortcut) - 1
			if gap < 1 {
				gap = 1
			}
			content += strings.Repeat(" ", gap) + item.Shortcut
		}
		for len([]rune(content)) < innerW {
			content += " "
		}

		if i == mb.HoverIndex && !item.Disabled {
			runes := []rune(content)
			runes[0] = '>'
			content = string(runes)
		}

		lines = append(lines, "│"+content+"│")
	}

	lines = append(lines, "└"+strings.Repeat("─", maxW-2)+"┘")
	return lines
}

// RightSideZone describes a clickable zone on the right side of the menu bar.
type RightSideZone struct {
	Start int    // X start position
	End   int    // X end position (exclusive)
	Type  string // "cpu", "mem", "clock"
}

// RightZones returns the clickable zones for the right side of the menu bar.
func (mb *MenuBar) RightZones(totalWidth int) []RightSideZone {
	rightStr := mb.renderRight()
	rightLen := len([]rune(rightStr))
	startX := totalWidth - rightLen

	var zones []RightSideZone
	x := startX + 1 // skip leading space
	if mb.ShowCPU {
		s := FormatCPU(mb.CPUPct)
		w := len([]rune(s))
		zones = append(zones, RightSideZone{Start: x, End: x + w, Type: "cpu"})
		x += w + 1 // +1 for space separator
	}
	if mb.ShowMemory {
		s := FormatMemory(mb.MemGB)
		w := len([]rune(s))
		zones = append(zones, RightSideZone{Start: x, End: x + w, Type: "mem"})
		x += w + 1
	}
	if mb.ShowBattery && mb.BatPresent {
		s := FormatBattery(mb.BatPct, mb.BatCharging)
		w := len([]rune(s))
		zones = append(zones, RightSideZone{Start: x, End: x + w, Type: "bat"})
		x += w + 1
	}
	if mb.ShowClock {
		s := time.Now().Format("03:04 PM")
		w := len([]rune(s))
		zones = append(zones, RightSideZone{Start: x, End: x + w, Type: "clock"})
	}
	return zones
}

func (mb *MenuBar) renderRight() string {
	var parts []string

	if mb.ShowCPU {
		parts = append(parts, FormatCPU(mb.CPUPct))
	}
	if mb.ShowMemory {
		parts = append(parts, FormatMemory(mb.MemGB))
	}
	if mb.ShowBattery && mb.BatPresent {
		parts = append(parts, FormatBattery(mb.BatPct, mb.BatCharging))
	}
	if mb.ShowClock {
		parts = append(parts, time.Now().Format("03:04 PM"))
	}
	if mb.Username != "" {
		parts = append(parts, "\uf007 "+mb.Username) //  user icon
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

// FormatCPU formats a CPU percentage for the menu bar with tmux-style icons.
func FormatCPU(pct float64) string {
	icon := "\uf108" // 󰈸 nf-md-desktop-classic (normal)
	if pct >= 80 {
		icon = "\uf0e7" //  nf-fa-bolt (high)
	} else if pct >= 50 {
		icon = "\uf0e7" //  nf-fa-bolt (medium)
	}
	return fmt.Sprintf("%s %2.0f%%", icon, pct)
}

// FormatMemory formats memory usage for the menu bar with tmux-style icons.
func FormatMemory(usedGB float64) string {
	icon := "\uf85a" // 󰡚 nf-md-memory
	return fmt.Sprintf("%s %.1fG", icon, usedGB)
}

// FormatBattery formats battery status for the menu bar.
func FormatBattery(pct float64, charging bool) string {
	icon := "\uf244" //  nf-fa-battery_empty
	if pct >= 80 {
		icon = "\uf240" //  nf-fa-battery_full
	} else if pct >= 60 {
		icon = "\uf241" //  nf-fa-battery_three_quarters
	} else if pct >= 40 {
		icon = "\uf242" //  nf-fa-battery_half
	} else if pct >= 20 {
		icon = "\uf243" //  nf-fa-battery_quarter
	}
	charge := ""
	if charging {
		charge = "\u26a1" // ⚡
	}
	return fmt.Sprintf("%s %.0f%%%s", icon, pct, charge)
}

// BatColorLevel returns a color level: "green", "yellow", or "red" based on battery percentage.
func BatColorLevel(pct float64) string {
	if pct >= 50 {
		return "green"
	}
	if pct >= 20 {
		return "yellow"
	}
	return "red"
}

// CPUColorLevel returns a color level: "green", "yellow", or "red" based on CPU usage.
func CPUColorLevel(pct float64) string {
	if pct >= 80 {
		return "red"
	}
	if pct >= 50 {
		return "yellow"
	}
	return "green"
}

// MemColorLevel returns a color level: "green", "yellow", or "red" based on memory usage percentage.
func MemColorLevel(usedGB, totalGB float64) string {
	if totalGB <= 0 {
		return "green"
	}
	pct := usedGB / totalGB * 100
	if pct >= 80 {
		return "red"
	}
	if pct >= 60 {
		return "yellow"
	}
	return "green"
}
