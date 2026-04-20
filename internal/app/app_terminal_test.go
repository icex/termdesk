package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

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
	cmd := m.openTerminalWindowWith("", nil, "", "")
	if cmd != nil {
		t.Error("expected nil cmd when at max windows")
	}
	if m.wm.Count() != 9 {
		t.Errorf("expected 9 windows after max limit, got %d", m.wm.Count())
	}
}

func TestPtyClosedHoldsWindowOpen(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID
	if m.wm.Count() != 1 {
		t.Fatal("expected 1 window")
	}

	// Simulate PTY closed message — holds window open with [exited] marker
	updated, _ := m.Update(PtyClosedMsg{WindowID: winID})
	model := updated.(Model)

	// Window should still exist but be marked as exited
	w := model.wm.WindowByID(winID)
	if w == nil {
		t.Fatal("expected window to still exist after PtyClosedMsg")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited")
	}
	if !strings.Contains(w.Title, "[exited]") {
		t.Errorf("expected title to contain [exited], got %q", w.Title)
	}

	// Press 'q' in terminal mode should close the exited window
	model.inputMode = ModeTerminal
	model.wm.FocusWindow(winID)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	model = updated.(Model)
	model = completeAnimations(model)
	if model.wm.Count() != 0 {
		t.Errorf("expected 0 windows after 'q' on exited window, got %d", model.wm.Count())
	}
}

func TestExitedWindowRestartWithR(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID

	// Mark as exited
	updated, _ := m.Update(PtyClosedMsg{WindowID: winID})
	model := updated.(Model)

	w := model.wm.WindowByID(winID)
	if w == nil || !w.Exited {
		t.Fatal("expected exited window")
	}

	// Press 'r' — should restart (reuse window frame)
	model.inputMode = ModeTerminal
	model.wm.FocusWindow(winID)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'r'}))
	model = updated.(Model)

	// Window should still exist but no longer be exited
	w = model.wm.WindowByID(winID)
	if w == nil {
		t.Fatal("window should still exist after restart")
	}
	if w.Exited {
		t.Error("expected Exited=false after restart")
	}
	if strings.Contains(w.Title, "[exited]") {
		t.Error("expected [exited] suffix removed from title")
	}
}

func TestExitedWindowSwallowsOtherKeys(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	winID := m.wm.FocusedWindow().ID

	updated, _ := m.Update(PtyClosedMsg{WindowID: winID})
	model := updated.(Model)

	// Press a random key on exited window — should not close it
	model.inputMode = ModeTerminal
	model.wm.FocusWindow(winID)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'x'}))
	model = updated.(Model)

	if model.wm.Count() != 1 {
		t.Errorf("expected window to still exist after 'x' key, got %d windows", model.wm.Count())
	}
	w := model.wm.WindowByID(winID)
	if w == nil || !w.Exited {
		t.Error("window should still be exited after irrelevant keypress")
	}
}

func TestPtyClosedDuplicateRespectsTilingLayout(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	closeID := fw.ID

	updated, _ := m.Update(PtyClosedMsg{WindowID: closeID})
	model := updated.(Model)
	updated, _ = model.Update(PtyClosedMsg{WindowID: closeID})
	model = updated.(Model)

	wa := model.wm.WorkArea()
	for _, w := range model.wm.Windows() {
		if w.Minimized || !w.Visible {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("remaining window width = %d, want %d after duplicate close in rows mode", w.Rect.Width, wa.Width)
		}
	}
}

func TestWindowArgsStoredOnCreate(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("npm", []string{"run", "dev"}, "Dev Server", "/tmp")
	m = completeAnimations(m)

	// Find the created window
	windows := m.wm.Windows()
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	w := windows[0]
	if w.Command != "npm" {
		t.Errorf("command: got %q, want %q", w.Command, "npm")
	}
	if len(w.Args) != 2 || w.Args[0] != "run" || w.Args[1] != "dev" {
		t.Errorf("args: got %v, want [run dev]", w.Args)
	}
	if w.WorkDir != "/tmp" {
		t.Errorf("workdir: got %q, want %q", w.WorkDir, "/tmp")
	}
}

func TestWindowArgsNilForShell(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("", nil, "", "/tmp")
	m = completeAnimations(m)

	windows := m.wm.Windows()
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].Args != nil {
		t.Errorf("shell window should have nil args, got %v", windows[0].Args)
	}
}

func TestBufferCleanupOnClose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	w := m.wm.Windows()[0]
	wid := w.ID
	m.windowBuffers[wid] = "some buffer content"

	m.closeTerminal(wid)

	if _, ok := m.windowBuffers[wid]; ok {
		t.Error("buffer should be removed when window is closed")
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

func TestCloseAllTerminalsClearsMap(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"one"}, "", "/tmp")
	m.openTerminalWindowWith("/bin/echo", []string{"two"}, "", "/tmp")
	m = completeAnimations(m)

	if len(m.terminals) == 0 {
		t.Fatal("expected terminals to be created")
	}
	m.closeAllTerminals()
	if len(m.terminals) != 0 {
		t.Errorf("expected terminals map to be empty, got %d", len(m.terminals))
	}
}

func TestResizeTerminalForWindow(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"resize"}, "", "/tmp")
	m = completeAnimations(m)

	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}
	term := m.terminals[w.ID]
	if term == nil {
		t.Fatal("expected terminal for focused window")
	}

	// Expand window and resize
	w.Rect.Width += 5
	w.Rect.Height += 2
	m.resizeTerminalForWindow(w)

	cr := w.ContentRect()
	if term.Width() != cr.Width || term.Height() != cr.Height {
		t.Fatalf("terminal size %dx%d, want %dx%d", term.Width(), term.Height(), cr.Width, cr.Height)
	}
}

func TestCreateTerminalWindowSkipAutoTileKeepsExistingLayout(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	orig := make(map[string]geometry.Rect)
	for _, w := range m.wm.Windows() {
		orig[w.ID] = w.Rect
	}

	cmd := m.createTerminalWindow(TerminalWindowOpts{
		Command:      "/bin/echo",
		Args:         []string{"autostart"},
		DisplayName:  "Auto",
		WorkDir:      "/tmp",
		SkipAutoTile: true,
	})
	if cmd == nil {
		t.Fatal("expected createTerminalWindow cmd")
	}

	for _, w := range m.wm.Windows() {
		if prev, ok := orig[w.ID]; ok && w.Rect != prev {
			t.Fatalf("existing window %s rect changed from %v to %v with SkipAutoTile", w.ID, prev, w.Rect)
		}
	}
}

func TestCreateTerminalWindowAppliesSelectedRowsTiling(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	cmd := m.createTerminalWindow(TerminalWindowOpts{
		Command:     "/bin/echo",
		Args:        []string{"rows"},
		DisplayName: "Rows",
		WorkDir:     "/tmp",
	})
	if cmd == nil {
		t.Fatal("expected createTerminalWindow cmd")
	}
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if w.Rect.Width != wa.Width {
			t.Fatalf("window %s width=%d want=%d after new window in rows tiling", w.ID, w.Rect.Width, wa.Width)
		}
	}
}

func TestApplyTileSpawnPresetLeftUsesAnchorSlot(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.applyTilingLayout()

	wins := m.wm.Windows()
	if len(wins) < 3 {
		t.Fatal("expected 3 windows")
	}
	anchorID := wins[1].ID
	anchorSlot := m.wm.TilingSlotOf(anchorID)
	if anchorSlot < 0 {
		t.Fatal("anchor slot should exist")
	}

	newWin := window.NewWindow("new-left", "New", geometry.Rect{X: 0, Y: 1, Width: 20, Height: 10}, nil)
	m.wm.AddWindow(newWin)
	m.tileSpawnPreset = "left"
	m.applyTileSpawnPreset("new-left", anchorID)

	if got := m.wm.TilingSlotOf("new-left"); got != anchorSlot {
		t.Fatalf("new slot = %d, want %d (anchor slot)", got, anchorSlot)
	}
}

func TestApplyTileSpawnPresetRightUsesAnchorPlusOne(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.applyTilingLayout()

	wins := m.wm.Windows()
	if len(wins) < 3 {
		t.Fatal("expected 3 windows")
	}
	anchorID := wins[1].ID
	anchorSlot := m.wm.TilingSlotOf(anchorID)
	if anchorSlot < 0 {
		t.Fatal("anchor slot should exist")
	}

	newWin := window.NewWindow("new-right", "New", geometry.Rect{X: 0, Y: 1, Width: 20, Height: 10}, nil)
	m.wm.AddWindow(newWin)
	m.tileSpawnPreset = "right"
	m.applyTileSpawnPreset("new-right", anchorID)

	if got := m.wm.TilingSlotOf("new-right"); got != anchorSlot+1 {
		t.Fatalf("new slot = %d, want %d (anchor+1)", got, anchorSlot+1)
	}
}

// --- windowIcon tests ---

func TestWindowIconShell(t *testing.T) {
	m := setupReadyModel()
	icon, color := m.windowIcon("")
	if icon == "" {
		t.Error("expected icon for shell")
	}
	_ = color

	icon2, _ := m.windowIcon("$SHELL")
	if icon2 != icon {
		t.Errorf("$SHELL icon %q should match empty icon %q", icon2, icon)
	}

	icon3, _ := m.windowIcon("bash")
	if icon3 != icon {
		t.Errorf("bash icon %q should match shell icon %q", icon3, icon)
	}
}

func TestWindowIconKnownCommands(t *testing.T) {
	m := setupReadyModel()
	tests := []struct {
		cmd      string
		wantIcon bool
	}{
		{"nvim", true},
		{"vim", true},
		{"mc", true},
		{"htop", true},
		{"python3", true},
		{"nano", true},
		{"/usr/bin/nvim", true}, // full path should also match
		{"unknown-binary", true}, // returns default terminal icon
	}
	for _, tc := range tests {
		icon, _ := m.windowIcon(tc.cmd)
		if tc.wantIcon && icon == "" {
			t.Errorf("windowIcon(%q) returned empty icon", tc.cmd)
		}
	}
}

func TestWindowIconFromRegistry(t *testing.T) {
	m := setupReadyModel()
	m.registry = []registry.RegistryEntry{
		{Command: "mycustomapp", Name: "Custom", Icon: "\uf0f6", IconColor: "#FF0000"},
	}
	icon, color := m.windowIcon("mycustomapp")
	if icon != "\uf0f6" {
		t.Errorf("expected registry icon, got %q", icon)
	}
	if color != "#FF0000" {
		t.Errorf("expected registry color #FF0000, got %q", color)
	}
}

func TestWindowIconFullPathFromRegistry(t *testing.T) {
	m := setupReadyModel()
	m.registry = []registry.RegistryEntry{
		{Command: "/usr/local/bin/myapp", Name: "MyApp", Icon: "\ue62b", IconColor: "#00FF00"},
	}
	// Match by base name
	icon, color := m.windowIcon("myapp")
	if icon != "\ue62b" {
		t.Errorf("expected registry icon for base name match, got %q", icon)
	}
	if color != "#00FF00" {
		t.Errorf("expected registry color, got %q", color)
	}
}

// --- hasActiveOverlay tests ---

func TestHasActiveOverlayNoOverlays(t *testing.T) {
	m := setupReadyModel()
	if m.hasActiveOverlay() {
		t.Error("expected no active overlay on fresh model")
	}
}

func TestHasActiveOverlayModal(t *testing.T) {
	m := setupReadyModel()
	m.modal = &ModalOverlay{Title: "Test"}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when modal is set")
	}
}

func TestHasActiveOverlayConfirmClose(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{Title: "Close?"}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when confirmClose is set")
	}
}

func TestHasActiveOverlayRenameDialog(t *testing.T) {
	m := setupReadyModel()
	m.renameDialog = &RenameDialog{WindowID: "w1", Text: []rune("test")}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when renameDialog is set")
	}
}

func TestHasActiveOverlayBufferNameDialog(t *testing.T) {
	m := setupReadyModel()
	m.bufferNameDialog = &BufferNameDialog{Text: []rune("buf")}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when bufferNameDialog is set")
	}
}

func TestHasActiveOverlayNewWorkspaceDialog(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = &NewWorkspaceDialog{}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when newWorkspaceDialog is set")
	}
}

func TestHasActiveOverlayLauncher(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when launcher is visible")
	}
}

func TestHasActiveOverlayExposeMode(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay in expose mode")
	}
}

func TestHasActiveOverlaySettings(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when settings is visible")
	}
}

func TestHasActiveOverlayClipboard(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.ToggleHistory()
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when clipboard is visible")
	}
}

func TestHasActiveOverlayWorkspacePicker(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when workspace picker is visible")
	}
}

func TestHasActiveOverlayContextMenu(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{}
	if !m.hasActiveOverlay() {
		t.Error("expected active overlay when context menu is set")
	}
}

// --- focusOrLaunchApp tests ---

func TestFocusOrLaunchAppShellBypassesDedup(t *testing.T) {
	m := setupReadyModel()
	// $SHELL and "" should go straight to launch (no dedup)
	result, _ := m.focusOrLaunchApp("$SHELL")
	m = result.(Model)
	// Should have created a window
	if m.wm.Count() != 1 {
		t.Errorf("expected 1 window after launching $SHELL, got %d", m.wm.Count())
	}
}

func TestFocusOrLaunchAppFocusesExisting(t *testing.T) {
	m := setupReadyModel()
	// Create a window with a specific command
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Command = "htop"

	// Create a second window and focus it
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw.ID == w.ID {
		t.Fatal("expected different focused window")
	}

	// Now focusOrLaunchApp("htop") should focus the existing window
	result, _ := m.focusOrLaunchApp("htop")
	m = result.(Model)

	if m.wm.FocusedWindow().ID != w.ID {
		t.Errorf("expected focused window to be %s (htop), got %s", w.ID, m.wm.FocusedWindow().ID)
	}
}

func TestFocusOrLaunchAppRestoresMinimized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	w.Command = "htop"
	w.Minimized = true

	result, cmd := m.focusOrLaunchApp("htop")
	m = result.(Model)

	if cmd == nil {
		t.Error("expected animation cmd for restore")
	}
	w = m.wm.WindowByID(w.ID)
	if w.Minimized {
		t.Error("expected window to be restored from minimized")
	}
}

func TestPtyClosedUsesSelectedTileAllLayout(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "all"
	m.applyTilingLayout()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}
	closeID := fw.ID

	updated, _ := m.Update(PtyClosedMsg{WindowID: closeID})
	m = updated.(Model)
	m = completeAnimations(m)

	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if w.Rect.Width >= wa.Width || w.Rect.Height >= wa.Height {
			t.Fatalf("window %s rect=%v not in tile-all shape after close (wa=%v)", w.ID, w.Rect, wa)
		}
	}
}
