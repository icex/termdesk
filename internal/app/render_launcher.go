package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/launcher"
)

// RenderLauncher draws the launcher overlay centered on the buffer.
func RenderLauncher(buf *Buffer, l *launcher.Launcher, theme config.Theme) {
	if l == nil || !l.Visible {
		return
	}

	boxW := 56
	if boxW > buf.Width-4 {
		boxW = buf.Width - 4
	}
	if boxW < 12 {
		return // too narrow to render
	}
	innerW := boxW
	hPad := 2

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
	titleStr := titleStyle.Render("Launcher")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW-2*hPad))

	// Search bar
	cursor := "█"
	queryDisplay := l.Query + cursor
	runes := []rune(queryDisplay)
	textW := innerW - hPad - 4 // account for padding + prompt icon
	if len(runes) > textW {
		runes = runes[len(runes)-textW:]
	}
	searchStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		PaddingLeft(hPad)
	searchStr := searchStyle.Render(" " + string(runes))

	// Results
	maxResults := min(8, len(l.Results))
	textResultW := innerW - hPad - 2
	var resultLines []string
	if maxResults == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(subtleFg).
			Background(bgColor).
			Width(innerW).
			PaddingLeft(hPad).
			Italic(true)
		resultLines = append(resultLines, emptyStyle.Render("No results"))
	}
	for i := 0; i < maxResults; i++ {
		entry := l.Results[i]
		icon := entry.Icon
		if icon == "" {
			icon = "\uf120"
		}
		star := ""
		if l.Favorites[entry.Command] {
			star = "★ "
		}
		label := star + icon + " " + entry.Name
		rn := []rune(label)
		if len(rn) > textResultW {
			label = string(rn[:textResultW-1]) + "…"
		}

		var lineStyle lipgloss.Style
		if i == l.SelectedIdx {
			lineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentFg).
				Background(accentColor).
				Width(innerW).
				PaddingLeft(hPad)
			label = "▸ " + label
		} else {
			lineStyle = lipgloss.NewStyle().
				Foreground(fgColor).
				Background(bgColor).
				Width(innerW).
				PaddingLeft(hPad)
			label = "  " + label
		}
		resultLines = append(resultLines, lineStyle.Render(label))
	}
	contentStr := strings.Join(resultLines, "\n")

	// Suggestions
	var suggStr string
	if len(l.Suggestions) > 0 {
		suggText := "Suggestions: " + strings.Join(l.Suggestions, ", ")
		rn := []rune(suggText)
		if len(rn) > innerW-2*hPad {
			suggText = string(rn[:innerW-2*hPad-1]) + "…"
		}
		suggStyle := lipgloss.NewStyle().
			Foreground(subtleFg).
			Background(bgColor).
			Width(innerW).
			PaddingLeft(hPad).
			Italic(true)
		suggStr = suggStyle.Render(suggText)
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerStr := footerStyle.Render("Tab complete │ ↑↓ navigate │ Ctrl+P pin │ Enter launch")

	// Compose
	parts := []string{spacer(), titleStr, sepStr, searchStr, sepStr, spacer(), contentStr}
	if suggStr != "" {
		parts = append(parts, sepStyle.Render(strings.Repeat("─", innerW-2*hPad)), suggStr)
	}
	parts = append(parts, spacer(), sepStyle.Render(strings.Repeat("─", innerW-2*hPad)), footerStr, spacer())
	inner := strings.Join(parts, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	startX := (buf.Width - w) / 2
	startY := (buf.Height - h) / 3 // slightly above center
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}
