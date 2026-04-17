package menubar

import (
	"strings"
	"testing"

	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/widget"
)

func testMenuBar(width int) *MenuBar {
	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "$SHELL"},
	}
	wb := widget.NewDefaultBar("testuser")
	return New(width, config.DefaultKeyBindings(), registry, wb)
}

// testMenuBarWithSubmenu returns a MenuBar with a submenu injected into View menu
// for testing submenu navigation. The submenu contains test theme items.
func testMenuBarWithSubmenu(width int) *MenuBar {
	mb := testMenuBar(width)
	// Inject a submenu item into the View menu (index 3)
	for i := range mb.Menus {
		if mb.Menus[i].Label == "View" {
			mb.Menus[i].Items = append(mb.Menus[i].Items, MenuItem{
				Label: "Test Themes", SubItems: []MenuItem{
					{Label: "Alpha", Action: "theme_alpha"},
					{Label: "Beta", Action: "theme_beta"},
					{Label: "─", Disabled: true},
					{Label: "Gamma", Action: "theme_gamma"},
				},
			})
			break
		}
	}
	return mb
}

func TestNew(t *testing.T) {
	mb := testMenuBar(120)
	if mb.Width != 120 {
		t.Errorf("width = %d, want 120", mb.Width)
	}
	if len(mb.Menus) == 0 {
		t.Error("expected default menus")
	}
	if mb.IsOpen() {
		t.Error("should not be open initially")
	}
}

func TestSetWidth(t *testing.T) {
	mb := testMenuBar(80)
	mb.SetWidth(120)
	if mb.Width != 120 {
		t.Errorf("width = %d, want 120", mb.Width)
	}
}

func TestSetTileSpawnLabel(t *testing.T) {
	mb := testMenuBar(120)
	mb.SetTileSpawnLabel("Right")

	found := false
	for _, menu := range mb.Menus {
		for _, item := range menu.Items {
			if item.Action == "tile_spawn_cycle" {
				found = true
				if !strings.HasSuffix(item.Label, "Next Tile Spawn: Right") {
					t.Fatalf("label = %q, want suffix %q", item.Label, "Next Tile Spawn: Right")
				}
			}
		}
	}
	if !found {
		t.Fatal("tile_spawn_cycle action not found in menus")
	}
}

func TestOpenCloseMenu(t *testing.T) {
	mb := testMenuBar(120)

	mb.OpenMenu(0)
	if !mb.IsOpen() {
		t.Error("should be open after OpenMenu")
	}
	if mb.OpenIndex != 0 {
		t.Errorf("OpenIndex = %d, want 0", mb.OpenIndex)
	}
	if mb.HoverIndex != 0 {
		t.Errorf("HoverIndex = %d, want 0", mb.HoverIndex)
	}

	mb.CloseMenu()
	if mb.IsOpen() {
		t.Error("should not be open after CloseMenu")
	}
}

func TestOpenMenuOutOfBounds(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(99)
	if mb.IsOpen() {
		t.Error("should not open out-of-bounds menu")
	}
	mb.OpenMenu(-1)
	if mb.IsOpen() {
		t.Error("should not open negative index menu")
	}
}

func TestMoveHover(t *testing.T) {
	mb := testMenuBar(120)
	// File menu: New Terminal (0), separator (1), New Workspace (2), Save Workspace (3),
	//            Load Workspace (4), separator (5), Detach (6), Quit (7)
	mb.OpenMenu(0)

	// Start at 0, move down → should skip separator (1) and go to 2 (New Workspace)
	mb.MoveHover(1)
	if mb.HoverIndex != 2 {
		t.Errorf("hover = %d, want 2 (New Workspace)", mb.HoverIndex)
	}

	// Move down → should go to 3 (Save Workspace)
	mb.MoveHover(1)
	if mb.HoverIndex != 3 {
		t.Errorf("hover = %d, want 3 (Save Workspace)", mb.HoverIndex)
	}

	// Move down → go to 4 (Load Workspace)
	mb.MoveHover(1)
	if mb.HoverIndex != 4 {
		t.Errorf("hover = %d, want 4 (Load Workspace)", mb.HoverIndex)
	}

	// Move down → skip separator (5) and go to 6 (Detach)
	mb.MoveHover(1)
	if mb.HoverIndex != 6 {
		t.Errorf("hover = %d, want 6 (Detach)", mb.HoverIndex)
	}

	// Move down → go to 7 (Quit)
	mb.MoveHover(1)
	if mb.HoverIndex != 7 {
		t.Errorf("hover = %d, want 7 (Quit)", mb.HoverIndex)
	}

	// Move down → wrap to 0 (New Terminal)
	mb.MoveHover(1)
	if mb.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0 (wrap)", mb.HoverIndex)
	}

	// Move up from 0 → wrap to last selectable (7, Quit)
	mb.MoveHover(-1)
	if mb.HoverIndex != 7 {
		t.Errorf("hover = %d, want 7 (wrap up)", mb.HoverIndex)
	}

	// Move up from 7 → go to 6 (Detach)
	mb.MoveHover(-1)
	if mb.HoverIndex != 6 {
		t.Errorf("hover = %d, want 6 (Detach)", mb.HoverIndex)
	}

	// Move up from 6 → skip separator (5) and go to 4 (Load Workspace)
	mb.MoveHover(-1)
	if mb.HoverIndex != 4 {
		t.Errorf("hover = %d, want 4 (skip separator up)", mb.HoverIndex)
	}
}

func TestMoveHoverNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	result := mb.MoveHover(1)
	if result != -1 {
		t.Errorf("MoveHover with no menu = %d, want -1", result)
	}
}

func TestMoveMenu(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)

	mb.MoveMenu(1)
	if mb.OpenIndex != 1 {
		t.Errorf("OpenIndex = %d, want 1", mb.OpenIndex)
	}

	// Wrap right
	for i := 0; i < len(mb.Menus); i++ {
		mb.MoveMenu(1)
	}
	if mb.OpenIndex != 1 {
		t.Errorf("OpenIndex after full wrap = %d, want 1", mb.OpenIndex)
	}

	// Wrap left
	mb.OpenIndex = 0
	mb.MoveMenu(-1)
	if mb.OpenIndex != len(mb.Menus)-1 {
		t.Errorf("OpenIndex wrap left = %d, want %d", mb.OpenIndex, len(mb.Menus)-1)
	}
}

func TestMoveMenuEmpty(t *testing.T) {
	mb := &MenuBar{}
	mb.MoveMenu(1) // Should not panic
}

func TestSelectedAction(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File: New Terminal, Minimize, separator, Detach, Quit
	mb.HoverIndex = 0

	action := mb.SelectedAction()
	if action != "new_terminal" {
		t.Errorf("action = %q, want new_terminal", action)
	}

	// Last item is Quit
	mb.HoverIndex = len(mb.Menus[0].Items) - 1
	action = mb.SelectedAction()
	if action != "quit" {
		t.Errorf("action = %q, want quit", action)
	}
}

func TestSelectedActionDisabled(t *testing.T) {
	mb := testMenuBar(120)
	// File menu (index 0) has a separator at index 1
	mb.OpenMenu(0)
	mb.HoverIndex = 1 // separator (first separator in File menu)

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("disabled/separator action = %q, want empty", action)
	}
}

func TestSelectedActionNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("no menu action = %q, want empty", action)
	}
}

func TestSelectedActionOutOfBounds(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)
	mb.HoverIndex = 99

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("out of bounds action = %q, want empty", action)
	}
}

func TestMenuXPositions(t *testing.T) {
	mb := testMenuBar(120)
	positions := mb.MenuXPositions()

	if len(positions) != len(mb.Menus) {
		t.Fatalf("positions len = %d, want %d", len(positions), len(mb.Menus))
	}

	// First menu should start at x=1
	if positions[0] != 1 {
		t.Errorf("first menu x = %d, want 1", positions[0])
	}

	// Positions should be increasing
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("positions not increasing: %d <= %d", positions[i], positions[i-1])
		}
	}
}

func TestMenuAtX(t *testing.T) {
	mb := testMenuBar(120)

	// Click on first menu label area
	idx := mb.MenuAtX(2)
	if idx != 0 {
		t.Errorf("MenuAtX(2) = %d, want 0", idx)
	}

	// Click outside any menu
	idx = mb.MenuAtX(100)
	if idx != -1 {
		t.Errorf("MenuAtX(100) = %d, want -1", idx)
	}
}

func TestDropdownItemAtY(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)

	idx := mb.DropdownItemAtY(0)
	if idx != 0 {
		t.Errorf("DropdownItemAtY(0) = %d, want 0", idx)
	}

	idx = mb.DropdownItemAtY(1)
	if idx != 1 {
		t.Errorf("DropdownItemAtY(1) = %d, want 1", idx)
	}

	idx = mb.DropdownItemAtY(99)
	if idx != -1 {
		t.Errorf("DropdownItemAtY(99) = %d, want -1", idx)
	}

	idx = mb.DropdownItemAtY(-1)
	if idx != -1 {
		t.Errorf("DropdownItemAtY(-1) = %d, want -1", idx)
	}
}

func TestDropdownItemNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	idx := mb.DropdownItemAtY(0)
	if idx != -1 {
		t.Errorf("DropdownItemAtY with no menu = %d, want -1", idx)
	}
}

func TestRender(t *testing.T) {
	mb := testMenuBar(80)
	rendered := mb.Render(80)

	if len(rendered) == 0 {
		t.Error("expected non-empty render")
	}

	// Should contain menu labels
	if !strings.Contains(rendered, "File") {
		t.Error("expected 'File' in render")
	}
	if !strings.Contains(rendered, "Help") {
		t.Error("expected 'Help' in render")
	}

	// Should contain clock (PM or AM)
	if !strings.Contains(rendered, "M") { // AM or PM
		t.Error("expected clock in render")
	}
}

func TestRenderOpenMenu(t *testing.T) {
	mb := testMenuBar(80)
	mb.OpenMenu(0)
	rendered := mb.Render(80)

	// Open menu label should be present (highlight applied at render time, no brackets)
	if !strings.Contains(rendered, " File ") {
		t.Errorf("open menu label not found: %q", rendered)
	}
}

func TestRenderDropdown(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)

	lines := mb.RenderDropdown()
	if len(lines) == 0 {
		t.Fatal("expected dropdown lines")
	}

	// First and last lines should be borders
	if !strings.HasPrefix(lines[0], "┌") {
		t.Error("expected top border")
	}
	if !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Error("expected bottom border")
	}

	// Should contain menu items
	allLines := strings.Join(lines, "\n")
	if !strings.Contains(allLines, "New Terminal") {
		t.Error("expected 'New Terminal' in dropdown")
	}
	// Should have a shortcut
	if !strings.Contains(allLines, " n") {
		t.Error("expected shortcut in dropdown")
	}
}

func TestRenderDropdownSeparator(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu has a separator

	lines := mb.RenderDropdown()
	// Should have a separator line ├───┤
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "├") && strings.HasSuffix(line, "┤") {
			found = true
		}
	}
	if !found {
		t.Error("expected separator line ├───┤ in dropdown")
	}
}

func TestRenderDropdownHover(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)
	mb.HoverIndex = 0

	lines := mb.RenderDropdown()
	// Hovered item should be present (highlight applied via lipgloss, no > indicator)
	if len(lines) < 2 {
		t.Fatal("expected dropdown lines with items")
	}
	// First item (New Terminal) should appear in the dropdown
	found := false
	for _, line := range lines {
		if strings.Contains(line, "New Terminal") {
			found = true
		}
	}
	if !found {
		t.Error("expected hovered item label in dropdown")
	}
}

func TestRenderDropdownNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	lines := mb.RenderDropdown()
	if lines != nil {
		t.Error("expected nil dropdown when no menu open")
	}
}

func TestRightZones(t *testing.T) {
	mb := testMenuBar(120)

	zones := mb.RightZones(120)
	// Zones should not overlap
	for i := 1; i < len(zones); i++ {
		if zones[i].Start < zones[i-1].End {
			t.Errorf("zone %q (start=%d) overlaps with %q (end=%d)",
				zones[i].Type, zones[i].Start, zones[i-1].Type, zones[i-1].End)
		}
	}
	// All zones should have positive width
	for _, z := range zones {
		if z.End <= z.Start {
			t.Errorf("zone %q has zero/negative width: start=%d end=%d", z.Type, z.Start, z.End)
		}
	}
}

func TestMenuCount(t *testing.T) {
	mb := testMenuBar(120)
	// Should have 5 menus: File, Edit, Apps, View, Help
	if len(mb.Menus) != 5 {
		t.Errorf("menu count = %d, want 5", len(mb.Menus))
	}
	want := []string{"File", "Edit", "Apps", "View", "Help"}
	for i, w := range want {
		if i < len(mb.Menus) && mb.Menus[i].Label != w {
			t.Errorf("menu %d = %q, want %s", i, mb.Menus[i].Label, w)
		}
	}
}

// --- Submenu tests ---

func TestEnterSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu

	// Navigate to "Themes" item (has SubItems)
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		item := mb.Menus[3].Items[mb.HoverIndex]
		if len(item.SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	if !mb.HasSubMenu() {
		t.Fatal("expected test item to have submenu")
	}

	ok := mb.EnterSubMenu()
	if !ok {
		t.Fatal("EnterSubMenu returned false")
	}
	if !mb.InSubMenu {
		t.Error("InSubMenu should be true")
	}
	if mb.SubHoverIndex < 0 {
		t.Error("SubHoverIndex should be >= 0")
	}
}

func TestExitSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()
	if !mb.InSubMenu {
		t.Fatal("should be in submenu")
	}

	mb.ExitSubMenu()
	if mb.InSubMenu {
		t.Error("InSubMenu should be false after ExitSubMenu")
	}
	if mb.SubHoverIndex != -1 {
		t.Errorf("SubHoverIndex = %d, want -1", mb.SubHoverIndex)
	}
}

func TestHasSubMenuFalse(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu — no submenus
	if mb.HasSubMenu() {
		t.Error("File menu items should not have submenus")
	}
}

func TestEnterSubMenuNoSubItems(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu — no submenus
	ok := mb.EnterSubMenu()
	if ok {
		t.Error("EnterSubMenu should return false for non-submenu item")
	}
}

func TestMoveHoverInSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()
	start := mb.SubHoverIndex

	// Move down
	mb.MoveHover(1)
	if mb.SubHoverIndex == start {
		t.Error("SubHoverIndex should change after MoveHover(1)")
	}

	// Move up — should wrap or go back
	for i := 0; i < 20; i++ {
		mb.MoveHover(-1)
	}
	// Should still be in submenu and valid index
	if !mb.InSubMenu {
		t.Error("should still be in submenu")
	}
	if mb.SubHoverIndex < 0 {
		t.Error("SubHoverIndex should be >= 0")
	}
}

func TestMoveHoverSubMenuSkipsSeparator(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()

	// The Themes submenu has a separator (between original and new themes).
	// Cycle through all items — hover should never land on a disabled/separator item.
	subItems := mb.Menus[3].Items[mb.HoverIndex].SubItems
	for i := 0; i < len(subItems)+2; i++ {
		mb.MoveHover(1)
		if mb.SubHoverIndex >= 0 && mb.SubHoverIndex < len(subItems) {
			if subItems[mb.SubHoverIndex].Disabled {
				t.Errorf("hover landed on disabled item at index %d", mb.SubHoverIndex)
			}
		}
	}
}

func TestSelectedActionInSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	// Without entering submenu, parent item returns ""
	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("submenu parent action = %q, want empty", action)
	}

	// Enter submenu and check first item
	mb.EnterSubMenu()
	action = mb.SelectedAction()
	if !strings.HasPrefix(action, "theme_") {
		t.Errorf("submenu action = %q, want theme_*", action)
	}
}

func TestSubMenuParentIndex(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	itemY, parentWidth := mb.SubMenuParentIndex()
	if itemY < 0 {
		t.Error("SubMenuParentIndex returned -1, expected valid index")
	}
	if parentWidth <= 0 {
		t.Error("parent dropdown width should be > 0")
	}
}

func TestSubMenuParentIndexNoSubmenu(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu — no submenus
	itemY, _ := mb.SubMenuParentIndex()
	if itemY != -1 {
		t.Errorf("SubMenuParentIndex = %d, want -1 for non-submenu item", itemY)
	}
}

func TestSubMenuItemAtY(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	idx := mb.SubMenuItemAtY(0)
	if idx != 0 {
		t.Errorf("SubMenuItemAtY(0) = %d, want 0", idx)
	}

	idx = mb.SubMenuItemAtY(3)
	if idx != 3 {
		t.Errorf("SubMenuItemAtY(3) = %d, want 3", idx)
	}

	idx = mb.SubMenuItemAtY(-1)
	if idx != -1 {
		t.Errorf("SubMenuItemAtY(-1) = %d, want -1", idx)
	}

	idx = mb.SubMenuItemAtY(99)
	if idx != -1 {
		t.Errorf("SubMenuItemAtY(99) = %d, want -1", idx)
	}
}

func TestSubMenuItemAtYNoSubmenu(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)
	idx := mb.SubMenuItemAtY(0)
	if idx != -1 {
		t.Errorf("SubMenuItemAtY with no submenu = %d, want -1", idx)
	}
}

func TestRenderSubMenuStyled(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu
	// Navigate to submenu item
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()

	result := mb.RenderSubMenuStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty submenu render")
	}
	if !strings.Contains(result, "Alpha") {
		t.Error("expected 'Alpha' in submenu render")
	}
}

func TestRenderSubMenuStyledNoSubmenu(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu
	result := mb.RenderSubMenuStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result != "" {
		t.Errorf("expected empty submenu render for non-submenu item, got %q", result)
	}
}

func TestRenderDropdownStyledShowsArrow(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu (has Test Themes submenu)
	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if !strings.Contains(result, "►") {
		t.Error("expected ► arrow for submenu item in styled dropdown")
	}
}

func TestCloseMenuResetsSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3)
	// Navigate to submenu item and enter submenu
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()

	mb.CloseMenu()
	if mb.InSubMenu {
		t.Error("CloseMenu should reset InSubMenu")
	}
	if mb.SubHoverIndex != -1 {
		t.Error("CloseMenu should reset SubHoverIndex")
	}
}

func TestMoveMenuResetsSubMenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3)
	// Navigate to submenu item and enter submenu
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}
	mb.EnterSubMenu()

	mb.MoveMenu(1) // Move to Help menu
	if mb.InSubMenu {
		t.Error("MoveMenu should reset InSubMenu")
	}
	if mb.SubHoverIndex != -1 {
		t.Error("MoveMenu should reset SubHoverIndex")
	}
}

func TestAppsMenuFromRegistry(t *testing.T) {
	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "$SHELL"},
		{Name: "Calculator", Icon: "\uf1ec", Command: "termdesk-calc"},
	}
	wb := widget.NewDefaultBar("user")
	mb := New(120, config.DefaultKeyBindings(), registry, wb)

	// Find Apps menu
	var appsMenu *Menu
	for i := range mb.Menus {
		if mb.Menus[i].Label == "Apps" {
			appsMenu = &mb.Menus[i]
			break
		}
	}
	if appsMenu == nil {
		t.Fatal("Apps menu not found")
	}
	if len(appsMenu.Items) != 2 {
		t.Errorf("Apps menu items = %d, want 2", len(appsMenu.Items))
	}
	if appsMenu.Items[1].Action != "launch:termdesk-calc" {
		t.Errorf("calc action = %q, want launch:termdesk-calc", appsMenu.Items[1].Action)
	}
}

// --- Additional coverage tests ---

func TestNewWithGameEntries(t *testing.T) {
	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "$SHELL", Category: "tools"},
		{Name: "Snake", Icon: "\uf11b", Command: "snake", Category: "games"},
		{Name: "Tetris", Icon: "\uf11b", Command: "tetris", Category: "games"},
	}
	wb := widget.NewDefaultBar("user")
	mb := New(120, config.DefaultKeyBindings(), registry, wb)

	var appsMenu *Menu
	for i := range mb.Menus {
		if mb.Menus[i].Label == "Apps" {
			appsMenu = &mb.Menus[i]
			break
		}
	}
	if appsMenu == nil {
		t.Fatal("Apps menu not found")
	}

	// Should have: Terminal, separator, Games submenu = 3 items
	if len(appsMenu.Items) != 3 {
		t.Errorf("Apps menu items = %d, want 3", len(appsMenu.Items))
	}

	// Last item should be the Games submenu parent
	gamesItem := appsMenu.Items[len(appsMenu.Items)-1]
	if len(gamesItem.SubItems) != 2 {
		t.Errorf("Games submenu items = %d, want 2", len(gamesItem.SubItems))
	}
	if gamesItem.SubItems[0].Action != "launch:snake" {
		t.Errorf("first game action = %q, want launch:snake", gamesItem.SubItems[0].Action)
	}
}

func TestNewWithEmptyRegistry(t *testing.T) {
	wb := widget.NewDefaultBar("user")
	mb := New(80, config.DefaultKeyBindings(), nil, wb)

	var appsMenu *Menu
	for i := range mb.Menus {
		if mb.Menus[i].Label == "Apps" {
			appsMenu = &mb.Menus[i]
			break
		}
	}
	if appsMenu == nil {
		t.Fatal("Apps menu not found")
	}
	if len(appsMenu.Items) != 0 {
		t.Errorf("Apps menu items = %d, want 0 for empty registry", len(appsMenu.Items))
	}
}

func TestOpenMenuAllDisabledFirstItems(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "─", Disabled: true},
				{Label: "─", Disabled: true},
				{Label: "Real Item", Action: "real"},
			}},
		},
		OpenIndex:  -1,
		HoverIndex: -1,
	}
	mb.OpenMenu(0)

	if mb.HoverIndex != 2 {
		t.Errorf("HoverIndex = %d, want 2 (first non-disabled)", mb.HoverIndex)
	}
}

func TestOpenMenuAllItemsDisabled(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "─", Disabled: true},
				{Label: "─", Disabled: true},
			}},
		},
		OpenIndex:  -1,
		HoverIndex: -1,
	}
	mb.OpenMenu(0)

	// All items disabled: HoverIndex wraps back to 0
	if mb.HoverIndex != 0 {
		t.Errorf("HoverIndex = %d, want 0 (all disabled fallback)", mb.HoverIndex)
	}
}

func TestMoveHoverEmptyMenu(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Empty", Items: []MenuItem{}},
		},
		OpenIndex:  0,
		HoverIndex: 0,
	}

	result := mb.MoveHover(1)
	if result != -1 {
		t.Errorf("MoveHover on empty menu = %d, want -1", result)
	}
}

func TestMoveHoverInSubMenuNoSubItems(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "Plain", Action: "plain"},
			}},
		},
		OpenIndex:     0,
		HoverIndex:    0,
		InSubMenu:     true,
		SubHoverIndex: 0,
	}

	result := mb.MoveHover(1)
	if mb.InSubMenu {
		t.Error("InSubMenu should be false after MoveHover with no sub items")
	}
	if mb.SubHoverIndex != -1 {
		t.Errorf("SubHoverIndex = %d, want -1", mb.SubHoverIndex)
	}
	if result != 0 {
		t.Errorf("result = %d, want 0 (HoverIndex)", result)
	}
}

func TestHoveredItemOutOfBounds(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "One", Action: "one"},
			}},
		},
		OpenIndex:  0,
		HoverIndex: 99,
	}

	if mb.HasSubMenu() {
		t.Error("HasSubMenu should be false when HoverIndex is out of bounds")
	}
}

func TestHoveredItemNoMenuOpen(t *testing.T) {
	mb := testMenuBar(120)
	if mb.HasSubMenu() {
		t.Error("HasSubMenu should be false when no menu is open")
	}
}

func TestEnterSubMenuAllDisabled(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "Parent", SubItems: []MenuItem{
					{Label: "─", Disabled: true},
					{Label: "─", Disabled: true},
				}},
			}},
		},
		OpenIndex:  0,
		HoverIndex: 0,
	}

	ok := mb.EnterSubMenu()
	if !ok {
		t.Fatal("EnterSubMenu should return true even if all sub items disabled")
	}
	if mb.SubHoverIndex != 0 {
		t.Errorf("SubHoverIndex = %d, want 0 (all disabled fallback)", mb.SubHoverIndex)
	}
}

func TestSelectedActionDisabledSubItem(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "Parent", SubItems: []MenuItem{
					{Label: "─", Disabled: true},
					{Label: "Good", Action: "good"},
				}},
			}},
		},
		OpenIndex:     0,
		HoverIndex:    0,
		InSubMenu:     true,
		SubHoverIndex: 0,
	}

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("action for disabled sub-item = %q, want empty", action)
	}
}

func TestDropdownDimensionsOpen(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)

	w, h := mb.DropdownDimensions()
	if w <= 0 {
		t.Errorf("dropdown width = %d, want > 0", w)
	}
	if h <= 0 {
		t.Errorf("dropdown height = %d, want > 0", h)
	}

	expectedH := len(mb.Menus[0].Items) + 2
	if h != expectedH {
		t.Errorf("dropdown height = %d, want %d", h, expectedH)
	}
}

func TestDropdownDimensionsNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	w, h := mb.DropdownDimensions()
	if w != 0 || h != 0 {
		t.Errorf("DropdownDimensions with no menu = (%d, %d), want (0, 0)", w, h)
	}
}

func TestDropdownDimensionsEmptyMenu(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Empty", Items: []MenuItem{}},
		},
		OpenIndex:  0,
		HoverIndex: -1,
	}

	w, h := mb.DropdownDimensions()
	if w != 0 || h != 0 {
		t.Errorf("DropdownDimensions for empty menu = (%d, %d), want (0, 0)", w, h)
	}
}

func TestDropdownDimensionsWithShortcuts(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)

	w, h := mb.DropdownDimensions()
	if w < 10 {
		t.Errorf("dropdown width = %d, expected reasonable width for File menu", w)
	}
	if h < 4 {
		t.Errorf("dropdown height = %d, expected at least 4 for File menu borders + items", h)
	}
}

func TestDropdownDimensionsWithSubmenu(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu (has Test Themes submenu)

	w, h := mb.DropdownDimensions()
	if w <= 0 || h <= 0 {
		t.Errorf("DropdownDimensions = (%d, %d), want positive", w, h)
	}
}

func TestRenderDropdownStyledNoShortcutNoSubmenu(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(1) // Edit menu
	mb.HoverIndex = 0

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown for Edit menu")
	}
	if !strings.Contains(result, "Paste") {
		t.Error("expected 'Paste' in styled dropdown")
	}
}

func TestEditMenuContainsCopyModeActions(t *testing.T) {
	mb := testMenuBar(120)
	edit := mb.Menus[1]

	labels := make([]string, 0, len(edit.Items))
	for _, it := range edit.Items {
		labels = append(labels, it.Label)
	}
	joined := strings.Join(labels, "|")
	for _, want := range []string{"Enter Copy Mode", "Copy Search Forward", "Copy Search Backward"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("edit menu missing %q: %v", want, labels)
		}
	}
}

func TestViewMenuContainsSwapActions(t *testing.T) {
	mb := testMenuBar(120)
	view := mb.Menus[3]

	labels := make([]string, 0, len(view.Items))
	for _, it := range view.Items {
		labels = append(labels, it.Label)
	}
	joined := strings.Join(labels, "|")
	for _, want := range []string{"Swap Left", "Swap Right", "Swap Up", "Swap Down"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("view menu missing %q: %v", want, labels)
		}
	}
}

func TestToggleTilingLabelDefaultsAndUpdates(t *testing.T) {
	mb := testMenuBar(120)
	got := ""
	for _, menu := range mb.Menus {
		for _, item := range menu.Items {
			if item.Action == "toggle_tiling" {
				got = item.Label
				break
			}
		}
	}
	if !strings.HasSuffix(got, "Tiling: Off (Columns)") {
		t.Fatalf("default toggle label = %q, want suffix %q", got, "Tiling: Off (Columns)")
	}

	mb.SetToggleTilingLabel(true, "Rows")
	got = ""
	for _, menu := range mb.Menus {
		for _, item := range menu.Items {
			if item.Action == "toggle_tiling" {
				got = item.Label
				break
			}
		}
	}
	if !strings.HasSuffix(got, "Tiling: On (Rows)") {
		t.Fatalf("updated toggle label = %q, want suffix %q", got, "Tiling: On (Rows)")
	}
}

func TestRenderDropdownStyledHoveredSubmenuParent(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu

	// Navigate to the submenu parent item (Test Themes)
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown")
	}
	if !strings.Contains(result, "►") {
		t.Error("expected ► arrow for submenu parent")
	}
}

func TestRenderDropdownStyledNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result != "" {
		t.Errorf("expected empty styled dropdown with no menu open, got length %d", len(result))
	}
}

func TestRenderDropdownStyledEmptyMenu(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Empty", Items: []MenuItem{}},
		},
		OpenIndex:  0,
		HoverIndex: -1,
	}

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result != "" {
		t.Errorf("expected empty styled dropdown for empty menu, got length %d", len(result))
	}
}

func TestRenderDropdownStyledDisabledItem(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0) // File menu has separators (disabled items)

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown for File menu")
	}
	if !strings.Contains(result, "─") {
		t.Error("expected separator in styled dropdown")
	}
}

func TestRenderDropdownStyledWithShortcutNotHovered(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenMenu(0)
	mb.HoverIndex = 2 // New Workspace (not index 0)

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown")
	}
	if !strings.Contains(result, "New Terminal") {
		t.Error("expected 'New Terminal' in dropdown")
	}
}

func TestRenderDropdownStyledInSubMenuMode(t *testing.T) {
	mb := testMenuBarWithSubmenu(120)
	mb.OpenMenu(3) // View menu

	// Navigate to submenu parent
	for mb.HoverIndex < len(mb.Menus[3].Items) {
		if len(mb.Menus[3].Items[mb.HoverIndex].SubItems) > 0 {
			break
		}
		mb.MoveHover(1)
	}

	mb.EnterSubMenu()

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown while in submenu")
	}
}

func TestSubMenuParentIndexNoMenuOpen(t *testing.T) {
	mb := testMenuBar(120)
	itemY, parentWidth := mb.SubMenuParentIndex()
	if itemY != -1 {
		t.Errorf("SubMenuParentIndex with no menu = %d, want -1", itemY)
	}
	if parentWidth != 0 {
		t.Errorf("parentWidth with no menu = %d, want 0", parentWidth)
	}
}

func TestSubMenuParentIndexNegativeHover(t *testing.T) {
	mb := testMenuBar(120)
	mb.OpenIndex = 0
	mb.HoverIndex = -1
	itemY, _ := mb.SubMenuParentIndex()
	if itemY != -1 {
		t.Errorf("SubMenuParentIndex with HoverIndex=-1 = %d, want -1", itemY)
	}
}

func TestDropdownInnerWidthNoMenu(t *testing.T) {
	mb := testMenuBar(120)
	w := mb.dropdownInnerWidth()
	if w != 0 {
		t.Errorf("dropdownInnerWidth with no menu = %d, want 0", w)
	}
}

func TestRightZonesNilWidgetBar(t *testing.T) {
	mb := &MenuBar{
		Menus:     []Menu{{Label: "Test"}},
		OpenIndex: -1,
		Width:     80,
		WidgetBar: nil,
	}

	zones := mb.RightZones(80)
	if zones != nil {
		t.Errorf("RightZones with nil WidgetBar should be nil, got %v", zones)
	}
}

func TestRenderWithNilWidgetBar(t *testing.T) {
	mb := &MenuBar{
		Menus:     []Menu{{Label: "Test"}},
		OpenIndex: -1,
		Width:     80,
		WidgetBar: nil,
	}

	rendered := mb.Render(80)
	if !strings.Contains(rendered, "Test") {
		t.Error("expected 'Test' in render with nil WidgetBar")
	}
}

func TestRenderDropdownEmptyMenu(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Empty", Items: []MenuItem{}},
		},
		OpenIndex:  0,
		HoverIndex: -1,
	}

	lines := mb.RenderDropdown()
	if lines != nil {
		t.Errorf("expected nil dropdown for empty menu, got %v", lines)
	}
}

func TestRenderNarrowWidth(t *testing.T) {
	mb := testMenuBar(10)
	rendered := mb.Render(10)
	if len(rendered) == 0 {
		t.Error("expected non-empty render even for narrow width")
	}
}

func TestMoveHoverAllDisabledMenu(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "─", Disabled: true},
				{Label: "─", Disabled: true},
			}},
		},
		OpenIndex:  0,
		HoverIndex: 0,
	}

	result := mb.MoveHover(1)
	if result != 0 {
		t.Errorf("MoveHover on all-disabled = %d, want 0 (start)", result)
	}
}

func TestRightZonesWithWidgetBar(t *testing.T) {
	mb := testMenuBar(120)

	zones := mb.RightZones(120)
	if len(zones) == 0 {
		t.Error("expected zones from widget bar")
	}

	// Verify zones contain expected widget types
	typeSet := make(map[string]bool)
	for _, z := range zones {
		typeSet[z.Type] = true
	}
	if !typeSet["clock"] {
		t.Error("expected 'clock' zone from widget bar")
	}
}

func TestRenderRightWithWidgetBar(t *testing.T) {
	mb := testMenuBar(120)
	right := mb.renderRight()
	if right == "" {
		t.Error("expected non-empty renderRight with widget bar")
	}
}

func TestNewWithNilWidgetBar(t *testing.T) {
	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "$SHELL"},
	}
	mb := New(80, config.DefaultKeyBindings(), registry, nil)
	if mb.WidgetBar != nil {
		t.Error("expected nil WidgetBar")
	}

	rendered := mb.Render(80)
	if !strings.Contains(rendered, "File") {
		t.Error("expected menu labels in render with nil WidgetBar")
	}
}

func TestRenderDropdownStyledCheckmarkItem(t *testing.T) {
	// Test an item whose label starts with a checkmark (simulating a toggle)
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "View", Items: []MenuItem{
				{Label: "\u2713 Animations", Action: "toggle_animations"},
				{Label: "  Show Desktop", Shortcut: "Ctrl+D", Action: "show_desktop"},
			}},
		},
		OpenIndex:  0,
		HoverIndex: 0,
	}

	result := mb.RenderDropdownStyled("#555", "#1E1E2E", "#FFF", "#61AFEF", "#CCC", "#888")
	if result == "" {
		t.Error("expected non-empty styled dropdown with checkmark item")
	}
	if !strings.Contains(result, "Animations") {
		t.Error("expected 'Animations' in styled dropdown")
	}
}

func TestMoveHoverSubMenuAllDisabled(t *testing.T) {
	mb := &MenuBar{
		Menus: []Menu{
			{Label: "Test", Items: []MenuItem{
				{Label: "Parent", SubItems: []MenuItem{
					{Label: "─", Disabled: true},
					{Label: "─", Disabled: true},
				}},
			}},
		},
		OpenIndex:     0,
		HoverIndex:    0,
		InSubMenu:     true,
		SubHoverIndex: 0,
	}

	// Should not infinite loop; breaks when returning to start
	result := mb.MoveHover(1)
	if result != 0 {
		t.Errorf("MoveHover in all-disabled submenu = %d, want 0", result)
	}
	if !mb.InSubMenu {
		t.Error("should still be in submenu")
	}
}
