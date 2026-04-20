package app

import (
"fmt"
"strings"

"github.com/charmbracelet/lipgloss"
"github.com/icex/termdesk/internal/config"
"github.com/icex/termdesk/internal/notification"
)

// RenderToasts draws toast notifications in the top-right corner of the buffer.
func RenderToasts(buf *Buffer, notifs *notification.Manager, theme config.Theme) {
	toasts := notifs.VisibleToasts()
	if len(toasts) == 0 {
		return
	}

	toastW := 50 // Increased from 42 to fit longer messages
	if toastW > buf.Width-4 {
		toastW = buf.Width - 4
	}

	fgColor := lipgloss.Color(theme.MenuBarFg)
	bgColor := lipgloss.Color(theme.ActiveBorderBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	y := 2 // start below menu bar
	for _, toast := range toasts {
		// Severity icon + color
		var icon string
		var iconColor lipgloss.Color
		switch toast.Severity {
		case notification.Info:
			icon = "ℹ"
			iconColor = lipgloss.Color(theme.AccentColor)
		case notification.Warning:
			icon = "⚠"
			iconColor = lipgloss.Color("#E5C07B")
		case notification.Error:
			icon = "✗"
			iconColor = lipgloss.Color("#E06C75")
		}

		iconStyle := lipgloss.NewStyle().
			Foreground(iconColor).
			Background(bgColor).
			Bold(true)

		titleStyle := lipgloss.NewStyle().
			Foreground(fgColor).
			Background(bgColor).
			Bold(true)

		bodyStyle := lipgloss.NewStyle().
			Foreground(subtleFg).
			Background(bgColor)

		// Use full body text - lipgloss will handle wrapping via Width on padding
		body := toast.Body

		// Title line
		titleLine := iconStyle.Render(icon+" ") + titleStyle.Render(toast.Title)
		// Body line
		bodyLine := bodyStyle.Render("  " + body)

		innerW := toastW - 2
		titlePad := lipgloss.NewStyle().
			Background(bgColor).
			Width(innerW)
		bodyPad := lipgloss.NewStyle().
			Background(bgColor).
			Width(innerW)

		inner := titlePad.Render(titleLine) + "\n" + bodyPad.Render(bodyLine)

		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			BorderBackground(bgColor)
		rendered := boxStyle.Render(inner)

		w := lipgloss.Width(rendered)
		h := lipgloss.Height(rendered)
		startX := buf.Width - w - 1
		if startX < 0 {
			startX = 0
		}
		stampANSI(buf, startX, y, rendered, w, h)
		y += h + 1
	}
}

// RenderNotificationCenter draws the notification center as a right-side panel.
func RenderNotificationCenter(buf *Buffer, notifs *notification.Manager, theme config.Theme) {
	if !notifs.CenterVisible() {
		return
	}

	items := notifs.HistoryItems()
	panelW := 45
	if panelW > buf.Width-4 {
		panelW = buf.Width - 4
	}
	hPad := 2
	innerW := panelW

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
	titleStr := titleStyle.Render("Notifications")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW-2*hPad))

	// Items
	textW := innerW - hPad - 4 // leave room for icon + padding
	maxItems := buf.Height - 8  // fit in available height
	if maxItems < 3 {
		maxItems = 3
	}

	var itemLines []string
	if len(items) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(subtleFg).
			Background(bgColor).
			Width(innerW).
			PaddingLeft(hPad).
			Italic(true)
		itemLines = append(itemLines, emptyStyle.Render("No notifications"))
	} else {
		visibleItems := items
		if len(visibleItems) > maxItems {
			visibleItems = visibleItems[:maxItems]
		}
		for i, item := range visibleItems {
			// Severity icon
			var icon string
			switch item.Severity {
			case notification.Info:
				icon = "ℹ"
			case notification.Warning:
				icon = "⚠"
			case notification.Error:
				icon = "✗"
			}

			// Title + body preview
			preview := item.Title
			if item.Body != "" {
				preview += ": " + item.Body
			}
			preview = strings.ReplaceAll(preview, "\n", "↵")
			runes := []rune(preview)
			if len(runes) > textW {
				preview = string(runes[:textW-1]) + "…"
			}

			readMark := " "
			if !item.Read {
				readMark = "●"
			}

			var lineStyle lipgloss.Style
			if i == notifs.CenterIndex() {
				lineStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(accentFg).
					Background(accentColor).
					Width(innerW).
					PaddingLeft(hPad)
			} else {
				lineStyle = lipgloss.NewStyle().
					Foreground(fgColor).
					Background(bgColor).
					Width(innerW).
					PaddingLeft(hPad)
			}
			label := readMark + " " + icon + " " + preview
			itemLines = append(itemLines, lineStyle.Render(label))
		}
		if len(items) > maxItems {
			moreStyle := lipgloss.NewStyle().
				Foreground(subtleFg).
				Background(bgColor).
				Width(innerW).
				PaddingLeft(hPad)
			itemLines = append(itemLines, moreStyle.Render(fmt.Sprintf("  ... +%d more", len(items)-maxItems)))
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
	footerStr := footerStyle.Render("↑↓ nav │ d del │ D clear │ Esc close")

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
	startX := buf.Width - w - 1
	startY := (buf.Height - h) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	stampANSI(buf, startX, startY, rendered, w, h)
}
