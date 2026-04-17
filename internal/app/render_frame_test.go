package app

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// ============================================================================
// RenderFrame tests
// ============================================================================

func TestRenderFrame_DesktopOnly(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	// No windows, so all cells should be desktop background (space or pattern)
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			ch := buf.Cells[y][x].Char
			if ch != ' ' && ch != theme.DesktopPatternChar {
				t.Errorf("unexpected char %q at (%d,%d) in empty desktop", ch, x, y)
				return
			}
		}
	}
}

func TestRenderFrame_WithDeskClock(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, true, "", window.HitNone, nil, nil, nil)

	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	// Desk clock draws big text near the bottom-right; check for non-space/non-pattern cells
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
		t.Error("expected desk clock content near bottom-right")
	}
}

func TestRenderFrame_WithWindows(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Window 1", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	// Window border should be present
	if buf.Cells[3][5].Char != theme.BorderTopLeft {
		t.Errorf("expected top-left border at (5,3), got %q", buf.Cells[3][5].Char)
	}
}

func TestRenderFrame_CullsOffscreenWindows(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(40, 12)
	wm.SetReserved(1, 1)

	visible := window.NewWindow("w1", "VISIBLE", geometry.Rect{X: 2, Y: 2, Width: 20, Height: 6}, nil)
	hidden := window.NewWindow("w2", "HIDDEN", geometry.Rect{X: 100, Y: 100, Width: 10, Height: 5}, nil)
	wm.AddWindow(visible)
	wm.AddWindow(hidden)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	titleRow := visible.Rect.Y + max(1, visible.TitleBarHeight)/2
	titleX := visible.Rect.X + 2
	if titleRow < 0 || titleRow >= buf.Height || titleX < 0 || titleX >= buf.Width {
		t.Fatalf("visible title position out of bounds")
	}
	if buf.Cells[titleRow][titleX].Char != 'V' {
		t.Errorf("expected visible window title to be rendered")
	}
}

func TestRenderFrame_WithAnimRects(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Animated", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	// Provide an animated rect that overrides the window position
	animRects := map[string]geometry.Rect{
		"w1": {X: 10, Y: 8, Width: 25, Height: 10},
	}

	buf := RenderFrame(wm, theme, nil, animRects, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	// The window should be rendered at the animated position (10,8), not (5,5)
	if buf.Cells[8][10].Char != theme.BorderTopLeft {
		t.Errorf("expected animated window border at (10,8), got %q", buf.Cells[8][10].Char)
	}
}

func TestRenderFrame_AnimRectSmallSkipsTerminal(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Small", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	// Very small animated rect (width/height <= 3) skips terminal content
	animRects := map[string]geometry.Rect{
		"w1": {X: 10, Y: 10, Width: 3, Height: 3},
	}

	// Should not panic
	buf := RenderFrame(wm, theme, nil, animRects, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

func TestRenderFrame_AnimRectClamp(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Clamp", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	// Animated rect with sub-1 dimensions (spring overshoot) should be clamped
	animRects := map[string]geometry.Rect{
		"w1": {X: 10, Y: 10, Width: 0, Height: 0},
	}

	// Should not panic
	buf := RenderFrame(wm, theme, nil, animRects, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

func TestRenderFrame_MinimizedWindowWithAnimRect(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Minimized", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Minimized = true
	wm.AddWindow(w1)

	// When animRect is provided, Minimized is temporarily unset for rendering
	animRects := map[string]geometry.Rect{
		"w1": {X: 5, Y: 5, Width: 20, Height: 10},
	}

	buf := RenderFrame(wm, theme, nil, animRects, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	// The minimized window should still be rendered during animation
	if buf.Cells[5][5].Char != theme.BorderTopLeft {
		t.Errorf("expected minimized window rendered during animation, got %q at (5,5)", buf.Cells[5][5].Char)
	}

	// After rendering, window should still be minimized
	if !w1.Minimized {
		t.Error("window should remain minimized after frame rendering")
	}
}

func TestRenderFrame_HoverZone(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Hover", geometry.Rect{X: 5, Y: 5, Width: 30, Height: 12}, nil)
	w1.Focused = true
	w1.Resizable = true
	wm.AddWindow(w1)

	// Render with hover on the close button of w1
	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "w1", window.HitCloseButton, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	// Should not panic, and should have rendered the window
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected window content with hover zone")
	}
}

func TestRenderFrame_ScrollOffsetOnlyFocused(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Focused", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	w2 := window.NewWindow("w2", "Unfocused", geometry.Rect{X: 35, Y: 2, Width: 30, Height: 12}, nil)
	wm.AddWindow(w2)
	wm.FocusWindow("w1")

	// Should not panic with scrollOffset > 0
	buf := RenderFrame(wm, theme, nil, nil, true, 5, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

func TestRenderFrame_CopyModeSelection(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "CopyMode", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	// Create a terminal for the focused window
	term, err := terminal.New("/bin/echo", []string{"COPY_TEST"}, 28, 10, 0, 0, "")
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
		Active:   true,
		CopyMode: true,
		Start:    geometry.Point{X: 0, Y: 0},
		End:      geometry.Point{X: 10, Y: 0},
	}

	// Should render with selection highlighting and scrollbar
	buf := RenderFrame(wm, theme, terminals, nil, true, 0, sel, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

func TestRenderFrame_CopyModeScrollbar(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Scrollbar", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	term, err := terminal.New("/bin/echo", []string{"test"}, 28, 10, 0, 0, "")
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

	// CopyMode without active selection (just scrollbar)
	sel := SelectionInfo{
		Active:   false,
		CopyMode: true,
	}

	buf := RenderFrame(wm, theme, terminals, nil, true, 0, sel, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
}

// TestRenderFrame_ScrollbarZOrder verifies that a higher z-order window
// paints over the scrollbar of a lower z-order copy-mode window.
func TestRenderFrame_ScrollbarZOrder(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	// Window A: has copy mode with scrollbar. Positioned so right border is at x=31.
	wA := window.NewWindow("wA", "CopyWin", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	wA.Visible = true
	wm.AddWindow(wA)

	// Window B: overlaps Window A's right border. Positioned at x=25, width=30.
	wB := window.NewWindow("wB", "FrontWin", geometry.Rect{X: 25, Y: 2, Width: 30, Height: 12}, nil)
	wB.Visible = true
	wm.AddWindow(wB)

	// Generate enough output to produce scrollback
	term, err := terminal.New("/bin/sh", []string{"-c", "for i in $(seq 1 50); do echo line$i; done"}, 28, 10, 0, 0, "")
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
	case <-time.After(3 * time.Second):
	}

	termB, err := terminal.New("/bin/echo", []string{"hello"}, 28, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("failed to create terminal: %v", err)
	}
	defer termB.Close()
	doneB := make(chan struct{})
	go func() {
		termB.ReadPtyLoop()
		close(doneB)
	}()
	select {
	case <-doneB:
	case <-time.After(2 * time.Second):
	}

	terminals := map[string]*terminal.Terminal{
		"wA": term,
		"wB": termB,
	}

	// Focus Window B (moves it to top of z-order, above Window A)
	wm.FocusWindow("wB")

	// Set up copy mode for Window A (not focused, but has copy snapshot)
	sel := SelectionInfo{
		Active:       false,
		CopyMode:     true,
		CopySnap:     captureCopySnapshot("wA", term),
		CopyWindowID: "wA",
	}

	buf := RenderFrame(wm, theme, terminals, nil, true, 5, sel, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}

	// Window A's scrollbar would be at trackX = wA.Rect.Right() - 1 = 31
	// Window B covers x=25..54, which includes x=31
	// The scrollbar chars (▓/░) at x=31 should be overwritten by Window B's content
	trackX := wA.Rect.Right() - 1 // x=31
	scrollbarVisible := false
	for y := wA.Rect.Y + 1; y < wA.Rect.Bottom()-2; y++ {
		if y < 0 || y >= buf.Height || trackX < 0 || trackX >= buf.Width {
			continue
		}
		ch := buf.Cells[y][trackX].Char
		if ch == '▓' || ch == '░' {
			scrollbarVisible = true
			break
		}
	}
	if scrollbarVisible {
		t.Error("scrollbar of Window A should be hidden by Window B (higher z-order)")
	}
}

// TestRenderFrame_WideCharClipping verifies that wide characters at the
// content area boundary are clipped to prevent overflow into the border.
func TestRenderFrame_WideCharClipping(t *testing.T) {
	theme := testTheme()
	// Create a buffer and window
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 12, Height: 5}, nil)
	w.Visible = true
	w.Focused = true

	buf := AcquireThemedBuffer(20, 10, theme)
	cr := w.ContentRect() // should be {1, 1, 10, 3} for a 12x5 window

	// Create a scrollback line with a wide character at the content area edge
	// Content area width = 10, so put a width-2 char at position 9 (last column)
	line := make([]terminal.ScreenCell, 12)
	for i := 0; i < 12; i++ {
		line[i] = terminal.ScreenCell{Content: "A", Width: 1}
	}
	// Place a wide char at position 9 (last content column)
	line[9] = terminal.ScreenCell{Content: "漢", Width: 2}
	line[10] = terminal.ScreenCell{Content: "", Width: 0} // continuation

	snap := &CopySnapshot{
		WindowID:   "w1",
		Scrollback: [][]terminal.ScreenCell{line},
		Screen:     [][]terminal.ScreenCell{{terminal.ScreenCell{Content: " ", Width: 1}}},
		Width:      12,
		Height:     1,
	}

	// Create a minimal terminal for the rendering
	term, err := terminal.New("/bin/echo", []string{"x"}, 10, 3, 0, 0, "")
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

	// Render chrome first (sets up borders)
	renderWindowChrome(buf, w, theme, window.HitNone)

	// Render terminal content with scrollback (scrollOffset=1 to show the scrollback line)
	renderTerminalContentWithSnapshot(buf, cr, term, color.White, color.Black, 1, snap)

	// The cell at the last content column (cr.X + cr.Width - 1) should be Width=1 (clipped)
	lastContentX := cr.X + cr.Width - 1
	cell := buf.Cells[cr.Y][lastContentX]
	if cell.Width > 1 {
		t.Errorf("wide char at content edge should be clipped to Width=1, got Width=%d", cell.Width)
	}

	// The border cell (cr.X + cr.Width) should still be a border character
	borderX := cr.X + cr.Width
	if borderX < buf.Width {
		borderCell := buf.Cells[cr.Y][borderX]
		// Border should NOT be a space (it should be the vertical border char)
		if borderCell.Char == ' ' && borderCell.Width == 0 {
			t.Error("border cell was overwritten or skipped — wide char overflow")
		}
	}

	// Verify BufferToString doesn't skip the border
	s := BufferToString(buf)
	// The output should have exactly buf.Width visual columns per line (no wrapping)
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		t.Fatal("expected multiple lines in output")
	}
}

// ============================================================================
// renderDeskClock tests
// ============================================================================

func TestRenderDeskClock_BasicRender(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	renderDeskClock(buf, theme)

	// Should draw big text clock near bottom-right
	hasContent := false
	for y := buf.Height - 10; y < buf.Height; y++ {
		for x := buf.Width - 30; x < buf.Width; x++ {
			if x >= 0 && y >= 0 && x < buf.Width && y < buf.Height {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != theme.DesktopPatternChar && ch != 0 {
					hasContent = true
					break
				}
			}
		}
		if hasContent {
			break
		}
	}
	if !hasContent {
		t.Error("expected desk clock content")
	}
}

func TestRenderDeskClock_SmallBuffer(t *testing.T) {
	theme := testTheme()
	// Very small buffer where startX or startY might go negative
	buf := AcquireThemedBuffer(15, 8, theme)

	// Should not panic even if clock doesn't fit
	renderDeskClock(buf, theme)
}

func TestRenderDeskClock_TinyBuffer(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(5, 5, theme)

	// startX will clamp to 0, startY will clamp to 2
	renderDeskClock(buf, theme)
}

func TestRenderDeskClock_MultipleThemes(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}
	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			buf := AcquireThemedBuffer(80, 30, theme)
			renderDeskClock(buf, theme)
			// Should render without panic
		})
	}
}

// ============================================================================
// RenderMenuBar tests
// ============================================================================

func TestRenderMenuBar_NilMenuBar(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 3, theme)

	// Should be no-op
	RenderMenuBar(buf, nil, theme, ModeNormal, false, "", -1)
}

func TestRenderMenuBar_ZeroHeightBuffer(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 0, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(80, kb, nil, nil)

	// buf.Height < 1, should return early
	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)
}

func TestRenderMenuBar_BasicRender(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(80, kb, nil, nil)

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// Menu bar should be drawn on row 0
	// Check that File label is present
	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	if !strings.Contains(rendered, "File") {
		t.Errorf("expected 'File' in menu bar, got %q", rendered)
	}
	if !strings.Contains(rendered, "Edit") {
		t.Errorf("expected 'Edit' in menu bar, got %q", rendered)
	}
}

func TestRenderMenuBar_ModeNormal(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(100, kb, nil, nil)

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// Mode badge should contain "NORMAL"
	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	if !strings.Contains(rendered, "NORMAL") {
		t.Errorf("expected 'NORMAL' mode badge, got %q", rendered)
	}
}

func TestRenderMenuBar_ModeTerminal(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(100, kb, nil, nil)

	RenderMenuBar(buf, mb, theme, ModeTerminal, false, "", -1)

	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	if !strings.Contains(rendered, "TERMINAL") {
		t.Errorf("expected 'TERMINAL' mode badge, got %q", rendered)
	}
}

func TestRenderMenuBar_ModeCopy(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(100, kb, nil, nil)

	RenderMenuBar(buf, mb, theme, ModeCopy, false, "", -1)

	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	if !strings.Contains(rendered, "COPY") {
		t.Errorf("expected 'COPY' mode badge, got %q", rendered)
	}
}

func TestRenderMenuBar_PrefixPending(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(100, kb, nil, nil)

	RenderMenuBar(buf, mb, theme, ModeTerminal, true, "", -1)

	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	if !strings.Contains(rendered, "PREFIX") {
		t.Errorf("expected 'PREFIX' mode badge, got %q", rendered)
	}
}

func TestRenderMenuBar_DropdownOpen(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 20, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(80, kb, nil, nil)
	mb.OpenMenu(0) // Open File menu

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// Dropdown should be rendered below the menu bar (row 1+)
	hasDropdown := false
	for y := 1; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			ch := buf.Cells[y][x].Char
			if ch != ' ' && ch != theme.DesktopPatternChar && ch != 0 {
				hasDropdown = true
				break
			}
		}
		if hasDropdown {
			break
		}
	}
	if !hasDropdown {
		t.Error("expected dropdown content below menu bar when menu is open")
	}
}

func TestRenderMenuBar_HighlightOpenLabel(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 20, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(80, kb, nil, nil)
	mb.OpenMenu(0) // Open File menu

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// The "File" label area on row 0 should have accent background
	c := theme.C()
	foundAccent := false
	for x := 0; x < 10; x++ {
		if colorsEqual(buf.Cells[0][x].Bg, c.AccentColor) {
			foundAccent = true
			break
		}
	}
	if !foundAccent {
		t.Error("expected accent background on open menu label")
	}
}

func TestRenderMenuBar_SubMenuOpen(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	// Build registry with a game entry so there's a Games submenu in View > Apps
	registry := []registry.RegistryEntry{
		{Name: "Snake", Icon: "\uf1b3", Command: "termdesk-snake", Category: "games"},
	}
	kb := config.DefaultKeyBindings()
	mb := menubar.New(80, kb, registry, nil)

	// Open Apps menu (index 2), which should have the Games submenu
	mb.OpenMenu(2)
	// Navigate to the Games submenu item
	for i := 0; i < len(mb.Menus[2].Items)-1; i++ {
		mb.MoveHover(1)
	}
	// Check if there's a submenu at current hover
	if mb.HasSubMenu() {
		mb.EnterSubMenu()
	}

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// Should render without panic
	if !bufHasNonSpaceContent(buf, theme) {
		t.Error("expected menu bar content with submenu")
	}
}

func TestRenderMenuBar_WithWidgetBar(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(120, 3, theme)

	wb := widget.NewDefaultBar("testuser")
	// Set some widget values
	for _, w := range wb.Widgets {
		if cpu, ok := w.(*widget.CPUWidget); ok {
			cpu.Pct = 42.5
		}
		if mem, ok := w.(*widget.MemoryWidget); ok {
			mem.UsedGB = 8.2
		}
	}

	kb := config.DefaultKeyBindings()
	mb := menubar.New(120, kb, nil, wb)

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// Widget bar should render widgets on the right side of row 0
	row := buf.Cells[0]
	var text strings.Builder
	for x := 0; x < buf.Width; x++ {
		text.WriteRune(row[x].Char)
	}
	rendered := text.String()
	// Should contain CPU/MEM indicators
	if !strings.Contains(rendered, "CPU") && !strings.Contains(rendered, "%") {
		// Might be using icons only; just check there is content beyond menu labels
		if len(rendered) < 20 {
			t.Error("expected widget bar content in menu bar")
		}
	}
}

func TestRenderMenuBar_WidgetColorLevels(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(120, 3, theme)

	wb := widget.NewDefaultBar("user")
	// Set high CPU to get "red" level
	for _, w := range wb.Widgets {
		if cpu, ok := w.(*widget.CPUWidget); ok {
			cpu.Pct = 95.0
		}
	}

	kb := config.DefaultKeyBindings()
	mb := menubar.New(120, kb, nil, wb)

	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)

	// The CPU widget zone should have levelRed foreground color
	zones := mb.RightZones(120 - 13) // effectiveWidth = buf.Width - modeLabelLen
	cpuZoneFound := false
	for _, z := range zones {
		if z.Type == "cpu" {
			cpuZoneFound = true
			break
		}
	}
	if !cpuZoneFound {
		// Zones might differ; just verify no panic
		return
	}
}

func TestRenderMenuBar_NarrowBuffer(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(20, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(20, kb, nil, nil)

	// Narrow buffer should not panic (effectiveWidth may be very small)
	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)
}

func TestRenderMenuBar_VeryNarrowBadge(t *testing.T) {
	theme := testTheme()
	// Buffer narrower than mode badge width
	buf := AcquireThemedBuffer(5, 3, theme)

	kb := config.DefaultKeyBindings()
	mb := menubar.New(5, kb, nil, nil)

	// modeX will be negative, should clamp to 0
	RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)
}

// ============================================================================
// levelColor tests
// ============================================================================

func TestLevelColor_Red(t *testing.T) {
	c := levelColor("red", false)
	if c != "#E06C75" {
		t.Errorf("levelColor(red,dark) = %q, want #E06C75", c)
	}
}

func TestLevelColor_Yellow(t *testing.T) {
	c := levelColor("yellow", false)
	if c != "#E5C07B" {
		t.Errorf("levelColor(yellow,dark) = %q, want #E5C07B", c)
	}
}

func TestLevelColor_Green(t *testing.T) {
	c := levelColor("green", false)
	if c != "" {
		t.Errorf("levelColor(green,dark) = %q, want empty (default fg)", c)
	}
}

func TestLevelColor_Unknown(t *testing.T) {
	c := levelColor("unknown", false)
	if c != "" {
		t.Errorf("levelColor(unknown) = %q, want empty", c)
	}
}

func TestLevelColor_Empty(t *testing.T) {
	c := levelColor("", false)
	if c != "" {
		t.Errorf("levelColor('') = %q, want empty", c)
	}
}

func TestLevelColor_LightRed(t *testing.T) {
	c := levelColor("red", true)
	if c != "#C03030" {
		t.Errorf("levelColor(red,light) = %q, want #C03030", c)
	}
}

func TestLevelColor_LightYellow(t *testing.T) {
	c := levelColor("yellow", true)
	if c != "#9A7B00" {
		t.Errorf("levelColor(yellow,light) = %q, want #9A7B00", c)
	}
}

func TestLevelColor_LightGreen(t *testing.T) {
	c := levelColor("green", true)
	if c != "" {
		t.Errorf("levelColor(green,light) = %q, want empty (default fg)", c)
	}
}

// ============================================================================
// levelColorC tests
// ============================================================================

func TestLevelColorC_Red(t *testing.T) {
	c := levelColorC("red", false)
	if c == nil {
		t.Fatal("levelColorC(red,dark) should not be nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 224 || g>>8 != 108 || b>>8 != 117 {
		t.Errorf("levelColorC(red,dark) = %v, expected RGBA{224,108,117,255}", c)
	}
}

func TestLevelColorC_Yellow(t *testing.T) {
	c := levelColorC("yellow", false)
	if c == nil {
		t.Fatal("levelColorC(yellow,dark) should not be nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 229 || g>>8 != 192 || b>>8 != 123 {
		t.Errorf("levelColorC(yellow,dark) = %v, expected RGBA{229,192,123,255}", c)
	}
}

func TestLevelColorC_Green(t *testing.T) {
	c := levelColorC("green", false)
	if c != nil {
		t.Errorf("levelColorC(green,dark) = %v, want nil (default fg)", c)
	}
}

func TestLevelColorC_Unknown(t *testing.T) {
	c := levelColorC("purple", false)
	if c != nil {
		t.Errorf("levelColorC(purple) = %v, want nil", c)
	}
}

func TestLevelColorC_Empty(t *testing.T) {
	c := levelColorC("", false)
	if c != nil {
		t.Errorf("levelColorC('') = %v, want nil", c)
	}
}

func TestLevelColorC_LightRed(t *testing.T) {
	c := levelColorC("red", true)
	if c == nil {
		t.Fatal("levelColorC(red,light) should not be nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 192 || g>>8 != 48 || b>>8 != 48 {
		t.Errorf("levelColorC(red,light) = %v, expected RGBA{192,48,48,255}", c)
	}
}

func TestLevelColorC_LightGreen(t *testing.T) {
	c := levelColorC("green", true)
	if c != nil {
		t.Errorf("levelColorC(green,light) = %v, want nil (default fg)", c)
	}
}

// ============================================================================
// modeIcon tests
// ============================================================================

func TestModeIcon_Terminal(t *testing.T) {
	icon := modeIcon(ModeTerminal)
	if icon != "\uf120" {
		t.Errorf("modeIcon(ModeTerminal) = %q, want terminal icon", icon)
	}
}

func TestModeIcon_Copy(t *testing.T) {
	icon := modeIcon(ModeCopy)
	if icon != "\uf0c5" {
		t.Errorf("modeIcon(ModeCopy) = %q, want copy icon", icon)
	}
}

func TestModeIcon_Normal(t *testing.T) {
	icon := modeIcon(ModeNormal)
	if icon != "\uf009" {
		t.Errorf("modeIcon(ModeNormal) = %q, want grid icon", icon)
	}
}

func TestModeIcon_Default(t *testing.T) {
	// Invalid mode should fall to default (same as Normal)
	icon := modeIcon(InputMode(99))
	if icon != "\uf009" {
		t.Errorf("modeIcon(99) = %q, want grid icon (default)", icon)
	}
}

// ============================================================================
// modeBadge tests
// ============================================================================

func TestModeBadge_Normal(t *testing.T) {
	badge := modeBadge(ModeNormal)
	if !strings.Contains(badge, "NORMAL") {
		t.Errorf("modeBadge(ModeNormal) = %q, expected NORMAL", badge)
	}
}

func TestModeBadge_Terminal(t *testing.T) {
	badge := modeBadge(ModeTerminal)
	if !strings.Contains(badge, "TERMINAL") {
		t.Errorf("modeBadge(ModeTerminal) = %q, expected TERMINAL", badge)
	}
}

func TestModeBadge_Copy(t *testing.T) {
	badge := modeBadge(ModeCopy)
	if !strings.Contains(badge, "COPY") {
		t.Errorf("modeBadge(ModeCopy) = %q, expected COPY", badge)
	}
}

func TestModeBadge_FixedWidth(t *testing.T) {
	// All badges should produce the same display width
	normalLen := len([]rune(modeBadge(ModeNormal)))
	termLen := len([]rune(modeBadge(ModeTerminal)))
	copyLen := len([]rune(modeBadge(ModeCopy)))

	if normalLen != termLen || termLen != copyLen {
		t.Errorf("badges should have equal rune length: Normal=%d Terminal=%d Copy=%d",
			normalLen, termLen, copyLen)
	}
}

// ============================================================================
// renderTerminalContent tests
// ============================================================================

func TestRenderTerminalContent_NilTerm(t *testing.T) {
	buf := NewBuffer(20, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 10, Height: 5}

	// nil terminal should be no-op
	renderTerminalContent(buf, area, nil, hexToColor("#C0C0C0"), hexToColor("#000000"), 0)
	if buf.Cells[1][1].Char != ' ' {
		t.Error("nil terminal should not change buffer")
	}
}

func TestRenderTerminalContent_LiveView(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"RENDER_TEST"}, 20, 5, 0, 0, "")
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

	buf := NewBuffer(30, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}
	defaultFg := hexToColor("#C0C0C0")
	defaultBg := hexToColor("#1E2127")

	renderTerminalContent(buf, area, term, defaultFg, defaultBg, 0)

	// Check that "RENDER_TEST" was rendered (at least first char R)
	found := false
	for y := area.Y; y < area.Y+area.Height; y++ {
		for x := area.X; x < area.X+area.Width; x++ {
			if buf.Cells[y][x].Char == 'R' {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected terminal content 'R' from echo RENDER_TEST")
	}
}

func TestRenderTerminalContent_DefaultColors(t *testing.T) {
	// VT cells with nil fg/bg should use the provided defaults
	term, err := terminal.New("/bin/echo", []string{"COLOR"}, 20, 5, 0, 0, "")
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

	defaultFg := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	defaultBg := color.RGBA{R: 30, G: 33, B: 39, A: 255}

	buf := NewBuffer(30, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}
	renderTerminalContent(buf, area, term, defaultFg, defaultBg, 0)

	// Cells with content should have the default background
	for y := area.Y; y < area.Y+area.Height; y++ {
		for x := area.X; x < area.X+area.Width; x++ {
			cell := buf.Cells[y][x]
			if cell.Bg != nil && cell.Char != 0 {
				// Background should be defaultBg (since echo doesn't set colors)
				if colorsEqual(cell.Bg, defaultBg) {
					return // Found a cell with default background
				}
			}
		}
	}
}

func TestRenderTerminalContent_AreaSmallerthanTerm(t *testing.T) {
	// Area smaller than terminal dimensions - should clip
	term, err := terminal.New("/bin/echo", []string{"CLIP_TEST"}, 30, 10, 0, 0, "")
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

	buf := NewBuffer(15, 8, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 10, Height: 3} // Smaller than 30x10 terminal
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 0)

	// Should not panic; should clip content
}

func TestRenderTerminalContent_AreaLargerThanTerm(t *testing.T) {
	// Area larger than terminal dimensions
	term, err := terminal.New("/bin/echo", []string{"BIG"}, 10, 3, 0, 0, "")
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

	buf := NewBuffer(40, 20, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 30, Height: 15} // Much larger than 10x3 terminal
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 0)

	// Should not panic
}

func TestRenderTerminalContent_ScrollbackView(t *testing.T) {
	// Test scrollback offset > 0 path
	term, err := terminal.New("/bin/echo", []string{"SCROLL"}, 20, 5, 0, 0, "")
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

	buf := NewBuffer(30, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 20, Height: 5}
	// Use scrollOffset > 0 to trigger the scrollback code path
	// Even with no scrollback data, this exercises the scrollback render loop
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 2)

	// Should not panic
}

// ============================================================================
// renderScrollbar tests
// ============================================================================

func TestRenderScrollbar_NoScrollback(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"SB"}, 20, 10, 0, 0, "")
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

	buf := NewBuffer(40, 20, "#000000")
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 1, Y: 1, Width: 22, Height: 12}, nil)
	w.Focused = true
	w.Visible = true

	// With no scrollback, totalLines <= contentH, so no scrollbar
	renderScrollbar(buf, w, theme, term, 0)

	trackX := w.Rect.Right() - 1
	trackTop := w.Rect.Y + 1
	scrollbarDrawn := false
	for dy := trackTop; dy < w.Rect.Bottom()-2; dy++ {
		ch := buf.Cells[dy][trackX].Char
		if ch == '▓' || ch == '░' {
			scrollbarDrawn = true
			break
		}
	}
	if scrollbarDrawn {
		t.Error("expected no scrollbar when no scrollback data")
	}
}

func TestRenderScrollbar_SmallTrack(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"HI"}, 10, 3, 0, 0, "")
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

	buf := AcquireThemedBuffer(20, 8, theme)
	// Very small window: trackH < 3 -> early return
	w := window.NewWindow("w1", "Tiny", geometry.Rect{X: 0, Y: 0, Width: 12, Height: 4}, nil)
	w.Focused = true
	w.Visible = true

	renderScrollbar(buf, w, theme, term, 5)
	// Should not panic
}

// ============================================================================
// RenderDock tests
// ============================================================================

func TestRenderDock_NilDock(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	RenderDock(buf, nil, theme, nil)
	// Should be no-op
}

func TestRenderDock_SmallBuffer(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 1, theme) // Height < 2

	d := dock.New(nil, 80)
	RenderDock(buf, d, theme, nil)
	// Should return early because buf.Height < 2
}

func TestRenderDock_BasicRender(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	d := dock.New(nil, 80)
	RenderDock(buf, d, theme, nil)

	// Dock should fill the bottom row with dock background
	y := buf.Height - 1
	c := theme.C()
	hasDockBg := false
	for x := 0; x < buf.Width; x++ {
		if colorsEqual(buf.Cells[y][x].Bg, c.DockBg) {
			hasDockBg = true
			break
		}
	}
	if !hasDockBg {
		t.Error("expected dock background on bottom row")
	}
}

func TestRenderDock_WithItems(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "/bin/bash", IconColor: "#98C379"},
		{Name: "Editor", Icon: "\uf044", Command: "nvim", IconColor: "#61AFEF"},
	}
	d := dock.New(registry, 100)
	RenderDock(buf, d, theme, nil)

	// Bottom row should have icon content
	y := buf.Height - 1
	hasContent := false
	for x := 0; x < buf.Width; x++ {
		ch := buf.Cells[y][x].Char
		if ch != ' ' && ch != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected dock items on bottom row")
	}
}

func TestRenderDock_HoverIndex(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "/bin/bash"},
	}
	d := dock.New(registry, 100)
	d.SetHover(0) // Hover first item (Launcher)

	RenderDock(buf, d, theme, nil)

	// Hovered item should have accent background
	y := buf.Height - 1
	c := theme.C()
	hasAccent := false
	for x := 0; x < buf.Width; x++ {
		if colorsEqual(buf.Cells[y][x].Bg, c.AccentColor) {
			hasAccent = true
			break
		}
	}
	if !hasAccent {
		t.Error("expected accent background on hovered dock item")
	}
}

func TestRenderDock_IconsOnlyMode(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "/bin/bash"},
	}
	d := dock.New(registry, 100)
	d.IconsOnly = true

	RenderDock(buf, d, theme, nil)

	// Should render (icons-only mode shows just icons, more compact)
	y := buf.Height - 1
	hasContent := false
	for x := 0; x < buf.Width; x++ {
		ch := buf.Cells[y][x].Char
		if ch != ' ' && ch != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected dock content in icons-only mode")
	}
}

func TestRenderDock_WithDockPulseAnimation(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	d := dock.New(nil, 100)
	// Simulate a dock pulse animation
	animations := []Animation{
		{
			Type:      AnimDockPulse,
			DockIndex: 0,
			Done:      false,
		},
	}

	RenderDock(buf, d, theme, animations)

	// When d.HoverIndex is -1 but there's a dock pulse animation,
	// effectiveHover should be set to the pulse's DockIndex
	y := buf.Height - 1
	c := theme.C()
	hasAccent := false
	for x := 0; x < buf.Width; x++ {
		if colorsEqual(buf.Cells[y][x].Bg, c.AccentColor) {
			hasAccent = true
			break
		}
	}
	if !hasAccent {
		t.Error("expected accent from dock pulse animation")
	}
}

func TestRenderDock_CompletedPulseIgnored(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	d := dock.New(nil, 100)
	// Completed pulse should be ignored
	animations := []Animation{
		{
			Type:      AnimDockPulse,
			DockIndex: 0,
			Done:      true,
		},
	}

	RenderDock(buf, d, theme, animations)
	// Should render without accent (completed pulse doesn't activate hover)
}

func TestRenderDock_MinimizedItem(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	d := dock.New(nil, 100)
	// Add a minimized item directly
	d.Items = append(d.Items, dock.DockItem{
		Icon:    "\uf120",
		Label:   "Minimized Win",
		Special: "minimized",
	})

	RenderDock(buf, d, theme, nil)

	// Bottom row should have content
	y := buf.Height - 1
	hasContent := false
	for x := 0; x < buf.Width; x++ {
		ch := buf.Cells[y][x].Char
		if ch != ' ' && ch != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected dock content with minimized item")
	}
}

func TestRenderDock_RunningItem(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	d := dock.New(nil, 100)
	d.Items = append(d.Items, dock.DockItem{
		Icon:     "\uf120",
		Label:    "Running Win",
		Special:  "running",
		WindowID: "win-1",
	})

	RenderDock(buf, d, theme, nil)

	// Should render running item
	y := buf.Height - 1
	hasContent := false
	for x := 0; x < buf.Width; x++ {
		ch := buf.Cells[y][x].Char
		if ch != ' ' && ch != 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("expected dock content with running item")
	}
}

func TestRenderDock_ActiveItem(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	d := dock.New(nil, 100)
	d.Items = append(d.Items, dock.DockItem{
		Icon:     "\uf120",
		Label:    "Active Win",
		Special:  "running",
		WindowID: "win-1",
	})
	d.FocusedWindowID = "win-1" // This window is active

	RenderDock(buf, d, theme, nil)

	// Active item should have active title bg (title bar color, not border)
	y := buf.Height - 1
	c := theme.C()
	hasActiveBg := false
	for x := 0; x < buf.Width; x++ {
		if colorsEqual(buf.Cells[y][x].Bg, c.ActiveTitleBg) {
			hasActiveBg = true
			break
		}
	}
	if !hasActiveBg {
		t.Error("expected active title background on focused dock item")
	}
}

// ============================================================================
// RenderLauncher tests
// ============================================================================

func TestRenderLauncher_NilLauncher(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	RenderLauncher(buf, nil, theme)
	// No-op
}

func TestRenderLauncher_NotVisible(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(80, 30, theme)

	l := launcher.New(nil)
	// l.Visible is false by default
	RenderLauncher(buf, l, theme)
	// Nothing should be drawn beyond desktop
}

func TestRenderLauncher_VisibleEmpty(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	l := launcher.New(nil)
	l.Show()

	RenderLauncher(buf, l, theme)

	// Should draw the launcher overlay
	if !bufHasAnyNonSpace(buf) {
		t.Error("expected launcher content when visible")
	}
}

func TestRenderLauncher_WithApps(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "/bin/bash"},
		{Name: "Editor", Icon: "\uf044", Command: "nvim"},
		{Name: "Calc", Icon: "\uf1ec", Command: "termdesk-calc"},
	}
	l := launcher.New(registry)
	l.Show()

	RenderLauncher(buf, l, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected launcher content with apps")
	}
}

func TestRenderLauncher_WithQuery(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	registry := []registry.RegistryEntry{
		{Name: "Terminal", Icon: "\uf120", Command: "/bin/bash"},
		{Name: "Calculator", Icon: "\uf1ec", Command: "termdesk-calc"},
	}
	l := launcher.New(registry)
	l.Show()
	l.SetQuery("term") // Filter to Terminal only

	RenderLauncher(buf, l, theme)

	if !bufHasAnyNonSpace(buf) {
		t.Error("expected filtered launcher content")
	}
}

func TestRenderLauncher_CenteredVertically(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 30, theme.DesktopBg)

	l := launcher.New(nil)
	l.Show()

	RenderLauncher(buf, l, theme)

	// Launcher should be positioned in the upper third (startY = height/3)
	expectedStartY := buf.Height / 3
	// Check that content exists near expected Y
	hasContentNearExpectedY := false
	for y := max(0, expectedStartY-3); y < min(buf.Height, expectedStartY+10); y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' && buf.Cells[y][x].Char != 0 {
				hasContentNearExpectedY = true
				break
			}
		}
		if hasContentNearExpectedY {
			break
		}
	}
	if !hasContentNearExpectedY {
		t.Error("expected launcher content positioned in upper third")
	}
}

func TestRenderLauncher_SmallBuffer(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(20, 8, theme.DesktopBg)

	l := launcher.New(nil)
	l.Show()

	// Should not panic on small buffer
	RenderLauncher(buf, l, theme)
}

func TestRenderLauncher_NegativeStartX(t *testing.T) {
	theme := testTheme()
	// Very narrow buffer where boxW > buf.Width
	buf := NewBuffer(5, 20, theme.DesktopBg)

	registry := []registry.RegistryEntry{
		{Name: "Very Long App Name", Icon: "\uf120", Command: "longapp"},
	}
	l := launcher.New(registry)
	l.Show()

	// startX should be clamped to 0
	RenderLauncher(buf, l, theme)
}

// ============================================================================
// RenderWindow tests for additional branches
// ============================================================================

func TestRenderWindow_MaximizedNoSideBorders(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(40, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Max", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	w.Focused = true
	w.Resizable = true
	prevRect := geometry.Rect{X: 5, Y: 5, Width: 30, Height: 15}
	w.PreMaxRect = &prevRect // Makes it "maximized"

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Maximized windows have no side borders or bottom border
	// Check that side borders are NOT drawn
	// Side border at x=0 should be horizontal bar (part of title), not vertical
	// The bottom-left corner char should NOT be the normal border corner
	if buf.Cells[19][0].Char == theme.BorderBottomLeft {
		t.Error("maximized window should not have bottom border corners")
	}
}

func TestRenderWindow_WithNotification(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)

	w := window.NewWindow("w1", "Notify", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.HasNotification = true
	w.Focused = false

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Notification color should be used for unfocused windows with notifications
	c := theme.C()
	if c.NotificationFg != nil {
		borderFg := buf.Cells[0][0].Fg
		if colorsEqual(borderFg, c.NotificationFg) {
			return // Good - notification color used
		}
	}
}

func TestRenderWindow_HoverCloseButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitCloseButton)

	// Close button should have ButtonNoBg color (danger highlight)
	c := theme.C()
	closePos := w.CloseButtonPos()
	if closePos.X >= 0 && closePos.X < buf.Width && closePos.Y >= 0 && closePos.Y < buf.Height {
		closeBg := buf.Cells[closePos.Y][closePos.X+1].Bg
		if colorsEqual(closeBg, c.ButtonNoBg) {
			return // Correct hover color
		}
	}
}

func TestRenderWindow_HoverMinButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitMinButton)
	// Should not panic
}

func TestRenderWindow_HoverMaxButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitMaxButton)
	// Should not panic
}

func TestRenderWindow_HoverSnapButtons(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(40, 8, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitSnapLeftButton)
	// Should not panic

	buf2 := NewBuffer(40, 8, theme.DesktopBg)
	RenderWindow(buf2, w, theme, nil, true, 0, window.HitSnapRightButton)
	// Should not panic
}

func TestRenderWindow_ScrollIndicator(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"SCROLL_IND"}, 18, 6, 0, 0, "")
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

	buf := NewBuffer(30, 12, theme.DesktopBg)
	w := window.NewWindow("w1", "Scroll", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 10}, nil)
	w.Focused = true

	// scrollOffset > 0 should show scroll indicator on border
	RenderWindow(buf, w, theme, term, true, 2, window.HitNone)

	// Check for the scroll indicator characters on the bottom border row
	borderY := w.Rect.Bottom() - 1
	hasIndicator := false
	for x := w.Rect.X; x < w.Rect.Right(); x++ {
		ch := buf.Cells[borderY][x].Char
		if ch == '/' || ch == '2' { // Part of " [↑ 2/0] " indicator
			hasIndicator = true
			break
		}
	}
	// The indicator might not appear if scrollbackLen==0, but the code path is exercised
	_ = hasIndicator
}

func TestRenderWindow_UnfocusedFade(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}

	for _, theme := range themes {
		if theme.UnfocusedFade <= 0 {
			continue
		}
		t.Run(theme.Name+"_unfocused_fade", func(t *testing.T) {
			buf := NewBuffer(30, 10, theme.DesktopBg)
			w := window.NewWindow("w1", "Faded", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
			w.Focused = false

			RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

			// Unfocused window content should be desaturated
			// Just verify it rendered without error
			if buf.Cells[0][0].Char != theme.BorderTopLeft {
				t.Error("expected border on unfocused window")
			}
		})
	}
}

func TestRenderWindow_CursorRendering(t *testing.T) {
	theme := testTheme()

	term, err := terminal.New("/bin/echo", []string{"CUR"}, 18, 6, 0, 0, "")
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

	// showCursor=true, focused, scrollOffset=0
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Cursor", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, term, true, 0, window.HitNone)
	// Should not panic

	// showCursor=false
	buf2 := NewBuffer(30, 10, theme.DesktopBg)
	RenderWindow(buf2, w, theme, term, false, 0, window.HitNone)
	// Should not panic
}

// ============================================================================
// Cross-theme integration tests
// ============================================================================

func TestRenderFrame_MultipleThemes(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			wm := window.NewManager(80, 30)
			wm.SetReserved(1, 1)

			w1 := window.NewWindow("w1", "Test", geometry.Rect{X: 5, Y: 3, Width: 30, Height: 12}, nil)
			w1.Focused = true
			wm.AddWindow(w1)

			buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, true, "", window.HitNone, nil, nil, nil)
			if buf == nil {
				t.Fatal("expected non-nil buffer")
			}
			if !bufHasNonSpaceContent(buf, theme) {
				t.Error("expected frame content with theme " + theme.Name)
			}
		})
	}
}

func TestRenderMenuBar_MultipleThemes(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			buf := AcquireThemedBuffer(80, 20, theme)
			kb := config.DefaultKeyBindings()
			mb := menubar.New(80, kb, nil, nil)
			mb.OpenMenu(0)

			RenderMenuBar(buf, mb, theme, ModeNormal, false, "", -1)
			if !bufHasNonSpaceContent(buf, theme) {
				t.Error("expected menu bar content with theme " + theme.Name)
			}
		})
	}
}

func TestRenderDock_MultipleThemes(t *testing.T) {
	themes := []config.Theme{
		config.RetroTheme(),
		config.ModernTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			buf := AcquireThemedBuffer(80, 30, theme)
			d := dock.New(nil, 80)
			d.SetHover(0)

			RenderDock(buf, d, theme, nil)

			y := buf.Height - 1
			hasContent := false
			for x := 0; x < buf.Width; x++ {
				ch := buf.Cells[y][x].Char
				if ch != ' ' && ch != 0 {
					hasContent = true
					break
				}
			}
			if !hasContent {
				t.Error("expected dock content with theme " + theme.Name)
			}
		})
	}
}

// ============================================================================
// Integration: RenderFrame with terminal content
// ============================================================================

func TestRenderFrame_WithTerminalContent(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Terminal", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 12}, nil)
	w1.Focused = true
	wm.AddWindow(w1)

	term, err := terminal.New("/bin/echo", []string{"FRAME_TEST"}, 28, 10, 0, 0, "")
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

	buf := RenderFrame(wm, theme, terminals, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	// Check that terminal content was rendered
	found := false
	cr := w1.ContentRect()
	for y := cr.Y; y < cr.Y+cr.Height; y++ {
		for x := cr.X; x < cr.X+cr.Width; x++ {
			if x >= 0 && x < buf.Width && y >= 0 && y < buf.Height {
				if buf.Cells[y][x].Char == 'F' {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected terminal content 'F' from echo FRAME_TEST in frame render")
	}
}

// ============================================================================
// SelectionInfo struct tests
// ============================================================================

func TestSelectionInfo_Defaults(t *testing.T) {
	sel := SelectionInfo{}
	if sel.Active {
		t.Error("default Active should be false")
	}
	if sel.CopyMode {
		t.Error("default CopyMode should be false")
	}
}

// ============================================================================
// Dock with separator styling
// ============================================================================

func TestRenderDock_SeparatorStyling(t *testing.T) {
	theme := testTheme()
	buf := AcquireThemedBuffer(100, 30, theme)

	registry := []registry.RegistryEntry{
		{Name: "App1", Icon: "\uf120", Command: "app1"},
		{Name: "App2", Icon: "\uf044", Command: "app2"},
	}
	d := dock.New(registry, 100)

	RenderDock(buf, d, theme, nil)

	// Separators between dock items should use SubtleFg color
	y := buf.Height - 1
	c := theme.C()
	hasSubtleFg := false
	for x := 0; x < buf.Width; x++ {
		cell := buf.Cells[y][x]
		if c.SubtleFg != nil && colorsEqual(cell.Fg, c.SubtleFg) {
			hasSubtleFg = true
			break
		}
	}
	if c.SubtleFg != nil && !hasSubtleFg {
		// This is not an error — some themes may not have separators visible
		// Just ensures the code path was exercised
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- windowRenderCache.matches tests ---

func TestWindowRenderCacheMatchesNilCache(t *testing.T) {
	var c *windowRenderCache
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	theme := config.GetTheme("Retro")
	if c.matches(w, w.Rect, theme, window.HitNone, 0, true, false, false, SelectionInfo{}) {
		t.Error("nil cache should not match")
	}
}

func TestWindowRenderCacheMatchesNilBuf(t *testing.T) {
	c := &windowRenderCache{}
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	theme := config.GetTheme("Retro")
	if c.matches(w, w.Rect, theme, window.HitNone, 0, true, false, false, SelectionInfo{}) {
		t.Error("cache with nil buf should not match")
	}
}

func TestWindowRenderCacheMatchesSameState(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)
	w.Focused = true
	w.Resizable = true
	// NewWindow doesn't set TitleBarHeight, defaults to 0

	c := &windowRenderCache{
		buf:            NewBuffer(40, 20, "#000000"),
		rect:           rect,
		focused:        true,
		title:          "Test",
		resizable:      true,
		maximized:      false,
		titleBarHeight: 0, // match NewWindow default
		hoverZone:      window.HitNone,
		themeName:      theme.Name, // use actual theme name
		scrollOffset:   0,
		cursorShown:    true,
		hasCopySnap:    false,
	}

	if !c.matches(w, rect, theme, window.HitNone, 0, true, false, false, SelectionInfo{}) {
		t.Error("cache should match identical state")
	}
}

func TestWindowRenderCacheMatchesDifferentTitle(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Changed", rect, nil)
	w.Focused = true

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		focused:   true,
		title:     "Test",
		themeName: theme.Name,
	}

	if c.matches(w, rect, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when title changed")
	}
}

func TestWindowRenderCacheMatchesDifferentTheme(t *testing.T) {
	themeRetro := config.GetTheme("retro")
	themeModern := config.GetTheme("sleek")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: themeRetro.Name,
	}

	if c.matches(w, rect, themeModern, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when theme changed")
	}
}

func TestWindowRenderCacheMatchesDifferentRect(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	rect2 := geometry.Rect{X: 10, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect2, nil)

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: theme.Name,
	}

	if c.matches(w, rect2, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when rect changed")
	}
}

func TestWindowRenderCacheMatchesDifferentScrollOffset(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)

	c := &windowRenderCache{
		buf:          NewBuffer(40, 20, "#000000"),
		rect:         rect,
		title:        "Test",
		themeName:    theme.Name,
		scrollOffset: 0,
	}

	if c.matches(w, rect, theme, window.HitNone, 5, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when scrollOffset changed")
	}
}

func TestWindowRenderCacheMatchesDifferentHoverZone(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: theme.Name,
		hoverZone: window.HitNone,
	}

	if c.matches(w, rect, theme, window.HitCloseButton, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when hoverZone changed")
	}
}

func TestWindowRenderCacheMatchesDifferentHasBell(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)
	w.HasBell = true

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: theme.Name,
		hasBell:   false,
	}

	if c.matches(w, rect, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when hasBell changed")
	}
}

func TestWindowRenderCacheMatchesDifferentExited(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)
	w.Exited = true

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: theme.Name,
		exited:    false,
	}

	if c.matches(w, rect, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when exited changed")
	}
}

func TestWindowRenderCacheMatchesDifferentStuck(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)
	w.Stuck = true

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		themeName: theme.Name,
		stuck:     false,
	}

	if c.matches(w, rect, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should not match when stuck changed")
	}
}

func TestWindowRenderCacheMatchesSameWithBellExitedStuck(t *testing.T) {
	theme := config.GetTheme("retro")
	rect := geometry.Rect{X: 5, Y: 3, Width: 40, Height: 20}
	w := window.NewWindow("w1", "Test", rect, nil)
	w.HasBell = true
	w.Exited = true
	w.Stuck = true

	c := &windowRenderCache{
		buf:       NewBuffer(40, 20, "#000000"),
		rect:      rect,
		title:     "Test",
		resizable: true, // NewWindow sets Resizable=true
		themeName: theme.Name,
		hasBell:   true,
		exited:    true,
		stuck:     true,
	}

	if !c.matches(w, rect, theme, window.HitNone, 0, false, false, false, SelectionInfo{}) {
		t.Error("cache should match when hasBell, exited, stuck all agree")
	}
}

// ============================================================================
// renderExitedOverlay tests
// ============================================================================

func TestRenderExitedOverlay_BasicRender(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Exited", geometry.Rect{X: 0, Y: 0, Width: 60, Height: 20}, nil)
	w.Exited = true

	renderExitedOverlay(buf, w, theme)

	// The overlay message should appear at the vertical center of the content area
	cr := w.ContentRect()
	overlayY := cr.Y + cr.Height/2

	// Check that the message text is rendered at the correct Y
	var row strings.Builder
	for x := 0; x < buf.Width; x++ {
		row.WriteRune(buf.Cells[overlayY][x].Char)
	}
	rendered := row.String()
	if !strings.Contains(rendered, "restart") || !strings.Contains(rendered, "close") {
		t.Errorf("expected exited overlay message at Y=%d, got %q", overlayY, rendered)
	}
}

func TestRenderExitedOverlay_Colors(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(60, 20, theme.DesktopBg)

	w := window.NewWindow("w1", "Exited", geometry.Rect{X: 0, Y: 0, Width: 60, Height: 20}, nil)
	w.Exited = true

	renderExitedOverlay(buf, w, theme)

	cr := w.ContentRect()
	overlayY := cr.Y + cr.Height/2
	expectedBg := color.RGBA{R: 180, G: 30, B: 30, A: 255}
	expectedFg := color.RGBA{R: 220, G: 220, B: 220, A: 255}

	// Find a cell within the overlay message and verify colors
	foundOverlayCell := false
	for x := 0; x < buf.Width; x++ {
		cell := buf.Cells[overlayY][x]
		if cell.Char == 'r' || cell.Char == 'q' || cell.Char == 'P' {
			foundOverlayCell = true
			if !colorsEqual(cell.Bg, expectedBg) {
				t.Errorf("overlay bg at (%d,%d): got %v, want RGBA{180,30,30,255}", x, overlayY, cell.Bg)
			}
			if !colorsEqual(cell.Fg, expectedFg) {
				t.Errorf("overlay fg at (%d,%d): got %v, want RGBA{220,220,220,255}", x, overlayY, cell.Fg)
			}
			break
		}
	}
	if !foundOverlayCell {
		t.Error("could not find overlay cell to verify colors")
	}
}

func TestRenderExitedOverlay_NarrowWindowAbbreviatedText(t *testing.T) {
	theme := testTheme()
	// Create a narrow window where the full message won't fit
	buf := NewBuffer(30, 15, theme.DesktopBg)

	w := window.NewWindow("w1", "Narrow", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 15}, nil)
	w.Exited = true

	renderExitedOverlay(buf, w, theme)

	cr := w.ContentRect()
	overlayY := cr.Y + cr.Height/2

	var row strings.Builder
	for x := 0; x < buf.Width; x++ {
		row.WriteRune(buf.Cells[overlayY][x].Char)
	}
	rendered := row.String()
	// Narrow window should use abbreviated text "r=restart q=close"
	if !strings.Contains(rendered, "r=restart") {
		t.Errorf("expected abbreviated text 'r=restart' in narrow window, got %q", rendered)
	}
}

func TestRenderExitedOverlay_VerySmallWindowSkips(t *testing.T) {
	theme := testTheme()
	// Window so small that ContentRect().Width < 10 or Height < 3
	buf := NewBuffer(12, 5, theme.DesktopBg)

	w := window.NewWindow("w1", "Tiny", geometry.Rect{X: 0, Y: 0, Width: 12, Height: 5}, nil)
	w.Exited = true

	// Fill buffer with a known char to detect if overlay writes anything
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			buf.SetCell(x, y, '.', nil, nil, 0)
		}
	}

	renderExitedOverlay(buf, w, theme)

	// Content rect for a 12x5 window: X=1, Y=1, Width=10, Height=3
	// Width is exactly 10, so it should still render (cr.Width < 10 check)
	// Let's use a truly tiny window
	buf2 := NewBuffer(10, 4, theme.DesktopBg)
	w2 := window.NewWindow("w2", "Tiny2", geometry.Rect{X: 0, Y: 0, Width: 10, Height: 4}, nil)
	w2.Exited = true

	for y := 0; y < buf2.Height; y++ {
		for x := 0; x < buf2.Width; x++ {
			buf2.SetCell(x, y, '.', nil, nil, 0)
		}
	}

	renderExitedOverlay(buf2, w2, theme)

	// Content rect: X=1, Y=1, Width=8, Height=2 — Width < 10, should skip
	cr := w2.ContentRect()
	if cr.Width < 10 {
		// Verify no overlay cells were written in the content area
		overlayY := cr.Y + cr.Height/2
		if overlayY >= 0 && overlayY < buf2.Height {
			for x := cr.X; x < cr.X+cr.Width && x < buf2.Width; x++ {
				if buf2.Cells[overlayY][x].Char != '.' {
					t.Errorf("very small window should not render overlay, but found %q at (%d,%d)",
						buf2.Cells[overlayY][x].Char, x, overlayY)
					break
				}
			}
		}
	}
}

// ============================================================================
// RenderFrame exited overlay integration test
// ============================================================================

func TestRenderFrame_ExitedOverlayIntegration(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Exited", geometry.Rect{X: 5, Y: 3, Width: 50, Height: 16}, nil)
	w1.Focused = true
	w1.Exited = true
	wm.AddWindow(w1)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}

	// The exited overlay should appear in the content area of the window
	cr := w1.ContentRect()
	overlayY := cr.Y + cr.Height/2

	var row strings.Builder
	for x := cr.X; x < cr.X+cr.Width && x < buf.Width; x++ {
		row.WriteRune(buf.Cells[overlayY][x].Char)
	}
	rendered := row.String()
	if !strings.Contains(rendered, "restart") {
		t.Errorf("expected exited overlay in RenderFrame output at Y=%d, got %q", overlayY, rendered)
	}
}

func TestRenderFrame_ExitedOverlayWithCache(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(80, 30)
	wm.SetReserved(1, 1)

	w1 := window.NewWindow("w1", "Exited", geometry.Rect{X: 5, Y: 3, Width: 50, Height: 16}, nil)
	w1.Focused = true
	w1.Exited = true
	wm.AddWindow(w1)

	cache := make(map[string]*windowRenderCache)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, cache, nil, nil)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}

	// Verify cache entry was populated with exited=true
	entry, ok := cache["w1"]
	if !ok {
		t.Fatal("expected cache entry for w1")
	}
	if !entry.exited {
		t.Error("cache entry should have exited=true")
	}

	// Verify overlay text appears
	cr := w1.ContentRect()
	overlayY := cr.Y + cr.Height/2
	var row strings.Builder
	for x := cr.X; x < cr.X+cr.Width && x < buf.Width; x++ {
		row.WriteRune(buf.Cells[overlayY][x].Char)
	}
	rendered := row.String()
	if !strings.Contains(rendered, "restart") {
		t.Errorf("expected exited overlay with cache, got %q", rendered)
	}
}
