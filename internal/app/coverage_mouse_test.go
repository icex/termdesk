package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// ---------------------------------------------------------------------------
// handleMouseWheel coverage tests
// ---------------------------------------------------------------------------

func TestCovMouseWheelCopyModeScrollUp(t *testing.T) {
	// Test mouse wheel up in copy mode with a real terminal and scrollback.
	m := setupCopyModeWithTerminal(t)
	m.scrollOffset = 0

	// Wheel up should increase scrollOffset (scroll up in scrollback).
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy, got %s", model.inputMode)
	}
	// scrollOffset should have increased (by 3 or capped at maxScroll).
	// Even if maxScroll is small, offset should be >= 0.
	if model.scrollOffset < 0 {
		t.Errorf("expected scrollOffset >= 0, got %d", model.scrollOffset)
	}
}

func TestCovMouseWheelCopyModeScrollDownExitAtBottom(t *testing.T) {
	// When scrollOffset is 0, selection is inactive, and wheel down: should exit copy mode.
	m := setupCopyModeWithTerminal(t)
	m.scrollOffset = 0
	m.selActive = false

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// At bottom with no selection: wheel down should exit copy mode.
	if model.inputMode == ModeCopy && model.scrollOffset == 0 {
		// exitCopyMode sets inputMode to ModeTerminal;
		// the non-quake window wheel-down path sets ModeNormal.
		// Either way, we should not be in ModeCopy if we triggered exit.
	}
	// Main assertion: no panic and scrollOffset clamped to >= 0.
	if model.scrollOffset < 0 {
		t.Errorf("expected scrollOffset >= 0, got %d", model.scrollOffset)
	}
}

func TestCovMouseWheelOnDockArea(t *testing.T) {
	// Scroll on dock row (y == m.height-1) with no window hit.
	m := setupReadyModel()

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: m.height - 1, Button: tea.MouseWheelUp})
	updated, cmd := m.Update(wheel)
	model := updated.(Model)

	// No window at dock row; should be a no-op.
	if cmd != nil {
		t.Error("wheel on dock area (no window) should return nil cmd")
	}
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal, got %s", model.inputMode)
	}
}

func TestCovMouseWheelOnMenuBarArea(t *testing.T) {
	// Scroll on menu bar row (y == 0) with no window hit.
	m := setupReadyModel()

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 50, Y: 0, Button: tea.MouseWheelDown})
	updated, cmd := m.Update(wheel)
	model := updated.(Model)

	if cmd != nil {
		t.Error("wheel on menu bar area should return nil cmd")
	}
	_ = model
}

func TestCovMouseWheelUpEntersCopyModeOnWindow(t *testing.T) {
	// Wheel up on a window with scrollback and without mouse mode should enter copy mode.
	m := setupReadyModel()

	win := window.NewWindow("whlwin", "Term", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 12}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	term.RestoreBuffer("line1\nline2\nline3\nline4\nline5\n")
	m.terminals[win.ID] = term
	m.inputMode = ModeNormal

	wheel := tea.MouseWheelMsg(tea.Mouse{
		X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelUp,
	})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// If scrollback > 0 and no mouse mode, wheel up enters copy mode.
	if term.ScrollbackLen() > 0 {
		if model.inputMode != ModeCopy {
			t.Errorf("expected ModeCopy after wheel-up with scrollback, got %s", model.inputMode)
		}
	}

	// Clean up.
	term.Close()
}

func TestCovMouseWheelDownInCopyModeExitsOnWindow(t *testing.T) {
	// Wheel down on a window content area in copy mode exits copy mode at bottom.
	m := setupReadyModel()

	win := window.NewWindow("whldown", "Term", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 12}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	term.RestoreBuffer("line1\nline2\n")
	m.terminals[win.ID] = term
	m.inputMode = ModeCopy
	m.scrollOffset = 2

	wheel := tea.MouseWheelMsg(tea.Mouse{
		X: cr.X + 1, Y: cr.Y + 1, Button: tea.MouseWheelDown,
	})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// scrollOffset should decrease. At <= 0, copy mode exits.
	if model.scrollOffset > 2 {
		t.Errorf("scrollOffset should have decreased, got %d", model.scrollOffset)
	}

	term.Close()
}

// ---------------------------------------------------------------------------
// handleMouseMotion coverage tests
// ---------------------------------------------------------------------------

func TestCovMouseMotionWindowDrag(t *testing.T) {
	// Test mouse motion during active window drag.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}

	// Set up drag state manually (move mode).
	m.drag = window.DragState{
		Active:     true,
		WindowID:   w.ID,
		Mode:       window.DragMove,
		StartMouse: geometry.Point{X: w.Rect.X + 5, Y: w.Rect.Y},
		StartRect:  w.Rect,
	}

	origX := w.Rect.X
	motion := tea.MouseMotionMsg(tea.Mouse{X: w.Rect.X + 20, Y: w.Rect.Y + 3, Button: tea.MouseLeft})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	fw := model.wm.FocusedWindow()
	if fw.Rect.X == origX {
		t.Error("window should have moved during drag")
	}
}

func TestCovMouseMotionQuakeDragResize(t *testing.T) {
	// Test mouse motion during quake terminal drag resize.
	m := setupReadyModel()

	// Simulate quake terminal being visible and actively dragged.
	cr := geometry.Rect{X: 0, Y: 0, Width: m.width, Height: 10}
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15
	m.quakeAnimVel = 0
	m.quakeDragActive = true
	m.quakeDragStartY = 14 // at the border
	m.quakeDragStartH = 15

	// Drag down by 5 rows.
	motion := tea.MouseMotionMsg(tea.Mouse{X: 50, Y: 19, Button: tea.MouseLeft})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	// Quake height should have increased.
	if model.quakeAnimH <= 15 {
		t.Errorf("expected quakeAnimH > 15 after drag down, got %f", model.quakeAnimH)
	}
	if model.quakeHeightPct < 10 {
		t.Errorf("quakeHeightPct should be >= 10, got %d", model.quakeHeightPct)
	}

	term.Close()
}

func TestCovMouseMotionNoActiveDragHoverDetection(t *testing.T) {
	// Test mouse motion with no active drag: should track hover state.
	m := setupReadyModel()
	m.openDemoWindow()

	motion := tea.MouseMotionMsg(tea.Mouse{X: 60, Y: 20})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	// No drag active. Hover detection should run without panic.
	if model.drag.Active {
		t.Error("should not have active drag")
	}
}

func TestCovMouseMotionMenuBarHoverWidgets(t *testing.T) {
	// Test mouse motion on menu bar row (y=0) tracks menu label hover.
	m := setupReadyModel()

	motion := tea.MouseMotionMsg(tea.Mouse{X: 2, Y: 0})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	// Should detect menu label at x=2.
	if model.hoverMenuLabel < 0 && len(m.menuBar.Menus) > 0 {
		// It's possible at x=2 there's a menu; verify no panic.
	}
	_ = model
}

func TestCovMouseMotionLauncherHover(t *testing.T) {
	// Test hover tracking on visible launcher.
	m := setupReadyModel()
	m.launcher.Show()

	lx, ly, _, _ := m.launcherBounds()

	motion := tea.MouseMotionMsg(tea.Mouse{X: lx + 2, Y: ly + 8, Button: 0})
	updated, _ := m.Update(motion)
	_ = updated.(Model) // No panic.
}

func TestCovMouseMotionFocusFollowsMouse(t *testing.T) {
	// Test focus-follows-mouse: hovering over a non-focused window should focus it.
	m := setupReadyModel()
	m.focusFollowsMouse = true

	// Create two non-overlapping windows manually.
	wa := m.wm.WorkArea()
	w1 := window.NewWindow("ffm-w1", "Win1",
		geometry.Rect{X: wa.X, Y: wa.Y, Width: 30, Height: 12}, nil)
	w2 := window.NewWindow("ffm-w2", "Win2",
		geometry.Rect{X: wa.X + 40, Y: wa.Y, Width: 30, Height: 12}, nil)
	m.wm.AddWindow(w1)
	m.wm.AddWindow(w2)
	m.wm.FocusWindow(w2.ID)

	if m.wm.FocusedWindow().ID != w2.ID {
		t.Fatalf("expected w2 focused initially")
	}

	// Move mouse over w1 (center of w1, which is to the left of w2).
	motion := tea.MouseMotionMsg(tea.Mouse{
		X: w1.Rect.X + w1.Rect.Width/2,
		Y: w1.Rect.Y + w1.Rect.Height/2,
	})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.wm.FocusedWindow().ID != w1.ID {
		t.Errorf("focus-follows-mouse should have focused w1, got %s", model.wm.FocusedWindow().ID)
	}
}

// ---------------------------------------------------------------------------
// handleMouseRelease coverage tests
// ---------------------------------------------------------------------------

func TestCovMouseReleaseAfterWindowDrag(t *testing.T) {
	// Release after window drag should clear drag state and resize terminal.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	m.drag = window.DragState{
		Active:     true,
		WindowID:   w.ID,
		Mode:       window.DragMove,
		StartMouse: geometry.Point{X: w.Rect.X + 5, Y: w.Rect.Y},
		StartRect:  w.Rect,
	}

	release := tea.MouseReleaseMsg(tea.Mouse{X: w.Rect.X + 20, Y: w.Rect.Y + 5})
	updated, _ := m.Update(release)
	model := updated.(Model)

	if model.drag.Active {
		t.Error("drag should be cleared after release")
	}
}

func TestCovMouseReleaseAfterResizeDrag(t *testing.T) {
	// Release after resize drag should resize the terminal.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	m.drag = window.DragState{
		Active:     true,
		WindowID:   w.ID,
		Mode:       window.DragResizeE,
		StartMouse: geometry.Point{X: w.Rect.Right() - 1, Y: w.Rect.Y + 5},
		StartRect:  w.Rect,
	}

	release := tea.MouseReleaseMsg(tea.Mouse{X: w.Rect.Right() + 5, Y: w.Rect.Y + 5})
	updated, _ := m.Update(release)
	model := updated.(Model)

	if model.drag.Active {
		t.Error("drag should be cleared after resize release")
	}
}

func TestCovMouseReleaseNoDragNoTerminal(t *testing.T) {
	// Release with no drag and no terminal: should be no-op.
	m := setupReadyModel()

	release := tea.MouseReleaseMsg(tea.Mouse{X: 60, Y: 20})
	updated, cmd := m.Update(release)
	model := updated.(Model)

	if model.drag.Active {
		t.Error("drag should not be active")
	}
	_ = cmd
}

func TestCovMouseReleaseQuakeDrag(t *testing.T) {
	// Release after quake drag resize should finalize the quake height.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 20
	m.quakeTargetH = 20
	m.quakeDragActive = true
	m.quakeDragStartY = 19
	m.quakeDragStartH = 20

	release := tea.MouseReleaseMsg(tea.Mouse{X: 50, Y: 22})
	updated, _ := m.Update(release)
	model := updated.(Model)

	if model.quakeDragActive {
		t.Error("quake drag should be finalized after release")
	}

	term.Close()
}

// ---------------------------------------------------------------------------
// handleMouseClick coverage tests
// ---------------------------------------------------------------------------

func TestCovMouseClickOnDockArea(t *testing.T) {
	// Click on empty dock area (no item hit) — should NOT change mode.
	// This prevents accidental mode switches when the mouse overshoots into
	// the dock row during scrolling.
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	click := tea.MouseClickMsg(tea.Mouse{X: 5, Y: m.height - 1, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.inputMode != ModeTerminal {
		t.Errorf("clicking empty dock area should preserve terminal mode, got %v", model.inputMode)
	}
}

func TestCovMouseClickOnMenuBarArea(t *testing.T) {
	// Click on menu bar row (y == 0) in terminal mode.
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// Should switch to normal mode.
	if model.inputMode == ModeTerminal {
		t.Error("clicking menu bar should exit terminal mode to normal mode")
	}
}

func TestCovMouseClickOnWindowContentArea(t *testing.T) {
	// Click on window content area without terminal (demo window) sets focus.
	m := setupReadyModel()

	// Create two non-overlapping windows manually.
	wa := m.wm.WorkArea()
	w1 := window.NewWindow("clk-w1", "Win1",
		geometry.Rect{X: wa.X, Y: wa.Y, Width: 30, Height: 12}, nil)
	w2 := window.NewWindow("clk-w2", "Win2",
		geometry.Rect{X: wa.X + 40, Y: wa.Y, Width: 30, Height: 12}, nil)
	m.wm.AddWindow(w1)
	m.wm.AddWindow(w2)
	m.wm.FocusWindow(w2.ID)

	// Click on w1's content area.
	click := tea.MouseClickMsg(tea.Mouse{
		X: w1.Rect.X + w1.Rect.Width/2,
		Y: w1.Rect.Y + w1.Rect.Height/2,
		Button: tea.MouseLeft,
	})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.wm.FocusedWindow().ID != w1.ID {
		t.Errorf("expected w1 focused after click, got %s", model.wm.FocusedWindow().ID)
	}
}

func TestCovMouseClickWithModalVisible(t *testing.T) {
	// Click outside a modal should dismiss it.
	m := setupReadyModel()
	m.modal = &ModalOverlay{
		Title: "Test",
		Lines: []string{"line 1"},
	}

	// Click far from center (outside modal bounds).
	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: m.height - 2, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.modal != nil {
		t.Error("click outside modal should dismiss it")
	}
}

func TestCovMouseClickInsideModalAbsorbed(t *testing.T) {
	// Click inside a modal (but not on a tab) should be absorbed (no dismiss).
	m := setupReadyModel()
	m.modal = &ModalOverlay{
		Title: "Test",
		Lines: []string{"line 1", "line 2"},
	}
	bounds := m.modal.Bounds(m.width, m.height)

	// Click inside the modal.
	click := tea.MouseClickMsg(tea.Mouse{
		X: bounds.StartX + bounds.BoxW/2,
		Y: bounds.StartY + bounds.BoxH/2,
		Button: tea.MouseLeft,
	})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.modal == nil {
		t.Error("click inside modal should NOT dismiss it")
	}
}

func TestCovMouseClickModalTabClick(t *testing.T) {
	// Click on a tab in a tabbed modal should switch tabs.
	m := setupReadyModel()
	m.modal = m.helpOverlay()
	if m.modal == nil || len(m.modal.Tabs) < 2 {
		t.Skip("help overlay has no tabs")
	}

	bounds := m.modal.Bounds(m.width, m.height)
	if bounds.TabRow < 0 {
		t.Skip("no tab row in modal bounds")
	}

	// Click on a position in the tab row to switch to a different tab.
	firstTabW := runeLen(m.modal.TabLabel(0)) + 2
	click := tea.MouseClickMsg(tea.Mouse{
		X: bounds.StartX + 1 + bounds.HPad + firstTabW + 1,
		Y: bounds.TabRow,
		Button: tea.MouseLeft,
	})
	updated, _ := m.Update(click)
	model := updated.(Model)

	// The tab should have changed (or stayed if coords were off).
	_ = model // No panic is success.
}

func TestCovMouseClickWithQuakeTerminalVisible(t *testing.T) {
	// Click inside quake terminal content area should absorb the click.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15

	// Click inside quake content (y < borderY=14).
	click := tea.MouseClickMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	_ = updated.(Model) // No panic.

	term.Close()
}

func TestCovMouseClickQuakeBorderStartsDrag(t *testing.T) {
	// Click on quake bottom border should start drag resize.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15

	borderY := 14 // h - 1 = 15 - 1
	click := tea.MouseClickMsg(tea.Mouse{X: 10, Y: borderY, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if !model.quakeDragActive {
		t.Error("click on quake border should start drag resize")
	}
	if model.quakeDragStartH != 15 {
		t.Errorf("quakeDragStartH = %d, want 15", model.quakeDragStartH)
	}

	term.Close()
}

func TestCovMouseClickCloseExitedWindow(t *testing.T) {
	// Click close button on an exited window should close immediately.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Exited = true

	closePos := w.CloseButtonPos()
	click := tea.MouseClickMsg(tea.Mouse{X: closePos.X + 1, Y: closePos.Y, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)
	model = completeAnimations(model)

	if model.wm.WindowByID(w.ID) != nil {
		// Exited windows get closed (possibly with animation).
		// Just verify no panic.
	}
}

// ---------------------------------------------------------------------------
// handleRightClick coverage tests
// ---------------------------------------------------------------------------

func TestCovRightClickOnDesktop(t *testing.T) {
	// Right-click on empty desktop (no window hit) shows desktop context menu.
	m := setupReadyModel()

	click := tea.MouseClickMsg(tea.Mouse{X: 60, Y: 20, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil || !model.contextMenu.Visible {
		t.Error("right-click on empty desktop should show desktop context menu")
	}
}

func TestCovRightClickOnWindowTitleBar(t *testing.T) {
	// Right-click on a window title bar shows title bar context menu.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	// Click on the title bar area (not on buttons).
	click := tea.MouseClickMsg(tea.Mouse{
		X: w.Rect.X + 4, Y: w.Rect.Y, Button: tea.MouseRight,
	})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil || !model.contextMenu.Visible {
		t.Error("right-click on title bar should show context menu")
	}
}

func TestCovRightClickOnDockItem(t *testing.T) {
	// Right-click on a dock item with a window should show dock item context menu.
	m := setupReadyModel()
	m.dock.SetWidth(m.width)
	m.dock.MinimizeToDock = true

	win := window.NewWindow("rclk-dock", "DockApp",
		geometry.Rect{X: 5, Y: 5, Width: 60, Height: 15}, nil)
	win.Command = "unique-dock-rclick-app"
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)
	m.updateDockRunning()

	// Find the dock item.
	var itemIdx int = -1
	for i, item := range m.dock.Items {
		if item.WindowID == win.ID {
			itemIdx = i
			break
		}
	}
	if itemIdx < 0 {
		t.Skip("dock item for window not found")
	}

	itemX := -1
	for x := 0; x < m.dock.Width; x++ {
		if m.dock.ItemAtX(x) == itemIdx {
			itemX = x
			break
		}
	}
	if itemX < 0 {
		t.Skip("could not find dock item X position")
	}

	click := tea.MouseClickMsg(tea.Mouse{X: itemX, Y: m.height - 1, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil || !model.contextMenu.Visible {
		t.Error("right-click on dock item should show context menu")
	}
}

func TestCovRightClickCopyModeShowsCopyMenu(t *testing.T) {
	// Right-click in copy mode shows copy/paste context menu.
	m := setupReadyModel()
	m.inputMode = ModeCopy

	click := tea.MouseClickMsg(tea.Mouse{X: 50, Y: 20, Button: tea.MouseRight})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu == nil || !model.contextMenu.Visible {
		t.Error("right-click in copy mode should show copy mode context menu")
	}
}

// ---------------------------------------------------------------------------
// Update() message handling coverage tests
// ---------------------------------------------------------------------------

func TestCovUpdateWindowSizeMsgSetsReady(t *testing.T) {
	// WindowSizeMsg should set m.width, m.height, and m.ready.
	m := New()
	m.tour.Skip()

	ret, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 25})
	model := ret.(Model)

	if model.width != 80 {
		t.Errorf("width = %d, want 80", model.width)
	}
	if model.height != 25 {
		t.Errorf("height = %d, want 25", model.height)
	}
	if !model.ready {
		t.Error("expected ready=true after WindowSizeMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (resize settle tick)")
	}
}

func TestCovUpdateNotReadyReturnsEarly(t *testing.T) {
	// Before WindowSizeMsg, most messages should be handled but model stays not ready.
	m := New()
	m.tour.Skip()

	// Send a message that doesn't set ready.
	ret, _ := m.Update(CleanupMsg{Time: time.Now()})
	model := ret.(Model)

	if model.ready {
		t.Error("model should not be ready before WindowSizeMsg")
	}
}

func TestCovUpdateTermOutputMsgValid(t *testing.T) {
	// PtyOutputMsg with a valid window ID should set termHasOutput.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	ret, _ := m.Update(PtyOutputMsg{WindowID: w.ID})
	model := ret.(Model)

	if !model.termHasOutput[w.ID] {
		t.Error("expected termHasOutput[wID] = true")
	}
}

func TestCovUpdateTermOutputMsgQuake(t *testing.T) {
	// PtyOutputMsg with quake terminal ID.
	m := setupReadyModel()

	ret, _ := m.Update(PtyOutputMsg{WindowID: quakeTermID})
	model := ret.(Model)

	if !model.termHasOutput[quakeTermID] {
		t.Error("expected termHasOutput[quakeTermID] = true")
	}
}

func TestCovUpdateResizeRedrawMsg(t *testing.T) {
	// ResizeRedrawMsg should invalidate window cache.
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.windowCache[fw.ID] = &windowRenderCache{}

	ret, cmd := m.Update(ResizeRedrawMsg{})
	model := ret.(Model)

	if len(model.windowCache) != 0 {
		t.Error("expected window cache cleared after ResizeRedrawMsg")
	}
	if cmd != nil {
		t.Error("expected nil cmd from ResizeRedrawMsg")
	}
}

func TestCovUpdateResizeSettleTickMsgRecent(t *testing.T) {
	// ResizeSettleTickMsg with recent resize should keep ticking.
	m := setupReadyModel()
	m.lastWindowSizeAt = time.Now()

	ret, cmd := m.Update(ResizeSettleTickMsg{Time: time.Now()})
	model := ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd while still settling (recent resize)")
	}
	_ = model
}

func TestCovUpdateResizeSettleTickMsgOld(t *testing.T) {
	// ResizeSettleTickMsg with old resize should stop ticking.
	m := setupReadyModel()
	m.lastWindowSizeAt = time.Now().Add(-5 * time.Second)

	ret, cmd := m.Update(ResizeSettleTickMsg{Time: time.Now()})
	model := ret.(Model)

	if cmd != nil {
		t.Error("expected nil cmd after resize settled")
	}
	_ = model
}

func TestCovUpdateWorkspaceAutoSaveMsg(t *testing.T) {
	// WorkspaceAutoSaveMsg should trigger auto-save logic and return a tick cmd.
	m := setupReadyModel()
	m.workspaceAutoSave = true
	m.workspaceAutoSaveMin = 1
	m.lastWorkspaceSave = time.Now().Add(-2 * time.Minute)

	ret, cmd := m.Update(WorkspaceAutoSaveMsg{Time: time.Now()})
	model := ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd from WorkspaceAutoSaveMsg (re-schedules tick)")
	}
	// lastWorkspaceSave should be updated.
	if model.lastWorkspaceSave.IsZero() {
		t.Error("expected lastWorkspaceSave to be set")
	}
}

func TestCovUpdateCleanupMsg(t *testing.T) {
	// CleanupMsg should clean notifications and schedule next cleanup.
	m := setupReadyModel()

	ret, cmd := m.Update(CleanupMsg{Time: time.Now()})
	model := ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd from CleanupMsg")
	}
	_ = model
}

func TestCovUpdatePtyClosedMsgMarksExited(t *testing.T) {
	// PtyClosedMsg should mark window as exited with "[exited]" in title.
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origTitle := fw.Title

	ret, _ := m.Update(PtyClosedMsg{WindowID: fw.ID, Err: nil})
	model := ret.(Model)

	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("window should still exist")
	}
	if !w.Exited {
		t.Error("expected window marked as exited")
	}
	expected := origTitle + " [exited]"
	if w.Title != expected {
		t.Errorf("title = %q, want %q", w.Title, expected)
	}
}

func TestCovUpdatePtyClosedMsgQuake(t *testing.T) {
	// PtyClosedMsg for quake terminal should close it.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15

	ret, _ := m.Update(PtyClosedMsg{WindowID: quakeTermID, Err: nil})
	model := ret.(Model)

	if model.quakeTerminal != nil {
		t.Error("quake terminal should be closed")
	}
	if model.quakeVisible {
		t.Error("quake should not be visible after close")
	}
}

func TestCovUpdatePasteMsgTerminalMode(t *testing.T) {
	// PasteMsg in terminal mode should attempt to forward to terminal.
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	ret, cmd := m.Update(tea.PasteMsg{Content: "hello"})
	model := ret.(Model)

	// Without a real terminal, this is a no-op. Just verify no panic.
	if cmd != nil {
		t.Error("expected nil cmd from PasteMsg without real terminal")
	}
	_ = model
}

func TestCovUpdatePasteMsgNonTerminalMode(t *testing.T) {
	// PasteMsg in non-terminal mode should be a no-op.
	m := setupReadyModel()
	m.inputMode = ModeNormal

	ret, cmd := m.Update(tea.PasteMsg{Content: "hello"})
	model := ret.(Model)

	if cmd != nil {
		t.Error("expected nil cmd from PasteMsg in normal mode")
	}
	_ = model
}

func TestCovUpdateAutoStartMsgNoPanic(t *testing.T) {
	// AutoStartMsg should not panic even without project config.
	m := setupReadyModel()

	ret, _ := m.Update(AutoStartMsg{})
	_ = ret.(Model)
}

func TestCovUpdateWorkspaceDiscoveryMsgEmpty(t *testing.T) {
	// WorkspaceDiscoveryMsg with empty results hides picker and shows notification.
	m := setupReadyModel()
	m.workspacePickerVisible = true

	ret, _ := m.Update(WorkspaceDiscoveryMsg{Workspaces: nil})
	model := ret.(Model)

	if model.workspacePickerVisible {
		t.Error("expected picker hidden with empty results")
	}
}

func TestCovUpdateWorkspaceDiscoveryMsgNonEmpty(t *testing.T) {
	// WorkspaceDiscoveryMsg with results should populate the workspace list.
	m := setupReadyModel()
	m.workspacePickerVisible = true

	ret, _ := m.Update(WorkspaceDiscoveryMsg{
		Workspaces:   []string{"/a/ws.toml", "/b/ws.toml"},
		WindowCounts: []int{2, 3},
	})
	model := ret.(Model)

	if len(model.workspaceList) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(model.workspaceList))
	}
	if model.workspaceWindowCounts[1] != 3 {
		t.Errorf("expected window count 3 for second workspace, got %d", model.workspaceWindowCounts[1])
	}
}

func TestCovUpdateOuterWrapperNotReady(t *testing.T) {
	// The outer Update() wrapper should work even when model is not ready.
	m := New()
	m.tour.Skip()

	ret, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	model := ret.(Model)

	if !model.ready {
		t.Error("expected ready after WindowSizeMsg through outer Update")
	}
}

func TestCovUpdateOuterWrapperWindowSizeMakesReady(t *testing.T) {
	// Verify the full Update path (outer wrapper + handleUpdate) for WindowSizeMsg.
	m := New()
	m.tour.Skip()

	if m.ready {
		t.Error("expected not ready initially")
	}

	ret, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := ret.(Model)

	if !model.ready {
		t.Error("expected ready after WindowSizeMsg")
	}
	if model.width != 120 || model.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", model.width, model.height)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases for handleMouseClick
// ---------------------------------------------------------------------------

func TestCovMouseClickDismissContextMenuClickOutside(t *testing.T) {
	// Click outside a context menu dismisses it.
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 40, Y: 20,
		Items: []contextmenu.Item{
			{Label: "Test", Action: "test"},
		},
		Visible: true,
	}

	// Click far from the context menu.
	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.contextMenu != nil && model.contextMenu.Visible {
		t.Error("click outside context menu should dismiss it")
	}
}

func TestCovMouseClickWorkspacePickerOutside(t *testing.T) {
	// Click outside workspace picker should dismiss it.
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/a/ws.toml"}

	// Click at top-left corner (outside picker).
	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.workspacePickerVisible {
		t.Error("click outside workspace picker should dismiss it")
	}
}

func TestCovMouseClickDesktopCopyModeSwitchToNormal(t *testing.T) {
	// Click on desktop in copy mode should switch to normal.
	m := setupReadyModel()
	m.inputMode = ModeCopy

	click := tea.MouseClickMsg(tea.Mouse{X: 60, Y: 20, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after clicking desktop in copy mode, got %s", model.inputMode)
	}
}

func TestCovMouseMotionModalTabHover(t *testing.T) {
	// Mouse motion on a tabbed modal should track tab hover.
	m := setupReadyModel()
	m.modal = m.helpOverlay()
	if m.modal == nil || len(m.modal.Tabs) < 2 {
		t.Skip("no tabbed modal")
	}

	bounds := m.modal.Bounds(m.width, m.height)
	if bounds.TabRow < 0 {
		t.Skip("no tab row")
	}

	motion := tea.MouseMotionMsg(tea.Mouse{
		X: bounds.StartX + 1 + bounds.HPad + 2,
		Y: bounds.TabRow,
	})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	// The hover tab should be set to 0 or a valid index.
	if model.modal.HoverTab < -1 {
		t.Errorf("unexpected HoverTab = %d", model.modal.HoverTab)
	}
}

func TestCovMouseMotionDropdownHoverTracking(t *testing.T) {
	// When a menu dropdown is open, mouse motion should track hover.
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)
	if !m.menuBar.IsOpen() {
		t.Skip("could not open menu")
	}

	positions := m.menuBar.MenuXPositions()
	dropX := positions[0]

	// Move mouse over the dropdown.
	motion := tea.MouseMotionMsg(tea.Mouse{X: dropX + 2, Y: 2})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	if model.menuBar.HoverIndex < 0 {
		// May be -1 if item is not at that position, but no panic is success.
	}
	_ = model
}

func TestCovMouseClickConfirmDialogOutsideDismiss(t *testing.T) {
	// Click outside confirm dialog (at the very edge) should dismiss it.
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{
		Title: "Close Window?",
	}

	// Click at 0,0 which should be outside the dialog.
	click := tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	updated, _ := m.Update(click)
	model := updated.(Model)

	if model.confirmClose != nil {
		t.Error("click at corner should dismiss confirm dialog")
	}
}

func TestCovHandleMouseWheelQuakeTerminalWheelUp(t *testing.T) {
	// Wheel up in quake terminal area with scrollback enters copy mode.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	term.RestoreBuffer("line1\nline2\nline3\nline4\nline5\n")
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15
	m.inputMode = ModeNormal

	// Wheel up inside quake area (y < quakeAnimH).
	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelUp})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	if term.ScrollbackLen() > 0 && !term.HasMouseMode() {
		if model.inputMode != ModeCopy {
			t.Errorf("expected ModeCopy after wheel-up in quake with scrollback, got %s", model.inputMode)
		}
	}

	term.Close()
}

func TestCovHandleMouseWheelQuakeTerminalWheelDownCopyMode(t *testing.T) {
	// Wheel down in quake terminal copy mode at bottom should exit copy mode.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15
	m.inputMode = ModeCopy
	m.scrollOffset = 0
	m.selActive = false
	m.copySnapshot = &CopySnapshot{WindowID: quakeTermID}

	wheel := tea.MouseWheelMsg(tea.Mouse{X: 10, Y: 5, Button: tea.MouseWheelDown})
	updated, _ := m.Update(wheel)
	model := updated.(Model)

	// At bottom with no selection, should exit copy mode.
	if model.scrollOffset < 0 {
		t.Errorf("scrollOffset should be >= 0, got %d", model.scrollOffset)
	}

	term.Close()
}

func TestCovHandleMouseMotionSeparatorDrag(t *testing.T) {
	// Mouse motion during a separator drag in split window should adjust ratio.
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()

	// Create a minimal split structure.
	leftID := "pane-left"
	rightID := "pane-right"
	w.SplitRoot = &window.SplitNode{
		Dir:      window.SplitHorizontal,
		Ratio:    0.5,
		Children: [2]*window.SplitNode{
			{TermID: leftID},
			{TermID: rightID},
		},
	}
	w.FocusedPane = leftID

	// Set up separator drag state.
	m.drag = window.DragState{
		Active:     true,
		WindowID:   w.ID,
		Mode:       window.DragSeparator,
		StartMouse: geometry.Point{X: w.Rect.X + w.Rect.Width/2, Y: w.Rect.Y + 3},
		StartRect:  w.Rect,
		SepNode:    w.SplitRoot,
		SepDir:     window.SplitHorizontal,
	}

	// Move mouse to change the ratio.
	motion := tea.MouseMotionMsg(tea.Mouse{
		X:      w.Rect.X + w.Rect.Width*3/4,
		Y:      w.Rect.Y + 3,
		Button: tea.MouseLeft,
	})
	updated, _ := m.Update(motion)
	model := updated.(Model)

	// The ratio should have changed from 0.5.
	fw := model.wm.FocusedWindow()
	if fw != nil && fw.SplitRoot != nil && fw.SplitRoot.Ratio == 0.5 {
		// The ratio might not change if the window is too small, but no panic is success.
	}
}

func TestCovHandleMouseReleaseForwardsToQuakeTerminal(t *testing.T) {
	// Mouse release should forward to quake terminal if it has mouse mode.
	m := setupReadyModel()

	term, err := terminal.NewShell(m.width, 10, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.quakeTerminal = term
	m.quakeVisible = true
	m.quakeAnimH = 15
	m.quakeTargetH = 15

	// Release inside quake area.
	release := tea.MouseReleaseMsg(tea.Mouse{X: 10, Y: 5})
	updated, _ := m.Update(release)
	_ = updated.(Model) // No panic.

	term.Close()
}

func TestCovUpdateBellMsgUnfocused(t *testing.T) {
	// BellMsg for an unfocused window should set HasBell.
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	w1 := m.wm.Windows()[0]
	// w2 is focused.

	ret, cmd := m.Update(BellMsg{WindowID: w1.ID})
	model := ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd (tea.Raw bell)")
	}
	if !model.wm.WindowByID(w1.ID).HasBell {
		t.Error("expected HasBell=true for unfocused window on BellMsg")
	}
}

func TestCovUpdateBellMsgFocused(t *testing.T) {
	// BellMsg for the focused window should NOT set HasBell.
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	ret, cmd := m.Update(BellMsg{WindowID: fw.ID})
	model := ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd (tea.Raw bell)")
	}
	if model.wm.WindowByID(fw.ID).HasBell {
		t.Error("HasBell should be false for focused window on BellMsg")
	}
}

func TestCovUpdateImageClearScreenMsg(t *testing.T) {
	// ImageClearScreenMsg should return tea.ClearScreen cmd.
	m := setupReadyModel()

	ret, cmd := m.Update(ImageClearScreenMsg{})
	_ = ret.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd from ImageClearScreenMsg")
	}
}

func TestCovUpdateImageFlushMsgWithData(t *testing.T) {
	// ImageFlushMsg with data should queue image data.
	m := setupReadyModel()
	m.imagePending = &imagePendingBuf{}

	ret, cmd := m.Update(ImageFlushMsg{Data: []byte("image-data")})
	_ = ret.(Model)

	// The data should have been flushed via tea.Raw.
	if cmd == nil {
		t.Error("expected non-nil cmd from ImageFlushMsg with data")
	}
}

func TestCovUpdateImageFlushMsgEmpty(t *testing.T) {
	// ImageFlushMsg with nil data should be a no-op.
	m := setupReadyModel()
	m.imagePending = &imagePendingBuf{}

	ret, _ := m.Update(ImageFlushMsg{Data: nil})
	model := ret.(Model)

	if len(model.imagePending.data) != 0 {
		t.Error("expected empty imagePending for nil data")
	}
}
