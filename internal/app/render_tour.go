package app

import (
"github.com/charmbracelet/lipgloss"
"github.com/icex/termdesk/internal/config"
"github.com/icex/termdesk/internal/tour"
)

// RenderTour renders the guided tour overlay as a centered modal.
func RenderTour(buf *Buffer, t *tour.Tour, theme config.Theme) {
	step := t.CurrentStep()
	if step == nil {
		return
	}

	accentBg := lipgloss.Color(theme.AccentColor)
	accentFg := lipgloss.Color(theme.AccentFg)
	fg := lipgloss.Color(theme.MenuBarFg)
	bg := lipgloss.Color(theme.ActiveBorderBg)
	subtle := lipgloss.Color(theme.SubtleFg)

	buttonLabel := "  Next  "
	if t.IsLast() {
		buttonLabel = " Finish "
	}

	contentStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Width(44).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(accentFg).
		Background(accentBg).
		Bold(true).
		Width(44).
		Align(lipgloss.Center).
		Padding(0, 1)

	footerStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Width(44).
		Align(lipgloss.Center)

	btnStyle := lipgloss.NewStyle().
		Foreground(accentFg).
		Background(accentBg).
		Bold(true)

	skipStyle := lipgloss.NewStyle().
		Foreground(subtle).
		Background(bg)

	title := titleStyle.Render(" Tour " + t.StepInfo() + " ")
	body := contentStyle.Render(step.Body)
	
	footer := footerStyle.Render(
		btnStyle.Render(buttonLabel) + styledSpacer(bg, "   ") + skipStyle.Render("Esc skip"),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentBg).
		BorderBackground(bg).
		Background(bg).
		Render(title + "\n" + body + "\n" + footer)

	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	x := (buf.Width - boxW) / 2
	y := (buf.Height - boxH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	stampANSI(buf, x, y, box, boxW, boxH)
}
