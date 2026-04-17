package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
)

// RenderShowKeys draws the recent key press overlay in the bottom-left corner.
func RenderShowKeys(buf *Buffer, events []showKeyEvent, theme config.Theme) {
	if len(events) == 0 {
		return
	}

	maxW := 44
	if maxW > buf.Width-4 {
		maxW = buf.Width - 4
	}
	if maxW < 20 {
		maxW = 20
	}
	innerW := maxW

	bgColor := lipgloss.Color(theme.ActiveBorderBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	accentColor := lipgloss.Color(theme.AccentColor)
	accentFg := lipgloss.Color(theme.AccentFg)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor).
		Width(innerW).
		Align(lipgloss.Center)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor)

	lineStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW)

	spacer := lipgloss.NewStyle().Background(bgColor).Width(innerW).Render("")

	// Render newest first
	var lines []string
	for i := len(events) - 1; i >= 0; i-- {
		evt := events[i]
		keyText := trimRunes(evt.Key, 12)
		keyTag := keyStyle.Render(" " + keyText + " ")
		var line string
		if evt.Action != "" {
			line = keyTag + " " + evt.Action
		} else {
			line = keyTag
		}
		lines = append(lines, lineStyle.Render(line))
		if len(lines) >= showKeysMaxEvents {
			break
		}
	}

	inner := strings.Join(append([]string{spacer, titleStyle.Render("Show Keys"), spacer}, lines...), "\n")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	startX := 1
	startY := buf.Height - h - 2
	if startY < 1 {
		startY = 1
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max == 1 {
		return "…"
	}
	return fmt.Sprintf("%s…", string(r[:max-1]))
}
