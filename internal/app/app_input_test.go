package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/window"
)

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

	// Move down (should skip separator at index 1 and go to index 2)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	if model.menuBar.HoverIndex != 2 {
		t.Errorf("hover = %d, want 2", model.menuBar.HoverIndex)
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
	m.menuBar.OpenMenu(0)    // File menu
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
	if v.Content == "" {
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
	if v.Content == "" {
		t.Error("expected rendered content with launcher")
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

// ── Prefix system tests ──

func TestPrefixKeyActivatesPending(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	// Press Ctrl+A (prefix key, default)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)

	if !model.prefixPending {
		t.Error("expected prefixPending to be true after prefix key")
	}
	if model.inputMode != ModeTerminal {
		t.Error("should still be in Terminal mode while prefix pending")
	}
}

func TestPrefixKeyActivatesPendingInCopyMode(t *testing.T) {
	m, term := setupCopyModeModel(t, "hello")
	defer term.Close()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)

	if !model.prefixPending {
		t.Error("expected prefixPending to be true after prefix key in copy mode")
	}
	if model.inputMode != ModeCopy {
		t.Error("should stay in Copy mode while prefix pending")
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
		t.Errorf("expected q->quit, got %q", am["q"])
	}
	if am["n"] != "new_terminal" {
		t.Errorf("expected n->new_terminal, got %q", am["n"])
	}
	if am["h"] != "snap_left" {
		t.Errorf("expected h->snap_left, got %q", am["h"])
	}

	// Check hardcoded alternates
	if am["ctrl+q"] != "quit" {
		t.Errorf("expected ctrl+q->quit, got %q", am["ctrl+q"])
	}
	if am["left"] != "snap_left" {
		t.Errorf("expected left->snap_left, got %q", am["left"])
	}
	if am["enter"] != "enter_terminal" {
		t.Errorf("expected enter->enter_terminal, got %q", am["enter"])
	}
}

func TestPrefixYShowsClipboard(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeTerminal
	m.prefixPending = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	m = updated.(Model)

	if !m.clipboard.Visible {
		t.Error("prefix+y should show clipboard history")
	}
}

// ── handleKeyPress: routing logic tests ──

func TestHandleKeyPressCapsLockNormalization(t *testing.T) {
	m := setupReadyModel()
	// Uppercase 'N' should be normalized to lowercase 'n' (new_terminal action)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'N', Text: "N"}))
	model := updated.(Model)
	// 'n' maps to new_terminal in Normal mode
	if model.wm.Count() != 1 {
		t.Errorf("expected 1 window from normalized 'N', got %d", model.wm.Count())
	}
}

func TestHandleKeyPressWorkspacePickerEsc(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/tmp/ws1.toml", "/tmp/ws2.toml"}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.workspacePickerVisible {
		t.Error("escape should close workspace picker")
	}
}

func TestHandleKeyPressWorkspacePickerQCloses(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/tmp/ws1.toml"}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	model := updated.(Model)
	if model.workspacePickerVisible {
		t.Error("q should close workspace picker")
	}
}

func TestHandleKeyPressWorkspacePickerNavigation(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/tmp/ws1.toml", "/tmp/ws2.toml", "/tmp/ws3.toml"}
	m.workspacePickerSelected = 0

	// Down
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model := updated.(Model)
	if model.workspacePickerSelected != 1 {
		t.Errorf("down: selected = %d, want 1", model.workspacePickerSelected)
	}

	// Down again
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = updated.(Model)
	if model.workspacePickerSelected != 2 {
		t.Errorf("j: selected = %d, want 2", model.workspacePickerSelected)
	}

	// Down wraps
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	if model.workspacePickerSelected != 0 {
		t.Errorf("down wrap: selected = %d, want 0", model.workspacePickerSelected)
	}

	// Up wraps
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	model = updated.(Model)
	if model.workspacePickerSelected != 2 {
		t.Errorf("up wrap: selected = %d, want 2", model.workspacePickerSelected)
	}

	// k moves up
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	model = updated.(Model)
	if model.workspacePickerSelected != 1 {
		t.Errorf("k: selected = %d, want 1", model.workspacePickerSelected)
	}
}

func TestHandleKeyPressModalScrollAndTabs(t *testing.T) {
	m := setupReadyModel()
	m.modal = m.helpOverlay()

	// Scroll down
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model := updated.(Model)
	if model.modal == nil {
		t.Fatal("modal should still be open")
	}
	if model.modal.ScrollY != 1 {
		t.Errorf("scroll = %d, want 1", model.modal.ScrollY)
	}

	// Scroll up
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	model = updated.(Model)
	if model.modal.ScrollY != 0 {
		t.Errorf("scroll = %d, want 0", model.modal.ScrollY)
	}

	// Tab switches tab
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.modal.ActiveTab != 1 {
		t.Errorf("active tab = %d, want 1", model.modal.ActiveTab)
	}

	// Shift+tab goes back
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)
	if model.modal.ActiveTab != 0 {
		t.Errorf("active tab = %d, want 0", model.modal.ActiveTab)
	}

	// Right moves tab forward
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.modal.ActiveTab != 1 {
		t.Errorf("active tab = %d, want 1 after right", model.modal.ActiveTab)
	}

	// Left moves tab backward
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model = updated.(Model)
	if model.modal.ActiveTab != 0 {
		t.Errorf("active tab = %d, want 0 after left", model.modal.ActiveTab)
	}

	// Number key selects tab directly
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	model = updated.(Model)
	if model.modal.ActiveTab != 1 {
		t.Errorf("active tab = %d, want 1 after pressing '2'", model.modal.ActiveTab)
	}

	// Esc closes modal
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.modal != nil {
		t.Error("escape should close modal")
	}
}

func TestHandleKeyPressConfirmDialogToggle(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{Title: "Close?", IsQuit: false}

	// Toggle button with left
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model := updated.(Model)
	if model.confirmClose.Selected != 1 {
		t.Errorf("selected = %d, want 1 after left", model.confirmClose.Selected)
	}

	// Toggle again with right
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.confirmClose.Selected != 0 {
		t.Errorf("selected = %d, want 0 after right", model.confirmClose.Selected)
	}

	// Toggle with tab
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.confirmClose.Selected != 1 {
		t.Errorf("selected = %d, want 1 after tab", model.confirmClose.Selected)
	}

	// Escape cancels
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.confirmClose != nil {
		t.Error("escape should dismiss confirm dialog")
	}
}

func TestHandleKeyPressConfirmDialogEnterOnNo(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.confirmClose = &ConfirmDialog{Title: "Close?", WindowID: fw.ID, Selected: 1}

	// Enter on No (selected=1) should dismiss
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	if model.confirmClose != nil {
		t.Error("enter on No should dismiss confirm dialog")
	}
	if model.wm.Count() != 1 {
		t.Error("window should still exist after cancel")
	}
}

func TestHandleKeyPressExposeModeNavigation(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	// Tab cycles forward
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model := updated.(Model)
	if !model.exposeMode {
		t.Error("should still be in expose mode after tab")
	}

	// Shift+tab cycles backward
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)
	if !model.exposeMode {
		t.Error("should still be in expose mode after shift+tab")
	}

	// Down also cycles
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model = updated.(Model)
	if !model.exposeMode {
		t.Error("should still be in expose mode after down")
	}

	// Up also cycles
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	model = updated.(Model)
	if !model.exposeMode {
		t.Error("should still be in expose mode after up")
	}

	// Enter exits expose
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.exposeMode {
		t.Error("enter should exit expose mode")
	}
	if model.inputMode != ModeNormal {
		t.Error("should be in normal mode after exiting expose")
	}
}

func TestHandleKeyPressExposeModeNumberSelect(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	// Press '1' to select first window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model := updated.(Model)
	if model.exposeMode {
		t.Error("number key should exit expose mode")
	}
}

func TestHandleKeyPressExposeModeFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.enterExpose()

	// Type a filter character
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	model := updated.(Model)
	if model.exposeFilter != "a" {
		t.Errorf("expose filter = %q, want 'a'", model.exposeFilter)
	}

	// Backspace removes filter character
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated.(Model)
	if model.exposeFilter != "" {
		t.Errorf("expose filter = %q, want empty after backspace", model.exposeFilter)
	}
}

func TestHandleKeyPressExposeModeEscClearsFilter(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.enterExpose()
	m.exposeFilter = "test"

	// Esc with filter active clears filter first
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.exposeFilter != "" {
		t.Errorf("expose filter = %q, want empty", model.exposeFilter)
	}
	if !model.exposeMode {
		t.Error("should still be in expose mode after clearing filter")
	}

	// Esc again exits expose
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.exposeMode {
		t.Error("second escape should exit expose mode")
	}
}

func TestHandleKeyPressCtrlQShowsConfirm(t *testing.T) {
	m := setupReadyModel()
	// Ctrl+Q in normal mode shows quit confirm
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Error("ctrl+q should show quit confirm dialog")
	}
}

func TestHandleKeyPressF1OpensHelp(t *testing.T) {
	m := setupReadyModel()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	model := updated.(Model)
	if model.modal == nil {
		t.Error("F1 should open help overlay in Normal mode")
	}
}

func TestHandleKeyPressF9TogglesExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// F9 enters expose
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF9}))
	model := updated.(Model)
	if !model.exposeMode {
		t.Error("F9 should enter expose mode")
	}
}

func TestHandleKeyPressQuickWindowCycling(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	focused1 := m.wm.FocusedWindow().ID

	// Alt+] should cycle forward (QuickNextWindow)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ']', Mod: tea.ModAlt}))
	model := updated.(Model)
	focused2 := model.wm.FocusedWindow().ID
	if focused1 == focused2 {
		t.Error("alt+] should cycle focused window")
	}

	// Alt+[ should cycle backward (QuickPrevWindow)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '[', Mod: tea.ModAlt}))
	model = updated.(Model)
	focused3 := model.wm.FocusedWindow().ID
	if focused2 == focused3 {
		t.Error("alt+[ should cycle focused window backward")
	}
}

func TestHandleKeyPressRoutesToNormalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// 'n' in normal mode creates a new terminal window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model := updated.(Model)
	if model.wm.Count() != 1 {
		t.Errorf("expected 1 window from 'n' in normal mode, got %d", model.wm.Count())
	}
}

func TestHandleKeyPressRoutesToTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal

	// In terminal mode, prefix key activates prefix pending
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.prefixPending {
		t.Error("ctrl+a should activate prefix pending in terminal mode")
	}
}

func TestHandleKeyPressPrefixPendingRouting(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Any key after prefix should be dispatched via handlePrefixAction
	// 'c' enters copy mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c"}))
	model := updated.(Model)
	if model.inputMode != ModeCopy {
		t.Errorf("prefix+c should enter copy mode, got mode %d", model.inputMode)
	}
	if model.prefixPending {
		t.Error("prefixPending should be cleared after dispatch")
	}
}

func TestHandleKeyPressRoutesToCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeCopy

	// 'q' in copy mode (without terminal) exits to Normal
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	model := updated.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after q in copy mode, got %d", model.inputMode)
	}
}

// ── handleNormalModeKey: additional tests ──

func TestNormalModeEscResetsDockFocus(t *testing.T) {
	m := setupReadyModel()
	// Esc in normal mode when not in dock nav
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.dockFocused {
		t.Error("esc should reset dock focus")
	}
}

func TestNormalModeCEntersCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c"}))
	model := updated.(Model)
	if model.inputMode != ModeCopy {
		t.Errorf("'c' should enter copy mode, got %d", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("scroll offset should be 0 when entering copy mode")
	}
}

func TestNormalModeFocusByNumber(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	windows := m.wm.Windows()
	windowIDs := []string{windows[0].ID, windows[1].ID, windows[2].ID}

	// Press '1' to focus first window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model := updated.(Model)
	fw := model.wm.FocusedWindow()
	if fw == nil || fw.ID != windowIDs[0] {
		t.Error("'1' should focus the first window")
	}

	// Press '2' to focus second window
	windows = model.wm.Windows()
	windowIDs = []string{windows[0].ID, windows[1].ID, windows[2].ID}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '2', Text: "2"}))
	model = updated.(Model)
	fw = model.wm.FocusedWindow()
	if fw == nil || fw.ID != windowIDs[1] {
		t.Error("'2' should focus the second window")
	}

	// Press '9' with only 3 windows - should not change focus
	focusBefore := fw.ID
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: '9', Text: "9"}))
	model = updated.(Model)
	fw = model.wm.FocusedWindow()
	if fw == nil || fw.ID != focusBefore {
		t.Error("'9' should not change focus when less than 9 windows exist")
	}
}

func TestNormalModePrefixKeyActivatesPending(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// Ctrl+A (prefix key) should activate prefix pending
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.prefixPending {
		t.Error("prefix key should activate prefix pending in normal mode")
	}
}

func TestNormalModeTabCyclesWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	// 3 windows: Tab cycles through all windows (bringing to front like Alt+]),
	// then enters dock → menu bar → back to windows.

	var model Model
	var updated tea.Model

	// Tab 3 times → cycles through all 3 windows (each brings to front)
	model = m
	ids := make(map[string]bool)
	ids[model.wm.FocusedWindow().ID] = true
	for i := 0; i < 3; i++ {
		updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		model = updated.(Model)
		if model.dockFocused || model.menuBarFocused {
			t.Errorf("tab %d should stay on windows, not enter dock/menubar", i+1)
		}
		ids[model.wm.FocusedWindow().ID] = true
	}
	if len(ids) < 3 {
		t.Errorf("should have visited at least 3 different windows, got %d", len(ids))
	}

	// Tab 4th time → enters dock (all windows visited)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if !model.dockFocused {
		t.Error("tab after cycling all windows should enter dock")
	}

	// Tab from dock → menu bar
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if !model.menuBarFocused {
		t.Error("tab from dock should enter menu bar")
	}

	// Tab from menu bar → back to windows
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.menuBarFocused || model.dockFocused {
		t.Error("tab from menu bar should go back to windows")
	}

	// Shift+Tab cycles backward
	prev := model.wm.FocusedWindow().ID
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)
	if model.wm.FocusedWindow().ID == prev {
		t.Error("shift+tab should cycle to different window")
	}
}

func TestNormalModeEnterEntersTerminalMode(t *testing.T) {
	m := setupReadyModel()
	// Use createTerminalWindow to get a real terminal
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"hello"}})
	m = completeAnimations(m)

	// 'enter' should enter terminal mode (enter_terminal action)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	if model.inputMode != ModeTerminal {
		t.Errorf("enter should switch to terminal mode, got %d", model.inputMode)
	}
}

func TestNormalModeSpaceLaunchesLauncher(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// 'space' toggles launcher
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "}))
	model := updated.(Model)
	if !model.launcher.Visible {
		t.Error("space should open launcher in normal mode")
	}
}

func TestNormalModeIEntersTerminal(t *testing.T) {
	m := setupReadyModel()
	// Create a real terminal window
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"test"}})
	m = completeAnimations(m)

	// 'i' is enter_terminal action
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	model := updated.(Model)
	if model.inputMode != ModeTerminal {
		t.Errorf("'i' should enter terminal mode, got %d", model.inputMode)
	}
}

func TestNormalModeDotFocusesDock(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// '.' focuses the dock
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '.', Text: "."}))
	model := updated.(Model)
	if !model.dockFocused {
		t.Error("'.' should focus the dock")
	}
}

func TestNormalModeWClosesWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	// 'w' opens close confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"}))
	model := updated.(Model)
	if model.confirmClose == nil {
		t.Error("'w' should show close confirm dialog")
	}
}

func TestNormalModeMMinimizes(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	if fw == nil {
		t.Fatal("expected focused window")
	}

	// 'm' minimizes window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}))
	model := updated.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w != nil && !w.Minimized {
		t.Error("'m' should minimize the focused window")
	}
}

func TestNormalModeXEntersExpose(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeNormal

	// 'x' enters expose mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model := updated.(Model)
	if !model.exposeMode {
		t.Error("'x' should enter expose mode")
	}
}

func TestNormalModeTTilesAll(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()

	// 't' tiles all windows
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	model := updated.(Model)
	model = completeAnimations(model)
	// After tiling, windows should have been rearranged
	windows := model.wm.Windows()
	if len(windows) != 2 {
		t.Errorf("expected 2 windows, got %d", len(windows))
	}
}

func TestNormalModeUnknownKeyNoop(t *testing.T) {
	m := setupReadyModel()
	windowsBefore := m.wm.Count()

	// An unmapped key like 'z' should be a no-op
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}))
	model := updated.(Model)
	if cmd != nil {
		t.Error("unknown key should return nil cmd")
	}
	if model.wm.Count() != windowsBefore {
		t.Error("unknown key should not create or remove windows")
	}
}

func TestNormalModeDockFocusedRouting(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal
	m.dockFocused = true
	m.dock.SetHover(0)

	// While dock is focused, keys should go to handleDockNav
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.dockFocused {
		t.Error("esc in dock nav should exit dock focus")
	}
}

// ── handleTerminalModeKey: additional tests ──

func TestTerminalModeNoTerminalFallsBackToNormal(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // demo window has no real terminal
	m.inputMode = ModeTerminal

	// Any non-prefix key should fall to "no terminal focused" and switch to Normal
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model := updated.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal when no terminal focused, got %d", model.inputMode)
	}
}

func TestTerminalModeForwardsKeyToTerminal(t *testing.T) {
	m := setupReadyModel()
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"hello"}})
	m = completeAnimations(m)
	m.inputMode = ModeTerminal

	// Any non-prefix key should be forwarded to terminal (not panic or change mode)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model := updated.(Model)
	if model.inputMode != ModeTerminal {
		t.Errorf("should stay in terminal mode after forwarding key, got %d", model.inputMode)
	}
}

func TestTerminalModePrefixActivation(t *testing.T) {
	m := setupReadyModel()
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"hello"}})
	m = completeAnimations(m)
	m.inputMode = ModeTerminal

	// Prefix key activates prefix pending
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if !model.prefixPending {
		t.Error("prefix key should activate prefix pending")
	}
	if model.inputMode != ModeTerminal {
		t.Error("should stay in terminal mode while prefix pending")
	}
}

// ── handlePrefixAction: additional tests ──

func TestPrefixCEntersCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c"}))
	model := updated.(Model)
	if model.inputMode != ModeCopy {
		t.Errorf("prefix+c should enter copy mode, got %d", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("scroll offset should be 0 when entering copy mode")
	}
}

func TestPrefixDSendsDetach(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+d sends detach OSC; should not panic
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model := updated.(Model)
	if model.prefixPending {
		t.Error("prefixPending should be cleared after prefix+d")
	}
}

func TestPrefixWindowByIndex(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	windows := m.wm.Windows()
	windowIDs := []string{windows[0].ID, windows[1].ID, windows[2].ID}
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+1 focuses first window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}))
	model := updated.(Model)
	fw := model.wm.FocusedWindow()
	if fw == nil || fw.ID != windowIDs[0] {
		t.Error("prefix+1 should focus first window")
	}
}

func TestPrefixWindowByIndexOutOfRange(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true
	focusBefore := m.wm.FocusedWindow().ID

	// prefix+9 with only 1 window should not change focus
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '9', Text: "9"}))
	model := updated.(Model)
	fw := model.wm.FocusedWindow()
	if fw == nil || fw.ID != focusBefore {
		t.Error("prefix+9 should not change focus when less than 9 windows")
	}
}

func TestPrefixUnrecognizedKeyForwardedToTerminal(t *testing.T) {
	m := setupReadyModel()
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"test"}})
	m = completeAnimations(m)
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// An unrecognized key after prefix should be forwarded to terminal
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}))
	model := updated.(Model)
	if model.prefixPending {
		t.Error("prefixPending should be cleared")
	}
	// Should not panic and should not change mode
}

func TestPrefixUnrecognizedKeyNoTerminal(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow() // demo window, no real terminal
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// With no terminal, unrecognized key should still not panic
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}))
	model := updated.(Model)
	_ = model
	_ = cmd
	// No panic means success
}

func TestPrefixNCreatesWindow(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+n creates new terminal
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model := updated.(Model)
	if model.wm.Count() != 1 {
		t.Errorf("prefix+n should create a window, got %d", model.wm.Count())
	}
}

func TestPrefixWClosesWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+w shows close confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"}))
	model := updated.(Model)
	if model.confirmClose == nil {
		t.Error("prefix+w should show close confirm dialog")
	}
	// Should exit terminal mode for dialog
	if model.inputMode != ModeNormal {
		t.Error("should switch to normal mode for confirm dialog")
	}
}

func TestPrefixHSnapsLeft(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true
	wa := m.wm.WorkArea()

	// prefix+h snaps left
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	model := updated.(Model)
	model = completeAnimations(model)
	fw := model.wm.FocusedWindow()
	if fw.Rect.Width != wa.Width/2 {
		t.Errorf("snap left: width = %d, want %d", fw.Rect.Width, wa.Width/2)
	}
	// Should stay in terminal mode (geometry action)
	if model.inputMode != ModeTerminal {
		t.Error("geometry action should stay in terminal mode")
	}
}

func TestPrefixLSnapsRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true
	wa := m.wm.WorkArea()

	// prefix+l snaps right
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}))
	model := updated.(Model)
	model = completeAnimations(model)
	fw := model.wm.FocusedWindow()
	if fw.Rect.X != wa.Width/2 {
		t.Errorf("snap right: x = %d, want %d", fw.Rect.X, wa.Width/2)
	}
}

func TestPrefixKMaximizes(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	model := updated.(Model)
	model = completeAnimations(model)
	fw := model.wm.FocusedWindow()
	if !fw.IsMaximized() {
		t.Error("prefix+k should maximize window")
	}
}

func TestPrefixJRestores(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origRect := fw.Rect

	// First maximize
	m.inputMode = ModeTerminal
	m.prefixPending = true
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	model := updated.(Model)
	model = completeAnimations(model)

	// Then restore
	model.inputMode = ModeTerminal
	model.prefixPending = true
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = updated.(Model)
	model = completeAnimations(model)
	fw = model.wm.FocusedWindow()
	if fw.Rect != origRect {
		t.Error("prefix+j should restore window to original rect")
	}
}

func TestPrefixF1OpensHelp(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+F1 opens help (maps to "help" action)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	model := updated.(Model)
	if model.modal == nil {
		t.Error("prefix+f1 should open help overlay")
	}
	if model.inputMode != ModeNormal {
		t.Error("should switch to normal mode for help overlay")
	}
}

// ── handleKeyPress: newWorkspaceDialog tests ──

func newTestWorkspaceDialog() *NewWorkspaceDialog {
	return &NewWorkspaceDialog{
		Name:       []rune("test"),
		DirPath:    "/tmp",
		DirEntries: []string{"a", "b", "c"},
		Cursor:     0,
	}
}

func TestNewWorkspaceDialogEscCancels(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.newWorkspaceDialog != nil {
		t.Error("escape should close new workspace dialog")
	}
}

func TestNewWorkspaceDialogTabCyclesFields(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()

	// Tab moves from name (0) to browser (1)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model := updated.(Model)
	if model.newWorkspaceDialog.Cursor != 1 {
		t.Errorf("cursor = %d, want 1 after tab", model.newWorkspaceDialog.Cursor)
	}

	// Tab moves from browser (1) to buttons (2)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.newWorkspaceDialog.Cursor != 2 {
		t.Errorf("cursor = %d, want 2 after second tab", model.newWorkspaceDialog.Cursor)
	}

	// Tab wraps from buttons (2) to name (0)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if model.newWorkspaceDialog.Cursor != 0 {
		t.Errorf("cursor = %d, want 0 after wrap", model.newWorkspaceDialog.Cursor)
	}
}

func TestNewWorkspaceDialogShiftTabGoesBack(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()

	// Shift+tab wraps from name (0) to button row (2)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model := updated.(Model)
	if model.newWorkspaceDialog.Cursor != 2 {
		t.Errorf("cursor = %d, want 2 after shift+tab from 0", model.newWorkspaceDialog.Cursor)
	}
}

func TestNewWorkspaceDialogBackspace(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune("abc")
	m.newWorkspaceDialog.TextCursor = 3

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model := updated.(Model)
	if string(model.newWorkspaceDialog.Name) != "ab" {
		t.Errorf("name = %q, want 'ab' after backspace", string(model.newWorkspaceDialog.Name))
	}
}

func TestNewWorkspaceDialogDelete(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune("abc")
	m.newWorkspaceDialog.TextCursor = 0

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
	model := updated.(Model)
	if string(model.newWorkspaceDialog.Name) != "bc" {
		t.Errorf("name = %q, want 'bc' after delete at start", string(model.newWorkspaceDialog.Name))
	}
}

func TestNewWorkspaceDialogHomeEnd(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune("hello")
	m.newWorkspaceDialog.TextCursor = 3

	// Home
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	model := updated.(Model)
	if model.newWorkspaceDialog.TextCursor != 0 {
		t.Errorf("text cursor = %d, want 0 after home", model.newWorkspaceDialog.TextCursor)
	}

	// End
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	model = updated.(Model)
	if model.newWorkspaceDialog.TextCursor != 5 {
		t.Errorf("text cursor = %d, want 5 after end", model.newWorkspaceDialog.TextCursor)
	}
}

func TestNewWorkspaceDialogCtrlU(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune("hello")
	m.newWorkspaceDialog.TextCursor = 3

	// Ctrl+U clears text before cursor
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if string(model.newWorkspaceDialog.Name) != "lo" {
		t.Errorf("name = %q, want 'lo' after ctrl+u at pos 3", string(model.newWorkspaceDialog.Name))
	}
	if model.newWorkspaceDialog.TextCursor != 0 {
		t.Errorf("text cursor = %d, want 0", model.newWorkspaceDialog.TextCursor)
	}
}

func TestNewWorkspaceDialogLeftRight(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune("hello")
	m.newWorkspaceDialog.TextCursor = 2

	// Left
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model := updated.(Model)
	if model.newWorkspaceDialog.TextCursor != 1 {
		t.Errorf("text cursor = %d, want 1 after left", model.newWorkspaceDialog.TextCursor)
	}

	// Right
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.newWorkspaceDialog.TextCursor != 2 {
		t.Errorf("text cursor = %d, want 2 after right", model.newWorkspaceDialog.TextCursor)
	}
}

func TestNewWorkspaceDialogButtonToggle(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Cursor = 2
	m.newWorkspaceDialog.Selected = 0

	// Left/right on button row toggles buttons
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model := updated.(Model)
	if model.newWorkspaceDialog.Selected != 1 {
		t.Errorf("selected = %d, want 1 after left on buttons", model.newWorkspaceDialog.Selected)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.newWorkspaceDialog.Selected != 0 {
		t.Errorf("selected = %d, want 0 after right on buttons", model.newWorkspaceDialog.Selected)
	}
}

func TestNewWorkspaceDialogEnterOnCancel(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Cursor = 2
	m.newWorkspaceDialog.Selected = 1 // Cancel

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	if model.newWorkspaceDialog != nil {
		t.Error("enter on Cancel should close dialog")
	}
}

func TestNewWorkspaceDialogEnterOnNameField(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Cursor = 0 // On name field

	// Enter on name field moves to browser
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	if model.newWorkspaceDialog.Cursor != 1 {
		t.Errorf("cursor = %d, want 1 (browser)", model.newWorkspaceDialog.Cursor)
	}
}

func TestNewWorkspaceDialogTyping(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Name = []rune{}
	m.newWorkspaceDialog.TextCursor = 0

	// Type characters
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'h', Text: "h"}))
	model := updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}))
	model = updated.(Model)
	if string(model.newWorkspaceDialog.Name) != "hi" {
		t.Errorf("name = %q, want 'hi'", string(model.newWorkspaceDialog.Name))
	}
}

func TestNewWorkspaceDialogBrowserNavigation(t *testing.T) {
	m := setupReadyModel()
	m.newWorkspaceDialog = newTestWorkspaceDialog()
	m.newWorkspaceDialog.Cursor = 1 // Browser section
	m.newWorkspaceDialog.DirSelect = 0

	// Down moves selection
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model := updated.(Model)
	if model.newWorkspaceDialog.DirSelect != 1 {
		t.Errorf("dir select = %d, want 1 after down", model.newWorkspaceDialog.DirSelect)
	}

	// Up moves back
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	model = updated.(Model)
	if model.newWorkspaceDialog.DirSelect != 0 {
		t.Errorf("dir select = %d, want 0 after up", model.newWorkspaceDialog.DirSelect)
	}
}

func TestNewWorkspaceDialogBrowserEnterNavigates(t *testing.T) {
	m := setupReadyModel()
	d := makeNewWorkspaceDialog("/tmp")
	m.newWorkspaceDialog = d
	d.Cursor = 1 // Browser section
	d.DirSelect = 0 // ".." entry

	origPath := d.DirPath
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	// Entering ".." should go to parent directory
	if model.newWorkspaceDialog.DirPath == origPath && origPath != "/" {
		t.Error("enter on '..' should navigate to parent directory")
	}
}

// ── handleKeyPress: renameDialog additional tests ──

func TestRenameDialogDeleteKey(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("abc"),
		Cursor:   0,
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
	model := updated.(Model)
	if string(model.renameDialog.Text) != "bc" {
		t.Errorf("text = %q, want 'bc' after delete at pos 0", string(model.renameDialog.Text))
	}
}

func TestRenameDialogLeftRight(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("hello"),
		Cursor:   3,
	}

	// Left
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model := updated.(Model)
	if model.renameDialog.Cursor != 2 {
		t.Errorf("cursor = %d, want 2 after left", model.renameDialog.Cursor)
	}

	// Right
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.renameDialog.Cursor != 3 {
		t.Errorf("cursor = %d, want 3 after right", model.renameDialog.Cursor)
	}
}

func TestRenameDialogHomeEnd(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("hello"),
		Cursor:   3,
	}

	// Home
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	model := updated.(Model)
	if model.renameDialog.Cursor != 0 {
		t.Errorf("cursor = %d, want 0 after home", model.renameDialog.Cursor)
	}

	// End
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	model = updated.(Model)
	if model.renameDialog.Cursor != 5 {
		t.Errorf("cursor = %d, want 5 after end", model.renameDialog.Cursor)
	}
}

func TestRenameDialogTabToggle(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("hello"),
		Cursor:   0,
		Selected: 0,
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model := updated.(Model)
	if model.renameDialog.Selected != 1 {
		t.Errorf("selected = %d, want 1 after tab", model.renameDialog.Selected)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)
	if model.renameDialog.Selected != 0 {
		t.Errorf("selected = %d, want 0 after shift+tab", model.renameDialog.Selected)
	}
}

func TestRenameDialogEnterOnCancel(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	origTitle := fw.Title
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("NewTitle"),
		Cursor:   7,
		Selected: 1, // Cancel
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model := updated.(Model)
	if model.renameDialog != nil {
		t.Error("enter on Cancel should close rename dialog")
	}
	if model.wm.FocusedWindow().Title != origTitle {
		t.Error("title should not change on cancel")
	}
}

func TestRenameDialogCtrlU(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: fw.ID,
		Text:     []rune("hello"),
		Cursor:   3,
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if string(model.renameDialog.Text) != "lo" {
		t.Errorf("text = %q, want 'lo' after ctrl+u at pos 3", string(model.renameDialog.Text))
	}
}

// ── handleKeyPress: global hotkey tests in terminal mode ──

func TestGlobalHotkeysNotBlockedInCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy

	// Ctrl+Q should show quit confirm in copy mode (non-terminal mode)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Error("ctrl+q should show quit confirm in copy mode")
	}
}

func TestF10OpenMenuInNormalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF10}))
	model := updated.(Model)
	if !model.menuBar.IsOpen() {
		t.Error("F10 should open menu in normal mode")
	}
}

func TestQuickWindowCyclingWorksInTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	focused1 := m.wm.FocusedWindow().ID

	// Alt+] should work even in terminal mode
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ']', Mod: tea.ModAlt}))
	model := updated.(Model)
	focused2 := model.wm.FocusedWindow().ID
	if focused1 == focused2 {
		t.Error("alt+] should cycle windows even in terminal mode")
	}
}

// ── handleKeyPress: tour intercepts all input ──

func TestTourInterceptsAllInput(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	// A normal key should be intercepted by tour
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model := updated.(Model)
	// 'n' is not a tour key, so it should be a no-op (no window created)
	if model.wm.Count() != 0 {
		t.Error("tour should intercept keys, not pass to normal mode")
	}
}

// ── handleNormalModeKey: settings shortcut ──

func TestNormalModeCommaOpensSettings(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// ',' opens settings
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: ',', Text: ","}))
	model := updated.(Model)
	if !model.settings.Visible {
		t.Error("',' should open settings panel")
	}
}

func TestNormalModeYTogglesClipboard(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test")
	m.inputMode = ModeNormal

	// 'y' toggles clipboard history
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model := updated.(Model)
	if !model.clipboard.Visible {
		t.Error("'y' should toggle clipboard history")
	}
}

func TestNormalModeBTogglesNotificationCenter(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal

	// 'b' toggles notification center
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Text: "b"}))
	model := updated.(Model)
	if !model.notifications.CenterVisible() {
		t.Error("'b' should toggle notification center")
	}
}

// ── handleKeyPress: overlay priority tests ──

func TestOverlayPriorityContextMenuOverNormal(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Items: []contextmenu.Item{
			{Label: "Test", Action: "test"},
		},
		Visible: true,
	}

	// With context menu visible, escape should close it, not affect windows
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.contextMenu.Visible {
		t.Error("escape should close context menu")
	}
}

func TestOverlayPrioritySettingsOverNormal(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	// With settings visible, escape should close settings
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.settings.Visible {
		t.Error("escape should close settings panel")
	}
}

func TestOverlayPriorityClipboardOverNormal(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test")
	m.clipboard.ShowHistory()

	// With clipboard visible, escape should close it
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model := updated.(Model)
	if model.clipboard.Visible {
		t.Error("escape should close clipboard")
	}
}

func TestOverlayPriorityMenuOverNormal(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	// With menu open, 'n' should NOT create a window
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model := updated.(Model)
	if model.wm.Count() != 0 {
		t.Error("menu should capture keys, not pass to normal mode")
	}
}

// ── handlePrefixAction: prefix actions switch to normal mode when needed ──

func TestPrefixQShowsConfirmAndSwitchesToNormal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}))
	model := updated.(Model)
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Error("prefix+q should show quit confirm")
	}
	if model.inputMode != ModeNormal {
		t.Error("quit action should switch to normal mode from terminal")
	}
}

func TestPrefixTabExitsTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// prefix+tab exits terminal mode (same as prefix+esc)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model := updated.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("prefix+tab should exit terminal mode, got mode %d", model.inputMode)
	}
	if model.prefixPending {
		t.Error("prefix should be cleared after prefix+tab")
	}
}

func TestPrefixDoublePrefixWithRealTerminal(t *testing.T) {
	m := setupReadyModel()
	m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo", Args: []string{"hi"}})
	m = completeAnimations(m)
	m.inputMode = ModeTerminal
	m.prefixPending = true

	// Double prefix (ctrl+a ctrl+a) should forward prefix key to terminal
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if model.prefixPending {
		t.Error("double prefix should clear prefixPending")
	}
	if model.inputMode != ModeTerminal {
		t.Error("should stay in terminal mode after double prefix")
	}
}

// --- tabCycleBackward tests ---

func TestTabCycleBackwardFromMenuBar(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0
	m.tabCycleDir = -1

	result, _ := m.tabCycleBackward()
	model := result.(Model)

	if model.menuBarFocused {
		t.Error("shift+tab from menu bar should leave menu bar")
	}
	if !model.dockFocused {
		t.Error("shift+tab from menu bar should enter dock")
	}
}

func TestTabCycleBackwardFromDockNoWindows(t *testing.T) {
	m := setupReadyModel()
	// No windows
	m.dockFocused = true
	m.tabCycleDir = -1

	result, _ := m.tabCycleBackward()
	model := result.(Model)

	if model.dockFocused {
		t.Error("shift+tab from dock should leave dock")
	}
	if !model.menuBarFocused {
		t.Error("shift+tab from dock with no windows should enter menu bar")
	}
}

func TestTabCycleBackwardFromDockWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.dockFocused = true
	m.tabCycleDir = -1

	result, _ := m.tabCycleBackward()
	model := result.(Model)

	if model.dockFocused {
		t.Error("shift+tab from dock should leave dock")
	}
	// Should focus a window (the last visible one)
	if model.wm.FocusedWindow() == nil {
		t.Error("expected a window to be focused")
	}
}

func TestTabCycleBackwardNoWindowsNoFocus(t *testing.T) {
	m := setupReadyModel()
	// No windows, not dock or menu bar focused
	m.tabCycleDir = -1

	result, _ := m.tabCycleBackward()
	model := result.(Model)

	if !model.menuBarFocused {
		t.Error("shift+tab with no windows should enter menu bar")
	}
}

func TestTabCycleBackwardCyclesThroughWindowsThenMenuBar(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tabCycleDir = -1

	// Cycle backward through both windows
	result, _ := m.tabCycleBackward()
	model := result.(Model)
	result, _ = model.tabCycleBackward()
	model = result.(Model)
	// Third tab should go to menu bar since we've visited all windows
	result, _ = model.tabCycleBackward()
	model = result.(Model)

	if !model.menuBarFocused {
		t.Error("after cycling through all windows, shift+tab should enter menu bar")
	}
}

func TestTabCycleBackwardDirectionChange(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	// Simulate coming from forward direction
	m.tabCycleDir = 1
	m.tabCycleCount = 2

	result, _ := m.tabCycleBackward()
	model := result.(Model)

	// Direction changed, counter should reset
	if model.tabCycleDir != -1 {
		t.Error("expected tab cycle direction to be -1 after backward")
	}
}

// --- isShellProcess tests ---

func TestIsShellProcess(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"", true},
		{"bash", true},
		{"zsh", true},
		{"fish", true},
		{"sh", true},
		{"dash", true},
		{"ksh", true},
		{"tcsh", true},
		{"csh", true},
		{"/bin/bash", true},
		{"/usr/bin/zsh", true},
		{"nvim", false},
		{"htop", false},
		{"python3", false},
		{"/usr/bin/nvim", false},
	}
	for _, tc := range tests {
		got := isShellProcess(tc.cmd)
		if got != tc.want {
			t.Errorf("isShellProcess(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// --- homeEndSeq tests ---

func TestHomeEndSeqShells(t *testing.T) {
	tests := []struct {
		cmd      string
		wantHome string
		wantEnd  string
	}{
		{"bash", "\x1b[1~", "\x1b[4~"},
		{"zsh", "\x1bOH", "\x1bOF"},
		{"sh", "\x01", "\x05"},
		{"dash", "\x01", "\x05"},
		{"ksh", "\x01", "\x05"},
		{"tcsh", "\x1bOH", "\x1bOF"},
		{"csh", "\x1bOH", "\x1bOF"},
	}
	for _, tc := range tests {
		home, end := homeEndSeq(tc.cmd)
		if home != tc.wantHome {
			t.Errorf("homeEndSeq(%q) home = %q, want %q", tc.cmd, home, tc.wantHome)
		}
		if end != tc.wantEnd {
			t.Errorf("homeEndSeq(%q) end = %q, want %q", tc.cmd, end, tc.wantEnd)
		}
	}
}

func TestHomeEndSeqNonShell(t *testing.T) {
	home, end := homeEndSeq("nvim")
	if home != "\x1b[H" {
		t.Errorf("homeEndSeq(nvim) home = %q, want \\x1b[H", home)
	}
	if end != "\x1b[F" {
		t.Errorf("homeEndSeq(nvim) end = %q, want \\x1b[F", end)
	}
}

// --- workspacePickerBounds tests ---

func TestWorkspacePickerBounds(t *testing.T) {
	m := setupReadyModel()
	m.workspaceList = []string{"/path/a", "/path/b", "/path/c"}
	x, y, w, h := m.workspacePickerBounds()
	if w <= 0 || h <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", w, h)
	}
	if x < 0 || y < 0 {
		t.Errorf("expected non-negative start, got (%d,%d)", x, y)
	}
}

func TestWorkspacePickerBoundsSmallScreen(t *testing.T) {
	m := setupReadyModel()
	m.width = 30
	m.height = 15
	m.workspaceList = []string{"/path/a"}
	x, y, w, h := m.workspacePickerBounds()
	if w > m.width {
		t.Errorf("picker width %d exceeds screen width %d", w, m.width)
	}
	if h > m.height {
		t.Errorf("picker height %d exceeds screen height %d", h, m.height)
	}
	_ = x
	_ = y
}

// --- executeMenuAction NEW tests (not duplicating layout tests) ---

func TestExecuteMenuActionToggleTilingFromInput(t *testing.T) {
	m := setupReadyModel()
	m.tilingMode = false
	result, _ := m.executeMenuAction("toggle_tiling")
	model := result.(Model)
	if !model.tilingMode {
		t.Error("expected tiling mode to be enabled")
	}
}

func TestExecuteMenuActionTileSpawnCycleFromInput(t *testing.T) {
	m := setupReadyModel()
	m.tileSpawnPreset = "auto"
	result, _ := m.executeMenuAction("tile_spawn_cycle")
	model := result.(Model)
	if model.tileSpawnPreset != "left" {
		t.Errorf("expected tile_spawn_cycle to change preset to left, got %q", model.tileSpawnPreset)
	}
}

func TestExecuteMenuActionShowKeysFromInput(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = false
	result, _ := m.executeMenuAction("show_keys")
	model := result.(Model)
	if !model.showKeys {
		t.Error("expected showKeys to be true after show_keys menu action")
	}
}

func TestExecuteMenuActionShowKeysToggleOffFromInput(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = true
	m.showKeysEvents = []showKeyEvent{{Key: "a"}}
	result, _ := m.executeMenuAction("show_keys")
	model := result.(Model)
	if model.showKeys {
		t.Error("expected showKeys to be false after toggling off")
	}
	if model.showKeysEvents != nil {
		t.Error("expected showKeysEvents to be cleared when show_keys toggled off")
	}
}

func TestExecuteMenuActionDockFocusWindowFromInput(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()

	result, _ := m.executeMenuAction("dock_focus_window")
	model := result.(Model)
	if model.inputMode != ModeTerminal {
		t.Error("dock_focus_window should switch to terminal mode")
	}
}

func TestExecuteMenuActionDockFocusWindowRestoresMinimizedFromInput(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Minimized = true

	result, _ := m.executeMenuAction("dock_focus_window")
	model := result.(Model)
	fwAfter := model.wm.FocusedWindow()
	if fwAfter != nil && fwAfter.Minimized {
		t.Error("dock_focus_window should unminimize the window")
	}
}

func TestExecuteMenuActionSwapDirectionsFromInput(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	window.TileColumns(m.wm.Windows(), m.wm.WorkArea())

	for _, dir := range []string{"swap_left", "swap_right", "swap_up", "swap_down"} {
		result, cmd := m.executeMenuAction(dir)
		_ = result
		if cmd == nil {
			t.Errorf("expected animation cmd for %s", dir)
		}
	}
}

func TestExecuteMenuActionPasteFromInput(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test paste")
	result, _ := m.executeMenuAction("paste")
	model := result.(Model)
	if model.inputMode != ModeTerminal {
		t.Error("paste should switch to terminal mode")
	}
}

func TestExecuteMenuActionSelectAllNoWindow(t *testing.T) {
	m := setupReadyModel()
	result, _ := m.executeMenuAction("select_all")
	model := result.(Model)
	if model.selActive {
		t.Error("select_all should not activate selection without a window")
	}
}

func TestExecuteMenuActionClearSelection(t *testing.T) {
	m := setupReadyModel()
	m.selActive = true
	result, _ := m.executeMenuAction("clear_selection")
	model := result.(Model)
	if model.selActive {
		t.Error("clear_selection should deactivate selection")
	}
}
