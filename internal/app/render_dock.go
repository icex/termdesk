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

	// Locate the item span (first/last non-Padding cell) — used for pill caps
	// and to skip the side fill when Pill mode is on.
	firstX, lastX := -1, -1
	for x, cell := range cells {
		if cell.Char == 0 || cell.Padding {
			continue
		}
		if firstX == -1 {
			firstX = x
		}
		lastX = x
	}

	if !d.Pill {
		// Classic full-width bar: paint the dock row with base dock colors.
		for x := 0; x < buf.Width; x++ {
			buf.SetCell(x, y, ' ', c.DockFg, c.DockBg, 0)
		}
	}

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
			// Hovered/selected: force theme's accent pair for guaranteed contrast
			// (icon colors can blend into AccentColor — e.g. a blue-toned icon on
			// catppuccin's blue accent) and bold for extra visual weight.
			bg = c.AccentColor
			fg = c.AccentFg
			attrs |= AttrBold
		}
		if cell.HasActivity && !cell.Accent {
			// Activity spinner uses AccentColor, which collides with the hover
			// background when Accent is also set — skip here so the AccentFg
			// chosen above stays visible on the AccentColor background.
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
		if d.Pill && cell.Padding {
			// Let the wallpaper show through outside the pill span.
			continue
		}
		fg, bg, attrs := styleDockCell(cell)
		buf.SetCell(x, y, cell.Char, fg, bg, attrs)
	}

	// Pill caps — rounded ends on the outside of the item span. fg is the
	// filled half (matches DockBg), bg is the transparent side (wallpaper bg).
	if d.Pill && firstX >= 0 && lastX >= firstX {
		capBg := c.DesktopBg
		if capBg == nil {
			capBg = color.Black
		}
		// '\ue0b6' = left half circle, '\ue0b4' = right half circle (Powerline).
		if firstX-1 >= 0 && firstX-1 < buf.Width {
			buf.SetCell(firstX-1, y, '\ue0b6', c.DockBg, capBg, 0)
		}
		if lastX+1 >= 0 && lastX+1 < buf.Width {
			buf.SetCell(lastX+1, y, '\ue0b4', c.DockBg, capBg, 0)
		}
	}
}
