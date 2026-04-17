package menubar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/widget"
)

// MenuItem represents a single item in a dropdown menu.
type MenuItem struct {
	Label    string
	Shortcut string // e.g., "Ctrl+N"
	Action   string // action identifier
	Disabled bool
	SubItems []MenuItem // submenu items (non-nil = has submenu)
}

// Menu represents a top-level menu with a dropdown.
type Menu struct {
	Label string
	Items []MenuItem
}

// MenuBar represents the top menu bar of the desktop.
type MenuBar struct {
	Menus         []Menu
	OpenIndex     int         // -1 = no menu open
	HoverIndex    int         // highlighted item in open menu, -1 = none
	InSubMenu     bool        // true = navigating inside a submenu
	SubHoverIndex int         // highlighted item in open submenu, -1 = none
	FocusIndex    int         // Tab-cycling focus on menu label, -1 = none
	Width         int         // total width available
	WidgetBar     *widget.Bar // right-side status widgets
}

// New creates a menu bar with default menus. Shortcuts reflect the user's keybindings.
// The Apps menu is built dynamically from the app registry.
func New(width int, kb config.KeyBindings, registry []registry.RegistryEntry, wb *widget.Bar) *MenuBar {
	// Build Apps menu from registry, grouping games into a submenu
	var appItems []MenuItem
	var gameItems []MenuItem
	for _, entry := range registry {
		label := entry.Icon + "  " + entry.Name
		action := "launch:" + entry.Command
		if entry.Category == "games" {
			gameItems = append(gameItems, MenuItem{Label: label, Action: action})
		} else {
			appItems = append(appItems, MenuItem{Label: label, Action: action})
		}
	}
	if len(gameItems) > 0 {
		appItems = append(appItems, MenuItem{Label: "─", Disabled: true})
		appItems = append(appItems, MenuItem{Label: "\uf11b  Games", SubItems: gameItems})
	}

	mb := &MenuBar{
		Menus: []Menu{
			{Label: "File", Items: []MenuItem{
				{Label: "\uf120  New Terminal", Shortcut: kb.NewTerminal, Action: "new_terminal"},
				{Label: "─", Disabled: true},
				{Label: "\uf07c  New Workspace...", Shortcut: kb.NewWorkspace, Action: "new_workspace"},
				{Label: "\uf0c7  Save Workspace", Shortcut: kb.SaveWorkspace, Action: "save_workspace"},
				{Label: "\uf115  Load Workspace...", Shortcut: kb.LoadWorkspace, Action: "load_workspace"},
				{Label: "─", Disabled: true},
				{Label: "\uf127  Detach", Shortcut: "Pfx+d", Action: "detach"},
				{Label: "\uf011  Quit", Shortcut: "Ctrl+Q", Action: "quit"},
			}},
			{Label: "Edit", Items: []MenuItem{
				{Label: "\uf0ea  Paste", Action: "paste"},
				{Label: "\uf46d  Clipboard History", Shortcut: kb.ClipboardHistory, Action: "clipboard_history"},
				{Label: "─", Disabled: true},
				{Label: "\ueb56  Split Horizontal", Shortcut: "Pfx+%", Action: "split_horizontal"},
				{Label: "\ueb57  Split Vertical", Shortcut: `Pfx+"`, Action: "split_vertical"},
				{Label: "\uf00d  Close Pane", Shortcut: "Pfx+x", Action: "close_pane"},
				{Label: "─", Disabled: true},
				{Label: "\uf0c5  Enter Copy Mode", Shortcut: "c / Pfx+c", Action: "copy_mode"},
				{Label: "\uf00e  Copy Search Forward", Shortcut: "/", Action: "copy_search_forward"},
				{Label: "\uf010  Copy Search Backward", Shortcut: "?", Action: "copy_search_backward"},
			}},
			{Label: "Apps", Items: appItems},
			{Label: "View", Items: []MenuItem{
				{Label: "\uf489  Quake Terminal", Shortcut: kb.QuakeTerminal, Action: "quake_terminal"},
				{Label: "\uf24d  Sync Panes: Off", Shortcut: "Pfx+" + kb.SyncPanes, Action: "sync_panes"},
				{Label: "─", Disabled: true},
				{Label: "\uf2d1  Minimize", Shortcut: kb.Minimize, Action: "minimize"},
				{Label: "─", Disabled: true},
				{Label: "\uf009  Tile All", Shortcut: kb.TileAll, Action: "tile_all"},
				{Label: "\uf0db  Tile Columns", Shortcut: kb.TileColumns, Action: "tile_columns"},
				{Label: "\uf0c9  Tile Rows", Shortcut: kb.TileRows, Action: "tile_rows"},
				{Label: "\uf2d0  Tile Maximized", Shortcut: kb.TileMaximized, Action: "tile_maximized"},
				{Label: "\uf2d2  Cascade", Shortcut: kb.Cascade, Action: "cascade"},
				{Label: "─", Disabled: true},
				{Label: "\uf108  Show Desktop", Shortcut: kb.ShowDesktop, Action: "show_desktop"},
				{Label: "\uf11c  Show Keys", Shortcut: kb.ShowKeys, Action: "show_keys"},
				{Label: "\uf00a  Toggle Tiling", Shortcut: kb.ToggleTiling, Action: "toggle_tiling"},
				{Label: "\uf061  Next Tile Spawn: Auto", Action: "tile_spawn_cycle"},
				{Label: "─", Disabled: true},
				{Label: "\uf104  Snap Left", Shortcut: kb.SnapLeft + "/←", Action: "snap_left"},
				{Label: "\uf105  Snap Right", Shortcut: kb.SnapRight + "/→", Action: "snap_right"},
				{Label: "\uf106  Snap Top", Shortcut: kb.SnapTop, Action: "snap_top"},
				{Label: "\uf107  Snap Bottom", Shortcut: kb.SnapBottom, Action: "snap_bottom"},
				{Label: "\uf05b  Center", Shortcut: kb.Center, Action: "center"},
				{Label: "─", Disabled: true},
				{Label: "\u2190  Swap Left", Shortcut: kb.SwapLeft, Action: "swap_left"},
				{Label: "\u2192  Swap Right", Shortcut: kb.SwapRight, Action: "swap_right"},
				{Label: "\u2191  Swap Up", Shortcut: kb.SwapUp, Action: "swap_up"},
				{Label: "\u2193  Swap Down", Shortcut: kb.SwapDown, Action: "swap_down"},
				{Label: "─", Disabled: true},
				{Label: "\uf013  Settings...", Shortcut: kb.Settings, Action: "settings"},
			}},
			{Label: "Help", Items: []MenuItem{
				{Label: "\uf11c  Keybindings", Shortcut: kb.Help, Action: "help_keys"},
				{Label: "\uf05a  About", Action: "about"},
			}},
		},
		OpenIndex:     -1,
		HoverIndex:    -1,
		InSubMenu:     false,
		SubHoverIndex: -1,
		FocusIndex:    -1,
		Width:         width,
		WidgetBar:     wb,
	}
	mb.SetToggleTilingLabel(false, "Columns")
	mb.SetTileSpawnLabel("Auto")
	return mb
}

// SetToggleTilingLabel updates the View menu toggle entry with current state.
func (mb *MenuBar) SetToggleTilingLabel(enabled bool, layout string) {
	state := "Off"
	if enabled {
		state = "On"
	}
	label := fmt.Sprintf("\uf00a  Tiling: %s (%s)", state, layout)
	for mi := range mb.Menus {
		for ii := range mb.Menus[mi].Items {
			if mb.Menus[mi].Items[ii].Action == "toggle_tiling" {
				mb.Menus[mi].Items[ii].Label = label
				return
			}
		}
	}
}

// SetTileSpawnLabel updates the View menu spawn preset entry.
func (mb *MenuBar) SetTileSpawnLabel(preset string) {
	label := fmt.Sprintf("\uf061  Next Tile Spawn: %s", preset)
	for mi := range mb.Menus {
		for ii := range mb.Menus[mi].Items {
			if mb.Menus[mi].Items[ii].Action == "tile_spawn_cycle" {
				mb.Menus[mi].Items[ii].Label = label
				return
			}
		}
	}
}

// SetSyncPanesLabel updates the View menu sync panes entry.
func (mb *MenuBar) SetSyncPanesLabel(enabled bool) {
	state := "Off"
	if enabled {
		state = "On"
	}
	label := fmt.Sprintf("\uf24d  Sync Panes: %s", state)
	mb.setMenuItemLabel("sync_panes", label)
}

// setMenuItemLabel finds a menu item by action and updates its label.
func (mb *MenuBar) setMenuItemLabel(action, label string) {
	for mi := range mb.Menus {
		for ii := range mb.Menus[mi].Items {
			if mb.Menus[mi].Items[ii].Action == action {
				mb.Menus[mi].Items[ii].Label = label
				return
			}
		}
	}
}

// SetMenuItemShortcut finds a menu item by action and updates its shortcut text.
func (mb *MenuBar) SetMenuItemShortcut(action, shortcut string) {
	for mi := range mb.Menus {
		for ii := range mb.Menus[mi].Items {
			if mb.Menus[mi].Items[ii].Action == action {
				mb.Menus[mi].Items[ii].Shortcut = shortcut
				return
			}
		}
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
	mb.InSubMenu = false
	mb.SubHoverIndex = -1
}

// IsOpen returns whether any menu is open.
func (mb *MenuBar) IsOpen() bool {
	return mb.OpenIndex >= 0
}

// MoveHover moves the hover in the open dropdown (or submenu), skipping separators.
// Returns new hover index.
func (mb *MenuBar) MoveHover(delta int) int {
	if mb.OpenIndex < 0 {
		return -1
	}

	// If navigating inside a submenu
	if mb.InSubMenu {
		item := mb.hoveredItem()
		if item == nil || len(item.SubItems) == 0 {
			mb.InSubMenu = false
			mb.SubHoverIndex = -1
			return mb.HoverIndex
		}
		subItems := item.SubItems
		n := len(subItems)
		start := mb.SubHoverIndex
		for {
			mb.SubHoverIndex += delta
			if mb.SubHoverIndex < 0 {
				mb.SubHoverIndex = n - 1
			}
			if mb.SubHoverIndex >= n {
				mb.SubHoverIndex = 0
			}
			if !subItems[mb.SubHoverIndex].Disabled {
				break
			}
			if mb.SubHoverIndex == start {
				break
			}
		}
		return mb.SubHoverIndex
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
	// Exit submenu when moving to a different parent item
	mb.InSubMenu = false
	mb.SubHoverIndex = -1
	return mb.HoverIndex
}

// hoveredItem returns the currently hovered MenuItem, or nil.
func (mb *MenuBar) hoveredItem() *MenuItem {
	if mb.OpenIndex < 0 || mb.HoverIndex < 0 {
		return nil
	}
	items := mb.Menus[mb.OpenIndex].Items
	if mb.HoverIndex >= len(items) {
		return nil
	}
	return &items[mb.HoverIndex]
}

// EnterSubMenu enters the submenu of the currently hovered item (if it has one).
// Returns true if a submenu was entered.
func (mb *MenuBar) EnterSubMenu() bool {
	item := mb.hoveredItem()
	if item == nil || len(item.SubItems) == 0 {
		return false
	}
	mb.InSubMenu = true
	mb.SubHoverIndex = 0
	// Skip to first non-disabled item
	for mb.SubHoverIndex < len(item.SubItems) && item.SubItems[mb.SubHoverIndex].Disabled {
		mb.SubHoverIndex++
	}
	if mb.SubHoverIndex >= len(item.SubItems) {
		mb.SubHoverIndex = 0
	}
	return true
}

// ExitSubMenu exits the current submenu back to the parent menu.
func (mb *MenuBar) ExitSubMenu() {
	mb.InSubMenu = false
	mb.SubHoverIndex = -1
}

// HasSubMenu returns true if the currently hovered item has a submenu.
func (mb *MenuBar) HasSubMenu() bool {
	item := mb.hoveredItem()
	return item != nil && len(item.SubItems) > 0
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
	mb.InSubMenu = false
	mb.SubHoverIndex = -1
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

	// If in submenu, return submenu item's action
	if mb.InSubMenu && len(item.SubItems) > 0 && mb.SubHoverIndex >= 0 && mb.SubHoverIndex < len(item.SubItems) {
		sub := item.SubItems[mb.SubHoverIndex]
		if sub.Disabled {
			return ""
		}
		return sub.Action
	}

	if item.Disabled || len(item.SubItems) > 0 {
		return "" // parent submenu items have no direct action
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
	for _, m := range mb.Menus {
		sb.WriteString(" " + m.Label + " ")
	}

	left := sb.String()

	// Right side: indicators + clock
	right := mb.renderRight()

	// Pad the middle — use DisplayWidth for right side (Nerd Font icons are 2 cells)
	rightW := widget.DisplayWidth(right)
	padding := width - len([]rune(left)) - rightW
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

	for _, item := range menu.Items {
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

		lines = append(lines, "│"+content+"│")
	}

	lines = append(lines, "└"+strings.Repeat("─", maxW-2)+"┘")
	return lines
}

// DropdownDimensions returns the width and height of the dropdown for the open menu.
// Used by the renderer to know the stampANSI target size.
func (mb *MenuBar) DropdownDimensions() (int, int) {
	if mb.OpenIndex < 0 {
		return 0, 0
	}
	menu := mb.Menus[mb.OpenIndex]
	if len(menu.Items) == 0 {
		return 0, 0
	}

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
	maxW += 2 // padding

	// height = items + 2 (top/bottom border)
	return maxW + 2, len(menu.Items) + 2
}

// RenderDropdownStyled returns a lipgloss-styled dropdown string.
func (mb *MenuBar) RenderDropdownStyled(borderFg, bgColor, hoverFg, hoverBg, normalFg, shortcutFg string) string {
	if mb.OpenIndex < 0 {
		return ""
	}
	menu := mb.Menus[mb.OpenIndex]
	if len(menu.Items) == 0 {
		return ""
	}

	// Calculate inner width (exclude separators)
	maxInnerW := 0
	for _, item := range menu.Items {
		if isSeparator(item) {
			continue
		}
		w := len([]rune(item.Label)) + 2
		if item.Shortcut != "" {
			w += len(item.Shortcut) + 2
		}
		if len(item.SubItems) > 0 {
			w += 2 // space + "►"
		}
		if w > maxInnerW {
			maxInnerW = w
		}
	}
	maxInnerW += 2 // extra padding

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(normalFg)).
		Background(lipgloss.Color(bgColor))

	hoverStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(hoverFg)).
		Background(lipgloss.Color(hoverBg)).
		Bold(true)

	shortcutStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(shortcutFg)).
		Background(lipgloss.Color(bgColor))

	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(borderFg)).
		Background(lipgloss.Color(bgColor)).
		Width(maxInnerW)

	var rows []string
	for i, item := range menu.Items {
		if isSeparator(item) {
			rows = append(rows, sepStyle.Render(strings.Repeat("─", maxInnerW)))
			continue
		}

		isHover := i == mb.HoverIndex && !item.Disabled && !mb.InSubMenu
		label := " " + item.Label

		if len(item.SubItems) > 0 {
			// Submenu parent: show "►" arrow on right
			arrow := "►"
			gap := maxInnerW - len([]rune(label)) - len([]rune(arrow)) - 1
			if gap < 1 {
				gap = 1
			}
			content := label + strings.Repeat(" ", gap) + arrow
			for len([]rune(content)) < maxInnerW {
				content += " "
			}
			if isHover {
				rows = append(rows, hoverStyle.Render(content))
			} else {
				rows = append(rows, normalStyle.Render(content))
			}
		} else if item.Shortcut != "" {
			gap := maxInnerW - len([]rune(label)) - len(item.Shortcut) - 1
			if gap < 1 {
				gap = 1
			}
			if isHover {
				content := label + strings.Repeat(" ", gap) + item.Shortcut
				for len([]rune(content)) < maxInnerW {
					content += " "
				}
				rows = append(rows, hoverStyle.Render(content))
			} else {
				labelPart := label + strings.Repeat(" ", gap)
				rows = append(rows, normalStyle.Render(labelPart)+shortcutStyle.Render(item.Shortcut+strings.Repeat(" ", maxInnerW-len([]rune(labelPart))-len(item.Shortcut))))
			}
		} else {
			content := label
			for len([]rune(content)) < maxInnerW {
				content += " "
			}
			if isHover {
				rows = append(rows, hoverStyle.Render(content))
			} else {
				rows = append(rows, normalStyle.Render(content))
			}
		}
	}

	inner := strings.Join(rows, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderFg)).
		BorderBackground(lipgloss.Color(bgColor))

	return boxStyle.Render(inner)
}

// SubMenuParentIndex returns the Y offset of the submenu parent item in the dropdown,
// and the width of the parent dropdown. Returns -1 if no submenu is active.
func (mb *MenuBar) SubMenuParentIndex() (itemY int, parentWidth int) {
	if mb.OpenIndex < 0 || mb.HoverIndex < 0 {
		return -1, 0
	}
	item := mb.hoveredItem()
	if item == nil || len(item.SubItems) == 0 {
		return -1, 0
	}
	// itemY = border(1) + hoverIndex
	return mb.HoverIndex, mb.dropdownInnerWidth() + 2 // +2 for left/right borders
}

// dropdownInnerWidth returns the inner content width of the open dropdown.
func (mb *MenuBar) dropdownInnerWidth() int {
	if mb.OpenIndex < 0 {
		return 0
	}
	menu := mb.Menus[mb.OpenIndex]
	maxW := 0
	for _, item := range menu.Items {
		if isSeparator(item) {
			continue
		}
		w := len([]rune(item.Label)) + 2
		if item.Shortcut != "" {
			w += len(item.Shortcut) + 2
		}
		if len(item.SubItems) > 0 {
			w += 2 // space + "►"
		}
		if w > maxW {
			maxW = w
		}
	}
	return maxW + 2 // padding
}

// SubMenuDimensions returns the total width and height of the open submenu
// (including border). Returns (0,0) if no submenu is active.
func (mb *MenuBar) SubMenuDimensions() (int, int) {
	item := mb.hoveredItem()
	if item == nil || len(item.SubItems) == 0 {
		return 0, 0
	}
	maxW := 0
	for _, sub := range item.SubItems {
		if isSeparator(sub) {
			continue
		}
		w := len([]rune(sub.Label)) + 4 // label + padding
		if w > maxW {
			maxW = w
		}
	}
	return maxW + 2, len(item.SubItems) + 2 // +2 for border
}

// RenderSubMenuStyled returns a lipgloss-styled submenu dropdown string.
func (mb *MenuBar) RenderSubMenuStyled(borderFg, bgColor, hoverFg, hoverBg, normalFg, shortcutFg string) string {
	item := mb.hoveredItem()
	if item == nil || len(item.SubItems) == 0 {
		return ""
	}
	subItems := item.SubItems

	maxInnerW := 0
	for _, sub := range subItems {
		if isSeparator(sub) {
			continue
		}
		w := len([]rune(sub.Label)) + 2
		if w > maxInnerW {
			maxInnerW = w
		}
	}
	maxInnerW += 2

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(normalFg)).
		Background(lipgloss.Color(bgColor))

	hoverStyleS := lipgloss.NewStyle().
		Foreground(lipgloss.Color(hoverFg)).
		Background(lipgloss.Color(hoverBg)).
		Bold(true)

	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(borderFg)).
		Background(lipgloss.Color(bgColor)).
		Width(maxInnerW)

	var rows []string
	for i, sub := range subItems {
		if isSeparator(sub) {
			rows = append(rows, sepStyle.Render(strings.Repeat("─", maxInnerW)))
			continue
		}

		isHover := mb.InSubMenu && i == mb.SubHoverIndex && !sub.Disabled
		content := " " + sub.Label
		for len([]rune(content)) < maxInnerW {
			content += " "
		}
		if isHover {
			rows = append(rows, hoverStyleS.Render(content))
		} else {
			rows = append(rows, normalStyle.Render(content))
		}
	}

	inner := strings.Join(rows, "\n")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderFg)).
		BorderBackground(lipgloss.Color(bgColor))

	return boxStyle.Render(inner)
}

// SubMenuItemAtY returns the submenu item index at the given Y offset (0-based from submenu top).
func (mb *MenuBar) SubMenuItemAtY(y int) int {
	item := mb.hoveredItem()
	if item == nil || len(item.SubItems) == 0 {
		return -1
	}
	if y >= 0 && y < len(item.SubItems) {
		return y
	}
	return -1
}

// RightSideZone describes a clickable zone on the right side of the menu bar.
type RightSideZone = widget.Zone

// RightZones returns the clickable zones for the right side of the menu bar.
func (mb *MenuBar) RightZones(totalWidth int) []RightSideZone {
	if mb.WidgetBar == nil {
		return nil
	}
	return mb.WidgetBar.RenderZones(totalWidth)
}

func (mb *MenuBar) renderRight() string {
	if mb.WidgetBar == nil {
		return ""
	}
	return mb.WidgetBar.Render()
}
