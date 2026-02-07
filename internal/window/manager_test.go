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
		{"overlap area returns front", geometry.Point{15, 10}, "w2"},
		{"w1 only area", geometry.Point{5, 2}, "w1"},
		{"w2 only area", geometry.Point{45, 15}, "w2"},
		{"outside all", geometry.Point{100, 100}, ""},
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

	got := m.WindowAt(geometry.Point{5, 5})
	if got != nil {
		t.Error("expected nil for minimized window")
	}
}

func TestWindowAtSkipsInvisible(t *testing.T) {
	m := newTestManager()
	w := newTestWindow("w1", "Win1", 0, 0, 40, 20)
	m.AddWindow(w)
	w.Visible = false

	got := m.WindowAt(geometry.Point{5, 5})
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
