package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/icex/termdesk/internal/config"
)

// styledSpacer returns a string of spaces with the given background color.
// Use this between buttons/elements in modals to prevent terminal default
// background (usually black) from bleeding through.
func styledSpacer(bg lipgloss.Color, spaces string) string {
	return lipgloss.NewStyle().Background(bg).Render(spaces)
}

// RenderConfirmDialog draws a confirmation dialog centered on the buffer.
func RenderConfirmDialog(buf *Buffer, dialog *ConfirmDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	contentBg := lipgloss.Color(theme.ActiveBorderBg)

	innerW := runeLen(dialog.Title) + 4
	if innerW < 26 {
		innerW = 26
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(dialog.Title)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	// Buttons — manually padded to equal width with centered text.
	// "Yes" and "No" are pre-padded to the same width so lipgloss
	// just applies colors without any centering math.
	yesText := "  \uf00c  Yes  "
	noText := "  \uf00d  No   "
	btnYesStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.AccentFg)).Background(lipgloss.Color(theme.AccentColor))
	btnNoStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.ButtonFg)).Background(lipgloss.Color(theme.ButtonNoBg))
	dimBtnFg := lipgloss.Color(theme.MenuBarFg)
	dimBtnStyle := lipgloss.NewStyle().Bold(true).
		Foreground(dimBtnFg).Background(contentBg)

	var yesStr, noStr string
	if dialog.Selected == 0 {
		yesStr = btnYesStyle.Render(yesText)
		noStr = dimBtnStyle.Render(noText)
	} else {
		yesStr = dimBtnStyle.Render(yesText)
		noStr = btnNoStyle.Render(noText)
	}
	
	btnRow := lipgloss.JoinHorizontal(lipgloss.Top, yesStr, styledSpacer(contentBg, "  "), noStr)
	btnRowStyle := lipgloss.NewStyle().
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Center)
	btnStr := btnRowStyle.Render(btnRow)

	// Compose
	inner := strings.Join([]string{titleStr, sepStr, btnStr}, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(contentBg)
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

// RenderRenameDialog draws a text input dialog for renaming a window.
// Uses the same lipgloss-based style as RenderConfirmDialog for consistency.
func RenderRenameDialog(buf *Buffer, dialog *RenameDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	title := "Rename Window"
	inputFieldW := 26

	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	contentBg := lipgloss.Color(theme.ActiveBorderBg)

	innerW := inputFieldW + 4
	if innerW < runeLen(title)+4 {
		innerW = runeLen(title) + 4
	}
	if innerW < 26 {
		innerW = 26
	}

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(title)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	// Input field — build visible text slice
	text := dialog.Text
	visStart := 0
	if dialog.Cursor > inputFieldW-1 {
		visStart = dialog.Cursor - inputFieldW + 1
	}
	var inputChars []rune
	for i := 0; i < inputFieldW; i++ {
		idx := visStart + i
		if idx < len(text) {
			inputChars = append(inputChars, text[idx])
		} else {
			inputChars = append(inputChars, ' ')
		}
	}
	inputPad := 2
	inputStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Width(innerW).
		PaddingLeft(inputPad)
	inputStr := inputStyle.Render(string(inputChars))

	// Buttons — same style as RenderConfirmDialog
	okText := "  \uf00c  OK    "
	cancelText := "  \uf00d  Cancel "
	btnYesStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.AccentFg)).Background(lipgloss.Color(theme.AccentColor))
	btnNoStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.ButtonFg)).Background(lipgloss.Color(theme.ButtonNoBg))
	dimBtnFg := lipgloss.Color(theme.MenuBarFg)
	dimBtnStyle := lipgloss.NewStyle().Bold(true).
		Foreground(dimBtnFg).Background(contentBg)

	var okStr, cancelStr string
	if dialog.Selected == 0 {
		okStr = btnYesStyle.Render(okText)
		cancelStr = dimBtnStyle.Render(cancelText)
	} else {
		okStr = dimBtnStyle.Render(okText)
		cancelStr = btnNoStyle.Render(cancelText)
	}
	
	btnRow := lipgloss.JoinHorizontal(lipgloss.Top, okStr, styledSpacer(contentBg, "  "), cancelStr)
	btnRowStyle := lipgloss.NewStyle().
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Center)
	btnStr := btnRowStyle.Render(btnRow)

	// Compose
	inner := strings.Join([]string{titleStr, sepStr, inputStr, sepStr, btnStr}, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(contentBg)
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

	// Invert fg/bg at cursor position for the text cursor
	cursorDisplayPos := dialog.Cursor - visStart
	// Row inside box: 0=title, 1=sep, 2=input, 3=buttons; +1 for border top
	cursorBufY := startY + 1 + 2
	cursorBufX := startX + 1 + inputPad + cursorDisplayPos
	if cursorBufX >= 0 && cursorBufX < buf.Width && cursorBufY >= 0 && cursorBufY < buf.Height {
		cell := &buf.Cells[cursorBufY][cursorBufX]
		cell.Fg, cell.Bg = cell.Bg, cell.Fg
	}
}

// RenderBufferNameDialog draws a compact input dialog for naming clipboard buffers.
func RenderBufferNameDialog(buf *Buffer, dialog *BufferNameDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	title := "Name Buffer"
	inputFieldW := 28

	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	contentBg := lipgloss.Color(theme.ActiveBorderBg)

	innerW := inputFieldW + 4
	if innerW < runeLen(title)+4 {
		innerW = runeLen(title) + 4
	}
	if innerW < 28 {
		innerW = 28
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(title)

	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	text := dialog.Text
	visStart := 0
	if dialog.Cursor > inputFieldW-1 {
		visStart = dialog.Cursor - inputFieldW + 1
	}
	var inputChars []rune
	for i := 0; i < inputFieldW; i++ {
		idx := visStart + i
		if idx < len(text) {
			inputChars = append(inputChars, text[idx])
		} else {
			inputChars = append(inputChars, ' ')
		}
	}
	inputStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Width(innerW).
		PaddingLeft(2)
	inputStr := inputStyle.Render(string(inputChars))

	hintStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Center)
	hintStr := hintStyle.Render("Enter: Save  Esc: Cancel")

	inner := strings.Join([]string{titleStr, sepStr, inputStr, sepStr, hintStr}, "\n")

	boxStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
	rendered := boxStyle.Render(inner)

	h := lipgloss.Height(rendered)
	w := lipgloss.Width(rendered)
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

// RenderNewWorkspaceDialog renders the new workspace creation dialog with directory browser.
func RenderNewWorkspaceDialog(buf *Buffer, dialog *NewWorkspaceDialog, theme config.Theme) {
	if dialog == nil {
		return
	}

	title := "New Workspace"
	innerW := 50

	fgColor := lipgloss.Color(theme.ActiveTitleFg)
	bgColor := lipgloss.Color(theme.ActiveTitleBg)
	borderColor := lipgloss.Color(theme.ActiveBorderFg)
	contentBg := lipgloss.Color(theme.ActiveBorderBg)
	accentFg := lipgloss.Color(theme.AccentFg)
	accentBg := lipgloss.Color(theme.AccentColor)
	subtleFg := lipgloss.Color(theme.SubtleFg)

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Width(innerW).
		Align(lipgloss.Center)
	titleStr := titleStyle.Render(title)

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(contentBg).
		Width(innerW)
	sepStr := sepStyle.Render(strings.Repeat("─", innerW))

	// Name input field
	nameLabelStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Bold(true).
		Width(innerW).
		PaddingLeft(2)
	nameLabelStr := nameLabelStyle.Render("Name:")

	nameFieldW := innerW - 4 // padding
	nameInputStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Width(innerW).
		PaddingLeft(2)
	if dialog.Cursor == 0 {
		nameInputStyle = nameInputStyle.Underline(true)
	}
	// Build visible name text
	visStart := 0
	if dialog.TextCursor > nameFieldW-1 {
		visStart = dialog.TextCursor - nameFieldW + 1
	}
	var nameChars []rune
	for i := 0; i < nameFieldW; i++ {
		idx := visStart + i
		if idx < len(dialog.Name) {
			nameChars = append(nameChars, dialog.Name[idx])
		} else {
			nameChars = append(nameChars, ' ')
		}
	}
	nameInputStr := nameInputStyle.Render(string(nameChars))

	// Directory browser label with current path
	pathLabelStyle := lipgloss.NewStyle().
		Foreground(fgColor).
		Background(contentBg).
		Bold(true).
		Width(innerW).
		PaddingLeft(2)
	pathLabel := "Directory:"
	pathLabelStr := pathLabelStyle.Render(pathLabel)

	// Current path display
	pathDisplayStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(contentBg).
		Width(innerW).
		PaddingLeft(3)
	dirDisplay := dialog.DirPath
	maxPathW := innerW - 4
	if len([]rune(dirDisplay)) > maxPathW {
		dirDisplay = "…" + string([]rune(dirDisplay)[len([]rune(dirDisplay))-maxPathW+1:])
	}
	pathDisplayStr := pathDisplayStyle.Render(dirDisplay)

	// Directory listing
	browserActive := dialog.Cursor == 1
	totalEntries := 1 + len(dialog.DirEntries) // ".." + entries
	visibleRows := dirBrowserVisibleRows
	if totalEntries < visibleRows {
		visibleRows = totalEntries
	}

	var dirRows []string
	for i := 0; i < visibleRows; i++ {
		idx := dialog.DirScroll + i
		if idx >= totalEntries {
			break
		}
		var entryName string
		var icon string
		if idx == 0 {
			entryName = ".."
			icon = "\uf07c " // folder-open
		} else {
			entryName = dialog.DirEntries[idx-1]
			icon = "\uf07b " // folder
		}

		display := icon + entryName
		maxEntryW := innerW - 6
		if len([]rune(display)) > maxEntryW {
			display = string([]rune(display)[:maxEntryW-1]) + "…"
		}

		rowStyle := lipgloss.NewStyle().
			Background(contentBg).
			Foreground(fgColor).
			Width(innerW).
			PaddingLeft(3)

		if browserActive && idx == dialog.DirSelect {
			rowStyle = rowStyle.
				Background(accentBg).
				Foreground(accentFg).
				Bold(true)
		}

		dirRows = append(dirRows, rowStyle.Render(display))
	}

	// Scroll indicator
	scrollInfo := ""
	if totalEntries > dirBrowserVisibleRows {
		scrollInfo = fmt.Sprintf(" (%d/%d)", dialog.DirSelect+1, totalEntries)
	}
	scrollStyle := lipgloss.NewStyle().
		Foreground(subtleFg).
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Right).
		PaddingRight(2)
	scrollStr := scrollStyle.Render(scrollInfo)

	// Buttons
	createText := "  \uf00c  Create "
	cancelText := "  \uf00d  Cancel "
	btnYesStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.AccentFg)).Background(lipgloss.Color(theme.AccentColor))
	btnNoStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(theme.ButtonFg)).Background(lipgloss.Color(theme.ButtonNoBg))
	dimBtnFg := lipgloss.Color(theme.MenuBarFg)
	dimBtnStyle := lipgloss.NewStyle().Bold(true).
		Foreground(dimBtnFg).Background(contentBg)

	var createStr, cancelStr string
	if dialog.Cursor == 2 && dialog.Selected == 0 {
		createStr = btnYesStyle.Render(createText)
	} else {
		createStr = dimBtnStyle.Render(createText)
	}
	if dialog.Cursor == 2 && dialog.Selected == 1 {
		cancelStr = btnNoStyle.Render(cancelText)
	} else {
		cancelStr = dimBtnStyle.Render(cancelText)
	}

	btnRow := lipgloss.JoinHorizontal(lipgloss.Top, createStr, styledSpacer(contentBg, "  "), cancelStr)
	btnRowStyle := lipgloss.NewStyle().
		Background(contentBg).
		Width(innerW).
		Align(lipgloss.Center)
	btnStr := btnRowStyle.Render(btnRow)

	// Compose all rows
	rows := []string{titleStr, sepStr, nameLabelStr, nameInputStr, sepStr, pathLabelStr, pathDisplayStr}
	rows = append(rows, dirRows...)
	rows = append(rows, scrollStr, sepStr, btnStr)
	inner := strings.Join(rows, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderBackground(contentBg)
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

	// Draw text cursor in name field
	if dialog.Cursor == 0 {
		nameVisStart := 0
		if dialog.TextCursor > nameFieldW-1 {
			nameVisStart = dialog.TextCursor - nameFieldW + 1
		}
		cursorDisplayPos := dialog.TextCursor - nameVisStart

		// Row inside box: 0=title, 1=sep, 2=nameLabel, 3=nameInput; +1 for border top
		cursorBufY := startY + 1 + 3
		cursorBufX := startX + 1 + 2 + cursorDisplayPos // border + padding

		if cursorBufX >= 0 && cursorBufX < buf.Width && cursorBufY >= 0 && cursorBufY < buf.Height {
			cell := &buf.Cells[cursorBufY][cursorBufX]
			cell.Fg, cell.Bg = cell.Bg, cell.Fg
		}
	}
}
