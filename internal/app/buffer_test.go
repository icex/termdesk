package app

import (
	"image/color"
	"strings"
	"testing"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/pkg/geometry"
)

// ── firstRune tests ──

func TestFirstRuneNormal(t *testing.T) {
	if r := firstRune("Hello"); r != 'H' {
		t.Errorf("firstRune(Hello) = %q, want 'H'", r)
	}
}

func TestFirstRuneEmpty(t *testing.T) {
	if r := firstRune(""); r != ' ' {
		t.Errorf("firstRune(\"\") = %q, want ' '", r)
	}
}

func TestFirstRuneUnicode(t *testing.T) {
	if r := firstRune("日本語"); r != '日' {
		t.Errorf("firstRune(日本語) = %q, want '日'", r)
	}
}

func TestFirstRuneSingleChar(t *testing.T) {
	if r := firstRune("X"); r != 'X' {
		t.Errorf("firstRune(X) = %q, want 'X'", r)
	}
}

// ── Cell.Content grapheme cluster tests ──

func TestBufferToStringPreservesGraphemeCluster(t *testing.T) {
	buf := NewBuffer(3, 1, "")
	// Simulate a multi-codepoint emoji: ☀️ = U+2600 + U+FE0F
	emoji := "☀\uFE0F"
	buf.Cells[0][0] = Cell{Char: '☀', Content: emoji, Fg: nil, Bg: nil, Width: 2}
	buf.Cells[0][1] = Cell{Char: ' ', Width: 0} // continuation cell
	buf.Cells[0][2] = Cell{Char: 'X', Width: 1}

	result := BufferToString(buf)
	if !strings.Contains(result, emoji) {
		t.Errorf("BufferToString should preserve grapheme cluster %q, got %q", emoji, result)
	}
	if !strings.Contains(result, "X") {
		t.Error("BufferToString should include 'X' after the wide emoji")
	}
}

func TestBufferToStringFallsBackToChar(t *testing.T) {
	buf := NewBuffer(2, 1, "")
	buf.Cells[0][0] = Cell{Char: 'A', Width: 1}
	buf.Cells[0][1] = Cell{Char: 'B', Width: 1}

	result := BufferToString(buf)
	// Strip ANSI escapes for comparison
	runes := stripANSI(result)
	if len(runes) < 2 || runes[0] != 'A' || runes[1] != 'B' {
		t.Errorf("expected 'AB', got runes %q", string(runes))
	}
}

// ── hexNibble tests ──

func TestHexNibbleDigits(t *testing.T) {
	for i := byte('0'); i <= '9'; i++ {
		got := hexNibble(i)
		want := i - '0'
		if got != want {
			t.Errorf("hexNibble(%c) = %d, want %d", i, got, want)
		}
	}
}

func TestHexNibbleLowerHex(t *testing.T) {
	tests := []struct {
		in   byte
		want uint8
	}{
		{'a', 10}, {'b', 11}, {'c', 12}, {'d', 13}, {'e', 14}, {'f', 15},
	}
	for _, tt := range tests {
		if got := hexNibble(tt.in); got != tt.want {
			t.Errorf("hexNibble(%c) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestHexNibbleUpperHex(t *testing.T) {
	tests := []struct {
		in   byte
		want uint8
	}{
		{'A', 10}, {'B', 11}, {'C', 12}, {'D', 13}, {'E', 14}, {'F', 15},
	}
	for _, tt := range tests {
		if got := hexNibble(tt.in); got != tt.want {
			t.Errorf("hexNibble(%c) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestHexNibbleInvalid(t *testing.T) {
	invalids := []byte{'g', 'G', 'z', 'Z', '!', ' ', '-', '.'}
	for _, b := range invalids {
		if got := hexNibble(b); got != 0 {
			t.Errorf("hexNibble(%c) = %d, want 0 for invalid", b, got)
		}
	}
}

// ── hexByte tests ──

func TestHexByte(t *testing.T) {
	if got := hexByte('F', 'F'); got != 255 {
		t.Errorf("hexByte(F,F) = %d, want 255", got)
	}
	if got := hexByte('0', '0'); got != 0 {
		t.Errorf("hexByte(0,0) = %d, want 0", got)
	}
	if got := hexByte('8', '0'); got != 128 {
		t.Errorf("hexByte(8,0) = %d, want 128", got)
	}
	if got := hexByte('a', 'b'); got != 0xab {
		t.Errorf("hexByte(a,b) = %d, want 171", got)
	}
}

// ── hexToColor tests ──

func TestHexToColorValid(t *testing.T) {
	c := hexToColor("#FF0000")
	if c == nil {
		t.Fatal("hexToColor(#FF0000) returned nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("hexToColor(#FF0000) = (%d,%d,%d), want (255,0,0)", r>>8, g>>8, b>>8)
	}
}

func TestHexToColorBlack(t *testing.T) {
	c := hexToColor("#000000")
	if c == nil {
		t.Fatal("hexToColor(#000000) returned nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 0 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("hexToColor(#000000) = (%d,%d,%d), want (0,0,0)", r>>8, g>>8, b>>8)
	}
}

func TestHexToColorLowerCase(t *testing.T) {
	c := hexToColor("#abcdef")
	if c == nil {
		t.Fatal("hexToColor(#abcdef) returned nil")
	}
	r, g, b, _ := c.RGBA()
	if r>>8 != 0xab || g>>8 != 0xcd || b>>8 != 0xef {
		t.Errorf("hexToColor(#abcdef) = (%d,%d,%d), want (171,205,239)", r>>8, g>>8, b>>8)
	}
}

func TestHexToColorInvalidLength(t *testing.T) {
	if c := hexToColor("#FFF"); c != nil {
		t.Error("hexToColor(#FFF) should return nil for short string")
	}
	if c := hexToColor(""); c != nil {
		t.Error("hexToColor(\"\") should return nil for empty string")
	}
	if c := hexToColor("#FFFFFFF"); c != nil {
		t.Error("hexToColor(#FFFFFFF) should return nil for too-long string")
	}
}

func TestHexToColorMissingHash(t *testing.T) {
	if c := hexToColor("FF0000F"); c != nil {
		t.Error("hexToColor without # should return nil")
	}
}

// ── runeLen tests ──

func TestRuneLen(t *testing.T) {
	if got := runeLen("hello"); got != 5 {
		t.Errorf("runeLen(hello) = %d, want 5", got)
	}
	if got := runeLen("日本語"); got != 3 {
		t.Errorf("runeLen(日本語) = %d, want 3", got)
	}
	if got := runeLen(""); got != 0 {
		t.Errorf("runeLen(\"\") = %d, want 0", got)
	}
	// Nerd font icon (multi-byte UTF-8 but one rune)
	if got := runeLen("\uf120"); got != 1 {
		t.Errorf("runeLen(\\uf120) = %d, want 1", got)
	}
}

// ── showPattern tests ──

func TestShowPatternZero(t *testing.T) {
	if showPattern(0, 0, 0) {
		t.Error("showPattern(0,...) should always return false")
	}
	if showPattern(0, 5, 5) {
		t.Error("showPattern(0,...) should always return false")
	}
}

func TestShowPatternBlockChars(t *testing.T) {
	// Block patterns always return true
	for _, pat := range []rune{'░', '▒', '▓'} {
		if !showPattern(pat, 0, 0) {
			t.Errorf("showPattern(%c, 0, 0) should return true", pat)
		}
		if !showPattern(pat, 5, 7) {
			t.Errorf("showPattern(%c, 5, 7) should return true", pat)
		}
	}
}

func TestShowPatternCheckerboard(t *testing.T) {
	// '▚' shows at (x+y)%2==0
	if !showPattern('▚', 0, 0) {
		t.Error("showPattern(▚, 0, 0) should be true")
	}
	if showPattern('▚', 1, 0) {
		t.Error("showPattern(▚, 1, 0) should be false")
	}
	if showPattern('▚', 0, 1) {
		t.Error("showPattern(▚, 0, 1) should be false")
	}
	if !showPattern('▚', 1, 1) {
		t.Error("showPattern(▚, 1, 1) should be true")
	}
}

func TestShowPatternBraille(t *testing.T) {
	// '⠿': staggered grid
	if !showPattern('⠿', 0, 0) {
		t.Error("showPattern(⠿, 0, 0) should be true (x%3==0 && y%2==0)")
	}
	if !showPattern('⠿', 1, 1) {
		t.Error("showPattern(⠿, 1, 1) should be true (x%3==1 && y%2==1)")
	}
	if showPattern('⠿', 2, 0) {
		t.Error("showPattern(⠿, 2, 0) should be false")
	}
}

func TestShowPatternCrossHatch(t *testing.T) {
	// '╳': (x+y)%3==0 || (x-y+100)%3==0
	if !showPattern('╳', 0, 0) {
		t.Error("showPattern(╳, 0, 0) should be true")
	}
}

func TestShowPatternDiamond(t *testing.T) {
	// '◇': (x%5==0 && y%3==0) || (x%5==2 && y%3==1)
	if !showPattern('◇', 0, 0) {
		t.Error("showPattern(◇, 0, 0) should be true")
	}
	if !showPattern('◇', 2, 1) {
		t.Error("showPattern(◇, 2, 1) should be true")
	}
}

func TestShowPatternFilledDiamond(t *testing.T) {
	// '◆': (x*7+y*13)%11==0
	if !showPattern('◆', 0, 0) {
		t.Error("showPattern(◆, 0, 0) should be true")
	}
}

func TestShowPatternWave(t *testing.T) {
	// '≈': (x+(y/2))%5==0
	if !showPattern('≈', 0, 0) {
		t.Error("showPattern(≈, 0, 0) should be true")
	}
	if showPattern('≈', 1, 0) {
		t.Error("showPattern(≈, 1, 0) should be false")
	}
}

func TestShowPatternDefault(t *testing.T) {
	// Unknown pattern chars use default: (x+y)%4==0
	if !showPattern('·', 0, 0) {
		t.Error("showPattern(·, 0, 0) should be true (default)")
	}
	if !showPattern('·', 4, 0) {
		t.Error("showPattern(·, 4, 0) should be true (default)")
	}
	if showPattern('·', 1, 0) {
		t.Error("showPattern(·, 1, 0) should be false (default)")
	}
}

// ── stampANSI tests ──

func TestStampANSISimple(t *testing.T) {
	buf := NewBuffer(20, 5, "#000000")
	// Stamp plain text (no ANSI styling)
	stampANSI(buf, 0, 0, "ABC", 10, 1)

	if buf.Cells[0][0].Char != 'A' {
		t.Errorf("cell[0][0] = %q, want 'A'", buf.Cells[0][0].Char)
	}
	if buf.Cells[0][1].Char != 'B' {
		t.Errorf("cell[0][1] = %q, want 'B'", buf.Cells[0][1].Char)
	}
	if buf.Cells[0][2].Char != 'C' {
		t.Errorf("cell[0][2] = %q, want 'C'", buf.Cells[0][2].Char)
	}
}

func TestStampANSIWithOffset(t *testing.T) {
	buf := NewBuffer(20, 5, "#000000")
	stampANSI(buf, 5, 2, "XY", 10, 1)

	if buf.Cells[2][5].Char != 'X' {
		t.Errorf("cell[2][5] = %q, want 'X'", buf.Cells[2][5].Char)
	}
	if buf.Cells[2][6].Char != 'Y' {
		t.Errorf("cell[2][6] = %q, want 'Y'", buf.Cells[2][6].Char)
	}
	// Before the offset should remain unchanged
	if buf.Cells[0][0].Char != ' ' {
		t.Error("cell[0][0] should remain space")
	}
}

func TestStampANSIMultiline(t *testing.T) {
	buf := NewBuffer(20, 5, "#000000")
	stampANSI(buf, 0, 0, "AB\nCD", 10, 2)

	if buf.Cells[0][0].Char != 'A' {
		t.Errorf("cell[0][0] = %q, want 'A'", buf.Cells[0][0].Char)
	}
	if buf.Cells[1][0].Char != 'C' {
		t.Errorf("cell[1][0] = %q, want 'C'", buf.Cells[1][0].Char)
	}
}

func TestStampANSIOutOfBounds(t *testing.T) {
	buf := NewBuffer(5, 3, "#000000")
	// Stamp at position beyond buffer — should not panic
	stampANSI(buf, 10, 10, "XY", 5, 1)
}

func TestStampANSIWithColor(t *testing.T) {
	buf := NewBuffer(20, 5, "#000000")
	// Red text using ANSI escape
	stampANSI(buf, 0, 0, "\x1b[31mR\x1b[0m", 5, 1)

	if buf.Cells[0][0].Char != 'R' {
		t.Errorf("cell[0][0] = %q, want 'R'", buf.Cells[0][0].Char)
	}
	// The cell should have a foreground color (from the ANSI red sequence)
	if buf.Cells[0][0].Fg == nil {
		t.Error("expected foreground color after ANSI color sequence")
	}
}

// ── Buffer cell state tests ──

func TestBufferCellState(t *testing.T) {
	buf := NewBuffer(5, 3, "#000000")
	buf.Set(0, 0, 'A', "#FFFFFF", "#000000")
	buf.Set(1, 0, 'B', "#FFFFFF", "#000000")
	buf.Cells[0][2] = Cell{Char: 'W', Width: 2, Fg: color.White, Bg: color.Black}
	if buf.Cells[0][0].Char != 'A' {
		t.Error("buffer should contain 'A' at (0,0)")
	}
	if buf.Cells[0][2].Width != 2 {
		t.Error("wide cell should have Width=2")
	}
}

// ── NewBuffer edge cases ──

func TestNewBufferBadColor(t *testing.T) {
	buf := NewBuffer(5, 3, "not-a-color")
	// Should use fallback black
	if buf.Cells[0][0].Bg == nil {
		t.Error("bg should not be nil (fallback black)")
	}
}

// ── SetCell tests ──

func TestBufferSetCell(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	fg := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	bg := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	buf.SetCell(3, 2, 'Z', fg, bg, AttrBold)

	cell := buf.Cells[2][3]
	if cell.Char != 'Z' {
		t.Errorf("char = %q, want 'Z'", cell.Char)
	}
	if cell.Attrs != AttrBold {
		t.Errorf("attrs = %d, want %d", cell.Attrs, AttrBold)
	}
}

func TestBufferSetCellOutOfBounds(t *testing.T) {
	buf := NewBuffer(5, 3, "#000000")
	// Should not panic
	buf.SetCell(-1, 0, 'X', nil, nil, 0)
	buf.SetCell(0, -1, 'X', nil, nil, 0)
	buf.SetCell(5, 0, 'X', nil, nil, 0)
	buf.SetCell(0, 3, 'X', nil, nil, 0)
}

// ── SetStringC tests ──

func TestBufferSetStringC(t *testing.T) {
	buf := NewBuffer(10, 3, "#000000")
	fg := color.RGBA{R: 255, A: 255}
	bg := color.RGBA{G: 255, A: 255}
	buf.SetStringC(1, 0, "Hi", fg, bg)

	if buf.Cells[0][1].Char != 'H' {
		t.Error("expected 'H' at (1,0)")
	}
	if buf.Cells[0][2].Char != 'i' {
		t.Error("expected 'i' at (2,0)")
	}
}

// ── FillRectC tests ──

func TestBufferFillRectC(t *testing.T) {
	buf := NewBuffer(10, 5, "#000000")
	fg := color.RGBA{R: 128, A: 255}
	bg := color.RGBA{B: 128, A: 255}
	r := geometry.Rect{X: 1, Y: 1, Width: 3, Height: 2}
	buf.FillRectC(r, '#', fg, bg)

	if buf.Cells[1][1].Char != '#' {
		t.Error("expected '#' at (1,1)")
	}
	if buf.Cells[2][3].Char != '#' {
		t.Error("expected '#' at (3,2)")
	}
	if buf.Cells[0][0].Char != ' ' {
		t.Error("cell outside fill should be space")
	}
}

// ── NewThemedBuffer tests ──

func TestNewThemedBuffer(t *testing.T) {
	theme := config.RetroTheme()
	buf := NewThemedBuffer(20, 10, theme)

	if buf.Width != 20 || buf.Height != 10 {
		t.Errorf("dimensions = %dx%d, want 20x10", buf.Width, buf.Height)
	}
	if buf.themeName != theme.Name {
		t.Errorf("themeName = %q, want %q", buf.themeName, theme.Name)
	}

	// All cells should have non-nil Bg
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.Cells[y][x].Bg == nil {
				t.Errorf("cell[%d][%d].Bg is nil", y, x)
				return
			}
		}
	}
}

// ── AcquireThemedBuffer / ReleaseBuffer pool tests ──

func TestBufferPoolRoundtrip(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(30, 15, theme)
	if buf.Width != 30 || buf.Height != 15 {
		t.Errorf("acquired buffer = %dx%d, want 30x15", buf.Width, buf.Height)
	}

	ReleaseBuffer(buf)

	// Re-acquire same size should reuse from pool
	buf2 := AcquireThemedBuffer(30, 15, theme)
	if buf2.Width != 30 || buf2.Height != 15 {
		t.Errorf("reacquired buffer = %dx%d, want 30x15", buf2.Width, buf2.Height)
	}
	ReleaseBuffer(buf2)
}

func TestBufferPoolDifferentSize(t *testing.T) {
	theme := config.RetroTheme()
	buf := AcquireThemedBuffer(30, 15, theme)
	ReleaseBuffer(buf)

	// Acquire a different size — should allocate new
	buf2 := AcquireThemedBuffer(40, 20, theme)
	if buf2.Width != 40 || buf2.Height != 20 {
		t.Errorf("different size buffer = %dx%d, want 40x20", buf2.Width, buf2.Height)
	}
	ReleaseBuffer(buf2)
}

// ── emulator pool tests ──

func TestAcquireReleaseEmulator(t *testing.T) {
	emu := acquireEmulator(80, 24)
	if emu == nil {
		t.Fatal("acquireEmulator returned nil")
	}
	releaseEmulator(emu, 80, 24)

	// Acquire again — should get a recycled emulator
	emu2 := acquireEmulator(80, 24)
	if emu2 == nil {
		t.Fatal("acquireEmulator returned nil on reuse")
	}
	releaseEmulator(emu2, 80, 24)
}

