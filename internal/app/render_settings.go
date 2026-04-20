package app

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/settings"
)

// RenderSettingsPanel draws the settings modal centered on the buffer.
func RenderSettingsPanel(buf *Buffer, panel *settings.Panel, theme config.Theme) {
	if panel == nil || !panel.Visible {
		return
	}

	innerW := panel.InnerWidth(buf.Width)

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
	titleStr := titleStyle.Render("Settings")

	// Tabs — with hover styling
	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor).
		Padding(0, 1)
	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Padding(0, 1)
	hoverTabStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(bgColor).
		Underline(true).
		Padding(0, 1)

	var tabs []string
	for i, sec := range panel.Sections {
		if i == panel.ActiveTab {
			tabs = append(tabs, activeTabStyle.Render(sec.Title))
		} else if i == panel.HoverTab {
			tabs = append(tabs, hoverTabStyle.Render(sec.Title))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(sec.Title))
		}
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	tabRowStyle := lipgloss.NewStyle().
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	tabStr := tabRowStyle.Render(tabRow)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW-4))

	// Items
	sec := panel.Sections[panel.ActiveTab]
	var itemLines []string
	for i, item := range sec.Items {
		var valStr string
		switch item.Type {
		case settings.TypeToggle:
			if item.BoolVal {
				valStr = "[✓]"
			} else {
				valStr = "[ ]"
			}
		case settings.TypeChoice:
			valStr = "◀ " + item.StrVal + " ▶"
		case settings.TypeText:
			isEditing := panel.TextEditing && i == panel.ActiveItem
			if isEditing {
				// Show edit buffer with cursor
				runes := []rune(panel.TextBuf)
				before := string(runes[:panel.TextCursor])
				cursor := "▏"
				after := ""
				if panel.TextCursor < len(runes) {
					after = string(runes[panel.TextCursor:])
				}
				valStr = "[ " + before + cursor + after + " ]"
			} else if len(item.Choices) > 0 {
				valStr = "◀ " + item.StrVal + " ▶ ✎"
			} else {
				valStr = item.StrVal + " ✎"
			}
		}

		label := item.Label
		// Pad label to align values on the right
		labelW := innerW - 8 - utf8.RuneCountInString(valStr)
		if labelW < 10 {
			labelW = 10
		}
		labelRunes := []rune(label)
		if len(labelRunes) > labelW {
			label = string(labelRunes[:labelW-1]) + "…"
		}
		for utf8.RuneCountInString(label) < labelW {
			label += " "
		}

		lineText := "  " + label + "  " + valStr

		var lineStyle lipgloss.Style
		if i == panel.ActiveItem {
			lineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentFg).
				Background(accentColor).
				Width(innerW)
		} else {
			lineStyle = lipgloss.NewStyle().
				Foreground(fgColor).
				Background(bgColor).
				Width(innerW)
		}
		itemLines = append(itemLines, lineStyle.Render(lineText))
	}
	contentStr := strings.Join(itemLines, "\n")

	// Footer
	footerSepStr := sepStyle.Render(strings.Repeat("─", innerW-4))
	footerStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerStr := footerStyle.Render("Tab/click sections │ ↑↓ nav │ Enter/click toggle │ ←→ cycle │ Esc close")

	// Compose
	parts := []string{spacer(), titleStr, spacer(), tabStr, sepStr, spacer(), contentStr, spacer(), footerSepStr, footerStr, spacer()}
	inner := strings.Join(parts, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	// Use Bounds() for position so click handling matches exactly.
	bounds := panel.Bounds(buf.Width, buf.Height)
	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)
	stampANSI(buf, bounds.X, bounds.Y, rendered, w, h)
}
