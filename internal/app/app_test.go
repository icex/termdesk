package app

import (
	"testing"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

// completeAnimations sends a tick far enough in the future to finish all animations.
func completeAnimations(m Model) Model {
	updated, _ := m.Update(AnimationTickMsg{Time: time.Now().Add(time.Second)})
	return updated.(Model)
}

func setupReadyModel() Model {
	m := New()
	m.width = 120
	m.height = 40
	m.wm.SetBounds(120, 40)
	m.wm.SetReserved(1, 1) // menu bar at top, dock at bottom
	m.menuBar.SetWidth(120)
	m.ready = true
	return m
}

func TestNew(t *testing.T) {
	m := New()
	if m.ready {
		t.Error("expected model to not be ready initially")
	}
	if m.wm == nil {
		t.Error("expected window manager to be initialized")
	}
	if m.theme.Name == "" {
		t.Error("expected theme to be set")
	}
}

func TestInit(t *testing.T) {
	m := New()
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init (system stats tick)")
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)
	if model.width != 120 || model.height != 40 || !model.ready {
		t.Errorf("expected ready 120x40, got %dx%d ready=%v", model.width, model.height, model.ready)
	}
}

func TestQuitCtrlC(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if cmd != nil {
		t.Error("ctrl+c should show confirm dialog, not quit immediately")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Fatal("expected quit confirm dialog")
	}
	// Confirm with 'y'
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming")
	}
}

func TestQuitCtrlQ(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if cmd != nil {
		t.Error("ctrl+q should show confirm dialog, not quit immediately")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Fatal("expected quit confirm dialog")
	}
	// Confirm with enter
	_, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming")
	}
}

func TestQuitQNoWindows(t *testing.T) {
	m := setupReadyModel()
	// In Normal mode, 'q' shows confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	m2 := updated.(Model)
	if m2.confirmClose == nil {
		t.Error("expected confirm dialog on q when no windows focused")
	}
	// Pressing 'y' should quit
	updated, cmd := m2.Update(tea.KeyPressMsg(tea.Key{Code: 'y'}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming with y")
	}
	_ = updated
}

func TestNoQuitQWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	// Window is focused, 'q' in terminal mode does nothing (goes to terminal)
	// but our demo windows are not terminals, so they fall through to normal mode
	// In normal mode with a focused window, 'q' shows confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	m2 := updated.(Model)
	if m2.confirmClose == nil {
		t.Error("expected confirm dialog on q with windows")
	}
}

func TestCtrlNOpensWindow(t *testing.T) {
	m := setupReadyModel()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.wm.Count() != 1 {
		t.Errorf("expected 1 window, got %d", model.wm.Count())
	}
}

func TestCtrlBracketCycles(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	focused1 := m.wm.FocusedWindow().ID

	// Ctrl+] cycles forward (always works, even with terminal focus)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ']', Mod: tea.ModCtrl}))
	model := updated.(Model)
	focused2 := model.wm.FocusedWindow().ID

	if focused1 == focused2 {
		t.Error("ctrl+] should change focused window")
	}
}

func TestCtrlWCloses(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	if m.wm.Count() != 1 {
		t.Fatal("expected 1 window")
	}

	// Ctrl+W should show confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl}))
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
		t.Errorf("expected 0 windows after close animation, got %d", model.wm.Count())
	}
}

func TestConfirmDialogCancel(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// Ctrl+W opens confirm
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.confirmClose == nil {
		t.Fatal("expected confirm dialog")
	}

	// Press 'n' to cancel
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model = updated.(Model)
	if model.confirmClose != nil {
		t.Error("confirm dialog should be dismissed")
	}
	if model.wm.Count() != 1 {
		t.Error("window should still exist after cancel")
	}
}

func TestCtrlWNoWindowsNoPanic(t *testing.T) {
	m := setupReadyModel()
	// Should not panic
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl}))
}

func TestSnapLeftRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	wa := m.wm.WorkArea()

	// In Normal mode, 'h' or 'left' snaps left
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'h'}))
	model := updated.(Model)
	model = completeAnimations(model)
	fw := model.wm.FocusedWindow()
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap left: width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}

	// 'l' snaps right
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'l'}))
	model = updated.(Model)
	model = completeAnimations(model)
	fw = model.wm.FocusedWindow()
	if fw.Rect.X != wa.Width/2 {
		t.Errorf("snap right: x = %d, want %d", fw.Rect.X, wa.Width/2)
	}
}

func TestMaximizeRestore(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	origRect := m.wm.FocusedWindow().Rect

	// In Normal mode, 'k' or 'up' maximizes
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'k'}))
	model := updated.(Model)
	model = completeAnimations(model)
	if !model.wm.FocusedWindow().IsMaximized() {
		t.Error("expected maximized after k (maximize)")
	}

	// 'j' or 'down' restores
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
	model = updated.(Model)
	model = completeAnimations(model)
	if model.wm.FocusedWindow().Rect != origRect {
		t.Error("expected original rect after j (restore)")
	}
}

func TestTileAll(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// In Normal mode, 't' tiles all windows
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 't'}))
	model := updated.(Model)
	model = completeAnimations(model)
	windows := model.wm.Windows()
	if windows[0].Rect.Width != 60 {
		t.Errorf("tile: w1 width = %d, want 60", windows[0].Rect.Width)
	}
}

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

func TestViewSetsAltScreen(t *testing.T) {
	m := setupReadyModel()
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen")
	}
	if v.MouseMode != tea.MouseModeCellMotion {
		t.Error("expected MouseModeCellMotion")
	}
}

func TestViewBeforeReady(t *testing.T) {
	m := New()
	v := m.View()
	if v.Content == nil {
		t.Error("expected loading content")
	}
}

func TestViewRendersWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	v := m.View()
	if v.Content == nil {
		t.Error("expected rendered content with windows")
	}
}

func TestOpenDemoWindowMinSize(t *testing.T) {
	m := New()
	m.wm.SetBounds(20, 10) // very small terminal
	m.ready = true
	m.openDemoWindow()

	w := m.wm.FocusedWindow()
	if w.Rect.Width < window.MinWindowWidth {
		t.Errorf("window width %d below min %d", w.Rect.Width, window.MinWindowWidth)
	}
	if w.Rect.Height < window.MinWindowHeight {
		t.Errorf("window height %d below min %d", w.Rect.Height, window.MinWindowHeight)
	}
}

func TestSnapNoWindowNoPanic(t *testing.T) {
	m := setupReadyModel()
	// These should not panic with no focused window (Normal mode keys)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'h'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'l'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'k'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
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

func TestUnknownMsgPassthrough(t *testing.T) {
	m := setupReadyModel()
	type customMsg struct{}
	updated, cmd := m.Update(customMsg{})
	model := updated.(Model)
	if cmd != nil || !model.ready {
		t.Error("unknown message should pass through")
	}
}

// Test that window positions use the same geometry helpers
func TestWindowButtonPosMatchesHitTest(t *testing.T) {
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	w.Resizable = true

	// Close button hit test should work at the position reported by CloseButtonPos
	closePos := w.CloseButtonPos()
	// CloseButtonPos returns the start of [X], hit inside the button
	p := geometry.Point{X: closePos.X + 1, Y: closePos.Y}
	zone := window.HitTest(w, p, 3, 3)
	if zone != window.HitCloseButton {
		t.Errorf("expected HitCloseButton at close pos, got %v", zone)
	}
}

func TestF10OpensMenu(t *testing.T) {
	m := setupReadyModel()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF10}))
	model := updated.(Model)
	if !model.menuBar.IsOpen() {
		t.Error("F10 should open menu")
	}
}

func TestMenuNavigation(t *testing.T) {
	m := setupReadyModel()

	// Open menu
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF10}))
	model := updated.(Model)

	// Move down
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	if model.menuBar.HoverIndex != 1 {
		t.Errorf("hover = %d, want 1", model.menuBar.HoverIndex)
	}

	// Move right to next menu
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.menuBar.OpenIndex != 1 {
		t.Errorf("menu = %d, want 1", model.menuBar.OpenIndex)
	}

	// Escape closes
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.menuBar.IsOpen() {
		t.Error("escape should close menu")
	}
}

func TestMenuActionQuit(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0) // File menu
	m.menuBar.HoverIndex = 1 // Quit

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	_ = model // quit cmd is returned
}

func TestMenuBarClickOpens(t *testing.T) {
	m := setupReadyModel()

	// Click on menu bar at y=0
	click := tea.MouseClickMsg(tea.Mouse{X: 2, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if !model.menuBar.IsOpen() {
		t.Error("clicking menu bar should open menu")
	}
}

func TestMenuBarClickToggle(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	// Click same menu again to close
	click := tea.MouseClickMsg(tea.Mouse{X: 2, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.menuBar.IsOpen() {
		t.Error("clicking open menu should close it")
	}
}

func TestViewRendersMenuBar(t *testing.T) {
	m := setupReadyModel()
	v := m.View()
	if v.Content == nil {
		t.Error("expected rendered content with menu bar")
	}
}

// --- Launcher tests ---

func TestCtrlSpaceOpensLauncher(t *testing.T) {
	m := setupReadyModel()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.launcher.Visible {
		t.Error("ctrl+space should open launcher")
	}
}

func TestCtrlSpaceTogglesLauncher(t *testing.T) {
	m := setupReadyModel()

	// Open
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.launcher.Visible {
		t.Fatal("launcher should be visible")
	}

	// Close
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.launcher.Visible {
		t.Error("second ctrl+space should close launcher")
	}
}

func TestLauncherEscapeCloses(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Press Escape
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.launcher.Visible {
		t.Error("escape should close launcher")
	}
}

func TestLauncherTyping(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Type 't' 'e'
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}))
	model = updated.(Model)

	if model.launcher.Query != "te" {
		t.Errorf("query = %q, want 'te'", model.launcher.Query)
	}
}

func TestLauncherNavigation(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Move down
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	if model.launcher.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", model.launcher.SelectedIdx)
	}

	// Move up
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	model = updated.(Model)
	if model.launcher.SelectedIdx != 0 {
		t.Errorf("selection = %d, want 0", model.launcher.SelectedIdx)
	}
}

func TestLauncherBackspace(t *testing.T) {
	m := setupReadyModel()

	// Open launcher and type
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Text: "b"}))
	model = updated.(Model)

	// Backspace
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated.(Model)
	if model.launcher.Query != "a" {
		t.Errorf("query = %q, want 'a'", model.launcher.Query)
	}
}

func TestLauncherEnterLaunches(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Press Enter — should launch the selected app and close launcher
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)

	if model.launcher.Visible {
		t.Error("launcher should close after enter")
	}
	if model.wm.Count() != 1 {
		t.Errorf("expected 1 window after launch, got %d", model.wm.Count())
	}
}

func TestLauncherClickOutsideDismisses(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Click anywhere
	click := tea.MouseClickMsg(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft})
	updated, _ = model.Update(click)
	model = updated.(Model)

	if model.launcher.Visible {
		t.Error("clicking outside launcher should dismiss it")
	}
}

func TestLauncherBlocksWindowKeys(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Alt+Tab should not cycle windows while launcher is open
	focusBefore := model.wm.FocusedWindow().ID
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModAlt}))
	model = updated.(Model)
	// The key went to the launcher (which ignores it), not the window manager
	if model.wm.FocusedWindow().ID != focusBefore {
		t.Error("launcher should capture keys, not pass to window manager")
	}
}

func TestLauncherCtrlQStillQuits(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	// Ctrl+Q should show confirm dialog (launcher hides + dialog shows)
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if cmd != nil {
		t.Error("ctrl+q should show confirm dialog, not quit immediately")
	}
	if model.launcher.Visible {
		t.Error("launcher should be hidden")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Fatal("expected quit confirm dialog")
	}

	// Confirm with 'y'
	_, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming")
	}
}

func TestViewRendersLauncher(t *testing.T) {
	m := setupReadyModel()

	// Open launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}))
	model := updated.(Model)

	v := model.View()
	if v.Content == nil {
		t.Error("expected rendered content with launcher")
	}
}

func TestMaxWindows(t *testing.T) {
	m := setupReadyModel()
	// Open maxWindows (9) demo windows
	for i := 0; i < 9; i++ {
		m.openDemoWindow()
	}
	if m.wm.Count() != 9 {
		t.Errorf("expected 9 windows, got %d", m.wm.Count())
	}
	// 10th via openTerminalWindowWith (which checks limit) should not open
	cmd := m.openTerminalWindowWith("", nil)
	if cmd != nil {
		t.Error("expected nil cmd when at max windows")
	}
	if m.wm.Count() != 9 {
		t.Errorf("expected 9 windows after max limit, got %d", m.wm.Count())
	}
}

func TestRenameDialogOpen(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Press 'r' to open rename dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	model := updated.(Model)

	if model.renameDialog == nil {
		t.Fatal("expected rename dialog to be open")
	}
	if model.renameDialog.WindowID != fw.ID {
		t.Errorf("rename dialog window ID = %q, want %q", model.renameDialog.WindowID, fw.ID)
	}
}

func TestRenameDialogType(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	originalTitle := m.wm.FocusedWindow().Title

	// Open rename dialog (pre-filled with current title)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	model := updated.(Model)

	if string(model.renameDialog.Text) != originalTitle {
		t.Errorf("initial text = %q, want %q", string(model.renameDialog.Text), originalTitle)
	}

	// Clear with Ctrl+U, then type "hello"
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	model = updated.(Model)
	for _, ch := range "hello" {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
		model = updated.(Model)
	}
	if string(model.renameDialog.Text) != "hello" {
		t.Errorf("rename text = %q, want 'hello'", string(model.renameDialog.Text))
	}
}

func TestRenameDialogConfirm(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID

	// Open rename dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	model := updated.(Model)

	// Clear with Ctrl+U, then type "NewName"
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	model = updated.(Model)
	for _, ch := range "NewName" {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
		model = updated.(Model)
	}

	// Press Enter to confirm
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)

	if model.renameDialog != nil {
		t.Error("rename dialog should be closed after Enter")
	}
	w := model.wm.WindowByID(winID)
	if w == nil {
		t.Fatal("window not found")
	}
	if w.Title != "NewName" {
		t.Errorf("window title = %q, want 'NewName'", w.Title)
	}
}

func TestRenameDialogCancel(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	originalTitle := m.wm.FocusedWindow().Title

	// Open rename dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	model := updated.(Model)

	// Type something
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = updated.(Model)

	// Press Escape to cancel
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)

	if model.renameDialog != nil {
		t.Error("rename dialog should be closed after Escape")
	}
	if model.wm.FocusedWindow().Title != originalTitle {
		t.Error("window title should not change after cancel")
	}
}

func TestSystemStatsMsgWithBattery(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(SystemStatsMsg{
		CPU:         42.5,
		MemGB:       8.2,
		BatPct:      75,
		BatCharging: true,
		BatPresent:  true,
	})
	model := updated.(Model)
	if cmd == nil {
		t.Error("expected tick cmd from SystemStatsMsg")
	}
	if model.menuBar.CPUPct != 42.5 {
		t.Errorf("CPU = %v, want 42.5", model.menuBar.CPUPct)
	}
	if model.menuBar.MemGB != 8.2 {
		t.Errorf("MemGB = %v, want 8.2", model.menuBar.MemGB)
	}
	if model.menuBar.BatPct != 75 {
		t.Errorf("BatPct = %v, want 75", model.menuBar.BatPct)
	}
	if !model.menuBar.BatCharging {
		t.Error("expected BatCharging=true")
	}
	if !model.menuBar.BatPresent {
		t.Error("expected BatPresent=true")
	}
}

func TestExposeSelectMaximizes(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Enter expose mode
	m.exposeMode = true

	// Press "1" to select first window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model := updated.(Model)

	if model.exposeMode {
		t.Error("expose should exit after selection")
	}
	fw := model.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	if !fw.IsMaximized() {
		t.Error("selected window should be maximized")
	}
}

func TestPtyClosedClosesWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID
	if m.wm.Count() != 1 {
		t.Fatal("expected 1 window")
	}

	// Simulate PTY closed message — starts close animation
	updated, _ := m.Update(PtyClosedMsg{WindowID: winID})
	model := updated.(Model)

	// Complete the close animation
	model = completeAnimations(model)
	if model.wm.Count() != 0 {
		t.Errorf("expected 0 windows after close animation, got %d", model.wm.Count())
	}
}

// ── Prefix system tests ──

func TestPrefixKeyActivatesPending(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	// Press Ctrl+G (prefix key, default)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	model := updated.(Model)

	if !model.prefixPending {
		t.Error("expected prefixPending to be true after prefix key")
	}
	if model.inputMode != ModeTerminal {
		t.Error("should still be in Terminal mode while prefix pending")
	}
}

func TestPrefixEscExitsTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Press Esc after prefix
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)

	if model.inputMode != ModeNormal {
		t.Error("expected Normal mode after prefix + Esc")
	}
	if model.prefixPending {
		t.Error("prefixPending should be cleared")
	}
}

func TestPrefixQuitShowsDialog(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Press 'q' after prefix
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	model := updated.(Model)

	if model.confirmClose == nil {
		t.Error("expected quit confirm dialog after prefix + q")
	}
	if model.inputMode != ModeNormal {
		t.Error("should switch to Normal mode for dialog")
	}
}

func TestPrefixSnapLeftStaysInTerminal(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Press 'h' after prefix (snap left — geometry action)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	model := updated.(Model)

	// Should stay in Terminal mode (geometry action)
	if model.inputMode != ModeTerminal {
		t.Error("expected to stay in Terminal mode after prefix + snap_left")
	}
}

func TestGlobalHotkeysBlockedInTerminal(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	// Press F1 directly — should NOT open help in Terminal mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	model := updated.(Model)

	if model.modal != nil {
		t.Error("F1 should NOT trigger help overlay in Terminal mode without prefix")
	}
}

func TestGlobalHotkeysWorkInNormal(t *testing.T) {
	m := setupReadyModel()

	// F1 in Normal mode should open help
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	model := updated.(Model)

	if model.modal == nil {
		t.Error("F1 should trigger help overlay in Normal mode")
	}
}

func TestPrefixHelpOpensOverlay(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Press F1 after prefix
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	model := updated.(Model)

	if model.modal == nil {
		t.Error("expected help overlay after prefix + F1")
	}
	if model.inputMode != ModeNormal {
		t.Error("should switch to Normal mode for help overlay")
	}
}

func TestDoublePrefixForwardsKey(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Press Ctrl+G again after prefix — should forward to terminal and stay in Terminal mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	model := updated.(Model)

	if model.prefixPending {
		t.Error("prefixPending should be cleared after double-prefix")
	}
	if model.inputMode != ModeTerminal {
		t.Error("should stay in Terminal mode after double-prefix")
	}
}

func TestF2ExitsTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	// F2 always exits Terminal mode (hardcoded escape hatch)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	model := updated.(Model)

	if model.inputMode != ModeNormal {
		t.Error("F2 should exit Terminal mode")
	}
}

func TestCustomPrefix(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	// Override prefix to Ctrl+]
	m.keybindings.Prefix = "ctrl+]"
	m.actionMap = BuildActionMap(m.keybindings)
	m.inputMode = ModeTerminal

	// Default prefix (Ctrl+G) should NOT activate prefix
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.prefixPending {
		t.Error("Ctrl+G should not activate prefix when custom prefix is Ctrl+]")
	}

	// Restore Terminal mode (no terminal found causes fallback to Normal)
	model.inputMode = ModeTerminal

	// Custom prefix (Ctrl+]) should activate prefix
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ']', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if !model.prefixPending {
		t.Error("Ctrl+] should activate prefix when configured as prefix key")
	}
}

func TestBuildActionMap(t *testing.T) {
	kb := config.DefaultKeyBindings()
	am := BuildActionMap(kb)

	// Check configurable keys
	if am["q"] != "quit" {
		t.Errorf("expected q→quit, got %q", am["q"])
	}
	if am["n"] != "new_terminal" {
		t.Errorf("expected n→new_terminal, got %q", am["n"])
	}
	if am["h"] != "snap_left" {
		t.Errorf("expected h→snap_left, got %q", am["h"])
	}

	// Check hardcoded alternates
	if am["ctrl+q"] != "quit" {
		t.Errorf("expected ctrl+q→quit, got %q", am["ctrl+q"])
	}
	if am["left"] != "snap_left" {
		t.Errorf("expected left→snap_left, got %q", am["left"])
	}
	if am["enter"] != "enter_terminal" {
		t.Errorf("expected enter→enter_terminal, got %q", am["enter"])
	}
}
