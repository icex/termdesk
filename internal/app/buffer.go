package app

import (
	"image/color"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/charmbracelet/x/vt"
	"github.com/mattn/go-runewidth"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/pkg/geometry"
)

// bufferPool reuses Buffer allocations across frames.
var bufferPool = sync.Pool{}

// emuPoolKey identifies an emulator pool by size.
type emuPoolKey struct{ w, h int }

// emuPools stores per-size sync.Pools for VT emulators used by stampANSI.
var emuPools sync.Map // map[emuPoolKey]*sync.Pool

// acquireEmulator returns a VT emulator of the given size, reusing from pool if available.
func acquireEmulator(w, h int) *vt.Emulator {
	key := emuPoolKey{w, h}
	pool, _ := emuPools.LoadOrStore(key, &sync.Pool{
		New: func() any { return vt.NewEmulator(w, h) },
	})
	return pool.(*sync.Pool).Get().(*vt.Emulator)
}

// releaseEmulator returns a VT emulator to the pool after clearing it.
func releaseEmulator(emu *vt.Emulator, w, h int) {
	// Clear screen and reset cursor so next use starts clean
	emu.Write([]byte("\x1b[2J\x1b[H"))
	key := emuPoolKey{w, h}
	if pool, ok := emuPools.Load(key); ok {
		pool.(*sync.Pool).Put(emu)
	}
}

// runeLen returns the number of runes (display columns) in a string.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// firstRune returns the first rune of a string without allocating a []rune slice.
// Returns ' ' for empty strings.
func firstRune(s string) rune {
	if s == "" {
		return ' '
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

// stripVS16 returns content with U+FE0F (Variation Selector 16) removed when
// the cell width is 1. VS16 triggers emoji presentation in host terminals,
// rendering a nominally 1-cell character as 2 cells wide, which shifts all
// subsequent cells and misaligns borders. For width>1 cells the VS16 is
// harmless (the grid already accounts for the wider display), so content is
// returned unchanged.
func stripVS16(content string, cellWidth int) string {
	if cellWidth > 1 || !strings.Contains(content, "\uFE0F") {
		return content
	}
	return strings.ReplaceAll(content, "\uFE0F", "")
}

// stampANSI parses an ANSI-styled string and writes its cells into the buffer
// at position (x, y). Uses a pooled vt emulator as an ANSI→cells parser.
func stampANSI(buf *Buffer, x, y int, s string, width, height int) {
	emu := acquireEmulator(width, height)
	// VT emulator needs \r\n for proper line breaks; lipgloss outputs bare \n
	s = strings.ReplaceAll(s, "\n", "\r\n")
	emu.Write([]byte(s))
	for row := 0; row < height; row++ {
		for col := 0; col < width; {
			bx, by := x+col, y+row
			if bx < 0 || bx >= buf.Width || by < 0 || by >= buf.Height {
				col++
				continue
			}
			cell := emu.CellAt(col, row)
			ch := ' '
			var content string
			var cellWidth int8 = 1
			if cell != nil {
				ch = firstRune(cell.Content)
				if len(cell.Content) > utf8.RuneLen(ch) {
					content = cell.Content
				}
				if cell.Width > 0 {
					cellWidth = int8(cell.Width)
				}
				if cell.Width == 0 {
					// Continuation cell — skip
					col++
					continue
				}
			}
			var fg, bg color.Color
			var attrs uint8
			if cell != nil {
				fg = cell.Style.Fg
				bg = cell.Style.Bg
				attrs = cell.Style.Attrs
			}
			buf.Cells[by][bx] = Cell{Char: ch, Fg: fg, Bg: bg, Attrs: attrs, Content: content, Width: cellWidth}
			col += int(cellWidth)
		}
	}
	releaseEmulator(emu, width, height)
}

// Cell represents a single terminal cell with character and style.
type Cell struct {
	Char    rune
	Fg      color.Color // nil = default
	Bg      color.Color // nil = default
	Attrs   uint8       // text attributes (bold, italic, etc.)
	Width   int8        // display width: 1 = normal, 2 = wide, 0 = continuation
	Content string      // full grapheme cluster (multi-codepoint emoji); empty = use Char
}

// Text attribute constants matching ultraviolet.
const (
	AttrBold = 1 << iota
	AttrFaint
	AttrItalic
	AttrBlink
	AttrRapidBlink
	AttrReverse
	AttrConceal
	AttrStrikethrough
)

// Buffer is a 2D grid of cells representing the terminal screen.
type Buffer struct {
	Width     int
	Height    int
	Cells     [][]Cell
	themeName string // tracks which theme filled this buffer (for pool reuse optimization)
}

// hexToColor converts a "#RRGGBB" hex string to color.Color.
// Uses manual hex parsing instead of fmt.Sscanf for performance.
func hexToColor(hex string) color.Color {
	if len(hex) != 7 || hex[0] != '#' {
		return nil
	}
	r := hexByte(hex[1], hex[2])
	g := hexByte(hex[3], hex[4])
	b := hexByte(hex[5], hex[6])
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func hexByte(hi, lo byte) uint8 {
	return hexNibble(hi)<<4 | hexNibble(lo)
}

func hexNibble(b byte) uint8 {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

// NewBuffer creates a buffer filled with spaces and the desktop background.
// All cells get explicit Fg and Bg colors to prevent terminal default bleed-through
// (e.g. Termux's blue background showing through after ANSI resets).
func NewBuffer(width, height int, bgColor string) *Buffer {
	fg := color.RGBA{R: 192, G: 192, B: 192, A: 255} // light gray default fg
	bg := hexToColor(bgColor)
	if bg == nil {
		bg = color.RGBA{R: 0, G: 0, B: 0, A: 255} // fallback black
	}
	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			cells[y][x] = Cell{Char: ' ', Fg: fg, Bg: bg, Width: 1}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells}
}

// Set sets a cell at the given position if it's within bounds.
// fg and bg are hex color strings like "#RRGGBB", or "" for default.
func (b *Buffer) Set(x, y int, char rune, fg, bg string) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: hexToColor(fg), Bg: hexToColor(bg), Width: 1}
	}
}

// SetCell sets a cell at the given position with color.Color values directly.
func (b *Buffer) SetCell(x, y int, char rune, fg, bg color.Color, attrs uint8) {
	if x >= 0 && x < b.Width && y >= 0 && y < b.Height {
		b.Cells[y][x] = Cell{Char: char, Fg: fg, Bg: bg, Attrs: attrs, Width: 1}
	}
}

// SetString writes a string starting at (x, y), clipping at buffer edges.
func (b *Buffer) SetString(x, y int, s string, fg, bg string) {
	col := 0
	for _, ch := range s {
		b.Set(x+col, y, ch, fg, bg)
		col++
	}
}

// FillRect fills a rectangular area with a character and colors.
func (b *Buffer) FillRect(r geometry.Rect, char rune, fg, bg string) {
	for y := r.Y; y < r.Bottom(); y++ {
		for x := r.X; x < r.Right(); x++ {
			b.Set(x, y, char, fg, bg)
		}
	}
}

// SetStringC writes a string starting at (x, y) using pre-parsed color.Color values.
func (b *Buffer) SetStringC(x, y int, s string, fg, bg color.Color) {
	col := 0
	for _, ch := range s {
		b.SetCell(x+col, y, ch, fg, bg, 0) // SetCell sets Width=1
		w := runewidth.RuneWidth(ch)
		if w < 1 {
			w = 1
		}
		col += w
	}
}

// SetStringCA writes a string starting at (x, y) with pre-parsed colors and attributes.
func (b *Buffer) SetStringCA(x, y int, s string, fg, bg color.Color, attrs uint8) {
	col := 0
	for _, ch := range s {
		b.SetCell(x+col, y, ch, fg, bg, attrs)
		w := runewidth.RuneWidth(ch)
		if w < 1 {
			w = 1
		}
		col += w
	}
}

// FillRectC fills a rectangular area using pre-parsed color.Color values.
func (b *Buffer) FillRectC(r geometry.Rect, char rune, fg, bg color.Color) {
	for y := r.Y; y < r.Bottom(); y++ {
		for x := r.X; x < r.Right(); x++ {
			b.SetCell(x, y, char, fg, bg, 0) // SetCell sets Width=1
		}
	}
}

// Blit copies src buffer cells into this buffer at (x, y).
func (b *Buffer) Blit(x, y int, src *Buffer) {
	if src == nil {
		return
	}
	for sy := 0; sy < src.Height; sy++ {
		dy := y + sy
		if dy < 0 || dy >= b.Height {
			continue
		}
		for sx := 0; sx < src.Width; sx++ {
			dx := x + sx
			if dx < 0 || dx >= b.Width {
				continue
			}
			b.Cells[dy][dx] = src.Cells[sy][sx]
		}
	}
}

// showPattern determines if a pattern character should be drawn at (x, y).
// Different patterns use different spatial algorithms for varied visual density.
func showPattern(patChar rune, x, y int) bool {
	switch patChar {
	case 0:
		return false // no pattern
	case '░', '▒', '▓':
		// Block patterns fill every cell (dense)
		return true
	case '▚':
		// Diagonal stripe: checkerboard creating a half-tone weave
		return (x+y)%2 == 0
	case '⠿':
		// Braille dots: staggered grid creating a fine mesh
		return (x%3 == 0 && y%2 == 0) || (x%3 == 1 && y%2 == 1)
	case '╳':
		// Cross-hatch: intersecting diagonals every 3 cells
		return (x+y)%3 == 0 || (x-y+100)%3 == 0
	case '◇':
		// Diamond outlines: offset grid creating a gem-like field
		return (x%5 == 0 && y%3 == 0) || (x%5 == 2 && y%3 == 1)
	case '◆':
		// Filled diamonds: scattered sparse pattern
		return (x*7+y*13)%11 == 0
	case '≈':
		// Wave pattern: horizontal ripples with phase offset per row
		return (x+(y/2))%5 == 0
	default:
		// Sparse diamond pattern for dots/symbols (original behavior)
		return (x+y)%4 == 0
	}
}

// WallpaperConfig describes how the desktop background should be filled.
type WallpaperConfig struct {
	Mode      string     // "theme", "color", "pattern", "program"
	Color     color.Color // solid fill color for "color" mode
	PatChar   rune        // pattern char for "pattern" mode
	PatFg     color.Color // pattern foreground for "pattern" mode
	PatBg     color.Color // pattern background for "pattern" mode
}

func NewThemedBuffer(width, height int, theme config.Theme) *Buffer {
	return NewWallpaperBuffer(width, height, theme, nil)
}

// NewWallpaperBuffer creates a buffer filled with the appropriate background.
// If wp is nil or mode is "theme", uses the theme's default desktop pattern.
func NewWallpaperBuffer(width, height int, theme config.Theme, wp *WallpaperConfig) *Buffer {
	c := theme.C()
	fg := c.DefaultFg
	bg := c.DesktopBg
	if bg == nil {
		bg = color.RGBA{A: 255}
	}

	patChar := theme.DesktopPatternChar
	patFg := c.DesktopPatternFg
	if patFg == nil {
		patFg = fg
	}

	// Apply wallpaper overrides
	if wp != nil {
		switch wp.Mode {
		case "color":
			if wp.Color != nil {
				bg = wp.Color
			}
			patChar = 0 // no pattern in solid color mode
		case "pattern":
			if wp.PatBg != nil {
				bg = wp.PatBg
			}
			if wp.PatChar != 0 {
				patChar = wp.PatChar
			}
			if wp.PatFg != nil {
				patFg = wp.PatFg
			}
		case "program":
			// Program mode: solid bg fill, terminal content overlaid later
			patChar = 0
		}
	}

	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			ch := ' '
			cellFg := fg
			if patChar != 0 {
				if showPattern(patChar, x, y) {
					ch = patChar
					cellFg = patFg
				}
			}
			cells[y][x] = Cell{Char: ch, Fg: cellFg, Bg: bg, Width: 1}
		}
	}
	return &Buffer{Width: width, Height: height, Cells: cells, themeName: theme.Name}
}

// AcquireThemedBuffer gets a buffer from the pool or creates a new one.
// The buffer is filled with the theme's background pattern, ready for rendering.
func AcquireThemedBuffer(width, height int, theme config.Theme) *Buffer {
	return AcquireWallpaperBuffer(width, height, theme, nil)
}

// AcquireWallpaperBuffer gets a buffer from the pool or creates a new one.
// The buffer is filled according to the wallpaper config.
func AcquireWallpaperBuffer(width, height int, theme config.Theme, wp *WallpaperConfig) *Buffer {
	if v := bufferPool.Get(); v != nil {
		buf := v.(*Buffer)
		if buf.Width == width && buf.Height == height {
			fillWallpaper(buf, theme, wp)
			return buf
		}
	}
	return NewWallpaperBuffer(width, height, theme, wp)
}

// ReleaseBuffer returns a buffer to the pool for reuse.
func ReleaseBuffer(buf *Buffer) {
	bufferPool.Put(buf)
}

// fillThemed resets all cells in an existing buffer to the theme's background pattern.
func fillThemed(buf *Buffer, theme config.Theme) {
	fillWallpaper(buf, theme, nil)
}

// fillWallpaper resets all cells to the wallpaper-aware background.
func fillWallpaper(buf *Buffer, theme config.Theme, wp *WallpaperConfig) {
	buf.themeName = theme.Name
	c := theme.C()
	fg := c.DefaultFg
	bg := c.DesktopBg
	if bg == nil {
		bg = color.RGBA{A: 255}
	}
	patChar := theme.DesktopPatternChar
	patFg := c.DesktopPatternFg
	if patFg == nil {
		patFg = fg
	}

	if wp != nil {
		switch wp.Mode {
		case "color":
			if wp.Color != nil {
				bg = wp.Color
			}
			patChar = 0
		case "pattern":
			if wp.PatBg != nil {
				bg = wp.PatBg
			}
			if wp.PatChar != 0 {
				patChar = wp.PatChar
			}
			if wp.PatFg != nil {
				patFg = wp.PatFg
			}
		case "program":
			patChar = 0
		}
	}

	for y := range buf.Cells {
		for x := range buf.Cells[y] {
			ch := ' '
			cellFg := fg
			if patChar != 0 {
				if showPattern(patChar, x, y) {
					ch = patChar
					cellFg = patFg
				}
			}
			buf.Cells[y][x] = Cell{Char: ch, Fg: cellFg, Bg: bg, Width: 1}
		}
	}
}
