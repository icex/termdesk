package app

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
)

// RenderTooltip renders a small tooltip box near the hover position.
func RenderTooltip(buf *Buffer, text string, x, y int, theme config.Theme) {
	if text == "" {
		return
	}

	fg := lipgloss.Color(theme.MenuBarFg)
	bg := lipgloss.Color(theme.ActiveBorderBg)

	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Padding(0, 1)

	box := style.Render(text)
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)

	// Position tooltip below and to the right of cursor
	tx := x + 1
	ty := y + 1
	// Clamp to buffer
	if tx+boxW > buf.Width {
		tx = buf.Width - boxW
	}
	if ty+boxH > buf.Height-1 { // don't overlap dock
		ty = y - boxH
	}
	if tx < 0 {
		tx = 0
	}
	if ty < 0 {
		ty = 0
	}

	stampANSI(buf, tx, ty, box, boxW, boxH)
}
