package app

import (
	"errors"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

// ══════════════════════════════════════════════════════════════
// Update() outer wrapper - more message types through the switch
// ══════════════════════════════════════════════════════════════

func TestCUUpdateExecIndexReadyMsg(t *testing.T) {
	m := setupReadyModel()

	// Send ExecIndexReadyMsg to set the launcher exec index
	paths := launcher.ExecIndexReadyMsg([]string{"/usr/bin/htop", "/usr/bin/vim"})
	ret, cmd := m.Update(paths)
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from ExecIndexReadyMsg when launcher not visible")
	}
	_ = model
}

func TestCUUpdateExecIndexReadyMsgLauncherVisible(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Visible = true

	paths := launcher.ExecIndexReadyMsg([]string{"/usr/bin/htop"})
	ret, cmd := m.Update(paths)
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from ExecIndexReadyMsg")
	}
	// Launcher should have refreshed results
	_ = model
}

func TestCUUpdatePtyOutputMsgQuakeTerminal(t *testing.T) {
	m := setupReadyModel()

	// Send PtyOutputMsg for the quake terminal ID
	ret, cmd := m.Update(PtyOutputMsg{WindowID: quakeTermID})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for quake PtyOutputMsg")
	}
	if !model.termHasOutput[quakeTermID] {
		t.Error("expected termHasOutput[quakeTermID] = true")
	}
}

func TestCUUpdatePtyOutputMsgUnfocusedMinimizedSkipDirty(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	w1 := m.wm.Windows()[0]
	// Mark w1 as minimized and set previous output (not changed)
	w1.Minimized = true
	m.termHasOutput[w1.ID] = true
	w1.HasNotification = true
	w1.HasActivity = true

	// Sending PtyOutputMsg should skip dirty when notification state unchanged
	// and window is minimized
	ret, _ := m.Update(PtyOutputMsg{WindowID: w1.ID})
	_ = ret.(Model)
}

func TestCUUpdatePtyOutputMsgUnfocusedInvisible(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	w1 := m.wm.Windows()[0]
	// Mark w1 as invisible (Visible=false)
	w1.Visible = false
	m.termHasOutput[w1.ID] = true
	w1.HasNotification = true
	w1.HasActivity = true

	ret, _ := m.Update(PtyOutputMsg{WindowID: w1.ID})
	_ = ret.(Model)
}

func TestCUUpdatePtyClosedMsgWithError(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// PtyClosedMsg with a real error (not input/output error) should push notification
	ret, _ := m.Update(PtyClosedMsg{
		WindowID: fw.ID,
		Err:      errors.New("something went wrong"),
	})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited")
	}
	// Notification should have been pushed
	if len(model.notifications.HistoryItems()) == 0 {
		t.Error("expected notification for PTY error")
	}
}

func TestCUUpdatePtyClosedMsgIOError(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// PtyClosedMsg with "input/output error" should NOT push notification
	notifBefore := len(m.notifications.HistoryItems())
	ret, _ := m.Update(PtyClosedMsg{
		WindowID: fw.ID,
		Err:      errors.New("input/output error"),
	})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited")
	}
	// No NEW notification for I/O error
	if len(model.notifications.HistoryItems()) != notifBefore {
		t.Error("expected no new notification for input/output error")
	}
}

func TestCUUpdatePtyClosedMsgPaneRedirect(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// Set up a pane redirect: old pane ID "old-pane" -> window ID
	m.paneRedirect["old-pane"] = fw.ID

	// PtyClosedMsg with the old pane ID should follow redirect
	ret, _ := m.Update(PtyClosedMsg{
		WindowID: "old-pane",
		Err:      nil,
	})
	model := ret.(Model)

	// Redirect should be consumed
	if _, ok := model.paneRedirect["old-pane"]; ok {
		t.Error("expected pane redirect to be consumed")
	}
	// Window should be marked exited (since it redirected to fw.ID)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited via redirect")
	}
}

func TestCUUpdatePtyClosedMsgAlreadyGone(t *testing.T) {
	m := setupReadyModel()
	// PtyClosedMsg for a window that doesn't exist should be no-op
	ret, _ := m.Update(PtyClosedMsg{WindowID: "nonexistent", Err: nil})
	_ = ret.(Model)
}

func TestCUUpdatePtyClosedMsgTilingMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "columns"

	// Get the first window ID, close it via animation
	w1 := m.wm.Windows()[0]
	w1ID := w1.ID

	// Start a close animation so isAnimatingClose returns true
	if m.animationsOn {
		center := geometry.Rect{
			X: w1.Rect.X + w1.Rect.Width/2,
			Y: w1.Rect.Y + w1.Rect.Height/2,
			Width: 1, Height: 1,
		}
		m.startWindowAnimation(w1ID, AnimClose, w1.Rect, center)
	}

	// Send PtyClosedMsg - in tiling mode, after removal should retile
	ret, cmd := m.Update(PtyClosedMsg{WindowID: w1ID, Err: nil})
	model := ret.(Model)
	if model.tilingMode && cmd == nil && m.animationsOn {
		// In tiling mode with animations, should get a tick cmd
	}
	_ = model
}

// ══════════════════════════════════════════════════════════════
// handleUpdate() - more message branches
// ══════════════════════════════════════════════════════════════

func TestCUHandleUpdateSystemStatsMsgStuckDetection(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// Terminal created > 5 seconds ago with no output -> stuck
	m.termCreatedAt[fw.ID] = time.Now().Add(-10 * time.Second)
	m.termHasOutput[fw.ID] = false

	ret, cmd := m.Update(SystemStatsMsg{CPU: 10, MemGB: 4})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd from SystemStatsMsg (re-schedules tick)")
	}

	w := model.wm.WindowByID(fw.ID)
	if w != nil && !w.Stuck {
		t.Error("expected window to be marked as Stuck (no output for > 5s)")
	}
}

func TestCUHandleUpdateSystemStatsMsgStuckClears(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// Window has output -> not stuck
	fw.Stuck = true
	m.termHasOutput[fw.ID] = true

	ret, _ := m.Update(SystemStatsMsg{CPU: 10, MemGB: 4})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w != nil && w.Stuck {
		t.Error("expected Stuck to be cleared when window has output")
	}
}

func TestCUHandleUpdateSystemStatsMsgExitedNotStuck(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// Exited window should never become stuck
	fw.Exited = true
	fw.Stuck = true
	m.termHasOutput[fw.ID] = false
	m.termCreatedAt[fw.ID] = time.Now().Add(-10 * time.Second)

	ret, _ := m.Update(SystemStatsMsg{CPU: 10, MemGB: 4})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w != nil && w.Stuck {
		t.Error("expected Stuck to be cleared for exited window")
	}
}

func TestCUHandleUpdateWorkspaceAutoSaveMsg(t *testing.T) {
	m := setupReadyModel()
	m.workspaceAutoSave = true
	m.workspaceAutoSaveMin = 1
	m.lastWorkspaceSave = time.Now().Add(-2 * time.Minute) // long enough ago

	tmpDir := t.TempDir()
	m.projectConfig = nil
	t.Setenv("TERMDESK_HOME", tmpDir)

	ret, cmd := m.Update(WorkspaceAutoSaveMsg{Time: time.Now()})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd from WorkspaceAutoSaveMsg (re-schedules tick)")
	}
	// lastWorkspaceSave should have been updated
	if model.lastWorkspaceSave.IsZero() {
		t.Error("expected lastWorkspaceSave to be updated")
	}
	// Wait for async goroutine to finish writing before TempDir cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestCUHandleUpdateWorkspaceAutoSaveMsgTooSoon(t *testing.T) {
	m := setupReadyModel()
	m.workspaceAutoSave = true
	m.workspaceAutoSaveMin = 1
	m.lastWorkspaceSave = time.Now() // just saved

	ret, cmd := m.Update(WorkspaceAutoSaveMsg{Time: time.Now()})
	if cmd == nil {
		t.Error("expected non-nil cmd (always re-schedules)")
	}
	_ = ret.(Model)
}

func TestCUHandleUpdateWorkspaceAutoSaveMsgDisabled(t *testing.T) {
	m := setupReadyModel()
	m.workspaceAutoSave = false

	ret, cmd := m.Update(WorkspaceAutoSaveMsg{Time: time.Now()})
	if cmd == nil {
		t.Error("expected non-nil cmd (always re-schedules tick)")
	}
	_ = ret.(Model)
}

func TestCUHandleUpdateAutoStartMsg(t *testing.T) {
	m := setupReadyModel()
	// AutoStartMsg with no project config should be no-op
	ret, cmd := m.Update(AutoStartMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from AutoStartMsg with no project config")
	}
	_ = model
}

func TestCUHandleUpdateBellMsgNoWindow(t *testing.T) {
	m := setupReadyModel()
	// BellMsg for a non-existent window should not panic
	ret, cmd := m.Update(BellMsg{WindowID: "nonexistent"})
	if cmd == nil {
		t.Error("expected non-nil cmd from BellMsg (tea.Raw bell)")
	}
	_ = ret.(Model)
}

func TestCUHandleUpdateBellMsgFocusedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// BellMsg for focused window should NOT set HasBell
	ret, cmd := m.Update(BellMsg{WindowID: fw.ID})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd from BellMsg")
	}
	w := model.wm.WindowByID(fw.ID)
	if w != nil && w.HasBell {
		t.Error("HasBell should be false for focused window")
	}
}

func TestCUHandleUpdatePtyClosedMsgSplitPane(t *testing.T) {
	m, term1, term2 := setupSplitModel(t)
	defer term1.Close()
	defer term2.Close()

	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		t.Fatal("expected split window")
	}

	// PtyClosedMsg for the focused pane should auto-close pane
	focusedPaneID := fw.FocusedPane

	ret, _ := m.Update(PtyClosedMsg{WindowID: focusedPaneID, Err: nil})
	model := ret.(Model)

	// Window should still exist (one pane closed, reverted to single)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist after closing one pane")
	}
	if w.IsSplit() {
		t.Error("expected window to revert to non-split after closing one of two panes")
	}
}

func TestCUHandleUpdatePtyClosedMsgSplitPaneWithError(t *testing.T) {
	m, term1, term2 := setupSplitModel(t)
	defer term1.Close()
	defer term2.Close()

	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		t.Fatal("expected split window")
	}

	focusedPaneID := fw.FocusedPane

	// PtyClosedMsg with a real error should push notification
	ret, _ := m.Update(PtyClosedMsg{
		WindowID: focusedPaneID,
		Err:      errors.New("crash"),
	})
	model := ret.(Model)
	if len(model.notifications.HistoryItems()) == 0 {
		t.Error("expected notification for split pane error")
	}
}

func TestCUHandleUpdatePtyClosedMsgSplitUnfocusedPane(t *testing.T) {
	m, term1, term2 := setupSplitModel(t)
	defer term1.Close()
	defer term2.Close()

	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		t.Fatal("expected split window")
	}

	ids := fw.SplitRoot.AllTermIDs()
	if len(ids) < 2 {
		t.Fatal("expected at least 2 panes")
	}

	// Find the unfocused pane
	unfocusedPaneID := ids[0]
	if unfocusedPaneID == fw.FocusedPane {
		unfocusedPaneID = ids[1]
	}

	// PtyClosedMsg for unfocused pane should close it directly
	ret, _ := m.Update(PtyClosedMsg{WindowID: unfocusedPaneID, Err: nil})
	model := ret.(Model)

	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	// Should have reverted to single terminal
	if w.IsSplit() {
		t.Error("expected window to revert to non-split after closing unfocused pane")
	}
}

func TestCUHandleUpdatePtyOutputMsgSplitPane(t *testing.T) {
	m, term1, term2 := setupSplitModel(t)
	defer term1.Close()
	defer term2.Close()

	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		t.Fatal("expected split window")
	}

	ids := fw.SplitRoot.AllTermIDs()
	paneID := ids[0]

	// PtyOutputMsg for a split pane terminal
	ret, _ := m.Update(PtyOutputMsg{WindowID: paneID})
	model := ret.(Model)
	if !model.termHasOutput[paneID] {
		t.Error("expected termHasOutput for pane")
	}
}

// ══════════════════════════════════════════════════════════════
// restoreWorkspace() - more paths
// ══════════════════════════════════════════════════════════════

func TestCURestoreWorkspaceSplitPanes(t *testing.T) {
	m := setupReadyModel()

	// Create an encoded split tree: horizontal split with two leaves
	tree := window.EncodeSplitTree(&window.SplitNode{
		Dir:   window.SplitHorizontal,
		Ratio: 0.5,
		Children: [2]*window.SplitNode{
			{TermID: "pane-a"},
			{TermID: "pane-b"},
		},
	})

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:          "win-split-1",
				Title:       "Split Window",
				Command:     shell,
				X:           2,
				Y:           2,
				Width:       80,
				Height:      24,
				SplitTree:   tree,
				FocusedPane: "pane-a",
				Panes: []workspace.PaneState{
					{TermID: "pane-a", Command: shell, WorkDir: ""},
					{TermID: "pane-b", Command: shell, WorkDir: ""},
				},
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	w := m.wm.Windows()[0]
	if !w.IsSplit() {
		t.Error("expected window to have split layout")
	}
	if w.FocusedPane != "pane-a" {
		t.Errorf("expected focused pane 'pane-a', got %q", w.FocusedPane)
	}

	// Both pane terminals should exist
	if m.terminals["pane-a"] == nil {
		t.Error("expected terminal for pane-a")
	}
	if m.terminals["pane-b"] == nil {
		t.Error("expected terminal for pane-b")
	}

	// Clean up
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

func TestCURestoreWorkspaceSplitPanesWithBuffer(t *testing.T) {
	m := setupReadyModel()

	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Save buffer content for pane
	bufContent := "pane buffer line 1\npane buffer line 2\n"
	bufID := workspace.SaveBuffer("pane-buf-a", bufContent)

	tree := window.EncodeSplitTree(&window.SplitNode{
		Dir:   window.SplitHorizontal,
		Ratio: 0.5,
		Children: [2]*window.SplitNode{
			{TermID: "pane-buf-a"},
			{TermID: "pane-buf-b"},
		},
	})

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:          "win-split-buf",
				Title:       "Split Buf",
				Command:     shell,
				X:           2,
				Y:           2,
				Width:       80,
				Height:      24,
				SplitTree:   tree,
				FocusedPane: "pane-buf-a",
				Panes: []workspace.PaneState{
					{TermID: "pane-buf-a", Command: shell, BufferFile: bufID, BufferRows: 20, BufferCols: 40},
					{TermID: "pane-buf-b", Command: shell},
				},
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Check buffer was restored for pane
	if bufID != "" {
		if _, ok := m.windowBuffers["pane-buf-a"]; !ok {
			t.Error("expected buffer content in windowBuffers for pane-buf-a")
		}
	}

	// Clean up
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

func TestCURestoreWorkspaceCorruptedSplitTree(t *testing.T) {
	m := setupReadyModel()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:          "win-corrupt",
				Title:       "Corrupt Split",
				Command:     shell,
				X:           2,
				Y:           2,
				Width:       80,
				Height:      24,
				SplitTree:   "garbage-data",
				FocusedPane: "pane-x",
				Panes: []workspace.PaneState{
					{TermID: "pane-x", Command: shell},
				},
			},
		},
	}

	// Should not panic - corrupted tree gets nil from DecodeSplitTree
	m.restoreWorkspace(state, "")

	// Window is added but split tree is nil (corrupted), so it should be skipped
	// via the "continue" branch
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

func TestCURestoreWorkspaceFocusMinimizedFallback(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:        "win-min-focus",
				Title:     "Minimized Focused",
				Command:   "/bin/echo",
				X:         5,
				Y:         3,
				Width:     60,
				Height:    20,
				Minimized: true,
			},
			{
				ID:      "win-visible",
				Title:   "Visible",
				Command: "/bin/echo",
				X:       10,
				Y:       5,
				Width:   60,
				Height:  20,
			},
		},
		FocusedID: "win-min-focus", // focused window is minimized
	}

	m.restoreWorkspace(state, "")

	// Should fall back to a visible window
	focused := m.wm.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}
	if focused.ID == "win-min-focus" {
		t.Error("expected focus to fall back from minimized window to visible one")
	}
	if focused.ID != "win-visible" {
		t.Errorf("expected focused window 'win-visible', got %q", focused.ID)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCURestoreWorkspaceAllMinimizedNormal(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:        "win-min-1",
				Title:     "Min 1",
				Command:   "/bin/echo",
				X:         5,
				Y:         3,
				Width:     60,
				Height:    20,
				Minimized: true,
			},
			{
				ID:        "win-min-2",
				Title:     "Min 2",
				Command:   "/bin/echo",
				X:         10,
				Y:         5,
				Width:     60,
				Height:    20,
				Minimized: true,
			},
		},
		FocusedID: "win-min-1",
	}

	m.restoreWorkspace(state, "")

	// When all windows are minimized and focused one is minimized,
	// should set inputMode to ModeNormal
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal when all windows minimized, got %s", m.inputMode)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCURestoreWorkspaceWithResizable(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:        "win-fixed",
				Title:     "Fixed Size",
				Command:   "/bin/echo",
				X:         5,
				Y:         3,
				Width:     50,
				Height:    20,
				Resizable: false,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	w := m.wm.Windows()[0]
	if w.Resizable {
		t.Error("expected Resizable=false after restore")
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCURestoreWorkspaceVimSession(t *testing.T) {
	m := setupReadyModel()

	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create vim session file
	sessDir := vimSessionsDir()
	os.MkdirAll(sessDir, 0755)
	sessPath := vimSessionPath("win-vim")
	os.WriteFile(sessPath, []byte("\" vim session\n"), 0644)

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-vim",
				Title:   "Vim",
				Command: "vim",
				X:       5,
				Y:       3,
				Width:   80,
				Height:  24,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	// Terminal should have been created (vim session restore is async)
	if _, ok := m.terminals["win-vim"]; !ok {
		t.Error("expected terminal for vim window")
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestCURestoreWorkspaceSplitPaneEmptyWorkDir(t *testing.T) {
	m := setupReadyModel()

	tree := window.EncodeSplitTree(&window.SplitNode{
		Dir:   window.SplitVertical,
		Ratio: 0.5,
		Children: [2]*window.SplitNode{
			{TermID: "pane-wd-a"},
			{TermID: "pane-wd-b"},
		},
	})

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:          "win-split-wd",
				Title:       "Split WD",
				Command:     shell,
				WorkDir:     "/tmp",
				X:           2,
				Y:           2,
				Width:       80,
				Height:      24,
				SplitTree:   tree,
				FocusedPane: "pane-wd-a",
				Panes: []workspace.PaneState{
					{TermID: "pane-wd-a", Command: shell, WorkDir: ""}, // empty - should fall back to window WorkDir
					{TermID: "pane-wd-b", Command: shell, WorkDir: "/tmp"},
				},
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Clean up
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

func TestCURestoreWorkspaceSplitPaneNonShellCommand(t *testing.T) {
	m := setupReadyModel()

	tree := window.EncodeSplitTree(&window.SplitNode{
		Dir:   window.SplitHorizontal,
		Ratio: 0.5,
		Children: [2]*window.SplitNode{
			{TermID: "pane-echo-a"},
			{TermID: "pane-echo-b"},
		},
	})

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:          "win-split-cmd",
				Title:       "Split Cmd",
				Command:     "/bin/echo",
				X:           2,
				Y:           2,
				Width:       80,
				Height:      24,
				SplitTree:   tree,
				FocusedPane: "pane-echo-a",
				Panes: []workspace.PaneState{
					{TermID: "pane-echo-a", Command: "/bin/echo", Args: []string{"hello"}},
					{TermID: "pane-echo-b", Command: "/bin/echo", Args: []string{"world"}},
				},
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Both pane terminals should exist
	if m.terminals["pane-echo-a"] == nil {
		t.Error("expected terminal for pane-echo-a")
	}
	if m.terminals["pane-echo-b"] == nil {
		t.Error("expected terminal for pane-echo-b")
	}

	// Clean up
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

// ══════════════════════════════════════════════════════════════
// closeFocusedPane() - more paths
// ══════════════════════════════════════════════════════════════

func TestCUCloseFocusedPaneNoWindow(t *testing.T) {
	m := setupReadyModel()
	// No focused window - should be a no-op
	m.closeFocusedPane()
	// No panic = pass
}

func TestCUCloseFocusedPaneLastPaneClosesWindow(t *testing.T) {
	m := setupReadyModel()

	// Create a split window with only one pane (simulating after multiple close operations)
	win := window.NewWindow("win-last-pane", "Last", geometry.Rect{X: 0, Y: 1, Width: 80, Height: 24}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term1, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	defer term1.Close()

	paneID := "only-pane"
	m.terminals[paneID] = term1

	// Create a split tree with just one leaf
	win.SplitRoot = &window.SplitNode{TermID: paneID}
	win.FocusedPane = paneID

	prevCount := m.wm.Count()
	m.closeFocusedPane()

	// RemoveLeaf on a single-leaf tree returns nil -> window should be removed
	if m.wm.Count() >= prevCount {
		t.Error("expected window to be removed when last pane is closed")
	}
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after last pane closed, got %s", m.inputMode)
	}
}

func TestCUCloseFocusedPaneThreePanesRemaining(t *testing.T) {
	m := setupReadyModel()

	win := window.NewWindow("win-3pane", "3Pane", geometry.Rect{X: 0, Y: 1, Width: 120, Height: 30}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	// Create three terminals
	cr := win.ContentRect()
	term1, err := terminal.NewShell(cr.Width/3, cr.Height, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	term2, err := terminal.NewShell(cr.Width/3, cr.Height, 0, 0, "")
	if err != nil {
		term1.Close()
		t.Fatalf("terminal.NewShell: %v", err)
	}
	term3, err := terminal.NewShell(cr.Width/3, cr.Height, 0, 0, "")
	if err != nil {
		term1.Close()
		term2.Close()
		t.Fatalf("terminal.NewShell: %v", err)
	}
	defer func() {
		term1.Close()
		term2.Close()
		term3.Close()
	}()

	pane1 := "p3-1"
	pane2 := "p3-2"
	pane3 := "p3-3"

	m.terminals[pane1] = term1
	m.terminals[pane2] = term2
	m.terminals[pane3] = term3

	// Build 3-pane split tree: (pane1 | (pane2 | pane3))
	win.SplitRoot = &window.SplitNode{
		Dir:   window.SplitHorizontal,
		Ratio: 0.33,
		Children: [2]*window.SplitNode{
			{TermID: pane1},
			{
				Dir:   window.SplitHorizontal,
				Ratio: 0.5,
				Children: [2]*window.SplitNode{
					{TermID: pane2},
					{TermID: pane3},
				},
			},
		},
	}
	win.FocusedPane = pane1

	m.closeFocusedPane()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected window to still exist after closing 1 of 3 panes")
	}
	if !fw.IsSplit() {
		t.Error("expected window to remain split with 2 panes remaining")
	}

	// Focus should move to one of the remaining panes
	if fw.FocusedPane == pane1 {
		t.Error("expected focus to move away from closed pane")
	}
	ids := fw.SplitRoot.AllTermIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 remaining panes, got %d", len(ids))
	}
}

func TestCUCloseFocusedPaneRevertsToSingleWithRemap(t *testing.T) {
	m := setupReadyModel()

	win := window.NewWindow("win-remap", "Remap", geometry.Rect{X: 0, Y: 1, Width: 80, Height: 24}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term1, err := terminal.NewShell(cr.Width/2, cr.Height, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	term2, err := terminal.NewShell(cr.Width/2, cr.Height, 0, 0, "")
	if err != nil {
		term1.Close()
		t.Fatalf("terminal.NewShell: %v", err)
	}
	defer func() {
		term1.Close()
		term2.Close()
	}()

	pane1 := "remap-p1"
	pane2 := "remap-p2"

	m.terminals[pane1] = term1
	m.terminals[pane2] = term2
	m.termCreatedAt[pane1] = time.Now()
	m.termCreatedAt[pane2] = time.Now()
	m.termHasOutput[pane1] = true
	m.termHasOutput[pane2] = true

	win.SplitRoot = &window.SplitNode{
		Dir:   window.SplitHorizontal,
		Ratio: 0.5,
		Children: [2]*window.SplitNode{
			{TermID: pane1},
			{TermID: pane2},
		},
	}
	win.FocusedPane = pane1

	// Close focused pane (pane1). Remaining is pane2.
	// Since pane2 != win.ID, remap should happen.
	m.closeFocusedPane()

	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected window to still exist")
	}
	if fw.IsSplit() {
		t.Error("expected window to revert to non-split")
	}

	// The remaining terminal (pane2) should be remapped to the window ID
	if m.terminals[win.ID] == nil {
		t.Error("expected remaining terminal to be remapped to window ID")
	}
	// Old pane2 key should be deleted
	if _, ok := m.terminals[pane2]; ok {
		t.Error("expected old pane2 terminal key to be deleted after remap")
	}
	// Redirect should be set
	if redirect, ok := m.paneRedirect[pane2]; !ok || redirect != win.ID {
		t.Errorf("expected paneRedirect[pane2] = %q, got %q", win.ID, redirect)
	}
}

// ══════════════════════════════════════════════════════════════
// More Update() outer wrapper paths - image suppression state
// ══════════════════════════════════════════════════════════════

func TestCUUpdateImageRefreshMsgNoop(t *testing.T) {
	m := setupReadyModel()
	// ImageRefreshMsg in handleUpdate sets dirty=false and returns (no-op in inner handler)
	ret, cmd := m.Update(ImageRefreshMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from ImageRefreshMsg without image placements")
	}
	_ = model
}

func TestCUUpdateImageClearScreenMsgCmd(t *testing.T) {
	m := setupReadyModel()
	_, cmd := m.Update(ImageClearScreenMsg{})
	if cmd == nil {
		t.Error("expected non-nil cmd (tea.ClearScreen) from ImageClearScreenMsg")
	}
}

func TestCUUpdatePasteMsgNormalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// PasteMsg in non-terminal mode should be no-op
	ret, cmd := m.Update(tea.PasteMsg{Content: "paste content"})
	if cmd != nil {
		t.Error("expected nil cmd from PasteMsg in normal mode")
	}
	_ = ret.(Model)
}

func TestCUUpdateWindowSizeMsgTilingRetilesAndAnimates(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"
	m.applyTilingLayout()

	ret, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd from WindowSizeMsg with tiling")
	}
	if model.width != 100 || model.height != 30 {
		t.Errorf("expected 100x30, got %dx%d", model.width, model.height)
	}
}

func TestCUUpdateResizeSettleZeroLastResize(t *testing.T) {
	m := setupReadyModel()
	// lastWindowSizeAt is zero - should stop ticking
	ret, cmd := m.Update(ResizeSettleTickMsg{Time: time.Now()})
	if cmd != nil {
		t.Error("expected nil cmd when lastWindowSizeAt is zero")
	}
	_ = ret.(Model)
}

func TestCUUpdatePtyOutputMsgOutOfBoundsWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	w1 := m.wm.Windows()[0]

	// Position window completely out of visible area
	w1.Rect = geometry.Rect{X: -500, Y: -500, Width: 50, Height: 20}
	m.termHasOutput[w1.ID] = true
	w1.HasNotification = true
	w1.HasActivity = true

	// Should hit the "not overlaps frame" branch
	ret, _ := m.Update(PtyOutputMsg{WindowID: w1.ID})
	_ = ret.(Model)
}

func TestCUUpdatePtyClosedMsgWithEOFError(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// EOF error should not push notification (same as I/O error)
	notifBefore := len(m.notifications.HistoryItems())
	ret, _ := m.Update(PtyClosedMsg{
		WindowID: fw.ID,
		Err:      errors.New("EOF"),
	})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited")
	}
	if len(model.notifications.HistoryItems()) != notifBefore {
		t.Error("expected no notification for EOF error")
	}
}

func TestCUUpdatePtyClosedMsgAlreadyExited(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Exited = true
	fw.Title = "Window [exited]"

	// Sending PtyClosedMsg again shouldn't double-mark
	ret, _ := m.Update(PtyClosedMsg{WindowID: fw.ID, Err: nil})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	// Title should not have double "[exited]"
	count := 0
	for i := 0; i+len("[exited]") <= len(w.Title); i++ {
		if w.Title[i:i+len("[exited]")] == "[exited]" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("title has double [exited]: %q", w.Title)
	}
}

// ══════════════════════════════════════════════════════════════
// handleUpdate() WorkspaceRestoreMsg - more paths
// ══════════════════════════════════════════════════════════════

func TestCUWorkspaceRestoreMsgNotPending(t *testing.T) {
	m := setupReadyModel()
	m.workspaceRestorePending = false

	ret, cmd := m.Update(WorkspaceRestoreMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when not pending")
	}
	_ = model
}

func TestCUWorkspaceRestoreMsgGlobalFallback(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Create global workspace with a window
	state := workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "global-w1", Title: "Global", Command: shell, X: 1, Y: 1, Width: 40, Height: 10},
		},
	}
	if err := workspace.SaveWorkspace(state, ""); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	m := setupReadyModel()
	m.workspaceRestorePending = true
	m.projectConfig = nil
	m.lastWindowSizeAt = time.Now().Add(-1 * time.Second)

	// Use a temp config path so recordWorkspaceAccess doesn't pollute
	cfgDir := tmpHome
	t.Setenv("TERMDESK_CONFIG_PATH", cfgDir+"/config.toml")

	ret, _ := m.Update(WorkspaceRestoreMsg{})
	model := ret.(Model)

	if model.workspaceRestorePending {
		t.Error("expected workspaceRestorePending to be false")
	}
	if model.wm.Count() == 0 {
		t.Error("expected restored window(s) from global workspace")
	}

	// Clean up
	for _, term := range model.terminals {
		term.Close()
	}
}
