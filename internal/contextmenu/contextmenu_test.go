package contextmenu

import "testing"

func TestDesktopMenu(t *testing.T) {
	m := DesktopMenu(10, 5, KeyBindings{})
	if !m.Visible {
		t.Error("menu should be visible")
	}
	if m.X != 10 || m.Y != 5 {
		t.Errorf("position = (%d,%d), want (10,5)", m.X, m.Y)
	}
	if len(m.Items) < 5 {
		t.Errorf("expected at least 5 items, got %d", len(m.Items))
	}
	// First item should be "New Terminal"
	if m.Items[0].Action != "new_terminal" {
		t.Errorf("first item action = %q, want new_terminal", m.Items[0].Action)
	}
}

func TestTitleBarMenu(t *testing.T) {
	m := TitleBarMenu(20, 3, true, false, KeyBindings{})
	if !m.Visible {
		t.Error("menu should be visible")
	}
	if m.Items[0].Action != "close_window" {
		t.Errorf("first item action = %q, want close_window", m.Items[0].Action)
	}
}

func TestMoveHover(t *testing.T) {
	m := DesktopMenu(0, 0, KeyBindings{})
	// Initial hover is 0 (New Terminal)
	if m.HoverIndex != 0 {
		t.Errorf("initial hover = %d, want 0", m.HoverIndex)
	}

	m.MoveHover(1)
	// Should skip separator at index 1
	if m.Items[m.HoverIndex].Disabled {
		t.Error("hover should skip disabled items")
	}

	// Move all the way around
	for range 20 {
		m.MoveHover(1)
	}
	if m.Items[m.HoverIndex].Disabled {
		t.Error("hover should never be on disabled item")
	}

	// Move backward
	for range 20 {
		m.MoveHover(-1)
	}
	if m.Items[m.HoverIndex].Disabled {
		t.Error("hover should never be on disabled item (backward)")
	}
}

func TestSelectedAction(t *testing.T) {
	m := DesktopMenu(0, 0, KeyBindings{})
	action := m.SelectedAction()
	if action != "new_terminal" {
		t.Errorf("action = %q, want new_terminal", action)
	}

	// Force to separator
	m.HoverIndex = 1
	action = m.SelectedAction()
	if action != "" {
		t.Errorf("disabled item action = %q, want empty", action)
	}
}

func TestItemAtY(t *testing.T) {
	m := DesktopMenu(0, 0, KeyBindings{})
	if m.ItemAtY(0) != 0 {
		t.Error("ItemAtY(0) should return 0")
	}
	if m.ItemAtY(-1) != -1 {
		t.Error("ItemAtY(-1) should return -1")
	}
	if m.ItemAtY(99) != -1 {
		t.Error("ItemAtY(99) should return -1")
	}
}

func TestHide(t *testing.T) {
	m := DesktopMenu(0, 0, KeyBindings{})
	m.Hide()
	if m.Visible {
		t.Error("should be hidden after Hide()")
	}
}

func TestInnerWidth(t *testing.T) {
	m := &Menu{
		Items: []Item{
			{Label: "AB", Action: "a"},
			{Label: "ABCDE", Action: "b"},
			{Label: "C", Action: "c"},
		},
	}
	// Longest label is "ABCDE" = 5 runes, plus 4 padding = 9
	got := m.InnerWidth()
	if got != 9 {
		t.Errorf("InnerWidth() = %d, want 9", got)
	}

	// Test with empty items
	empty := &Menu{}
	if empty.InnerWidth() != 2 {
		t.Errorf("InnerWidth() for empty menu = %d, want 2", empty.InnerWidth())
	}

	// Test with multi-byte UTF-8 label (emoji/icon)
	m2 := &Menu{
		Items: []Item{
			{Label: "Hello"},
			{Label: "\u2603 Snow"}, // snowman is 1 rune + " Snow" = 6 runes
		},
	}
	got2 := m2.InnerWidth()
	if got2 != 10 { // 6 + 4 = 10
		t.Errorf("InnerWidth() with unicode = %d, want 10", got2)
	}
}

func TestWidth(t *testing.T) {
	m := &Menu{
		Items: []Item{
			{Label: "Hello", Action: "a"},
		},
	}
	// InnerWidth = 5 + 4 = 9, Width = 9 + 2 = 11
	got := m.Width()
	if got != 11 {
		t.Errorf("Width() = %d, want 11", got)
	}
}

func TestHeight(t *testing.T) {
	m := &Menu{
		Items: []Item{
			{Label: "A"},
			{Label: "B"},
			{Label: "C"},
		},
	}
	// Height = 3 items + 2 border = 5
	got := m.Height()
	if got != 5 {
		t.Errorf("Height() = %d, want 5", got)
	}

	// Empty menu
	empty := &Menu{}
	if empty.Height() != 2 {
		t.Errorf("Height() for empty menu = %d, want 2", empty.Height())
	}
}

func TestContains(t *testing.T) {
	m := &Menu{
		X: 10, Y: 5,
		Items: []Item{
			{Label: "Item One", Action: "a"},
			{Label: "Item Two", Action: "b"},
		},
	}
	w := m.Width()
	h := m.Height()

	// Inside: top-left corner
	if !m.Contains(10, 5) {
		t.Error("Contains(10,5) should be true (top-left)")
	}

	// Inside: bottom-right edge minus 1
	if !m.Contains(10+w-1, 5+h-1) {
		t.Error("Contains at bottom-right edge should be true")
	}

	// Outside: just past right edge
	if m.Contains(10+w, 5) {
		t.Error("Contains just past right edge should be false")
	}

	// Outside: just past bottom edge
	if m.Contains(10, 5+h) {
		t.Error("Contains just past bottom edge should be false")
	}

	// Outside: left of menu
	if m.Contains(9, 5) {
		t.Error("Contains left of menu should be false")
	}

	// Outside: above menu
	if m.Contains(10, 4) {
		t.Error("Contains above menu should be false")
	}

	// Inside: middle of menu
	if !m.Contains(12, 6) {
		t.Error("Contains(12,6) should be true (middle)")
	}
}

func TestCopyModeMenu(t *testing.T) {
	m := CopyModeMenu(15, 20)
	if !m.Visible {
		t.Error("menu should be visible")
	}
	if m.X != 15 || m.Y != 20 {
		t.Errorf("position = (%d,%d), want (15,20)", m.X, m.Y)
	}
	if len(m.Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(m.Items))
	}
	// Check first item
	if m.Items[0].Action != "copy_selection" {
		t.Errorf("first item action = %q, want copy_selection", m.Items[0].Action)
	}
	// Check separator
	if !m.Items[2].Disabled {
		t.Error("item 2 should be a separator (disabled)")
	}
	// Check last item
	if m.Items[4].Action != "clear_selection" {
		t.Errorf("last item action = %q, want clear_selection", m.Items[4].Action)
	}
}

func TestDockItemMenu(t *testing.T) {
	m := DockItemMenu(5, 30)
	if !m.Visible {
		t.Error("menu should be visible")
	}
	if m.X != 5 || m.Y != 30 {
		t.Errorf("position = (%d,%d), want (5,30)", m.X, m.Y)
	}
	if len(m.Items) != 5 {
		t.Errorf("expected 5 items, got %d", len(m.Items))
	}
	// Check first item
	if m.Items[0].Action != "dock_focus_window" {
		t.Errorf("first item action = %q, want dock_focus_window", m.Items[0].Action)
	}
	// Check separator at index 3
	if !m.Items[3].Disabled {
		t.Error("item 3 should be a separator (disabled)")
	}
	// Check last item
	if m.Items[4].Action != "close_window" {
		t.Errorf("last item action = %q, want close_window", m.Items[4].Action)
	}
}

func TestClamp(t *testing.T) {
	// Menu that fits - no clamping needed
	t.Run("fits", func(t *testing.T) {
		m := &Menu{
			X: 0, Y: 1,
			Items: []Item{
				{Label: "Short", Action: "a"},
			},
		}
		w := m.Width()
		h := m.Height()
		m.Clamp(100, 100)
		if m.X != 0 || m.Y != 1 {
			t.Errorf("position changed when it should fit: (%d,%d)", m.X, m.Y)
		}
		_ = w
		_ = h
	})

	// Menu overflows right edge
	t.Run("overflow_right", func(t *testing.T) {
		m := &Menu{
			X: 90, Y: 1,
			Items: []Item{
				{Label: "Long Label Here", Action: "a"},
			},
		}
		screenW := 100
		m.Clamp(screenW, 100)
		if m.X+m.Width() > screenW {
			t.Errorf("menu overflows right: X=%d, Width=%d, screenW=%d", m.X, m.Width(), screenW)
		}
	})

	// Menu overflows bottom edge
	t.Run("overflow_bottom", func(t *testing.T) {
		m := &Menu{
			X: 0, Y: 45,
			Items: []Item{
				{Label: "A", Action: "a"},
				{Label: "B", Action: "b"},
				{Label: "C", Action: "c"},
				{Label: "D", Action: "d"},
				{Label: "E", Action: "e"},
				{Label: "F", Action: "f"},
				{Label: "G", Action: "g"},
				{Label: "H", Action: "h"},
			},
		}
		screenH := 50
		m.Clamp(100, screenH)
		if m.Y+m.Height() > screenH {
			t.Errorf("menu overflows bottom: Y=%d, Height=%d, screenH=%d", m.Y, m.Height(), screenH)
		}
	})

	// Menu clamped to minimum X=0
	t.Run("clamp_negative_x", func(t *testing.T) {
		m := &Menu{
			X: -5, Y: 5,
			Items: []Item{
				{Label: "A", Action: "a"},
			},
		}
		m.Clamp(100, 100)
		if m.X < 0 {
			t.Errorf("X should not be negative, got %d", m.X)
		}
	})

	// Menu clamped to minimum Y=1
	t.Run("clamp_low_y", func(t *testing.T) {
		m := &Menu{
			X: 5, Y: 0,
			Items: []Item{
				{Label: "A", Action: "a"},
			},
		}
		m.Clamp(100, 100)
		if m.Y < 1 {
			t.Errorf("Y should be at least 1, got %d", m.Y)
		}
	})

	// Tiny screen: both X and Y pushed negative after right/bottom clamp
	t.Run("tiny_screen", func(t *testing.T) {
		m := &Menu{
			X: 5, Y: 5,
			Items: []Item{
				{Label: "A", Action: "a"},
			},
		}
		// Screen smaller than menu: forces X negative after right clamp, then X=0
		m.Clamp(3, 3)
		if m.X < 0 {
			t.Errorf("X should be clamped to 0, got %d", m.X)
		}
		if m.Y < 1 {
			t.Errorf("Y should be clamped to 1, got %d", m.Y)
		}
	})
}

func TestSelectedActionOutOfBounds(t *testing.T) {
	m := DesktopMenu(0, 0, KeyBindings{})

	// Negative hover index
	m.HoverIndex = -1
	action := m.SelectedAction()
	if action != "" {
		t.Errorf("SelectedAction() with HoverIndex=-1 should be empty, got %q", action)
	}

	// Hover index beyond items
	m.HoverIndex = 999
	action = m.SelectedAction()
	if action != "" {
		t.Errorf("SelectedAction() with HoverIndex=999 should be empty, got %q", action)
	}

	// Hover index exactly at length
	m.HoverIndex = len(m.Items)
	action = m.SelectedAction()
	if action != "" {
		t.Errorf("SelectedAction() with HoverIndex=len(Items) should be empty, got %q", action)
	}
}

func TestTitleBarMenuResizableDisabled(t *testing.T) {
	m := TitleBarMenu(10, 5, false, false, KeyBindings{})
	// Maximize, Snap Left, Snap Right, Center should be disabled when not resizable
	for _, item := range m.Items {
		switch item.Action {
		case "maximize", "snap_left", "snap_right", "center":
			if !item.Disabled {
				t.Errorf("item %q should be disabled when resizable=false", item.Action)
			}
		case "close_window", "minimize":
			if item.Disabled {
				t.Errorf("item %q should NOT be disabled when resizable=false", item.Action)
			}
		}
	}
}

func TestMoveHoverEmptyMenu(t *testing.T) {
	m := &Menu{}
	// Should not panic
	m.MoveHover(1)
	m.MoveHover(-1)
	if m.HoverIndex != 0 {
		t.Errorf("HoverIndex should remain 0 on empty menu, got %d", m.HoverIndex)
	}
}

func TestMoveHoverAllDisabled(t *testing.T) {
	m := &Menu{
		Items: []Item{
			{Label: "─", Disabled: true},
			{Label: "─", Disabled: true},
			{Label: "─", Disabled: true},
		},
	}
	// Moving through all-disabled items should cycle through all of them without panicking
	m.MoveHover(1)
	// HoverIndex will end up on a disabled item since there are no enabled items
	// Just verify no panic and it stays within bounds
	if m.HoverIndex < 0 || m.HoverIndex >= len(m.Items) {
		t.Errorf("HoverIndex out of bounds: %d", m.HoverIndex)
	}
}

func TestMoveHoverWraparound(t *testing.T) {
	m := &Menu{
		Items: []Item{
			{Label: "First", Action: "first"},
			{Label: "─", Disabled: true},
			{Label: "Last", Action: "last"},
		},
	}

	// Start at 0 (First), move backward should wrap to Last (index 2)
	m.HoverIndex = 0
	m.MoveHover(-1)
	if m.HoverIndex != 2 {
		t.Errorf("MoveHover(-1) from 0 should wrap to 2, got %d", m.HoverIndex)
	}

	// Move forward from last should wrap to first (index 0)
	m.MoveHover(1)
	if m.HoverIndex != 0 {
		t.Errorf("MoveHover(1) from last should wrap to 0, got %d", m.HoverIndex)
	}
}
