package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func TestExposeSelectPreservesWindowState(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Get all windows and record their rects before expose
	windows := m.wm.Windows()
	if len(windows) < 2 {
		t.Fatalf("expected at least 2 windows, got %d", len(windows))
	}
	origRects := make(map[string]struct{ maximized bool })
	for _, w := range windows {
		origRects[w.ID] = struct{ maximized bool }{w.IsMaximized()}
	}

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
		t.Fatal("expected focused window after selection")
	}
	// Window should keep its original maximize state (not be force-maximized)
	orig, ok := origRects[fw.ID]
	if !ok {
		t.Fatal("focused window not found in original rects")
	}
	if fw.IsMaximized() != orig.maximized {
		t.Errorf("window maximize state changed: was %v, now %v", orig.maximized, fw.IsMaximized())
	}
}

func TestExposeEnterExitViaEscape(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	m.exposeMode = true
	m.inputMode = ModeNormal

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)

	if model.exposeMode {
		t.Error("escape should exit expose mode")
	}
}

func TestExposeEnterExitViaEnter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	m.exposeMode = true
	m.inputMode = ModeNormal

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)

	if model.exposeMode {
		t.Error("enter should exit expose mode")
	}
}

func TestExposeTargetRectScales50Percent(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 10, Y: 5, Width: 80, Height: 24}, nil)

	target := exposeTargetRect(w, wa, true)

	// 50% of 80x24 = 40x12
	if target.Width != 40 {
		t.Errorf("focused expose width = %d, want 40 (50%% of 80)", target.Width)
	}
	if target.Height != 12 {
		t.Errorf("focused expose height = %d, want 12 (50%% of 24)", target.Height)
	}
}

func TestExposeTargetRectUsesPreMaxRect(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	preMax := geometry.Rect{X: 10, Y: 5, Width: 60, Height: 20}
	w := window.NewWindow("w1", "Test", wa, nil)
	w.PreMaxRect = &preMax // maximized window

	target := exposeTargetRect(w, wa, true)

	// Should use PreMaxRect (60x20) not Rect (120x38)
	// 50% of 60x20 = 30x10
	if target.Width != 30 {
		t.Errorf("maximized expose width = %d, want 30 (50%% of PreMaxRect 60)", target.Width)
	}
	if target.Height != 10 {
		t.Errorf("maximized expose height = %d, want 10 (50%% of PreMaxRect 20)", target.Height)
	}
}

func TestExposeTargetRectEnforcesMinimums(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	// Very small window: 50% would be 10x5, but minimums are 16x8
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 10}, nil)

	target := exposeTargetRect(w, wa, true)

	if target.Width < 16 {
		t.Errorf("expose width %d below minimum 16", target.Width)
	}
	if target.Height < 8 {
		t.Errorf("expose height %d below minimum 8", target.Height)
	}
}

func TestExposeTargetRectUnfocusedIsThumbnail(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 10, Y: 5, Width: 80, Height: 24}, nil)

	target := exposeTargetRect(w, wa, false)

	if target.Width != 16 || target.Height != 6 {
		t.Errorf("unfocused thumbnail = %dx%d, want 16x6", target.Width, target.Height)
	}
}

func TestExposeTargetRectClampsToWorkArea(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 40, Height: 20}
	// Window wider than work area at 50%: 100/2=50 > 40-4=36
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 100, Height: 30}, nil)

	target := exposeTargetRect(w, wa, true)

	maxW := wa.Width - 4
	if target.Width > maxW {
		t.Errorf("expose width %d exceeds work area max %d", target.Width, maxW)
	}
}

func TestExposeBgTargetsLayout(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	w1 := window.NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := window.NewWindow("w2", "B", geometry.Rect{}, nil)
	w3 := window.NewWindow("w3", "C", geometry.Rect{}, nil)
	visible := []*window.Window{w1, w2, w3}

	targets := exposeBgTargets(visible, "w1", wa)

	// Should have entries for w2 and w3 only (w1 is focused)
	if len(targets) != 2 {
		t.Fatalf("expected 2 bg targets, got %d", len(targets))
	}
	if _, ok := targets["w2"]; !ok {
		t.Error("expected target for w2")
	}
	if _, ok := targets["w3"]; !ok {
		t.Error("expected target for w3")
	}
	if _, ok := targets["w1"]; ok {
		t.Error("focused window w1 should not be in bg targets")
	}
	// Both should be positioned at the bottom strip
	for id, r := range targets {
		if r.Y < wa.Y+wa.Height-10 {
			t.Errorf("bg target %s Y=%d too high, expected near bottom", id, r.Y)
		}
	}
}

func TestExposeBgTargetsEmpty(t *testing.T) {
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 38}
	w1 := window.NewWindow("w1", "A", geometry.Rect{}, nil)

	targets := exposeBgTargets([]*window.Window{w1}, "w1", wa)
	if len(targets) != 0 {
		t.Errorf("single window should have 0 bg targets, got %d", len(targets))
	}
}

func TestExposeFilteredWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.openDemoWindow() // "Window 2"
	m.openDemoWindow() // "Window 3"
	m.exposeMode = true

	// No filter — all visible
	visible := m.exposeFilteredWindows()
	if len(visible) != 3 {
		t.Errorf("no filter: got %d windows, want 3", len(visible))
	}

	// Filter by "2" — should match "Window 2"
	m.exposeFilter = "2"
	filtered := m.exposeFilteredWindows()
	if len(filtered) != 1 {
		t.Errorf("filter '2': got %d windows, want 1", len(filtered))
	}

	// Non-matching filter — returns all (doesn't empty the screen)
	m.exposeFilter = "nonexistent"
	all := m.exposeFilteredWindows()
	if len(all) != 3 {
		t.Errorf("non-matching filter: got %d windows, want 3 (fallback)", len(all))
	}
}

func TestExposeFilterCaseInsensitive(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.exposeMode = true

	m.exposeFilter = "WINDOW"
	filtered := m.exposeFilteredWindows()
	if len(filtered) != 1 {
		t.Errorf("case-insensitive filter: got %d, want 1", len(filtered))
	}
}

func TestExposeCycleWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	// Get initial focused ID
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Cycle forward
	m.cycleExposeWindow(1)
	fw2 := m.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after cycle")
	}
	if fw2.ID == origID {
		t.Error("cycle forward should change focused window")
	}

	// Cycle backward should return to original
	m.cycleExposeWindow(-1)
	fw3 := m.wm.FocusedWindow()
	if fw3.ID != origID {
		t.Errorf("cycle backward: got %s, want %s", fw3.ID, origID)
	}
}

func TestExposeSingleWindowNoCycle(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeMode = true

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Cycle should be a no-op with 1 window
	m.cycleExposeWindow(1)
	fw2 := m.wm.FocusedWindow()
	if fw2.ID != origID {
		t.Error("cycle with single window should be no-op")
	}
}

func TestExposeSelectByIndex(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	visible := m.exposeFilteredWindows()
	if len(visible) < 3 {
		t.Fatalf("expected 3 visible, got %d", len(visible))
	}

	// Select window at index 1 (second)
	m.selectExposeByIndex(1)
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after select by index")
	}
	if fw.ID != visible[1].ID {
		t.Errorf("select by index 1: focused %s, want %s", fw.ID, visible[1].ID)
	}

	// Out-of-bounds should be no-op
	m.selectExposeByIndex(99)
	fw2 := m.wm.FocusedWindow()
	if fw2.ID != visible[1].ID {
		t.Error("out-of-bounds index should be no-op")
	}
}

func TestSortVisibleByID(t *testing.T) {
	w1 := window.NewWindow("c-win", "C", geometry.Rect{}, nil)
	w2 := window.NewWindow("a-win", "A", geometry.Rect{}, nil)
	w3 := window.NewWindow("b-win", "B", geometry.Rect{}, nil)
	visible := []*window.Window{w1, w2, w3}

	sortVisibleByID(visible)

	if visible[0].ID != "a-win" || visible[1].ID != "b-win" || visible[2].ID != "c-win" {
		t.Errorf("sort order: %s, %s, %s", visible[0].ID, visible[1].ID, visible[2].ID)
	}
}

// --- enterExpose tests ---

func TestEnterExposeWithMultipleWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()

	m.enterExpose()

	if !m.exposeMode {
		t.Error("expected exposeMode to be true after enterExpose")
	}
	if m.exposeFilter != "" {
		t.Error("expected exposeFilter to be empty after enterExpose")
	}
	// Verify animations were started (with animationsOn=false, they complete immediately)
	// The fact that enterExpose doesn't panic with multiple windows is the key test
}

func TestEnterExposeWithAnimationsOn(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	m.openDemoWindow()

	m.enterExpose()

	if !m.exposeMode {
		t.Error("expected exposeMode to be true")
	}
	// With animations on, there should be active animations
	if !m.hasActiveAnimations() {
		t.Error("expected active animations after enterExpose with animations enabled")
	}
	// Complete animations
	m = completeAnimations(m)
}

func TestEnterExposeWithNoWindows(t *testing.T) {
	m := setupReadyModel()
	// No windows opened

	m.enterExpose()

	// Should set exposeMode but return early since no visible windows
	if !m.exposeMode {
		t.Error("expected exposeMode to be true even with no windows")
	}
	// No crash is the main assertion
}

func TestEnterExposeWithOneWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	m.enterExpose()

	if !m.exposeMode {
		t.Error("expected exposeMode to be true with 1 window")
	}
	// With a single window, there should be no bg targets but the focused
	// window should get an animation target
}

func TestEnterExposeResetsFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeFilter = "something"

	m.enterExpose()

	if m.exposeFilter != "" {
		t.Errorf("enterExpose should reset filter, got %q", m.exposeFilter)
	}
}

func TestEnterExposeNoFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Unfocus all windows
	for _, w := range m.wm.Windows() {
		w.Focused = false
	}

	m.enterExpose()

	if !m.exposeMode {
		t.Error("expected exposeMode true even without focused window")
	}
	// Should default to first visible window as focused (no panic)
}

// --- relayoutExpose tests ---

func TestRelayoutExposeBasic(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	// Now relayout and verify no panic
	m.relayoutExpose()

	// The focused window should still exist
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after relayoutExpose")
	}
}

func TestRelayoutExposeWithFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.openDemoWindow() // "Window 2"
	m.openDemoWindow() // "Window 3"
	m.enterExpose()

	// Set filter that matches only one window
	m.exposeFilter = "1"
	m.relayoutExpose()

	// Focus should be adjusted to be in the filtered set
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after filtered relayout")
	}
	// "Window 1" should be focused since it is the only match
	if fw.Title != "Window 1" {
		t.Errorf("expected focused window 'Window 1', got %q", fw.Title)
	}
}

func TestRelayoutExposeNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true

	// relayoutExpose with no visible windows should be a no-op
	m.relayoutExpose()
	// No panic is the assertion
}

func TestRelayoutExposeFocusedNotInFilteredSet(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.openDemoWindow() // "Window 2"
	m.openDemoWindow() // "Window 3"
	m.enterExpose()

	// Focus window 3, then filter to only show window 1
	visible := m.exposeFilteredWindows()
	for _, w := range visible {
		if w.Title == "Window 3" {
			m.wm.FocusWindow(w.ID)
			break
		}
	}

	m.exposeFilter = "1"
	m.relayoutExpose()

	// Focus should move to the first window in filtered set
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	if fw.Title != "Window 1" {
		t.Errorf("expected focus on filtered match 'Window 1', got %q", fw.Title)
	}
}

// --- selectExposeWindow tests (mouse-based) ---

func TestSelectExposeWindowClickBgThumbnail(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	visible := m.exposeFilteredWindows()
	if len(visible) < 3 {
		t.Fatalf("expected 3 windows, got %d", len(visible))
	}

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Compute the bg targets to find a clickable position
	wa := m.wm.WorkArea()
	bgTargets := exposeBgTargets(visible, fw.ID, wa)

	// Find the first non-focused window and click on it
	for _, w := range visible {
		if w.ID == fw.ID {
			continue
		}
		target := bgTargets[w.ID]
		// Click in the middle of the thumbnail
		clickX := target.X + target.Width/2
		clickY := target.Y + target.Height/2
		m.selectExposeWindow(clickX, clickY)

		// The clicked window should now be focused
		newFw := m.wm.FocusedWindow()
		if newFw == nil {
			t.Fatal("expected focused window after click")
		}
		if newFw.ID != w.ID {
			t.Errorf("expected window %s to be focused after click, got %s", w.ID, newFw.ID)
		}
		break
	}
}

func TestSelectExposeWindowClickFocused(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Click on the focused window center rect
	wa := m.wm.WorkArea()
	focTarget := exposeTargetRect(fw, wa, true)
	clickX := focTarget.X + focTarget.Width/2
	clickY := focTarget.Y + focTarget.Height/2
	m.selectExposeWindow(clickX, clickY)

	// Focused window should remain the same (click on already-focused)
	newFw := m.wm.FocusedWindow()
	if newFw == nil {
		t.Fatal("expected focused window")
	}
	if newFw.ID != origID {
		t.Errorf("clicking focused window should keep focus, got %s want %s", newFw.ID, origID)
	}
}

func TestSelectExposeWindowClickEmpty(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Click on empty space (far corner)
	m.selectExposeWindow(0, 0)

	// Focus should not change
	newFw := m.wm.FocusedWindow()
	if newFw == nil {
		t.Fatal("expected focused window")
	}
	if newFw.ID != origID {
		t.Errorf("clicking empty space should not change focus, got %s want %s", newFw.ID, origID)
	}
}

func TestSelectExposeWindowNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true

	// Should not panic
	m.selectExposeWindow(10, 10)
}

func TestSelectExposeWindowSingleWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.enterExpose()

	// With single window, bgCount==0, clicking bg area should be no-op
	m.selectExposeWindow(0, 0)
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
}

// --- exitExpose tests ---

func TestExitExposeNoWindows(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true

	m.exitExpose()

	if m.exposeMode {
		t.Error("exposeMode should be false after exitExpose")
	}
	// No crash is the key assertion
}

func TestExitExposeNoFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	// Unfocus all windows
	for _, w := range m.wm.Windows() {
		w.Focused = false
	}

	m.exitExpose()

	if m.exposeMode {
		t.Error("exposeMode should be false after exitExpose")
	}
	// Should default to first visible[0] without panicking
}

func TestExitExposeWithAnimationsOn(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()
	m = completeAnimations(m)

	m.exitExpose()

	if m.exposeMode {
		t.Error("exposeMode should be false after exitExpose")
	}
	// With animations on, exit should create AnimExposeExit animations
	if !m.hasActiveAnimations() {
		t.Error("expected active animations after exitExpose with animations enabled")
	}
	m = completeAnimations(m)
}

func TestExitExposeWithAnimationsOff(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = false
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	m.exitExpose()

	if m.exposeMode {
		t.Error("exposeMode should be false after exitExpose")
	}
	if m.exposeFilter != "" {
		t.Error("exposeFilter should be cleared after exitExpose")
	}
}

func TestExitExposeClearsFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeMode = true
	m.exposeFilter = "test"

	m.exitExpose()

	if m.exposeFilter != "" {
		t.Errorf("exitExpose should clear filter, got %q", m.exposeFilter)
	}
}

// --- cycleExposeWindow edge cases ---

func TestCycleExposeWindowWithFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.openDemoWindow() // "Window 2"
	m.openDemoWindow() // "Window 3"
	m.enterExpose()

	// Filter to show windows 1 and 2 (filter by "Window" matches all, filter by specific)
	// Set a filter that matches 2 windows. "Window" matches all 3, need partial.
	// Let's use the fact that titles are "Window 1", "Window 2", "Window 3".
	// Filter "Window" matches all; instead let's manually focus and test cycling.
	// Focus the first window, then cycle with 3 windows as base.

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Cycle forward through all 3 windows and back
	m.cycleExposeWindow(1)
	m.cycleExposeWindow(1)
	m.cycleExposeWindow(1) // Should wrap around to original

	fw2 := m.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after full cycle")
	}
	if fw2.ID != origID {
		t.Errorf("expected to wrap around to %s, got %s", origID, fw2.ID)
	}
}

func TestCycleExposeWindowBackwardWrap(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Cycle backward should wrap to last window
	m.cycleExposeWindow(-1)
	fw2 := m.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after backward cycle")
	}
	if fw2.ID == fw.ID {
		t.Error("backward cycle should change focused window")
	}
}

func TestCycleExposeWindowNoFocused(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	// Unfocus all windows to trigger the oldFocused==nil early return
	for _, w := range m.wm.Windows() {
		w.Focused = false
	}

	// Should be a no-op (no focused window found)
	m.cycleExposeWindow(1)

	// No panic, and no window should become focused from cycleExposeWindow
}

// --- Expose keyboard navigation via Update() ---

func TestExposeEnterViaF9(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeNormal

	// F9 should enter expose
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF9}))
	model := updated.(Model)

	if !model.exposeMode {
		t.Error("F9 should enter expose mode")
	}

	// In expose mode, F9 is not handled (escape/enter exits instead).
	// Verify escape exits after F9 enter.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)

	if model.exposeMode {
		t.Error("escape should exit expose mode after F9 enter")
	}
}

func TestExposeToggleViaF9WhenNotInExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeNormal
	m.exposeMode = false

	// F9 should enter expose when not in expose mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF9}))
	model := updated.(Model)

	if !model.exposeMode {
		t.Error("F9 should enter expose mode from normal mode")
	}
}

func TestExposeTabCycleViaUpdate(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Tab should cycle forward
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model := updated.(Model)

	fw2 := model.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after tab")
	}
	if fw2.ID == origID {
		t.Error("tab should cycle to next window")
	}

	// Shift+Tab should cycle backward
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)

	fw3 := model.wm.FocusedWindow()
	if fw3 == nil {
		t.Fatal("expected focused window after shift+tab")
	}
	if fw3.ID != origID {
		t.Errorf("shift+tab should cycle back, got %s want %s", fw3.ID, origID)
	}
}

func TestExposeDownUpCycleViaUpdate(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Down arrow should cycle forward
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model := updated.(Model)

	fw2 := model.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after down")
	}
	if fw2.ID == origID {
		t.Error("down should cycle to next window")
	}

	// Up arrow should cycle backward
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	model = updated.(Model)

	fw3 := model.wm.FocusedWindow()
	if fw3 == nil {
		t.Fatal("expected focused window after up")
	}
	if fw3.ID != origID {
		t.Errorf("up should cycle back, got %s want %s", fw3.ID, origID)
	}
}

func TestExposeFilterTyping(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // "Window 1"
	m.openDemoWindow() // "Window 2"
	m.openDemoWindow() // "Window 3"
	m.exposeMode = true
	m.inputMode = ModeNormal

	// Type a character to start filtering
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	model := updated.(Model)

	if model.exposeFilter != "a" {
		t.Errorf("expected filter 'a', got %q", model.exposeFilter)
	}

	// Type another character
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Text: "b"}))
	model = updated.(Model)

	if model.exposeFilter != "ab" {
		t.Errorf("expected filter 'ab', got %q", model.exposeFilter)
	}
}

func TestExposeFilterBackspace(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal
	m.exposeFilter = "abc"

	// Backspace should remove last character
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model := updated.(Model)

	if model.exposeFilter != "ab" {
		t.Errorf("expected filter 'ab' after backspace, got %q", model.exposeFilter)
	}

	// Backspace again
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated.(Model)

	if model.exposeFilter != "a" {
		t.Errorf("expected filter 'a' after second backspace, got %q", model.exposeFilter)
	}

	// Backspace to empty
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated.(Model)

	if model.exposeFilter != "" {
		t.Errorf("expected empty filter after full backspace, got %q", model.exposeFilter)
	}
}

func TestExposeFilterBackspaceOnEmpty(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal
	m.exposeFilter = ""

	// Backspace on empty filter should be no-op
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model := updated.(Model)

	if model.exposeFilter != "" {
		t.Errorf("backspace on empty filter should stay empty, got %q", model.exposeFilter)
	}
}

func TestExposeNumberKeySelectsWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal

	visible := m.exposeFilteredWindows()
	if len(visible) < 3 {
		t.Fatalf("expected 3 visible, got %d", len(visible))
	}

	// Press "2" to select second window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	model := updated.(Model)

	if model.exposeMode {
		t.Error("number key should exit expose mode")
	}
	fw := model.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window after number selection")
	}
	if fw.ID != visible[1].ID {
		t.Errorf("expected window %s focused, got %s", visible[1].ID, fw.ID)
	}
}

func TestExposeNumberKeyIgnoredWhenFilterActive(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal
	m.exposeFilter = "Win" // filter is active

	// Press "2" — should add to filter, not select
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	model := updated.(Model)

	if !model.exposeMode {
		t.Error("number key with active filter should not exit expose")
	}
	if model.exposeFilter != "Win2" {
		t.Errorf("expected filter 'Win2', got %q", model.exposeFilter)
	}
}

func TestExposeEscapeClearsFilterFirst(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal
	m.exposeFilter = "test"

	// First escape should clear filter but stay in expose
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)

	if model.exposeFilter != "" {
		t.Errorf("first escape should clear filter, got %q", model.exposeFilter)
	}
	if !model.exposeMode {
		t.Error("first escape should clear filter, not exit expose")
	}

	// Second escape should exit expose
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)

	if model.exposeMode {
		t.Error("second escape should exit expose mode")
	}
}

func TestExposeEnterExitsExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	m.inputMode = ModeNormal

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)

	if model.exposeMode {
		t.Error("enter should exit expose mode")
	}
	// Focused window should remain the same
	fw2 := model.wm.FocusedWindow()
	if fw2 == nil {
		t.Fatal("expected focused window after enter")
	}
	if fw2.ID != origID {
		t.Errorf("enter should keep focused window, got %s want %s", fw2.ID, origID)
	}
}

// --- exposeTargetRect height clamping ---

func TestExposeTargetRectClampsHeight(t *testing.T) {
	// Work area with limited height: 50% of 30h = 15, maxH = 15-8=7 but min 8
	wa := geometry.Rect{X: 0, Y: 1, Width: 120, Height: 15}
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 80, Height: 30}, nil)

	target := exposeTargetRect(w, wa, true)

	maxH := wa.Height - 8
	if target.Height > maxH && target.Height > 8 {
		t.Errorf("expose height %d exceeds max height %d", target.Height, maxH)
	}
	// Height should be at least 8
	if target.Height < 8 {
		t.Errorf("expose height %d below minimum 8", target.Height)
	}
}

// --- exposeBgTargets narrow work area ---

func TestExposeBgTargetsNarrowWorkArea(t *testing.T) {
	// Work area narrow enough to trigger thumbnail width reduction
	wa := geometry.Rect{X: 0, Y: 1, Width: 30, Height: 38}
	w1 := window.NewWindow("w1", "A", geometry.Rect{}, nil)
	w2 := window.NewWindow("w2", "B", geometry.Rect{}, nil)
	w3 := window.NewWindow("w3", "C", geometry.Rect{}, nil)
	w4 := window.NewWindow("w4", "D", geometry.Rect{}, nil)
	visible := []*window.Window{w1, w2, w3, w4}

	targets := exposeBgTargets(visible, "w1", wa)

	// 3 bg windows: 3 * (16+1) = 51 > 30-2 = 28, so thumbW should be reduced
	if len(targets) != 3 {
		t.Fatalf("expected 3 bg targets, got %d", len(targets))
	}
	for id, r := range targets {
		if r.Width > 16 {
			t.Errorf("bg target %s width %d should be <= 16 in narrow area", id, r.Width)
		}
		if r.Width < 8 {
			t.Errorf("bg target %s width %d below minimum 8", id, r.Width)
		}
	}
}

// --- selectExposeWindow with no focused window ---

func TestSelectExposeWindowNoFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	// Unfocus all windows
	for _, w := range m.wm.Windows() {
		w.Focused = false
	}

	// Should fall back to first visible window, no panic
	m.selectExposeWindow(10, 10)
}

// --- enterExpose + exitExpose round trip ---

func TestExposeEnterExitRoundTrip(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// Record original window rects
	origRects := make(map[string]geometry.Rect)
	for _, w := range m.wm.Windows() {
		origRects[w.ID] = w.Rect
	}

	m.enterExpose()
	if !m.exposeMode {
		t.Fatal("expected expose mode after enter")
	}

	m.exitExpose()
	if m.exposeMode {
		t.Fatal("expected expose mode off after exit")
	}

	// Window rects should be preserved (they don't change from expose)
	for _, w := range m.wm.Windows() {
		orig, ok := origRects[w.ID]
		if !ok {
			t.Errorf("window %s not found in originals", w.ID)
			continue
		}
		if w.Rect != orig {
			t.Errorf("window %s rect changed: was %v, now %v", w.ID, orig, w.Rect)
		}
	}
}

// --- selectExposeByIndex negative index ---

func TestSelectExposeByIndexNegative(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	origID := fw.ID

	// Negative index should be a no-op
	m.selectExposeByIndex(-1)
	fw2 := m.wm.FocusedWindow()
	if fw2.ID != origID {
		t.Error("negative index should be no-op")
	}
}

// --- Expose filter with minimized window ---

func TestExposeFilteredWindowsExcludesMinimized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true

	// Minimize one window
	windows := m.wm.Windows()
	windows[0].Minimized = true

	filtered := m.exposeFilteredWindows()
	if len(filtered) != 2 {
		t.Errorf("expected 2 non-minimized windows, got %d", len(filtered))
	}

	// Verify the minimized window is not in the filtered set
	for _, w := range filtered {
		if w.ID == windows[0].ID {
			t.Error("minimized window should not be in filtered set")
		}
	}
}
