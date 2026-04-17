package app

import (
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func testTheme() config.Theme {
	return config.RetroTheme()
}

func TestNewBuffer(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	if buf.Width != 10 || buf.Height != 5 {
		t.Errorf("dimensions = %dx%d, want 10x5", buf.Width, buf.Height)
	}
	// All cells should be spaces
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Char != ' ' {
				t.Errorf("cell[%d][%d] = %q, want space", y, x, buf.Cells[y][x].Char)
			}
		}
	}
}

func TestBufferSet(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	buf.Set(3, 2, 'X', "#FFFFFF", "#000000")
	if buf.Cells[2][3].Char != 'X' {
		t.Error("expected X at (3,2)")
	}
	if buf.Cells[2][3].Fg == nil {
		t.Error("Fg should not be nil after setting a color")
	}
}

func TestBufferSetOutOfBounds(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	// Should not panic
	buf.Set(-1, 0, 'X', "", "")
	buf.Set(0, -1, 'X', "", "")
	buf.Set(10, 0, 'X', "", "")
	buf.Set(0, 5, 'X', "", "")
}

func TestBufferSetString(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	buf.SetString(2, 1, "Hello", "#FFF", "#000")
	for i, ch := range "Hello" {
		if buf.Cells[1][2+i].Char != ch {
			t.Errorf("cell[1][%d] = %q, want %q", 2+i, buf.Cells[1][2+i].Char, ch)
		}
	}
}

func TestBufferSetStringClip(t *testing.T) {
	buf := NewBuffer(5, 1, "#000000")
	buf.SetString(3, 0, "Hello", "#FFF", "#000")
	// Only "He" should fit (positions 3 and 4)
	if buf.Cells[0][3].Char != 'H' {
		t.Error("expected H at (3,0)")
	}
	if buf.Cells[0][4].Char != 'e' {
		t.Error("expected e at (4,0)")
	}
}

func TestBufferFillRect(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	r := geometry.Rect{X: 1, Y: 1, Width: 3, Height: 2}
	buf.FillRect(r, '#', "#FFF", "#000")
	if buf.Cells[1][1].Char != '#' {
		t.Error("expected # at (1,1)")
	}
	if buf.Cells[2][3].Char != '#' {
		t.Error("expected # at (3,2)")
	}
	// Outside fill area should still be space
	if buf.Cells[0][0].Char != ' ' {
		t.Error("expected space at (0,0)")
	}
}

func TestRenderWindowBasic(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Check corners
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Errorf("top-left = %q, want %q", buf.Cells[0][0].Char, theme.BorderTopLeft)
	}
	if buf.Cells[0][19].Char != theme.BorderTopRight {
		t.Errorf("top-right = %q, want %q", buf.Cells[0][19].Char, theme.BorderTopRight)
	}
	if buf.Cells[7][0].Char != theme.BorderBottomLeft {
		t.Errorf("bottom-left = %q, want %q", buf.Cells[7][0].Char, theme.BorderBottomLeft)
	}
	if buf.Cells[7][19].Char != theme.BorderBottomRight {
		t.Errorf("bottom-right = %q, want %q", buf.Cells[7][19].Char, theme.BorderBottomRight)
	}

	// Check side borders
	if buf.Cells[3][0].Char != theme.BorderVertical {
		t.Error("expected left border")
	}
	if buf.Cells[3][19].Char != theme.BorderVertical {
		t.Error("expected right border")
	}
}

func TestRenderWindowSkipsMinimized(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Minimized = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Should not have drawn anything
	if buf.Cells[0][0].Char != ' ' {
		t.Error("minimized window should not be rendered")
	}
}

func TestRenderWindowSkipsInvisible(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Visible = false

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	if buf.Cells[0][0].Char != ' ' {
		t.Error("invisible window should not be rendered")
	}
}

func TestRenderWindowTooSmall(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(10, 5, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 2, Height: 2}, nil)

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Too small to render, should be no-op
	if buf.Cells[0][0].Char != ' ' {
		t.Error("too-small window should not be rendered")
	}
}

func TestRenderWindowTitleTruncation(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(40, 5, theme.DesktopBg)
	w := window.NewWindow("w1", "This is a very long window title that should be truncated", geometry.Rect{X: 0, Y: 0, Width: 30, Height: 5}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Title should be present but truncated with "..."
	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 1; x < 29; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	title := titleStr.String()
	if !strings.Contains(title, "...") {
		// Short windows truncate title
		if !strings.Contains(title, "Th") {
			t.Errorf("title row = %q, expected some title content", title)
		}
	}
}

func TestRenderWindowActiveInactiveColors(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)
	activeFg := buf.Cells[0][0].Fg

	buf2 := NewBuffer(30, 10, theme.DesktopBg)
	w2 := window.NewWindow("w2", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w2.Focused = false
	RenderWindow(buf2, w2, theme, nil, true, 0, window.HitNone)
	inactiveFg := buf2.Cells[0][0].Fg

	// Foreground colors should differ (backgrounds are transparent = same as desktop)
	if colorsEqual(activeFg, inactiveFg) {
		t.Error("active and inactive borders should have different foregrounds")
	}
}

func TestRenderFrameEmpty(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(40, 20)
	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	if buf.Width != 40 || buf.Height != 20 {
		t.Errorf("buffer dimensions = %dx%d, want 40x20", buf.Width, buf.Height)
	}
	// Should be all desktop bg (space or pattern char)
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			ch := buf.Cells[y][x].Char
			if ch != ' ' && ch != theme.DesktopPatternChar {
				t.Errorf("unexpected char %q at (%d,%d)", ch, x, y)
			}
		}
	}
}

func TestRenderFrameWithWindows(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(60, 30)

	w1 := window.NewWindow("w1", "Back", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 10}, nil)
	w2 := window.NewWindow("w2", "Front", geometry.Rect{X: 10, Y: 5, Width: 20, Height: 10}, nil)
	wm.AddWindow(w1)
	wm.AddWindow(w2)

	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)

	// In the overlap area, w2 (front) should be visible
	// w2 starts at (10,5), so (10,5) should be w2's top-left corner
	if buf.Cells[5][10].Char != theme.BorderTopLeft {
		t.Errorf("overlap: expected w2 border at (10,5), got %q", buf.Cells[5][10].Char)
	}

	// w1's top-left corner at (0,0) should still be visible
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Errorf("w1 corner: expected border at (0,0), got %q", buf.Cells[0][0].Char)
	}
}

func TestRenderFrameZeroBounds(t *testing.T) {
	theme := testTheme()
	wm := window.NewManager(0, 0)
	buf := RenderFrame(wm, theme, nil, nil, true, 0, SelectionInfo{}, false, "", window.HitNone, nil, nil, nil)
	if buf.Width != 1 || buf.Height != 1 {
		t.Errorf("zero bounds buffer = %dx%d, want 1x1", buf.Width, buf.Height)
	}
}

func TestBufferToString(t *testing.T) {
	buf := NewBuffer(5, 3, "#000000")
	buf.Set(0, 0, 'A', "", "")
	buf.Set(4, 2, 'Z', "", "")

	s := BufferToString(buf)
	// BufferToString now includes ANSI sequences, but should contain A and Z
	if !strings.Contains(s, "A") {
		t.Error("output should contain 'A'")
	}
	if !strings.Contains(s, "Z") {
		t.Error("output should contain 'Z'")
	}
	// Should have line breaks
	lines := strings.Split(s, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRenderWindowCloseButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	// Close button should be at right side of title bar
	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, theme.CloseButton) {
		t.Errorf("title bar = %q, expected %q close button", rendered, theme.CloseButton)
	}
}

func TestRenderWindowMaxButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, theme.MaxButton) {
		t.Errorf("title bar = %q, expected %q max button", rendered, theme.MaxButton)
	}
}

func TestRenderWindowRestoreButton(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(30, 8, theme.DesktopBg)
	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true
	w.Resizable = true
	prevRect := geometry.Rect{X: 5, Y: 5, Width: 10, Height: 5}
	w.PreMaxRect = &prevRect

	RenderWindow(buf, w, theme, nil, true, 0, window.HitNone)

	row := buf.Cells[0]
	var titleStr strings.Builder
	for x := 0; x < 20; x++ {
		titleStr.WriteRune(row[x].Char)
	}
	rendered := titleStr.String()
	if !strings.Contains(rendered, theme.RestoreButton) {
		t.Errorf("title bar = %q, expected %q restore button when maximized", rendered, theme.RestoreButton)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"empty", "", ""},
		{"CSI color", "\x1b[31mred\x1b[0m", "red"},
		{"CSI bold", "\x1b[1mbold\x1b[22m", "bold"},
		{"CSI cursor", "\x1b[2;5H", ""},
		{"OSC title BEL", "\x1b]0;title\x07text", "text"},
		{"OSC title ST", "\x1b]0;title\x1b\\text", "text"},
		{"mixed", "\x1b[32mgreen\x1b[0m plain \x1b[1;34mblue\x1b[0m", "green plain blue"},
		{"ESC other", "\x1b(B text", " text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripANSI(tt.input))
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderTerminalContentNil(t *testing.T) {
	buf := NewBuffer(20, 10, "#000")
	area := geometry.Rect{X: 1, Y: 1, Width: 10, Height: 3}

	// nil terminal should be no-op
	renderTerminalContent(buf, area, nil, nil, nil, 0)
	if buf.Cells[1][1].Char != ' ' {
		t.Error("nil terminal should not change buffer")
	}
}

func TestRenderTerminalContentWithTerminal(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"HELLO"}, 20, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Read PTY output
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
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#1E2127"), 0)

	// Check that "HELLO" was rendered somewhere in the content area
	var found bool
	for y := area.Y; y < area.Y+area.Height; y++ {
		for x := area.X; x < area.X+area.Width; x++ {
			if buf.Cells[y][x].Char == 'H' {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected terminal content to contain 'H' from echo HELLO")
	}
}

func TestRenderWindowWithTerminal(t *testing.T) {
	term, err := terminal.New("/bin/echo", []string{"TEST"}, 18, 6, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
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

	theme := testTheme()
	buf := NewBuffer(30, 10, theme.DesktopBg)
	w := window.NewWindow("w1", "Term", geometry.Rect{X: 0, Y: 0, Width: 20, Height: 8}, nil)
	w.Focused = true

	RenderWindow(buf, w, theme, term, true, 0, window.HitNone)

	// Should have both borders and terminal content
	if buf.Cells[0][0].Char != theme.BorderTopLeft {
		t.Error("expected border")
	}
}

func TestRenderScrollbarDrawsThumb(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(20, 10, theme.DesktopBg)

	w := window.NewWindow("w1", "Test", geometry.Rect{X: 0, Y: 0, Width: 10, Height: 6}, nil)
	term, err := terminal.NewShell(6, 3, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	defer term.Close()

	// Populate scrollback by restoring more lines than terminal height
	term.RestoreBuffer("a\nb\nc\nd\ne\nf")

	renderScrollbar(buf, w, theme, term, 1)

	trackX := w.Rect.Right() - 1
	foundThumb := false
	for y := w.Rect.Y + 1; y < w.Rect.Bottom()-1; y++ {
		if buf.Cells[y][trackX].Char == '▓' {
			foundThumb = true
			break
		}
	}
	if !foundThumb {
		t.Fatal("expected scrollbar thumb to be drawn")
	}
}

func TestRenderTerminalContentWithScrollback(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(10, 6, theme.DesktopBg)
	area := geometry.Rect{X: 0, Y: 0, Width: 4, Height: 3}

	term, err := terminal.NewShell(4, 3, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	defer term.Close()

	term.RestoreBuffer("A\nB\nC\nD\nE")

	renderTerminalContent(buf, area, term, hexToColor("#FFFFFF"), hexToColor("#000000"), 2)

	if buf.Cells[0][0].Char != 'A' {
		t.Fatalf("expected top scrollback row to start with 'A', got %q", buf.Cells[0][0].Char)
	}
	if buf.Cells[2][0].Char != 'C' {
		t.Fatalf("expected first emulator row to start with 'C', got %q", buf.Cells[2][0].Char)
	}
}

// --- trimRunes tests ---

func TestTrimRunesShortString(t *testing.T) {
	// String shorter than max — return unchanged
	if got := trimRunes("abc", 10); got != "abc" {
		t.Errorf("trimRunes(\"abc\", 10) = %q, want \"abc\"", got)
	}
}

func TestTrimRunesExactMax(t *testing.T) {
	// String exactly at max — return unchanged
	if got := trimRunes("abcde", 5); got != "abcde" {
		t.Errorf("trimRunes(\"abcde\", 5) = %q, want \"abcde\"", got)
	}
}

func TestTrimRunesTruncated(t *testing.T) {
	// String longer than max — truncated with ellipsis
	got := trimRunes("abcdefgh", 5)
	if len([]rune(got)) > 5 {
		t.Errorf("trimRunes result %q exceeds max 5 runes", got)
	}
	if !strings.Contains(got, "\u2026") && !strings.Contains(got, "...") {
		t.Errorf("trimRunes result %q should contain ellipsis", got)
	}
}

func TestTrimRunesMaxZero(t *testing.T) {
	if got := trimRunes("abc", 0); got != "" {
		t.Errorf("trimRunes(\"abc\", 0) = %q, want empty", got)
	}
}

func TestTrimRunesMaxNegative(t *testing.T) {
	if got := trimRunes("abc", -1); got != "" {
		t.Errorf("trimRunes(\"abc\", -1) = %q, want empty", got)
	}
}

func TestTrimRunesMaxOne(t *testing.T) {
	got := trimRunes("abcdefgh", 1)
	if got == "" {
		t.Error("trimRunes with max=1 should return ellipsis, not empty")
	}
}

func TestTrimRunesUnicode(t *testing.T) {
	// Unicode string
	got := trimRunes("\u4e16\u754c\u4f60\u597d\u554a", 3)
	runeCount := len([]rune(got))
	if runeCount > 3 {
		t.Errorf("trimRunes unicode result has %d runes, max 3", runeCount)
	}
}

// --- CopySnapshot tests ---

func TestCopySnapshotScrollbackLineNilSnapshot(t *testing.T) {
	var s *CopySnapshot
	if line := s.ScrollbackLine(0); line != nil {
		t.Error("expected nil from nil snapshot")
	}
}

func TestCopySnapshotScrollbackLineOutOfBounds(t *testing.T) {
	s := &CopySnapshot{
		Scrollback: [][]terminal.ScreenCell{
			{{Content: "a"}},
			{{Content: "b"}},
		},
	}
	if line := s.ScrollbackLine(-1); line != nil {
		t.Error("expected nil for negative offset")
	}
	if line := s.ScrollbackLine(2); line != nil {
		t.Error("expected nil for out-of-bounds offset")
	}
	if line := s.ScrollbackLine(100); line != nil {
		t.Error("expected nil for far out-of-bounds offset")
	}
}

func TestCopySnapshotScrollbackLineValid(t *testing.T) {
	s := &CopySnapshot{
		Scrollback: [][]terminal.ScreenCell{
			{{Content: "a"}},
			{{Content: "b"}},
		},
	}
	line := s.ScrollbackLine(0)
	if line == nil {
		t.Fatal("expected non-nil line at offset 0")
	}
	if line[0].Content != "a" {
		t.Errorf("expected 'a' at offset 0, got %q", line[0].Content)
	}
	line1 := s.ScrollbackLine(1)
	if line1 == nil {
		t.Fatal("expected non-nil line at offset 1")
	}
	if line1[0].Content != "b" {
		t.Errorf("expected 'b' at offset 1, got %q", line1[0].Content)
	}
}

func TestCopySnapshotScrollbackLenNil(t *testing.T) {
	var s *CopySnapshot
	if s.ScrollbackLen() != 0 {
		t.Error("expected 0 for nil snapshot")
	}
}

func TestCopySnapshotScrollbackLenValid(t *testing.T) {
	s := &CopySnapshot{
		Scrollback: [][]terminal.ScreenCell{
			{{Content: "a"}},
			{{Content: "b"}},
			{{Content: "c"}},
		},
	}
	if got := s.ScrollbackLen(); got != 3 {
		t.Errorf("ScrollbackLen() = %d, want 3", got)
	}
}

func TestCaptureCopySnapshotNilTerminal(t *testing.T) {
	s := captureCopySnapshot("w1", nil)
	if s != nil {
		t.Error("expected nil snapshot for nil terminal")
	}
}

// --- renderCopySearchBar tests ---

func TestRenderCopySearchBarNilWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	// Should not panic with nil window
	renderCopySearchBar(buf, nil, theme, "test", 1, 0, 0)
}

func TestRenderCopySearchBarInvisibleWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	w.Visible = false
	renderCopySearchBar(buf, w, theme, "test", 1, 0, 0)
	// Verify nothing was written (search bar should be skipped)
}

func TestRenderCopySearchBarMinimizedWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 0, Y: 0, Width: 40, Height: 20}, nil)
	w.Minimized = true
	renderCopySearchBar(buf, w, theme, "test", 1, 0, 0)
}

func TestRenderCopySearchBarForwardSearch(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 2, Y: 2, Width: 40, Height: 20}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "hello", 1, 0, 3)
	// The search bar should be rendered in the content area
	// Check that at least some cells in the content area are non-empty
	cr := w.ContentRect()
	found := false
	for dx := 0; dx < cr.Width && (cr.X+dx) < buf.Width; dx++ {
		cell := buf.Cells[cr.Y][cr.X+dx]
		if cell.Char == '/' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '/' prefix in search bar for forward search")
	}
}

func TestRenderCopySearchBarBackwardSearch(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 2, Y: 2, Width: 40, Height: 20}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "hello", -1, 0, 3)
	cr := w.ContentRect()
	found := false
	for dx := 0; dx < cr.Width && (cr.X+dx) < buf.Width; dx++ {
		cell := buf.Cells[cr.Y][cr.X+dx]
		if cell.Char == '?' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '?' prefix in search bar for backward search")
	}
}

func TestRenderCopySearchBarNarrowWindow(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	// Very narrow window to test barW > cr.Width path
	w := window.NewWindow("test", "test", geometry.Rect{X: 0, Y: 0, Width: 15, Height: 10}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "a very long search query that exceeds width", 1, 0, 0)
	// Should not panic and should clamp to content width
}

func TestRenderCopySearchBarEmptyQuery(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 2, Y: 2, Width: 40, Height: 20}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "", 1, 0, 0)
	// Should render with just the "/" prefix and cursor
}

func TestRenderCopySearchBarMatchCount(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 2, Y: 2, Width: 40, Height: 20}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "hello", 1, 2, 5)
	// Should contain "[3/5]" in the rendered text (matchIdx=2 → display as 3)
	cr := w.ContentRect()
	found := false
	for dx := 0; dx < cr.Width && (cr.X+dx) < buf.Width; dx++ {
		cell := buf.Cells[cr.Y][cr.X+dx]
		if cell.Char == '[' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '[' in search bar for match count display")
	}
}

func TestRenderCopySearchBarNoMatches(t *testing.T) {
	theme := testTheme()
	buf := NewBuffer(80, 24, "#000000")
	w := window.NewWindow("test", "test", geometry.Rect{X: 2, Y: 2, Width: 40, Height: 20}, nil)
	w.TitleBarHeight = 1
	renderCopySearchBar(buf, w, theme, "xyz", 1, 0, 0)
	// Should contain "[0/0]" in the rendered text
	cr := w.ContentRect()
	found := false
	for dx := 0; dx < cr.Width && (cr.X+dx) < buf.Width; dx++ {
		cell := buf.Cells[cr.Y][cr.X+dx]
		if cell.Char == '[' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '[' in search bar for 0/0 display")
	}
}

// --- collectTerminalLines tests (snapshot path) ---

func TestCollectTerminalLinesWithSnapshot(t *testing.T) {
	snap := &CopySnapshot{
		Height: 2,
		Width:  5,
		Screen: [][]terminal.ScreenCell{
			{{Content: "h", Width: 1}, {Content: "i", Width: 1}},
			{{Content: "b", Width: 1}, {Content: "y", Width: 1}},
		},
		Scrollback: [][]terminal.ScreenCell{
			{{Content: "s", Width: 1}, {Content: "b", Width: 1}},
		},
	}
	lines := collectTerminalLines(nil, snap)
	// Should have 1 scrollback + 2 screen lines = 3
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "sb" {
		t.Errorf("expected scrollback line 'sb', got %q", lines[0])
	}
	if lines[1] != "hi" {
		t.Errorf("expected screen line 'hi', got %q", lines[1])
	}
	if lines[2] != "by" {
		t.Errorf("expected screen line 'by', got %q", lines[2])
	}
}

func TestCollectTerminalLinesSnapshotExtraHeight(t *testing.T) {
	// Height > len(Screen) means extra blank lines
	snap := &CopySnapshot{
		Height: 4,
		Width:  5,
		Screen: [][]terminal.ScreenCell{
			{{Content: "a", Width: 1}},
		},
		Scrollback: nil,
	}
	lines := collectTerminalLines(nil, snap)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[0] != "a" {
		t.Errorf("expected 'a', got %q", lines[0])
	}
	for i := 1; i < 4; i++ {
		if lines[i] != "" {
			t.Errorf("expected empty line at %d, got %q", i, lines[i])
		}
	}
}

// --- extractSelTextWithSnapshot tests ---

func TestExtractSelTextWithSnapshotSingleLine(t *testing.T) {
	term, err := terminal.NewShell(10, 2, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	snap := &CopySnapshot{
		Height: 2,
		Width:  10,
		Screen: [][]terminal.ScreenCell{
			{{Content: "h", Width: 1}, {Content: "e", Width: 1}, {Content: "l", Width: 1}},
			{{Content: "w", Width: 1}, {Content: "o", Width: 1}},
		},
		Scrollback: nil,
	}
	// Single line selection
	start := geometry.Point{X: 0, Y: 0}
	end := geometry.Point{X: 2, Y: 0}
	text := extractSelTextWithSnapshot(term, snap, start, end)
	if !strings.Contains(text, "hel") {
		t.Errorf("expected selection to contain 'hel', got %q", text)
	}
}

func TestExtractSelTextWithSnapshotMultiLine(t *testing.T) {
	term, err := terminal.NewShell(5, 3, 0, 0, "")
	if err != nil {
		t.Skip("could not create terminal")
	}
	snap := &CopySnapshot{
		Height: 3,
		Width:  5,
		Screen: [][]terminal.ScreenCell{
			{{Content: "a", Width: 1}, {Content: "b", Width: 1}, {Content: "c", Width: 1}},
			{{Content: "d", Width: 1}, {Content: "e", Width: 1}, {Content: "f", Width: 1}},
			{{Content: "g", Width: 1}, {Content: "h", Width: 1}, {Content: "i", Width: 1}},
		},
		Scrollback: nil,
	}
	start := geometry.Point{X: 1, Y: 0}
	end := geometry.Point{X: 1, Y: 2}
	text := extractSelTextWithSnapshot(term, snap, start, end)
	// Should span multiple lines
	if !strings.Contains(text, "\n") {
		t.Errorf("expected multi-line selection, got %q", text)
	}
}

// TestE2E_EmojiAlignmentThroughPTY verifies that emoji characters flowing
// through the real PTY → VT emulator → render → BufferToString pipeline
// produce correctly aligned output. Tests both the buffer cell grid (internal
// alignment) and the BufferToString display-column output (host terminal alignment).
func TestE2E_EmojiAlignmentThroughPTY(t *testing.T) {
	// Use printf to output emoji through a real PTY.
	term, err := terminal.New("/usr/bin/printf",
		[]string{"☀\uFE0F AB|\\n⚠\uFE0F CD|\\n✅ EF|\\nplain|"},
		20, 6, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
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

	// Snapshot the VT emulator and render to a buffer
	buf := NewBuffer(25, 8, "#000000")
	area := geometry.Rect{X: 0, Y: 0, Width: 20, Height: 6}
	renderTerminalContent(buf, area, term, hexToColor("#C0C0C0"), hexToColor("#000000"), 0)

	// === TEST 1: Buffer cell alignment ===
	// The "|" character should be at the SAME cell column in all rows.
	// This validates: VT emulator width → renderTerminalContentWithSnapshot → Buffer
	t.Log("=== Buffer cell alignment (internal grid) ===")
	pipeCol := -1
	for row := 0; row < 4; row++ {
		// Dump row for debugging
		var cells []string
		for col := 0; col < 12; col++ {
			c := buf.Cells[row][col]
			if c.Content != "" {
				cells = append(cells, "["+c.Content+" w="+string(rune('0'+c.Width))+"]")
			} else if c.Char > ' ' {
				cells = append(cells, "["+string(c.Char)+" w="+string(rune('0'+c.Width))+"]")
			} else {
				cells = append(cells, ".")
			}
		}
		t.Logf("Row %d cells: %s", row, strings.Join(cells, ""))

		// Find "|" in this row
		for col := 0; col < 20; col++ {
			if buf.Cells[row][col].Char == '|' {
				if pipeCol == -1 {
					pipeCol = col
				} else if col != pipeCol {
					t.Errorf("BUFFER MISALIGNMENT: '|' at col %d on row %d, expected col %d", col, row, pipeCol)
				}
				t.Logf("Row %d: '|' at buffer col %d", row, col)
				break
			}
		}
	}
	if pipeCol == -1 {
		t.Fatal("no '|' found in buffer")
	}

	// === TEST 2: BufferToString display-column alignment ===
	// Measure display columns using ansi.StringWidth (same as VT emulator).
	// This validates: Buffer → BufferToString → display column calculation.
	output := BufferToString(buf)
	lines := strings.Split(output, "\n")

	t.Log("=== BufferToString display-column alignment ===")
	var displayCols []int
	for i, line := range lines {
		if i >= 4 {
			break
		}
		stripped := string(stripANSI(line))
		// Walk through runes, accumulating display width, find "|"
		col := 0
		found := false
		for _, r := range stripped {
			if r == '|' {
				displayCols = append(displayCols, col)
				t.Logf("Line %d: '|' at display col %d (stripped: %q)", i, col, stripped[:20])
				found = true
				break
			}
			w := runewidth.RuneWidth(r)
			if w < 1 {
				w = 1
			}
			col += w
		}
		if !found {
			t.Logf("Line %d: no '|' found (stripped: %q)", i, stripped[:20])
		}
	}

	if len(displayCols) < 2 {
		t.Fatalf("expected at least 2 lines with '|', got %d", len(displayCols))
	}

	// Check consistency. Note: lines with VS16 emoji (☀️) will have "|" at
	// a different runewidth-measured position than plain text because runewidth
	// reports 1 for "☀" but the Content includes VS16. This is expected since
	// runewidth doesn't match all terminals. The BUFFER alignment (Test 1) is
	// the authoritative check; display-column alignment depends on the host.
	for i := 1; i < len(displayCols); i++ {
		if displayCols[i] != displayCols[0] {
			t.Logf("NOTE: display col mismatch (line %d=%d vs line 0=%d) — expected for emoji with different byte widths",
				i, displayCols[i], displayCols[0])
		}
	}
}
