package app

import (
"strings"

"github.com/charmbracelet/lipgloss"
"github.com/icex/termdesk/internal/clipboard"
"github.com/icex/termdesk/internal/config"
)

// RenderClipboardHistory draws the clipboard history overlay centered on the buffer.
func RenderClipboardHistory(buf *Buffer, clip *clipboard.Clipboard, theme config.Theme) {
	if clip == nil || !clip.Visible {
		return
	}

	items := clip.HistoryItems()

	// Dimensions
	maxW := 50
	if maxW > buf.Width-6 {
		maxW = buf.Width - 6
	}
	hPad := 3
	innerW := maxW

	// Theme colors
	fgColor := lipgloss.Color(theme.MenuBarFg)
	bgColor := lipgloss.Color(theme.ActiveBorderBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	accentColor := lipgloss.Color(theme.AccentColor)
	accentFg := lipgloss.Color(theme.AccentFg)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	spacer := func() string {
		return lipgloss.NewStyle().Background(bgColor).Width(innerW).Render("")
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render("Clipboard History")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW-2*hPad))

	// Items
	textW := innerW - hPad
	var itemLines []string
	if len(items) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(subtleFg).
			Background(bgColor).
			Width(innerW).
			PaddingLeft(hPad).
			Italic(true)
		itemLines = append(itemLines, emptyStyle.Render("No clipboard entries"))
	} else {
		for i, item := range items {
			preview := strings.ReplaceAll(item, "\n", "↵")
			runes := []rune(preview)
			if len(runes) > textW-4 {
				preview = string(runes[:textW-5]) + "…"
			}

			var lineStyle lipgloss.Style
			if i == clip.SelectedIdx {
				lineStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(accentFg).
					Background(accentColor).
					Width(innerW).
					PaddingLeft(hPad)
				preview = "▸ " + preview
			} else {
				lineStyle = lipgloss.NewStyle().
					Foreground(fgColor).
					Background(bgColor).
					Width(innerW).
					PaddingLeft(hPad)
				preview = "  " + preview
			}
			itemLines = append(itemLines, lineStyle.Render(preview))
		}
	}
	contentStr := strings.Join(itemLines, "\n")

	// Footer
	footerSepStr := sepStyle.Render(strings.Repeat("─", innerW-2*hPad))
	footerStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerStr := footerStyle.Render("↑↓ navigate │ Enter paste │ v view │ Esc close")

	// Compose
	parts := []string{spacer(), titleStr, sepStr, spacer(), contentStr, spacer(), footerSepStr, footerStr, spacer()}
	inner := strings.Join(parts, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	startX := (buf.Width - w) / 2
	startY := (buf.Height - h) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}
