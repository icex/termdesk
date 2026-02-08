package menubar

import (
	"strings"
	"testing"

	"github.com/icex/termdesk/internal/config"
)

func TestNew(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(80, config.DefaultKeyBindings())
	mb.SetWidth(120)
	if mb.Width != 120 {
		t.Errorf("width = %d, want 120", mb.Width)
	}
}

func TestOpenCloseMenu(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())

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
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
	mb.OpenMenu(0) // File menu: New Terminal (0), Minimize (1), separator (2), Quit (3)

	// Start at 0, move down → should go to 1
	mb.MoveHover(1)
	if mb.HoverIndex != 1 {
		t.Errorf("hover = %d, want 1", mb.HoverIndex)
	}

	// Move down → should skip separator (2) and go to 3 (Detach)
	mb.MoveHover(1)
	if mb.HoverIndex != 3 {
		t.Errorf("hover = %d, want 3 (skip separator)", mb.HoverIndex)
	}

	// Move down → go to 4 (Quit)
	mb.MoveHover(1)
	if mb.HoverIndex != 4 {
		t.Errorf("hover = %d, want 4 (Quit)", mb.HoverIndex)
	}

	// Move down → wrap to 0
	mb.MoveHover(1)
	if mb.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0 (wrap)", mb.HoverIndex)
	}

	// Move up from 0 → wrap to last selectable (4)
	mb.MoveHover(-1)
	if mb.HoverIndex != 4 {
		t.Errorf("hover = %d, want 4 (wrap up)", mb.HoverIndex)
	}

	// Move up from 4 → go to 3 (Detach)
	mb.MoveHover(-1)
	if mb.HoverIndex != 3 {
		t.Errorf("hover = %d, want 3 (Detach)", mb.HoverIndex)
	}

	// Move up from 3 → skip separator (2) and go to 1
	mb.MoveHover(-1)
	if mb.HoverIndex != 1 {
		t.Errorf("hover = %d, want 1 (skip separator up)", mb.HoverIndex)
	}
}

func TestMoveHoverNoMenu(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	result := mb.MoveHover(1)
	if result != -1 {
		t.Errorf("MoveHover with no menu = %d, want -1", result)
	}
}

func TestMoveMenu(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
	mb.OpenMenu(0) // File: New Terminal, Minimize, separator, Quit
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
	mb := New(120, config.DefaultKeyBindings())
	// File menu (index 0) has a separator at index 2
	mb.OpenMenu(0)
	mb.HoverIndex = 2 // separator

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("disabled/separator action = %q, want empty", action)
	}
}

func TestSelectedActionNoMenu(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("no menu action = %q, want empty", action)
	}
}

func TestSelectedActionOutOfBounds(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	mb.OpenMenu(0)
	mb.HoverIndex = 99

	action := mb.SelectedAction()
	if action != "" {
		t.Errorf("out of bounds action = %q, want empty", action)
	}
}

func TestMenuXPositions(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())

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
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
	idx := mb.DropdownItemAtY(0)
	if idx != -1 {
		t.Errorf("DropdownItemAtY with no menu = %d, want -1", idx)
	}
}

func TestRender(t *testing.T) {
	mb := New(80, config.DefaultKeyBindings())
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
	mb := New(80, config.DefaultKeyBindings())
	mb.OpenMenu(0)
	rendered := mb.Render(80)

	// Open menu label should be present (highlight applied at render time, no brackets)
	if !strings.Contains(rendered, " File ") {
		t.Errorf("open menu label not found: %q", rendered)
	}
}

func TestRenderDropdown(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
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
	mb := New(120, config.DefaultKeyBindings())
	lines := mb.RenderDropdown()
	if lines != nil {
		t.Error("expected nil dropdown when no menu open")
	}
}

func TestRenderNoIndicators(t *testing.T) {
	mb := New(80, config.DefaultKeyBindings())
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
	if !strings.Contains(s, "12%") && !strings.Contains(s, "13%") {
		t.Errorf("FormatCPU(12.5) = %q, expected percentage", s)
	}
}

func TestFormatMemory(t *testing.T) {
	s := FormatMemory(4.2)
	if !strings.Contains(s, "4.2G") {
		t.Errorf("FormatMemory(4.2) = %q, expected memory value", s)
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

func TestFormatBattery(t *testing.T) {
	tests := []struct {
		pct      float64
		charging bool
		wantPct  string
		wantBolt bool
	}{
		{95, false, "95%", false},
		{72, true, "72%", true},
		{45, false, "45%", false},
		{25, false, "25%", false},
		{8, true, "8%", true},
		{100, false, "100%", false},
		{0, false, "0%", false},
	}
	for _, tt := range tests {
		s := FormatBattery(tt.pct, tt.charging)
		if !strings.Contains(s, tt.wantPct) {
			t.Errorf("FormatBattery(%v, %v) = %q, missing %q", tt.pct, tt.charging, s, tt.wantPct)
		}
		hasBolt := strings.Contains(s, "\u26a1")
		if hasBolt != tt.wantBolt {
			t.Errorf("FormatBattery(%v, %v) bolt=%v, want %v", tt.pct, tt.charging, hasBolt, tt.wantBolt)
		}
	}
}

func TestBatColorLevel(t *testing.T) {
	if BatColorLevel(80) != "green" {
		t.Error("80% should be green")
	}
	if BatColorLevel(50) != "green" {
		t.Error("50% should be green")
	}
	if BatColorLevel(30) != "yellow" {
		t.Error("30% should be yellow")
	}
	if BatColorLevel(20) != "yellow" {
		t.Error("20% should be yellow")
	}
	if BatColorLevel(10) != "red" {
		t.Error("10% should be red")
	}
	if BatColorLevel(0) != "red" {
		t.Error("0% should be red")
	}
}

func TestRightZonesRuneWidth(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	mb.ShowCPU = true
	mb.ShowMemory = true
	mb.ShowBattery = false
	mb.ShowClock = true
	mb.CPUPct = 25
	mb.MemGB = 4.0

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

func TestRightZonesWithBattery(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	mb.ShowCPU = true
	mb.ShowMemory = true
	mb.ShowBattery = true
	mb.BatPresent = true
	mb.BatPct = 75
	mb.ShowClock = true

	zones := mb.RightZones(120)

	// Should have 4 zones: cpu, mem, bat, clock
	types := make(map[string]bool)
	for _, z := range zones {
		types[z.Type] = true
	}
	for _, want := range []string{"cpu", "mem", "bat", "clock"} {
		if !types[want] {
			t.Errorf("missing zone type %q", want)
		}
	}

	// Zones should not overlap
	for i := 1; i < len(zones); i++ {
		if zones[i].Start < zones[i-1].End {
			t.Errorf("zone %q overlaps with %q", zones[i].Type, zones[i-1].Type)
		}
	}
}

func TestRightZonesNoBattery(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	mb.ShowBattery = true
	mb.BatPresent = false // no battery on system

	zones := mb.RightZones(120)
	for _, z := range zones {
		if z.Type == "bat" {
			t.Error("should not have battery zone when BatPresent=false")
		}
	}
}

func TestRenderRightWithUsername(t *testing.T) {
	mb := New(80, config.DefaultKeyBindings())
	mb.Username = "testuser"
	rendered := mb.Render(80)

	if !strings.Contains(rendered, "testuser") {
		t.Errorf("expected username in render: %q", rendered)
	}
}

func TestRenderRightWithBattery(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	mb.ShowBattery = true
	mb.BatPresent = true
	mb.BatPct = 85

	rendered := mb.Render(120)
	if !strings.Contains(rendered, "85%") {
		t.Errorf("expected battery percentage in render: %q", rendered)
	}
}

func TestMenuCount(t *testing.T) {
	mb := New(120, config.DefaultKeyBindings())
	// Should have 4 menus: File, Apps, View, Help (no Edit)
	if len(mb.Menus) != 4 {
		t.Errorf("menu count = %d, want 4", len(mb.Menus))
	}
	if mb.Menus[0].Label != "File" {
		t.Errorf("menu 0 = %q, want File", mb.Menus[0].Label)
	}
	if mb.Menus[1].Label != "Apps" {
		t.Errorf("menu 1 = %q, want Apps", mb.Menus[1].Label)
	}
	if mb.Menus[2].Label != "View" {
		t.Errorf("menu 2 = %q, want View", mb.Menus[2].Label)
	}
	if mb.Menus[3].Label != "Help" {
		t.Errorf("menu 3 = %q, want Help", mb.Menus[3].Label)
	}
}
