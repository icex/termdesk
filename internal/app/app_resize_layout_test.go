package app

import (
	"testing"

	"github.com/icex/termdesk/internal/window"

	tea "charm.land/bubbletea/v2"
)

func resizeTo(m Model, w, h int) Model {
	u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return completeAnimations(u.(Model))
}

// Phone attaches, opens and maximizes a terminal; a desktop then attaches to
// the same session. The window must fill the desktop, not stay phone-sized.
func TestMaximizedOnPhoneStaysMaximizedOnDesktop(t *testing.T) {
	m := New()
	m.tour.Skip()
	m = resizeTo(m, 45, 30)

	m.openTerminalWindowWith("/bin/cat", nil, "", "")
	m = completeAnimations(m)
	fw := m.wm.FocusedWindow()
	if !fw.AutoMaximized || fw.Rect != m.wm.WorkArea() {
		t.Fatalf("new window on phone = %v auto=%v, want auto-maximized %v", fw.Rect, fw.AutoMaximized, m.wm.WorkArea())
	}

	u, _ := m.executeAction("maximize", tea.KeyPressMsg(tea.Key{}), "")
	m = completeAnimations(u.(Model))
	if fw.AutoMaximized || !fw.IsMaximized() {
		t.Fatalf("maximize on auto-maximized window: max=%v auto=%v, want explicit", fw.IsMaximized(), fw.AutoMaximized)
	}

	m = resizeTo(m, 200, 60)
	if fw.Rect != m.wm.WorkArea() {
		t.Errorf("after desktop attach = %v, want %v", fw.Rect, m.wm.WorkArea())
	}
}

func TestAutoMaximizedRestoresOnDesktop(t *testing.T) {
	m := New()
	m.tour.Skip()
	m = resizeTo(m, 45, 30)
	m.openTerminalWindowWith("/bin/cat", nil, "", "")
	m = completeAnimations(m)
	fw := m.wm.FocusedWindow()

	m = resizeTo(m, 200, 60)
	if fw.IsMaximized() || fw.Rect == m.wm.WorkArea() {
		t.Errorf("auto-maximized window should restore on a big screen, got %v max=%v", fw.Rect, fw.IsMaximized())
	}
}

// Touch taps report a motion event at the click position; that must not
// count as dragging the window out of its maximized state.
func TestTapOnMaximizedTitleBarKeepsMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/cat", nil, "", "")
	m = completeAnimations(m)
	fw := m.wm.FocusedWindow()
	u, _ := m.executeAction("maximize", tea.KeyPressMsg(tea.Key{}), "")
	m = completeAnimations(u.(Model))

	x, y := fw.Rect.X+2, fw.Rect.Y
	u, _ = m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = u.(Model)
	u, _ = m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = u.(Model)
	u, _ = m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = u.(Model)

	if !fw.IsMaximized() || fw.Rect != m.wm.WorkArea() {
		t.Errorf("after tap = %v max=%v, want maximized %v", fw.Rect, fw.IsMaximized(), m.wm.WorkArea())
	}
}

func TestSnappedWindowFollowsResize(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/cat", nil, "", "")
	m = completeAnimations(m)
	fw := m.wm.FocusedWindow()
	u, _ := m.executeAction("snap_right", tea.KeyPressMsg(tea.Key{}), "")
	m = completeAnimations(u.(Model))

	m = resizeTo(m, 200, 60)
	if want := window.SnapRect(window.SnapRightHalf, m.wm.WorkArea()); fw.Rect != want {
		t.Errorf("snapped right after resize = %v, want %v", fw.Rect, want)
	}

	// Phone-sized: full screen; back to desktop: snapped again.
	m = resizeTo(m, 45, 30)
	if fw.Rect != m.wm.WorkArea() {
		t.Errorf("snapped window on phone = %v, want %v", fw.Rect, m.wm.WorkArea())
	}
	m = resizeTo(m, 160, 50)
	if want := window.SnapRect(window.SnapRightHalf, m.wm.WorkArea()); fw.Rect != want {
		t.Errorf("snapped right after phone round trip = %v, want %v", fw.Rect, want)
	}
}
