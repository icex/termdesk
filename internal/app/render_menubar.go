package app

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/widget"

	"github.com/mattn/go-runewidth"
)


// RenderMenuBar draws the menu bar at the top of the buffer.
// hoverWidget is the name of the widget currently hovered by the mouse (empty=none).
// hoverMenuLabel is the index of the menu label hovered by mouse (-1=none).
func RenderMenuBar(buf *Buffer, mb *menubar.MenuBar, theme config.Theme, mode InputMode, prefixPending bool, hoverWidget string, hoverMenuLabel int) {
	if mb == nil || buf.Height < 1 {
		return
	}

	c := theme.C()

	// Fill menu bar row with menu bar background
	mbFg := c.MenuBarFg
	mbBg := c.MenuBarBg
	if mbFg == nil {
		mbFg = c.ActiveTitleFg
	}
	if mbBg == nil {
		mbBg = c.ActiveTitleBg
	}
	for x := 0; x < buf.Width; x++ {
		buf.SetCell(x, 0, ' ', mbFg, mbBg, AttrBold)
	}

	// Compute mode badge (placed at far right, fixed width)
	var modeLabel string
	var modeFC, modeBC color.Color
	if prefixPending {
		modeLabel = " \uf11c PREFIX   "
		modeFC = c.ButtonFg
		modeBC = c.ButtonNoBg // danger color for prefix mode
	} else {
		modeLabel = modeBadge(mode)
		switch mode {
		case ModeTerminal:
			modeFC = c.ButtonFg
			modeBC = c.ButtonYesBg // green/positive for terminal
		case ModeCopy:
			modeFC = c.AccentFg
			modeBC = c.AccentColor // accent for copy mode
		default:
			modeFC = c.AccentFg
			modeBC = c.AccentColor // accent for normal mode
		}
	}
	modeLabelLen := widget.DisplayWidth(modeLabel)

	// Render bar text with reduced width (leave room for space + mode badge)
	effectiveWidth := buf.Width - modeLabelLen - 1
	if effectiveWidth < 1 {
		effectiveWidth = 1
	}
	barText := mb.Render(effectiveWidth)
	col := 0
	for _, ch := range barText {
		if col >= buf.Width {
			break
		}
		rw := runewidth.RuneWidth(ch)
		if rw >= 2 {
			// Wide character: set width on primary cell, continuation on next
			buf.Cells[0][col] = Cell{Char: ch, Fg: mbFg, Bg: mbBg, Width: int8(rw), Attrs: AttrBold}
			if col+1 < buf.Width {
				buf.Cells[0][col+1] = Cell{Char: ' ', Fg: mbFg, Bg: mbBg, Width: 0, Attrs: AttrBold}
			}
		} else {
			buf.SetCell(col, 0, ch, mbFg, mbBg, AttrBold)
		}
		col += rw
	}

	// Highlight open, Tab-focused, or hovered menu label.
	// Compute menu positions once for both highlight and dropdown rendering.
	var menuPositions []int
	highlightIdx := -1
	if mb.IsOpen() {
		highlightIdx = mb.OpenIndex
	} else if mb.FocusIndex >= 0 && mb.FocusIndex < len(mb.Menus) {
		highlightIdx = mb.FocusIndex
	}
	// Apply hover highlight (subtle) or active highlight (bold accent)
	if highlightIdx >= 0 || hoverMenuLabel >= 0 {
		menuPositions = mb.MenuXPositions()
	}
	if highlightIdx >= 0 {
		pos := menuPositions[highlightIdx]
		labelW := len([]rune(mb.Menus[highlightIdx].Label)) + 2
		hlStart := pos
		if pos > 0 {
			hlStart = pos - 1
		}
		for x := hlStart; x < hlStart+labelW && x < buf.Width; x++ {
			buf.Cells[0][x].Bg = c.AccentColor
			buf.Cells[0][x].Fg = c.AccentFg
			buf.Cells[0][x].Attrs = AttrBold
		}
	}
	if hoverMenuLabel >= 0 && hoverMenuLabel != highlightIdx && hoverMenuLabel < len(mb.Menus) {
		pos := menuPositions[hoverMenuLabel]
		labelW := len([]rune(mb.Menus[hoverMenuLabel].Label)) + 2
		hlStart := pos
		if pos > 0 {
			hlStart = pos - 1
		}
		for x := hlStart; x < hlStart+labelW && x < buf.Width; x++ {
			buf.Cells[0][x].Fg = c.AccentFg
			buf.Cells[0][x].Bg = c.AccentColor
		}
	}

	// Colorize right-side widget indicators using widget bar color levels
	lightTheme := theme.IsLight()
	if mb.WidgetBar != nil {
		zones := mb.RightZones(effectiveWidth)
		for _, zone := range zones {
			isHovered := hoverWidget != "" && zone.Type == hoverWidget
			var zoneColor color.Color
			for _, w := range mb.WidgetBar.Widgets {
				if w.Name() == zone.Type {
					zoneColor = levelColorC(w.ColorLevel(), lightTheme)
					break
				}
			}
			if isHovered {
				// Hover effect: accent background covers full zone (including padding)
				for x := zone.Start; x < zone.End && x < buf.Width; x++ {
					if x >= 0 && x < buf.Width {
						buf.Cells[0][x].Bg = c.AccentColor
						buf.Cells[0][x].Fg = c.AccentFg
					}
				}
			} else if zoneColor != nil {
				// Level color only on inner text (skip 1-space padding on each side)
				for x := zone.Start + 1; x < zone.End-1 && x < buf.Width; x++ {
					if x >= 0 && x < buf.Width {
						buf.Cells[0][x].Fg = zoneColor
					}
				}
			}
		}
		// Colorize separator characters (│) between widget zones
		if c.SubtleFg != nil && len(zones) > 1 {
			for i := 0; i < len(zones)-1; i++ {
				sepStart := zones[i].End
				sepEnd := zones[i+1].Start
				for x := sepStart; x < sepEnd && x < buf.Width; x++ {
					if x >= 0 && x < buf.Width {
						buf.Cells[0][x].Fg = c.SubtleFg
					}
				}
			}
		}
	}

	// Mode badge at the far right (1 space gap after widgets)
	modeX := buf.Width - modeLabelLen
	if modeX < 0 {
		modeX = 0
	}
	col = 0
	for _, ch := range modeLabel {
		cx := modeX + col
		buf.SetCell(cx, 0, ch, modeFC, modeBC, 0)
		col += runewidth.RuneWidth(ch)
	}

	// Render dropdown if open (reuses menuPositions computed above)
	if mb.IsOpen() {
		dropX := menuPositions[mb.OpenIndex]
		// Use lipgloss-styled dropdown
		borderFg := theme.ActiveBorderFg
		ddBg := theme.ActiveBorderBg
		ddFg := theme.MenuBarFg
		hoverFg := theme.AccentFg
		hoverBg := theme.AccentColor
		shortcutFg := theme.SubtleFg
		styled := mb.RenderDropdownStyled(borderFg, ddBg, hoverFg, hoverBg, ddFg, shortcutFg)
		if styled != "" {
			w := lipgloss.Width(styled)
			h := lipgloss.Height(styled)
			stampANSI(buf, dropX, 1, styled, w, h)
		}
		// Render submenu if the hovered item has SubItems
		if mb.HasSubMenu() {
			subStyled := mb.RenderSubMenuStyled(borderFg, ddBg, hoverFg, hoverBg, ddFg, shortcutFg)
			if subStyled != "" {
				parentItemY, parentWidth := mb.SubMenuParentIndex()
				subX := dropX + parentWidth
				subY := 1 + parentItemY + 1 // 1 for menubar row, +1 for border
				sw := lipgloss.Width(subStyled)
				sh := lipgloss.Height(subStyled)
				// Clamp to screen bounds
				if subX+sw > buf.Width {
					subX = dropX - sw // flip to left side
				}
				stampANSI(buf, subX, subY, subStyled, sw, sh)
			}
		}
	}
}

// levelColor maps a color level name to a hex color (used by overlays).
// "green" returns empty (use default fg). light selects darker variants for light themes.
func levelColor(level string, light bool) string {
	if light {
		switch level {
		case "red":
			return "#C03030"
		case "yellow":
			return "#9A7B00"
		default:
			return ""
		}
	}
	switch level {
	case "red":
		return "#E06C75"
	case "yellow":
		return "#E5C07B"
	default:
		return ""
	}
}

// Pre-parsed level colors for dark themes.
var (
	levelRed    = color.RGBA{R: 224, G: 108, B: 117, A: 255} // #E06C75
	levelYellow = color.RGBA{R: 229, G: 192, B: 123, A: 255} // #E5C07B
	defaultViewFg color.Color = color.RGBA{R: 192, G: 192, B: 192, A: 255} // #C0C0C0
)

// Pre-parsed level colors for light themes (darker for contrast).
var (
	levelRedLight    = color.RGBA{R: 192, G: 48, B: 48, A: 255}  // #C03030
	levelYellowLight = color.RGBA{R: 154, G: 123, B: 0, A: 255}  // #9A7B00
)

// levelColorC maps a color level name to a pre-parsed color.Color.
// "green" returns nil (use default fg). light selects darker variants for light themes.
func levelColorC(level string, light bool) color.Color {
	if light {
		switch level {
		case "red":
			return levelRedLight
		case "yellow":
			return levelYellowLight
		default:
			return nil
		}
	}
	switch level {
	case "red":
		return levelRed
	case "yellow":
		return levelYellow
	default:
		return nil
	}
}

// modeIcon returns a Nerd Font icon for the input mode.
func modeIcon(mode InputMode) string {
	switch mode {
	case ModeTerminal:
		return "\uf120" //  terminal
	case ModeCopy:
		return "\uf0c5" //  copy
	default:
		return "\uf009" //  grid (window management)
	}
}

// modeBadge returns a fixed-width mode badge string.
// All modes produce the same display width so switching modes
// doesn't shift the clock/CPU/MEM indicators.
func modeBadge(mode InputMode) string {
	icon := modeIcon(mode)
	name := mode.String()
	// Pad name to 8 chars ("TERMINAL" is the longest)
	for len(name) < 8 {
		name += " "
	}
	return " " + icon + " " + name + " "
}
