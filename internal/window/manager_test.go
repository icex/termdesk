package window

import (
	"testing"

	"github.com/icex/termdesk/pkg/geometry"
)

func newTestManager() *Manager {
	return NewManager(120, 40)
}

func newTestWindow(id, title string, x, y, w, h int) *Window {
	return NewWindow(id, title, geometry.Rect{X: x, Y: y, Width: w, Height: h}, nil)
}

func TestNewManager(t *testing.T) {
	m := NewManager(120, 40)
	if m.bounds.Width != 120 || m.bounds.Height != 40 {
		t.Errorf("bounds = %v, want 120x40", m.bounds)
	}
	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
}

func TestSetBounds(t *testing.T) {
	m := newTestManager()
	m.SetBounds(200, 60)
	if m.bounds.Width != 200 || m.bounds.Height != 60 {
		t.Errorf("bounds after SetBounds = %v", m.bounds)
	}
}

func TestSetReserved(t *testing.T) {
	m := NewManager(100, 50)
	m.SetReserved(1, 1)
	wa := m.WorkArea()
	want := geometry.Rect{X: 0, Y: 1, Width: 100, Height: 48}
	if wa != want {
		t.Errorf("WorkArea() = %v, want %v", wa, want)
	}
}

func TestAddWindow(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 10, 5, 40, 20)
	m.AddWindow(w)

	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}
	if m.focused != "w1" {
		t.Errorf("focused = %q, want %q", m.focused, "w1")
	}
	if !w.Focused {
		t.Error("expected window to be focused after add")
	}
}

func TestAddMultipleWindowsFocus(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	if m.Count() != 3 {
		t.Errorf("Count() = %d, want 3", m.Count())
	}
	// Last added should be focused
	if m.focused != "w3" {
		t.Errorf("focused = %q, want %q", m.focused, "w3")
	}
	if w1.Focused || w2.Focused {
		t.Error("only the last window should be focused")
	}
	if !w3.Focused {
		t.Error("w3 should be focused")
	}
}

func TestRemoveWindow(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)

	m.RemoveWindow("w2")
	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}
	// Focus should move to remaining window
	if m.focused != "w1" {
		t.Errorf("focused = %q, want %q", m.focused, "w1")
	}
}

func TestRemoveOnlyWindow(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	m.RemoveWindow("w1")

	if m.Count() != 0 {
		t.Errorf("Count() = %d, want 0", m.Count())
	}
	if m.focused != "" {
		t.Errorf("focused = %q, want empty", m.focused)
	}
}

func TestRemoveNonexistent(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	m.RemoveWindow("nonexistent")
	if m.Count() != 1 {
		t.Error("removing nonexistent should not change count")
	}
}

func TestFocusWindow(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Focus w1 (should move to front)
	m.FocusWindow("w1")
	if m.focused != "w1" {
		t.Errorf("focused = %q, want w1", m.focused)
	}
	// w1 should be last in slice (top of z-stack)
	windows := m.Windows()
	if windows[len(windows)-1].ID != "w1" {
		t.Error("focused window should be at end of z-stack")
	}
	if w2.Focused || w3.Focused {
		t.Error("only w1 should be focused")
	}
}

func TestFocusNonexistent(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	m.FocusWindow("nonexistent")
	// Should not crash, focused should remain w1
	if m.focused != "w1" {
		t.Errorf("focused = %q, want w1", m.focused)
	}
}

func TestFocusedWindow(t *testing.T) {
	m := newTestManager()
	if m.FocusedWindow() != nil {
		t.Error("expected nil FocusedWindow with no windows")
	}

	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	got := m.FocusedWindow()
	if got == nil || got.ID != "w1" {
		t.Error("expected FocusedWindow to return w1")
	}
}

func TestWindowAt(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Back", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Front", 10, 5, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)

	tests := []struct {
		name string
		p    geometry.Point
		want string // expected window ID, or "" for nil
	}{
		{"overlap area returns front", geometry.Point{X: 15, Y: 10}, "w2"},
		{"w1 only area", geometry.Point{X: 5, Y: 2}, "w1"},
		{"w2 only area", geometry.Point{X: 45, Y: 15}, "w2"},
		{"outside all", geometry.Point{X: 100, Y: 100}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.WindowAt(tt.p)
			if tt.want == "" {
				if got != nil {
					t.Errorf("WindowAt(%v) = %q, want nil", tt.p, got.ID)
				}
			} else {
				if got == nil || got.ID != tt.want {
					id := ""
					if got != nil {
						id = got.ID
					}
					t.Errorf("WindowAt(%v) = %q, want %q", tt.p, id, tt.want)
				}
			}
		})
	}
}

func TestWindowAtSkipsMinimized(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	w.Minimized = true

	got := m.WindowAt(geometry.Point{X: 5, Y: 5})
	if got != nil {
		t.Error("expected nil for minimized window")
	}
}

func TestWindowAtSkipsInvisible(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	w.Visible = false

	got := m.WindowAt(geometry.Point{X: 5, Y: 5})
	if got != nil {
		t.Error("expected nil for invisible window")
	}
}

func TestWindowByID(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	got := m.WindowByID("w1")
	if got == nil || got.ID != "w1" {
		t.Error("expected to find w1")
	}

	got = m.WindowByID("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestVisibleCount(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	if m.VisibleCount() != 3 {
		t.Errorf("VisibleCount() = %d, want 3", m.VisibleCount())
	}

	w2.Minimized = true
	if m.VisibleCount() != 2 {
		t.Errorf("VisibleCount() = %d, want 2", m.VisibleCount())
	}

	w1.Visible = false
	if m.VisibleCount() != 1 {
		t.Errorf("VisibleCount() = %d, want 1", m.VisibleCount())
	}
}

func TestCycleForward(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// w3 is focused (last added). Cycle forward should wrap around.
	m.CycleForward()
	focused := m.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}
	// After cycling forward from the last (w3), should go to first visible
	// The exact behavior depends on implementation, but it should change focus
	if focused.ID == "w3" {
		t.Error("expected focus to change from w3")
	}
}

func TestCycleBackward(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)

	// w2 is focused. Cycle backward should go to w1.
	m.CycleBackward()
	focused := m.FocusedWindow()
	if focused == nil || focused.ID != "w1" {
		id := ""
		if focused != nil {
			id = focused.ID
		}
		t.Errorf("expected w1, got %q", id)
	}
}

func TestCycleSingleWindow(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	m.CycleForward()
	if m.focused != "w1" {
		t.Error("single window cycle forward should keep focus")
	}

	m.CycleBackward()
	if m.focused != "w1" {
		t.Error("single window cycle backward should keep focus")
	}
}

func TestCycleNoWindows(t *testing.T) {
	m := newTestManager()
	// Should not panic
	m.CycleForward()
	m.CycleBackward()
}

func TestCycleSkipsMinimized(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	w2.Minimized = true
	m.FocusWindow("w1")
	m.CycleForward()

	focused := m.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}
	if focused.ID == "w2" {
		t.Error("cycle should skip minimized window w2")
	}
}

func TestZOrderAfterFocus(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Focus w1 should bring it to front
	m.FocusWindow("w1")
	windows := m.Windows()
	if windows[len(windows)-1].ID != "w1" {
		t.Error("w1 should be at top of z-stack after focus")
	}

	// z-order should be: w2, w3, w1
	if windows[0].ID != "w2" {
		t.Errorf("expected w2 at bottom, got %q", windows[0].ID)
	}
	if windows[1].ID != "w3" {
		t.Errorf("expected w3 in middle, got %q", windows[1].ID)
	}
}

func TestReservedBottom(t *testing.T) {
	m := NewManager(100, 50)
	if m.ReservedBottom() != 0 {
		t.Errorf("ReservedBottom() = %d, want 0", m.ReservedBottom())
	}
	m.SetReserved(1, 2)
	if m.ReservedBottom() != 2 {
		t.Errorf("ReservedBottom() = %d, want 2", m.ReservedBottom())
	}
}

func TestFocusNextVisible(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Focus next visible excluding w3 (which is currently focused)
	m.FocusNextVisible("w3")
	focused := m.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}
	// Should focus w2 (the topmost visible excluding w3)
	if focused.ID != "w2" {
		t.Errorf("FocusNextVisible excluding w3: focused = %q, want w2", focused.ID)
	}
}

func TestFocusNextVisibleSkipsMinimized(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	w2.Minimized = true
	m.FocusNextVisible("w3")
	focused := m.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}
	// w2 is minimized, so should focus w1
	if focused.ID != "w1" {
		t.Errorf("FocusNextVisible skipping minimized: focused = %q, want w1", focused.ID)
	}
}

func TestFocusNextVisibleNoneAvailable(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w1)

	// Exclude the only window
	m.FocusNextVisible("w1")
	focused := m.FocusedWindow()
	if focused != nil {
		t.Errorf("expected nil focused window, got %q", focused.ID)
	}
	if m.focused != "" {
		t.Errorf("expected empty focused ID, got %q", m.focused)
	}
}

func TestFocusNextVisibleAllMinimized(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)

	w1.Minimized = true
	m.FocusNextVisible("w2")
	focused := m.FocusedWindow()
	if focused != nil {
		t.Errorf("expected nil focused when all others minimized, got %q", focused.ID)
	}
}

func TestClampAllWindows(t *testing.T) {
	m := NewManager(100, 50)
	m.SetReserved(1, 1)
	// Work area: {0, 1, 100, 48}

	// Window that extends beyond the right edge
	w1 := newTestWindow("w1", "Win1", 80, 10, 40, 20)
	m.AddWindow(w1)

	// Window that extends below the bottom
	w2 := newTestWindow("w2", "Win2", 10, 35, 40, 20)
	m.AddWindow(w2)

	// Window that is left of work area
	w3 := newTestWindow("w3", "Win3", -5, -2, 40, 20)
	m.AddWindow(w3)

	m.ClampAllWindows()

	wa := m.WorkArea()

	// Check w1 is within bounds
	if w1.Rect.Right() > wa.Right() {
		t.Errorf("w1 right %d > work area right %d", w1.Rect.Right(), wa.Right())
	}
	if w1.Rect.X < wa.X {
		t.Errorf("w1 X %d < work area X %d", w1.Rect.X, wa.X)
	}

	// Check w2 is within bounds
	if w2.Rect.Bottom() > wa.Bottom() {
		t.Errorf("w2 bottom %d > work area bottom %d", w2.Rect.Bottom(), wa.Bottom())
	}

	// Check w3 is within bounds
	if w3.Rect.X < wa.X {
		t.Errorf("w3 X %d < work area X %d", w3.Rect.X, wa.X)
	}
	if w3.Rect.Y < wa.Y {
		t.Errorf("w3 Y %d < work area Y %d", w3.Rect.Y, wa.Y)
	}
}

func TestClampAllWindowsMaximized(t *testing.T) {
	m := NewManager(100, 50)
	m.SetReserved(1, 1)

	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	prev := w.Rect
	w.PreMaxRect = &prev
	w.Rect = geometry.Rect{X: 0, Y: 0, Width: 200, Height: 100} // oversized

	m.ClampAllWindows()

	wa := m.WorkArea()
	if w.Rect != wa {
		t.Errorf("maximized window after clamp = %v, want work area %v", w.Rect, wa)
	}
}

func TestClampAllWindowsSkipsMinimized(t *testing.T) {
	m := NewManager(100, 50)
	m.SetReserved(1, 1)

	w := newTestWindow("w1", "Win1", -10, -10, 40, 20)
	m.AddWindow(w)
	w.Minimized = true
	origRect := w.Rect

	m.ClampAllWindows()

	if w.Rect != origRect {
		t.Errorf("minimized window should be unchanged: got %v, want %v", w.Rect, origRect)
	}
}

func TestClampAllWindowsOversized(t *testing.T) {
	m := NewManager(60, 30)
	m.SetReserved(1, 1)
	// Work area: {0, 1, 60, 28}

	w := newTestWindow("w1", "Win1", 0, 1, 80, 40)
	m.AddWindow(w)

	m.ClampAllWindows()

	wa := m.WorkArea()
	if w.Rect.Width > wa.Width {
		t.Errorf("width %d exceeds work area %d", w.Rect.Width, wa.Width)
	}
	if w.Rect.Height > wa.Height {
		t.Errorf("height %d exceeds work area %d", w.Rect.Height, wa.Height)
	}
}

func TestCycleBackwardUnfocused(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	m.AddWindow(w1)
	m.AddWindow(w2)

	// Manually clear the focused state
	m.focused = "nonexistent"
	m.CycleBackward()
	// Should not panic; idx < 0 so returns early
	// focused remains unchanged because we returned early
	if m.focused != "nonexistent" {
		t.Errorf("CycleBackward with invalid focus should not change, got %q", m.focused)
	}
}

func TestFocusWindowNoRaise(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	w2.HasNotification = true
	w2.HasBell = true
	w2.HasActivity = true

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// w3 is focused and at the end. Focus w1 without raising.
	m.FocusWindowNoRaise("w1")

	// w1 should be focused
	if m.focused != "w1" {
		t.Errorf("focused = %q, want w1", m.focused)
	}
	if !w1.Focused {
		t.Error("w1 should be focused")
	}
	// Others should NOT be focused
	if w2.Focused || w3.Focused {
		t.Error("only w1 should be focused")
	}

	// Z-order should NOT change: w1 should still be at position 0 (bottom)
	windows := m.Windows()
	if windows[0].ID != "w1" {
		t.Errorf("expected w1 at position 0 (no raise), got %q", windows[0].ID)
	}
	if windows[len(windows)-1].ID != "w3" {
		t.Errorf("expected w3 at top (unchanged), got %q", windows[len(windows)-1].ID)
	}

	// Notification flags on focused window should be cleared
	if w1.HasNotification || w1.HasBell || w1.HasActivity {
		t.Error("focused window's notification flags should be cleared")
	}
}

func TestFocusWindowNoRaiseNonexistent(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	m.FocusWindowNoRaise("nonexistent")
	// focused should be empty (not found)
	if m.focused != "" {
		t.Errorf("focused = %q, want empty for nonexistent", m.focused)
	}
	// w1 should be unfocused
	if w.Focused {
		t.Error("w1 should be unfocused when targeting nonexistent ID")
	}
}

func TestFocusNextVisibleNoRaise(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)
	w2.HasNotification = true

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Focus next visible excluding w3, without raising
	m.FocusNextVisibleNoRaise("w3")

	// Should focus w2 (topmost visible excluding w3)
	if m.focused != "w2" {
		t.Errorf("focused = %q, want w2", m.focused)
	}
	if !w2.Focused {
		t.Error("w2 should be focused")
	}
	// w2's notification should be cleared
	if w2.HasNotification {
		t.Error("w2 notification should be cleared on focus")
	}
	// z-order should NOT change
	windows := m.Windows()
	if windows[len(windows)-1].ID != "w3" {
		t.Errorf("w3 should remain at top (no raise), got %q", windows[len(windows)-1].ID)
	}
}

func TestFocusNextVisibleNoRaiseSkipsMinimized(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	w2.Minimized = true
	m.FocusNextVisibleNoRaise("w3")

	// Should skip w2 (minimized) and focus w1
	if m.focused != "w1" {
		t.Errorf("focused = %q, want w1", m.focused)
	}
}

func TestFocusNextVisibleNoRaiseNoneAvailable(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	m.FocusNextVisibleNoRaise("w1")
	if m.focused != "" {
		t.Errorf("focused = %q, want empty", m.focused)
	}
}

func TestTilingSlotOf(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// All are visible, non-minimized, resizable — in z-order: w1(0), w2(1), w3(2)
	// But FocusWindow reorders. After AddWindow sequence: w1 at bottom, w2 in middle, w3 at top.
	// Actually after AddWindow w1, w2 (focus=w2, w1 at 0, w2 at 1), w3 (focus=w3, w1 at 0, w2 at 1, w3 at 2)
	// Wait: FocusWindow in AddWindow moves to end. So order is: w1, w2, w3 in slice.
	// But w2 was moved to end when w2 was added. Then w3 was added and moved to end.
	// Actually: AddWindow w1 -> [w1], focused=w1
	// AddWindow w2 -> [w1, w2], focused=w2 (w2 at end)
	// AddWindow w3 -> [w1, w2, w3], focused=w3 (w3 at end)
	// TilingSlotOf counts among visible, non-minimized, resizable
	slot0 := m.TilingSlotOf("w1")
	slot1 := m.TilingSlotOf("w2")
	slot2 := m.TilingSlotOf("w3")

	if slot0 != 0 {
		t.Errorf("TilingSlotOf(w1) = %d, want 0", slot0)
	}
	if slot1 != 1 {
		t.Errorf("TilingSlotOf(w2) = %d, want 1", slot1)
	}
	if slot2 != 2 {
		t.Errorf("TilingSlotOf(w3) = %d, want 2", slot2)
	}
}

func TestTilingSlotOfSkipsIneligible(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	w2.Minimized = true // not eligible for tiling

	slot1 := m.TilingSlotOf("w1")
	slot3 := m.TilingSlotOf("w3")
	slotMin := m.TilingSlotOf("w2")

	if slot1 != 0 {
		t.Errorf("TilingSlotOf(w1) = %d, want 0", slot1)
	}
	if slot3 != 1 {
		t.Errorf("TilingSlotOf(w3) = %d, want 1 (w2 is minimized)", slot3)
	}
	if slotMin != -1 {
		t.Errorf("TilingSlotOf(w2) = %d, want -1 (minimized)", slotMin)
	}
}

func TestTilingSlotOfNonexistent(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	slot := m.TilingSlotOf("nonexistent")
	if slot != -1 {
		t.Errorf("TilingSlotOf(nonexistent) = %d, want -1", slot)
	}
}

func TestTilingSlotOfNonResizable(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)

	w1.Resizable = false

	slot := m.TilingSlotOf("w1")
	if slot != -1 {
		t.Errorf("TilingSlotOf(non-resizable) = %d, want -1", slot)
	}
	slot2 := m.TilingSlotOf("w2")
	if slot2 != 0 {
		t.Errorf("TilingSlotOf(w2) = %d, want 0", slot2)
	}
}

func TestPlaceWindowAtTilingSlot(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Current tiling order: w1(0), w2(1), w3(2)
	// Move w3 to slot 0
	m.PlaceWindowAtTilingSlot("w3", 0)

	slot := m.TilingSlotOf("w3")
	if slot != 0 {
		t.Errorf("after PlaceWindowAtTilingSlot(w3, 0): slot = %d, want 0", slot)
	}
}

func TestPlaceWindowAtTilingSlotNegative(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	// Negative slot should be a no-op
	m.PlaceWindowAtTilingSlot("w1", -1)
	if m.TilingSlotOf("w1") != 0 {
		t.Error("negative slot should be a no-op")
	}
}

func TestPlaceWindowAtTilingSlotNonexistent(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)

	// Non-existent window should be a no-op
	m.PlaceWindowAtTilingSlot("nonexistent", 0)
	if m.Count() != 1 {
		t.Error("non-existent window should not change count")
	}
}

func TestPlaceWindowAtTilingSlotEnd(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// Move w1 to slot 5 (beyond end — should go to end)
	m.PlaceWindowAtTilingSlot("w1", 5)

	slot := m.TilingSlotOf("w1")
	if slot != 2 {
		t.Errorf("after placing w1 at slot 5 (beyond end): slot = %d, want 2", slot)
	}
}

func TestVisibleWindows(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	vis := m.VisibleWindows()
	if len(vis) != 3 {
		t.Fatalf("VisibleWindows() = %d, want 3", len(vis))
	}

	w2.Minimized = true
	vis = m.VisibleWindows()
	if len(vis) != 2 {
		t.Fatalf("VisibleWindows() after minimize = %d, want 2", len(vis))
	}
	for _, w := range vis {
		if w.ID == "w2" {
			t.Error("minimized w2 should not appear in VisibleWindows")
		}
	}

	w1.Visible = false
	vis = m.VisibleWindows()
	if len(vis) != 1 {
		t.Fatalf("VisibleWindows() after invisible = %d, want 1", len(vis))
	}
	if vis[0].ID != "w3" {
		t.Errorf("expected w3 in VisibleWindows, got %q", vis[0].ID)
	}
}

func TestVisibleWindowsByID(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("c", "Win C", 0, 0, 40, 20)
	w2 := newTestWindow("a", "Win A", 5, 5, 40, 20)
	w3 := newTestWindow("b", "Win B", 10, 10, 40, 20)

	m.AddWindow(w1) // ID "c"
	m.AddWindow(w2) // ID "a"
	m.AddWindow(w3) // ID "b"

	// Z-order is c, a, b. VisibleWindowsByID should sort by ID: a, b, c.
	byID := m.VisibleWindowsByID()
	if len(byID) != 3 {
		t.Fatalf("VisibleWindowsByID() = %d, want 3", len(byID))
	}
	if byID[0].ID != "a" || byID[1].ID != "b" || byID[2].ID != "c" {
		t.Errorf("VisibleWindowsByID order: %s, %s, %s, want a, b, c",
			byID[0].ID, byID[1].ID, byID[2].ID)
	}
}

func TestVisibleWindowsByIDSkipsMinimized(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("a", "Win A", 0, 0, 40, 20)
	w2 := newTestWindow("b", "Win B", 5, 5, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)

	w1.Minimized = true
	byID := m.VisibleWindowsByID()
	if len(byID) != 1 {
		t.Fatalf("VisibleWindowsByID() = %d, want 1", len(byID))
	}
	if byID[0].ID != "b" {
		t.Errorf("expected b, got %q", byID[0].ID)
	}
}

func TestFocusedIndex(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)
	w3 := newTestWindow("w3", "Win3", 10, 10, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)
	m.AddWindow(w3)

	// w3 is focused (last added), visible windows order: w1, w2, w3
	idx := m.FocusedIndex()
	if idx != 2 {
		t.Errorf("FocusedIndex() = %d, want 2", idx)
	}

	m.FocusWindow("w1")
	// After focusing w1, z-order: w2, w3, w1. Visible: w2, w3, w1
	idx = m.FocusedIndex()
	if idx != 2 {
		t.Errorf("FocusedIndex() after focusing w1 = %d, want 2", idx)
	}
}

func TestFocusedIndexNoFocus(t *testing.T) {
	m := newTestManager()

	idx := m.FocusedIndex()
	if idx != -1 {
		t.Errorf("FocusedIndex() with no windows = %d, want -1", idx)
	}

	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	m.focused = ""
	idx = m.FocusedIndex()
	if idx != -1 {
		t.Errorf("FocusedIndex() with no focused = %d, want -1", idx)
	}
}

func TestFocusedIndexMinimizedNotInVisible(t *testing.T) {
	m := newTestManager()
	w1 := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	w2 := newTestWindow("w2", "Win2", 5, 5, 40, 20)

	m.AddWindow(w1)
	m.AddWindow(w2)

	// Minimize the focused window
	w2.Minimized = true
	idx := m.FocusedIndex()
	// w2 is focused but minimized, so it's not in visible windows
	if idx != -1 {
		t.Errorf("FocusedIndex() for minimized focused = %d, want -1", idx)
	}
}
