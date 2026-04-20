package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

// ============================================================================
// collectTerminalLines tests
// ============================================================================

func TestCVCollectLinesScreenOnly(t *testing.T) {
	// Create a terminal with screen content but no scrollback.
	term, err := terminal.New("/bin/echo", []string{"HELLO SCREEN"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	lines := collectTerminalLines(term, nil)
	if len(lines) == 0 {
		t.Fatal("expected non-empty lines from terminal screen")
	}

	// At least one line should contain "HELLO SCREEN"
	found := false
	for _, line := range lines {
		if strings.Contains(line, "HELLO SCREEN") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'HELLO SCREEN' in collected lines")
	}
}

func TestCVCollectLinesWithSnapshot(t *testing.T) {
	// Create a snapshot with known content.
	snap := &CopySnapshot{
		WindowID: "test-win",
		Scrollback: [][]terminal.ScreenCell{
			// offset 0 = most recent scrollback line
			{
				{Content: "S", Width: 1},
				{Content: "B", Width: 1},
				{Content: "1", Width: 1},
			},
		},
		Screen: [][]terminal.ScreenCell{
			{
				{Content: "L", Width: 1},
				{Content: "I", Width: 1},
				{Content: "V", Width: 1},
				{Content: "E", Width: 1},
			},
		},
		Width:  4,
		Height: 1,
	}

	// When snapshot is provided, collectTerminalLines should use snapshot data.
	// term can be nil since the snapshot path is taken.
	lines := collectTerminalLines(nil, snap)

	// Should have scrollback lines + screen lines.
	// Scrollback: 1 line, Screen: 1 line = 2 total
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (1 scrollback + 1 screen), got %d", len(lines))
	}

	// First line should be scrollback (offset sbLen-1 = 0)
	if !strings.Contains(lines[0], "SB1") {
		t.Errorf("expected scrollback line 'SB1', got %q", lines[0])
	}

	// Second line should be screen content
	if !strings.Contains(lines[1], "LIVE") {
		t.Errorf("expected screen line 'LIVE', got %q", lines[1])
	}
}

func TestCVCollectLinesNilSnapshotNilTerm(t *testing.T) {
	// With nil snapshot and nil term, should not panic.
	// term.ScrollbackLen() etc. will panic with nil, so this path
	// requires the snapshot path. Actually if both are nil we get a panic.
	// The function is only called with a valid term or valid snapshot.
	// Test with snapshot having empty data.
	snap := &CopySnapshot{
		WindowID:   "empty",
		Scrollback: nil,
		Screen:     nil,
		Width:      10,
		Height:     0,
	}
	lines := collectTerminalLines(nil, snap)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines from empty snapshot, got %d", len(lines))
	}
}

func TestCVCollectLinesSnapshotMultipleScrollback(t *testing.T) {
	snap := &CopySnapshot{
		WindowID: "multi-sb",
		Scrollback: [][]terminal.ScreenCell{
			// offset 0 = most recent
			{{Content: "R", Width: 1}, {Content: "2", Width: 1}},
			// offset 1 = older
			{{Content: "R", Width: 1}, {Content: "1", Width: 1}},
		},
		Screen: [][]terminal.ScreenCell{
			{{Content: "S", Width: 1}, {Content: "0", Width: 1}},
		},
		Width:  2,
		Height: 1,
	}

	lines := collectTerminalLines(nil, snap)
	// 2 scrollback + 1 screen = 3
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Scrollback is iterated from sbLen-1 down to 0 (oldest first).
	// sbLen=2, so offset 1 (older) first, then offset 0 (newer).
	if !strings.Contains(lines[0], "R1") {
		t.Errorf("first line should be older scrollback 'R1', got %q", lines[0])
	}
	if !strings.Contains(lines[1], "R2") {
		t.Errorf("second line should be newer scrollback 'R2', got %q", lines[1])
	}
	if !strings.Contains(lines[2], "S0") {
		t.Errorf("third line should be screen 'S0', got %q", lines[2])
	}
}

// ============================================================================
// cellsToString tests
// ============================================================================

func TestCVCellsToStringEmpty(t *testing.T) {
	result := cellsToString(nil)
	if result != "" {
		t.Errorf("expected empty string for nil cells, got %q", result)
	}
}

func TestCVCellsToStringBasic(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "H", Width: 1},
		{Content: "i", Width: 1},
		{Content: " ", Width: 1},
	}
	result := cellsToString(cells)
	if result != "Hi" { // trailing spaces trimmed
		t.Errorf("expected 'Hi', got %q", result)
	}
}

func TestCVCellsToStringSkipsZeroWidth(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "A", Width: 1},
		{Content: "", Width: 0}, // continuation cell, should be skipped
		{Content: "B", Width: 1},
	}
	result := cellsToString(cells)
	if result != "AB" {
		t.Errorf("expected 'AB', got %q", result)
	}
}

func TestCVCellsToStringEmptyContentBecomesSpace(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "X", Width: 1},
		{Content: "", Width: 1}, // width=1 but empty content => space
		{Content: "Y", Width: 1},
	}
	result := cellsToString(cells)
	if result != "X Y" {
		t.Errorf("expected 'X Y', got %q", result)
	}
}

// ============================================================================
// View() tests
// ============================================================================

func TestCVViewNotReady(t *testing.T) {
	m := New()
	m.ready = false

	v := m.View()
	// The view should have AltScreen enabled.
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
	// Content should be "Starting termdesk..."
	// We can't directly access SetContent string, but the function should not panic.
}

func TestCVViewReadyNoWindows(t *testing.T) {
	m := setupReadyModel()

	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
	// Should have MouseModeAllMotion
	if v.MouseMode != 2 { // MouseModeAllMotion = 2
		t.Errorf("expected MouseModeAllMotion (2), got %d", v.MouseMode)
	}
}

func TestCVViewWithTooltip(t *testing.T) {
	m := setupReadyModel()
	m.tooltipText = "Test Tooltip"
	m.tooltipX = 20
	m.tooltipY = 10

	// Should render without panic, tooltip overlay drawn.
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
}

func TestCVViewWithShowKeys(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = true
	m.showKeysEvents = []showKeyEvent{
		{Key: "a", Action: "test", At: time.Now()},
	}

	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
}

func TestCVViewWithResizeIndicator(t *testing.T) {
	m := setupReadyModel()
	m.showResizeIndicator = true
	m.lastWindowSizeAt = time.Now() // recent enough to display

	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
}

func TestCVViewWithCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"COPY"}, "CopyWin", "")
	m = completeAnimations(m)

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Enable copy mode with a snapshot.
	if term, ok := m.terminals[fw.ID]; ok {
		m.copySnapshot = captureCopySnapshot(fw.ID, term)
	}
	m.inputMode = ModeCopy
	m.scrollOffset = 0

	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}

	// Cleanup
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCVViewCacheHitsOnSecondCall(t *testing.T) {
	m := setupReadyModel()

	// First call renders.
	_ = m.View()
	// The cache should be populated — viewGen == updateGen.
	if m.cache.viewGen != m.cache.updateGen {
		t.Error("expected viewGen == updateGen after first View()")
	}

	// Second call should return cached result.
	v2 := m.View()
	if !v2.AltScreen {
		t.Error("expected cached view to have AltScreen=true")
	}
}

func TestCVViewWithQuakeTerminal(t *testing.T) {
	m := setupReadyModel()

	// Create a quake terminal.
	qt, err := terminal.New("/bin/echo", []string{"QUAKE"}, m.width, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("failed to create quake terminal: %v", err)
	}
	defer qt.Close()

	done := make(chan struct{})
	go func() {
		qt.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	m.quakeTerminal = qt
	m.quakeVisible = true
	m.quakeAnimH = 15.0

	// Also create a regular window to test focus dimming.
	m.openDemoWindow()

	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true")
	}
}

// ============================================================================
// getNativeCursor tests
// ============================================================================

func TestCVGetNativeCursorNonTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor in non-terminal mode")
	}
}

func TestCVGetNativeCursorCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor in copy mode")
	}
}

func TestCVGetNativeCursorWithContextMenu(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.contextMenu = &contextmenu.Menu{Visible: true, Items: []contextmenu.Item{{Label: "Test"}}}

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor when context menu is visible")
	}
}

func TestCVGetNativeCursorWithConfirmClose(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.confirmClose = &ConfirmDialog{Title: "Quit?"}

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor when confirm dialog is visible")
	}
}

func TestCVGetNativeCursorWithModal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.modal = &ModalOverlay{Title: "Help"}

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor when modal is visible")
	}
}

func TestCVGetNativeCursorWithRenameDialog(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.renameDialog = &RenameDialog{}

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor when rename dialog is visible")
	}
}

func TestCVGetNativeCursorNoFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor with no focused window")
	}
}

func TestCVGetNativeCursorWithFocusedTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	m.openTerminalWindowWith("/bin/echo", []string{"CURSOR"}, "Cursor", "")
	m = completeAnimations(m)

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// Wait for terminal output.
	time.Sleep(200 * time.Millisecond)

	cursor := m.getNativeCursor()
	// The cursor may or may not be visible depending on echo's exit behavior,
	// but the function should not panic.
	_ = cursor

	// Cleanup
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCVGetNativeCursorMinimizedWindow(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw != nil {
		fw.Minimized = true
	}

	cursor := m.getNativeCursor()
	if cursor != nil {
		t.Error("expected nil cursor for minimized window")
	}
}

func TestCVGetNativeCursorQuakeTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal

	qt, err := terminal.New("/bin/echo", []string{"Q"}, m.width, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("failed to create quake terminal: %v", err)
	}
	defer qt.Close()

	done := make(chan struct{})
	go func() {
		qt.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	m.quakeTerminal = qt
	m.quakeVisible = true
	m.quakeAnimH = 15.0

	cursor := m.getNativeCursor()
	// May or may not be visible, but should not panic.
	_ = cursor
}

// ============================================================================
// appHomeDir tests
// ============================================================================

func TestCVAppHomeDirWithEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(envHomeDir, tmpDir)

	got := appHomeDir()
	if got != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, got)
	}
}

func TestCVAppHomeDirEmptyEnv(t *testing.T) {
	t.Setenv(envHomeDir, "")

	got := appHomeDir()
	// Should fall back to os.UserHomeDir().
	home, err := os.UserHomeDir()
	if err != nil {
		// On some CI, UserHomeDir might fail; just check non-empty.
		if got == "" {
			t.Error("expected non-empty home dir when TERMDESK_HOME is empty")
		}
		return
	}
	if got != home {
		t.Errorf("expected %q, got %q", home, got)
	}
}

func TestCVAppHomeDirWhitespaceEnv(t *testing.T) {
	t.Setenv(envHomeDir, "   ")

	got := appHomeDir()
	// Whitespace-only should be treated as empty.
	if got == "   " {
		t.Error("whitespace-only TERMDESK_HOME should be ignored")
	}
}

// ============================================================================
// captureWorkspaceState tests
// ============================================================================

func TestCVCaptureWorkspaceStateWithTerminals(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"capture1"}, "Win1", "")
	m = completeAnimations(m)
	m.openTerminalWindowWith("/bin/echo", []string{"capture2"}, "Win2", "")
	m = completeAnimations(m)

	state := m.captureWorkspaceState()
	if len(state.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(state.Windows))
	}

	// Each window should have position and size.
	for _, ws := range state.Windows {
		if ws.Width <= 0 || ws.Height <= 0 {
			t.Errorf("window %s has invalid size %dx%d", ws.ID, ws.Width, ws.Height)
		}
	}

	// Cleanup
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCVCaptureWorkspaceStateSkipsExited(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	wins := m.wm.Windows()
	if len(wins) == 0 {
		t.Fatal("expected at least 1 window")
	}
	wins[0].Exited = true

	state := m.captureWorkspaceState()
	if len(state.Windows) != 0 {
		t.Errorf("expected 0 windows (exited should be skipped), got %d", len(state.Windows))
	}
}

// ============================================================================
// restoreWorkspace tests
// ============================================================================

func TestCVRestoreWorkspaceEmptyState(t *testing.T) {
	m := setupReadyModel()
	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 0 {
		t.Errorf("expected 0 windows from empty state, got %d", m.wm.Count())
	}
}

func TestCVRestoreWorkspaceWithTempFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Save a workspace file.
	state := workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "ws-win-1",
				Title:   "Restored",
				Command: "/bin/echo",
				Args:    []string{"restored"},
				X:       5,
				Y:       3,
				Width:   50,
				Height:  15,
			},
		},
		FocusedID: "ws-win-1",
	}
	if err := workspace.SaveWorkspace(state, tmpDir); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	// Load and restore.
	m := setupReadyModel()
	loaded, err := workspace.LoadWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded workspace")
	}

	m.restoreWorkspace(loaded, tmpDir)

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Cleanup
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCVRestoreWorkspaceFocusMinimizedFallback(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "wmin", Title: "Min", Command: "/bin/echo", X: 2, Y: 2, Width: 50, Height: 15, Minimized: true},
			{ID: "wvis", Title: "Vis", Command: "/bin/echo", X: 10, Y: 10, Width: 50, Height: 15},
		},
		FocusedID: "wmin", // focused window is minimized
	}

	m.restoreWorkspace(state, "")

	// Should fallback to a non-minimized window.
	fw := m.wm.FocusedWindow()
	if fw != nil && fw.ID == "wmin" && fw.Minimized {
		t.Error("focused window should not be a minimized window")
	}

	// Cleanup
	for _, term := range m.terminals {
		term.Close()
	}
}

// ============================================================================
// stashCurrentWorkspace tests
// ============================================================================

func TestCVStashCurrentWorkspace(t *testing.T) {
	m := setupReadyModel()
	m.activeWorkspacePath = "/tmp/test-workspace/.termdesk-workspace.toml"
	m.backgroundWorkspaces = make(map[string]*backgroundWorkspace)

	// Open a demo window to have something to stash.
	m.openDemoWindow()

	origWM := m.wm

	m.stashCurrentWorkspace()

	// Should have been stashed into backgroundWorkspaces.
	bg, ok := m.backgroundWorkspaces[m.activeWorkspacePath]
	if !ok {
		t.Fatal("expected workspace to be stashed in backgroundWorkspaces")
	}
	if bg.wm != origWM {
		t.Error("expected stashed wm to be the original wm")
	}
	if bg.terminals == nil {
		t.Error("expected stashed terminals to be non-nil")
	}
}

func TestCVStashCurrentWorkspaceNoPath(t *testing.T) {
	m := setupReadyModel()
	m.activeWorkspacePath = "" // no active path = global workspace
	m.backgroundWorkspaces = make(map[string]*backgroundWorkspace)

	// Open a terminal window.
	m.openTerminalWindowWith("/bin/echo", []string{"stash"}, "Stash", "")
	m = completeAnimations(m)

	m.stashCurrentWorkspace()

	// Global workspace should close terminals, not stash.
	if len(m.backgroundWorkspaces) != 0 {
		t.Error("global workspace should not be stashed in backgroundWorkspaces")
	}
}

// ============================================================================
// switchToWorkspace tests
// ============================================================================

func TestCVSwitchToWorkspaceSamePathNoOp(t *testing.T) {
	m := setupReadyModel()
	m.backgroundWorkspaces = make(map[string]*backgroundWorkspace)
	m.activeWorkspacePath = "/tmp/same/.termdesk-workspace.toml"

	cmd := m.switchToWorkspace("/tmp/same/.termdesk-workspace.toml")
	if cmd != nil {
		t.Error("expected nil cmd when switching to same workspace")
	}
}

func TestCVSwitchToWorkspaceFromBackground(t *testing.T) {
	m := setupReadyModel()
	m.backgroundWorkspaces = make(map[string]*backgroundWorkspace)
	m.activeWorkspacePath = "/tmp/current/.termdesk-workspace.toml"

	// Stash a background workspace.
	bgWM := window.NewManager(m.width, m.height)
	bgWM.SetBounds(m.width, m.height)
	bgWM.SetReserved(1, 1)
	w := window.NewWindow("bg-w1", "Background", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 10}, nil)
	bgWM.AddWindow(w)

	targetPath := "/tmp/target/.termdesk-workspace.toml"
	m.backgroundWorkspaces[targetPath] = &backgroundWorkspace{
		wm:        bgWM,
		terminals: make(map[string]*terminal.Terminal),
	}

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	actualTargetPath := filepath.Join(targetDir, ".termdesk-workspace.toml")
	m.backgroundWorkspaces[actualTargetPath] = m.backgroundWorkspaces[targetPath]
	delete(m.backgroundWorkspaces, targetPath)

	_ = m.switchToWorkspace(actualTargetPath)

	// Background workspace should be restored.
	if _, ok := m.backgroundWorkspaces[actualTargetPath]; ok {
		t.Error("target workspace should be removed from background after restore")
	}
	if m.wm.Count() != 1 {
		t.Errorf("expected 1 window from background workspace, got %d", m.wm.Count())
	}
}

// ============================================================================
// getVisibleLineText tests
// ============================================================================

func TestCVGetVisibleLineTextWithSnapshot(t *testing.T) {
	snap := &CopySnapshot{
		WindowID: "test",
		Scrollback: [][]terminal.ScreenCell{
			// offset 0 = most recent
			{{Content: "N", Width: 1}, {Content: "E", Width: 1}, {Content: "W", Width: 1}},
			// offset 1 = older
			{{Content: "O", Width: 1}, {Content: "L", Width: 1}, {Content: "D", Width: 1}},
		},
		Screen: [][]terminal.ScreenCell{
			{{Content: "S", Width: 1}, {Content: "C", Width: 1}, {Content: "R", Width: 1}},
		},
		Width:  3,
		Height: 1,
	}

	// absLine 0 = oldest scrollback (offset sbLen-1 = 1)
	line0 := getVisibleLineText(0, 2, nil, snap)
	if !strings.Contains(line0, "OLD") {
		t.Errorf("absLine 0 should be oldest scrollback 'OLD', got %q", line0)
	}

	// absLine 1 = newest scrollback (offset 0)
	line1 := getVisibleLineText(1, 2, nil, snap)
	if !strings.Contains(line1, "NEW") {
		t.Errorf("absLine 1 should be 'NEW', got %q", line1)
	}

	// absLine 2 = screen row 0
	line2 := getVisibleLineText(2, 2, nil, snap)
	if !strings.Contains(line2, "SCR") {
		t.Errorf("absLine 2 should be screen 'SCR', got %q", line2)
	}

	// absLine 3 = out of range
	line3 := getVisibleLineText(3, 2, nil, snap)
	if line3 != "" {
		t.Errorf("absLine 3 should be empty, got %q", line3)
	}
}

func TestCVGetVisibleLineTextLiveTerminal(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"VISIBLE LINE"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	sbLen := term.ScrollbackLen()
	// First screen row = absLine sbLen.
	line := getVisibleLineText(sbLen, sbLen, term, nil)
	if !strings.Contains(line, "VISIBLE LINE") {
		t.Errorf("expected 'VISIBLE LINE' in screen row 0, got %q", line)
	}

	// Out of range line.
	oob := getVisibleLineText(sbLen+100, sbLen, term, nil)
	if oob != "" {
		t.Errorf("expected empty for out-of-range absLine, got %q", oob)
	}
}

// ============================================================================
// renderSearchHighlights tests
// ============================================================================

func TestCVRenderSearchHighlightsEmptyQuery(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(40, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	// Empty query should be a no-op (no highlights).
	renderSearchHighlights(buf, contentRect, nil, nil, 0, 0, "", theme)
}

func TestCVRenderSearchHighlightsWithMatch(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"FINDME"}, 20, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	buf := NewBuffer(30, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	sbLen := term.ScrollbackLen()

	// Render terminal content first.
	renderTerminalContent(buf, contentRect, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 0)

	// Then highlight search.
	renderSearchHighlights(buf, contentRect, term, nil, 0, sbLen, "FINDME", theme)

	// Check that the highlighted cells have accent colors.
	c := theme.C()
	foundHighlight := false
	for dy := 0; dy < contentRect.Height; dy++ {
		for dx := 0; dx < contentRect.Width; dx++ {
			bx := contentRect.X + dx
			by := contentRect.Y + dy
			if bx < buf.Width && by < buf.Height {
				cell := buf.Cells[by][bx]
				if colorsEqual(cell.Bg, c.AccentColor) && cell.Char != ' ' && cell.Char != 0 {
					foundHighlight = true
					break
				}
			}
		}
		if foundHighlight {
			break
		}
	}
	if !foundHighlight {
		t.Error("expected search highlight with accent color for 'FINDME'")
	}
}

func TestCVRenderSearchHighlightsWithSnapshot(t *testing.T) {
	theme := testTheme()

	snap := &CopySnapshot{
		WindowID:   "snap",
		Scrollback: nil,
		Screen: [][]terminal.ScreenCell{
			{
				{Content: "S", Width: 1},
				{Content: "E", Width: 1},
				{Content: "A", Width: 1},
				{Content: "R", Width: 1},
				{Content: "C", Width: 1},
				{Content: "H", Width: 1},
			},
		},
		Width:  6,
		Height: 1,
	}

	// Need a terminal (even though snapshot overrides); create a minimal one.
	term, err := terminal.New("/bin/echo", []string{"x"}, 10, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	buf := NewBuffer(20, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 10, Height: 3}

	// Render with snapshot.
	renderTerminalContentWithSnapshot(buf, contentRect, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 0, snap)
	renderSearchHighlights(buf, contentRect, term, snap, 0, 0, "SEARCH", theme)

	// Should highlight "SEARCH" in the snapshot content.
	c := theme.C()
	foundHighlight := false
	for dx := 0; dx < contentRect.Width; dx++ {
		bx := contentRect.X + dx
		by := contentRect.Y
		if bx < buf.Width && by < buf.Height {
			cell := buf.Cells[by][bx]
			if colorsEqual(cell.Bg, c.AccentColor) {
				foundHighlight = true
				break
			}
		}
	}
	if !foundHighlight {
		t.Error("expected search highlight in snapshot content")
	}
}

// ============================================================================
// RenderFrame additional coverage tests
// ============================================================================

func TestCVRenderFrameWithDeskClock(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, true, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	// Clock should draw something in the bottom-right area.
	hasClockContent := false
	for y := buf.Height - 10; y < buf.Height; y++ {
		for x := buf.Width - 30; x < buf.Width; x++ {
			if x >= 0 && y >= 0 && x < buf.Width && y < buf.Height {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar && ch != 0 {
					hasClockContent = true
					break
				}
			}
		}
		if hasClockContent {
			break
		}
	}
	if !hasClockContent {
		t.Error("expected desk clock content")
	}
}

func TestCVRenderFrameWithCopyModeSearchQuery(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Search", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	term, err := terminal.New("/bin/echo", []string{"SEARCHME"}, 28, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("failed to create terminal: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	terminals := map[string]*terminal.Terminal{
		"w1": term,
	}

	sel := SelectionInfo{
		Active:       false,
		CopyMode:     true,
		CopyWindowID: "w1",
		SearchQuery:  "SEARCH",
		CopyCursorX:  0,
		CopyCursorY:  0,
	}

	// Should render search highlights.
	buf := RenderFrame(wm, theme, terminals, nil, true, 0, sel, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

func TestCVRenderFrameWithAnimRectsAndTerminal(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Animated", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	term, err := terminal.New("/bin/echo", []string{"ANIM"}, 28, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("failed to create terminal: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	terminals := map[string]*terminal.Terminal{
		"w1": term,
	}

	animRects := map[string]geometry.Rect{
		"w1": {X: 10, Y: 8, Width: 25, Height: 10},
	}

	buf := RenderFrame(wm, theme, terminals, animRects, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	// Window should be at animated position.
	if buf.Cells[8][10].Char != theme.BorderTopLeft {
		t.Errorf("expected animated window border at (10,8)")
	}
}

func TestCVRenderFrameCacheCleanup(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Cache", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	cache := make(map[string]*windowRenderCache)

	// First render populates cache.
	buf1 := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, cache, nil, nil)
	if buf1 == nil {
		t.Fatal("expected non-nil buffer")
	}
	if _, ok := cache["w1"]; !ok {
		t.Error("expected cache entry for w1 after first render")
	}

	// Remove the window.
	wm.RemoveWindow("w1")

	// Second render should clean up the stale cache entry.
	buf2 := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, cache, nil, nil)
	if buf2 == nil {
		t.Fatal("expected non-nil buffer")
	}
	if _, ok := cache["w1"]; ok {
		t.Error("expected stale cache entry for w1 to be cleaned up")
	}
}

func TestCVRenderFrameZeroSize(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(0, 0)
	wm.SetReserved(0, 0)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer even for zero-size")
	}
}

// ============================================================================
// updateWorkspaceWidget tests
// ============================================================================

func TestCVUpdateWorkspaceWidgetNilWidget(t *testing.T) {
	m := setupReadyModel()
	m.workspaceWidget = nil

	// Should not panic.
	m.updateWorkspaceWidget("/some/path")
}

func TestCVUpdateWorkspaceWidgetEmptyDir(t *testing.T) {
	m := setupReadyModel()
	w := &widget.WorkspaceWidget{}
	m.workspaceWidget = w

	m.updateWorkspaceWidget("")
	if w.DisplayName != "Default" {
		t.Errorf("expected 'Default' for empty dir, got %q", w.DisplayName)
	}
}

func TestCVUpdateWorkspaceWidgetWithDir(t *testing.T) {
	m := setupReadyModel()
	w := &widget.WorkspaceWidget{}
	m.workspaceWidget = w

	m.updateWorkspaceWidget("/home/user/Projects/myapp")
	if w.DisplayName != "myapp" {
		t.Errorf("expected 'myapp', got %q", w.DisplayName)
	}
}

// ============================================================================
// isWordSep tests
// ============================================================================

func TestCVIsWordSep(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{' ', true},
		{'\t', true},
		{'.', true},
		{'(', true},
		{')', true},
		{'a', false},
		{'Z', false},
		{'0', false},
		{'_', false},
	}
	for _, tt := range tests {
		got := isWordSep(tt.r)
		if got != tt.want {
			t.Errorf("isWordSep(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

// ============================================================================
// CopySnapshot tests
// ============================================================================

func TestCVCopySnapshotScrollbackLen(t *testing.T) {
	var nilSnap *CopySnapshot
	if nilSnap.ScrollbackLen() != 0 {
		t.Error("nil CopySnapshot.ScrollbackLen() should be 0")
	}

	snap := &CopySnapshot{
		Scrollback: make([][]terminal.ScreenCell, 5),
	}
	if snap.ScrollbackLen() != 5 {
		t.Errorf("expected ScrollbackLen=5, got %d", snap.ScrollbackLen())
	}
}

func TestCVCopySnapshotScrollbackLine(t *testing.T) {
	var nilSnap *CopySnapshot
	if nilSnap.ScrollbackLine(0) != nil {
		t.Error("nil CopySnapshot.ScrollbackLine should return nil")
	}

	snap := &CopySnapshot{
		Scrollback: [][]terminal.ScreenCell{
			{{Content: "A", Width: 1}},
			{{Content: "B", Width: 1}},
		},
	}

	if snap.ScrollbackLine(-1) != nil {
		t.Error("negative offset should return nil")
	}
	if snap.ScrollbackLine(2) != nil {
		t.Error("out-of-range offset should return nil")
	}
	line := snap.ScrollbackLine(0)
	if len(line) != 1 || line[0].Content != "A" {
		t.Errorf("expected line with 'A', got %v", line)
	}
}

func TestCVCaptureCopySnapshotNilTerm(t *testing.T) {
	snap := captureCopySnapshot("test", nil)
	if snap != nil {
		t.Error("expected nil snapshot for nil terminal")
	}
}

func TestCVCaptureCopySnapshotWithTerm(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"SNAP"}, 20, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	snap := captureCopySnapshot("win-snap", term)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.WindowID != "win-snap" {
		t.Errorf("expected WindowID 'win-snap', got %q", snap.WindowID)
	}
	if snap.Width <= 0 || snap.Height <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", snap.Width, snap.Height)
	}
	if len(snap.Screen) == 0 {
		t.Error("expected non-empty screen in snapshot")
	}
}

// ============================================================================
// findMatchingBracket edge case tests
// ============================================================================

func TestCVFindMatchingBracketOutOfBounds(t *testing.T) {
	lines := []string{"hello"}

	// y out of bounds.
	ry, rx := findMatchingBracket(lines, -1, 0)
	if ry != -1 || rx != -1 {
		t.Errorf("expected (-1,-1) for y<0, got (%d,%d)", ry, rx)
	}

	// x out of bounds.
	ry, rx = findMatchingBracket(lines, 0, 100)
	if ry != -1 || rx != -1 {
		t.Errorf("expected (-1,-1) for x out of bounds, got (%d,%d)", ry, rx)
	}

	// Not a bracket character.
	ry, rx = findMatchingBracket(lines, 0, 0) // 'h'
	if ry != -1 || rx != -1 {
		t.Errorf("expected (-1,-1) for non-bracket char, got (%d,%d)", ry, rx)
	}
}

func TestCVFindMatchingBracketUnmatched(t *testing.T) {
	lines := []string{"(unclosed"}
	ry, rx := findMatchingBracket(lines, 0, 0)
	if ry != -1 || rx != -1 {
		t.Errorf("expected (-1,-1) for unmatched bracket, got (%d,%d)", ry, rx)
	}
}

// ============================================================================
// renderWindowHints tests
// ============================================================================

func TestCVRenderWindowHintsCopyMode(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Hints", geometry.Rect{X: 0, Y: 0, Width: 50, Height: 15}, nil)
	w.Focused = true
	w.Visible = true

	// Render chrome first for the bottom border.
	renderWindowChrome(buf, w, theme, window.HitNone)

	kb := config.DefaultKeyBindings()
	renderWindowHints(buf, w, theme, ModeCopy, kb, false, "")

	borderY := w.Rect.Bottom() - 1
	var row strings.Builder
	for x := w.Rect.X; x < w.Rect.Right(); x++ {
		if x < buf.Width {
			row.WriteRune(buf.Cells[borderY][x].Char)
		}
	}
	rendered := row.String()
	if !strings.Contains(rendered, "search") && !strings.Contains(rendered, "select") {
		t.Errorf("expected copy mode hints, got %q", rendered)
	}
}

func TestCVRenderWindowHintsTerminalMode(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Hints", geometry.Rect{X: 0, Y: 0, Width: 50, Height: 15}, nil)
	w.Focused = true
	w.Visible = true

	renderWindowChrome(buf, w, theme, window.HitNone)

	kb := config.DefaultKeyBindings()
	renderWindowHints(buf, w, theme, ModeTerminal, kb, false, "")

	borderY := w.Rect.Bottom() - 1
	var row strings.Builder
	for x := w.Rect.X; x < w.Rect.Right(); x++ {
		if x < buf.Width {
			row.WriteRune(buf.Cells[borderY][x].Char)
		}
	}
	rendered := row.String()
	if !strings.Contains(rendered, "prefix") && !strings.Contains(rendered, "PREFIX") {
		t.Errorf("expected terminal mode hints with prefix, got %q", rendered)
	}
}

func TestCVRenderWindowHintsNilWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)
	kb := config.DefaultKeyBindings()

	// Should not panic with nil window.
	renderWindowHints(buf, nil, theme, ModeNormal, kb, false, "")
}

func TestCVRenderWindowHintsMaximizedNoHints(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Max", geometry.Rect{X: 0, Y: 0, Width: 60, Height: 20}, nil)
	w.Focused = true
	w.Visible = true
	w.Resizable = true
	w.PreMaxRect = &geometry.Rect{X: 5, Y: 5, Width: 30, Height: 10} // maximized

	kb := config.DefaultKeyBindings()
	renderWindowHints(buf, w, theme, ModeNormal, kb, false, "")

	// Maximized windows have no bottom border, so no hints should be drawn.
	// Just ensure no panic.
}

func TestCVRenderWindowHintsNarrowWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)

	w := window.NewWindow("w1", "Narrow", geometry.Rect{X: 0, Y: 0, Width: 15, Height: 8}, nil)
	w.Focused = true
	w.Visible = true

	kb := config.DefaultKeyBindings()
	// Window width < 20 should return early.
	renderWindowHints(buf, w, theme, ModeNormal, kb, false, "")
}

// ============================================================================
// renderExitedOverlay additional tests
// ============================================================================

func TestCVRenderExitedOverlayWithWindow(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w := window.NewWindow("w1", "Exited", geometry.Rect{X: 5, Y: 3, Width: 50, Height: 16}, nil)
	w.Focused = true
	w.Exited = true
	wm.AddWindow(w)

	// RenderFrame should draw the exited overlay.
	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	cr := w.ContentRect()
	overlayY := cr.Y + cr.Height/2

	var row strings.Builder
	for x := cr.X; x < cr.X+cr.Width && x < buf.Width; x++ {
		row.WriteRune(buf.Cells[overlayY][x].Char)
	}
	rendered := row.String()
	if !strings.Contains(rendered, "restart") {
		t.Errorf("expected exited overlay message, got %q", rendered)
	}
}

// ============================================================================
// renderCopyCursor tests
// ============================================================================

func TestCVRenderCopyCursor(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	// Cursor at (0,0) with no scrollback/scroll offset.
	renderCopyCursor(buf, contentRect, 0, 0, 0, 0, 5, theme)

	// Cell at (1,1) should have accent color.
	c := theme.C()
	cell := buf.Cells[1][1]
	if !colorsEqual(cell.Bg, c.AccentColor) {
		t.Error("expected accent color at cursor position")
	}
}

func TestCVRenderCopyCursorOutOfBounds(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	// Cursor at out-of-bounds position should be no-op.
	renderCopyCursor(buf, contentRect, 100, 100, 0, 0, 5, theme)
	// Should not panic.
}

// ============================================================================
// renderSelection tests
// ============================================================================

func TestCVRenderSelection(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	// Fill content area with known colors.
	for dy := 0; dy < contentRect.Height; dy++ {
		for dx := 0; dx < contentRect.Width; dx++ {
			buf.SetCell(contentRect.X+dx, contentRect.Y+dy, 'X',
				hexToColor("#FFFFFF"), hexToColor("#000000"), 0)
		}
	}

	start := geometry.Point{X: 0, Y: 0}
	end := geometry.Point{X: 5, Y: 0}

	renderSelection(buf, contentRect, start, end, 0, 0, 5)

	// Cells in selection range should have inverted fg/bg.
	for dx := 0; dx <= 5; dx++ {
		bx := contentRect.X + dx
		by := contentRect.Y
		cell := buf.Cells[by][bx]
		// After inversion: fg should be the old bg (#000000) and bg should be old fg (#FFFFFF).
		if colorsEqual(cell.Fg, hexToColor("#000000")) && colorsEqual(cell.Bg, hexToColor("#FFFFFF")) {
			continue // correct
		}
		t.Errorf("cell at (%d,%d) should be inverted", bx, by)
		break
	}
}

func TestCVRenderSelectionReversed(t *testing.T) {
	buf := NewBuffer(30, 10, "#000000")
	contentRect := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}

	// Reversed selection (end before start).
	start := geometry.Point{X: 10, Y: 2}
	end := geometry.Point{X: 0, Y: 0}

	// Should normalize and not panic.
	renderSelection(buf, contentRect, start, end, 0, 0, 5)
}

// ============================================================================
// isAppStateCapable / isVimEditor tests
// ============================================================================

func TestCVIsAppStateCapableAdditional(t *testing.T) {
	if !isAppStateCapable("/path/to/termdesk-snake") {
		t.Error("expected termdesk-snake to be app state capable")
	}
	if isAppStateCapable("vim") {
		t.Error("vim should not be app state capable")
	}
}

func TestCVIsVimEditorAdditional(t *testing.T) {
	if !isVimEditor("/usr/local/bin/nvim") {
		t.Error("expected /usr/local/bin/nvim to be vim editor")
	}
	if isVimEditor("emacs") {
		t.Error("emacs should not be vim editor")
	}
}

// ============================================================================
// vimSessionPath / vimSessionsDir additional tests
// ============================================================================

func TestCVVimSessionPathEmptyHome(t *testing.T) {
	t.Setenv(envHomeDir, "")
	// On most systems, UserHomeDir will work, so this won't return empty.
	// But if it did, the path should be empty.
	path := vimSessionPath("test")
	// Just ensure no panic; path may or may not be empty.
	_ = path
}

// ============================================================================
// saveWorkspaceNow coverage
// ============================================================================

func TestCVSaveWorkspaceNowWithNotification(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{ProjectDir: tmpDir}

	m.saveWorkspaceNow()
	time.Sleep(100 * time.Millisecond)

	// Should have pushed a notification.
	if len(m.notifications.HistoryItems()) == 0 {
		t.Error("expected notification after saveWorkspaceNow")
	}

	// Check notification mentions the workspace name.
	found := false
	for _, n := range m.notifications.HistoryItems() {
		if strings.Contains(n.Body, filepath.Base(tmpDir)) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected notification to mention workspace directory name")
	}
}

// ============================================================================
// renderCopySearchBar tests
// ============================================================================

func TestCVRenderCopySearchBar(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Search", geometry.Rect{X: 0, Y: 0, Width: 50, Height: 15}, nil)
	w.Focused = true
	w.Visible = true

	renderCopySearchBar(buf, w, theme, "hello", 1, 2, 5)

	// Check that search bar content appears in the buffer.
	cr := w.ContentRect()
	var row strings.Builder
	for x := 0; x < buf.Width; x++ {
		if cr.Y < buf.Height {
			row.WriteRune(buf.Cells[cr.Y][x].Char)
		}
	}
	rendered := row.String()
	if !strings.Contains(rendered, "hello") {
		t.Errorf("expected search query 'hello' in search bar, got %q", rendered)
	}
	if !strings.Contains(rendered, "3/5") {
		t.Errorf("expected match count '3/5' in search bar, got %q", rendered)
	}
}

func TestCVRenderCopySearchBarReverseDir(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "RevSearch", geometry.Rect{X: 0, Y: 0, Width: 50, Height: 15}, nil)
	w.Focused = true
	w.Visible = true

	renderCopySearchBar(buf, w, theme, "rev", -1, 0, 3)

	// Should display '?' prefix for reverse search.
	cr := w.ContentRect()
	var row strings.Builder
	for x := 0; x < buf.Width; x++ {
		if cr.Y < buf.Height {
			row.WriteRune(buf.Cells[cr.Y][x].Char)
		}
	}
	rendered := row.String()
	if !strings.Contains(rendered, "?") {
		t.Errorf("expected '?' prefix for reverse search, got %q", rendered)
	}
}

func TestCVRenderCopySearchBarNilWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	// Should not panic.
	renderCopySearchBar(buf, nil, theme, "test", 1, 0, 0)
}

func TestCVRenderCopySearchBarEmptyQuery(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Empty", geometry.Rect{X: 0, Y: 0, Width: 50, Height: 15}, nil)
	w.Focused = true
	w.Visible = true

	// Empty query should still render the bar (with cursor block).
	renderCopySearchBar(buf, w, theme, "", 1, 0, 0)
}

// ============================================================================
// renderScrollbar with snapshot
// ============================================================================

func TestCVRenderScrollbarWithSnapshot(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"SB"}, 20, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	done := make(chan struct{})
	go func() {
		term.ReadPtyLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	snap := &CopySnapshot{
		WindowID:   "w1",
		Scrollback: make([][]terminal.ScreenCell, 100), // 100 lines of scrollback
		Screen:     make([][]terminal.ScreenCell, 10),
		Width:      20,
		Height:     10,
	}

	buf := NewBuffer(40, 20, "#000000")
	w := window.NewWindow("w1", "Scroll", geometry.Rect{X: 1, Y: 1, Width: 22, Height: 14}, nil)
	w.Focused = true
	w.Visible = true

	renderScrollbarWithSnapshot(buf, w, theme, term, 50, snap)

	// Scrollbar should be drawn on the right border.
	trackX := w.Rect.Right() - 1
	trackTop := w.Rect.Y + 1
	scrollbarDrawn := false
	for dy := trackTop; dy < w.Rect.Bottom()-2; dy++ {
		ch := buf.Cells[dy][trackX].Char
		if ch == '\u2593' || ch == '\u2591' { // ▓ or ░
			scrollbarDrawn = true
			break
		}
	}
	if !scrollbarDrawn {
		t.Error("expected scrollbar to be drawn with snapshot scrollback")
	}
}
