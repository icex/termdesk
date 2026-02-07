package menubar

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	mb := New(120)
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
	mb := New(80)
	mb.SetWidth(120)
	if mb.Width != 120 {
		t.Errorf("width = %d, want 120", mb.Width)
	}
}

func TestOpenCloseMenu(t *testing.T) {
	mb := New(120)

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
	mb := New(120)
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
	mb := New(120)
	mb.OpenMenu(0) // File menu has 2 items

	// Start at 0, move down
	mb.MoveHover(1)
	if mb.HoverIndex != 1 {
		t.Errorf("hover = %d, want 1", mb.HoverIndex)
	}

	// Wrap around
	mb.MoveHover(1)
	if mb.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0 (wrapped)", mb.HoverIndex)
	}

	// Move up from 0 wraps to last
	mb.MoveHover(-1)
	if mb.HoverIndex != 1 {
		t.Errorf("hover = %d, want 1 (wrapped up)", mb.HoverIndex)
	}
}

func TestMoveHoverNoMenu(t *testing.T) {
	mb := New(120)
	result := mb.MoveHover(1)
	if result != -1 {
		t.Errorf("MoveHover with no menu = %d, want -1", result)
	}
}

func TestMoveMenu(t *testing.T) {
	mb := New(120)
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
	mb := New(120)
	mb.OpenMenu(0) // File: New Terminal, Quit
	mb.HoverIndex = 0

	action := mb.SelectedAction()
	if action != "new_terminal" {
		t.Errorf("action = %q, want new_terminal", action)
	}

	mb.HoverIndex = 1
	action = mb.SelectedAction()
	if action != "quit" {
		t.Errorf("action = %q, want quit", action)
	}
}

func TestSelectedActionDisabled(t *testing.T) {
	mb := New(120)
	mb.OpenMenu(1) // Edit: Copy (disabled), Paste (disabled)
	mb.HoverIndex = 0

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("disabled action = %q, want empty", action)
	}
}

func TestSelectedActionNoMenu(t *testing.T) {
	mb := New(120)
	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("no menu action = %q, want empty", action)
	}
}

func TestSelectedActionOutOfBounds(t *testing.T) {
	mb := New(120)
	mb.OpenMenu(0)
	mb.HoverIndex = 99

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("out of bounds action = %q, want empty", action)
	}
}

func TestMenuXPositions(t *testing.T) {
	mb := New(120)
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
	mb := New(120)

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
	mb := New(120)
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
	mb := New(120)
	idx := mb.DropdownItemAtY(0)
	if idx != -1 {
		t.Errorf("DropdownItemAtY with no menu = %d, want -1", idx)
	}
}

func TestRender(t *testing.T) {
	mb := New(80)
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
	mb := New(80)
	mb.OpenMenu(0)
	rendered := mb.Render(80)

	// Open menu should be bracketed
	if !strings.Contains(rendered, "[File]") {
		t.Errorf("open menu not bracketed: %q", rendered)
	}
}

func TestRenderDropdown(t *testing.T) {
	mb := New(120)
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
	if !strings.Contains(allLines, "Ctrl+N") {
		t.Error("expected shortcut in dropdown")
	}
}

func TestRenderDropdownHover(t *testing.T) {
	mb := New(120)
	mb.OpenMenu(0)
	mb.HoverIndex = 0

	lines := mb.RenderDropdown()
	// Hovered item should have > indicator
	found := false
	for _, line := range lines {
		if strings.Contains(line, ">") {
			found = true
		}
	}
	if !found {
		t.Error("expected hover indicator > in dropdown")
	}
}

func TestRenderDropdownNoMenu(t *testing.T) {
	mb := New(120)
	lines := mb.RenderDropdown()
	if lines != nil {
		t.Error("expected nil dropdown when no menu open")
	}
}

func TestRenderNoIndicators(t *testing.T) {
	mb := New(80)
	mb.ShowClock = false
	mb.ShowCPU = false
	mb.ShowMemory = false
	rendered := mb.Render(80)

	if strings.Contains(rendered, "CPU") {
		t.Error("should not show CPU when disabled")
	}
	if strings.Contains(rendered, "MEM") {
		t.Error("should not show MEM when disabled")
	}
}

func TestFormatCPU(t *testing.T) {
	s := FormatCPU(12.5)
	if s != "CPU:12%" && s != "CPU:13%" {
		t.Errorf("FormatCPU(12.5) = %q", s)
	}
}

func TestFormatMemory(t *testing.T) {
	s := FormatMemory(4.2)
	if s != "MEM:4.2G" {
		t.Errorf("FormatMemory(4.2) = %q", s)
	}
}

func TestClockString(t *testing.T) {
	s := ClockString()
	if len(s) == 0 {
		t.Error("empty clock string")
	}
	// Should match format "HH:MM AM/PM"
	if !strings.Contains(s, ":") {
		t.Errorf("clock = %q, expected HH:MM format", s)
	}
}
