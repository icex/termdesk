package app

import (
	"testing"

	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

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
	if m.Init() != nil {
		t.Error("expected nil cmd from Init")
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
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Error("expected quit on ctrl+c")
	}
}

func TestQuitCtrlQ(t *testing.T) {
	m := setupReadyModel()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Error("expected quit on ctrl+q")
	}
}

func TestQuitQNoWindows(t *testing.T) {
	m := setupReadyModel()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	if cmd == nil {
		t.Error("expected quit on q when no windows focused")
	}
}

func TestNoQuitQWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	if cmd != nil {
		t.Error("q should not quit when a window is focused")
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

func TestAltTabCycles(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	focused1 := m.wm.FocusedWindow().ID

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModAlt}))
	model := updated.(Model)
	focused2 := model.wm.FocusedWindow().ID

	if focused1 == focused2 {
		t.Error("alt+tab should change focused window")
	}
}

func TestCtrlWCloses(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	if m.wm.Count() != 1 {
		t.Fatal("expected 1 window")
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.wm.Count() != 0 {
		t.Errorf("expected 0 windows after ctrl+w, got %d", model.wm.Count())
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

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl}))
	model := updated.(Model)
	fw := model.wm.FocusedWindow()
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap left: width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}))
	model = updated.(Model)
	fw = model.wm.FocusedWindow()
	if fw.Rect.X != wa.Width/2 {
		t.Errorf("snap right: x = %d, want %d", fw.Rect.X, wa.Width/2)
	}
}

func TestMaximizeRestore(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	origRect := m.wm.FocusedWindow().Rect

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.wm.FocusedWindow().IsMaximized() {
		t.Error("expected maximized after ctrl+up")
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown, Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.wm.FocusedWindow().Rect != origRect {
		t.Error("expected original rect after ctrl+down restore")
	}
}

func TestTileAll(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}))
	model := updated.(Model)
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

	// Click close button position
	closePos := w.CloseButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: closePos.X + 1, Y: closePos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	if model.wm.Count() != 0 {
		t.Errorf("expected window closed, count = %d", model.wm.Count())
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
	if v.MouseMode != tea.MouseModeAllMotion {
		t.Error("expected MouseModeAllMotion")
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
	// These should not panic with no focused window
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight, Mod: tea.ModCtrl}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown, Mod: tea.ModCtrl}))
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

	// Ctrl+Q should still quit even with launcher open
	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Error("ctrl+q should quit even with launcher open")
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

func TestPtyClosedClosesWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID
	if m.wm.Count() != 1 {
		t.Fatal("expected 1 window")
	}

	// Simulate PTY closed message
	updated, _ := m.Update(PtyClosedMsg{WindowID: winID})
	model := updated.(Model)
	if model.wm.Count() != 0 {
		t.Errorf("expected 0 windows after PtyClosedMsg, got %d", model.wm.Count())
	}
}
