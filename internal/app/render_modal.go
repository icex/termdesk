package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
)

// RenderModal draws a scrollable modal overlay centered on the buffer.
// Supports tabbed modals (when modal.Tabs is non-nil).
func RenderModal(buf *Buffer, modal *ModalOverlay, theme config.Theme) {
	if modal == nil {
		return
	}

	// Resolve which lines to display (tabs or plain)
	lines := modal.Lines
	hasTabBar := modal.Tabs != nil && len(modal.Tabs) > 0
	if hasTabBar {
		if modal.ActiveTab >= len(modal.Tabs) {
			modal.ActiveTab = 0
		}
		lines = modal.Tabs[modal.ActiveTab].Lines
	}

	// Calculate stable dimensions across all tabs
	maxLineW := runeLen(modal.Title)
	if hasTabBar {
		// Ensure width fits all tab labels on one line
		tabBarW := 0
		for i := range modal.Tabs {
			tabBarW += runeLen(modal.TabLabel(i)) + 2 // label + padding
		}
		if tabBarW > maxLineW {
			maxLineW = tabBarW
		}
		for _, tab := range modal.Tabs {
			for _, line := range tab.Lines {
				if w := runeLen(line); w > maxLineW {
					maxLineW = w
				}
			}
		}
	} else {
		for _, line := range lines {
			if w := runeLen(line); w > maxLineW {
				maxLineW = w
			}
		}
	}
	hPad := 3 // horizontal padding each side
	innerW := maxLineW + 2*hPad
	if innerW > buf.Width-6 {
		innerW = buf.Width - 6
	}

	maxTabLines := len(lines)
	if hasTabBar {
		for _, tab := range modal.Tabs {
			if len(tab.Lines) > maxTabLines {
				maxTabLines = len(tab.Lines)
			}
		}
	}

	visibleLines := buf.Height - 14 // room for padding + spacers
	if visibleLines < 3 {
		visibleLines = 3
	}
	if visibleLines > maxTabLines {
		visibleLines = maxTabLines
	}

	// Clamp scroll
	maxScroll := len(lines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if modal.ScrollY > maxScroll {
		modal.ScrollY = maxScroll
	}

	// Theme colors for lipgloss
	fgColor := lipgloss.Color(theme.MenuBarFg)
	bgColor := lipgloss.Color(theme.ActiveBorderBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)

	accentColor := lipgloss.Color(theme.AccentColor)
	accentFg := lipgloss.Color(theme.AccentFg)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	// Spacer helper
	spacer := func() string {
		return lipgloss.NewStyle().Background(bgColor).Width(innerW).Render("")
	}

	// Title — centered, bold, accent background
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(modal.Title)

	// Separator — subtle
	sepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW-2*hPad))

	// Tab bar (if tabbed)
	tabBarStr := ""
	if hasTabBar {
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
		for i := range modal.Tabs {
			label := modal.TabLabel(i)
			if i == modal.ActiveTab {
				tabs = append(tabs, activeTabStyle.Render(label))
			} else if i == modal.HoverTab {
				tabs = append(tabs, hoverTabStyle.Render(label))
			} else {
				tabs = append(tabs, inactiveTabStyle.Render(label))
			}
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
		tabRowStyle := lipgloss.NewStyle().
			Background(bgColor).
			Width(innerW).
			PaddingLeft(hPad)
		tabBarStr = tabRowStyle.Render(row)
	}

	// Content lines with left padding
	contentStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		PaddingLeft(hPad)

	textW := innerW - hPad // max text width after left padding
	var contentLines []string
	for i := 0; i < visibleLines; i++ {
		lineIdx := modal.ScrollY + i
		if lineIdx < len(lines) {
			line := lines[lineIdx]
			lineRunes := []rune(line)
			if len(lineRunes) > textW {
				line = string(lineRunes[:textW])
			}
			contentLines = append(contentLines, contentStyle.Render(line))
		} else {
			contentLines = append(contentLines, contentStyle.Render(""))
		}
	}
	contentStr := strings.Join(contentLines, "\n")

	// Footer separator + navigation hints
	footerSepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerSepStr := footerSepStyle.Render(strings.Repeat("─", innerW-2*hPad))

	footerText := ""
	if hasTabBar {
		footerText = fmt.Sprintf("Tab/1-%d navigate  \u2502  \u2191\u2193 scroll  \u2502  Esc close", len(modal.Tabs))
	} else {
		footerText = "\u2191\u2193 scroll  \u2502  Esc close"
	}
	footerStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	footerStr := footerStyle.Render(footerText)

	// Compose inner content with generous spacing
	parts := []string{spacer(), titleStr, sepStr}
	if tabBarStr != "" {
		parts = append(parts, spacer(), tabBarStr)
	}
	parts = append(parts, spacer(), contentStr, spacer(), footerSepStr, footerStr, spacer())
	inner := strings.Join(parts, "\n")

	// Wrap in rounded border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	// Stamp into buffer centered
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
