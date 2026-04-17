package app

import (
	"image/color"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
)

var stuckRed = color.RGBA{R: 220, G: 50, B: 50, A: 255}

// RenderDock draws the dock at the bottom of the buffer with per-cell accent coloring.
func RenderDock(buf *Buffer, d *dock.Dock, theme config.Theme, animations []Animation) {
	if d == nil || buf.Height < 2 {
		return
	}

	c := theme.C()
	lightTheme := theme.IsLight()
	y := buf.Height - 1

	// Fill dock row with base dock colors
	for x := 0; x < buf.Width; x++ {
		buf.SetCell(x, y, ' ', c.DockFg, c.DockBg, 0)
	}

	// Determine effective hover for rendering (never mutate state in View)
	// Scan animations directly instead of building a map.
	effectiveHover := d.HoverIndex
	if effectiveHover < 0 {
		for _, a := range animations {
			if a.Type == AnimDockPulse && !a.Done {
				effectiveHover = a.DockIndex
				break
			}
		}
	}

	// Render dock cells with per-cell styling
	cells := d.RenderCellsWithHover(buf.Width, effectiveHover)

	// styleDockCell computes fg, bg, attrs for a dock cell.
	styleDockCell := func(cell dock.DockCell) (color.Color, color.Color, uint8) {
		fg := c.DockFg
		bg := c.DockBg
		attrs := uint8(0)
		if cell.IconColor != "" {
			fg = hexToColor(cell.IconColor)
			if lightTheme {
				fg = darkenColor(fg, 0.65)
			}
		}
		if cell.Separator {
			fg = c.SubtleFg
		}
		if cell.Minimized {
			fg = c.NotificationFg
			if fg == nil {
				fg = levelYellow
			}
		}
		if cell.Running {
			fg = c.DockFg
		}
		if cell.Active {
			bg = c.ActiveTitleBg
			if cell.IconColor == "" {
				fg = c.ActiveTitleFg
			}
			attrs = AttrBold
		}
		if cell.Accent {
			bg = c.AccentColor
			if cell.IconColor == "" {
				fg = c.AccentFg
			}
		}
		if cell.HasActivity {
			fg = c.AccentColor
		}
		if cell.HasBell {
			fg = levelYellow
		}
		if cell.Stuck {
			fg = stuckRed
		}
		return fg, bg, attrs
	}

	for x, cell := range cells {
		if cell.Char == 0 {
			continue
		}
		fg, bg, attrs := styleDockCell(cell)
		buf.SetCell(x, y, cell.Char, fg, bg, attrs)
	}
}
