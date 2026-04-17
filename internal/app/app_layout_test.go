package app

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func TestUpdateDockRunningSetsFocusedWindowID(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	m.updateDockRunning()

	if m.dock.FocusedWindowID != fw.ID {
		t.Errorf("FocusedWindowID = %q, want %q", m.dock.FocusedWindowID, fw.ID)
	}
}

func TestUpdateDockRunningNoFocusedWindow(t *testing.T) {
	m := setupReadyModel()

	m.updateDockRunning()

	if m.dock.FocusedWindowID != "" {
		t.Errorf("FocusedWindowID = %q, want empty (no windows)", m.dock.FocusedWindowID)
	}
}

func TestUpdateDockRunningMinimizedItemsAppear(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Minimize first window
	windows := m.wm.Windows()
	windows[0].Minimized = true

	m.updateDockRunning()

	hasMinimized := false
	for _, item := range m.dock.Items {
		if item.Special == "minimized" && item.WindowID == windows[0].ID {
			hasMinimized = true
			break
		}
	}
	if !hasMinimized {
		t.Error("minimized window should appear in dock items")
	}
}

func TestUpdateDockRunningAlwaysShown(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	m.updateDockRunning()

	// Running window should be associated with a dock shortcut (WindowID set on a Special="" item)
	// OR appear as a separate running item for unmatched commands.
	found := false
	for _, item := range m.dock.Items {
		if item.WindowID == fw.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("running window should always appear in dock (via shortcut or separate entry)")
	}
}

func TestUpdateDockMinimizedNotShownWhenDisabled(t *testing.T) {
	m := setupReadyModel()
	m.dock.MinimizeToDock = false
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	fw.Minimized = true

	m.updateDockRunning()

	for _, item := range m.dock.Items {
		if item.Special == "minimized" {
			t.Error("minimized items should not appear when MinimizeToDock is false")
			break
		}
	}
}

func TestHideDockAppsToggleRestoresItems(t *testing.T) {
	m := setupReadyModel()

	// Count initial app items (Special == "")
	initialAppCount := 0
	for _, item := range m.dock.Items {
		if item.Special == "" {
			initialAppCount++
		}
	}
	if initialAppCount == 0 {
		t.Fatal("expected some app items in dock initially")
	}

	// Hide app shortcuts
	m.hideDockApps = true
	m.updateDockRunning()

	hiddenAppCount := 0
	for _, item := range m.dock.Items {
		if item.Special == "" {
			hiddenAppCount++
		}
	}
	if hiddenAppCount != 0 {
		t.Errorf("expected 0 app items when hidden, got %d", hiddenAppCount)
	}

	// Toggle back off — items must reappear
	m.hideDockApps = false
	m.updateDockRunning()

	restoredAppCount := 0
	for _, item := range m.dock.Items {
		if item.Special == "" {
			restoredAppCount++
		}
	}
	if restoredAppCount != initialAppCount {
		t.Errorf("expected %d app items after restore, got %d", initialAppCount, restoredAppCount)
	}
}

func TestUpdateDockRunningDynItemsAfterExpose(t *testing.T) {
	m := setupReadyModel()
	m.dock.MinimizeToDock = true

	// Use a unique command so the window does NOT match any dock shortcut,
	// forcing a separate dynamic "running" entry.
	id := "win-unique"
	win := window.NewWindow(id, "UniqueApp",
		geometry.Rect{X: 5, Y: 5, Width: 60, Height: 15}, nil)
	win.Command = "unique-test-cmd-xyz"
	m.wm.AddWindow(win)

	m.updateDockRunning()

	// Find expose index and first dynamic item index
	exposeIdx := -1
	firstDynIdx := -1
	for i, item := range m.dock.Items {
		if item.Special == "expose" {
			exposeIdx = i
		}
		if (item.Special == "running" || item.Special == "minimized") && firstDynIdx == -1 {
			firstDynIdx = i
		}
	}
	if exposeIdx == -1 {
		t.Fatal("expose item not found")
	}
	if firstDynIdx == -1 {
		t.Fatal("dynamic item not found")
	}
	if firstDynIdx <= exposeIdx {
		t.Errorf("dynamic items (idx %d) should appear after expose (idx %d)", firstDynIdx, exposeIdx)
	}
}

func TestUpdateDockRunningLabelTruncation(t *testing.T) {
	m := setupReadyModel()
	m.dock.MinimizeToDock = true

	// Create a window with a very long title
	id := "win-long"
	win := window.NewWindow(id, "This Is A Very Long Window Title That Should Be Truncated",
		geometry.Rect{X: 0, Y: 1, Width: 80, Height: 24}, nil)
	m.wm.AddWindow(win)

	m.updateDockRunning()

	for _, item := range m.dock.Items {
		if item.WindowID == id {
			labelRunes := []rune(item.Label)
			// Progressive truncation: first level is 12 chars max (then 8, 4, 1)
			if len(labelRunes) > 12 {
				t.Errorf("label should be truncated to 12 chars max, got %d: %q", len(labelRunes), item.Label)
			}
			return
		}
	}
	t.Error("expected to find dock item for long-titled window")
}

func TestUpdateDockRunningPreservesOrder(t *testing.T) {
	m := setupReadyModel()
	m.dock.MinimizeToDock = true
	m.openDemoWindow()
	m.openDemoWindow()

	m.updateDockRunning()

	// Record order of dynamic items
	var firstOrder []string
	for _, item := range m.dock.Items {
		if item.Special == "running" || item.Special == "minimized" {
			firstOrder = append(firstOrder, item.WindowID)
		}
	}

	// Run again -- order should be preserved
	m.updateDockRunning()

	var secondOrder []string
	for _, item := range m.dock.Items {
		if item.Special == "running" || item.Special == "minimized" {
			secondOrder = append(secondOrder, item.WindowID)
		}
	}

	if len(firstOrder) != len(secondOrder) {
		t.Fatalf("order length changed: %d -> %d", len(firstOrder), len(secondOrder))
	}
	for i := range firstOrder {
		if firstOrder[i] != secondOrder[i] {
			t.Errorf("order changed at %d: %s -> %s", i, firstOrder[i], secondOrder[i])
		}
	}
}

func TestMinimizedDockLabel(t *testing.T) {
	tests := []struct {
		command   string
		wantIcon  string
		wantLabel string
	}{
		{"", "\uf120", "T"},
		{"$SHELL", "\uf120", "T"},
		{"nvim", "\ue62b", "V"},
		{"vim", "\ue62b", "V"},
		{"htop", "\uf200", "M"},
		{"mc", "\uf07b", "F"},
		{"unknowncmd", "\uf2d0", "U"},
	}
	for _, tt := range tests {
		w := &window.Window{Command: tt.command}
		icon, label := minimizedDockLabel(w)
		if icon != tt.wantIcon {
			t.Errorf("minimizedDockLabel(%q) icon = %q, want %q", tt.command, icon, tt.wantIcon)
		}
		if label != tt.wantLabel {
			t.Errorf("minimizedDockLabel(%q) label = %q, want %q", tt.command, label, tt.wantLabel)
		}
	}
}

func TestRenderDockActiveStyling(t *testing.T) {
	m := setupReadyModel()
	m.dock.MinimizeToDock = true
	m.openDemoWindow()
	m.updateDockRunning()

	buf := AcquireThemedBuffer(120, 40, m.theme)
	RenderDock(buf, m.dock, m.theme, nil)

	// The dock is at the bottom row (y=39)
	y := buf.Height - 1
	hasBold := false
	for x := 0; x < buf.Width; x++ {
		if buf.Cells[y][x].Attrs&AttrBold != 0 {
			hasBold = true
			break
		}
	}
	if !hasBold {
		t.Error("expected bold cells in dock for active window")
	}
}

func TestUpdateDockRunningExitedWindowShowsStuck(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Exited = true

	m.updateDockRunning()

	found := false
	for _, item := range m.dock.Items {
		if item.WindowID == w.ID {
			found = true
			if !item.Stuck {
				t.Error("exited window should show as Stuck in dock (red indicator)")
			}
		}
	}
	if !found {
		t.Error("expected dock item for exited window")
	}
}

func TestRenderDockWithoutWindows(t *testing.T) {
	m := setupReadyModel()
	m.updateDockRunning()

	buf := AcquireThemedBuffer(120, 40, m.theme)
	RenderDock(buf, m.dock, m.theme, nil)

	// Should not panic and bottom row should have dock content
	y := buf.Height - 1
	hasContent := false
	for x := 0; x < buf.Width; x++ {
		if buf.Cells[y][x].Char != ' ' && buf.Cells[y][x].Char != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected dock content even without windows")
	}
}

func TestRenderDockNilDock(t *testing.T) {
	m := setupReadyModel()
	buf := AcquireThemedBuffer(120, 40, m.theme)
	// RenderDock with nil dock should not panic
	RenderDock(buf, nil, m.theme, nil)
}

// --- Layout function tests ---

func TestTileColumns(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	m.animateTileColumns()
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	windows := m.wm.Windows()
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	// Sort by ID to match tiling's deterministic ordering.
	sort.Slice(windows, func(i, j int) bool { return windows[i].ID < windows[j].ID })

	expectedColW := wa.Width / 2
	for i, w := range windows {
		if w.Rect.Height != wa.Height {
			t.Errorf("window %d: height = %d, want %d", i, w.Rect.Height, wa.Height)
		}
		if w.Rect.Y != wa.Y {
			t.Errorf("window %d: Y = %d, want %d", i, w.Rect.Y, wa.Y)
		}
		if w.Rect.Width != expectedColW && i < len(windows)-1 {
			t.Errorf("window %d: width = %d, want %d", i, w.Rect.Width, expectedColW)
		}
	}
	if windows[0].Rect.X != wa.X {
		t.Errorf("first window X = %d, want %d", windows[0].Rect.X, wa.X)
	}
	if windows[1].Rect.X != wa.X+expectedColW {
		t.Errorf("second window X = %d, want %d", windows[1].Rect.X, wa.X+expectedColW)
	}
}

func TestTileColumnsNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.animateTileColumns()
	m = completeAnimations(m)
}

func TestTileRows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	m.animateTileRows()
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	windows := m.wm.Windows()
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	// Sort by ID to match tiling's deterministic ordering.
	sort.Slice(windows, func(i, j int) bool { return windows[i].ID < windows[j].ID })

	expectedRowH := wa.Height / 2
	for i, w := range windows {
		if w.Rect.Width != wa.Width {
			t.Errorf("window %d: width = %d, want %d", i, w.Rect.Width, wa.Width)
		}
		if w.Rect.X != wa.X {
			t.Errorf("window %d: X = %d, want %d", i, w.Rect.X, wa.X)
		}
		if w.Rect.Height != expectedRowH && i < len(windows)-1 {
			t.Errorf("window %d: height = %d, want %d", i, w.Rect.Height, expectedRowH)
		}
	}
	if windows[0].Rect.Y != wa.Y {
		t.Errorf("first window Y = %d, want %d", windows[0].Rect.Y, wa.Y)
	}
	if windows[1].Rect.Y != wa.Y+expectedRowH {
		t.Errorf("second window Y = %d, want %d", windows[1].Rect.Y, wa.Y+expectedRowH)
	}
}

func TestTileRowsNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.animateTileRows()
	m = completeAnimations(m)
}

func TestCascade(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()

	m.animateCascade()
	m = completeAnimations(m)

	windows := m.wm.Windows()
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	// Sort by ID to match tiling's deterministic ordering.
	sort.Slice(windows, func(i, j int) bool { return windows[i].ID < windows[j].ID })

	w0 := windows[0].Rect
	for i := 1; i < len(windows); i++ {
		if windows[i].Rect.Width != w0.Width {
			t.Errorf("window %d: width = %d, want %d (same as window 0)", i, windows[i].Rect.Width, w0.Width)
		}
		if windows[i].Rect.Height != w0.Height {
			t.Errorf("window %d: height = %d, want %d (same as window 0)", i, windows[i].Rect.Height, w0.Height)
		}
	}

	for i := 1; i < len(windows); i++ {
		prev := windows[i-1].Rect
		curr := windows[i].Rect
		if curr.X <= prev.X && curr.Y <= prev.Y {
			t.Errorf("window %d should be offset from window %d: (%d,%d) vs (%d,%d)",
				i, i-1, curr.X, curr.Y, prev.X, prev.Y)
		}
	}
}

func TestCascadeNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.animateCascade()
	m = completeAnimations(m)
}

func TestTileMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	m.animateTileMaximized()
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	windows := m.wm.Windows()
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}

	for i, w := range windows {
		if w.Rect != wa {
			t.Errorf("window %d: rect = %v, want work area %v", i, w.Rect, wa)
		}
		if !w.IsMaximized() {
			t.Errorf("window %d: expected to be maximized", i)
		}
	}
}

func TestTileMaximizedNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.animateTileMaximized()
	m = completeAnimations(m)
}

func TestShowDesktop(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	for _, w := range m.wm.Windows() {
		if w.Minimized {
			t.Fatal("window should not be minimized initially")
		}
	}

	m.showDesktop()

	for i, w := range m.wm.Windows() {
		if !w.Minimized {
			t.Errorf("window %d: expected Minimized=true after showDesktop", i)
		}
	}
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal (%d)", m.inputMode, ModeNormal)
	}
}

func TestShowDesktopNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.showDesktop()
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal (%d)", m.inputMode, ModeNormal)
	}
}

func TestMaximizeFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	if fw.IsMaximized() {
		t.Fatal("window should not be maximized initially")
	}

	m.maximizeFocusedWindow()
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	if fw.Rect != wa {
		t.Errorf("maximized rect = %v, want work area %v", fw.Rect, wa)
	}
	if !fw.IsMaximized() {
		t.Error("expected IsMaximized()=true after maximizeFocusedWindow")
	}
}

func TestMaximizeFocusedWindowNoWindow(t *testing.T) {
	m := setupReadyModel()
	m.maximizeFocusedWindow()
}

func TestMaximizeFocusedWindowAlreadyMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	window.Maximize(fw, m.wm.WorkArea())
	maxRect := fw.Rect

	m.maximizeFocusedWindow()

	if fw.Rect != maxRect {
		t.Errorf("rect changed after maximizing already-maximized window: %v -> %v", maxRect, fw.Rect)
	}
}

func TestMinimizeWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	if fw.Minimized {
		t.Fatal("window should not be minimized initially")
	}

	m.minimizeWindow(fw)

	if !fw.Minimized {
		t.Error("expected Minimized=true after minimizeWindow")
	}
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal (%d)", m.inputMode, ModeNormal)
	}
}

func TestMinimizeWindowAlreadyMinimized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	fw.Minimized = true
	m.minimizeWindow(fw)

	if !fw.Minimized {
		t.Error("window should still be minimized")
	}
}

func TestMinimizeWindowFocusesNext(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	windows := m.wm.Windows()
	if len(windows) < 2 {
		t.Fatal("expected at least 2 windows")
	}

	lastWin := m.wm.FocusedWindow()
	if lastWin == nil {
		t.Fatal("expected focused window")
	}

	m.minimizeWindow(lastWin)
	m = completeAnimations(m)

	newFocus := m.wm.FocusedWindow()
	if newFocus != nil && newFocus.ID == lastWin.ID {
		t.Error("focus should have moved away from minimized window")
	}
}

func TestRestoreMinimizedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	origRect := fw.Rect

	m.minimizeWindow(fw)
	if !fw.Minimized {
		t.Fatal("expected window to be minimized")
	}

	m.restoreMinimizedWindow(fw)
	m = completeAnimations(m)

	if fw.Minimized {
		t.Error("expected Minimized=false after restoreMinimizedWindow")
	}
	if fw.Rect != origRect {
		t.Errorf("restored rect = %v, want original %v", fw.Rect, origRect)
	}
	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal (%d)", m.inputMode, ModeNormal)
	}
}

func TestRestoreMinimizedWindowInTilingModeUsesManualTilingFlow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()
	m = completeAnimations(m)

	var toRestore *window.Window
	for _, w := range m.wm.Windows() {
		if !w.Minimized && w.Visible {
			toRestore = w
			break
		}
	}
	if toRestore == nil {
		t.Fatal("expected visible window to minimize/restore")
	}

	m.minimizeWindow(toRestore)
	m = completeAnimations(m)
	if !toRestore.Minimized {
		t.Fatal("expected window to be minimized")
	}

	m.restoreMinimizedWindow(toRestore)
	m = completeAnimations(m)

	if toRestore.Minimized {
		t.Fatal("expected restored window to be visible")
	}
	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("window %s width=%d want=%d after tiled restore", w.ID, w.Rect.Width, wa.Width)
		}
	}
}

func TestRestoreFirstMinimizedTileReturnsToFirstSlot(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "columns"
	m.applyTilingLayout()
	m = completeAnimations(m)

	wins := m.wm.Windows()
	if len(wins) < 3 {
		t.Fatal("expected at least 3 windows")
	}
	first := wins[0]
	expectedFirstRect := first.Rect

	m.minimizeWindow(first)
	m = completeAnimations(m)
	if !first.Minimized {
		t.Fatal("expected first window minimized")
	}

	m.restoreMinimizedWindow(first)
	m = completeAnimations(m)
	if first.Minimized {
		t.Fatal("expected first window restored")
	}
	if first.Rect != expectedFirstRect {
		t.Fatalf("first window rect=%v want original first slot=%v", first.Rect, expectedFirstRect)
	}
}

func TestRestoreFirstTileAfterFocusRaiseKeepsOriginalSlot(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()
	m = completeAnimations(m)

	// Pick the current first visual tile by topmost Y.
	var first *window.Window
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if first == nil || w.Rect.Y < first.Rect.Y {
			first = w
		}
	}
	if first == nil {
		t.Fatal("expected first tile")
	}
	expected := first.Rect

	// Simulate click focus raise before minimize (real UI path).
	m.wm.FocusWindow(first.ID)
	m.minimizeWindow(first)
	m = completeAnimations(m)
	if !first.Minimized {
		t.Fatal("expected first tile minimized")
	}

	m.restoreMinimizedWindow(first)
	m = completeAnimations(m)
	if first.Minimized {
		t.Fatal("expected first tile restored")
	}
	if first.Rect != expected {
		t.Fatalf("restored rect=%v want original first-slot rect=%v", first.Rect, expected)
	}
}

func TestRestoreMinimizedWindowNotMinimized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	origRect := fw.Rect
	m.restoreMinimizedWindow(fw)

	if fw.Minimized {
		t.Error("window should not become minimized from restore")
	}
	if fw.Rect != origRect {
		t.Errorf("rect changed unexpectedly: %v -> %v", origRect, fw.Rect)
	}
}

// --- executeAction tests ---

func TestExecuteActionTileColumns(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, cmd := m.executeAction("tile_columns", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Rect.Height != wa.Height {
			t.Errorf("window height = %d, want %d", w.Rect.Height, wa.Height)
		}
	}
}

func TestExecuteActionTileRows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, cmd := m.executeAction("tile_rows", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Rect.Width != wa.Width {
			t.Errorf("window width = %d, want %d", w.Rect.Width, wa.Width)
		}
	}
}

func TestExecuteActionCascade(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true

	result, cmd := m.executeAction("cascade", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by cascade action")
	}
	m = completeAnimations(m)

	windows := m.wm.Windows()
	if len(windows) >= 2 {
		if windows[0].Rect.Width != windows[1].Rect.Width {
			t.Error("cascade windows should have same width")
		}
	}
}

func TestExecuteActionShowDesktop(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, cmd := m.executeAction("show_desktop", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}

	for i, w := range m.wm.Windows() {
		if !w.Minimized {
			t.Errorf("window %d: expected minimized after show_desktop", i)
		}
	}
}

func TestExecuteActionCenterWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, cmd := m.executeAction("center", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw = m.wm.FocusedWindow()
	expectedX := wa.X + (wa.Width-fw.Rect.Width)/2
	expectedY := wa.Y + (wa.Height-fw.Rect.Height)/2
	if fw.Rect.X != expectedX {
		t.Errorf("centered X = %d, want %d", fw.Rect.X, expectedX)
	}
	if fw.Rect.Y != expectedY {
		t.Errorf("centered Y = %d, want %d", fw.Rect.Y, expectedY)
	}
}

func TestExecuteActionExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, cmd := m.executeAction("expose", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	if !m.exposeMode {
		t.Error("expected exposeMode=true after expose action")
	}
}

func TestExecuteActionNewTerminal(t *testing.T) {
	m := setupReadyModel()

	initialCount := len(m.wm.Windows())
	result, _ := m.executeAction("new_terminal", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if len(m.wm.Windows()) <= initialCount {
		t.Error("expected a new window after new_terminal action")
	}
}

func TestExecuteActionCloseWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, _ := m.executeAction("close_window", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if m.confirmClose == nil {
		t.Error("expected confirmClose dialog after close_window action")
	}
}

func TestConfirmCloseRelayoutsWithAutoTilingRows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	wins := m.wm.Windows()
	if len(wins) < 2 {
		t.Fatal("expected at least two windows")
	}
	closeID := wins[0].ID
	m.confirmClose = &ConfirmDialog{WindowID: closeID, Selected: 0}

	result, cmd := m.confirmAccept()
	m = result.(Model)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for close confirmation")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("window %s width = %d, want %d after close relayout", w.ID, w.Rect.Width, wa.Width)
		}
	}
}

func TestExecuteActionQuit(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("quit", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if m.confirmClose == nil || !m.confirmClose.IsQuit {
		t.Error("expected quit confirm dialog after quit action")
	}
}

func TestExecuteActionFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("expose", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if m.inputMode != ModeNormal {
		t.Errorf("inputMode = %d, want ModeNormal (%d) after expose from terminal mode", m.inputMode, ModeNormal)
	}
}

func TestExecuteActionSettings(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("settings", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if !m.settings.Visible {
		t.Error("expected settings panel to be visible after settings action")
	}

	result, _ = m.executeAction("settings", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if m.settings.Visible {
		t.Error("expected settings panel to be hidden after second settings action")
	}
}

func TestExecuteActionTileMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true

	result, cmd := m.executeAction("tile_maximized", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by tile_maximized action")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for i, w := range m.wm.Windows() {
		if w.Rect != wa {
			t.Errorf("window %d: rect = %v, want work area %v", i, w.Rect, wa)
		}
	}
}

func TestExecuteActionMaximizeDisablesPersistentTiling(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.tilingMode = true

	result, cmd := m.executeAction("maximize", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for maximize")
	}
	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by maximize action")
	}
}

func TestExecuteActionMinimize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, cmd := m.executeAction("minimize", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}

	if !fw.Minimized {
		t.Error("expected window to be minimized after minimize action")
	}
}

func TestExecuteActionSnapLeft(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, cmd := m.executeAction("snap_left", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw := m.wm.FocusedWindow()
	if fw.Rect.X != wa.X {
		t.Errorf("snap_left: X = %d, want %d", fw.Rect.X, wa.X)
	}
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap_left: width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}
}

func TestExecuteActionSnapRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, cmd := m.executeAction("snap_right", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw := m.wm.FocusedWindow()
	if fw.Rect.X != wa.X+wa.Width/2 {
		t.Errorf("snap_right: X = %d, want %d", fw.Rect.X, wa.X+wa.Width/2)
	}
}

func TestExecuteActionNextPrevWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	fw1 := m.wm.FocusedWindow()

	result, _ := m.executeAction("next_window", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	fw2 := m.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after next_window")
	}
	if fw2.ID == fw1.ID {
		t.Error("expected focus to change after next_window")
	}

	result, _ = m.executeAction("prev_window", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	fw3 := m.wm.FocusedWindow()
	if fw3 == nil {
		t.Fatal("expected focused window after prev_window")
	}
	if fw3.ID != fw1.ID {
		t.Error("expected focus to return to original window after prev_window")
	}
}

func TestExecuteActionHelp(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("help", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if m.modal == nil {
		t.Error("expected modal overlay after help action")
	}
}

func TestExecuteActionLauncher(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("launcher", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)

	if !m.launcher.Visible {
		t.Error("expected launcher to be visible after launcher action")
	}
}

func TestExecuteActionUnknown(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("nonexistent", tea.KeyPressMsg(tea.Key{}), "")
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for unknown action")
	}
}

// --- executeMenuAction tests ---

func TestExecuteMenuActionNewTerminal(t *testing.T) {
	m := setupReadyModel()

	initialCount := len(m.wm.Windows())
	result, _ := m.executeMenuAction("new_terminal")
	m = result.(Model)

	if len(m.wm.Windows()) <= initialCount {
		t.Error("expected a new window after new_terminal menu action")
	}
}

func TestExecuteMenuActionQuit(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("quit")
	m = result.(Model)

	if m.confirmClose == nil || !m.confirmClose.IsQuit {
		t.Error("expected quit confirm dialog after quit menu action")
	}
}

func TestExecuteMenuActionTileAll(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("tile_all")
	m = result.(Model)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Rect.Y < wa.Y || w.Rect.Y+w.Rect.Height > wa.Y+wa.Height {
			t.Errorf("window %s not within work area Y bounds", w.ID)
		}
	}
}

func TestExecuteMenuActionTileColumns(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("tile_columns")
	m = result.(Model)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Rect.Height != wa.Height {
			t.Errorf("window height = %d, want %d after tile_columns menu action", w.Rect.Height, wa.Height)
		}
	}
}

func TestExecuteMenuActionTileRows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("tile_rows")
	m = result.(Model)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Rect.Width != wa.Width {
			t.Errorf("window width = %d, want %d after tile_rows menu action", w.Rect.Width, wa.Width)
		}
	}
}

func TestTilingModePersistsRowsLayout(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"

	m.applyTilingLayout()

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("window width = %d, want %d in rows layout", w.Rect.Width, wa.Width)
		}
	}
}

func TestTilingModePersistsTileAllLayout(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "all"

	m.applyTilingLayout()

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible {
			continue
		}
		if w.Rect.Right() > wa.Right() || w.Rect.Bottom() > wa.Bottom() {
			t.Fatalf("window %s out of work area after tile-all layout: rect=%v wa=%v", w.ID, w.Rect, wa)
		}
	}
}

func TestCloseAnimationRespectsRowsTiling(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	wins := m.wm.Windows()
	if len(wins) < 2 {
		t.Fatal("expected at least 2 windows")
	}
	closeID := wins[0].ID

	m.finalizeAnimation(&Animation{Type: AnimClose, WindowID: closeID})

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("window width = %d, want %d (rows tiling after close)", w.Rect.Width, wa.Width)
		}
	}
}

func TestCloseAnimationSchedulesRetileTicks(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()
	m.animationsOn = true

	wins := m.wm.Windows()
	if len(wins) < 3 {
		t.Fatal("expected at least 3 windows")
	}
	closeID := wins[0].ID
	closeRect := wins[0].Rect

	// Force close animation to settle on first tick so finalizeAnimation runs
	// during updateAnimations and spawns retile animations.
	m.startWindowAnimation(closeID, AnimClose, closeRect, closeRect)

	updated, cmd := m.Update(AnimationTickMsg{Time: time.Now()})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("expected non-nil tick cmd after close finalized and retile started")
	}
	if m.wm.WindowByID(closeID) != nil {
		t.Fatal("expected closed window to be removed")
	}
	if len(m.animations) == 0 {
		t.Fatal("expected active retile animations after close")
	}
}

func TestExecuteMenuActionCascade(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true

	result, _ := m.executeMenuAction("cascade")
	m = result.(Model)
	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by cascade menu action")
	}

	windows := m.wm.Windows()
	if len(windows) >= 2 {
		if windows[0].Rect.Width != windows[1].Rect.Width {
			t.Error("cascade menu action: windows should have same width")
		}
	}
}

func TestExecuteMenuActionTileMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true

	result, _ := m.executeMenuAction("tile_maximized")
	m = result.(Model)
	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by tile_maximized menu action")
	}

	wa := m.wm.WorkArea()
	for i, w := range m.wm.Windows() {
		if w.Rect != wa {
			t.Errorf("window %d: rect = %v, want work area %v after tile_maximized", i, w.Rect, wa)
		}
	}
}

func TestExecuteMenuActionShowDesktop(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	result, cmd := m.executeMenuAction("show_desktop")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}

	for i, w := range m.wm.Windows() {
		if !w.Minimized {
			t.Errorf("window %d: expected minimized after show_desktop menu action", i)
		}
	}
}

func TestExecuteMenuActionCloseWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("close_window")
	m = result.(Model)

	if m.confirmClose == nil {
		t.Error("expected confirmClose dialog after close_window menu action")
	}
}

func TestExecuteMenuActionMaximize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, _ := m.executeMenuAction("maximize")
	m = result.(Model)

	fw = m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after maximize")
	}
	wa := m.wm.WorkArea()
	if fw.Rect != wa {
		t.Errorf("maximized rect = %v, want work area %v", fw.Rect, wa)
	}
}

func TestExecuteMenuActionMaximizeDisablesPersistentTiling(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.tilingMode = true

	result, _ := m.executeMenuAction("maximize")
	m = result.(Model)

	if m.tilingMode {
		t.Fatal("expected persistent tiling mode to be disabled by maximize menu action")
	}
}

func TestExecuteMenuActionMaximizeToggle(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origRect := fw.Rect

	result, _ := m.executeMenuAction("maximize")
	m = result.(Model)

	result, _ = m.executeMenuAction("maximize")
	m = result.(Model)

	fw = m.wm.FocusedWindow()
	if fw.Rect != origRect {
		t.Errorf("restored rect = %v, want original %v", fw.Rect, origRect)
	}
}

func TestExecuteMenuActionSnapLeftRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	wa := m.wm.WorkArea()

	result, _ := m.executeMenuAction("snap_left")
	m = result.(Model)
	fw := m.wm.FocusedWindow()
	if fw.Rect.X != wa.X {
		t.Errorf("snap_left: X = %d, want %d", fw.Rect.X, wa.X)
	}
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap_left: width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}

	result, _ = m.executeMenuAction("snap_right")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.X != wa.X+wa.Width/2 {
		t.Errorf("snap_right: X = %d, want %d", fw.Rect.X, wa.X+wa.Width/2)
	}
}

func TestExecuteMenuActionCenter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("center")
	m = result.(Model)

	wa := m.wm.WorkArea()
	fw := m.wm.FocusedWindow()
	expectedX := wa.X + (wa.Width-fw.Rect.Width)/2
	expectedY := wa.Y + (wa.Height-fw.Rect.Height)/2
	if fw.Rect.X != expectedX {
		t.Errorf("center menu: X = %d, want %d", fw.Rect.X, expectedX)
	}
	if fw.Rect.Y != expectedY {
		t.Errorf("center menu: Y = %d, want %d", fw.Rect.Y, expectedY)
	}
}

func TestExecuteMenuActionMinimize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, cmd := m.executeMenuAction("minimize")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}

	if !fw.Minimized {
		t.Error("expected window to be minimized after minimize menu action")
	}
}

func TestExecuteMenuActionSettings(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("settings")
	m = result.(Model)

	if !m.settings.Visible {
		t.Error("expected settings panel visible after settings menu action")
	}
}

func TestExecuteMenuActionHelp(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("help_keys")
	m = result.(Model)

	if m.modal == nil {
		t.Error("expected modal overlay after help_keys menu action")
	}
}

func TestExecuteMenuActionAbout(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("about")
	m = result.(Model)

	if m.modal == nil {
		t.Error("expected modal overlay after about menu action")
	}
}

func TestExecuteMenuActionClipboardHistory(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("clipboard_history")
	m = result.(Model)

	if !m.clipboard.Visible {
		t.Error("expected clipboard history visible after clipboard_history menu action")
	}
}

func TestExecuteMenuActionCopyModeAndSearch(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	// Mark this window as terminal-backed for menu actions.
	m.terminals[fw.ID] = nil

	result, _ := m.executeMenuAction("copy_mode")
	m = result.(Model)
	if m.inputMode != ModeCopy {
		t.Fatalf("expected ModeCopy, got %d", m.inputMode)
	}

	result, _ = m.executeMenuAction("copy_search_forward")
	m = result.(Model)
	if !m.copySearchActive || m.copySearchDir != 1 {
		t.Fatalf("expected active forward search, active=%v dir=%d", m.copySearchActive, m.copySearchDir)
	}

	result, _ = m.executeMenuAction("copy_search_backward")
	m = result.(Model)
	if !m.copySearchActive || m.copySearchDir != -1 {
		t.Fatalf("expected active backward search, active=%v dir=%d", m.copySearchActive, m.copySearchDir)
	}
}

func TestExecuteMenuActionSwapLeft(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	wins := m.wm.Windows()
	if len(wins) < 2 {
		t.Fatal("expected at least 2 windows")
	}
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	before := fw.Rect

	result, cmd := m.executeMenuAction("swap_left")
	m = result.(Model)
	if cmd == nil {
		t.Fatal("expected tickAnimation cmd for swap_left")
	}

	fw = m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after swap")
	}
	if fw.Rect == before {
		t.Fatal("expected focused window rect to change after swap_left")
	}
}

func TestExecuteMenuActionSnapTopBottom(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	wa := m.wm.WorkArea()

	result, _ := m.executeMenuAction("snap_top")
	m = result.(Model)
	fw := m.wm.FocusedWindow()
	if fw.Rect.Y != wa.Y {
		t.Errorf("snap_top: Y = %d, want %d", fw.Rect.Y, wa.Y)
	}
	if fw.Rect.Height != wa.Height/2 {
		t.Errorf("snap_top: height = %d, want %d", fw.Rect.Height, wa.Height/2)
	}

	result, _ = m.executeMenuAction("snap_bottom")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Y != wa.Y+wa.Height/2 {
		t.Errorf("snap_bottom: Y = %d, want %d", fw.Rect.Y, wa.Y+wa.Height/2)
	}
}

func TestExecuteMenuActionNewWorkspace(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("new_workspace")
	m = result.(Model)

	if m.newWorkspaceDialog == nil {
		t.Error("expected new workspace dialog after new_workspace menu action")
	}
}

func TestExecuteMenuActionUnknown(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("nonexistent_action")
	_ = result.(Model)
}

func TestSyncTilingMenuLabel(t *testing.T) {
	m := setupReadyModel()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.syncTilingMenuLabel()

	found := ""
	for _, menu := range m.menuBar.Menus {
		for _, item := range menu.Items {
			if item.Action == "toggle_tiling" {
				found = item.Label
				break
			}
		}
	}
	if found == "" {
		t.Fatal("toggle_tiling menu item not found")
	}
	if !strings.HasSuffix(found, "Tiling: On (Rows)") {
		t.Fatalf("toggle tiling label = %q, want suffix %q", found, "Tiling: On (Rows)")
	}
}

// --- applySettings tests ---

func TestApplySettingsThemeChange(t *testing.T) {
	m := setupReadyModel()
	origTheme := m.theme.Name

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "theme" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.CycleChoice(1)
				break
			}
		}
	}

	m.applySettings()

	if m.theme.Name == origTheme {
		t.Error("expected theme to change after applySettings with new theme")
	}
}

func TestApplySettingsAnimationsToggle(t *testing.T) {
	m := setupReadyModel()
	origAnimations := m.animationsOn

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "animations" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.Toggle()
				break
			}
		}
	}

	m.applySettings()

	if m.animationsOn == origAnimations {
		t.Errorf("expected animationsOn to toggle from %v", origAnimations)
	}
}

func TestApplySettingsIconsOnlyToggle(t *testing.T) {
	m := setupReadyModel()
	origIconsOnly := m.dock.IconsOnly

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "icons_only" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.Toggle()
				break
			}
		}
	}

	m.applySettings()

	if m.dock.IconsOnly == origIconsOnly {
		t.Errorf("expected IconsOnly to toggle from %v", origIconsOnly)
	}
}

func TestApplySettingsShowDeskClockToggle(t *testing.T) {
	m := setupReadyModel()
	origClock := m.showDeskClock

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "show_desk_clock" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.Toggle()
				break
			}
		}
	}

	m.applySettings()

	if m.showDeskClock == origClock {
		t.Errorf("expected showDeskClock to toggle from %v", origClock)
	}
}

func TestApplySettingsAnimationSpeed(t *testing.T) {
	m := setupReadyModel()

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "animation_speed" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.CycleChoice(1)
				break
			}
		}
	}

	m.applySettings()

	if m.springs == nil {
		t.Error("expected springs to be non-nil after applySettings")
	}
}

func TestApplySettingsAnimationStyle(t *testing.T) {
	m := setupReadyModel()

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "animation_style" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.CycleChoice(1)
				break
			}
		}
	}

	m.applySettings()

	if m.springs == nil {
		t.Error("expected springs to be non-nil after applySettings")
	}
}

func TestApplySettingsMinimizeToDock(t *testing.T) {
	m := setupReadyModel()
	origMinimizeToDock := m.dock.MinimizeToDock

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "minimize_to_dock" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.Toggle()
				break
			}
		}
	}

	m.applySettings()

	if m.dock.MinimizeToDock == origMinimizeToDock {
		t.Errorf("expected MinimizeToDock to toggle from %v", origMinimizeToDock)
	}
}

func TestApplySettingsHideDockWhenMaximized(t *testing.T) {
	m := setupReadyModel()
	origHide := m.hideDockWhenMaximized

	for si, sec := range m.settings.Sections {
		for ii, item := range sec.Items {
			if item.Key == "hide_dock_when_maximized" {
				m.settings.SetTab(si)
				m.settings.SetItem(ii)
				m.settings.Toggle()
				break
			}
		}
	}

	m.applySettings()

	if m.hideDockWhenMaximized == origHide {
		t.Errorf("expected hideDockWhenMaximized to toggle from %v", origHide)
	}
}

// --- handleSettingsClick tests ---

func TestSettingsClickOutsideDismisses(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	if !m.settings.Visible {
		t.Fatal("settings should be visible")
	}

	mouse := tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}
	result, _ := m.handleSettingsClick(mouse)
	m = result.(Model)

	if m.settings.Visible {
		t.Error("expected settings to be hidden after clicking outside panel")
	}
}

func TestSettingsClickOnTab(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	if m.settings.ActiveTab != 0 {
		t.Fatal("expected initial tab to be 0")
	}

	innerW := m.settings.InnerWidth(m.width)
	sec := m.settings.Sections[m.settings.ActiveTab]
	innerH := 6 + len(sec.Items) + 4
	boxW := innerW + 2
	boxH := innerH + 2
	startX := (m.width - boxW) / 2
	startY := (m.height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}

	// Tabs row is at relY=3 => absolute Y = startY + 1 (border) + 3
	tabsY := startY + 1 + 3

	// Compute click X for the second tab
	totalTabW := 0
	for _, s := range m.settings.Sections {
		totalTabW += len([]rune(s.Title)) + 2
	}
	tabOffset := (innerW - totalTabW) / 2
	if tabOffset < 0 {
		tabOffset = 0
	}
	firstTabW := len([]rune(m.settings.Sections[0].Title)) + 2
	clickX := startX + 1 + tabOffset + firstTabW + 1

	mouse := tea.Mouse{X: clickX, Y: tabsY, Button: tea.MouseLeft}
	result, _ := m.handleSettingsClick(mouse)
	m = result.(Model)

	if m.settings.ActiveTab != 1 {
		t.Errorf("expected ActiveTab=1 after clicking second tab, got %d", m.settings.ActiveTab)
	}
}

func TestSettingsClickOnItem(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	// Item at index 1 in General tab is "animations" (TypeToggle)
	origAnimations := m.settings.Sections[0].Items[1].BoolVal

	innerW := m.settings.InnerWidth(m.width)
	sec := m.settings.Sections[m.settings.ActiveTab]
	innerH := 6 + len(sec.Items) + 4
	boxW := innerW + 2
	boxH := innerH + 2
	startX := (m.width - boxW) / 2
	startY := (m.height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	_ = boxW
	_ = boxH

	// Items start at relY=6, so item 1 (animations) is at relY=7
	itemY := startY + 1 + 7
	itemX := startX + 1 + innerW/2

	mouse := tea.Mouse{X: itemX, Y: itemY, Button: tea.MouseLeft}
	result, _ := m.handleSettingsClick(mouse)
	m = result.(Model)

	newAnimations := m.settings.Sections[0].Items[1].BoolVal
	if newAnimations == origAnimations {
		t.Error("expected animations toggle to change after clicking on it")
	}
}

// --- Additional executeAction tests (uncovered branches) ---

func TestExecuteActionNewWorkspace(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("new_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for new_workspace action")
	}
	if m.newWorkspaceDialog == nil {
		t.Error("expected new workspace dialog after new_workspace action")
	}
}

func TestExecuteActionNewWorkspaceFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("new_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after new_workspace from terminal, got %d", m.inputMode)
	}
	if m.newWorkspaceDialog == nil {
		t.Error("expected new workspace dialog")
	}
}

func TestExecuteActionEnterTerminal(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// No terminal exists for demo windows, so should stay in current mode
	result, cmd := m.executeAction("enter_terminal", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for enter_terminal with no terminal")
	}
	if m.inputMode == ModeTerminal {
		t.Error("should not enter terminal mode without a terminal")
	}
}

func TestExecuteActionEnterTerminalNoWindow(t *testing.T) {
	m := setupReadyModel()

	// No windows at all
	result, cmd := m.executeAction("enter_terminal", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestExecuteActionRename(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, cmd := m.executeAction("rename", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for rename action")
	}
	if m.renameDialog == nil {
		t.Error("expected rename dialog after rename action")
	}
	if m.renameDialog.WindowID != fw.ID {
		t.Errorf("rename dialog window ID = %q, want %q", m.renameDialog.WindowID, fw.ID)
	}
}

func TestExecuteActionRenameNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("rename", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.renameDialog != nil {
		t.Error("should not create rename dialog without focused window")
	}
}

func TestExecuteActionRenameFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("rename", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after rename from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionDockFocus(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("dock_focus", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for dock_focus action")
	}
	if !m.dockFocused {
		t.Error("expected dockFocused=true after dock_focus action")
	}
}

func TestExecuteActionDockFocusFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("dock_focus", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after dock_focus from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionMaximize(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, cmd := m.executeAction("maximize", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw = m.wm.FocusedWindow()
	if fw.Rect != wa {
		t.Errorf("maximized rect = %v, want work area %v", fw.Rect, wa)
	}
}

func TestExecuteActionMaximizeNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("maximize", tea.KeyPressMsg(tea.Key{}), "")
	_ = result.(Model)
	if cmd == nil {
		t.Error("expected tickAnimation cmd even without focused window")
	}
}

func TestExecuteActionRestore(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Maximize first
	window.Maximize(fw, m.wm.WorkArea())
	origPreMax := fw.PreMaxRect

	result, cmd := m.executeAction("restore", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	fw = m.wm.FocusedWindow()
	if origPreMax != nil && fw.IsMaximized() {
		t.Error("expected window to be restored (not maximized)")
	}
}

func TestExecuteActionRestoreNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("restore", tea.KeyPressMsg(tea.Key{}), "")
	_ = result.(Model)
	if cmd == nil {
		t.Error("expected tickAnimation cmd")
	}
}

func TestExecuteActionSnapTop(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, cmd := m.executeAction("snap_top", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw := m.wm.FocusedWindow()
	if fw.Rect.Y != wa.Y {
		t.Errorf("snap_top: Y = %d, want %d", fw.Rect.Y, wa.Y)
	}
	if fw.Rect.Height != wa.Height/2 {
		t.Errorf("snap_top: height = %d, want %d", fw.Rect.Height, wa.Height/2)
	}
}

func TestExecuteActionSnapBottom(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, cmd := m.executeAction("snap_bottom", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	fw := m.wm.FocusedWindow()
	if fw.Rect.Y != wa.Y+wa.Height/2 {
		t.Errorf("snap_bottom: Y = %d, want %d", fw.Rect.Y, wa.Y+wa.Height/2)
	}
}

func TestExecuteActionMoveLeft(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origX := fw.Rect.X

	result, cmd := m.executeAction("move_left", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for move_left")
	}
	fw = m.wm.FocusedWindow()
	if fw.Rect.X >= origX {
		t.Error("expected window to move left")
	}
}

func TestExecuteActionMoveRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origX := fw.Rect.X

	result, cmd := m.executeAction("move_right", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for move_right")
	}
	fw = m.wm.FocusedWindow()
	if fw.Rect.X <= origX {
		t.Error("expected window to move right")
	}
}

func TestExecuteActionMoveUp(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	// Place window lower so it can move up
	fw := m.wm.FocusedWindow()
	fw.Rect.Y = 10

	result, cmd := m.executeAction("move_up", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for move_up")
	}
	fw = m.wm.FocusedWindow()
	if fw.Rect.Y >= 10 {
		t.Error("expected window to move up")
	}
}

func TestExecuteActionMoveDown(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origY := fw.Rect.Y

	result, cmd := m.executeAction("move_down", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for move_down")
	}
	fw = m.wm.FocusedWindow()
	if fw.Rect.Y <= origY {
		t.Error("expected window to move down")
	}
}

func TestExecuteActionMoveNoWindow(t *testing.T) {
	m := setupReadyModel()

	for _, action := range []string{"move_left", "move_right", "move_up", "move_down"} {
		result, cmd := m.executeAction(action, tea.KeyPressMsg(tea.Key{}), "")
		_ = result.(Model)
		if cmd != nil {
			t.Errorf("%s: expected nil cmd without focused window", action)
		}
	}
}

func TestExecuteActionGrowShrinkWidth(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origW := fw.Rect.Width

	result, _ := m.executeAction("grow_width", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Width <= origW {
		t.Error("expected width to grow")
	}

	grownW := fw.Rect.Width
	result, _ = m.executeAction("shrink_width", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Width >= grownW {
		t.Error("expected width to shrink")
	}
}

func TestExecuteActionGrowShrinkHeight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origH := fw.Rect.Height

	result, _ := m.executeAction("grow_height", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Height <= origH {
		t.Error("expected height to grow")
	}

	grownH := fw.Rect.Height
	result, _ = m.executeAction("shrink_height", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Height >= grownH {
		t.Error("expected height to shrink")
	}
}

func TestExecuteActionResizeNoWindow(t *testing.T) {
	m := setupReadyModel()

	for _, action := range []string{"grow_width", "shrink_width", "grow_height", "shrink_height"} {
		result, cmd := m.executeAction(action, tea.KeyPressMsg(tea.Key{}), "")
		_ = result.(Model)
		if cmd != nil {
			t.Errorf("%s: expected nil cmd without focused window", action)
		}
	}
}

func TestExecuteActionResizeNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Resizable = false
	origW := fw.Rect.Width

	result, _ := m.executeAction("grow_width", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect.Width != origW {
		t.Error("non-resizable window should not change width")
	}
}

func TestExecuteActionClipboardHistory(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("clipboard_history", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for clipboard_history")
	}
	if !m.clipboard.Visible {
		t.Error("expected clipboard history visible after clipboard_history action")
	}
}

func TestExecuteActionNotificationCenter(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("notification_center", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for notification_center")
	}
}

func TestExecuteActionSaveWorkspace(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{ProjectDir: tmpDir}

	result, _ := m.executeAction("save_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	time.Sleep(100 * time.Millisecond) // async save goroutine
}

func TestExecuteActionSaveWorkspaceFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{ProjectDir: tmpDir}
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("save_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after save_workspace from terminal, got %d", m.inputMode)
	}
	// saveWorkspaceNow spawns a goroutine; wait for it to finish
	// so t.TempDir cleanup doesn't race with the file write.
	time.Sleep(100 * time.Millisecond)
}

func TestExecuteActionLoadWorkspace(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("load_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	// cmd may be non-nil (async workspace discovery)
	if !m.workspacePickerVisible {
		t.Error("expected workspacePickerVisible to be true")
	}
}

func TestExecuteActionLoadWorkspaceFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("load_workspace", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after load_workspace from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionToggleExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// Toggle on
	result, cmd := m.executeAction("toggle_expose", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	if !m.exposeMode {
		t.Error("expected exposeMode=true after toggle_expose")
	}

	// Toggle off
	result, cmd = m.executeAction("toggle_expose", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd (tickAnimation)")
	}
	if m.exposeMode {
		t.Error("expected exposeMode=false after second toggle_expose")
	}
}

func TestExecuteActionToggleExposeFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("toggle_expose", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after toggle_expose from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionMenuBar(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("menu_bar", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for menu_bar")
	}
	if m.menuBar.OpenIndex < 0 {
		t.Error("expected menu to be open after menu_bar action")
	}
}

func TestExecuteActionMenuFile(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("menu_file", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.menuBar.OpenIndex != 0 {
		t.Errorf("expected OpenIndex=0 after menu_file, got %d", m.menuBar.OpenIndex)
	}
}

func TestExecuteActionMenuEdit(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("menu_edit", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.menuBar.OpenIndex != 1 {
		t.Errorf("expected OpenIndex=1 after menu_edit, got %d", m.menuBar.OpenIndex)
	}
}

func TestExecuteActionMenuApps(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("menu_apps", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.menuBar.OpenIndex != 2 {
		t.Errorf("expected OpenIndex=2 after menu_apps, got %d", m.menuBar.OpenIndex)
	}
}

func TestExecuteActionMenuView(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("menu_view", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.menuBar.OpenIndex != 3 {
		t.Errorf("expected OpenIndex=3 after menu_view, got %d", m.menuBar.OpenIndex)
	}
}

func TestExecuteActionMenusFromTerminalMode(t *testing.T) {
	for _, action := range []string{"menu_bar", "menu_file", "menu_edit", "menu_apps", "menu_view"} {
		m := setupReadyModel()
		m.inputMode = ModeTerminal

		result, _ := m.executeAction(action, tea.KeyPressMsg(tea.Key{}), "")
		m = result.(Model)
		if m.inputMode != ModeNormal {
			t.Errorf("%s: expected ModeNormal from terminal, got %d", action, m.inputMode)
		}
	}
}

func TestExecuteActionCloseWindowNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeAction("close_window", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.confirmClose != nil {
		t.Error("should not create confirm dialog without focused window")
	}
}

func TestExecuteActionCloseWindowFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("close_window", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after close_window from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionQuitFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("quit", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after quit from terminal, got %d", m.inputMode)
	}
	if m.confirmClose == nil || !m.confirmClose.IsQuit {
		t.Error("expected quit confirm dialog")
	}
}

func TestExecuteActionMinimizeNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeAction("minimize", tea.KeyPressMsg(tea.Key{}), "")
	_ = result.(Model)
	if cmd == nil {
		t.Error("expected tickAnimation cmd")
	}
}

func TestExecuteActionMinimizeFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("minimize", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after minimize from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionSnapNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Resizable = false
	origRect := fw.Rect

	for _, action := range []string{"snap_left", "snap_right", "snap_top", "snap_bottom", "center", "maximize"} {
		result, _ := m.executeAction(action, tea.KeyPressMsg(tea.Key{}), "")
		m = result.(Model)
		fw = m.wm.FocusedWindow()
		if fw.Rect != origRect {
			t.Errorf("%s: non-resizable window rect changed from %v to %v", action, origRect, fw.Rect)
		}
	}
}

func TestExecuteActionLauncherFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("launcher", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after launcher from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionHelpFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("help", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after help from terminal, got %d", m.inputMode)
	}
}

func TestExecuteActionSettingsFromTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	result, _ := m.executeAction("settings", tea.KeyPressMsg(tea.Key{}), "")
	m = result.(Model)
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after settings from terminal, got %d", m.inputMode)
	}
}

// --- confirmAccept tests ---

func TestConfirmAcceptNilDialog(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = nil

	result, cmd := m.confirmAccept()
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when confirmClose is nil")
	}
}

func TestConfirmAcceptNoButton(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{Title: "Close?", Selected: 1} // 1 = No

	result, cmd := m.confirmAccept()
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when No is selected")
	}
	if m.confirmClose != nil {
		t.Error("expected confirmClose to be cleared after No")
	}
}

func TestConfirmAcceptQuit(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{ProjectDir: tmpDir}
	m.confirmClose = &ConfirmDialog{Title: "Quit?", IsQuit: true, Selected: 0} // 0 = Yes

	result, cmd := m.confirmAccept()
	_ = result.(Model)
	if cmd == nil {
		t.Error("expected tea.Quit cmd after accepting quit")
	}
}

func TestConfirmAcceptCloseWindowWithAnimation(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	m.confirmClose = &ConfirmDialog{
		Title:    "Close?",
		WindowID: fw.ID,
		Selected: 0, // Yes
	}

	result, cmd := m.confirmAccept()
	m = result.(Model)
	if cmd == nil {
		t.Error("expected tickAnimation cmd for close animation")
	}
	if m.confirmClose != nil {
		t.Error("expected confirmClose to be cleared")
	}
}

func TestConfirmAcceptCloseWindowNotFound(t *testing.T) {
	m := setupReadyModel()

	// Window ID that doesn't exist in the manager
	m.confirmClose = &ConfirmDialog{
		Title:    "Close?",
		WindowID: "nonexistent-id",
		Selected: 0, // Yes
	}

	result, cmd := m.confirmAccept()
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when window not found (no animation)")
	}
	if m.confirmClose != nil {
		t.Error("expected confirmClose to be cleared")
	}
}

// --- Additional executeMenuAction tests (uncovered branches) ---

func TestExecuteMenuActionSaveWorkspace(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{ProjectDir: tmpDir}

	result, cmd := m.executeMenuAction("save_workspace")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for save_workspace menu action")
	}
	time.Sleep(100 * time.Millisecond) // wait for async save goroutine
}

func TestExecuteMenuActionLoadWorkspace(t *testing.T) {
	m := setupReadyModel()

	result, _ := m.executeMenuAction("load_workspace")
	m = result.(Model)
	// cmd may be non-nil (async workspace discovery)
	if !m.workspacePickerVisible {
		t.Error("expected workspacePickerVisible to be true")
	}
}

func TestExecuteMenuActionMinimizeNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeMenuAction("minimize")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for minimize without focused window")
	}
}

func TestExecuteMenuActionMaximizeNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeMenuAction("maximize")
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for maximize without focused window")
	}
}

func TestExecuteMenuActionSnapNoWindow(t *testing.T) {
	m := setupReadyModel()

	for _, action := range []string{"snap_left", "snap_right", "snap_top", "snap_bottom", "center"} {
		result, cmd := m.executeMenuAction(action)
		_ = result.(Model)
		if cmd != nil {
			t.Errorf("%s: expected nil cmd without focused window", action)
		}
	}
}

func TestExecuteMenuActionCloseWindowNoWindow(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeMenuAction("close_window")
	m = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for close_window without focused window")
	}
	if m.confirmClose != nil {
		t.Error("should not create confirm dialog without focused window")
	}
}

func TestExecuteMenuActionLaunchPrefix(t *testing.T) {
	m := setupReadyModel()

	// launch: prefix should try to launch via registry
	result, _ := m.executeMenuAction("launch:$SHELL")
	m = result.(Model)
	// Should create a new window (shell launch)
	if m.wm.Count() == 0 {
		t.Error("expected at least 1 window after launch:$SHELL")
	}
}

func TestExecuteMenuActionDetach(t *testing.T) {
	m := setupReadyModel()

	// Should not panic; detach writes to stdout but we just check it returns
	result, cmd := m.executeMenuAction("detach")
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for detach action")
	}
}

func TestExecuteMenuActionShowDesktopNoWindows(t *testing.T) {
	m := setupReadyModel()

	result, cmd := m.executeMenuAction("show_desktop")
	_ = result.(Model)
	if cmd == nil {
		t.Error("expected tickAnimation cmd for show_desktop")
	}
}

func TestExecuteMenuActionMaximizeNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Resizable = false
	origRect := fw.Rect

	result, _ := m.executeMenuAction("maximize")
	m = result.(Model)
	fw = m.wm.FocusedWindow()
	if fw.Rect != origRect {
		t.Error("non-resizable window should not be maximized via menu")
	}
}

func TestExecuteMenuActionSnapNonResizable(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Resizable = false
	origRect := fw.Rect

	for _, action := range []string{"snap_left", "snap_right", "snap_top", "snap_bottom", "center"} {
		result, _ := m.executeMenuAction(action)
		m = result.(Model)
		fw = m.wm.FocusedWindow()
		if fw.Rect != origRect {
			t.Errorf("%s: non-resizable window rect changed", action)
		}
	}
}

func TestSettingsClickInsideBorderNoOp(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	innerW := m.settings.InnerWidth(m.width)
	sec := m.settings.Sections[m.settings.ActiveTab]
	innerH := 6 + len(sec.Items) + 4
	boxW := innerW + 2
	boxH := innerH + 2
	startX := (m.width - boxW) / 2
	startY := (m.height - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	_ = innerW
	_ = boxW
	_ = boxH

	// Click on left border (startX, startY+2)
	mouse := tea.Mouse{X: startX, Y: startY + 2, Button: tea.MouseLeft}
	result, _ := m.handleSettingsClick(mouse)
	m = result.(Model)

	if !m.settings.Visible {
		t.Error("settings should remain visible after clicking on border")
	}
}

// --- isLocalShellTitle tests ---

func TestIsLocalShellTitle_MatchesUserAtHostname(t *testing.T) {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	if user == "" {
		t.Skip("no USER or LOGNAME env var set")
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		t.Skip("hostname is empty")
	}

	// Should match user@hostname
	title := user + "@" + hostname
	if !isLocalShellTitle(title) {
		t.Errorf("expected %q to be identified as a local shell title", title)
	}

	// Should match user@hostname:path
	title = user + "@" + hostname + ":/home"
	if !isLocalShellTitle(title) {
		t.Errorf("expected %q to be identified as a local shell title", title)
	}
}

func TestIsLocalShellTitle_RejectsNonShellTitles(t *testing.T) {
	// A title that doesn't start with user@ should not match
	if isLocalShellTitle("nvim ~/code/main.go") {
		t.Error("expected nvim title to not be a local shell title")
	}
	if isLocalShellTitle("htop") {
		t.Error("expected htop to not be a local shell title")
	}
	if isLocalShellTitle("") {
		t.Error("expected empty string to not be a local shell title")
	}
}

func TestIsLocalShellTitle_ShortHostname(t *testing.T) {
	user := os.Getenv("USER")
	if user == "" {
		t.Skip("no USER env var set")
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		t.Skip("hostname is empty")
	}
	// Even just user@shorthost should match
	shortHost := hostname
	if idx := len(hostname); idx > 0 {
		// Build a title with just the first part of the hostname
		for i, c := range hostname {
			if c == '.' {
				shortHost = hostname[:i]
				break
			}
		}
	}
	title := user + "@" + shortHost
	if !isLocalShellTitle(title) {
		t.Errorf("expected %q with short host to be a local shell title", title)
	}
}

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "Terminal 1", "Terminal 1"},
		{"path with symbols", "nvim ~/code/main.go", "nvim ~/code/main.go"},
		{"variation selectors stripped", "✨\uFE0F Session", "✨ Session"},
		{"zero-width joiners", "a\u200Db", "ab"},
		{"symbols kept", "✳ Session", "✳ Session"},
		{"braille kept", "⠂ thinking", "⠂ thinking"},
		{"unicode letters kept", "日本語タイトル", "日本語タイトル"},
		{"unicode punctuation kept", "title — dash", "title — dash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- normalizeTileSpawnPreset tests ---

func TestNormalizeTileSpawnPreset(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"auto", "auto"},
		{"AUTO", "auto"},
		{"  auto  ", "auto"},
		{"left", "left"},
		{"LEFT", "left"},
		{"right", "right"},
		{"RIGHT", "right"},
		{"up", "up"},
		{"UP", "up"},
		{"down", "down"},
		{"DOWN", "down"},
		{"invalid", "auto"},
		{"", "auto"},
	}
	for _, tc := range tests {
		got := normalizeTileSpawnPreset(tc.input)
		if got != tc.want {
			t.Errorf("normalizeTileSpawnPreset(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- tileSpawnPresetLabel tests ---

func TestTileSpawnPresetLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"auto", "Auto"},
		{"left", "Left"},
		{"right", "Right"},
		{"up", "Up"},
		{"down", "Down"},
		{"invalid", "Auto"},
		{"", "Auto"},
	}
	for _, tc := range tests {
		got := tileSpawnPresetLabel(tc.input)
		if got != tc.want {
			t.Errorf("tileSpawnPresetLabel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- absInt tests ---

func TestAbsInt(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},
		{5, 5},
		{-5, 5},
		{-1, 1},
		{1, 1},
		{-100, 100},
	}
	for _, tc := range tests {
		got := absInt(tc.input)
		if got != tc.want {
			t.Errorf("absInt(%d) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- swapFocusedWindow tests ---

func TestSwapFocusedWindowNoWindows(t *testing.T) {
	m := setupReadyModel()
	// No windows => no crash, returns unchanged model
	m2 := m.swapFocusedWindow(swapLeft)
	if m2.wm.Count() != 0 {
		t.Error("expected 0 windows")
	}
}

func TestSwapFocusedWindowSingleWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	// Single window => notification about no window in that direction
	m2 := m.swapFocusedWindow(swapLeft)
	if m2.wm.Count() != 1 {
		t.Error("expected 1 window")
	}
}

func TestSwapFocusedWindowLeftRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	// Tile columns so they are side-by-side
	window.TileColumns(m.wm.Windows(), m.wm.WorkArea())

	wins := m.wm.Windows()
	if len(wins) < 2 {
		t.Fatal("expected 2 windows")
	}

	// Focus the rightmost window
	var rightWin *window.Window
	for _, w := range wins {
		if rightWin == nil || w.Rect.X > rightWin.Rect.X {
			rightWin = w
		}
	}
	m.wm.FocusWindow(rightWin.ID)
	origRect := rightWin.Rect

	// Swap left
	m2 := m.swapFocusedWindow(swapLeft)
	m2 = completeAnimations(m2)

	// The focused window should now be at the left position
	fw := m2.wm.WindowByID(rightWin.ID)
	if fw.Rect.X >= origRect.X {
		t.Errorf("expected window to move left, was at X=%d, now at X=%d", origRect.X, fw.Rect.X)
	}
}

func TestSwapFocusedWindowUpDown(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	// Tile rows so they are stacked vertically
	window.TileRows(m.wm.Windows(), m.wm.WorkArea())

	wins := m.wm.Windows()
	if len(wins) < 2 {
		t.Fatal("expected 2 windows")
	}

	// Focus the bottom window
	var bottomWin *window.Window
	for _, w := range wins {
		if bottomWin == nil || w.Rect.Y > bottomWin.Rect.Y {
			bottomWin = w
		}
	}
	m.wm.FocusWindow(bottomWin.ID)
	origRect := bottomWin.Rect

	// Swap up
	m2 := m.swapFocusedWindow(swapUp)
	m2 = completeAnimations(m2)

	fw := m2.wm.WindowByID(bottomWin.ID)
	if fw.Rect.Y >= origRect.Y {
		t.Errorf("expected window to move up, was at Y=%d, now at Y=%d", origRect.Y, fw.Rect.Y)
	}
}

// --- currentTilingSlotByRect tests (rows layout) ---

func TestCurrentTilingSlotByRectRows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingLayout = "rows"
	window.TileRows(m.wm.Windows(), m.wm.WorkArea())

	wins := m.wm.Windows()
	if len(wins) < 3 {
		t.Fatal("expected 3 windows")
	}

	// Each window should have a valid slot
	for _, w := range wins {
		slot := m.currentTilingSlotByRect(w.ID)
		if slot < 0 {
			t.Errorf("window %s should have a valid tile slot in rows mode", w.ID)
		}
	}

	// Non-existent window should return -1
	if slot := m.currentTilingSlotByRect("nonexistent"); slot != -1 {
		t.Errorf("expected -1 for non-existent window, got %d", slot)
	}
}

// --- cycleTileSpawnPreset tests ---

func TestCycleTileSpawnPreset(t *testing.T) {
	m := setupReadyModel()
	m.tileSpawnPreset = "auto"
	m.cycleTileSpawnPreset()
	if m.tileSpawnPreset != "left" {
		t.Errorf("after auto, expected left, got %s", m.tileSpawnPreset)
	}
	m.cycleTileSpawnPreset()
	if m.tileSpawnPreset != "right" {
		t.Errorf("after left, expected right, got %s", m.tileSpawnPreset)
	}
	m.cycleTileSpawnPreset()
	if m.tileSpawnPreset != "up" {
		t.Errorf("after right, expected up, got %s", m.tileSpawnPreset)
	}
	m.cycleTileSpawnPreset()
	if m.tileSpawnPreset != "down" {
		t.Errorf("after up, expected down, got %s", m.tileSpawnPreset)
	}
	m.cycleTileSpawnPreset()
	if m.tileSpawnPreset != "auto" {
		t.Errorf("after down, expected auto, got %s", m.tileSpawnPreset)
	}
}

// --- slicesEqual tests ---

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{}, []string{}, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{nil, []string{"a"}, false},
	}
	for _, tc := range tests {
		got := slicesEqual(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("slicesEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// --- syncTilingMenuLabel tests ---

func TestSyncTilingMenuLabelNilMenuBar(t *testing.T) {
	m := setupReadyModel()
	m.menuBar = nil
	// Should not panic
	m.syncTilingMenuLabel()
}

// --- Bell indicator tests ---

func TestUpdateDockRunningBellPropagation(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Get a non-focused window and set HasBell
	var bellWin *window.Window
	fw := m.wm.FocusedWindow()
	for _, w := range m.wm.Windows() {
		if w.ID != fw.ID {
			bellWin = w
			break
		}
	}
	if bellWin == nil {
		t.Fatal("expected non-focused window")
	}
	bellWin.HasBell = true

	m.updateDockRunning()

	found := false
	for _, item := range m.dock.Items {
		if item.WindowID == bellWin.ID {
			found = true
			if !item.HasBell {
				t.Error("expected dock item to have HasBell=true")
			}
		}
	}
	if !found {
		t.Error("expected dock item for bell window")
	}
}

func TestBellMsgSetsHasBellOnUnfocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	var unfocused *window.Window
	fw := m.wm.FocusedWindow()
	for _, w := range m.wm.Windows() {
		if w.ID != fw.ID {
			unfocused = w
			break
		}
	}
	if unfocused == nil {
		t.Fatal("expected unfocused window")
	}

	// Send BellMsg for unfocused window
	result, _ := m.Update(BellMsg{WindowID: unfocused.ID})
	m = result.(Model)

	w := m.wm.WindowByID(unfocused.ID)
	if w == nil {
		t.Fatal("window not found after BellMsg")
	}
	if !w.HasBell {
		t.Error("expected HasBell=true on unfocused window after BellMsg")
	}
}

func TestBellMsgDoesNotSetHasBellOnFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	result, _ := m.Update(BellMsg{WindowID: fw.ID})
	m = result.(Model)

	w := m.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("window not found after BellMsg")
	}
	if w.HasBell {
		t.Error("expected HasBell=false on focused window after BellMsg")
	}
}

func TestFocusWindowClearsBell(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	var unfocused *window.Window
	fw := m.wm.FocusedWindow()
	for _, w := range m.wm.Windows() {
		if w.ID != fw.ID {
			unfocused = w
			break
		}
	}
	if unfocused == nil {
		t.Fatal("expected unfocused window")
	}
	unfocused.HasBell = true

	// Focus the window with bell
	m.wm.FocusWindow(unfocused.ID)

	if unfocused.HasBell {
		t.Error("expected HasBell to be cleared when window is focused")
	}
}
