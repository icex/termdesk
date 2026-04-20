package app

import (
	"testing"
	"time"

	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// Tests prefixed with TestCS (Coverage Small) to avoid name conflicts.

// ══════════════════════════════════════════════
// Quake terminal helper functions
// ══════════════════════════════════════════════

func TestCSQuakeFullHeightDefault(t *testing.T) {
	m := setupReadyModel()
	m.quakeHeightPct = 0 // triggers default 40%
	h := m.quakeFullHeight()
	expected := m.height * 40 / 100
	if h != expected {
		t.Errorf("quakeFullHeight() default = %d, want %d", h, expected)
	}
}

func TestCSQuakeFullHeightCustom(t *testing.T) {
	m := setupReadyModel()
	m.quakeHeightPct = 60
	h := m.quakeFullHeight()
	expected := m.height * 60 / 100
	if h != expected {
		t.Errorf("quakeFullHeight() 60%% = %d, want %d", h, expected)
	}
}

func TestCSQuakeFullHeightMinimum(t *testing.T) {
	m := setupReadyModel()
	m.height = 8
	m.quakeHeightPct = 10
	h := m.quakeFullHeight()
	if h < 5 {
		t.Errorf("quakeFullHeight() should clamp to 5, got %d", h)
	}
}

func TestCSQuakeContentSize(t *testing.T) {
	m := setupReadyModel()
	m.quakeHeightPct = 40
	cols, rows := m.quakeContentSize()
	if cols != m.width {
		t.Errorf("quakeContentSize cols = %d, want %d", cols, m.width)
	}
	expectedRows := m.quakeFullHeight() - 1
	if rows != expectedRows {
		t.Errorf("quakeContentSize rows = %d, want %d", rows, expectedRows)
	}
}

func TestCSToggleQuakeTerminalShowHide(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = false
	m.quakeHeightPct = 40

	// Show quake
	cmd := m.toggleQuakeTerminal()
	if m.quakeTerminal == nil {
		t.Skip("could not create quake terminal (PTY limit)")
	}
	defer func() {
		if m.quakeTerminal != nil {
			m.quakeTerminal.Close()
		}
	}()
	_ = cmd
	if !m.quakeVisible {
		t.Error("expected quakeVisible=true after toggle")
	}
	if m.inputMode != ModeTerminal {
		t.Error("expected ModeTerminal after showing quake")
	}
	fullH := float64(m.quakeFullHeight())
	if m.quakeAnimH != fullH {
		t.Errorf("quakeAnimH = %f, want %f (no animation)", m.quakeAnimH, fullH)
	}

	// Hide quake
	cmd = m.toggleQuakeTerminal()
	_ = cmd
	if m.quakeVisible {
		t.Error("expected quakeVisible=false after second toggle")
	}
	if m.quakeAnimH != 0 {
		t.Errorf("quakeAnimH = %f, want 0 (no animation)", m.quakeAnimH)
	}
}

func TestCSToggleQuakeWithAnimation(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.quakeHeightPct = 40

	cmd := m.toggleQuakeTerminal()
	if m.quakeTerminal == nil {
		t.Skip("could not create quake terminal (PTY limit)")
	}
	defer func() {
		if m.quakeTerminal != nil {
			m.quakeTerminal.Close()
		}
	}()
	if cmd == nil {
		t.Error("expected animation tick cmd when animations on")
	}

	// Hide with animation
	cmd = m.toggleQuakeTerminal()
	if cmd == nil {
		t.Error("expected animation tick cmd on hide")
	}
}

func TestCSResizeQuake(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = false
	m.quakeHeightPct = 40

	m.toggleQuakeTerminal()
	if m.quakeTerminal == nil {
		t.Skip("could not create quake terminal (PTY limit)")
	}
	defer m.quakeTerminal.Close()

	origH := m.quakeAnimH

	// Grow
	m.resizeQuake(5)
	if m.quakeAnimH != origH+5 {
		t.Errorf("after grow: quakeAnimH = %f, want %f", m.quakeAnimH, origH+5)
	}

	// Shrink below minimum
	m.resizeQuake(-1000)
	if m.quakeAnimH < 5 {
		t.Errorf("quakeAnimH should not go below 5, got %f", m.quakeAnimH)
	}

	// Grow above maximum
	m.resizeQuake(10000)
	maxH := m.height * 9 / 10
	if int(m.quakeAnimH) > maxH {
		t.Errorf("quakeAnimH should not exceed %d, got %f", maxH, m.quakeAnimH)
	}
}

func TestCSCloseQuakeTerminal(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = false

	m.toggleQuakeTerminal()
	if m.quakeTerminal == nil {
		t.Skip("could not create quake terminal (PTY limit)")
	}

	m.closeQuakeTerminal()
	if m.quakeTerminal != nil {
		t.Error("expected quakeTerminal to be nil after close")
	}
	if m.quakeVisible {
		t.Error("expected quakeVisible=false after close")
	}
	if m.terminals[quakeTermID] != nil {
		t.Error("expected quake entry removed from terminals map")
	}
}

// ══════════════════════════════════════════════
// hasImageBlockingOverlay
// ══════════════════════════════════════════════

func TestCSHasImageBlockingOverlayNone(t *testing.T) {
	m := setupReadyModel()
	if m.hasImageBlockingOverlay() {
		t.Error("expected false with no overlays")
	}
}

func TestCSHasImageBlockingOverlayModal(t *testing.T) {
	m := setupReadyModel()
	m.modal = &ModalOverlay{Title: "test"}
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with modal")
	}
}

func TestCSHasImageBlockingOverlayConfirmClose(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{}
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with confirmClose")
	}
}

func TestCSHasImageBlockingOverlayLauncher(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Visible = true
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with launcher visible")
	}
}

func TestCSHasImageBlockingOverlayExpose(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with expose mode")
	}
}

func TestCSHasImageBlockingOverlaySettings(t *testing.T) {
	m := setupReadyModel()
	m.settings.Visible = true
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with settings visible")
	}
}

func TestCSHasImageBlockingOverlayClipboard(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Visible = true
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with clipboard visible")
	}
}

func TestCSHasImageBlockingOverlayWorkspacePicker(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with workspace picker visible")
	}
}

func TestCSHasImageBlockingOverlayContextMenu(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{Visible: true}
	if !m.hasImageBlockingOverlay() {
		t.Error("expected true with context menu")
	}
}

// ══════════════════════════════════════════════
// getTooltipAt
// ══════════════════════════════════════════════

func TestCSGetTooltipAtDockRow(t *testing.T) {
	m := setupReadyModel()
	// Bottom row is dock
	tip := m.getTooltipAt(5, m.height-1)
	// May or may not find a dock item — just ensure no panic
	_ = tip
}

func TestCSGetTooltipAtMenuBar(t *testing.T) {
	m := setupReadyModel()
	tip := m.getTooltipAt(0, 0)
	// Menu bar row — should return something or empty
	_ = tip
}

func TestCSGetTooltipAtWindowTitleBar(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("tooltip-test", "My Window", geometry.Rect{X: 10, Y: 5, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	// Click on title bar area
	tip := m.getTooltipAt(15, 5)
	if tip != "My Window" {
		t.Errorf("expected tooltip 'My Window', got %q", tip)
	}
}

func TestCSGetTooltipAtTitleBarButtons(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("tooltip-btn", "Win", geometry.Rect{X: 10, Y: 5, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	// Test various title bar button positions — just verify we get known tooltip strings
	pos := win.CloseButtonPos()
	tip := m.getTooltipAt(pos.X, pos.Y)
	validTips := map[string]bool{"Close window": true, "Maximize": true, "Minimize": true, "Win": true}
	if tip != "" && !validTips[tip] {
		t.Errorf("unexpected tooltip %q at close button pos", tip)
	}
}

func TestCSGetTooltipAtEmpty(t *testing.T) {
	m := setupReadyModel()
	// Middle of empty desktop
	tip := m.getTooltipAt(60, 20)
	if tip != "" {
		t.Errorf("expected empty tooltip for empty desktop, got %q", tip)
	}
}

// ══════════════════════════════════════════════
// closeAllTerminals
// ══════════════════════════════════════════════

func TestCSCloseAllTerminals(t *testing.T) {
	m := setupReadyModel()

	// Create a couple terminals
	win1 := window.NewWindow("close-all-1", "T1", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win1)
	cr := win1.ContentRect()
	term1, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	m.terminals[win1.ID] = term1

	win2 := window.NewWindow("close-all-2", "T2", geometry.Rect{X: 40, Y: 1, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win2)
	term2, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		term1.Close()
		t.Skip("could not create second terminal")
	}
	m.terminals[win2.ID] = term2

	m.closeAllTerminals()

	if len(m.terminals) != 0 {
		t.Errorf("expected 0 terminals after closeAll, got %d", len(m.terminals))
	}
}

// ══════════════════════════════════════════════
// resizeTerminalForWindow
// ══════════════════════════════════════════════

func TestCSResizeTerminalForWindow(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("resize-tw", "Term", geometry.Rect{X: 0, Y: 1, Width: 60, Height: 20}, nil)
	m.wm.AddWindow(win)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	defer term.Close()
	m.terminals[win.ID] = term

	// Change window size and call resize
	win.Rect.Width = 80
	win.Rect.Height = 25
	m.resizeTerminalForWindow(win)

	newCR := win.ContentRect()
	if term.Width() != newCR.Width || term.Height() != newCR.Height {
		t.Errorf("terminal not resized: got %dx%d, want %dx%d",
			term.Width(), term.Height(), newCR.Width, newCR.Height)
	}
}

func TestCSResizeTerminalForWindowNoTerminal(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("resize-no-term", "Term", geometry.Rect{X: 0, Y: 1, Width: 60, Height: 20}, nil)
	m.wm.AddWindow(win)
	// No terminal registered — should not panic
	m.resizeTerminalForWindow(win)
}

// ══════════════════════════════════════════════
// enterMenuBarFocus
// ══════════════════════════════════════════════

func TestCSEnterMenuBarFocusNormal(t *testing.T) {
	m := setupReadyModel()
	m.enterMenuBarFocus(1)
	if !m.menuBarFocused {
		t.Error("expected menuBarFocused=true")
	}
	if m.dockFocused {
		t.Error("expected dockFocused=false")
	}
	if m.menuBarFocusIdx != 1 {
		t.Errorf("expected focusIdx=1, got %d", m.menuBarFocusIdx)
	}
}

func TestCSEnterMenuBarFocusNegativeIdx(t *testing.T) {
	m := setupReadyModel()
	m.enterMenuBarFocus(-5)
	if m.menuBarFocusIdx != 0 {
		t.Errorf("expected focusIdx clamped to 0, got %d", m.menuBarFocusIdx)
	}
}

func TestCSEnterMenuBarFocusOverflow(t *testing.T) {
	m := setupReadyModel()
	m.enterMenuBarFocus(999)
	expected := len(m.menuBar.Menus) - 1
	if m.menuBarFocusIdx != expected {
		t.Errorf("expected focusIdx clamped to %d, got %d", expected, m.menuBarFocusIdx)
	}
}

// ══════════════════════════════════════════════
// showKeyboardTooltip
// ══════════════════════════════════════════════

func TestCSShowKeyboardTooltipWithWindow(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("kbtooltip", "My Term", geometry.Rect{X: 10, Y: 5, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	m.showKeyboardTooltip()
	if m.tooltipText != "My Term" {
		t.Errorf("expected tooltip 'My Term', got %q", m.tooltipText)
	}
}

func TestCSShowKeyboardTooltipNoWindow(t *testing.T) {
	m := setupReadyModel()
	m.showKeyboardTooltip()
	if m.tooltipText != "Press N for new terminal" {
		t.Errorf("expected default tooltip, got %q", m.tooltipText)
	}
}

// ══════════════════════════════════════════════
// currentTilingSlotByRect
// ══════════════════════════════════════════════

func TestCSCurrentTilingSlotByRectColumns(t *testing.T) {
	m := setupReadyModel()
	m.tilingLayout = "columns"

	w1 := window.NewWindow("tile-1", "A", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 20}, nil)
	w2 := window.NewWindow("tile-2", "B", geometry.Rect{X: 40, Y: 1, Width: 40, Height: 20}, nil)
	m.wm.AddWindow(w1)
	m.wm.AddWindow(w2)

	slot := m.currentTilingSlotByRect("tile-1")
	if slot != 0 {
		t.Errorf("expected slot 0 for left tile, got %d", slot)
	}
	slot = m.currentTilingSlotByRect("tile-2")
	if slot != 1 {
		t.Errorf("expected slot 1 for right tile, got %d", slot)
	}
}

func TestCSCurrentTilingSlotByRectRows(t *testing.T) {
	m := setupReadyModel()
	m.tilingLayout = "rows"

	w1 := window.NewWindow("row-1", "A", geometry.Rect{X: 0, Y: 1, Width: 80, Height: 10}, nil)
	w2 := window.NewWindow("row-2", "B", geometry.Rect{X: 0, Y: 11, Width: 80, Height: 10}, nil)
	m.wm.AddWindow(w1)
	m.wm.AddWindow(w2)

	slot := m.currentTilingSlotByRect("row-1")
	if slot != 0 {
		t.Errorf("expected slot 0 for top row, got %d", slot)
	}
	slot = m.currentTilingSlotByRect("row-2")
	if slot != 1 {
		t.Errorf("expected slot 1 for bottom row, got %d", slot)
	}
}

func TestCSCurrentTilingSlotByRectNotFound(t *testing.T) {
	m := setupReadyModel()
	slot := m.currentTilingSlotByRect("nonexistent")
	if slot != -1 {
		t.Errorf("expected -1 for nonexistent window, got %d", slot)
	}
}

func TestCSCurrentTilingSlotByRectSkipsMinimized(t *testing.T) {
	m := setupReadyModel()
	m.tilingLayout = "columns"

	w1 := window.NewWindow("vis-1", "A", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 20}, nil)
	w2 := window.NewWindow("min-1", "B", geometry.Rect{X: 40, Y: 1, Width: 40, Height: 20}, nil)
	w2.Minimized = true
	m.wm.AddWindow(w1)
	m.wm.AddWindow(w2)

	slot := m.currentTilingSlotByRect("min-1")
	if slot != -1 {
		t.Errorf("expected -1 for minimized window, got %d", slot)
	}
}

// ══════════════════════════════════════════════
// launcherBounds and workspacePickerBounds
// ══════════════════════════════════════════════

func TestCSLauncherBounds(t *testing.T) {
	m := setupReadyModel()
	x, y, w, h := m.launcherBounds()
	if x < 0 || y < 0 {
		t.Errorf("negative bounds: x=%d, y=%d", x, y)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("zero/negative size: w=%d, h=%d", w, h)
	}
}

func TestCSLauncherBoundsNarrowScreen(t *testing.T) {
	m := setupReadyModel()
	m.width = 30
	x, y, w, h := m.launcherBounds()
	if x < 0 {
		t.Errorf("negative x: %d", x)
	}
	if w > m.width {
		t.Errorf("launcher wider than screen: %d > %d", w, m.width)
	}
	_ = y
	_ = h
}

func TestCSWorkspacePickerBounds(t *testing.T) {
	m := setupReadyModel()
	m.workspaceList = []string{"/path/a", "/path/b", "/path/c"}
	x, y, w, h := m.workspacePickerBounds()
	if x < 0 || y < 0 {
		t.Errorf("negative bounds: x=%d, y=%d", x, y)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("zero/negative size: w=%d, h=%d", w, h)
	}
}

func TestCSWorkspacePickerBoundsNarrow(t *testing.T) {
	m := setupReadyModel()
	m.width = 30
	m.workspaceList = []string{"/path/a"}
	x, _, w, _ := m.workspacePickerBounds()
	if x < 0 {
		t.Errorf("negative x: %d", x)
	}
	_ = w
}

// ══════════════════════════════════════════════
// launcherResultIdx
// ══════════════════════════════════════════════

func TestCSLauncherResultIdxValid(t *testing.T) {
	m := setupReadyModel()
	// Set up launcher with some results
	m.launcher.Results = make([]launcher.AppEntry, 5)

	boundsY := 10
	// First result should be at boundsY + 7 (border + 6 header)
	idx := m.launcherResultIdx(boundsY+7, boundsY)
	if idx != 0 {
		t.Errorf("expected result idx 0, got %d", idx)
	}

	idx = m.launcherResultIdx(boundsY+9, boundsY)
	if idx != 2 {
		t.Errorf("expected result idx 2, got %d", idx)
	}
}

func TestCSLauncherResultIdxOutOfRange(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Results = make([]launcher.AppEntry, 3)

	idx := m.launcherResultIdx(5, 10) // above launcher
	if idx != -1 {
		t.Errorf("expected -1 for out of range, got %d", idx)
	}
}

// ══════════════════════════════════════════════
// handleMenuBarRightClick various branches
// ══════════════════════════════════════════════

func TestCSHandleMenuBarRightClickClock(t *testing.T) {
	m := setupReadyModel()
	result, _ := m.handleMenuBarRightClick("clock")
	rm := result.(Model)
	if rm.modal == nil {
		t.Error("expected modal overlay for clock right-click")
	}
	if rm.modal.Title != "Date & Time" {
		t.Errorf("expected 'Date & Time' modal, got %q", rm.modal.Title)
	}
}

func TestCSHandleMenuBarRightClickNotification(t *testing.T) {
	m := setupReadyModel()
	m.handleMenuBarRightClick("notification")
	// Just verify no panic — notification center toggles
}

func TestCSHandleMenuBarRightClickWorkspace(t *testing.T) {
	m := setupReadyModel()
	result, _ := m.handleMenuBarRightClick("workspace")
	_ = result
	// Should toggle workspace picker — no panic
}

func TestCSHandleMenuBarRightClickUnknown(t *testing.T) {
	m := setupReadyModel()
	result, _ := m.handleMenuBarRightClick("unknown_zone")
	_ = result
	// Should handle gracefully
}

// ══════════════════════════════════════════════
// ensureCursorVisible edge cases
// ══════════════════════════════════════════════

func TestCSEnsureCursorVisibleAboveViewport(t *testing.T) {
	m := setupReadyModel()
	m.copyCursorY = 5
	m.scrollOffset = 0

	// sbLen=100, contentH=20, maxScroll=80
	m.ensureCursorVisible(100, 20, 80)
	// Cursor at line 5 in scrollback, offset should be set to bring it into view
	if m.scrollOffset < 0 || m.scrollOffset > 80 {
		t.Errorf("scrollOffset out of range: %d", m.scrollOffset)
	}
}

func TestCSEnsureCursorVisibleBelowViewport(t *testing.T) {
	m := setupReadyModel()
	m.copyCursorY = 150
	m.scrollOffset = 80

	m.ensureCursorVisible(100, 20, 80)
	if m.scrollOffset < 0 || m.scrollOffset > 80 {
		t.Errorf("scrollOffset out of range: %d", m.scrollOffset)
	}
}

func TestCSEnsureCursorVisibleInView(t *testing.T) {
	m := setupReadyModel()
	m.copyCursorY = 90
	m.scrollOffset = 5

	m.ensureCursorVisible(100, 20, 80)
	// Cursor should be visible, offset should be reasonable
	if m.scrollOffset < 0 || m.scrollOffset > 80 {
		t.Errorf("scrollOffset out of range: %d", m.scrollOffset)
	}
}

// ══════════════════════════════════════════════
// launchCommandLine
// ══════════════════════════════════════════════

func TestCSLaunchCommandLineEmpty(t *testing.T) {
	m := setupReadyModel()
	result, cmd := m.launchCommandLine("")
	rm := result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for empty command line")
	}
	if rm.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal, got %d", rm.inputMode)
	}
}

func TestCSLaunchCommandLineNotFound(t *testing.T) {
	m := setupReadyModel()
	result, cmd := m.launchCommandLine("__nonexistent_command_xyz123__")
	rm := result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for nonexistent command")
	}
	// Should have pushed a warning notification
	_ = rm
}

// ══════════════════════════════════════════════
// launchExecEntry
// ══════════════════════════════════════════════

func TestCSLaunchExecEntryNotFound(t *testing.T) {
	m := setupReadyModel()
	result, cmd := m.launchExecEntry("__nonexistent_exec_xyz123__", nil)
	rm := result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for nonexistent command")
	}
	_ = rm
}

// ══════════════════════════════════════════════
// signalAppsForCapture
// ══════════════════════════════════════════════

func TestCSSignalAppsForCaptureNoTerminals(t *testing.T) {
	m := setupReadyModel()
	// No terminals — should not panic
	m.signalAppsForCapture()
}

func TestCSSignalAppsForCaptureWithTerminal(t *testing.T) {
	m := setupReadyModel()
	win := window.NewWindow("signal-test", "T", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 15}, nil)
	m.wm.AddWindow(win)
	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	defer term.Close()
	m.terminals[win.ID] = term
	m.termCreatedAt[win.ID] = time.Now()

	// Should not panic
	m.signalAppsForCapture()
}

// ══════════════════════════════════════════════
// normalizeTilingLayout
// ══════════════════════════════════════════════

func TestCSNormalizeTilingLayout(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"columns", "columns"},
		{"Columns", "columns"},
		{" COLUMNS ", "columns"},
		{"rows", "rows"},
		{"Rows", "rows"},
		{"all", "all"},
		{"tile_all", "all"},
		{"grid", "all"},
		{"", "columns"},
		{"unknown", "columns"},
	}
	for _, tt := range tests {
		got := normalizeTilingLayout(tt.input)
		if got != tt.want {
			t.Errorf("normalizeTilingLayout(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ══════════════════════════════════════════════
// loadSelectedWorkspace
// ══════════════════════════════════════════════

func TestCSLoadSelectedWorkspaceOutOfRange(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerSelected = -1
	cmd := m.loadSelectedWorkspace()
	if cmd != nil {
		t.Error("expected nil for out-of-range selection")
	}

	m.workspacePickerSelected = 100
	cmd = m.loadSelectedWorkspace()
	if cmd != nil {
		t.Error("expected nil for out-of-range selection")
	}
}
