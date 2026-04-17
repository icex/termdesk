package app

import (
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// hostEmojiWide is true when the host terminal renders emoji-capable characters
// (e.g. ☀ U+2600) as 2 cells even without VS16. Detected at startup via
// TERMDESK_EMOJI_WIDTH=2 env var (set by probeEmojiWidth in main.go).
var hostEmojiWide = initHostEmojiWide()

func initHostEmojiWide() bool {
	val := os.Getenv("TERMDESK_EMOJI_WIDTH")
	if val == "2" {
		// Can't use logging here (may not be initialized yet)
		return true
	}
	return false
}

// builderPool reuses strings.Builder allocations in BufferToString.
var builderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

const (
	styleCacheMax = 4096
	colorCacheMax = 4096
	colorNilKey   = 0xffffffff
)

type styleKey struct {
	fg    uint32
	bg    uint32
	attrs uint8
}

var (
	styleCacheMu sync.RWMutex
	styleCache   = make(map[styleKey]string, styleCacheMax)
	fgCacheMu    sync.RWMutex
	fgCache      = make(map[uint32]string, colorCacheMax)
	bgCacheMu    sync.RWMutex
	bgCache      = make(map[uint32]string, colorCacheMax)
)

func colorKey(c color.Color) uint32 {
	if c == nil {
		return colorNilKey
	}
	// ANSI palette colors get a distinct key space (bit 31 set) to avoid
	// cache collisions with TrueColor values that happen to have the same RGB.
	if idx, ok := ansiPaletteIndex(c); ok {
		return 0x80000000 | uint32(idx)
	}
	r, g, b, _ := c.RGBA()
	return (uint32(r>>8) << 16) | (uint32(g>>8) << 8) | uint32(b>>8)
}

func sgrForAttrsAndColors(attrs uint8, fg, bg color.Color) string {
	key := styleKey{fg: colorKey(fg), bg: colorKey(bg), attrs: attrs}
	return sgrForStyleKey(key, fg, bg)
}

// sgrForStyleKey looks up or generates the combined SGR sequence for a given key.
// fg/bg are passed for generating the sequence on cache miss.
func sgrForStyleKey(key styleKey, fg, bg color.Color) string {
	styleCacheMu.RLock()
	if s, ok := styleCache[key]; ok {
		styleCacheMu.RUnlock()
		return s
	}
	styleCacheMu.RUnlock()

	var sb strings.Builder
	sb.Grow(32)
	sb.WriteString("\x1b[0")
	if key.attrs&AttrBold != 0 {
		sb.WriteString(";1")
	}
	if key.attrs&AttrFaint != 0 {
		sb.WriteString(";2")
	}
	if key.attrs&AttrItalic != 0 {
		sb.WriteString(";3")
	}
	if key.attrs&AttrBlink != 0 {
		sb.WriteString(";5")
	}
	if key.attrs&AttrReverse != 0 {
		sb.WriteString(";7")
	}
	if key.attrs&AttrConceal != 0 {
		sb.WriteString(";8")
	}
	if key.attrs&AttrStrikethrough != 0 {
		sb.WriteString(";9")
	}
	appendSGRColorFg(&sb, fg)
	appendSGRColorBg(&sb, bg)
	sb.WriteByte('m')
	s := sb.String()

	styleCacheMu.Lock()
	if len(styleCache) >= styleCacheMax {
		// Evict half instead of clearing all — preserves frequently-used entries
		i := 0
		half := styleCacheMax / 2
		for k := range styleCache {
			if i >= half {
				break
			}
			delete(styleCache, k)
			i++
		}
	}
	styleCache[key] = s
	styleCacheMu.Unlock()
	return s
}

func sgrForFg(c color.Color) string {
	key := colorKey(c)
	if key == colorNilKey {
		return ""
	}
	return sgrForFgKey(key, c)
}

// sgrForFgKey looks up or generates the foreground sequence for a given color key.
func sgrForFgKey(key uint32, c color.Color) string {
	fgCacheMu.RLock()
	if s, ok := fgCache[key]; ok {
		fgCacheMu.RUnlock()
		return s
	}
	fgCacheMu.RUnlock()

	var sb strings.Builder
	sb.Grow(24)
	writeColorFg(&sb, c)
	s := sb.String()

	fgCacheMu.Lock()
	if len(fgCache) >= colorCacheMax {
		i := 0
		half := colorCacheMax / 2
		for k := range fgCache {
			if i >= half {
				break
			}
			delete(fgCache, k)
			i++
		}
	}
	fgCache[key] = s
	fgCacheMu.Unlock()
	return s
}

func sgrForBg(c color.Color) string {
	key := colorKey(c)
	if key == colorNilKey {
		return ""
	}
	return sgrForBgKey(key, c)
}

// sgrForBgKey looks up or generates the background sequence for a given color key.
func sgrForBgKey(key uint32, c color.Color) string {
	bgCacheMu.RLock()
	if s, ok := bgCache[key]; ok {
		bgCacheMu.RUnlock()
		return s
	}
	bgCacheMu.RUnlock()

	var sb strings.Builder
	sb.Grow(24)
	writeColorBg(&sb, c)
	s := sb.String()

	bgCacheMu.Lock()
	if len(bgCache) >= colorCacheMax {
		i := 0
		half := colorCacheMax / 2
		for k := range bgCache {
			if i >= half {
				break
			}
			delete(bgCache, k)
			i++
		}
	}
	bgCache[key] = s
	bgCacheMu.Unlock()
	return s
}

// writeColorFg writes an ANSI foreground escape sequence to the builder.
// For ANSI palette colors (BasicColor/IndexedColor 0-255), outputs palette
// escape codes so the outer terminal applies its own palette — just like
// tmux/screen. TrueColor values use 24-bit RGB sequences.
func writeColorFg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	if idx, ok := ansiPaletteIndex(c); ok {
		writeANSIFg(sb, idx)
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString("\x1b[38;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
	sb.WriteByte('m')
}

// writeColorBg writes an ANSI background escape sequence to the builder.
func writeColorBg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	if idx, ok := ansiPaletteIndex(c); ok {
		writeANSIBg(sb, idx)
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString("\x1b[48;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
	sb.WriteByte('m')
}

// appendSGRColorFg appends ";38;2;R;G;B" to a combined SGR sequence.
// Used within a single \x1b[...m sequence to avoid separate resets.
func appendSGRColorFg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	if idx, ok := ansiPaletteIndex(c); ok {
		appendANSIFg(sb, idx)
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString(";38;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
}

// appendSGRColorBg appends ";48;2;R;G;B" to a combined SGR sequence.
func appendSGRColorBg(sb *strings.Builder, c color.Color) {
	if c == nil {
		return
	}
	if idx, ok := ansiPaletteIndex(c); ok {
		appendANSIBg(sb, idx)
		return
	}
	r, g, b, _ := c.RGBA()
	sb.WriteString(";48;2;")
	sb.WriteString(strconv.FormatUint(uint64(r>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(g>>8), 10))
	sb.WriteByte(';')
	sb.WriteString(strconv.FormatUint(uint64(b>>8), 10))
}

// ansiPaletteIndex extracts the palette index from ANSI color types.
// Returns (index, true) for BasicColor (0-15) and IndexedColor (0-255).
func ansiPaletteIndex(c color.Color) (int, bool) {
	switch v := c.(type) {
	case ansi.BasicColor:
		return int(v), true
	case ansi.IndexedColor:
		return int(v), true
	}
	return 0, false
}

// writeANSIFg writes an ANSI palette foreground escape sequence.
// 0-7: \e[30-37m, 8-15: \e[90-97m, 16-255: \e[38;5;Nm
func writeANSIFg(sb *strings.Builder, idx int) {
	if idx < 8 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(30 + idx))
		sb.WriteByte('m')
	} else if idx < 16 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(90 + idx - 8))
		sb.WriteByte('m')
	} else {
		sb.WriteString("\x1b[38;5;")
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteByte('m')
	}
}

// writeANSIBg writes an ANSI palette background escape sequence.
// 0-7: \e[40-47m, 8-15: \e[100-107m, 16-255: \e[48;5;Nm
func writeANSIBg(sb *strings.Builder, idx int) {
	if idx < 8 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(40 + idx))
		sb.WriteByte('m')
	} else if idx < 16 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(100 + idx - 8))
		sb.WriteByte('m')
	} else {
		sb.WriteString("\x1b[48;5;")
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteByte('m')
	}
}

// appendANSIFg appends ANSI palette fg to a combined SGR sequence.
func appendANSIFg(sb *strings.Builder, idx int) {
	sb.WriteByte(';')
	if idx < 8 {
		sb.WriteString(strconv.Itoa(30 + idx))
	} else if idx < 16 {
		sb.WriteString(strconv.Itoa(90 + idx - 8))
	} else {
		sb.WriteString("38;5;")
		sb.WriteString(strconv.Itoa(idx))
	}
}

// appendANSIBg appends ANSI palette bg to a combined SGR sequence.
func appendANSIBg(sb *strings.Builder, idx int) {
	sb.WriteByte(';')
	if idx < 8 {
		sb.WriteString(strconv.Itoa(40 + idx))
	} else if idx < 16 {
		sb.WriteString(strconv.Itoa(100 + idx - 8))
	} else {
		sb.WriteString("48;5;")
		sb.WriteString(strconv.Itoa(idx))
	}
}

// attrsToANSI returns ANSI SGR sequences for text attributes.
func attrsToANSI(attrs uint8) string {
	if attrs == 0 {
		return ""
	}
	var parts []string
	if attrs&AttrBold != 0 {
		parts = append(parts, "1")
	}
	if attrs&AttrFaint != 0 {
		parts = append(parts, "2")
	}
	if attrs&AttrItalic != 0 {
		parts = append(parts, "3")
	}
	if attrs&AttrBlink != 0 {
		parts = append(parts, "5")
	}
	if attrs&AttrReverse != 0 {
		parts = append(parts, "7")
	}
	if attrs&AttrConceal != 0 {
		parts = append(parts, "8")
	}
	if attrs&AttrStrikethrough != 0 {
		parts = append(parts, "9")
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// desaturateColor converts a color toward grayscale.
// t=0 returns the original, t=1 returns fully grayscale.
func desaturateColor(c color.Color, t float64) color.Color {
	if c == nil {
		return nil
	}
	r, g, b, _ := c.RGBA()
	rr := float64(r >> 8)
	gg := float64(g >> 8)
	bb := float64(b >> 8)
	// Perceived luminance (ITU-R BT.709)
	lum := 0.2126*rr + 0.7152*gg + 0.0722*bb
	return color.RGBA{
		R: uint8(rr*(1-t) + lum*t),
		G: uint8(gg*(1-t) + lum*t),
		B: uint8(bb*(1-t) + lum*t),
		A: 255,
	}
}

// blendColor linearly interpolates between two colors.
// t=0 returns c1, t=1 returns c2.
func blendColor(c1, c2 color.Color, t float64) color.Color {
	if c1 == nil {
		return c2
	}
	if c2 == nil {
		return c1
	}
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()
	blend := func(a, b uint32) uint8 {
		return uint8((float64(a>>8)*(1-t) + float64(b>>8)*t))
	}
	return color.RGBA{
		R: blend(r1, r2),
		G: blend(g1, g2),
		B: blend(b1, b2),
		A: 255,
	}
}

// darkenColor reduces brightness of a color by the given factor (0.0–1.0).
// Used to improve icon contrast on light backgrounds.
func darkenColor(c color.Color, factor float64) color.Color {
	if c == nil {
		return nil
	}
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(float64(r>>8) * factor),
		G: uint8(float64(g>>8) * factor),
		B: uint8(float64(b>>8) * factor),
		A: 255,
	}
}

// colorsEqual compares two color.Color values for equality.
func colorsEqual(a, b color.Color) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// stripANSI removes ANSI escape sequences from a string, returning runes.
func stripANSI(s string) []rune {
	var result []rune
	i := 0
	runes := []rune(s)
	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) {
			i++ // skip ESC
			switch {
			case runes[i] == '[':
				// CSI sequence: ESC [ ... final byte (0x40-0x7E)
				i++
				for i < len(runes) && (runes[i] < 0x40 || runes[i] > 0x7E) {
					i++
				}
				if i < len(runes) {
					i++ // skip final byte
				}
			case runes[i] == ']':
				// OSC sequence: ESC ] ... ST or BEL
				i++
				for i < len(runes) {
					if runes[i] == '\x07' { // BEL
						i++
						break
					}
					if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '\\' { // ST
						i += 2
						break
					}
					i++
				}
			default:
				// Other ESC sequences (e.g., ESC ( B for charset):
				// skip intermediate bytes (0x20-0x2F) then final byte (0x30-0x7E)
				for i < len(runes) && runes[i] >= 0x20 && runes[i] <= 0x2F {
					i++
				}
				if i < len(runes) {
					i++ // skip final byte
				}
			}
		} else {
			result = append(result, runes[i])
			i++
		}
	}
	return result
}

// BufferToString converts the cell buffer to an ANSI-colored string.
// Uses targeted SGR sequences instead of full resets to prevent terminal
// default colors from bleeding through (fixes Termux blue background issue).
//
// Optimization: pre-computes uint32 color keys once per cell to avoid
// redundant RGBA() interface calls. Cache lookups use RWMutex for
// read-heavy access and evict-half on overflow.
func BufferToString(buf *Buffer) string {
	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(buf.Width * buf.Height * 50) // ~180KB for 120x30: each cell needs ~45 bytes for full ANSI color

	var prevFgKey, prevBgKey uint32
	var prevAttrs uint8
	firstCell := true

	for y := 0; y < buf.Height; y++ {
		x := 0
		for x < buf.Width {
			cell := buf.Cells[y][x]

			// Pre-compute color keys once — avoids redundant RGBA() calls
			// in both comparison and cache lookup paths.
			fgKey := colorKey(cell.Fg)
			bgKey := colorKey(cell.Bg)

			fgChanged := firstCell || fgKey != prevFgKey
			bgChanged := firstCell || bgKey != prevBgKey
			attrsChanged := firstCell || cell.Attrs != prevAttrs

			if fgChanged || bgChanged || attrsChanged {
				if attrsChanged {
					// Attrs changed — must reset, then re-emit attrs + both colors
					// in a combined SGR to avoid momentary terminal default flash.
					key := styleKey{fg: fgKey, bg: bgKey, attrs: cell.Attrs}
					sb.WriteString(sgrForStyleKey(key, cell.Fg, cell.Bg))
				} else {
					// Only colors changed — use targeted sequences (no reset)
					if fgChanged {
						if fgKey == colorNilKey {
							// nil fg — no explicit escape needed
						} else {
							sb.WriteString(sgrForFgKey(fgKey, cell.Fg))
						}
					}
					if bgChanged {
						if bgKey == colorNilKey {
							// nil bg — no explicit escape needed
						} else {
							sb.WriteString(sgrForBgKey(bgKey, cell.Bg))
						}
					}
				}
				prevFgKey = fgKey
				prevBgKey = bgKey
				prevAttrs = cell.Attrs
				firstCell = false
			}

			if cell.Content != "" {
				sb.WriteString(cell.Content)
			} else {
				sb.WriteRune(cell.Char)
			}

			// Advance by the larger of cell.Width and the actual host-terminal
			// display width. The VT emulator may underreport width for emoji.
			advance := int(cell.Width)
			if advance < 1 {
				advance = 1
			}
			var displayW int
			if cell.Content != "" {
				displayW = runewidth.StringWidth(cell.Content)
			} else {
				displayW = runewidth.RuneWidth(cell.Char)
			}
			if displayW > advance {
				advance = displayW
			}
			// When the host terminal renders emoji-capable characters as 2 cells
			// (detected at startup via TERMDESK_EMOJI_WIDTH probe), compensate
			// for the VT emulator reporting Width=1 due to PTY chunk splitting
			// (VS16 arriving in a separate chunk → lost by the emulator).
			// Ranges: Misc Symbols U+2600-U+26FF, Dingbats U+2700-U+27BF,
			// Misc Symbols U+2B50-U+2B55.
			if hostEmojiWide && advance == 1 && cell.Width <= 1 {
				r := cell.Char
				if (r >= 0x2600 && r <= 0x27BF) || (r >= 0x2B50 && r <= 0x2B55) {
					advance = 2
				}
			}
			x += advance
		}
		if y < buf.Height-1 {
			// No reset at end of line — colors carry over to avoid
			// terminal default bleeding through on line transitions.
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\x1b[0m") // final reset
	result := sb.String()
	builderPool.Put(sb)
	return result
}
