package app

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/contextmenu"
)

// RenderContextMenu draws a right-click context menu at the menu's position.
func RenderContextMenu(buf *Buffer, menu *contextmenu.Menu, theme config.Theme) {
	if menu == nil || !menu.Visible {
		return
	}

	fgColor := lipgloss.Color(theme.MenuBarFg)
	bgColor := lipgloss.Color(theme.ActiveBorderBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	accentColor := lipgloss.Color(theme.AccentColor)
	accentFg := lipgloss.Color(theme.AccentFg)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	innerW := menu.InnerWidth()

	normalStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(bgColor)

	hoverStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentFg).
		Background(accentColor)

	shortcutStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(bgColor)

	var lines []string
	for i, item := range menu.Items {
		if item.Disabled {
			sepStyle := lipgloss.NewStyle().
				Foreground(subtleFg).
				Background(bgColor).
				Width(innerW)
			lines = append(lines, sepStyle.Render(strings.Repeat("─", innerW)))
			continue
		}

		label := "  " + item.Label
		isHover := i == menu.HoverIndex

		if item.Shortcut != "" {
			gap := innerW - utf8.RuneCountInString(label) - utf8.RuneCountInString(item.Shortcut) - 1
			if gap < 1 {
				gap = 1
			}
			if isHover {
				content := label + strings.Repeat(" ", gap) + item.Shortcut
				for utf8.RuneCountInString(content) < innerW {
					content += " "
				}
				lines = append(lines, hoverStyle.Width(innerW).Render(content))
			} else {
				labelPart := label + strings.Repeat(" ", gap)
				scPad := innerW - utf8.RuneCountInString(labelPart) - utf8.RuneCountInString(item.Shortcut)
				if scPad < 0 {
					scPad = 0
				}
				lines = append(lines, normalStyle.Render(labelPart)+shortcutStyle.Render(item.Shortcut+strings.Repeat(" ", scPad)))
			}
		} else {
			for utf8.RuneCountInString(label) < innerW {
				label += " "
			}
			if isHover {
				lines = append(lines, hoverStyle.Width(innerW).Render(label))
			} else {
				lines = append(lines, normalStyle.Width(innerW).Render(label))
			}
		}
	}

	inner := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(bgColor)
	rendered := boxStyle.Render(inner)

	w := lipgloss.Width(rendered)
	h := lipgloss.Height(rendered)

	// Position at menu coordinates (already clamped via menu.Clamp at creation)
	stampANSI(buf, menu.X, menu.Y, rendered, w, h)
}

// ctxMenuKB converts config.KeyBindings to contextmenu.KeyBindings.
func ctxMenuKB(kb config.KeyBindings) contextmenu.KeyBindings {
	return contextmenu.KeyBindings{
		NewTerminal: kb.NewTerminal,
		CloseWindow: kb.CloseWindow,
		Minimize:    kb.Minimize,
		Maximize:    kb.Maximize,
		SnapLeft:    kb.SnapLeft,
		SnapRight:   kb.SnapRight,
		Center:      kb.Center,
		TileAll:     kb.TileAll,
		Cascade:     kb.Cascade,
		Settings:    kb.Settings,
	}
}
