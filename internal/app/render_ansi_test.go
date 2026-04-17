package app

import (
	"image/color"
	"strings"
	"testing"
)

// ── appendSGRColorFg tests ──

func TestAppendSGRColorFg(t *testing.T) {
	var sb strings.Builder
	c := color.RGBA{R: 255, G: 128, B: 0, A: 255}
	appendSGRColorFg(&sb, c)

	got := sb.String()
	if !strings.HasPrefix(got, ";38;2;") {
		t.Errorf("appendSGRColorFg = %q, want prefix ';38;2;'", got)
	}
	if got != ";38;2;255;128;0" {
		t.Errorf("appendSGRColorFg = %q, want ';38;2;255;128;0'", got)
	}
}

func TestAppendSGRColorFgNil(t *testing.T) {
	var sb strings.Builder
	appendSGRColorFg(&sb, nil)
	if sb.Len() != 0 {
		t.Error("appendSGRColorFg(nil) should write nothing")
	}
}

// ── appendSGRColorBg tests ──

func TestAppendSGRColorBg(t *testing.T) {
	var sb strings.Builder
	c := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	appendSGRColorBg(&sb, c)

	got := sb.String()
	if got != ";48;2;10;20;30" {
		t.Errorf("appendSGRColorBg = %q, want ';48;2;10;20;30'", got)
	}
}

func TestAppendSGRColorBgNil(t *testing.T) {
	var sb strings.Builder
	appendSGRColorBg(&sb, nil)
	if sb.Len() != 0 {
		t.Error("appendSGRColorBg(nil) should write nothing")
	}
}

// ── writeColorFg tests ──

func TestWriteColorFg(t *testing.T) {
	var sb strings.Builder
	c := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	writeColorFg(&sb, c)

	got := sb.String()
	if !strings.HasPrefix(got, "\x1b[38;2;") {
		t.Errorf("writeColorFg = %q, want prefix '\\x1b[38;2;'", got)
	}
	if !strings.HasSuffix(got, "m") {
		t.Errorf("writeColorFg = %q, should end with 'm'", got)
	}
	if got != "\x1b[38;2;200;100;50m" {
		t.Errorf("writeColorFg = %q, want '\\x1b[38;2;200;100;50m'", got)
	}
}

func TestWriteColorFgNil(t *testing.T) {
	var sb strings.Builder
	writeColorFg(&sb, nil)
	if sb.Len() != 0 {
		t.Error("writeColorFg(nil) should write nothing")
	}
}

// ── writeColorBg tests ──

func TestWriteColorBg(t *testing.T) {
	var sb strings.Builder
	c := color.RGBA{R: 0, G: 255, B: 128, A: 255}
	writeColorBg(&sb, c)

	got := sb.String()
	if got != "\x1b[48;2;0;255;128m" {
		t.Errorf("writeColorBg = %q, want '\\x1b[48;2;0;255;128m'", got)
	}
}

func TestWriteColorBgNil(t *testing.T) {
	var sb strings.Builder
	writeColorBg(&sb, nil)
	if sb.Len() != 0 {
		t.Error("writeColorBg(nil) should write nothing")
	}
}

// ── attrsToANSI tests ──

func TestAttrsToANSIZero(t *testing.T) {
	if got := attrsToANSI(0); got != "" {
		t.Errorf("attrsToANSI(0) = %q, want empty", got)
	}
}

func TestAttrsToANSIBold(t *testing.T) {
	got := attrsToANSI(AttrBold)
	if got != "\x1b[1m" {
		t.Errorf("attrsToANSI(Bold) = %q, want '\\x1b[1m'", got)
	}
}

func TestAttrsToANSIItalic(t *testing.T) {
	got := attrsToANSI(AttrItalic)
	if got != "\x1b[3m" {
		t.Errorf("attrsToANSI(Italic) = %q, want '\\x1b[3m'", got)
	}
}

func TestAttrsToANSIMultiple(t *testing.T) {
	got := attrsToANSI(AttrBold | AttrItalic)
	if got != "\x1b[1;3m" {
		t.Errorf("attrsToANSI(Bold|Italic) = %q, want '\\x1b[1;3m'", got)
	}
}

func TestAttrsToANSIFaint(t *testing.T) {
	got := attrsToANSI(AttrFaint)
	if got != "\x1b[2m" {
		t.Errorf("attrsToANSI(Faint) = %q, want '\\x1b[2m'", got)
	}
}

func TestAttrsToANSIBlink(t *testing.T) {
	got := attrsToANSI(AttrBlink)
	if got != "\x1b[5m" {
		t.Errorf("attrsToANSI(Blink) = %q, want '\\x1b[5m'", got)
	}
}

func TestAttrsToANSIReverse(t *testing.T) {
	got := attrsToANSI(AttrReverse)
	if got != "\x1b[7m" {
		t.Errorf("attrsToANSI(Reverse) = %q, want '\\x1b[7m'", got)
	}
}

func TestAttrsToANSIConceal(t *testing.T) {
	got := attrsToANSI(AttrConceal)
	if got != "\x1b[8m" {
		t.Errorf("attrsToANSI(Conceal) = %q, want '\\x1b[8m'", got)
	}
}

func TestAttrsToANSIStrikethrough(t *testing.T) {
	got := attrsToANSI(AttrStrikethrough)
	if got != "\x1b[9m" {
		t.Errorf("attrsToANSI(Strikethrough) = %q, want '\\x1b[9m'", got)
	}
}

func TestAttrsToANSIAll(t *testing.T) {
	all := uint8(AttrBold | AttrFaint | AttrItalic | AttrBlink | AttrReverse | AttrConceal | AttrStrikethrough)
	got := attrsToANSI(all)
	if got != "\x1b[1;2;3;5;7;8;9m" {
		t.Errorf("attrsToANSI(all) = %q, want '\\x1b[1;2;3;5;7;8;9m'", got)
	}
}

func TestAttrsToANSIRapidBlinkOnly(t *testing.T) {
	// AttrRapidBlink is bit 4 but not included in attrsToANSI — no SGR code for it
	got := attrsToANSI(AttrRapidBlink)
	if got != "" {
		t.Errorf("attrsToANSI(RapidBlink) = %q, want empty (not mapped to SGR)", got)
	}
}

// ── desaturateColor tests ──

func TestDesaturateColorNil(t *testing.T) {
	if got := desaturateColor(nil, 0.5); got != nil {
		t.Error("desaturateColor(nil) should return nil")
	}
}

func TestDesaturateColorZero(t *testing.T) {
	c := color.RGBA{R: 255, G: 100, B: 50, A: 255}
	got := desaturateColor(c, 0)
	r, g, b, _ := got.RGBA()
	if r>>8 != 255 || g>>8 != 100 || b>>8 != 50 {
		t.Errorf("desaturateColor(t=0) = (%d,%d,%d), want original (255,100,50)", r>>8, g>>8, b>>8)
	}
}

func TestDesaturateColorFull(t *testing.T) {
	c := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	got := desaturateColor(c, 1.0)
	r, g, b, _ := got.RGBA()
	// Full desaturation should produce grayscale
	// For pure red: lum = 0.2126*255 = 54.213
	// All channels should be ~54
	if r>>8 != g>>8 || g>>8 != b>>8 {
		t.Errorf("desaturateColor(t=1) should be grayscale, got (%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

// ── blendColor tests ──

func TestBlendColorBothNil(t *testing.T) {
	got := blendColor(nil, nil, 0.5)
	if got != nil {
		t.Error("blendColor(nil, nil) should return nil")
	}
}

func TestBlendColorFirstNil(t *testing.T) {
	c2 := color.RGBA{R: 100, A: 255}
	got := blendColor(nil, c2, 0.5)
	r, _, _, _ := got.RGBA()
	if r>>8 != 100 {
		t.Error("blendColor(nil, c2) should return c2")
	}
}

func TestBlendColorSecondNil(t *testing.T) {
	c1 := color.RGBA{R: 200, A: 255}
	got := blendColor(c1, nil, 0.5)
	r, _, _, _ := got.RGBA()
	if r>>8 != 200 {
		t.Error("blendColor(c1, nil) should return c1")
	}
}

func TestBlendColorStart(t *testing.T) {
	c1 := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	c2 := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	got := blendColor(c1, c2, 0)
	r, g, b, _ := got.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("blendColor(t=0) = (%d,%d,%d), want (255,0,0)", r>>8, g>>8, b>>8)
	}
}

func TestBlendColorEnd(t *testing.T) {
	c1 := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	c2 := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	got := blendColor(c1, c2, 1.0)
	r, g, b, _ := got.RGBA()
	if r>>8 != 0 || g>>8 != 255 || b>>8 != 0 {
		t.Errorf("blendColor(t=1) = (%d,%d,%d), want (0,255,0)", r>>8, g>>8, b>>8)
	}
}

func TestBlendColorMidpoint(t *testing.T) {
	c1 := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	c2 := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	got := blendColor(c1, c2, 0.5)
	r, g, b, _ := got.RGBA()
	if r>>8 != 100 || g>>8 != 50 || b>>8 != 25 {
		t.Errorf("blendColor(t=0.5) = (%d,%d,%d), want (100,50,25)", r>>8, g>>8, b>>8)
	}
}

// ── colorsEqual tests ──

func TestColorsEqualBothNil(t *testing.T) {
	if !colorsEqual(nil, nil) {
		t.Error("colorsEqual(nil, nil) should be true")
	}
}

func TestColorsEqualFirstNil(t *testing.T) {
	if colorsEqual(nil, color.Black) {
		t.Error("colorsEqual(nil, black) should be false")
	}
}

func TestColorsEqualSecondNil(t *testing.T) {
	if colorsEqual(color.Black, nil) {
		t.Error("colorsEqual(black, nil) should be false")
	}
}

func TestColorsEqualSame(t *testing.T) {
	c := color.RGBA{R: 128, G: 64, B: 32, A: 255}
	if !colorsEqual(c, c) {
		t.Error("colorsEqual with same color should be true")
	}
}

func TestColorsEqualDifferent(t *testing.T) {
	c1 := color.RGBA{R: 128, G: 64, B: 32, A: 255}
	c2 := color.RGBA{R: 129, G: 64, B: 32, A: 255}
	if colorsEqual(c1, c2) {
		t.Error("colorsEqual with different colors should be false")
	}
}

// ── BufferToString edge cases ──

func TestBufferToStringWithColors(t *testing.T) {
	buf := NewBuffer(3, 2, "#000000")
	// Set cells with explicit colors
	red := color.RGBA{R: 255, A: 255}
	green := color.RGBA{G: 255, A: 255}
	buf.SetCell(0, 0, 'R', red, color.Black, 0)
	buf.SetCell(1, 0, 'G', green, color.Black, 0)

	s := BufferToString(buf)
	if !strings.Contains(s, "R") {
		t.Error("output should contain 'R'")
	}
	if !strings.Contains(s, "G") {
		t.Error("output should contain 'G'")
	}
	// Should contain ANSI color escapes
	if !strings.Contains(s, "\x1b[") {
		t.Error("output should contain ANSI escape sequences")
	}
}

func TestBufferToStringWithAttrs(t *testing.T) {
	buf := NewBuffer(3, 1, "#000000")
	buf.Cells[0][0] = Cell{Char: 'B', Fg: color.White, Bg: color.Black, Attrs: AttrBold, Width: 1}
	buf.Cells[0][1] = Cell{Char: 'I', Fg: color.White, Bg: color.Black, Attrs: AttrItalic, Width: 1}
	buf.Cells[0][2] = Cell{Char: 'N', Fg: color.White, Bg: color.Black, Attrs: 0, Width: 1}

	s := BufferToString(buf)
	if !strings.Contains(s, "B") || !strings.Contains(s, "I") || !strings.Contains(s, "N") {
		t.Error("output should contain B, I, N")
	}
	// Should contain bold sequence (;1) within a reset-based SGR
	if !strings.Contains(s, ";1") {
		t.Error("output should contain bold attribute (;1)")
	}
}

func TestBufferToStringWideChar(t *testing.T) {
	buf := NewBuffer(5, 1, "#000000")
	buf.Cells[0][0] = Cell{Char: '日', Fg: color.White, Bg: color.Black, Width: 2}
	buf.Cells[0][1] = Cell{Char: ' ', Width: 0} // continuation cell
	buf.Cells[0][2] = Cell{Char: 'X', Fg: color.White, Bg: color.Black, Width: 1}

	s := BufferToString(buf)
	if !strings.Contains(s, "日") {
		t.Error("output should contain wide character '日'")
	}
	if !strings.Contains(s, "X") {
		t.Error("output should contain 'X'")
	}
}

func TestBufferToStringSingleCell(t *testing.T) {
	buf := NewBuffer(1, 1, "#000000")
	buf.Set(0, 0, '@', "#FFFFFF", "#000000")
	s := BufferToString(buf)
	if !strings.Contains(s, "@") {
		t.Error("single cell output should contain '@'")
	}
	if !strings.HasSuffix(s, "\x1b[0m") {
		t.Error("output should end with ANSI reset")
	}
}

func TestBufferToStringNoColorChange(t *testing.T) {
	buf := NewBuffer(3, 1, "#000000")
	fg := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	bg := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	// All cells have the same colors — should emit color only once
	buf.SetCell(0, 0, 'A', fg, bg, 0)
	buf.SetCell(1, 0, 'B', fg, bg, 0)
	buf.SetCell(2, 0, 'C', fg, bg, 0)

	s := BufferToString(buf)
	if !strings.Contains(s, "ABC") {
		t.Error("output should contain 'ABC'")
	}
}

// ── stripANSI edge cases ──

func TestStripANSINestedSequences(t *testing.T) {
	input := "\x1b[1;38;2;255;0;0mHello\x1b[0m"
	got := string(stripANSI(input))
	if got != "Hello" {
		t.Errorf("stripANSI with complex SGR = %q, want 'Hello'", got)
	}
}

func TestStripANSIOnlyEscapes(t *testing.T) {
	input := "\x1b[31m\x1b[42m\x1b[0m"
	got := string(stripANSI(input))
	if got != "" {
		t.Errorf("stripANSI(only escapes) = %q, want empty", got)
	}
}

func TestBufferToStringEmojiVS16Alignment(t *testing.T) {
	// Simulate VT emulator output for "☀️ test" where ☀️ has Width=2 (VS16 emoji)
	buf := NewBuffer(10, 1, "#000000")
	fg := color.RGBA{R: 192, G: 192, B: 192, A: 255}
	bg := color.RGBA{A: 255}

	// ☀️ at col 0, Width=2 (as the VT emulator reports for VS16 emoji)
	buf.Cells[0][0] = Cell{Char: '☀', Content: "☀\uFE0F", Fg: fg, Bg: bg, Width: 2}
	buf.Cells[0][1] = Cell{Char: ' ', Fg: fg, Bg: bg, Width: 0} // continuation
	buf.Cells[0][2] = Cell{Char: ' ', Fg: fg, Bg: bg, Width: 1} // space
	buf.Cells[0][3] = Cell{Char: 't', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][4] = Cell{Char: 'e', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][5] = Cell{Char: 's', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][6] = Cell{Char: 't', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][7] = Cell{Char: '|', Fg: fg, Bg: bg, Width: 1} // border

	s := BufferToString(buf)
	text := string(stripANSI(s))

	// The output should have ☀️ (2 cells) + space + "test" + "|"
	// Continuation cell at pos 1 should be SKIPPED (not output)
	if strings.Contains(text, "☀\uFE0F ") && strings.Contains(text, "test|") {
		// good
	} else if strings.Contains(text, "☀ ") && strings.Contains(text, "test|") {
		// also acceptable - VS16 may be stripped
	} else {
		t.Errorf("emoji alignment broken: got text %q", text)
	}

	// Verify no double space between emoji and text
	// If continuation cell leaks, we'd see "☀️  test" (double space) or "☀️ X test" (garbage)
	emojiIdx := strings.Index(text, "☀")
	if emojiIdx >= 0 {
		after := text[emojiIdx:]
		if strings.Contains(after, "  test") {
			t.Errorf("continuation cell leaked as extra space: %q", text)
		}
	}
}

func TestBufferToStringNarrowEmojiNoVS16Alignment(t *testing.T) {
	// Test that ☀ WITHOUT VS16 (Width=1 in emulator) stays narrow
	buf := NewBuffer(8, 1, "#000000")
	fg := color.RGBA{R: 192, G: 192, B: 192, A: 255}
	bg := color.RGBA{A: 255}

	buf.Cells[0][0] = Cell{Char: '☀', Fg: fg, Bg: bg, Width: 1} // narrow
	buf.Cells[0][1] = Cell{Char: ' ', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][2] = Cell{Char: 'A', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][3] = Cell{Char: 'B', Fg: fg, Bg: bg, Width: 1}
	buf.Cells[0][4] = Cell{Char: '|', Fg: fg, Bg: bg, Width: 1} // border

	s := BufferToString(buf)
	text := string(stripANSI(s))

	if !strings.Contains(text, "☀ AB|") {
		t.Errorf("narrow emoji alignment broken: got %q, want '☀ AB|'", text)
	}
}

func BenchmarkBufferToString(b *testing.B) {
	buf := NewBuffer(120, 30, "#1e1e2e")
	// Fill with varied colors to exercise cache
	colors := []color.Color{
		color.RGBA{R: 255, A: 255},
		color.RGBA{G: 255, A: 255},
		color.RGBA{B: 255, A: 255},
		color.RGBA{R: 200, G: 200, B: 200, A: 255},
		color.RGBA{R: 40, G: 40, B: 40, A: 255},
	}
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			fg := colors[(x+y)%len(colors)]
			bg := colors[(x+y+1)%len(colors)]
			buf.Cells[y][x] = Cell{Char: 'A' + rune(x%26), Fg: fg, Bg: bg, Width: 1}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BufferToString(buf)
	}
}

func BenchmarkBufferToStringSameColor(b *testing.B) {
	buf := NewBuffer(120, 30, "#1e1e2e")
	// All same color — cache hit rate should be 100%
	fg := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	bg := color.RGBA{R: 30, G: 30, B: 46, A: 255}
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			buf.Cells[y][x] = Cell{Char: 'X', Fg: fg, Bg: bg, Width: 1}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BufferToString(buf)
	}
}
