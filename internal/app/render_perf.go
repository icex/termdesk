package app

import (
	"fmt"
	"image/color"

	"github.com/icex/termdesk/internal/config"
)

// renderPerfOverlay draws the performance statistics overlay in the top-right corner.
func renderPerfOverlay(buf *Buffer, pt *perfTracker, theme config.Theme) {
	if pt == nil || !pt.enabled {
		return
	}

	lines := []string{
		fmt.Sprintf(" FPS: %.1f ", pt.fps),
		fmt.Sprintf(" View:   %s ", fmtDur(pt.lastViewTime)),
		fmt.Sprintf(" Update: %s ", fmtDur(pt.lastUpdateTime)),
		fmt.Sprintf(" Frame:  %s ", fmtDur(pt.lastFrameTime)),
		fmt.Sprintf(" ANSI:   %s ", fmtDur(pt.lastANSITime)),
		fmt.Sprintf(" Output: %dK ", pt.lastANSIBytes/1024),
		fmt.Sprintf(" Wins: %d H:%d M:%d ", pt.lastWindowCount, pt.lastCacheHits, pt.lastCacheMisses),
	}

	// Find max width
	maxW := 0
	for _, l := range lines {
		w := len([]rune(l))
		if w > maxW {
			maxW = w
		}
	}

	// Position: top-right corner, below menu bar
	panelH := len(lines) + 2 // +2 for top/bottom border
	panelW := maxW + 2        // +2 for side borders
	startX := buf.Width - panelW - 1
	startY := 1 // below menu bar

	if startX < 0 {
		startX = 0
	}

	// Colors
	fg := color.RGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF} // green text
	bg := color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xFF} // dark bg
	borderFg := color.RGBA{R: 0x40, G: 0x80, B: 0x40, A: 0xFF}
	_ = theme // available for future theming

	// Top border
	y := startY
	if y < buf.Height {
		buf.SetCell(startX, y, '┌', borderFg, bg, 0)
		for x := startX + 1; x < startX+panelW-1 && x < buf.Width; x++ {
			buf.SetCell(x, y, '─', borderFg, bg, 0)
		}
		if startX+panelW-1 < buf.Width {
			buf.SetCell(startX+panelW-1, y, '┐', borderFg, bg, 0)
		}
	}

	// Content lines
	for i, line := range lines {
		y = startY + 1 + i
		if y >= buf.Height {
			break
		}
		buf.SetCell(startX, y, '│', borderFg, bg, 0)
		runes := []rune(line)
		for j := 0; j < maxW; j++ {
			x := startX + 1 + j
			if x >= buf.Width {
				break
			}
			ch := ' '
			if j < len(runes) {
				ch = runes[j]
			}
			buf.SetCell(x, y, ch, fg, bg, 0)
		}
		if startX+panelW-1 < buf.Width {
			buf.SetCell(startX+panelW-1, y, '│', borderFg, bg, 0)
		}
	}

	// Bottom border
	y = startY + panelH - 1
	if y < buf.Height {
		buf.SetCell(startX, y, '└', borderFg, bg, 0)
		for x := startX + 1; x < startX+panelW-1 && x < buf.Width; x++ {
			buf.SetCell(x, y, '─', borderFg, bg, 0)
		}
		if startX+panelW-1 < buf.Width {
			buf.SetCell(startX+panelW-1, y, '┘', borderFg, bg, 0)
		}
	}
}
