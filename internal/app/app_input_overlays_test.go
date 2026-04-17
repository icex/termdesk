package app

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// ── handleDockNav tests ──

func TestDockNavLeft(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	// Start at index 2 so left goes to 1
	m.dock.SetHover(2)

	updated, _, _ := m.handleDockNav("left")
	model := updated.(Model)

	if model.dock.HoverIndex != 1 {
		t.Errorf("dock hover = %d, want 1", model.dock.HoverIndex)
	}
}

func TestDockNavLeftWraps(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(0)

	updated, _, _ := m.handleDockNav("left")
	model := updated.(Model)

	expected := m.dock.ItemCount() - 1
	if model.dock.HoverIndex != expected {
		t.Errorf("dock hover = %d, want %d (wrapped)", model.dock.HoverIndex, expected)
	}
}

func TestDockNavRight(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(0)

	updated, _, _ := m.handleDockNav("right")
	model := updated.(Model)

	if model.dock.HoverIndex != 1 {
		t.Errorf("dock hover = %d, want 1", model.dock.HoverIndex)
	}
}

func TestDockNavRightWraps(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	last := m.dock.ItemCount() - 1
	m.dock.SetHover(last)

	updated, _, _ := m.handleDockNav("right")
	model := updated.(Model)

	if model.dock.HoverIndex != 0 {
		t.Errorf("dock hover = %d, want 0 (wrapped)", model.dock.HoverIndex)
	}
}

func TestDockNavH(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(2)

	updated, _, _ := m.handleDockNav("h")
	model := updated.(Model)

	if model.dock.HoverIndex != 1 {
		t.Errorf("dock hover = %d, want 1", model.dock.HoverIndex)
	}
}

func TestDockNavL(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(0)

	updated, _, _ := m.handleDockNav("l")
	model := updated.(Model)

	if model.dock.HoverIndex != 1 {
		t.Errorf("dock hover = %d, want 1", model.dock.HoverIndex)
	}
}

func TestDockNavEscapeExitsFocus(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(1)

	updated, _, _ := m.handleDockNav("esc")
	model := updated.(Model)

	if model.dockFocused {
		t.Error("escape should exit dock focus")
	}
	if model.dock.HoverIndex != -1 {
		t.Errorf("dock hover = %d, want -1 after escape", model.dock.HoverIndex)
	}
}

func TestDockNavDotExitsFocus(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(1)

	updated, _, _ := m.handleDockNav(".")
	model := updated.(Model)

	if model.dockFocused {
		t.Error("'.' should exit dock focus")
	}
}

func TestDockNavUpExitsFocus(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(1)

	updated, _, _ := m.handleDockNav("up")
	model := updated.(Model)

	if model.dockFocused {
		t.Error("'up' should exit dock focus")
	}
}

func TestDockNavKExitsFocus(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(1)

	updated, _, _ := m.handleDockNav("k")
	model := updated.(Model)

	if model.dockFocused {
		t.Error("'k' should exit dock focus")
	}
}

func TestDockNavEnterActivates(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	// Hover on the launcher (index 0, Special="launcher")
	m.dock.SetHover(0)

	updated, _, _ := m.handleDockNav("enter")
	model := updated.(Model)

	// activateDockItem for "launcher" toggles the launcher
	if model.dockFocused {
		t.Error("enter should exit dock focus")
	}
	if !model.launcher.Visible {
		t.Error("enter on launcher item should toggle launcher visible")
	}
}

func TestDockNavUnknownKeyExitsDock(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(1)

	updated, cmd, handled := m.handleDockNav("x")
	model := updated.(Model)

	if handled {
		t.Error("unknown key should not be handled by dock nav")
	}
	if cmd != nil {
		t.Error("unknown key should return nil cmd")
	}
	if model.dockFocused {
		t.Error("unknown key should exit dock focus")
	}
}

// ── activateDockItem tests ──

func TestActivateDockItemLauncher(t *testing.T) {
	m := setupReadyModel()
	// Index 0 is the launcher (Special="launcher")
	updated, _ := m.activateDockItem(0)
	model := updated.(Model)

	if model.dockFocused {
		t.Error("activating dock item should exit dock focus")
	}
	if !model.launcher.Visible {
		t.Error("activating launcher item should toggle launcher")
	}
}

func TestActivateDockItemExpose(t *testing.T) {
	m := setupReadyModel()
	// The last item is expose (Special="expose")
	last := len(m.dock.Items) - 1
	updated, cmd := m.activateDockItem(last)
	model := updated.(Model)

	if model.dockFocused {
		t.Error("activating dock item should exit dock focus")
	}
	if !model.exposeMode {
		t.Error("activating expose item should enter expose mode")
	}
	if cmd == nil {
		t.Error("expected animation tick cmd for expose")
	}
}

func TestActivateDockItemExposeToggle(t *testing.T) {
	m := setupReadyModel()
	m.exposeMode = true
	last := len(m.dock.Items) - 1

	updated, _ := m.activateDockItem(last)
	model := updated.(Model)

	if model.exposeMode {
		t.Error("activating expose item when already in expose should exit expose")
	}
}

func TestActivateDockItemMinimized(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.Windows()[0]
	w.Minimized = true

	// Add a minimized item to dock
	m.dock.Items = append(m.dock.Items, dock.DockItem{
		Icon:     "\uf120",
		Label:    "Min",
		Special:  "minimized",
		WindowID: w.ID,
	})
	idx := len(m.dock.Items) - 1

	updated, cmd := m.activateDockItem(idx)
	model := updated.(Model)

	if model.dockFocused {
		t.Error("should exit dock focus")
	}
	rw := model.wm.WindowByID(w.ID)
	if rw == nil {
		t.Fatal("window should still exist")
	}
	if rw.Minimized {
		t.Error("minimized window should be restored")
	}
	if cmd == nil {
		t.Error("expected animation tick cmd for restore")
	}
}

func TestActivateDockItemRunningFocuses(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	windows := m.wm.Windows()
	w1 := windows[0]
	w2 := windows[1]
	// Focus w2 (the last opened window is typically focused)
	m.wm.FocusWindow(w2.ID)

	// Add a running item for w1
	m.dock.Items = append(m.dock.Items, dock.DockItem{
		Icon:     "\uf120",
		Label:    "Run",
		Special:  "running",
		WindowID: w1.ID,
	})
	idx := len(m.dock.Items) - 1

	updated, _ := m.activateDockItem(idx)
	model := updated.(Model)

	fw := model.wm.FocusedWindow()
	if fw == nil || fw.ID != w1.ID {
		t.Error("activating running item should focus that window")
	}
	if model.inputMode != ModeTerminal {
		t.Error("focusing running window should enter terminal mode")
	}
}

func TestActivateDockItemRunningMinimizes(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.Windows()[0]
	m.wm.FocusWindow(w.ID)

	// Add a running item for the focused window
	m.dock.Items = append(m.dock.Items, dock.DockItem{
		Icon:     "\uf120",
		Label:    "Run",
		Special:  "running",
		WindowID: w.ID,
	})
	idx := len(m.dock.Items) - 1

	updated, cmd := m.activateDockItem(idx)
	model := updated.(Model)

	rw := model.wm.WindowByID(w.ID)
	if rw == nil {
		t.Fatal("window should still exist")
	}
	if !rw.Minimized {
		t.Error("clicking running item for focused window should minimize it")
	}
	if cmd == nil {
		t.Error("expected animation tick cmd for minimize")
	}
}

func TestActivateDockItemOutOfBounds(t *testing.T) {
	m := setupReadyModel()

	updated, cmd := m.activateDockItem(-1)
	model := updated.(Model)
	if cmd != nil {
		t.Error("out of bounds should return nil cmd")
	}
	_ = model

	updated, cmd = m.activateDockItem(999)
	if cmd != nil {
		t.Error("out of bounds should return nil cmd")
	}
}

func TestActivateDockItemWithCommand(t *testing.T) {
	m := setupReadyModel()
	// Find a dock item with a command (skip launcher and expose)
	cmdIdx := -1
	for i, item := range m.dock.Items {
		if item.Special == "" && item.Command != "" {
			cmdIdx = i
			break
		}
	}
	if cmdIdx == -1 {
		t.Skip("no dock items with commands found")
	}

	updated, _ := m.activateDockItem(cmdIdx)
	model := updated.(Model)

	if model.dockFocused {
		t.Error("should exit dock focus after launching app")
	}
	// A new window should have been created
	if model.wm.Count() < 1 {
		t.Error("expected at least one window after launching app from dock")
	}
}

// ── handleCopyModeKey tests ──

func setupCopyModeModel(t *testing.T, content string) (Model, *terminal.Terminal) {
	t.Helper()
	m := setupReadyModel()

	win := window.NewWindow("term1", "Term", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 6}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	term.RestoreBuffer(content)
	m.terminals[win.ID] = term
	m.inputMode = ModeCopy
	// Initialize cursor at center of visible content (like enterCopyModeForWindow)
	sbLen := term.ScrollbackLen()
	midRow := cr.Height / 2
	m.copyCursorY = mouseToAbsLine(midRow, 0, sbLen, cr.Height)
	m.copyCursorX = 0
	m.copySnapshot = captureCopySnapshot(win.ID, term)

	return m, term
}

func TestCopyModeEscExitsSelection(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	m.selActive = true

	// With no terminal, escape should exit copy mode entirely
	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := updated.(Model)

	// No terminal -> exits copy mode entirely
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal, got %d", model.inputMode)
	}
	if model.selActive {
		t.Error("selection should be cleared")
	}
}

func TestCopyModeQExits(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}), "q")
	model := updated.(Model)

	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after q, got %d", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Errorf("scroll offset should be 0 after q, got %d", model.scrollOffset)
	}
}

func TestCopyModeIExitsToTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}), "i")
	model := updated.(Model)

	// Without a terminal, exits to ModeNormal (no terminal found)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal (no terminal), got %d", model.inputMode)
	}
}

func TestCopyModeNoTerminalFallback(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	m.scrollOffset = 5

	// Any key should trigger the no-terminal fallback
	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model := updated.(Model)

	// No terminal -> reset to Normal
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal on no terminal, got %d", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("scroll offset should be reset on no terminal")
	}
}

func TestCopyModeVisualSelectionAndYank(t *testing.T) {
	m, term := setupCopyModeModel(t, "hello\nworld")
	defer term.Close()

	// Enter visual selection
	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'v', Text: "v"}), "v")
	model := updated.(Model)
	if !model.selActive {
		t.Fatal("expected selection to become active")
	}

	// Select first line columns 0-4
	model.selStart = geometry.Point{X: 0, Y: 0}
	model.selEnd = geometry.Point{X: 4, Y: 0}

	got := captureStdout(t, func() {
		updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}), "y")
		model = updated.(Model)
	})

	if model.selActive {
		t.Error("selection should be cleared after yank")
	}
	if model.clipboard.Len() == 0 {
		t.Fatal("expected clipboard to have at least one entry after yank")
	}
	if got == "" {
		t.Fatal("expected OSC52 output to be written")
	}
}

func TestCopyModeEnterExitsToTerminal(t *testing.T) {
	m, term := setupCopyModeModel(t, "one\ntwo\nthree")
	defer term.Close()

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model := updated.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after enter, got %d", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("scroll offset should reset on enter")
	}
}

func TestCopyModeHomeEndAdjustsSelection(t *testing.T) {
	m, term := setupCopyModeModel(t, "a\nb\nc\nd\ne\nf")
	defer term.Close()

	// Activate selection
	m.selActive = true
	m.scrollOffset = 1

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), "home")
	model := updated.(Model)
	if model.scrollOffset != term.ScrollbackLen() {
		t.Errorf("expected scrollOffset=maxScroll, got %d", model.scrollOffset)
	}
	if model.selEnd.X != 0 || model.selEnd.Y != 0 {
		t.Errorf("expected selEnd at 0,0, got %+v", model.selEnd)
	}

	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}), "shift+g")
	model = updated.(Model)
	if model.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after end, got %d", model.scrollOffset)
	}
	if model.selEnd.X != term.Width()-1 {
		t.Errorf("expected selEnd.X at last col, got %d", model.selEnd.X)
	}
}

func TestCopyModeGGGoesToTop(t *testing.T) {
	content := strings.Repeat("line\n", 20)
	m, term := setupCopyModeModel(t, content)
	defer term.Close()

	m.scrollOffset = 1

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}), "g")
	model := updated.(Model)
	if model.scrollOffset != 1 {
		t.Errorf("expected scrollOffset unchanged after first g, got %d", model.scrollOffset)
	}

	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}), "g")
	model = updated.(Model)
	if model.scrollOffset != term.ScrollbackLen() {
		t.Errorf("expected scrollOffset=maxScroll after gg, got %d", model.scrollOffset)
	}
}

func TestCopyModeCountPrefix(t *testing.T) {
	content := strings.Repeat("line\n", 20)
	m, term := setupCopyModeModel(t, content)
	defer term.Close()

	startY := m.copyCursorY

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '3', Text: "3"}), "3")
	updated, _ = updated.(Model).handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model := updated.(Model)
	if model.copyCursorY != startY-3 {
		t.Errorf("expected copyCursorY=%d after 3+up, got %d", startY-3, model.copyCursorY)
	}

	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model = updated.(Model)
	if model.copyCursorY != startY-4 {
		t.Errorf("expected copyCursorY=%d after another up, got %d", startY-4, model.copyCursorY)
	}
}

func TestCopyModeSearchForward(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("line ")
		if i < 10 {
			sb.WriteByte('0')
		}
		sb.WriteString(strconv.Itoa(i))
	}
	m, term := setupCopyModeModel(t, sb.String())
	defer term.Close()

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Text: "/"}), "/")
	model := updated.(Model)
	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Text: "line 02"}), "")
	model = updated.(Model)
	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model = updated.(Model)

	sbLen := term.ScrollbackLen()
	want := sbLen - 2
	if model.scrollOffset != want {
		t.Errorf("scrollOffset = %d, want %d after search", model.scrollOffset, want)
	}
}

func TestCopyModeSearchBackward(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("line ")
		if i < 10 {
			sb.WriteByte('0')
		}
		sb.WriteString(strconv.Itoa(i))
	}
	m, term := setupCopyModeModel(t, sb.String())
	defer term.Close()

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Text: "?"}), "?")
	model := updated.(Model)
	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Text: "line 05"}), "")
	model = updated.(Model)
	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model = updated.(Model)

	sbLen := term.ScrollbackLen()
	want := sbLen - 5
	if model.scrollOffset != want {
		t.Errorf("scrollOffset = %d, want %d after backward search", model.scrollOffset, want)
	}
}

func TestCopyModePageScrollAdjustsSelection(t *testing.T) {
	m, term := setupCopyModeModel(t, "one\ntwo\nthree\nfour\nfive\nsix")
	defer term.Close()

	m.selActive = true
	m.selEnd = geometry.Point{X: 0, Y: 3}

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}), "pgup")
	model := updated.(Model)
	if model.scrollOffset == 0 {
		t.Error("expected scrollOffset to increase on pgup")
	}
	if model.selEnd.Y >= m.selEnd.Y {
		t.Error("expected selEnd.Y to move up on pgup")
	}

	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}), "pgdown")
	model = updated.(Model)
	if model.scrollOffset != 0 {
		t.Error("expected scrollOffset to decrease on pgdown")
	}
}

func TestCopyModeLeftRightClampsSelection(t *testing.T) {
	m, term := setupCopyModeModel(t, "hello")
	defer term.Close()

	m.selActive = true
	m.copyCursorX = 0
	// copyCursorY keeps its init value (0)
	m.selEnd = geometry.Point{X: 0, Y: m.copyCursorY}

	updated, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}), "left")
	model := updated.(Model)
	if model.copyCursorX != 0 {
		t.Errorf("expected copyCursorX to clamp at 0, got %d", model.copyCursorX)
	}

	model.copyCursorX = term.Width() - 1
	model.selEnd.X = term.Width() - 1
	updated, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}), "right")
	model = updated.(Model)
	if model.copyCursorX != term.Width()-1 {
		t.Errorf("expected copyCursorX to clamp at last col, got %d", model.copyCursorX)
	}
}

// ── handleClipboardKey tests ──

func TestClipboardKeyEscCloses(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("esc")
	model := updated.(Model)

	if model.clipboard.Visible {
		t.Error("escape should close clipboard history")
	}
}

func TestClipboardKeyYCloses(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("y")
	model := updated.(Model)

	if model.clipboard.Visible {
		t.Error("y should close clipboard history")
	}
}

func TestClipboardKeyUpMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("first")
	m.clipboard.Copy("second")
	m.clipboard.Copy("third")
	m.clipboard.ShowHistory()

	// Initial selection is 0
	updated, _ := m.handleClipboardKey("up")
	model := updated.(Model)

	// MoveSelection(-1) wraps from 0 to 2
	if model.clipboard.SelectedIdx != 2 {
		t.Errorf("selection = %d, want 2 (wrapped)", model.clipboard.SelectedIdx)
	}
}

func TestClipboardKeyDownMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("first")
	m.clipboard.Copy("second")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("down")
	model := updated.(Model)

	if model.clipboard.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", model.clipboard.SelectedIdx)
	}
}

func TestClipboardKeyKMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("first")
	m.clipboard.Copy("second")
	m.clipboard.ShowHistory()
	m.clipboard.MoveSelection(1) // Now at index 1

	updated, _ := m.handleClipboardKey("k")
	model := updated.(Model)

	if model.clipboard.SelectedIdx != 0 {
		t.Errorf("selection = %d, want 0", model.clipboard.SelectedIdx)
	}
}

func TestClipboardKeyJMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("first")
	m.clipboard.Copy("second")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("j")
	model := updated.(Model)

	if model.clipboard.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", model.clipboard.SelectedIdx)
	}
}

func TestClipboardKeyDeleteRemovesEntry(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("first")
	m.clipboard.Copy("second")
	m.clipboard.Copy("third")
	m.clipboard.ShowHistory()

	// Selection starts at 0 (newest = "third")
	updated, _ := m.handleClipboardKey("d")
	model := updated.(Model)

	if model.clipboard.Len() != 2 {
		t.Errorf("clipboard len = %d, want 2", model.clipboard.Len())
	}
}

func TestClipboardKeyDeleteAdjustsSelection(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("only")
	m.clipboard.ShowHistory()

	// Delete the only item
	updated, _ := m.handleClipboardKey("d")
	model := updated.(Model)

	if model.clipboard.Len() != 0 {
		t.Errorf("clipboard len = %d, want 0", model.clipboard.Len())
	}
}

func TestClipboardKeyEnterClosesAndPastes(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("paste-me")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("enter")
	model := updated.(Model)

	if model.clipboard.Visible {
		t.Error("enter should close clipboard history")
	}
	if model.inputMode != ModeTerminal {
		t.Errorf("enter should set terminal mode, got %d", model.inputMode)
	}
}

func TestClipboardKeyVOpensViewer(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("view-me")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("v")
	model := updated.(Model)

	if model.clipboard.Visible {
		t.Error("v should close clipboard history")
	}
}

func TestClipboardKeyNOpensNameDialog(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("entry")
	m.clipboard.ShowHistory()

	updated, _ := m.handleClipboardKey("n")
	model := updated.(Model)
	if model.bufferNameDialog == nil {
		t.Fatal("n should open buffer name dialog")
	}
}

func TestClipboardKeyNClearName(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("entry")
	m.clipboard.ShowHistory()
	m.clipboard.SetSelectedName(0, "tmp")

	updated, _ := m.handleClipboardKey("N")
	model := updated.(Model)
	if got := model.clipboard.SelectedName(); got != "" {
		t.Fatalf("selected name = %q, want empty", got)
	}
}

// ── handleNotificationCenterKey tests ──

func TestNotificationCenterEscCloses(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("Title", "Body", notification.Info)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("esc")
	model := updated.(Model)

	if model.notifications.CenterVisible() {
		t.Error("escape should close notification center")
	}
}

func TestNotificationCenterBCloses(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("Title", "Body", notification.Info)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("b")
	model := updated.(Model)

	if model.notifications.CenterVisible() {
		t.Error("b should close notification center")
	}
}

func TestNotificationCenterDownMoves(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("First", "Body1", notification.Info)
	m.notifications.Push("Second", "Body2", notification.Warning)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("down")
	model := updated.(Model)

	if model.notifications.CenterIndex() != 1 {
		t.Errorf("center index = %d, want 1", model.notifications.CenterIndex())
	}
}

func TestNotificationCenterUpMoves(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("First", "Body1", notification.Info)
	m.notifications.Push("Second", "Body2", notification.Warning)
	m.notifications.ShowCenter()

	// Move down first, then up
	updated, _ := m.handleNotificationCenterKey("down")
	model := updated.(Model)
	updated, _ = model.handleNotificationCenterKey("up")
	model = updated.(Model)

	if model.notifications.CenterIndex() != 0 {
		t.Errorf("center index = %d, want 0", model.notifications.CenterIndex())
	}
}

func TestNotificationCenterJMoves(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("First", "Body1", notification.Info)
	m.notifications.Push("Second", "Body2", notification.Info)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("j")
	model := updated.(Model)

	if model.notifications.CenterIndex() != 1 {
		t.Errorf("center index = %d, want 1", model.notifications.CenterIndex())
	}
}

func TestNotificationCenterKMoves(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("First", "Body1", notification.Info)
	m.notifications.Push("Second", "Body2", notification.Info)
	m.notifications.ShowCenter()

	// Move to 1, then k back to 0
	m.notifications.MoveCenterSelection(1)
	updated, _ := m.handleNotificationCenterKey("k")
	model := updated.(Model)

	if model.notifications.CenterIndex() != 0 {
		t.Errorf("center index = %d, want 0", model.notifications.CenterIndex())
	}
}

func TestNotificationCenterDeleteRemoves(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("Title", "Body", notification.Info)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("d")
	model := updated.(Model)

	items := model.notifications.HistoryItems()
	if len(items) != 0 {
		t.Errorf("history len = %d, want 0 after delete", len(items))
	}
}

func TestNotificationCenterClearAll(t *testing.T) {
	m := setupReadyModel()
	m.notifications.Push("A", "a", notification.Info)
	m.notifications.Push("B", "b", notification.Warning)
	m.notifications.Push("C", "c", notification.Error)
	m.notifications.ShowCenter()

	updated, _ := m.handleNotificationCenterKey("D")
	model := updated.(Model)

	items := model.notifications.HistoryItems()
	if len(items) != 0 {
		t.Errorf("history len = %d, want 0 after clear all", len(items))
	}
}

func TestNotificationCenterDeleteEmpty(t *testing.T) {
	m := setupReadyModel()
	m.notifications.ShowCenter()

	// Should not panic on empty
	updated, cmd := m.handleNotificationCenterKey("d")
	model := updated.(Model)
	_ = model
	if cmd != nil {
		t.Error("delete on empty should return nil cmd")
	}
}

// ── handleTourKey tests ──

func TestTourKeyEscapeSkips(t *testing.T) {
	m := setupReadyModel()
	// Re-enable tour for this test
	m.tour.Active = true
	m.tour.Current = 0

	updated, _ := m.handleTourKey("esc")
	model := updated.(Model)

	if model.tour.Active {
		t.Error("escape should skip/end the tour")
	}
}

func TestTourKeyEscapeAlias(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	updated, _ := m.handleTourKey("escape")
	model := updated.(Model)

	if model.tour.Active {
		t.Error("'escape' should skip the tour")
	}
}

func TestTourKeyEnterAdvances(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	updated, _ := m.handleTourKey("enter")
	model := updated.(Model)

	if model.tour.Current != 1 {
		t.Errorf("tour step = %d, want 1", model.tour.Current)
	}
	if !model.tour.Active {
		t.Error("tour should still be active after advancing one step")
	}
}

func TestTourKeySpaceAdvances(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	updated, _ := m.handleTourKey("space")
	model := updated.(Model)

	if model.tour.Current != 1 {
		t.Errorf("tour step = %d, want 1", model.tour.Current)
	}
}

func TestTourKeyRightAdvances(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	updated, _ := m.handleTourKey("right")
	model := updated.(Model)

	if model.tour.Current != 1 {
		t.Errorf("tour step = %d, want 1", model.tour.Current)
	}
}

func TestTourKeyEnterFinishesTour(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	// Set to last step
	m.tour.Current = len(m.tour.Steps) - 1

	updated, _ := m.handleTourKey("enter")
	model := updated.(Model)

	if model.tour.Active {
		t.Error("tour should be inactive after advancing past last step")
	}
}

func TestTourKeyUnknownNoop(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0

	updated, cmd := m.handleTourKey("x")
	model := updated.(Model)

	if cmd != nil {
		t.Error("unknown key should return nil cmd")
	}
	if model.tour.Current != 0 {
		t.Errorf("tour step should not change on unknown key, got %d", model.tour.Current)
	}
}

// ── handleContextMenuKey tests ──

func TestContextMenuEscapeHides(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})

	updated, _ := m.handleContextMenuKey("esc")
	model := updated.(Model)

	if model.contextMenu.Visible {
		t.Error("escape should hide context menu")
	}
}

func TestContextMenuDownMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	// HoverIndex starts at 0 ("New Terminal")

	updated, _ := m.handleContextMenuKey("down")
	model := updated.(Model)

	// Should skip separator at index 1 and go to index 2
	if model.contextMenu.HoverIndex != 2 {
		t.Errorf("hover = %d, want 2 (skipping separator)", model.contextMenu.HoverIndex)
	}
}

func TestContextMenuUpMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	// Start at item 2
	m.contextMenu.HoverIndex = 2

	updated, _ := m.handleContextMenuKey("up")
	model := updated.(Model)

	// MoveHover(-1) should skip separator at index 1 and go to index 0
	if model.contextMenu.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0 (skipping separator)", model.contextMenu.HoverIndex)
	}
}

func TestContextMenuJMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})

	updated, _ := m.handleContextMenuKey("j")
	model := updated.(Model)

	if model.contextMenu.HoverIndex != 2 {
		t.Errorf("hover = %d, want 2", model.contextMenu.HoverIndex)
	}
}

func TestContextMenuKMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	m.contextMenu.HoverIndex = 2

	updated, _ := m.handleContextMenuKey("k")
	model := updated.(Model)

	if model.contextMenu.HoverIndex != 0 {
		t.Errorf("hover = %d, want 0", model.contextMenu.HoverIndex)
	}
}

func TestContextMenuEnterSelectsAction(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	// Hover on "New Terminal" (index 0, Action="new_terminal")
	m.contextMenu.HoverIndex = 0

	updated, _ := m.handleContextMenuKey("enter")
	model := updated.(Model)

	if model.contextMenu.Visible {
		t.Error("enter should hide context menu")
	}
	// The "new_terminal" action should have created a window
	if model.wm.Count() < 1 {
		t.Error("expected at least 1 window after selecting 'New Terminal'")
	}
}

func TestContextMenuSpaceSelectsAction(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	m.contextMenu.HoverIndex = 0

	updated, _ := m.handleContextMenuKey("space")
	model := updated.(Model)

	if model.contextMenu.Visible {
		t.Error("space should hide context menu")
	}
}

func TestContextMenuEnterOnSeparatorNoop(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})
	// Force hover to separator (index 1, Disabled=true)
	m.contextMenu.HoverIndex = 1

	// SelectedAction() returns "" for disabled items
	windowsBefore := m.wm.Count()
	updated, _ := m.handleContextMenuKey("enter")
	model := updated.(Model)

	if model.contextMenu.Visible {
		t.Error("enter should hide context menu even on separator")
	}
	if model.wm.Count() != windowsBefore {
		t.Error("selecting separator should not create windows")
	}
}

func TestContextMenuEscapeAlias(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = contextmenu.DesktopMenu(10, 10, contextmenu.KeyBindings{})

	updated, _ := m.handleContextMenuKey("escape")
	model := updated.(Model)

	if model.contextMenu.Visible {
		t.Error("'escape' should hide context menu")
	}
}

// ── handleSettingsKey tests ──

func TestSettingsKeyEscCloses(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	updated, _ := m.handleSettingsKey("esc")
	model := updated.(Model)

	if model.settings.Visible {
		t.Error("escape should close settings panel")
	}
}

func TestSettingsKeyCommaCloses(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	updated, _ := m.handleSettingsKey(",")
	model := updated.(Model)

	if model.settings.Visible {
		t.Error("comma should close settings panel")
	}
}

func TestSettingsKeyTabNextTab(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	updated, _ := m.handleSettingsKey("tab")
	model := updated.(Model)

	if model.settings.ActiveTab != 1 {
		t.Errorf("active tab = %d, want 1", model.settings.ActiveTab)
	}
}

func TestSettingsKeyShiftTabPrevTab(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.NextTab() // move to tab 1

	updated, _ := m.handleSettingsKey("shift+tab")
	model := updated.(Model)

	if model.settings.ActiveTab != 0 {
		t.Errorf("active tab = %d, want 0", model.settings.ActiveTab)
	}
}

func TestSettingsKeyDownMovesItem(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	updated, _ := m.handleSettingsKey("down")
	model := updated.(Model)

	if model.settings.ActiveItem != 1 {
		t.Errorf("active item = %d, want 1", model.settings.ActiveItem)
	}
}

func TestSettingsKeyUpMovesItem(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.NextItem() // move to item 1

	updated, _ := m.handleSettingsKey("up")
	model := updated.(Model)

	if model.settings.ActiveItem != 0 {
		t.Errorf("active item = %d, want 0", model.settings.ActiveItem)
	}
}

func TestSettingsKeyJMovesItem(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	updated, _ := m.handleSettingsKey("j")
	model := updated.(Model)

	if model.settings.ActiveItem != 1 {
		t.Errorf("active item = %d, want 1", model.settings.ActiveItem)
	}
}

func TestSettingsKeyKMovesItem(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.NextItem()

	updated, _ := m.handleSettingsKey("k")
	model := updated.(Model)

	if model.settings.ActiveItem != 0 {
		t.Errorf("active item = %d, want 0", model.settings.ActiveItem)
	}
}

func TestSettingsKeyEnterTogglesBoolean(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	// Navigate to a toggle item (index 1 = "Animations" in General tab)
	m.settings.SetItem(1)

	item := m.settings.CurrentItem()
	if item == nil || item.Type != settings.TypeToggle {
		t.Fatal("expected toggle item at index 1")
	}
	origVal := item.BoolVal

	updated, _ := m.handleSettingsKey("enter")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.BoolVal == origVal {
		t.Error("enter should toggle the boolean value")
	}
}

func TestSettingsKeySpaceTogglesBoolean(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.SetItem(1) // Animations toggle

	item := m.settings.CurrentItem()
	origVal := item.BoolVal

	updated, _ := m.handleSettingsKey("space")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.BoolVal == origVal {
		t.Error("space should toggle the boolean value")
	}
}

func TestSettingsKeyEnterOnChoiceNoop(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	// Index 0 = "Theme" which is TypeChoice
	m.settings.SetItem(0)

	item := m.settings.CurrentItem()
	if item == nil || item.Type != settings.TypeChoice {
		t.Fatal("expected choice item at index 0")
	}
	origVal := item.StrVal

	updated, _ := m.handleSettingsKey("enter")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.StrVal != origVal {
		t.Error("enter on choice item should not change value")
	}
}

func TestSettingsKeyRightCyclesChoice(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	// Index 0 = "Theme" which is TypeChoice
	m.settings.SetItem(0)

	item := m.settings.CurrentItem()
	origVal := item.StrVal

	updated, _ := m.handleSettingsKey("right")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.StrVal == origVal {
		t.Error("right should cycle the choice value")
	}
}

func TestSettingsKeyLeftCyclesChoice(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.SetItem(0) // Theme

	item := m.settings.CurrentItem()
	origVal := item.StrVal

	updated, _ := m.handleSettingsKey("left")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.StrVal == origVal {
		t.Error("left should cycle the choice value backwards")
	}
}

func TestSettingsKeyHCyclesChoice(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.SetItem(0)

	item := m.settings.CurrentItem()
	origVal := item.StrVal

	updated, _ := m.handleSettingsKey("h")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.StrVal == origVal {
		t.Error("h should cycle choice backwards")
	}
}

func TestSettingsKeyLCyclesChoice(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()
	m.settings.SetItem(0)

	item := m.settings.CurrentItem()
	origVal := item.StrVal

	updated, _ := m.handleSettingsKey("l")
	model := updated.(Model)

	newItem := model.settings.CurrentItem()
	if newItem.StrVal == origVal {
		t.Error("l should cycle choice forward")
	}
}

func TestSettingsKeyEscAppliesSettings(t *testing.T) {
	m := setupReadyModel()
	m.settings.Show()

	// Change the animations toggle
	m.settings.SetItem(1) // Animations
	originalAnimOn := m.animationsOn
	m.settings.Toggle() // flip the value

	updated, _ := m.handleSettingsKey("esc")
	model := updated.(Model)

	// applySettings should have been called, changing animationsOn
	if model.animationsOn == originalAnimOn {
		t.Error("escape should apply settings (animations toggle should be changed)")
	}
}

// ── handleLauncherKey tests ──

func TestLauncherKeyEscCloses(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("escape should close launcher")
	}
}

func TestLauncherKeyCtrlSlashCloses(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: '/', Mod: tea.ModCtrl}), "ctrl+/")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("ctrl+/ should close launcher")
	}
}

func TestLauncherKeyCtrlSpaceCloses(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: ' ', Mod: tea.ModCtrl}), "ctrl+space")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("ctrl+space should close launcher")
	}
}

func TestLauncherKeyUpMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()
	m.launcher.MoveSelection(1) // Move down first

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model := updated.(Model)

	if model.launcher.SelectedIdx != 0 {
		t.Errorf("selection = %d, want 0", model.launcher.SelectedIdx)
	}
}

func TestLauncherKeyDownMovesSelection(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), "down")
	model := updated.(Model)

	if model.launcher.SelectedIdx != 1 {
		t.Errorf("selection = %d, want 1", model.launcher.SelectedIdx)
	}
}

func TestLauncherKeyBackspaceDeletesChar(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()
	m.launcher.TypeChar('a')
	m.launcher.TypeChar('b')

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}), "backspace")
	model := updated.(Model)

	if model.launcher.Query != "a" {
		t.Errorf("query = %q, want 'a'", model.launcher.Query)
	}
}

func TestLauncherKeyEnterLaunches(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("enter should close launcher")
	}
	if model.wm.Count() < 1 {
		t.Error("enter should launch the selected app")
	}
}

func TestLauncherKeyCtrlCShowsQuitConfirm(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), "ctrl+c")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("ctrl+c should hide launcher")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Error("ctrl+c in launcher should show quit confirm dialog")
	}
}

func TestLauncherKeyCtrlQShowsQuitConfirm(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}), "ctrl+q")
	model := updated.(Model)

	if model.launcher.Visible {
		t.Error("ctrl+q should hide launcher")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Error("ctrl+q in launcher should show quit confirm dialog")
	}
}

func TestLauncherKeyTyping(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}), "t")
	model := updated.(Model)
	updated, _ = model.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}), "e")
	model = updated.(Model)

	if model.launcher.Query != "te" {
		t.Errorf("query = %q, want 'te'", model.launcher.Query)
	}
}

func TestLauncherKeyTabCompletesSelected(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()
	m.launcher.SetQuery("te")

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), "tab")
	model := updated.(Model)
	if model.launcher.Query == "" {
		t.Fatal("tab completion should set a query")
	}
}

func TestLauncherKeyCtrlUpDownHistory(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Toggle()
	m.launcher.RecordQuery("nvim")
	m.launcher.RecordQuery("htop")

	updated, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}), "ctrl+up")
	model := updated.(Model)
	if model.launcher.Query != "htop" {
		t.Fatalf("ctrl+up query = %q, want htop", model.launcher.Query)
	}

	updated, _ = model.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown, Mod: tea.ModCtrl}), "ctrl+down")
	model = updated.(Model)
	if model.launcher.Query != "" {
		t.Fatalf("ctrl+down query = %q, want empty", model.launcher.Query)
	}
}

// ── handleMenuKey tests ──

func TestMenuKeyEscCloses(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	updated, _ := m.handleMenuKey("esc")
	model := updated.(Model)

	if model.menuBar.IsOpen() {
		t.Error("escape should close menu")
	}
}

func TestMenuKeyF10Closes(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	updated, _ := m.handleMenuKey("f10")
	model := updated.(Model)

	if model.menuBar.IsOpen() {
		t.Error("f10 should close menu")
	}
}

func TestMenuKeyUpMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)
	m.menuBar.HoverIndex = 2

	updated, _ := m.handleMenuKey("up")
	model := updated.(Model)

	if model.menuBar.HoverIndex >= 2 {
		t.Errorf("hover = %d, should have moved up", model.menuBar.HoverIndex)
	}
}

func TestMenuKeyDownMovesHover(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	updated, _ := m.handleMenuKey("down")
	model := updated.(Model)

	if model.menuBar.HoverIndex == 0 {
		t.Error("hover should have moved down from 0")
	}
}

func TestMenuKeyLeftMovesMenu(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(1) // Start on Edit menu

	updated, _ := m.handleMenuKey("left")
	model := updated.(Model)

	if model.menuBar.OpenIndex != 0 {
		t.Errorf("open index = %d, want 0", model.menuBar.OpenIndex)
	}
}

func TestMenuKeyRightMovesMenu(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)

	updated, _ := m.handleMenuKey("right")
	model := updated.(Model)

	// Might enter submenu or move to next menu depending on if current has submenu
	// Just verify it changed from initial state
	if model.menuBar.OpenIndex == 0 && !model.menuBar.InSubMenu {
		t.Error("right should move menu or enter submenu")
	}
}

func TestMenuKeyEscExitsSubmenu(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(0)
	m.menuBar.InSubMenu = true

	updated, _ := m.handleMenuKey("esc")
	model := updated.(Model)

	if model.menuBar.InSubMenu {
		t.Error("escape should exit submenu first")
	}
	// Menu should still be open (just exited submenu)
	if !model.menuBar.IsOpen() {
		t.Error("menu should still be open after exiting submenu")
	}
}

func TestMenuKeyLeftExitsSubmenu(t *testing.T) {
	m := setupReadyModel()
	m.menuBar.OpenMenu(1)
	m.menuBar.InSubMenu = true

	updated, _ := m.handleMenuKey("left")
	model := updated.(Model)

	if model.menuBar.InSubMenu {
		t.Error("left should exit submenu")
	}
}

// --- cellsToString tests ---

func TestCellsToStringEmpty(t *testing.T) {
	if got := cellsToString(nil); got != "" {
		t.Errorf("cellsToString(nil) = %q, want empty", got)
	}
	if got := cellsToString([]terminal.ScreenCell{}); got != "" {
		t.Errorf("cellsToString([]) = %q, want empty", got)
	}
}

func TestCellsToStringNormal(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "H", Width: 1},
		{Content: "e", Width: 1},
		{Content: "l", Width: 1},
		{Content: "l", Width: 1},
		{Content: "o", Width: 1},
	}
	if got := cellsToString(cells); got != "Hello" {
		t.Errorf("cellsToString = %q, want %q", got, "Hello")
	}
}

func TestCellsToStringTrailingSpaces(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "A", Width: 1},
		{Content: "B", Width: 1},
		{Content: "", Width: 1},  // empty = space
		{Content: " ", Width: 1}, // space
	}
	if got := cellsToString(cells); got != "AB" {
		t.Errorf("cellsToString = %q, want %q (trailing spaces trimmed)", got, "AB")
	}
}

func TestCellsToStringWidthZeroSkipped(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "A", Width: 1},
		{Content: "", Width: 0},  // wide-char continuation, skipped
		{Content: "B", Width: 1},
	}
	if got := cellsToString(cells); got != "AB" {
		t.Errorf("cellsToString = %q, want %q (width=0 skipped)", got, "AB")
	}
}

func TestCellsToStringEmptyContent(t *testing.T) {
	cells := []terminal.ScreenCell{
		{Content: "", Width: 1},
		{Content: "X", Width: 1},
		{Content: "", Width: 1},
	}
	// Empty content → space, trailing trimmed: " X"
	got := cellsToString(cells)
	if got != " X" {
		t.Errorf("cellsToString = %q, want %q", got, " X")
	}
}

// --- handleMenuBarFocusNav tests ---

func TestMenuBarFocusNavLeft(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 1

	result, _, handled := m.handleMenuBarFocusNav("left")
	model := result.(Model)
	if !handled {
		t.Error("expected left to be handled")
	}
	if model.menuBarFocusIdx != 0 {
		t.Errorf("expected menuBarFocusIdx 0, got %d", model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavLeftWraps(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	result, _, handled := m.handleMenuBarFocusNav("left")
	model := result.(Model)
	if !handled {
		t.Error("expected left to be handled")
	}
	numMenus := len(m.menuBar.Menus)
	if model.menuBarFocusIdx != numMenus-1 {
		t.Errorf("expected menuBarFocusIdx to wrap to %d, got %d", numMenus-1, model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavRight(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	result, _, handled := m.handleMenuBarFocusNav("right")
	model := result.(Model)
	if !handled {
		t.Error("expected right to be handled")
	}
	if model.menuBarFocusIdx != 1 {
		t.Errorf("expected menuBarFocusIdx 1, got %d", model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavRightWraps(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	numMenus := len(m.menuBar.Menus)
	m.menuBarFocusIdx = numMenus - 1

	result, _, handled := m.handleMenuBarFocusNav("right")
	model := result.(Model)
	if !handled {
		t.Error("expected right to be handled")
	}
	if model.menuBarFocusIdx != 0 {
		t.Errorf("expected menuBarFocusIdx to wrap to 0, got %d", model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavEnter(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 2

	result, _, handled := m.handleMenuBarFocusNav("enter")
	model := result.(Model)
	if !handled {
		t.Error("expected enter to be handled")
	}
	if model.menuBarFocused {
		t.Error("expected menuBarFocused to be false after enter")
	}
	if !model.menuBar.IsOpen() {
		t.Error("expected menu to be open after enter")
	}
}

func TestMenuBarFocusNavSpace(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	result, _, handled := m.handleMenuBarFocusNav("space")
	model := result.(Model)
	if !handled {
		t.Error("expected space to be handled")
	}
	if model.menuBarFocused {
		t.Error("expected menuBarFocused to be false after space")
	}
}

func TestMenuBarFocusNavDown(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 1

	result, _, handled := m.handleMenuBarFocusNav("down")
	model := result.(Model)
	if !handled {
		t.Error("expected down to be handled")
	}
	if model.menuBarFocused {
		t.Error("expected menuBarFocused to be false after down (opens menu)")
	}
}

func TestMenuBarFocusNavEscape(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 1

	result, _, handled := m.handleMenuBarFocusNav("esc")
	model := result.(Model)
	if !handled {
		t.Error("expected escape to be handled")
	}
	if model.menuBarFocused {
		t.Error("expected menuBarFocused to be false after escape")
	}
}

func TestMenuBarFocusNavUnrecognized(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 1

	result, _, handled := m.handleMenuBarFocusNav("x")
	model := result.(Model)
	if handled {
		t.Error("expected unrecognized key to not be handled")
	}
	if model.menuBarFocused {
		t.Error("expected menuBarFocused to be false for unrecognized key")
	}
}

func TestMenuBarFocusNavH(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 2

	result, _, handled := m.handleMenuBarFocusNav("h")
	model := result.(Model)
	if !handled {
		t.Error("expected h to be handled (same as left)")
	}
	if model.menuBarFocusIdx != 1 {
		t.Errorf("expected menuBarFocusIdx 1, got %d", model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavL(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	result, _, handled := m.handleMenuBarFocusNav("l")
	model := result.(Model)
	if !handled {
		t.Error("expected l to be handled (same as right)")
	}
	if model.menuBarFocusIdx != 1 {
		t.Errorf("expected menuBarFocusIdx 1, got %d", model.menuBarFocusIdx)
	}
}

func TestMenuBarFocusNavJ(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	_, _, handled := m.handleMenuBarFocusNav("j")
	if !handled {
		t.Error("expected j to be handled (same as down)")
	}
}

func TestMenuBarFocusNavTab(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	_, _, handled := m.handleMenuBarFocusNav("tab")
	if !handled {
		t.Error("expected tab to be handled")
	}
}

func TestMenuBarFocusNavShiftTab(t *testing.T) {
	m := setupReadyModel()
	m.menuBarFocused = true
	m.menuBarFocusIdx = 0

	_, _, handled := m.handleMenuBarFocusNav("shift+tab")
	if !handled {
		t.Error("expected shift+tab to be handled")
	}
}

// --- handleCopyModeKey tests ---

func setupCopyModel() Model {
	m := setupReadyModel()

	win := window.NewWindow("copywin", "Term", geometry.Rect{X: 0, Y: 1, Width: 40, Height: 12}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		// Shell creation failed (e.g. PTY fd limit). Return model in copy mode
		// but without a terminal — callers must handle nil copySnapshot gracefully.
		m.inputMode = ModeCopy
		return m
	}
	// Add some scrollback so scroll operations have data to work with
	term.RestoreBuffer("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")
	m.terminals[win.ID] = term
	m.inputMode = ModeCopy
	// Initialize cursor at center of visible content (like enterCopyModeForWindow)
	sbLen := term.ScrollbackLen()
	midRow := cr.Height / 2
	m.copyCursorY = mouseToAbsLine(midRow, 0, sbLen, cr.Height)
	m.copyCursorX = 0
	m.copySnapshot = captureCopySnapshot(win.ID, term)
	return m
}

func TestCopyModeEscClearsSelectionKeepsCopyMode(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.scrollOffset = 5

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := ret.(Model)
	if model.selActive {
		t.Error("expected selActive to be false after esc with selection")
	}
	// Should NOT exit copy mode on first esc with active selection
	if model.inputMode != ModeCopy {
		t.Error("expected still in copy mode after esc with active selection")
	}
}

func TestCopyModeEscExitsCopyMode(t *testing.T) {
	m := setupCopyModel()
	m.selActive = false
	m.scrollOffset = 5

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := ret.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after esc without selection, got %s", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("expected scrollOffset reset")
	}
}

func TestCopyModeQ(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 3

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"}), "q")
	model := ret.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after q, got %s", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("expected scrollOffset reset after q")
	}
}

func TestCopyModeI(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 3

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'i', Text: "i"}), "i")
	model := ret.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after i, got %s", model.inputMode)
	}
}

func TestCopyModeV(t *testing.T) {
	m := setupCopyModel()
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'v', Text: "v"}), "v")
	model := ret.(Model)
	if !model.selActive {
		t.Error("expected selActive after v")
	}
}

func TestCopyModeVToggleOff(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'v', Text: "v"}), "v")
	model := ret.(Model)
	if model.selActive {
		t.Error("expected selActive off after v toggle")
	}
}

func TestCopyModeUpScrolls(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0
	startY := m.copyCursorY

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model := ret.(Model)
	if model.copyCursorY != startY-1 {
		t.Errorf("expected copyCursorY=%d after up, got %d", startY-1, model.copyCursorY)
	}
}

func TestCopyModeDownAtBottomClamps(t *testing.T) {
	m := setupCopyModel()
	// Place cursor at the very last line
	fw := m.wm.FocusedWindow()
	term := m.terminals[fw.ID]
	snap := m.copySnapshotForWindow(fw.ID)
	totalLines := term.ScrollbackLen() + term.Height()
	if snap != nil {
		totalLines = snap.ScrollbackLen() + snap.Height
	}
	m.copyCursorY = totalLines - 1
	m.scrollOffset = 0
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), "down")
	model := ret.(Model)
	// Cursor clamps at bottom, stays in copy mode
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after down at bottom, got %s", model.inputMode)
	}
	if model.copyCursorY != totalLines-1 {
		t.Errorf("expected cursor clamped at %d, got %d", totalLines-1, model.copyCursorY)
	}
}

func TestCopyModeKWithCount(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0
	m.copyCount = 5
	startY := m.copyCursorY

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}), "k")
	model := ret.(Model)
	// k moves cursor up by count
	if model.copyCursorY >= startY {
		t.Errorf("expected copyCursorY < %d after k with count, got %d", startY, model.copyCursorY)
	}
	if model.copyCount != 0 {
		t.Error("expected copyCount reset after movement")
	}
}

func TestCopyModeLeftWithSelection(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.copyCursorX = 5
	m.copyCursorY = 3
	m.selEnd = geometry.Point{X: 5, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}), "left")
	model := ret.(Model)
	if model.selEnd.X != 4 {
		t.Errorf("expected selEnd.X=4, got %d", model.selEnd.X)
	}
}

func TestCopyModeRightWithSelection(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.copyCursorX = 5
	m.copyCursorY = 3
	m.selEnd = geometry.Point{X: 5, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}), "right")
	model := ret.(Model)
	if model.selEnd.X != 6 {
		t.Errorf("expected selEnd.X=6, got %d", model.selEnd.X)
	}
}

func TestCopyModePgUp(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}), "pgup")
	model := ret.(Model)
	// Should scroll up by terminal height
	if model.scrollOffset <= 0 {
		t.Error("expected scroll offset to increase on pgup")
	}
}

func TestCopyModePgDownAtBottomClamps(t *testing.T) {
	m := setupCopyModel()
	fw := m.wm.FocusedWindow()
	term := m.terminals[fw.ID]
	snap := m.copySnapshotForWindow(fw.ID)
	totalLines := term.ScrollbackLen() + term.Height()
	if snap != nil {
		totalLines = snap.ScrollbackLen() + snap.Height
	}
	m.copyCursorY = totalLines - 1
	m.scrollOffset = 0
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}), "pgdown")
	model := ret.(Model)
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after pgdown at bottom, got %s", model.inputMode)
	}
}

func TestCopyModeHome(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), "home")
	model := ret.(Model)
	// scrollOffset should be set to maxScroll (top of buffer)
	_ = model
}

func TestCopyModeEndWithoutSelection(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 5
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}), "end")
	model := ret.(Model)
	if model.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after end, got %d", model.scrollOffset)
	}
	// Stays in copy mode — cursor moves to bottom
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after end, got %s", model.inputMode)
	}
}

func TestCopyModeEndWithSelection(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 5
	m.selActive = true
	m.selEnd = geometry.Point{X: 0, Y: 0}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}), "end")
	model := ret.(Model)
	if model.scrollOffset != 0 {
		t.Errorf("expected scrollOffset 0 after end, got %d", model.scrollOffset)
	}
	// With selection, should stay in copy mode
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after end with selection, got %s", model.inputMode)
	}
}

func TestCopyModeGG(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0

	// First g
	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}), "g")
	model := ret.(Model)
	if model.copyLastKey != "g" {
		t.Error("expected copyLastKey to be 'g' after first g")
	}

	// Second g -> go to top
	ret, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}), "g")
	model = ret.(Model)
	if model.copyLastKey != "" {
		t.Error("expected copyLastKey to be empty after gg")
	}
}

func TestCopyModeShiftG(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 5
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModShift, Text: "G"}), "shift+g")
	model := ret.(Model)
	if model.scrollOffset != 0 {
		t.Error("expected scrollOffset 0 after shift+g")
	}
	// Stays in copy mode — cursor moves to bottom
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after shift+g, got %s", model.inputMode)
	}
}

func TestCopyModeDigitCount(t *testing.T) {
	m := setupCopyModel()
	m.copyCount = 0

	// Type "3"
	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '3', Text: "3"}), "3")
	model := ret.(Model)
	if model.copyCount != 3 {
		t.Errorf("expected copyCount 3, got %d", model.copyCount)
	}

	// Type "5" -> 35
	ret, _ = model.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '5', Text: "5"}), "5")
	model = ret.(Model)
	if model.copyCount != 35 {
		t.Errorf("expected copyCount 35, got %d", model.copyCount)
	}
}

func TestCopyModeSlashStartsSearch(t *testing.T) {
	m := setupCopyModel()

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}), "/")
	model := ret.(Model)
	if !model.copySearchActive {
		t.Error("expected copySearchActive after /")
	}
	if model.copySearchDir != 1 {
		t.Errorf("expected search dir 1, got %d", model.copySearchDir)
	}
}

func TestCopyModeQuestionStartsBackwardSearch(t *testing.T) {
	m := setupCopyModel()

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}), "?")
	model := ret.(Model)
	if !model.copySearchActive {
		t.Error("expected copySearchActive after ?")
	}
	if model.copySearchDir != -1 {
		t.Errorf("expected search dir -1, got %d", model.copySearchDir)
	}
}

func TestCopyModeSearchEsc(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "test"
	m.copySearchDir = 1

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := ret.(Model)
	if model.copySearchActive {
		t.Error("expected search to be cancelled after esc")
	}
	if model.copySearchQuery != "" {
		t.Error("expected search query to be cleared")
	}
}

func TestCopyModeSearchBackspace(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "test"

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}), "backspace")
	model := ret.(Model)
	if model.copySearchQuery != "tes" {
		t.Errorf("expected 'tes' after backspace, got %q", model.copySearchQuery)
	}
}

func TestCopyModeSearchBackspaceEmpty(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = ""

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}), "backspace")
	model := ret.(Model)
	if model.copySearchQuery != "" {
		t.Errorf("expected empty query after backspace on empty, got %q", model.copySearchQuery)
	}
}

func TestCopyModeSearchTypeChar(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = ""

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}), "a")
	model := ret.(Model)
	if model.copySearchQuery != "a" {
		t.Errorf("expected 'a', got %q", model.copySearchQuery)
	}
}

func TestCopyModeSearchEnter(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "test"
	m.copySearchDir = 1

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model := ret.(Model)
	if model.copySearchActive {
		t.Error("expected search to end after enter")
	}
}

func TestCopyModeNoTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	// No windows open

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}), "k")
	model := ret.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal when no terminal, got %s", model.inputMode)
	}
}

func TestCopyModePrefixKey(t *testing.T) {
	m := setupCopyModel()
	prefixKey := m.keybindings.Prefix

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}), prefixKey)
	model := ret.(Model)
	if !model.prefixPending {
		t.Error("expected prefixPending after prefix key in copy mode")
	}
}

// --- New Phase 1 copy mode tests ---

func TestCopyModeHalfPageUp(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 0
	startY := m.copyCursorY

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}), "ctrl+u")
	model := ret.(Model)
	if model.copyCursorY >= startY {
		t.Errorf("expected copyCursorY < %d after ctrl+u, got %d", startY, model.copyCursorY)
	}
}

func TestCopyModeHalfPageDown(t *testing.T) {
	m := setupCopyModel()
	m.scrollOffset = 5

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}), "ctrl+d")
	model := ret.(Model)
	if model.scrollOffset >= 5 {
		t.Errorf("expected scroll offset to decrease on ctrl+d, got %d", model.scrollOffset)
	}
}

func TestCopyModeHalfPageDownAtBottomClamps(t *testing.T) {
	m := setupCopyModel()
	fw := m.wm.FocusedWindow()
	term := m.terminals[fw.ID]
	snap := m.copySnapshotForWindow(fw.ID)
	totalLines := term.ScrollbackLen() + term.Height()
	if snap != nil {
		totalLines = snap.ScrollbackLen() + snap.Height
	}
	m.copyCursorY = totalLines - 1
	m.scrollOffset = 0
	m.selActive = false

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}), "ctrl+d")
	model := ret.(Model)
	if model.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy after ctrl+d at bottom, got %s", model.inputMode)
	}
}

func TestCopyModeBackToIndentation(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.selEnd = geometry.Point{X: 10, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '6', Mod: tea.ModShift, Text: "^"}), "shift+6")
	model := ret.(Model)
	// ^ should move to first non-space char (likely 0 for these test lines)
	if model.selEnd.X > 10 {
		t.Errorf("expected selEnd.X to decrease or stay, got %d", model.selEnd.X)
	}
}

func TestCopyModeEndOfLine(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.selEnd = geometry.Point{X: 0, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '4', Mod: tea.ModShift, Text: "$"}), "shift+4")
	model := ret.(Model)
	// $ should move to end of line
	if model.selEnd.X <= 0 {
		t.Error("expected selEnd.X to increase on $ motion")
	}
}

func TestCopyModeBracketMatch(t *testing.T) {
	// Test the helper function directly
	lines := []string{
		"hello (world)",
		"foo [bar] baz",
		"nested {a{b}c}",
	}
	y, x := findMatchingBracket(lines, 0, 6) // ( at position 6
	if y != 0 || x != 12 {
		t.Errorf("expected match at (0,12), got (%d,%d)", y, x)
	}

	y, x = findMatchingBracket(lines, 0, 12) // ) at position 12
	if y != 0 || x != 6 {
		t.Errorf("expected reverse match at (0,6), got (%d,%d)", y, x)
	}

	// No bracket at position
	y, x = findMatchingBracket(lines, 0, 0)
	if y != -1 || x != -1 {
		t.Errorf("expected -1,-1 for non-bracket, got (%d,%d)", y, x)
	}
}

func TestCopyModeBracketMatchNested(t *testing.T) {
	lines := []string{"fn(a(b))"}
	y, x := findMatchingBracket(lines, 0, 2) // first (
	if y != 0 || x != 7 {
		t.Errorf("expected outer match at (0,7), got (%d,%d)", y, x)
	}
	y, x = findMatchingBracket(lines, 0, 4) // inner (
	if y != 0 || x != 6 {
		t.Errorf("expected inner match at (0,6), got (%d,%d)", y, x)
	}
}

func TestCopyModeOtherEnd(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.selStart = geometry.Point{X: 2, Y: 1}
	m.selEnd = geometry.Point{X: 8, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'o', Text: "o"}), "o")
	model := ret.(Model)
	if model.selStart.X != 8 || model.selStart.Y != 3 {
		t.Errorf("expected selStart swapped to (8,3), got (%d,%d)", model.selStart.X, model.selStart.Y)
	}
	if model.selEnd.X != 2 || model.selEnd.Y != 1 {
		t.Errorf("expected selEnd swapped to (2,1), got (%d,%d)", model.selEnd.X, model.selEnd.Y)
	}
}

func TestCopyModeOtherEndNoSelection(t *testing.T) {
	m := setupCopyModel()
	m.selActive = false
	start := m.selStart
	end := m.selEnd

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'o', Text: "o"}), "o")
	model := ret.(Model)
	// No swap when not selecting
	if model.selStart != start || model.selEnd != end {
		t.Error("expected no swap when selection not active")
	}
}

func TestCopyModeCopyLine(t *testing.T) {
	m := setupCopyModel()
	m.selActive = false
	m.scrollOffset = 0

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'Y', Mod: tea.ModShift, Text: "Y"}), "shift+y")
	model := ret.(Model)
	// Should copy a line to clipboard
	text := model.clipboard.Paste()
	// The line content depends on setupCopyModel's buffer
	_ = text
	_ = model
}

func TestCopyModeWordForward(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.copyCursorX = 0
	m.copyCursorY = 3
	m.selEnd = geometry.Point{X: 0, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'w', Text: "w"}), "w")
	model := ret.(Model)
	if model.selEnd.X <= 0 && model.selEnd.Y == 3 {
		t.Error("expected selEnd.X to increase on w motion")
	}
}

func TestCopyModeWordBackward(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.copyCursorX = 10
	m.copyCursorY = 3
	m.selEnd = geometry.Point{X: 10, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'b', Text: "b"}), "b")
	model := ret.(Model)
	if model.selEnd.X >= 10 {
		t.Errorf("expected selEnd.X to decrease on b motion, got %d", model.selEnd.X)
	}
}

func TestCopyModeWordEnd(t *testing.T) {
	m := setupCopyModel()
	m.selActive = true
	m.copyCursorX = 0
	m.copyCursorY = 3
	m.selEnd = geometry.Point{X: 0, Y: 3}

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}), "e")
	model := ret.(Model)
	if model.selEnd.X <= 0 && model.selEnd.Y == 3 {
		t.Error("expected selEnd.X to increase on e motion")
	}
}

func TestIsWordSep(t *testing.T) {
	if !isWordSep(' ') {
		t.Error("space should be word separator")
	}
	if !isWordSep('(') {
		t.Error("( should be word separator")
	}
	if isWordSep('a') {
		t.Error("'a' should not be word separator")
	}
	if isWordSep('Z') {
		t.Error("'Z' should not be word separator")
	}
	if isWordSep('5') {
		t.Error("'5' should not be word separator")
	}
}

func TestCopyModeIncrementalSearch(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = ""
	m.copySearchDir = 1

	// Type a character — should trigger incremental search
	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'l', Text: "l"}), "l")
	model := ret.(Model)
	if model.copySearchQuery != "l" {
		t.Errorf("expected query 'l', got %q", model.copySearchQuery)
	}
	// Match count should be populated (the buffer has lines with 'l')
	if model.copySearchMatchCount < 0 {
		t.Error("expected non-negative match count")
	}
}

func TestCopyModeSearchMatchCount(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "line"
	m.copySearchDir = 1

	// Trigger incremental search
	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"}), "1")
	model := ret.(Model)
	// Query is now "line1"
	if model.copySearchQuery != "line1" {
		t.Errorf("expected query 'line1', got %q", model.copySearchQuery)
	}
}

func TestCopyModeSearchNextN(t *testing.T) {
	m := setupCopyModel()
	m.copySearchQuery = "line"
	m.copySearchDir = 1
	m.scrollOffset = 0

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}), "n")
	model := ret.(Model)
	// n should have applied search and potentially moved scroll
	if model.copySearchMatchCount < 0 {
		t.Error("expected non-negative match count after n")
	}
}

func TestCopyModeSearchPrevN(t *testing.T) {
	m := setupCopyModel()
	m.copySearchQuery = "line"
	m.copySearchDir = 1
	m.scrollOffset = 0

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: 'N', Mod: tea.ModShift, Text: "N"}), "shift+n")
	model := ret.(Model)
	// N should search in reverse direction
	if model.copySearchMatchCount < 0 {
		t.Error("expected non-negative match count after N")
	}
}

func TestCopyModeSearchEscClearsCount(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "test"
	m.copySearchDir = 1
	m.copySearchMatchCount = 5
	m.copySearchMatchIdx = 2

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := ret.(Model)
	if model.copySearchMatchCount != 0 {
		t.Errorf("expected match count 0 after esc, got %d", model.copySearchMatchCount)
	}
	if model.copySearchMatchIdx != 0 {
		t.Errorf("expected match idx 0 after esc, got %d", model.copySearchMatchIdx)
	}
}

func TestCopyModeSearchEnterKeepsQuery(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "line"
	m.copySearchDir = 1

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), "enter")
	model := ret.(Model)
	if model.copySearchActive {
		t.Error("expected copySearchActive=false after enter")
	}
	// Query should be preserved for n/N navigation
	if model.copySearchQuery != "line" {
		t.Errorf("expected query preserved after enter, got %q", model.copySearchQuery)
	}
}

func TestCopyModeSearchBackspaceUpdatesSearch(t *testing.T) {
	m := setupCopyModel()
	m.copySearchActive = true
	m.copySearchQuery = "lin"
	m.copySearchDir = 1

	ret, _ := m.handleCopyModeKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}), "backspace")
	model := ret.(Model)
	if model.copySearchQuery != "li" {
		t.Errorf("expected query 'li', got %q", model.copySearchQuery)
	}
	// Should have re-applied search incrementally
}

// --- handleLauncherKey tests ---

func TestLauncherKeyEsc(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), "esc")
	model := ret.(Model)
	if model.launcher.Visible {
		t.Error("expected launcher to be hidden after esc")
	}
}

func TestLauncherKeyCtrlC(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}), "ctrl+c")
	model := ret.(Model)
	if model.launcher.Visible {
		t.Error("expected launcher hidden after ctrl+c")
	}
	if model.confirmClose == nil {
		t.Error("expected quit confirm dialog after ctrl+c in launcher")
	}
}

func TestLauncherKeyUpDown(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), "up")
	model := ret.(Model)
	_ = model // just checking it doesn't panic

	ret, _ = model.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), "down")
	model = ret.(Model)
	_ = model
}

func TestLauncherKeyBackspace(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	m.launcher.TypeChar('h')
	m.launcher.TypeChar('i')

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}), "backspace")
	model := ret.(Model)
	if model.launcher.Query != "h" {
		t.Errorf("expected query 'h' after backspace, got %q", model.launcher.Query)
	}
}

func TestLauncherKeyTab(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}), "tab")
	model := ret.(Model)
	_ = model // just verify no panic
}

func TestLauncherKeyTypeChar(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}), "x")
	model := ret.(Model)
	if model.launcher.Query != "x" {
		t.Errorf("expected query 'x', got %q", model.launcher.Query)
	}
}

func TestLauncherKeyCtrlUpDown(t *testing.T) {
	m := setupReadyModel()
	m.launcher.Show()
	// Record some query history
	m.launcher.TypeChar('a')
	m.launcher.RecordQuery("a")
	m.launcher.TypeChar('b')
	m.launcher.RecordQuery("ab")

	ret, _ := m.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}), "ctrl+up")
	model := ret.(Model)
	_ = model

	ret, _ = model.handleLauncherKey(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown, Mod: tea.ModCtrl}), "ctrl+down")
	model = ret.(Model)
	_ = model
}
