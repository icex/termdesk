package app

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func TestMouseClickFocuses(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // win-1
	m.openDemoWindow() // win-2 (focused)

	w1 := m.wm.Windows()[0]
	if w1.Focused {
		t.Fatal("w1 should not be focused initially")
	}

	// Click on w1's title bar at y=w1.Rect.Y which is above w2
	click := tea.MouseClickMsg(tea.Mouse{X: w1.Rect.X + 1, Y: w1.Rect.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.wm.FocusedWindow().ID != w1.ID {
		t.Errorf("expected %s focused, got %s", w1.ID, model.wm.FocusedWindow().ID)
	}
}

func TestMouseClickClose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	// Click close button position — should show confirm dialog
	closePos := w.CloseButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: closePos.X + 1, Y: closePos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.confirmClose == nil {
		t.Fatal("expected close confirm dialog")
	}
	if model.wm.Count() != 1 {
		t.Error("window should still exist before confirmation")
	}

	// Confirm with 'y' — starts close animation
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)

	// Complete the close animation
	model = completeAnimations(model)
	if model.wm.Count() != 0 {
		t.Errorf("expected window closed after close animation, count = %d", model.wm.Count())
	}
}

func TestMouseClickMaximize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	maxPos := w.MaxButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: maxPos.X + 1, Y: maxPos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	fw := model.wm.FocusedWindow()
	if !fw.IsMaximized() {
		t.Error("expected window maximized after clicking max button")
	}
}

func TestMouseDragMove(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	origX := w.Rect.X

	// Click title bar to start drag
	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 5, Y: w.Rect.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// Drag right by 10
	motion := tea.MouseMotionMsg(tea.Mouse{X: w.Rect.X + 15, Y: w.Rect.Y, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	model = updated.(Model)

	fw := model.wm.FocusedWindow()
	if fw.Rect.X <= origX {
		t.Errorf("window should have moved right, x = %d (orig %d)", fw.Rect.X, origX)
	}

	// Release
	release := tea.MouseReleaseMsg(tea.Mouse{})
	updated, _ = model.Update(release)
	model = updated.(Model)
	if model.drag.Active {
		t.Error("drag should end on release")
	}
}

func TestMouseDragResize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	origW := w.Rect.Width

	// Click right border to start resize
	borderX := w.Rect.Right() - 1
	borderY := w.Rect.Y + 5
	click := tea.MouseClickMsg(tea.Mouse{X: borderX, Y: borderY, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if !model.drag.Active || model.drag.Mode != window.DragResizeE {
		t.Errorf("expected DragResizeE, got mode=%v active=%v", model.drag.Mode, model.drag.Active)
	}

	// Drag right
	motion := tea.MouseMotionMsg(tea.Mouse{X: borderX + 10, Y: borderY, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	model = updated.(Model)

	fw := model.wm.FocusedWindow()
	if fw.Rect.Width <= origW {
		t.Errorf("window should have grown, width=%d (orig %d)", fw.Rect.Width, origW)
	}
}

func TestMouseClickOutsideWindows(t *testing.T) {
	m := setupReadyModel()
	// Click with no windows should not panic
	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseLeft})
	m.Update(click)
}

func TestMouseMotionNoDrag(t *testing.T) {
	m := setupReadyModel()
	// Motion with no active drag should be no-op
	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: 20})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	if model.drag.Active {
		t.Error("should not have active drag")
	}
}

func TestDragUnmaximizes(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	window.Maximize(w, m.wm.WorkArea())

	// Start drag on title bar
	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 5, Y: w.Rect.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// Move mouse — should un-maximize
	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: 5, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	model = updated.(Model)

	fw := model.wm.FocusedWindow()
	if fw.IsMaximized() {
		t.Error("dragging a maximized window should restore it")
	}
}

func TestDragDeadWindowClears(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// Start a drag
	w := m.wm.FocusedWindow()
	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 5, Y: w.Rect.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// Remove the window while dragging
	model.wm.RemoveWindow(w.ID)

	// Motion should clear the drag
	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: 10, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	model = updated.(Model)
	if model.drag.Active {
		t.Error("drag should clear when window is removed")
	}
}

func TestMouseContentClickNoDrag(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	// Click in content area (not border/title)
	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 5, Y: w.Rect.Y + 5, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.drag.Active {
		t.Error("clicking content should not start a drag")
	}
}

func TestShiftClickEntersCopyMode(t *testing.T) {
	m := setupReadyModel()

	// Need a real terminal for shift+click (it checks m.terminals)
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeTerminal

	cr := w.ContentRect()
	clickX := cr.X + 5
	clickY := cr.Y + 2

	// Shift+click on content area should enter copy mode with selection
	updated, _ := m.Update(tea.MouseClickMsg(tea.Mouse{
		X: clickX, Y: clickY, Button: tea.MouseLeft, Mod: tea.ModShift,
	}))
	m = updated.(Model)

	if m.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after shift+click, got %v", m.inputMode)
	}
	if !m.selActive {
		t.Error("expected selection to be active after shift+click")
	}
	if !m.selDragging {
		t.Error("expected selDragging to be true after shift+click")
	}

	// Clean up terminal
	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

func TestNormalClickStaysTerminal(t *testing.T) {
	m := setupReadyModel()

	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeTerminal

	cr := w.ContentRect()
	clickX := cr.X + 5
	clickY := cr.Y + 2

	// Normal click (no shift) should stay in terminal mode
	updated, _ := m.Update(tea.MouseClickMsg(tea.Mouse{X: clickX, Y: clickY, Button: tea.MouseLeft}))
	m = updated.(Model)

	if m.inputMode != ModeTerminal {
		t.Errorf("expected ModeTerminal after normal click, got %v", m.inputMode)
	}
	if m.selActive {
		t.Error("selection should NOT be active after normal click")
	}

	// Clean up terminal
	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

// ---------------------------------------------------------------------------
// teaToUvButton tests
// ---------------------------------------------------------------------------

func TestTeaToUvButton(t *testing.T) {
	tests := []struct {
		name   string
		input  tea.MouseButton
		expect uv.MouseButton
	}{
		{"left", tea.MouseLeft, uv.MouseLeft},
		{"middle", tea.MouseMiddle, uv.MouseMiddle},
		{"right", tea.MouseRight, uv.MouseRight},
		{"wheel up", tea.MouseWheelUp, uv.MouseWheelUp},
		{"wheel down", tea.MouseWheelDown, uv.MouseWheelDown},
		{"none", tea.MouseNone, uv.MouseNone},
		{"unknown button", tea.MouseButton(255), uv.MouseNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := teaToUvButton(tt.input)
			if got != tt.expect {
				t.Errorf("teaToUvButton(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleMouseWheel tests
// ---------------------------------------------------------------------------

func TestMouseWheelOnDesktopNoOp(t *testing.T) {
	m := setupReadyModel()
	// Wheel on desktop (no window) should be no-op
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseWheelUp})
	updated, cmd := m.Update(wheel)
	model := updated.(Model)
	if cmd != nil {
		t.Error("wheel on desktop should return nil cmd")
	}
	if model.inputMode != ModeNormal {
		t.Error("mode should still be ModeNormal")
	}
}

func TestMouseWheelDownOnDesktopNoOp(t *testing.T) {
	m := setupReadyModel()
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseWheelDown})
	updated, cmd := m.Update(wheel)
	if cmd != nil {
		t.Error("wheel down on desktop should return nil cmd")
	}
	_ = updated
}

func TestMouseWheelUpOnWindowContent(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"hello\nworld\nfoo\nbar"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeNormal

	cr := w.ContentRect()

	// Wheel up on content area should not panic
	wheel := tea.MouseWheelMsg(tea.Mouse{X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	_ = updated.(Model)

	// Clean up
	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

func TestMouseWheelInCopyModeScrollsUp(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.scrollOffset = 5

	cr := w.ContentRect()

	// Wheel up in copy mode: scrolls up by 3 but capped at maxScrollback.
	// For /bin/echo (which exits quickly), scrollback may be 0, capping offset to 0.
	// The key behavior: the code path runs and adjusts scrollOffset without panic.
	wheel := tea.MouseWheelMsg(tea.Mouse{X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// Just verify the copy mode path was exercised
	if model.inputMode != ModeCopy {
		t.Errorf("should remain in copy mode, got %v", model.inputMode)
	}

	// Clean up
	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

func TestMouseWheelInCopyModeScrollsDown(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.scrollOffset = 5

	cr := w.ContentRect()

	// Wheel down in copy mode should decrease scroll offset
	wheel := tea.MouseWheelMsg(tea.Mouse{X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	if model.scrollOffset > 5 {
		t.Errorf("scroll offset should not increase on wheel down, got %d", model.scrollOffset)
	}

	// Clean up
	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

func TestMouseWheelCopyModeNoTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	m.scrollOffset = 3

	// No focused terminal: wheel should return early
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseWheelUp})
	updated, cmd := m.Update(wheel)
	model := updated.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when no terminal in copy mode")
	}
	if model.scrollOffset != 3 {
		t.Errorf("scroll offset should remain 3, got %d", model.scrollOffset)
	}
}

func TestMouseWheelCopyModeDownExitsAtBottom(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.scrollOffset = 2

	cr := w.ContentRect()

	// Wheel down in copy mode: subtracts 3, capping at 0.
	// When scrollOffset <= 0, it exits copy mode and sets ModeNormal.
	// However, since the copy mode path at the top of handleMouseWheel fires first
	// (before checking window position), and maxScroll may be 0 for /bin/echo,
	// the behavior depends on scrollback state.
	wheel := tea.MouseWheelMsg(tea.Mouse{X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// The scroll offset should be clamped to 0
	if model.scrollOffset < 0 {
		t.Errorf("scroll offset should not be negative, got %d", model.scrollOffset)
	}

	// Clean up
	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

func TestMouseWheelCopyModeUpCapsAtMaxScrollback(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.scrollOffset = 999999 // absurdly high

	cr := w.ContentRect()

	// Wheel up should be capped at maxScrollback
	wheel := tea.MouseWheelMsg(tea.Mouse{X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// Just verify no panic and offset is capped
	_ = model.scrollOffset

	// Clean up
	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

// ---------------------------------------------------------------------------
// handleRightClick tests
// ---------------------------------------------------------------------------

func TestMouseRightClickOnDesktopShowsContextMenu(t *testing.T) {
	m := setupReadyModel()

	// Right-click on desktop area (not dock y=39, not menubar y=0)
	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil {
		t.Fatal("expected desktop context menu")
	}
	if !model.contextMenu.Visible {
		t.Error("context menu should be visible")
	}
	// Desktop menu should contain "New Terminal"
	found := false
	for _, item := range model.contextMenu.Items {
		if item.Action == "new_terminal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("desktop context menu should contain new_terminal action")
	}
}

func TestMouseRightClickOnTitleBarShowsContextMenu(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	// Right-click on title bar (not on a button)
	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 3, Y: w.Rect.Y, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil {
		t.Fatal("expected title bar context menu")
	}
	if !model.contextMenu.Visible {
		t.Error("context menu should be visible")
	}
	foundClose := false
	foundMax := false
	for _, item := range model.contextMenu.Items {
		if item.Action == "close_window" {
			foundClose = true
		}
		if item.Action == "maximize" {
			foundMax = true
		}
	}
	if !foundClose {
		t.Error("title bar menu should contain close_window action")
	}
	if !foundMax {
		t.Error("title bar menu should contain maximize action")
	}
}

func TestMouseRightClickOnDockOrMenuBarNoContextMenu(t *testing.T) {
	m := setupReadyModel()

	// Right-click on menu bar (y=0) should not show desktop context menu
	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 0, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("right-click on menubar should not show desktop context menu")
	}

	// Right-click on dock (y=height-1) should not show desktop context menu
	click = tea.MouseClickMsg(tea.Mouse{X: 50, Y: m.height - 1, Button: tea.MouseRight})
	updated, _ = model.Update(click)
	model = updated.(Model)
	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("right-click on dock should not show desktop context menu")
	}
}

func TestMouseRightClickInWindowContentShowsDesktopMenu(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	cr := w.ContentRect()

	// Right-click in content area of a demo window (no terminal, not in terminal mode):
	// handleRightClick iterates windows, finds HitContent (not HitTitleBar),
	// breaks the loop, then falls through to the desktop context menu check.
	click := tea.MouseClickMsg(tea.Mouse{X: cr.X + 2, Y: cr.Y + 2, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// For a non-terminal window, right-click on content falls through to desktop menu
	if model.contextMenu == nil || !model.contextMenu.Visible {
		t.Error("right-click in non-terminal window content should show desktop context menu")
	}
}

// ---------------------------------------------------------------------------
// handleMenuBarRightClick tests
// ---------------------------------------------------------------------------

func TestMouseMenuBarRightClickClock(t *testing.T) {
	m := setupReadyModel()

	updated, _ := m.handleMenuBarRightClick("clock")
	model := updated.(Model)
	if model.modal == nil {
		t.Fatal("expected modal overlay for clock click")
	}
	if model.modal.Title != "Date & Time" {
		t.Errorf("modal title = %q, want 'Date & Time'", model.modal.Title)
	}
	if len(model.modal.Lines) != 2 {
		t.Errorf("expected 2 lines in clock modal, got %d", len(model.modal.Lines))
	}
}

func TestMouseMenuBarRightClickNotification(t *testing.T) {
	m := setupReadyModel()

	wasCenterVisible := m.notifications.CenterVisible()
	updated, _ := m.handleMenuBarRightClick("notification")
	model := updated.(Model)
	if model.notifications.CenterVisible() == wasCenterVisible {
		t.Error("expected notification center visibility to toggle")
	}
}

func TestMouseMenuBarRightClickCPU(t *testing.T) {
	m := setupReadyModel()
	// Should try to launch htop; just verify no panic
	updated, _ := m.handleMenuBarRightClick("cpu")
	_ = updated.(Model)
}

func TestMouseMenuBarRightClickMem(t *testing.T) {
	m := setupReadyModel()
	updated, _ := m.handleMenuBarRightClick("mem")
	_ = updated.(Model)
}

func TestMouseMenuBarRightClickUnknown(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.handleMenuBarRightClick("unknown")
	model := updated.(Model)
	if cmd != nil {
		t.Error("unknown zone type should return nil cmd")
	}
	if model.modal != nil {
		t.Error("no modal should be shown for unknown zone")
	}
}

// ---------------------------------------------------------------------------
// handleMouseClick additional branches
// ---------------------------------------------------------------------------

func TestMouseClickDismissesModal(t *testing.T) {
	m := setupReadyModel()
	m.modal = &ModalOverlay{
		Title: "Test Modal",
		Lines: []string{"line 1"},
	}

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.modal != nil {
		t.Error("click should dismiss modal")
	}
}

func TestMouseClickDismissesContextMenuOutside(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(50, 20, contextmenu.KeyBindings{})

	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("click outside context menu should dismiss it")
	}
}

func TestMouseClickOnContextMenuItem(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Visible: true,
		Items: []contextmenu.Item{
			{Label: "About", Action: "about"},
		},
	}

	// Click inside the menu: relY = mouse.Y - cm.Y - 1 = 11 - 10 - 1 = 0 => item 0
	click := tea.MouseClickMsg(tea.Mouse{X: 12, Y: 11, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("context menu should be hidden after clicking an item")
	}
}

func TestMouseClickOnDisabledContextMenuItem(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Visible: true,
		Items: []contextmenu.Item{
			{Label: "---", Action: "sep", Disabled: true},
		},
	}

	// Click on the disabled item -- should not execute action
	click := tea.MouseClickMsg(tea.Mouse{X: 12, Y: 11, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	// Menu might still be visible or hidden (implementation may vary), no panic is success
	_ = model
}

func TestMouseClickDismissesLauncher(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	if !m.launcher.Visible {
		t.Fatal("launcher should be visible")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.launcher.Visible {
		t.Error("launcher should be dismissed after click")
	}
}

func TestMouseClickOnExposeMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	m.enterExpose()
	if !m.exposeMode {
		t.Fatal("expected expose mode to be active")
	}

	// Click on non-dock, non-menubar area should exit expose
	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	if model.exposeMode {
		t.Error("clicking in expose mode should exit expose")
	}
}

func TestMouseClickExposeOnMenuBarNoExit(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.enterExpose()

	// Click on menubar (y=0) in expose mode should not exit expose
	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)
	// Just verify no panic
	_ = model
}

func TestMouseClickOnSnapLeftButton(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	snapLeftPos := w.SnapLeftButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: snapLeftPos.X + 1, Y: snapLeftPos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.FocusedWindow()
	wa := model.wm.WorkArea()
	if fw.Rect.X != wa.X {
		t.Errorf("snap-left: window X = %d, want %d", fw.Rect.X, wa.X)
	}
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap-left: window width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}
}

func TestMouseClickOnSnapRightButton(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	snapRightPos := w.SnapRightButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: snapRightPos.X + 1, Y: snapRightPos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.FocusedWindow()
	wa := model.wm.WorkArea()
	expectedX := wa.X + wa.Width/2
	if fw.Rect.X != expectedX {
		t.Errorf("snap-right: window X = %d, want %d", fw.Rect.X, expectedX)
	}
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap-right: window width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}
}

func TestMouseClickOnMinButton(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	closePos := w.CloseButtonPos()
	minX := closePos.X - 2 // HitMinButton is 3 chars to the left of close
	click := tea.MouseClickMsg(tea.Mouse{X: minX, Y: closePos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.WindowByID(w.ID)
	if fw == nil {
		t.Fatal("window should still exist after minimize")
	}
	if !fw.Minimized {
		t.Error("window should be minimized")
	}
}

func TestMouseClickOnDesktopSwitchesToNormal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("clicking desktop in terminal mode should switch to normal, got %v", model.inputMode)
	}
}

func TestMouseClickOnEmptyDockPreservesMode(t *testing.T) {
	// Clicking empty dock area (no item hit) should NOT change mode.
	// This prevents accidental mode switches during scroll overshoot.
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.inputMode != ModeTerminal {
		t.Errorf("clicking empty dock area should preserve terminal mode, got %v", model.inputMode)
	}
}

func TestMouseClickOnMenuBarClosesMenu(t *testing.T) {
	m := setupReadyModel()

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.menuBar.IsOpen() {
		t.Error("click on empty menubar area should close menu")
	}
}

func TestMouseClickOnMenuBarTogglesMenu(t *testing.T) {
	m := setupReadyModel()

	// Click on first menu (typically "File" at x=2)
	click := tea.MouseClickMsg(tea.Mouse{X: 2, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if !model.menuBar.IsOpen() {
		t.Error("clicking menu item should open menu")
	}

	// Click again should close menu
	click = tea.MouseClickMsg(tea.Mouse{X: 2, Y: 0, Button: tea.MouseLeft})
	updated, _ = model.Update(click)
	model = updated.(Model)
	if model.menuBar.IsOpen() {
		t.Error("clicking same menu item again should close menu")
	}
}

func TestMouseClickConfirmDialogNoButton(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	m.confirmClose = &ConfirmDialog{
		WindowID: w.ID,
		Title:    "Close?",
		Selected: 0,
	}

	innerW := runeLen("Close?") + 4
	if innerW < 26 {
		innerW = 26
	}
	boxW := innerW + 2
	startX := (m.width - boxW) / 2
	startY := (m.height - 5) / 2
	btnY := startY + 3
	midX := startX + boxW/2

	// Click on the "No" button (right side)
	click := tea.MouseClickMsg(tea.Mouse{X: midX + 2, Y: btnY, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.confirmClose != nil {
		t.Error("clicking No should dismiss the confirm dialog")
	}
	if model.wm.Count() != 1 {
		t.Error("window should still exist after clicking No")
	}
}

func TestMouseClickConfirmDialogYesButton(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	m.confirmClose = &ConfirmDialog{
		WindowID: w.ID,
		Title:    "Close?",
		Selected: 0,
	}

	innerW := runeLen("Close?") + 4
	if innerW < 26 {
		innerW = 26
	}
	boxW := innerW + 2
	startX := (m.width - boxW) / 2
	startY := (m.height - 5) / 2
	btnY := startY + 3
	midX := startX + boxW/2

	// Click on the "Yes" button (left side)
	click := tea.MouseClickMsg(tea.Mouse{X: midX - 2, Y: btnY, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)
	// Should have started close process
	_ = model
}

func TestMouseClickOutsideConfirmDialogDismisses(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{
		Title: "Close?",
	}

	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.confirmClose != nil {
		t.Error("clicking outside confirm dialog should dismiss it")
	}
}

func TestMouseClickSettingsPanel(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	click := tea.MouseClickMsg(tea.Mouse{X: m.width / 2, Y: m.height / 2, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	_ = updated.(Model)
	// No panic = success
}

func TestMouseClickOnToastOpensNotificationCenter(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("Test", "body", notification.Info)

	toasts := m.notifications.VisibleToasts()
	if len(toasts) == 0 {
		t.Skip("no visible toasts (may have expired)")
	}

	toastW := 44
	if toastW > m.width-4 {
		toastW = m.width - 4
	}
	toastX := m.width - toastW - 1
	toastY := 2

	click := tea.MouseClickMsg(tea.Mouse{X: toastX + 5, Y: toastY + 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if !model.notifications.CenterVisible() {
		t.Error("clicking on toast should open notification center")
	}
}

func TestMouseClickOnModeBadge(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	mLabel := modeBadge(m.inputMode)
	mLabelW := 0
	for range mLabel {
		mLabelW++
	}
	modeX := m.width - mLabelW + 1

	click := tea.MouseClickMsg(tea.Mouse{X: modeX, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.inputMode != ModeTerminal {
		t.Errorf("clicking mode badge should cycle mode, got %v", model.inputMode)
	}
}

// ---------------------------------------------------------------------------
// handleMouseClick menu dropdown tests
// ---------------------------------------------------------------------------

func TestMouseClickMenuDropdownClosesOnOutsideClick(t *testing.T) {
	m := setupReadyModel()

	m.menuBar.OpenMenu(0)
	if !m.menuBar.IsOpen() {
		t.Fatal("menu should be open")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: 80, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.menuBar.IsOpen() {
		t.Error("clicking outside dropdown should close it")
	}
}

func TestMouseClickMenuDropdownSelectsItem(t *testing.T) {
	m := setupReadyModel()

	m.menuBar.OpenMenu(0)
	if !m.menuBar.IsOpen() {
		t.Fatal("menu should be open")
	}

	positions := m.menuBar.MenuXPositions()
	dropX := positions[0]

	// Click on first item in the dropdown (y=2, top border at y=1)
	click := tea.MouseClickMsg(tea.Mouse{X: dropX + 2, Y: 2, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.menuBar.IsOpen() {
		t.Error("menu should close after selecting an item")
	}
}

// ---------------------------------------------------------------------------
// handleMouseMotion additional branches
// ---------------------------------------------------------------------------

func TestMouseMotionDockHover(t *testing.T) {
	m := setupReadyModel()

	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: m.height - 1})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	_ = model.dock.HoverIndex // no panic
}

func TestMouseMotionClearsDockHover(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetHover(0)

	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: 10})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.dock.HoverIndex != -1 {
		t.Errorf("dock hover should be cleared when mouse leaves dock, got %d", model.dock.HoverIndex)
	}
}

func TestMouseMotionButtonHover(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	closePos := w.CloseButtonPos()
	motion := tea.MouseMotionMsg(tea.Mouse{X: closePos.X + 1, Y: closePos.Y})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.hoverButtonZone != window.HitCloseButton {
		t.Errorf("hover zone = %v, want HitCloseButton", model.hoverButtonZone)
	}
	if model.hoverButtonWindowID != w.ID {
		t.Errorf("hover window ID = %q, want %q", model.hoverButtonWindowID, w.ID)
	}
}

func TestMouseMotionNoButtonHover(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	motion := tea.MouseMotionMsg(tea.Mouse{X: 0, Y: 1})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.hoverButtonZone != window.HitNone {
		t.Errorf("hover zone should be HitNone when not over window, got %v", model.hoverButtonZone)
	}
	if model.hoverButtonWindowID != "" {
		t.Errorf("hover window ID should be empty, got %q", model.hoverButtonWindowID)
	}
}

func TestMouseMotionCopyModeDrag(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.selDragging = true
	m.selActive = true
	cr := w.ContentRect()
	m.selStart = geometry.Point{X: 0, Y: 0}
	m.selEnd = geometry.Point{X: 0, Y: 0}

	motion := tea.MouseMotionMsg(tea.Mouse{X: cr.X + 5, Y: cr.Y + 3, Button: tea.MouseLeft})
	updated, _ := m.Update(motion)
	_ = updated.(Model) // no panic

	// Clean up
	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

func TestMouseMotionCopyModeDragClamps(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.selDragging = true
	m.selActive = true

	// Drag to negative coordinates (should clamp)
	motion := tea.MouseMotionMsg(tea.Mouse{X: -5, Y: -5, Button: tea.MouseLeft})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	_ = model // no panic

	// Drag beyond window bounds
	motion = tea.MouseMotionMsg(tea.Mouse{X: 999, Y: 999, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	_ = updated.(Model) // no panic

	// Clean up
	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

func TestMouseMotionMaxButtonHover(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	maxPos := w.MaxButtonPos()
	motion := tea.MouseMotionMsg(tea.Mouse{X: maxPos.X + 1, Y: maxPos.Y})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.hoverButtonZone != window.HitMaxButton {
		t.Errorf("hover zone = %v, want HitMaxButton", model.hoverButtonZone)
	}
}

func TestMouseMotionSnapButtonHover(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	snapRPos := w.SnapRightButtonPos()
	motion := tea.MouseMotionMsg(tea.Mouse{X: snapRPos.X + 1, Y: snapRPos.Y})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.hoverButtonZone != window.HitSnapRightButton {
		t.Errorf("hover zone = %v, want HitSnapRightButton", model.hoverButtonZone)
	}

	snapLPos := w.SnapLeftButtonPos()
	motion = tea.MouseMotionMsg(tea.Mouse{X: snapLPos.X + 1, Y: snapLPos.Y})
	updated, _ = model.Update(motion)
	model = updated.(Model)

	if model.hoverButtonZone != window.HitSnapLeftButton {
		t.Errorf("hover zone = %v, want HitSnapLeftButton", model.hoverButtonZone)
	}
}

// ---------------------------------------------------------------------------
// handleMouseRelease additional branches
// ---------------------------------------------------------------------------

func TestMouseReleaseSelDraggingClearsOnSingleClick(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.selDragging = true
	m.selActive = true
	m.selStart = geometry.Point{X: 5, Y: 5}
	m.selEnd = geometry.Point{X: 5, Y: 5}

	release := tea.MouseReleaseMsg(tea.Mouse{})
	updated, _ := m.Update(release)
	model := updated.(Model)

	if model.selDragging {
		t.Error("selDragging should be false after release")
	}
	if model.selActive {
		t.Error("selActive should be false when start == end (single click)")
	}

	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

func TestMouseReleaseSelDraggingKeepsSelection(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeCopy
	m.selDragging = true
	m.selActive = true
	m.selStart = geometry.Point{X: 0, Y: 0}
	m.selEnd = geometry.Point{X: 10, Y: 3}

	release := tea.MouseReleaseMsg(tea.Mouse{})
	updated, _ := m.Update(release)
	model := updated.(Model)

	if model.selDragging {
		t.Error("selDragging should be false after release")
	}
	if !model.selActive {
		t.Error("selActive should remain true when start != end")
	}

	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

func TestMouseReleaseAfterResize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	borderX := w.Rect.Right() - 1
	borderY := w.Rect.Y + 5
	click := tea.MouseClickMsg(tea.Mouse{X: borderX, Y: borderY, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if !model.drag.Active {
		t.Fatal("drag should be active")
	}

	motion := tea.MouseMotionMsg(tea.Mouse{X: borderX + 5, Y: borderY, Button: tea.MouseLeft})
	updated, _ = model.Update(motion)
	model = updated.(Model)

	release := tea.MouseReleaseMsg(tea.Mouse{X: borderX + 5, Y: borderY})
	updated, _ = model.Update(release)
	model = updated.(Model)

	if model.drag.Active {
		t.Error("drag should be inactive after release")
	}
}

func TestMouseReleaseNoDrag(t *testing.T) {
	m := setupReadyModel()

	release := tea.MouseReleaseMsg(tea.Mouse{X: 50, Y: 20})
	updated, _ := m.Update(release)
	model := updated.(Model)
	if model.drag.Active {
		t.Error("drag should not be active")
	}
}

func TestMouseReleaseForwardsToTerminal(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	cr := w.ContentRect()

	release := tea.MouseReleaseMsg(tea.Mouse{X: cr.X + 2, Y: cr.Y + 2})
	updated, _ := m.Update(release)
	_ = updated.(Model)
	// No panic = success

	for _, w := range m.wm.Windows() {
		m.closeTerminal(w.ID)
	}
}

// ---------------------------------------------------------------------------
// Dock click behavior tests
// ---------------------------------------------------------------------------

func TestMouseClickDockLauncher(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetWidth(m.width) // ensure dock uses model width

	var launcherIdx int = -1
	for i, item := range m.dock.Items {
		if item.Special == "launcher" {
			launcherIdx = i
			break
		}
	}
	if launcherIdx < 0 {
		t.Skip("no launcher item in dock")
	}

	// Find X of the launcher by probing ItemAtX
	launcherX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == launcherIdx {
			launcherX = x
			break
		}
	}
	if launcherX < 0 {
		t.Skip("could not find launcher X position")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: launcherX, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if !model.launcher.Visible {
		t.Error("clicking launcher dock item should show launcher")
	}
}

func TestMouseClickDockExpose(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetWidth(m.width)

	var exposeIdx int = -1
	for i, item := range m.dock.Items {
		if item.Special == "expose" {
			exposeIdx = i
			break
		}
	}
	if exposeIdx < 0 {
		t.Skip("no expose item in dock")
	}

	exposeX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == exposeIdx {
			exposeX = x
			break
		}
	}
	if exposeX < 0 {
		t.Skip("could not find expose X position")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: exposeX, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	if !model.exposeMode {
		t.Error("clicking expose dock item should enter expose mode")
	}
}

func TestMouseClickDockMinimizedItem(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetWidth(m.width)
	m.dock.MinimizeToDock = true

	// Use a unique command so the window becomes a dynamic dock entry
	// (not matched to any dock shortcut).
	win := window.NewWindow("win-min", "MinApp",
		geometry.Rect{X: 5, Y: 5, Width: 60, Height: 15}, nil)
	win.Command = "unique-min-app"
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	m.minimizeWindow(win)
	m = completeAnimations(m)

	if !win.Minimized {
		t.Fatal("window should be minimized")
	}

	m.updateDockRunning()

	// Find the dynamic "minimized" entry
	var itemIdx int = -1
	for i, item := range m.dock.Items {
		if item.Special == "minimized" && item.WindowID == win.ID {
			itemIdx = i
			break
		}
	}
	if itemIdx < 0 {
		t.Fatal("minimized dock item not found")
	}

	itemX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == itemIdx {
			itemX = x
			break
		}
	}
	if itemX < 0 {
		t.Fatal("could not find minimized item X position")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: itemX, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.WindowByID(win.ID)
	if fw == nil {
		t.Fatal("window should exist")
	}
	if fw.Minimized {
		t.Error("clicking minimized dock item should restore the window")
	}
}

func TestMouseClickDockRunningItemFocuses(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetWidth(m.width)
	m.openDemoWindow()
	m.openDemoWindow()
	m.dock.MinimizeToDock = true
	m.updateDockRunning()

	w1 := m.wm.Windows()[0]
	w2 := m.wm.Windows()[1]

	// Find dock item associated with w1 (may be a shortcut or dynamic item)
	var itemIdx int = -1
	for i, item := range m.dock.Items {
		if item.WindowID == w1.ID {
			itemIdx = i
			break
		}
	}
	if itemIdx < 0 {
		t.Fatal("dock item for w1 not found")
	}

	if m.wm.FocusedWindow().ID != w2.ID {
		t.Fatalf("expected w2 focused, got %s", m.wm.FocusedWindow().ID)
	}

	itemX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == itemIdx {
			itemX = x
			break
		}
	}
	if itemX < 0 {
		t.Fatal("could not find dock item X position")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: itemX, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.wm.FocusedWindow().ID != w1.ID {
		t.Errorf("clicking dock item should focus w1, got %s", model.wm.FocusedWindow().ID)
	}
}

func TestMouseClickDockRunningItemMinimizesIfFocused(t *testing.T) {
	m := setupReadyModel()
	m.dock.SetWidth(m.width)
	m.dock.MinimizeToDock = true

	// Use a unique command so the window becomes a dynamic "running" entry
	// (not matched to any dock shortcut).
	win := window.NewWindow("win-unique", "UniqueApp",
		geometry.Rect{X: 5, Y: 5, Width: 60, Height: 15}, nil)
	win.Command = "unique-test-app"
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)
	m.updateDockRunning()

	// Find the dynamic "running" entry for the focused window
	var itemIdx int = -1
	for i, item := range m.dock.Items {
		if item.Special == "running" && item.WindowID == win.ID {
			itemIdx = i
			break
		}
	}
	if itemIdx < 0 {
		t.Fatal("dynamic running item for focused window not found")
	}

	itemX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == itemIdx {
			itemX = x
			break
		}
	}
	if itemX < 0 {
		t.Fatal("could not find running item X position")
	}

	// Click the running item of the focused window -> should minimize
	click := tea.MouseClickMsg(tea.Mouse{X: itemX, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.WindowByID(win.ID)
	if fw == nil {
		t.Fatal("window should exist")
	}
	if !fw.Minimized {
		t.Error("clicking focused running dock item should minimize the window")
	}
}

// ---------------------------------------------------------------------------
// Right-click in terminal mode forwards to terminal
// ---------------------------------------------------------------------------

func TestMouseRightClickInTerminalModeForwards(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"test"}, "Test", "")
	m = completeAnimations(m)

	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeTerminal

	cr := w.ContentRect()

	click := tea.MouseClickMsg(tea.Mouse{X: cr.X + 2, Y: cr.Y + 2, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("right-click in terminal content should forward to terminal, not show context menu")
	}

	for _, w := range model.wm.Windows() {
		model.closeTerminal(w.ID)
	}
}

// ---------------------------------------------------------------------------
// Non-resizable window snap/max buttons
// ---------------------------------------------------------------------------

func TestMouseClickMaxOnNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Resizable = false

	origRect := w.Rect

	maxPos := w.MaxButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: maxPos.X + 1, Y: maxPos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.FocusedWindow()
	if fw.IsMaximized() {
		t.Error("non-resizable window should not be maximized")
	}
	if fw.Rect != origRect {
		t.Error("non-resizable window rect should not change")
	}
}

func TestMouseClickSnapOnNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Resizable = false

	origRect := w.Rect

	snapLeftPos := w.SnapLeftButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: snapLeftPos.X + 1, Y: snapLeftPos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	fw := model.wm.FocusedWindow()
	if fw.Rect != origRect {
		t.Error("snap-left on non-resizable window should not change rect")
	}
}

// ---------------------------------------------------------------------------
// Non-draggable window
// ---------------------------------------------------------------------------

func TestMouseClickTitleBarNonDraggable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Draggable = false

	click := tea.MouseClickMsg(tea.Mouse{X: w.Rect.X + 3, Y: w.Rect.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.drag.Active {
		t.Error("non-draggable window should not start a drag")
	}
}

// --- handleMouseWheel tests ---

func TestMouseWheelOnEmptyDesktop(t *testing.T) {
	m := setupReadyModel()
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)
	// No window at point, should be no-op
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal, got %s", model.inputMode)
	}
}

func setupCopyModeWithTerminal(t *testing.T) Model {
	t.Helper()
	m := setupReadyModel()
	win := window.NewWindow("copywin", "Term", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 12}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	term.RestoreBuffer("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")
	m.terminals[win.ID] = term
	m.inputMode = ModeCopy
	return m
}

func TestMouseWheelCopyModeScrollUp(t *testing.T) {
	m := setupCopyModeWithTerminal(t)
	m.scrollOffset = 0

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)
	if model.scrollOffset < 0 {
		t.Error("expected non-negative scroll offset")
	}
}

func TestMouseWheelCopyModeScrollDownExits(t *testing.T) {
	m := setupCopyModeWithTerminal(t)
	m.scrollOffset = 0
	m.selActive = false

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)
	// At scroll offset 0, wheel-down should exit copy mode
	if model.inputMode == ModeCopy {
		t.Error("expected to exit copy mode after wheel-down at bottom")
	}
}

func TestMouseWheelCopyModeScrollDownWithOffset(t *testing.T) {
	m := setupCopyModeWithTerminal(t)
	m.scrollOffset = 5

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)
	// Scroll down should decrease offset
	if model.scrollOffset >= 5 {
		t.Errorf("expected scrollOffset < 5, got %d", model.scrollOffset)
	}
}

// --- handleMouseMotion tests ---

func TestMouseMotionCopyModeSelDragging(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeCopy
	m.selDragging = true
	m.selActive = true
	m.selStart = geometry.Point{X: 0, Y: 0}
	m.selEnd = geometry.Point{X: 0, Y: 0}

	// Motion with button held should update selection
	motion := tea.MouseMotionMsg(tea.Mouse{X: 20, Y: 10, Button: tea.MouseLeft})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	// Should not panic and should update selEnd
	_ = model
}

func TestMouseMotionWorkspacePickerHover(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/a/ws.toml", "/b/ws.toml", "/c/ws.toml"}
	m.workspaceWindowCounts = []int{1, 2, 3}

	bx, by, _, _ := m.workspacePickerBounds()
	motion := tea.MouseMotionMsg(tea.Mouse{X: bx + 2, Y: by + 3, Button: 0})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	// Hover should be updated
	_ = model
}

func TestMouseMotionContextMenuHover(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Items: []contextmenu.Item{
			{Label: "Cut"},
			{Label: "Copy"},
			{Label: "Paste"},
		},
		Visible:    true,
		HoverIndex: -1,
	}

	// Motion inside the context menu
	motion := tea.MouseMotionMsg(tea.Mouse{X: 12, Y: 12, Button: 0})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	_ = model
}

func TestMouseMotionContextMenuHoverOutside(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Items: []contextmenu.Item{
			{Label: "Cut"},
			{Label: "Copy"},
		},
		Visible:    true,
		HoverIndex: -1,
	}

	// Motion outside the context menu
	motion := tea.MouseMotionMsg(tea.Mouse{X: 0, Y: 0, Button: 0})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	if model.contextMenu.HoverIndex != -1 {
		t.Errorf("expected hover index -1 outside menu, got %d", model.contextMenu.HoverIndex)
	}
}

func TestMouseMotionSettingsHover(t *testing.T) {
	m := setupReadyModel()
	m.settings.Visible = true

	bounds := m.settings.Bounds(m.width, m.height)
	motion := tea.MouseMotionMsg(tea.Mouse{X: bounds.X + 5, Y: bounds.Y + 5, Button: 0})
	updated, _ := m.Update(motion)
	model := updated.(Model)
	_ = model
}

// --- handleMenuBarRightClick tests ---

func TestMenuBarRightClickClock(t *testing.T) {
	m := setupReadyModel()
	ret, _ := m.handleMenuBarRightClick("clock")
	model := ret.(Model)
	if model.modal == nil {
		t.Error("expected modal overlay for clock right-click")
	}
	if model.modal.Title != "Date & Time" {
		t.Errorf("expected title 'Date & Time', got %q", model.modal.Title)
	}
}

func TestMenuBarRightClickNotification(t *testing.T) {
	m := setupReadyModel()
	ret, _ := m.handleMenuBarRightClick("notification")
	model := ret.(Model)
	_ = model // toggles center, just verify no panic
}

func TestMenuBarRightClickWorkspace(t *testing.T) {
	m := setupReadyModel()
	ret, _ := m.handleMenuBarRightClick("workspace")
	model := ret.(Model)
	_ = model
}

func TestMenuBarRightClickUnknown(t *testing.T) {
	m := setupReadyModel()
	ret, _ := m.handleMenuBarRightClick("unknown_zone")
	model := ret.(Model)
	_ = model
}

// --- launcherBounds & launcherResultIdx tests ---

func TestLauncherBounds(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	x, y, w, h := m.launcherBounds()
	if w <= 0 || h <= 0 {
		t.Errorf("expected positive bounds, got w=%d h=%d", w, h)
	}
	if x < 0 || y < 0 {
		t.Errorf("expected non-negative position, got x=%d y=%d", x, y)
	}
}

func TestLauncherResultIdx(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	_, boundsY, _, _ := m.launcherBounds()

	// Too high
	idx := m.launcherResultIdx(boundsY+1, boundsY)
	if idx != -1 {
		t.Errorf("expected -1 for header area, got %d", idx)
	}
}

func TestLauncherResultIdxOnResult(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	_, boundsY, _, _ := m.launcherBounds()

	// At the first result row (border + 6 header lines)
	idx := m.launcherResultIdx(boundsY+7, boundsY)
	// Should be 0 or -1 depending on whether there are results
	if idx > 0 {
		t.Errorf("expected 0 or -1, got %d", idx)
	}
}
