package app

import (
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
)

// renderSyncPanesIndicator highlights the top border of all synced windows with accent color.
func renderSyncPanesIndicator(buf *Buffer, wm *window.Manager, theme config.Theme) {
	c := theme.C()
	for _, w := range wm.Windows() {
		if !w.Visible || w.Minimized {
			continue
		}
		// Paint accent color on the top border row
		y := w.Rect.Y
		if y < 0 || y >= buf.Height {
			continue
		}
		for x := w.Rect.X; x < w.Rect.Right() && x < buf.Width; x++ {
			if x < 0 {
				continue
			}
			buf.Cells[y][x].Bg = c.AccentColor
			buf.Cells[y][x].Fg = c.AccentFg
		}
	}
}
