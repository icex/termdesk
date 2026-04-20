package app

import (
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

// completeAnimations sends enough ticks to settle all spring animations.
func completeAnimations(m Model) Model {
	now := time.Now()
	for i := 0; i < 120; i++ { // ~2 seconds at 60fps
		now = now.Add(16 * time.Millisecond)
		updated, _ := m.Update(AnimationTickMsg{Time: now})
		m = updated.(Model)
		if !m.hasActiveAnimations() {
			break
		}
	}
	return m
}

func setupReadyModel() Model {
	m := New()
	m.tour.Skip() // skip tour for tests
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
	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil cmd from Init (system stats tick)")
	}
}

func TestTickCommandsReturnMessages(t *testing.T) {
	m := setupReadyModel()

	if msg := m.tickSystemStats(); msg == nil {
		t.Fatal("expected non-nil tickSystemStats cmd")
	} else if _, ok := msg().(SystemStatsMsg); !ok {
		t.Fatalf("expected SystemStatsMsg from tickSystemStats, got %T", msg())
	}

	if msg := tickAnimation(); msg == nil {
		t.Fatal("expected non-nil tickAnimation cmd")
	} else if _, ok := msg().(AnimationTickMsg); !ok {
		t.Fatalf("expected AnimationTickMsg from tickAnimation, got %T", msg())
	}

	if msg := tickTooltipCheck(); msg == nil {
		t.Fatal("expected non-nil tickTooltipCheck cmd")
	} else if _, ok := msg().(TooltipCheckMsg); !ok {
		t.Fatalf("expected TooltipCheckMsg from tickTooltipCheck, got %T", msg())
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

func TestUpdateWindowSizeRetilesWhenTilingModeEnabled(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "columns"
	m.wm.SetBounds(60, 24)
	m.applyTilingLayout()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)
	model = completeAnimations(model)

	wa := model.wm.WorkArea()
	for _, w := range model.wm.Windows() {
		if w.Minimized || !w.Visible || !w.Resizable {
			continue
		}
		if w.Rect.Height != wa.Height {
			t.Fatalf("window %s height=%d want=%d after resize retile", w.ID, w.Rect.Height, wa.Height)
		}
	}
}

func TestQuitCtrlC(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if cmd != nil {
		t.Error("ctrl+c should show confirm dialog, not quit immediately")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Fatal("expected quit confirm dialog")
	}
	// Confirm with 'y'
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming")
	}
}

func TestQuitCtrlQ(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	model := updated.(Model)
	if cmd != nil {
		t.Error("ctrl+q should show confirm dialog, not quit immediately")
	}
	if model.confirmClose == nil || !model.confirmClose.IsQuit {
		t.Fatal("expected quit confirm dialog")
	}
	// Confirm with enter
	_, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming")
	}
}

func TestQuitQNoWindows(t *testing.T) {
	m := setupReadyModel()
	// In Normal mode, 'q' shows confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	m2 := updated.(Model)
	if m2.confirmClose == nil {
		t.Error("expected confirm dialog on q when no windows focused")
	}
	// Pressing 'y' should quit
	updated, cmd := m2.Update(tea.KeyPressMsg(tea.Key{Code: 'y'}))
	if cmd == nil {
		t.Error("expected quit cmd after confirming with y")
	}
	_ = updated
}

func TestNoQuitQWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	// Window is focused, 'q' in terminal mode does nothing (goes to terminal)
	// but our demo windows are not terminals, so they fall through to normal mode
	// In normal mode with a focused window, 'q' shows confirm dialog
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	m2 := updated.(Model)
	if m2.confirmClose == nil {
		t.Error("expected confirm dialog on q with windows")
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
	if v.Content == "" {
		t.Error("expected loading content")
	}
}

func TestViewRendersWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	v := m.View()
	if v.Content == "" {
		t.Error("expected rendered content with windows")
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

func TestSnapNoWindowNoPanic(t *testing.T) {
	m := setupReadyModel()
	// These should not panic with no focused window (Normal mode keys)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'h'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'l'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'k'}))
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'j'}))
}

func TestSystemStatsMsgWithBattery(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(SystemStatsMsg{
		CPU:         42.5,
		MemGB:       8.2,
		BatPct:      75,
		BatCharging: true,
		BatPresent:  true,
	})
	model := updated.(Model)
	if cmd == nil {
		t.Error("expected tick cmd from SystemStatsMsg")
	}
	// Widget bar should be updated (check via widget types)
	if model.widgetBar == nil {
		t.Fatal("widgetBar is nil")
	}
	for _, w := range model.widgetBar.Widgets {
		switch ww := w.(type) {
		case *widget.CPUWidget:
			if ww.Pct != 42.5 {
				t.Errorf("CPU = %v, want 42.5", ww.Pct)
			}
		case *widget.BatteryWidget:
			if ww.Pct != 75 {
				t.Errorf("BatPct = %v, want 75", ww.Pct)
			}
			if !ww.Charging {
				t.Error("expected Charging=true")
			}
			if !ww.Present {
				t.Error("expected Present=true")
			}
		}
	}
}

// ── Animation: currentRect tests ──

func TestCurrentRect(t *testing.T) {
	a := Animation{
		Type:    AnimOpen,
		X:       10.4,
		Y:       20.6,
		W:       80.3,
		H:       24.7,
		EndRect: geometry.Rect{X: 10, Y: 20, Width: 80, Height: 24},
	}
	r := a.currentRect()
	if r.X != 10 {
		t.Errorf("X = %d, want 10", r.X)
	}
	if r.Y != 21 {
		t.Errorf("Y = %d, want 21 (rounded from 20.6)", r.Y)
	}
	if r.Width != 80 {
		t.Errorf("Width = %d, want 80", r.Width)
	}
	if r.Height != 25 {
		t.Errorf("Height = %d, want 25 (rounded from 24.7)", r.Height)
	}
}

func TestCurrentRectMinWidth(t *testing.T) {
	// Width and height should be clamped to at least 1
	a := Animation{
		X: 5, Y: 5, W: 0.1, H: 0.2,
	}
	r := a.currentRect()
	if r.Width < 1 {
		t.Errorf("Width = %d, want >= 1", r.Width)
	}
	if r.Height < 1 {
		t.Errorf("Height = %d, want >= 1", r.Height)
	}
}

// ── Animation: animatedRect tests ──

func TestAnimatedRectWithAnimation(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected a focused window")
	}

	// Start an animation for this window
	from := geometry.Rect{X: 10, Y: 10, Width: 40, Height: 15}
	to := geometry.Rect{X: 20, Y: 20, Width: 60, Height: 20}
	m.startWindowAnimation(w.ID, AnimSnap, from, to)

	r, animating := m.animatedRect(w.ID)
	if !animating {
		t.Error("expected animating = true")
	}
	// The initial position should be at or near 'from'
	if r.X < 5 || r.X > 25 {
		t.Errorf("animated X = %d, expected near from.X=10", r.X)
	}
}

func TestAnimatedRectNoAnimation(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected a focused window")
	}

	_, animating := m.animatedRect(w.ID)
	if animating {
		t.Error("expected animating = false when no animation active")
	}
}

// ── Animation: dockPulseProgress tests ──

func TestDockPulseProgress(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true

	// No pulse initially
	p := m.dockPulseProgress(0)
	if p != -1 {
		t.Errorf("expected -1, got %f", p)
	}

	// Start a dock pulse
	m.startDockPulse(0)
	p = m.dockPulseProgress(0)
	if p == -1 {
		t.Error("expected non-negative progress after starting dock pulse")
	}
}

// ── Animation: startDockPulse tests ──

func TestStartDockPulse(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true

	if m.hasActiveAnimations() {
		t.Error("expected no active animations initially")
	}

	m.startDockPulse(2)
	if !m.hasActiveAnimations() {
		t.Error("expected active animation after startDockPulse")
	}
	if len(m.animations) != 1 {
		t.Fatalf("expected 1 animation, got %d", len(m.animations))
	}
	a := m.animations[0]
	if a.Type != AnimDockPulse {
		t.Errorf("expected AnimDockPulse, got %d", a.Type)
	}
	if a.DockIndex != 2 {
		t.Errorf("DockIndex = %d, want 2", a.DockIndex)
	}
}

func TestStartDockPulseAnimationsOff(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = false
	m.startDockPulse(0)
	if m.hasActiveAnimations() {
		t.Error("expected no animation when animations are off")
	}
}

// ── tickTooltipCheck test ──

func TestTickTooltipCheck(t *testing.T) {
	cmd := tickTooltipCheck()
	if cmd == nil {
		t.Error("expected non-nil cmd from tickTooltipCheck")
	}
}

// ── cycleInputMode tests ──

func TestCycleInputModeNormalToTerminal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeNormal
	m.cycleInputMode()
	if m.inputMode != ModeTerminal {
		t.Errorf("expected ModeTerminal, got %v", m.inputMode)
	}
}

func TestCycleInputModeTerminalToCopy(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	m.cycleInputMode()
	if m.inputMode != ModeCopy {
		t.Errorf("expected ModeCopy, got %v", m.inputMode)
	}
}

func TestCycleInputModeCopyToNormal(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	m.cycleInputMode()
	if m.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal, got %v", m.inputMode)
	}
}

// ── SetProgram test ──

func TestSetProgram(t *testing.T) {
	m := New()
	if m.progRef == nil {
		t.Fatal("progRef should be initialized in New()")
	}
	if m.progRef.p != nil {
		t.Error("progRef.p should be nil before SetProgram")
	}
	// Verify the method exists and does not panic with nil
	m.SetProgram(nil)
	if m.progRef.p != nil {
		t.Error("progRef.p should be nil after SetProgram(nil)")
	}
}

// ── aboutOverlay test ──

func TestAboutOverlay(t *testing.T) {
	m := setupReadyModel()
	overlay := m.aboutOverlay()
	if overlay == nil {
		t.Fatal("expected non-nil overlay")
	}
	if overlay.Title != "About" {
		t.Errorf("Title = %q, want 'About'", overlay.Title)
	}
	if len(overlay.Lines) == 0 {
		t.Error("expected non-empty Lines")
	}
	found := false
	for _, line := range overlay.Lines {
		if strings.Contains(line, "termdesk") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'termdesk' in about overlay lines")
	}
}

// ── commandDisplayName tests ──

func TestCommandDisplayNameBash(t *testing.T) {
	name := commandDisplayName("/bin/bash")
	if name != "bash" {
		t.Errorf("got %q, want 'bash'", name)
	}
}

func TestCommandDisplayNameNvim(t *testing.T) {
	name := commandDisplayName("/usr/bin/nvim")
	if name != "nvim" {
		t.Errorf("got %q, want 'nvim'", name)
	}
}

func TestCommandDisplayNameTermdeskApp(t *testing.T) {
	name := commandDisplayName("termdesk-calc")
	if name != "Calc" {
		t.Errorf("got %q, want 'Calc'", name)
	}
}

func TestCommandDisplayNameTermdeskAppPath(t *testing.T) {
	name := commandDisplayName("/home/user/go/bin/termdesk-calc")
	if name != "Calc" {
		t.Errorf("got %q, want 'Calc'", name)
	}
}

func TestCommandDisplayNameSimple(t *testing.T) {
	name := commandDisplayName("htop")
	if name != "htop" {
		t.Errorf("got %q, want 'htop'", name)
	}
}

// ── openTerminalWindowMaximized test ──

func TestOpenTerminalWindowMaximized(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowMaximized("/bin/echo", []string{"hello"}, "Echo", "")
	m = completeAnimations(m)
	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}
	if w.PreMaxRect == nil {
		t.Error("expected PreMaxRect to be set for maximized window")
	}
	wa := m.wm.WorkArea()
	if w.Rect != wa {
		t.Errorf("window rect %v != work area %v", w.Rect, wa)
	}
}

// ── openFixedTerminalWindow test ──

func TestOpenFixedTerminalWindow(t *testing.T) {
	m := setupReadyModel()
	m.openFixedTerminalWindow("/bin/echo", []string{"test"}, 50, 20, "Fixed")
	m = completeAnimations(m)
	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}
	if w.Rect.Width != 50 {
		t.Errorf("Width = %d, want 50", w.Rect.Width)
	}
	if w.Rect.Height != 20 {
		t.Errorf("Height = %d, want 20", w.Rect.Height)
	}
	if w.Resizable {
		t.Error("expected Resizable = false for fixed window")
	}
}

// ── View() additional branch tests ──

func TestViewWithClipboardOverlay(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("test clipboard entry")
	m.clipboard.ShowHistory()
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with clipboard overlay")
	}
}

func TestViewWithNotificationCenter(t *testing.T) {
	m := setupReadyModel()
	m.notifications.ShowCenter()
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with notification center")
	}
}

func TestViewWithSettingsPanel(t *testing.T) {
	m := setupReadyModel()
	m.settings.Visible = true
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with settings panel")
	}
}

func TestViewWithContextMenu(t *testing.T) {
	m := setupReadyModel()
	m.contextMenu = &contextmenu.Menu{
		X: 10, Y: 10,
		Items: []contextmenu.Item{
			{Label: "Test", Action: "test"},
		},
		Visible: true,
	}
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with context menu")
	}
}

func TestViewWithModalHelp(t *testing.T) {
	m := setupReadyModel()
	m.modal = m.helpOverlay()
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with help modal")
	}
}

func TestViewWithModalAbout(t *testing.T) {
	m := setupReadyModel()
	m.modal = m.aboutOverlay()
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with about modal")
	}
}

func TestViewWithExposeMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.exposeMode = true
	v := m.View()
	if v.Content == "" {
		t.Error("expected content in expose mode")
	}
}

func TestViewWithConfirmDialog(t *testing.T) {
	m := setupReadyModel()
	m.confirmClose = &ConfirmDialog{Title: "Close?", IsQuit: true}
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with confirm dialog")
	}
}

func TestViewWithRenameDialog(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	m.renameDialog = &RenameDialog{
		WindowID: w.ID,
		Text:     []rune("New Title"),
		Cursor:   9,
	}
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with rename dialog")
	}
}

func TestViewWithWorkspacePicker(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = []string{"/tmp/ws1.toml", "/tmp/ws2.toml"}
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with workspace picker")
	}
}

func TestViewWithTourActive(t *testing.T) {
	m := setupReadyModel()
	m.tour.Active = true
	m.tour.Current = 0
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with tour active")
	}
}

func TestViewWithDockFocused(t *testing.T) {
	m := setupReadyModel()
	m.dockFocused = true
	m.dock.SetHover(0)
	v := m.View()
	if v.Content == "" {
		t.Error("expected content with dock focused")
	}
}

// ── getTooltipAt tests ──

func TestGetTooltipAtDock(t *testing.T) {
	m := setupReadyModel()
	if len(m.dock.Items) == 0 {
		t.Skip("no dock items to test")
	}
	// Hover over positions in the dock row; verify no panic
	_ = m.getTooltipAt(5, m.height-1)
}

func TestGetTooltipAtEmpty(t *testing.T) {
	m := setupReadyModel()
	// Middle of empty desktop should return ""
	tip := m.getTooltipAt(m.width/2, m.height/2)
	if tip != "" {
		t.Errorf("expected empty tooltip for empty desktop, got %q", tip)
	}
}

func TestGetTooltipAtMenuBar(t *testing.T) {
	m := setupReadyModel()
	// Menu bar is at y=0; verify no panic
	_ = m.getTooltipAt(2, 0)
}

func TestGetTooltipAtWindowTitleBar(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Skip("no window")
	}
	cx := w.Rect.X + w.Rect.Width/2
	cy := w.Rect.Y
	tip := m.getTooltipAt(cx, cy)
	if tip == "" {
		t.Error("expected non-empty tooltip on window title bar")
	}
}

// ── showKeyboardTooltip tests ──

func TestShowKeyboardTooltipWithWindow(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}
	m.showKeyboardTooltip()
	if m.tooltipText == "" {
		t.Error("expected non-empty tooltip text")
	}
	if m.tooltipText != w.Title {
		t.Errorf("tooltip = %q, want %q", m.tooltipText, w.Title)
	}
}

func TestShowKeyboardTooltipNoWindow(t *testing.T) {
	m := setupReadyModel()
	m.showKeyboardTooltip()
	if m.tooltipText == "" {
		t.Error("expected hint tooltip when no window focused")
	}
	if !strings.Contains(m.tooltipText, "terminal") {
		t.Errorf("expected hint about terminal, got %q", m.tooltipText)
	}
}

// ── Update() additional branch tests ──

func TestPasteMsgInTerminalMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeTerminal
	// Without a real terminal, verify it does not panic
	updated, cmd := m.Update(tea.PasteMsg{Content: "hello paste"})
	if cmd != nil {
		t.Error("expected nil cmd from PasteMsg without terminal")
	}
	_ = updated
}

func TestPtyOutputMsgUnfocusedNotification(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	windows := m.wm.Windows()
	if len(windows) < 2 {
		t.Fatal("need at least 2 windows")
	}
	firstID := windows[0].ID

	// Send PtyOutputMsg for the first (unfocused) window
	updated, _ := m.Update(PtyOutputMsg{WindowID: firstID})
	model := updated.(Model)
	w := model.wm.WindowByID(firstID)
	if w == nil {
		t.Fatal("window not found")
	}
	if !w.HasNotification {
		t.Error("expected HasNotification=true for unfocused window")
	}
}

func TestPtyClosedMsgMarksExited(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}
	wid := w.ID

	updated, _ := m.Update(PtyClosedMsg{WindowID: wid, Err: nil})
	model := updated.(Model)

	// Window should still exist but be marked as exited
	ew := model.wm.WindowByID(wid)
	if ew == nil {
		t.Fatal("expected window to still exist after PtyClosedMsg")
	}
	if !ew.Exited {
		t.Error("expected window to be marked as exited")
	}
}

func TestCursorBlinkMsgNoop(t *testing.T) {
	// Cursor blinking is handled natively by tea.Cursor — CursorBlinkMsg is a no-op.
	m := setupReadyModel()
	updated, cmd := m.Update(CursorBlinkMsg{})
	_ = updated.(Model)
	if cmd != nil {
		t.Error("CursorBlinkMsg should return nil cmd (cursor handled natively)")
	}
}

func TestAnimationTickMsgWithAnimations(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected focused window")
	}

	from := w.Rect
	to := geometry.Rect{X: from.X + 10, Y: from.Y + 10, Width: from.Width, Height: from.Height}
	m.startWindowAnimation(w.ID, AnimSnap, from, to)

	if !m.hasActiveAnimations() {
		t.Fatal("expected active animations")
	}

	now := time.Now()
	updated, cmd := m.Update(AnimationTickMsg{Time: now})
	model := updated.(Model)
	if model.hasActiveAnimations() && cmd == nil {
		t.Error("expected tick cmd while animation is active")
	}
}

func TestAnimationTickMsgNoAnimations(t *testing.T) {
	m := setupReadyModel()
	now := time.Now()
	updated, cmd := m.Update(AnimationTickMsg{Time: now})
	if cmd != nil {
		t.Error("expected nil cmd when no animations are active")
	}
	_ = updated
}

func TestCleanupMsg(t *testing.T) {
	m := setupReadyModel()
	updated, cmd := m.Update(CleanupMsg{Time: time.Now()})
	if cmd == nil {
		t.Error("expected tick cmd from CleanupMsg (schedules next cleanup)")
	}
	_ = updated
}

// ── Additional animation helper tests ──

func TestSettledAnimation(t *testing.T) {
	target := geometry.Rect{X: 50, Y: 50, Width: 80, Height: 24}
	a := &Animation{
		X: 50.0, Y: 50.0, W: 80.0, H: 24.0,
		EndRect: target,
	}
	if !settled(a) {
		t.Error("expected animation to be settled when at target with no velocity")
	}
}

func TestNotSettledAnimation(t *testing.T) {
	target := geometry.Rect{X: 50, Y: 50, Width: 80, Height: 24}
	a := &Animation{
		X: 10.0, Y: 10.0, W: 40.0, H: 12.0,
		EndRect: target,
		VX:      5, VY: 5, VW: 5, VH: 5,
	}
	if settled(a) {
		t.Error("expected animation not to be settled when far from target")
	}
}

func TestIsAnimatingClose(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected window")
	}

	if m.isAnimatingClose(w.ID) {
		t.Error("should not be animating close initially")
	}

	center := geometry.Rect{X: w.Rect.X + w.Rect.Width/2, Y: w.Rect.Y + w.Rect.Height/2, Width: 1, Height: 1}
	m.startWindowAnimation(w.ID, AnimClose, w.Rect, center)

	if !m.isAnimatingClose(w.ID) {
		t.Error("expected isAnimatingClose=true after starting close animation")
	}
}

func TestHasExposeAnimations(t *testing.T) {
	m := setupReadyModel()
	if m.hasExposeAnimations() {
		t.Error("expected no expose animations initially")
	}

	m.animationsOn = true
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected window")
	}
	m.startExposeAnimation(w.ID, AnimExpose,
		w.Rect,
		geometry.Rect{X: 10, Y: 10, Width: 30, Height: 10})

	if !m.hasExposeAnimations() {
		t.Error("expected expose animations after startExposeAnimation")
	}
}

func TestViewWithExposeAnimations(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	w := m.wm.FocusedWindow()
	if w == nil {
		t.Fatal("expected window")
	}
	m.startExposeAnimation(w.ID, AnimExpose,
		w.Rect,
		geometry.Rect{X: 10, Y: 5, Width: 30, Height: 10})

	v := m.View()
	if v.Content == "" {
		t.Error("expected content during expose transition")
	}
}

// ── InputMode String tests ──

func TestInputModeString(t *testing.T) {
	tests := []struct {
		mode InputMode
		want string
	}{
		{ModeNormal, "NORMAL"},
		{ModeTerminal, "TERMINAL"},
		{ModeCopy, "COPY"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("InputMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// ── Window limit test ──

func TestMaxWindowsLimit(t *testing.T) {
	m := setupReadyModel()
	for i := 0; i < maxWindows; i++ {
		m.openDemoWindow()
	}
	if m.wm.Count() != maxWindows {
		t.Fatalf("expected %d windows, got %d", maxWindows, m.wm.Count())
	}
	cmd := m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/echo"})
	if cmd != nil {
		t.Error("expected nil cmd when at max windows")
	}
}

// ── View cache test ──

func TestViewCache(t *testing.T) {
	m := setupReadyModel()
	v1 := m.View()
	v2 := m.View()
	if v1.AltScreen != v2.AltScreen {
		t.Error("expected same AltScreen from cached view")
	}
}

// ── helpOverlay test ──

func TestHelpOverlay(t *testing.T) {
	m := setupReadyModel()
	overlay := m.helpOverlay()
	if overlay == nil {
		t.Fatal("expected non-nil overlay")
	}
	if overlay.Title != "Help" {
		t.Errorf("Title = %q, want 'Help'", overlay.Title)
	}
	if len(overlay.Tabs) != 6 {
		t.Errorf("expected 6 tabs, got %d", len(overlay.Tabs))
	}
}

// ── Spring tests ──

func TestSpringPresetsAllStyles(t *testing.T) {
	speeds := []string{"slow", "normal", "fast"}
	styles := []string{"smooth", "snappy", "bouncy"}
	for _, speed := range speeds {
		for _, style := range styles {
			snappy, bouncy, smooth, expose := springPresets(speed, style)
			_ = snappy
			_ = bouncy
			_ = smooth
			_ = expose
		}
	}
}

func TestNewSpringCache(t *testing.T) {
	c := newSpringCache("normal", "snappy")
	if c == nil {
		t.Fatal("expected non-nil spring cache")
	}
}

func TestSpringForType(t *testing.T) {
	m := setupReadyModel()
	types := []AnimationType{AnimOpen, AnimClose, AnimMaximize, AnimRestore, AnimSnap, AnimTile, AnimMinimize, AnimRestore2, AnimExpose, AnimExposeExit}
	for _, typ := range types {
		_ = m.springForType(typ)
	}
}

// ── updateAnimations dock pulse tests ──

func TestUpdateAnimationsDockPulseDone(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.startDockPulse(0)

	start := m.animations[0].StartTime
	now := start.Add(500 * time.Millisecond) // past dockPulseDur (400ms)
	hasActive := m.updateAnimations(now)
	if hasActive {
		t.Error("expected dock pulse to be done after 500ms")
	}
}

func TestUpdateAnimationsDockPulseInProgress(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.startDockPulse(0)

	start := m.animations[0].StartTime
	now := start.Add(200 * time.Millisecond) // 50% of 400ms
	hasActive := m.updateAnimations(now)
	if !hasActive {
		t.Error("expected dock pulse to still be active at 50%")
	}
	if m.animations[0].Progress <= 0 {
		t.Error("expected non-zero progress during pulse")
	}
}

// --- tickResizeSettle, tickWorkspaceAutoSave, tickCleanup tests ---
// Note: these are tea.Tick commands that block for their duration. We can only verify
// they return non-nil commands (calling cmd() would block for the timer duration).

func TestTickResizeSettleReturnsCmd(t *testing.T) {
	cmd := tickResizeSettle()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from tickResizeSettle")
	}
}

func TestTickWorkspaceAutoSaveReturnsCmd(t *testing.T) {
	cmd := tickWorkspaceAutoSave()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from tickWorkspaceAutoSave")
	}
}

func TestTickCleanupReturnsCmd(t *testing.T) {
	cmd := tickCleanup()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from tickCleanup")
	}
}

// --- TabLabel tests ---

func TestTabLabel(t *testing.T) {
	mo := &ModalOverlay{
		Tabs: []HelpTab{
			{Title: "Keys"},
			{Title: "Mouse"},
			{Title: "Tips"},
		},
	}

	tests := []struct {
		index int
		want  string
	}{
		{0, "Keys [1]"},
		{1, "Mouse [2]"},
		{2, "Tips [3]"},
		{-1, ""},
		{3, ""},
	}
	for _, tc := range tests {
		got := mo.TabLabel(tc.index)
		if got != tc.want {
			t.Errorf("TabLabel(%d) = %q, want %q", tc.index, got, tc.want)
		}
	}
}

func TestTabLabelNoTabs(t *testing.T) {
	mo := &ModalOverlay{Lines: []string{"line1"}}
	if got := mo.TabLabel(0); got != "" {
		t.Errorf("TabLabel(0) with no tabs = %q, want empty", got)
	}
}

// --- TabAtX tests ---

func TestTabAtX(t *testing.T) {
	mo := &ModalOverlay{
		Tabs: []HelpTab{
			{Title: "Keys"},
			{Title: "Mouse"},
		},
	}

	// First tab label is "1·Keys" which has some width
	if idx := mo.TabAtX(0); idx != 0 {
		t.Errorf("TabAtX(0) = %d, want 0 (first tab)", idx)
	}

	// Large X should return -1 (past all tabs)
	if idx := mo.TabAtX(1000); idx != -1 {
		t.Errorf("TabAtX(1000) = %d, want -1 (past all tabs)", idx)
	}
}

func TestTabAtXNoTabs(t *testing.T) {
	mo := &ModalOverlay{Lines: []string{"line1"}}
	if idx := mo.TabAtX(0); idx != -1 {
		t.Errorf("TabAtX(0) with nil tabs = %d, want -1", idx)
	}
}

func TestTabAtXSecondTab(t *testing.T) {
	mo := &ModalOverlay{
		Tabs: []HelpTab{
			{Title: "A"},
			{Title: "B"},
		},
	}
	// First tab "1·A" = 3 runes + 2 padding = 5 chars
	// Second tab starts at x=5
	firstTabW := runeLen(mo.TabLabel(0)) + 2
	if idx := mo.TabAtX(firstTabW); idx != 1 {
		t.Errorf("TabAtX(%d) = %d, want 1 (second tab)", firstTabW, idx)
	}
}

// --- Bounds tests ---

func TestModalBoundsSimple(t *testing.T) {
	mo := &ModalOverlay{
		Title: "About",
		Lines: []string{"Line 1", "Line 2", "Line 3"},
	}
	b := mo.Bounds(120, 40)
	if b.BoxW <= 0 || b.BoxH <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", b.BoxW, b.BoxH)
	}
	if b.StartX < 0 || b.StartY < 0 {
		t.Errorf("expected non-negative start, got (%d,%d)", b.StartX, b.StartY)
	}
	if b.TabRow != -1 {
		t.Errorf("expected TabRow=-1 for non-tabbed modal, got %d", b.TabRow)
	}
}

func TestModalBoundsWithTabs(t *testing.T) {
	mo := &ModalOverlay{
		Title: "Help",
		Tabs: []HelpTab{
			{Title: "Keys", Lines: []string{"a - do thing"}},
			{Title: "Mouse", Lines: []string{"click - do thing"}},
		},
	}
	b := mo.Bounds(120, 40)
	if b.TabRow < 0 {
		t.Errorf("expected TabRow >= 0 for tabbed modal, got %d", b.TabRow)
	}
	if b.BoxW <= 0 || b.BoxH <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", b.BoxW, b.BoxH)
	}
}

func TestModalBoundsSmallScreen(t *testing.T) {
	mo := &ModalOverlay{
		Title: "A very long title that exceeds the screen width easily",
		Lines: []string{"Short line"},
	}
	b := mo.Bounds(20, 20)
	// Should clamp to screen bounds
	if b.StartX < 0 {
		t.Errorf("StartX should be >= 0 on small screen, got %d", b.StartX)
	}
	if b.StartY < 0 {
		t.Errorf("StartY should be >= 0 on small screen, got %d", b.StartY)
	}
}

// --- recordShowKey tests ---

func TestRecordShowKeyDisabled(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = false
	m.recordShowKey("q", "quit")
	if len(m.showKeysEvents) != 0 {
		t.Error("expected no events recorded when showKeys is disabled")
	}
}

func TestRecordShowKeyEmptyKey(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = true
	m.recordShowKey("", "quit")
	if len(m.showKeysEvents) != 0 {
		t.Error("expected no events recorded for empty key")
	}
}

func TestRecordShowKeyAddsEvent(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = true
	m.recordShowKey("q", "quit")
	if len(m.showKeysEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(m.showKeysEvents))
	}
	if m.showKeysEvents[0].Key != "q" {
		t.Errorf("expected key 'q', got %q", m.showKeysEvents[0].Key)
	}
	if m.showKeysEvents[0].Action != "quit" {
		t.Errorf("expected action 'quit', got %q", m.showKeysEvents[0].Action)
	}
}

func TestRecordShowKeyMaxEvents(t *testing.T) {
	m := setupReadyModel()
	m.showKeys = true
	// Add more than max events
	for i := 0; i < showKeysMaxEvents+5; i++ {
		m.recordShowKey("k", "action")
	}
	if len(m.showKeysEvents) != showKeysMaxEvents {
		t.Errorf("expected %d events (max), got %d", showKeysMaxEvents, len(m.showKeysEvents))
	}
}

// --- hasActiveAnimation tests ---

func TestHasActiveAnimationNone(t *testing.T) {
	m := setupReadyModel()
	if m.hasActiveAnimation("some-id") {
		t.Error("expected no active animation on fresh model")
	}
}

func TestHasActiveAnimationWithAnim(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	// Start an explicit animation
	from := fw.Rect
	to := geometry.Rect{X: 0, Y: 0, Width: 50, Height: 20}
	m.startWindowAnimation(fw.ID, AnimSnap, from, to)
	if !m.hasActiveAnimation(fw.ID) {
		t.Error("expected active animation after starting one")
	}
	// Non-matching ID should return false
	if m.hasActiveAnimation("nonexistent") {
		t.Error("expected no animation for non-existent ID")
	}
}

// --- handleUpdate message type tests ---

func TestHandleUpdateRawMsg(t *testing.T) {
	m := setupReadyModel()
	ret, cmd := m.Update(tea.RawMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for RawMsg")
	}
	_ = model
}

func TestHandleUpdateResizeRedrawMsg(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	// Add a window cache entry to verify it gets cleared
	m.windowCache[fw.ID] = &windowRenderCache{}

	ret, cmd := m.Update(ResizeRedrawMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for ResizeRedrawMsg")
	}
	if len(model.windowCache) != 0 {
		t.Error("expected window cache to be cleared after ResizeRedrawMsg")
	}
}

func TestHandleUpdateResizeSettleTickMsgStillSettling(t *testing.T) {
	m := setupReadyModel()
	m.lastWindowSizeAt = time.Now() // resize just happened

	ret, cmd := m.Update(ResizeSettleTickMsg{Time: time.Now()})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd while still settling")
	}
	_ = model
}

func TestHandleUpdateResizeSettleTickMsgSettled(t *testing.T) {
	m := setupReadyModel()
	m.lastWindowSizeAt = time.Now().Add(-2 * time.Second) // long ago

	ret, cmd := m.Update(ResizeSettleTickMsg{Time: time.Now()})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd after resize settled")
	}
	_ = model
}

func TestHandleUpdateCleanupMsg(t *testing.T) {
	m := setupReadyModel()
	ret, cmd := m.Update(CleanupMsg{Time: time.Now()})
	model := ret.(Model)
	if cmd == nil {
		t.Error("expected non-nil cmd from CleanupMsg (re-schedules tick)")
	}
	_ = model
}

func TestHandleUpdateKittyFlushMsg(t *testing.T) {
	m := setupReadyModel()
	m.kittyPending = &kittyPendingBuf{}

	// KittyFlushMsg appends data to kittyPending, then the deferred flush
	// at the end of Update() converts it to a tea.Raw cmd and clears the buffer.
	_, cmd := m.Update(KittyFlushMsg{Data: []byte("test-data")})
	// The cmd should be non-nil (tea.Raw wrapping the data)
	if cmd == nil {
		t.Error("expected non-nil cmd from KittyFlushMsg with data")
	}
}

func TestHandleUpdateKittyFlushMsgEmpty(t *testing.T) {
	m := setupReadyModel()
	m.kittyPending = &kittyPendingBuf{}

	ret, _ := m.Update(KittyFlushMsg{Data: nil})
	model := ret.(Model)
	if len(model.kittyPending.data) != 0 {
		t.Error("expected kittyPending to be empty for nil data")
	}
}

func TestHandleUpdateCustomWidgetResultMsg(t *testing.T) {
	m := setupReadyModel()
	cw := &widget.ShellWidget{WidgetName: "test", Command: "echo hi"}
	m.customWidgets = map[string]*widget.ShellWidget{"test": cw}

	ret, cmd := m.Update(CustomWidgetResultMsg{
		Results: map[string]string{"test": "hello"},
	})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from CustomWidgetResultMsg")
	}
	// ShellWidget.Render() returns the formatted output
	got := model.customWidgets["test"].Render()
	if got != "hello" {
		t.Errorf("expected custom widget render 'hello', got %q", got)
	}
}

func TestHandleUpdatePasteMsg(t *testing.T) {
	m := setupReadyModel()
	// PasteMsg in non-terminal mode should do nothing
	ret, cmd := m.Update(tea.PasteMsg{Content: "test"})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from PasteMsg in normal mode")
	}
	_ = model
}

func TestHandleUpdateCursorBlinkMsg(t *testing.T) {
	m := setupReadyModel()
	ret, cmd := m.Update(CursorBlinkMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd from CursorBlinkMsg")
	}
	_ = model
}

func TestHandleUpdateAnimationTickMsgNoAnims(t *testing.T) {
	m := setupReadyModel()
	ret, cmd := m.Update(AnimationTickMsg{Time: time.Now()})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when no animations are active")
	}
	_ = model
}

func TestHandleUpdateAnimationTickMsgWithAnim(t *testing.T) {
	m := setupReadyModel()
	m.animationsOn = true
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	from := fw.Rect
	to := geometry.Rect{X: 10, Y: 5, Width: 60, Height: 20}
	m.startWindowAnimation(fw.ID, AnimSnap, from, to)

	ret, cmd := m.Update(AnimationTickMsg{Time: time.Now()})
	model := ret.(Model)
	// Animation just started so it should still be active
	if cmd == nil {
		t.Error("expected non-nil cmd when animations still active")
	}
	_ = model
}

func TestHandleUpdatePtyOutputMsgFirstOutput(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()
	fw.Stuck = true // mark as stuck

	ret, _ := m.Update(PtyOutputMsg{WindowID: fw.ID})
	model := ret.(Model)
	if !model.termHasOutput[fw.ID] {
		t.Error("expected termHasOutput to be true after PtyOutputMsg")
	}
	if model.wm.WindowByID(fw.ID).Stuck {
		t.Error("expected Stuck to be cleared after first output")
	}
}

func TestHandleUpdatePtyOutputMsgUnfocusedNotification(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	w1 := m.wm.Windows()[0]
	// w2 is focused, w1 is unfocused

	ret, _ := m.Update(PtyOutputMsg{WindowID: w1.ID})
	model := ret.(Model)
	if !model.wm.WindowByID(w1.ID).HasNotification {
		t.Error("expected HasNotification for unfocused window after PtyOutputMsg")
	}
}

func TestHandleUpdatePtyClosedMsgMarksExited(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	fw := m.wm.FocusedWindow()

	ret, _ := m.Update(PtyClosedMsg{WindowID: fw.ID, Err: nil})
	model := ret.(Model)
	w := model.wm.WindowByID(fw.ID)
	if w == nil {
		t.Fatal("expected window to still exist")
	}
	if !w.Exited {
		t.Error("expected window to be marked as exited")
	}
	if !strings.Contains(w.Title, "[exited]") {
		t.Errorf("expected title to contain [exited], got %q", w.Title)
	}
}

func TestHandleUpdateWorkspaceDiscoveryMsgVisible(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true

	ret, _ := m.Update(WorkspaceDiscoveryMsg{
		Workspaces:   []string{"/a/ws.toml", "/b/ws.toml"},
		WindowCounts: []int{3, 1},
	})
	model := ret.(Model)
	if len(model.workspaceList) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(model.workspaceList))
	}
	if model.workspaceWindowCounts[0] != 3 {
		t.Errorf("expected window count 3, got %d", model.workspaceWindowCounts[0])
	}
}

func TestHandleUpdateWorkspaceDiscoveryMsgEmpty(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true

	ret, _ := m.Update(WorkspaceDiscoveryMsg{
		Workspaces: nil,
	})
	model := ret.(Model)
	if model.workspacePickerVisible {
		t.Error("expected workspace picker to be hidden with empty results")
	}
}

func TestHandleUpdateWorkspaceDiscoveryMsgNotVisible(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = false

	ret, _ := m.Update(WorkspaceDiscoveryMsg{
		Workspaces:   []string{"/a/ws.toml"},
		WindowCounts: []int{1},
	})
	model := ret.(Model)
	// Should not update workspace list when picker is not visible
	if len(model.workspaceList) != 0 {
		t.Error("expected workspace list to remain empty when picker not visible")
	}
}

func TestHandleUpdateUnknownMsg(t *testing.T) {
	m := setupReadyModel()
	type unknownMsg struct{}
	ret, cmd := m.Update(unknownMsg{})
	model := ret.(Model)
	if cmd != nil {
		t.Error("expected nil cmd for unknown message type")
	}
	_ = model
}

// --- getTooltipAt tests ---

func TestGetTooltipAtDockRow(t *testing.T) {
	m := setupReadyModel()
	// Bottom row is the dock
	tip := m.getTooltipAt(2, m.height-1)
	// Should return a dock item label or empty string
	_ = tip // We just verify it doesn't panic
}

func TestGetTooltipAtMenuBarRow(t *testing.T) {
	m := setupReadyModel()
	// Row 0 is the menu bar
	tip := m.getTooltipAt(2, 0)
	// Menu bar items should return something
	if tip != "" && !strings.Contains(tip, "menu") {
		t.Logf("menu bar tooltip: %q", tip)
	}
}

func TestGetTooltipAtDesktopArea(t *testing.T) {
	m := setupReadyModel()
	// Middle of screen with no windows
	tip := m.getTooltipAt(60, 20)
	if tip != "" {
		t.Errorf("expected empty tooltip on empty desktop, got %q", tip)
	}
}

// --- closeAllTerminals tests ---

func TestCloseAllTerminalsEmpty(t *testing.T) {
	m := setupReadyModel()
	m.closeAllTerminals()
	if len(m.terminals) != 0 {
		t.Error("expected empty terminals after close")
	}
}

func TestCloseAllTerminalsWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	if len(m.terminals) < 2 {
		t.Skip("demo windows don't create real terminals")
	}
	m.closeAllTerminals()
	if len(m.terminals) != 0 {
		t.Error("expected empty terminals after close all")
	}
}

// --- WindowSizeMsg in handleUpdate ---

func TestHandleUpdateWindowSizeMsg(t *testing.T) {
	m := New()
	m.tour.Skip()
	ret, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	model := ret.(Model)
	if model.width != 100 || model.height != 50 {
		t.Errorf("expected 100x50, got %dx%d", model.width, model.height)
	}
	if !model.ready {
		t.Error("expected ready after WindowSizeMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (resize settle tick)")
	}
}

func TestHandleUpdateWindowSizeMsgExitsCopyMode(t *testing.T) {
	m := setupReadyModel()
	m.inputMode = ModeCopy
	m.scrollOffset = 5
	m.selActive = true

	ret, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	model := ret.(Model)
	if model.inputMode != ModeNormal {
		t.Errorf("expected ModeNormal after resize in copy mode, got %s", model.inputMode)
	}
	if model.scrollOffset != 0 {
		t.Error("expected scrollOffset to be reset after resize")
	}
	if model.selActive {
		t.Error("expected selActive to be false after resize")
	}
}
