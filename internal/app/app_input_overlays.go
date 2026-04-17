package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

// handleDockNav handles keyboard navigation of the dock bar.
// Returns (model, cmd, handled). When handled=false, the key should be
// processed by normal-mode key handling.
func (m Model) handleDockNav(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "left", "h":
		idx := m.dock.HoverIndex - 1
		if idx < 0 {
			idx = m.dock.ItemCount() - 1
		}
		m.dock.SetHover(idx)
		if idx >= 0 && idx < len(m.dock.Items) {
			m.tooltipText = m.dock.Items[idx].TooltipText()
			m.tooltipX = m.dock.ItemCenterX(idx)
			m.tooltipY = m.height - 2
		}
		return m, nil, true
	case "right", "l":
		idx := m.dock.HoverIndex + 1
		if idx >= m.dock.ItemCount() {
			idx = 0
		}
		m.dock.SetHover(idx)
		if idx >= 0 && idx < len(m.dock.Items) {
			m.tooltipText = m.dock.Items[idx].TooltipText()
			m.tooltipX = m.dock.ItemCenterX(idx)
			m.tooltipY = m.height - 2
		}
		return m, nil, true
	case "enter", "space":
		m.tooltipText = ""
		ret, cmd := m.activateDockItem(m.dock.HoverIndex)
		return ret, cmd, true
	case "tab":
		ret, cmd := m.tabCycleForward()
		return ret, cmd, true
	case "shift+tab":
		ret, cmd := m.tabCycleBackward()
		return ret, cmd, true
	case "esc", "escape", ".", "up", "k":
		m.dockFocused = false
		m.dock.SetHover(-1)
		m.tooltipText = ""
		return m, nil, true
	}
	// Unrecognized key — exit dock focus and let normal-mode handling process it
	m.dockFocused = false
	m.dock.SetHover(-1)
	m.tooltipText = ""
	return m, nil, false
}

// handleMenuBarFocusNav handles keyboard navigation when the menu bar has Tab focus.
// Returns (model, cmd, handled). When handled=false, the key should be
// processed by normal-mode key handling.
func (m Model) handleMenuBarFocusNav(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "left", "h":
		m.menuBarFocusIdx--
		if m.menuBarFocusIdx < 0 {
			m.menuBarFocusIdx = len(m.menuBar.Menus) - 1
		}
		m.menuBar.FocusIndex = m.menuBarFocusIdx
		return m, nil, true
	case "right", "l":
		m.menuBarFocusIdx++
		if m.menuBarFocusIdx >= len(m.menuBar.Menus) {
			m.menuBarFocusIdx = 0
		}
		m.menuBar.FocusIndex = m.menuBarFocusIdx
		return m, nil, true
	case "enter", "space", "down", "j":
		// Open the dropdown for the focused menu label
		m.menuBarFocused = false
		m.menuBar.FocusIndex = -1
		m.menuBar.OpenMenu(m.menuBarFocusIdx)
		return m, nil, true
	case "tab":
		m.menuBar.FocusIndex = -1
		ret, cmd := m.tabCycleForward()
		return ret, cmd, true
	case "shift+tab":
		m.menuBar.FocusIndex = -1
		ret, cmd := m.tabCycleBackward()
		return ret, cmd, true
	case "esc", "escape":
		m.menuBarFocused = false
		m.menuBar.FocusIndex = -1
		return m, nil, true
	}
	// Unrecognized key — exit menu bar focus and let normal-mode handle it
	m.menuBarFocused = false
	m.menuBar.FocusIndex = -1
	return m, nil, false
}

// activateDockItem launches or restores the dock item at the given index.
func (m Model) activateDockItem(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.dock.Items) {
		return m, nil
	}
	item := m.dock.Items[idx]
	m.dockFocused = false
	m.dock.SetHover(-1)

	switch item.Special {
	case "launcher":
		m.launcher.Toggle()
		if m.launcher.Visible && m.launcher.NeedsExecScan() {
			m.launcher.MarkExecLoading()
			return m, launcher.ScanExecIndex()
		}
		return m, nil
	case "expose":
		if m.exposeMode {
			m.exitExpose()
		} else {
			m.enterExpose()
		}
		return m, tickAnimation()
	case "minimized":
		// Restore the minimized window
		if w := m.wm.WindowByID(item.WindowID); w != nil {
			m.restoreMinimizedWindow(w)
			return m, tickAnimation()
		}
		return m, nil
	case "running":
		// Toggle minimize/focus for running windows
		if w := m.wm.WindowByID(item.WindowID); w != nil {
			// If the window is already focused, minimize it
			if m.wm.FocusedWindow() != nil && m.wm.FocusedWindow().ID == w.ID {
				m.minimizeWindow(w)
				return m, tickAnimation()
			}
			// Otherwise, focus the window
			m.wm.FocusWindow(w.ID)
			m.inputMode = ModeTerminal
		}
		return m, nil
	default:
		// Launch the app (focus existing if already running)
		if item.Command != "" {
			return m.focusOrLaunchApp(item.Command)
		}
	}
	return m, nil
}

func (m Model) handleCopyModeKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// Resolve terminal and content dimensions for copy mode.
	// For quake terminal there is no focused window — use quake state directly.
	var term *terminal.Terminal
	var contentH int
	var windowID string
	if m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID {
		term = m.quakeTerminal
		contentH = int(m.quakeAnimH) - 1
		if contentH < 1 {
			contentH = 1
		}
		windowID = quakeTermID
	} else {
		fw, t := m.focusedTerminal()
		if fw == nil || t == nil {
			m.exitCopyMode()
			m.inputMode = ModeNormal
			return m, nil
		}
		term = t
		// For split panes, use pane rect height and pane terminal ID
		if fw.IsSplit() && fw.FocusedPane != "" {
			contentH = m.paneRectForTerm(fw, fw.FocusedPane).Height
			windowID = fw.FocusedPane
		} else {
			contentH = fw.ContentRect().Height
			windowID = fw.ID
		}
	}
	if term == nil {
		m.exitCopyMode()
		m.inputMode = ModeNormal
		return m, nil
	}
	if m.copySearchActive {
		switch key {
		case "esc", "escape":
			m.copySearchActive = false
			m.copySearchQuery = ""
			m.copySearchDir = 0
			m.copySearchMatchCount = 0
			m.copySearchMatchIdx = 0
			return m, nil
		case "enter":
			m.copySearchActive = false
			return m, nil
		case "backspace":
			if m.copySearchQuery != "" {
				runes := []rune(m.copySearchQuery)
				m.copySearchQuery = string(runes[:len(runes)-1])
			}
			m = m.applyCopySearch(m.copySearchDir)
			return m, nil
		default:
			k := tea.Key(msg)
			if k.Text != "" {
				for _, ch := range k.Text {
					m.copySearchQuery += string(ch)
				}
			}
			m = m.applyCopySearch(m.copySearchDir)
			return m, nil
		}
	}
	if key == m.keybindings.Prefix {
		m.prefixPending = true
		return m, nil
	}
	if key == "/" || key == "?" {
		m.copySearchActive = true
		m.copySearchQuery = ""
		if key == "?" {
			m.copySearchDir = -1
		} else {
			m.copySearchDir = 1
		}
		m.copyCount = 0
		m.copyLastKey = ""
		return m, nil
	}
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		m.copyCount = m.copyCount*10 + int(key[0]-'0')
		return m, nil
	}
	if key != "g" {
		m.copyLastKey = ""
	}
	snap := m.copySnapshotForWindow(windowID)
	maxScroll := term.ScrollbackLen()
	if snap != nil {
		maxScroll = snap.ScrollbackLen()
	}
	termH := term.Height()
	termW := term.Width()
	if snap != nil {
		termH = snap.Height
		termW = snap.Width
	}
	totalLines := maxScroll + termH

	// moveCursor moves the copy cursor and keeps the viewport following it.
	moveCursor := func(dx, dy int) {
		m.copyCursorY += dy
		if m.copyCursorY < 0 {
			m.copyCursorY = 0
		}
		if m.copyCursorY >= totalLines {
			m.copyCursorY = totalLines - 1
		}
		m.copyCursorX += dx
		if m.copyCursorX < 0 {
			m.copyCursorX = 0
		}
		if m.copyCursorX >= termW {
			m.copyCursorX = termW - 1
		}
		// Sync selection end to cursor when selection is active
		if m.selActive {
			m.selEnd.X = m.copyCursorX
			m.selEnd.Y = m.copyCursorY
		}
		m.ensureCursorVisible(maxScroll, contentH, maxScroll)
	}

	setCursorPos := func(x, y int) {
		if y < 0 {
			y = 0
		}
		if y >= totalLines {
			y = totalLines - 1
		}
		if x < 0 {
			x = 0
		}
		if x >= termW {
			x = termW - 1
		}
		m.copyCursorX = x
		m.copyCursorY = y
		if m.selActive {
			m.selEnd.X = x
			m.selEnd.Y = y
		}
		m.ensureCursorVisible(maxScroll, contentH, maxScroll)
	}

	switch key {
	case "esc", "escape":
		m.copyCount = 0
		m.copyLastKey = ""
		m.copySearchActive = false
		m.copySearchQuery = ""
		m.copySearchDir = 0
		if m.selActive {
			m.selActive = false
		} else {
			m.exitCopyMode()
		}
	case "q":
		m.exitCopyMode()
		m.inputMode = ModeNormal
	case "i":
		m.exitCopyMode()
	case "v":
		m.copyCount = 0
		m.copyLastKey = ""
		// Toggle visual selection mode
		if m.selActive {
			m.selActive = false
		} else {
			m.selActive = true
			// Start selection at cursor position
			m.selStart = geometry.Point{X: m.copyCursorX, Y: m.copyCursorY}
			m.selEnd = m.selStart
		}
	case "y", "enter":
		m.copyCount = 0
		m.copyLastKey = ""
		// Yank (copy) selected text, or just exit copy mode on enter with no selection
		if m.selActive {
			text := extractSelTextWithSnapshot(term, snap, m.selStart, m.selEnd)
			if text != "" {
				writeOSC52(text)
				m.clipboard.Copy(text)
			}
			m.selActive = false
			m.clearCopySearch()
			m.copySnapshot = nil
		}
		if key == "enter" {
			m.exitCopyMode()
		}
	case "up", "k":
		step := 1
		if m.copyCount > 0 {
			step = m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, -step)
	case "down", "j":
		step := 1
		if m.copyCount > 0 {
			step = m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, step)
	case "left", "h":
		step := 1
		if m.copyCount > 0 {
			step = m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(-step, 0)
	case "right", "l":
		step := 1
		if m.copyCount > 0 {
			step = m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(step, 0)
	case "pgup":
		page := contentH
		if m.copyCount > 0 {
			page *= m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, -page)
	case "pgdown":
		page := contentH
		if m.copyCount > 0 {
			page *= m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, page)
	case "home", "g":
		if key == "g" {
			if m.copyLastKey != "g" {
				m.copyLastKey = "g"
				m.copyCount = 0
				return m, nil
			}
			m.copyLastKey = ""
		}
		m.copyCount = 0
		setCursorPos(0, 0)
	case "end", "G", "shift+g":
		m.copyCount = 0
		m.copyLastKey = ""
		setCursorPos(termW-1, totalLines-1)

	// --- Halfpage scroll (Ctrl+U / Ctrl+D) ---
	case "ctrl+u":
		half := contentH / 2
		if half < 1 {
			half = 1
		}
		if m.copyCount > 0 {
			half *= m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, -half)
	case "ctrl+d":
		half := contentH / 2
		if half < 1 {
			half = 1
		}
		if m.copyCount > 0 {
			half *= m.copyCount
		}
		m.copyCount = 0
		m.copyLastKey = ""
		moveCursor(0, half)

	// --- Line start (0) ---
	case "0":
		if m.copyCount > 0 {
			// '0' is part of a numeric count, already handled above
			break
		}
		m.copyLastKey = ""
		setCursorPos(0, m.copyCursorY)

	// --- Back to indentation (^) ---
	case "^", "shift+6": // ^ key
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		if m.copyCursorY >= 0 && m.copyCursorY < len(lines) {
			line := lines[m.copyCursorY]
			col := 0
			for _, r := range line {
				if r != ' ' && r != '\t' {
					break
				}
				col++
			}
			setCursorPos(col, m.copyCursorY)
		}

	// --- End of line ($) ---
	case "$", "shift+4": // $ key
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		if m.copyCursorY >= 0 && m.copyCursorY < len(lines) {
			runes := []rune(lines[m.copyCursorY])
			end := len(runes) - 1
			for end > 0 && runes[end] == ' ' {
				end--
			}
			setCursorPos(end, m.copyCursorY)
		}

	// --- Paragraph navigation ({/}) ---
	case "{", "shift+[": // { — previous paragraph
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		if len(lines) > 0 {
			y := m.copyCursorY - 1
			for y > 0 && strings.TrimSpace(lines[y]) == "" {
				y--
			}
			for y > 0 && strings.TrimSpace(lines[y]) != "" {
				y--
			}
			if y < 0 {
				y = 0
			}
			setCursorPos(0, y)
		}
	case "}", "shift+]": // } — next paragraph
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		if len(lines) > 0 {
			y := m.copyCursorY + 1
			for y < len(lines) && strings.TrimSpace(lines[y]) != "" {
				y++
			}
			for y < len(lines) && strings.TrimSpace(lines[y]) == "" {
				y++
			}
			if y >= len(lines) {
				y = len(lines) - 1
			}
			setCursorPos(0, y)
		}

	// --- Matching bracket (%) ---
	case "%", "shift+5": // % key
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		if m.copyCursorY >= 0 && m.copyCursorY < len(lines) {
			newY, newX := findMatchingBracket(lines, m.copyCursorY, m.copyCursorX)
			if newY >= 0 {
				setCursorPos(newX, newY)
			}
		}

	// --- Other end (o) — toggle between start/end of selection ---
	case "o":
		m.copyCount = 0
		m.copyLastKey = ""
		if m.selActive {
			m.selStart, m.selEnd = m.selEnd, m.selStart
			m.copyCursorX = m.selEnd.X
			m.copyCursorY = m.selEnd.Y
			m.ensureCursorVisible(maxScroll, contentH, maxScroll)
		}

	// --- Search next/prev (n/N) ---
	case "n":
		m.copyCount = 0
		m.copyLastKey = ""
		if m.copySearchQuery != "" {
			m = m.applyCopySearch(m.copySearchDir)
		}
	case "N", "shift+n":
		m.copyCount = 0
		m.copyLastKey = ""
		if m.copySearchQuery != "" {
			m = m.applyCopySearch(-m.copySearchDir)
		}

	// --- Copy line (Y) ---
	case "Y", "shift+y":
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		lineIdx := m.copyCursorY
		if lineIdx >= 0 && lineIdx < len(lines) {
			text := strings.TrimRight(lines[lineIdx], " ")
			if text != "" {
				writeOSC52(text)
				m.clipboard.Copy(text)
			}
		}

	// --- Append selection (A) ---
	case "A", "shift+a":
		m.copyCount = 0
		m.copyLastKey = ""
		if m.selActive {
			text := extractSelTextWithSnapshot(term, snap, m.selStart, m.selEnd)
			if text != "" {
				writeOSC52(text)
				m.clipboard.Append(text)
			}
		}

	// --- Word motions (w/b/e) — cross line boundaries like vim/tmux ---
	case "w":
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		y, x := m.copyCursorY, m.copyCursorX
		if y >= 0 && y < len(lines) {
			runes := []rune(lines[y])
			// Skip current word
			for x < len(runes) && !isWordSep(runes[x]) {
				x++
			}
			// Skip separators
			for x < len(runes) && isWordSep(runes[x]) {
				x++
			}
			// If we hit end of line, wrap to next line's first word
			for x >= len(runes) && y+1 < len(lines) {
				y++
				runes = []rune(lines[y])
				x = 0
				// Skip leading separators on new line
				for x < len(runes) && isWordSep(runes[x]) {
					x++
				}
				if x < len(runes) {
					break
				}
			}
			if x >= len(runes) && len(runes) > 0 {
				x = len(runes) - 1
			}
			if x < 0 {
				x = 0
			}
			setCursorPos(x, y)
		}
	case "b":
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		y, x := m.copyCursorY, m.copyCursorX
		if y >= 0 && y < len(lines) {
			x--
			// If we went before line start, wrap to previous line
			for x < 0 && y > 0 {
				y--
				runes := []rune(lines[y])
				x = len(runes) - 1
				// Skip trailing separators
				for x >= 0 && isWordSep(runes[x]) {
					x--
				}
				if x >= 0 {
					break
				}
			}
			if x < 0 {
				x = 0
			}
			runes := []rune(lines[y])
			if len(runes) > 0 && x < len(runes) {
				// Skip separators backward
				for x > 0 && isWordSep(runes[x]) {
					x--
				}
				// Skip word chars backward to find start
				for x > 0 && !isWordSep(runes[x-1]) {
					x--
				}
			}
			setCursorPos(x, y)
		}
	case "e":
		m.copyCount = 0
		m.copyLastKey = ""
		lines := collectTerminalLines(term, snap)
		y, x := m.copyCursorY, m.copyCursorX
		if y >= 0 && y < len(lines) {
			x++
			runes := []rune(lines[y])
			// If we hit end of line, wrap to next line
			for x >= len(runes) && y+1 < len(lines) {
				y++
				runes = []rune(lines[y])
				x = 0
			}
			if x < len(runes) {
				// Skip separators forward
				for x < len(runes) && isWordSep(runes[x]) {
					x++
				}
				// If skipping separators hit end of line, wrap
				for x >= len(runes) && y+1 < len(lines) {
					y++
					runes = []rune(lines[y])
					x = 0
					for x < len(runes) && isWordSep(runes[x]) {
						x++
					}
				}
				// Move to end of word
				for x < len(runes)-1 && !isWordSep(runes[x+1]) {
					x++
				}
			}
			if x < 0 {
				x = 0
			}
			if x >= len(runes) && len(runes) > 0 {
				x = len(runes) - 1
			}
			setCursorPos(x, y)
		}
	}
	return m, nil
}

// isWordSep returns true if the rune is a word separator.
func isWordSep(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == ',' || r == ';' || r == ':' ||
		r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}' ||
		r == '<' || r == '>' || r == '"' || r == '\'' || r == '`' || r == '/' ||
		r == '\\' || r == '|' || r == '!' || r == '@' || r == '#' || r == '$' ||
		r == '%' || r == '^' || r == '&' || r == '*' || r == '-' || r == '=' ||
		r == '+' || r == '~'
}

// findMatchingBracket finds the matching bracket for the character at (y, x).
func findMatchingBracket(lines []string, y, x int) (int, int) {
	if y < 0 || y >= len(lines) {
		return -1, -1
	}
	runes := []rune(lines[y])
	if x < 0 || x >= len(runes) {
		return -1, -1
	}
	ch := runes[x]
	var match rune
	dir := 1
	switch ch {
	case '(':
		match = ')'
	case ')':
		match, dir = '(', -1
	case '[':
		match = ']'
	case ']':
		match, dir = '[', -1
	case '{':
		match = '}'
	case '}':
		match, dir = '{', -1
	case '<':
		match = '>'
	case '>':
		match, dir = '<', -1
	default:
		return -1, -1
	}
	depth := 1
	cy, cx := y, x
	for {
		cx += dir
		for cx < 0 || (cy < len(lines) && cx >= len([]rune(lines[cy]))) {
			cy += dir
			if cy < 0 || cy >= len(lines) {
				return -1, -1
			}
			if dir > 0 {
				cx = 0
			} else {
				cx = len([]rune(lines[cy])) - 1
			}
		}
		if cy < 0 || cy >= len(lines) {
			return -1, -1
		}
		r := []rune(lines[cy])[cx]
		if r == ch {
			depth++
		} else if r == match {
			depth--
			if depth == 0 {
				return cy, cx
			}
		}
	}
}

func (m Model) applyCopySearch(dir int) Model {
	// Resolve terminal for copy search — quake or focused window.
	var term *terminal.Terminal
	var windowID string
	if m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID {
		term = m.quakeTerminal
		windowID = quakeTermID
	} else {
		fw, t := m.focusedTerminal()
		if t == nil {
			return m
		}
		term = t
		if fw.IsSplit() && fw.FocusedPane != "" {
			windowID = fw.FocusedPane
		} else {
			windowID = fw.ID
		}
	}
	if term == nil {
		return m
	}
	snap := m.copySnapshotForWindow(windowID)
	query := strings.TrimSpace(m.copySearchQuery)
	if query == "" {
		m.copySearchMatchCount = 0
		m.copySearchMatchIdx = 0
		return m
	}

	lines := collectTerminalLines(term, snap)
	if len(lines) == 0 {
		return m
	}

	sbLen := term.ScrollbackLen()
	if snap != nil {
		sbLen = snap.ScrollbackLen()
	}
	// Search from cursor position rather than viewport offset
	start := m.copyCursorY
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		start = len(lines) - 1
	}

	q := strings.ToLower(query)
	match := func(idx int) bool {
		return strings.Contains(strings.ToLower(lines[idx]), q)
	}

	// Collect all matching line indices for count
	var matchLines []int
	for i := 0; i < len(lines); i++ {
		if match(i) {
			matchLines = append(matchLines, i)
		}
	}
	m.copySearchMatchCount = len(matchLines)

	if len(matchLines) == 0 {
		m.copySearchMatchIdx = 0
		return m
	}

	// Find next match from current position
	found := -1
	if dir >= 0 {
		for _, idx := range matchLines {
			if idx > start {
				found = idx
				break
			}
		}
		if found == -1 {
			found = matchLines[0] // wrap
		}
	} else {
		for i := len(matchLines) - 1; i >= 0; i-- {
			if matchLines[i] < start {
				found = matchLines[i]
				break
			}
		}
		if found == -1 {
			found = matchLines[len(matchLines)-1] // wrap
		}
	}

	// Determine match index (1-based display)
	for i, idx := range matchLines {
		if idx == found {
			m.copySearchMatchIdx = i
			break
		}
	}

	// Move cursor to the found match line
	m.copyCursorY = found
	// Position cursor X at the start of the match within the line
	if found >= 0 && found < len(lines) {
		idx := strings.Index(strings.ToLower(lines[found]), q)
		if idx >= 0 {
			m.copyCursorX = len([]rune(lines[found][:idx]))
		}
	}
	if m.selActive {
		m.selEnd.X = m.copyCursorX
		m.selEnd.Y = m.copyCursorY
	}

	if found < sbLen {
		m.scrollOffset = sbLen - found
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset > sbLen {
			m.scrollOffset = sbLen
		}
	} else {
		m.scrollOffset = 0
	}
	return m
}

func collectTerminalLines(term *terminal.Terminal, snap *CopySnapshot) []string {
	if snap != nil {
		sbLen := snap.ScrollbackLen()
		lines := make([]string, 0, sbLen+snap.Height)
		for offset := sbLen - 1; offset >= 0; offset-- {
			lines = append(lines, cellsToString(snap.ScrollbackLine(offset)))
		}
		for y := 0; y < snap.Height; y++ {
			if y < len(snap.Screen) {
				lines = append(lines, cellsToString(snap.Screen[y]))
			} else {
				lines = append(lines, "")
			}
		}
		return lines
	}

	sbLen := term.ScrollbackLen()
	height := term.Height()
	lines := make([]string, 0, sbLen+height)

	for offset := sbLen - 1; offset >= 0; offset-- {
		line := term.ScrollbackLine(offset)
		lines = append(lines, cellsToString(line))
	}

	snapScreen, _, h := term.SnapshotScreen()
	if snapScreen != nil && h > 0 {
		for y := 0; y < h; y++ {
			if y < len(snapScreen) {
				lines = append(lines, cellsToString(snapScreen[y]))
			} else {
				lines = append(lines, "")
			}
		}
		return lines
	}

	width := term.Width()
	for y := 0; y < height; y++ {
		row := make([]terminal.ScreenCell, width)
		for x := 0; x < width; x++ {
			if cell := term.CellAt(x, y); cell != nil {
				row[x] = terminal.ScreenCell{
					Content: cell.Content,
					Attrs:   cell.Style.Attrs,
					Fg:      cell.Style.Fg,
					Bg:      cell.Style.Bg,
					Width:   int8(cell.Width),
				}
			}
		}
		lines = append(lines, cellsToString(row))
	}
	return lines
}

func cellsToString(cells []terminal.ScreenCell) string {
	if len(cells) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, cell := range cells {
		w := int(cell.Width)
		if w == 0 {
			continue
		}
		if cell.Content == "" {
			sb.WriteByte(' ')
		} else {
			sb.WriteString(cell.Content)
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

func (m Model) handleClipboardKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "escape", "y":
		m.clipboard.HideHistory()
	case "up", "k":
		m.clipboard.MoveSelection(-1)
	case "down", "j":
		m.clipboard.MoveSelection(1)
	case "d", "delete":
		// Delete the selected clipboard entry
		items := m.clipboard.HistoryItems()
		if m.clipboard.SelectedIdx >= 0 && m.clipboard.SelectedIdx < len(items) {
			m.clipboard.DeleteItem(m.clipboard.SelectedIdx)
			// Adjust selection if necessary
			if m.clipboard.SelectedIdx >= m.clipboard.Len() && m.clipboard.Len() > 0 {
				m.clipboard.SelectedIdx = m.clipboard.Len() - 1
			}
		}
	case "v":
		text := m.clipboard.PasteSelected()
		if text != "" {
			m.clipboard.HideHistory()
			return m, m.openClipboardViewer(text)
		}
	case "n":
		items := m.clipboard.HistoryItems()
		if m.clipboard.SelectedIdx >= 0 && m.clipboard.SelectedIdx < len(items) {
			initial := []rune(m.clipboard.SelectedName())
			m.bufferNameDialog = &BufferNameDialog{
				Text:   initial,
				Cursor: len(initial),
			}
		}
	case "N":
		m.clipboard.SetSelectedName(m.clipboard.SelectedIdx, "")
	case "enter", "space":
		text := m.clipboard.PasteSelected()
		if text != "" {
			if _, term := m.focusedTerminal(); term != nil {
				term.WriteInput([]byte(text))
			}
		}
		m.clipboard.HideHistory()
		m.inputMode = ModeTerminal
	}
	return m, nil
}

func (m Model) handleNotificationCenterKey(key string) (tea.Model, tea.Cmd) {
	items := m.notifications.HistoryItems()
	switch key {
	case "esc", "escape", "b":
		m.notifications.HideCenter()
	case "up", "k":
		m.notifications.MoveCenterSelection(-1)
	case "down", "j":
		m.notifications.MoveCenterSelection(1)
	case "d":
		if len(items) > 0 && m.notifications.CenterIndex() < len(items) {
			m.notifications.DeleteFromHistory(items[m.notifications.CenterIndex()].ID)
		}
	case "D":
		m.notifications.ClearHistory()
	}
	return m, nil
}

func (m Model) handleTourKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "escape":
		m.tour.Skip()
		m.saveTourCompleted()
	case "enter", "space", "right":
		if !m.tour.Next() {
			m.saveTourCompleted()
		}
	}
	return m, nil
}

func (m Model) handleContextMenuKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "escape":
		m.contextMenu.Hide()
	case "up", "k":
		m.contextMenu.MoveHover(-1)
	case "down", "j":
		m.contextMenu.MoveHover(1)
	case "enter", "space":
		action := m.contextMenu.SelectedAction()
		m.contextMenu.Hide()
		if action != "" {
			return m.executeMenuAction(action)
		}
	}
	return m, nil
}

func (m Model) handleSettingsKey(key string) (tea.Model, tea.Cmd) {
	// Text editing mode: most keys go to text buffer
	if m.settings.TextEditing {
		apply := false
		switch key {
		case "esc", "escape":
			m.settings.CancelTextEdit()
		case "enter":
			m.settings.CommitTextEdit()
			apply = true
		case "backspace":
			m.settings.TextBackspace()
		case "delete":
			m.settings.TextDelete()
		case "left":
			m.settings.TextMoveCursor(-1)
		case "right":
			m.settings.TextMoveCursor(1)
		default:
			// Insert printable characters (single rune or pasted string)
			if len(key) == 1 || (len(key) > 1 && !strings.HasPrefix(key, "ctrl+") && !strings.HasPrefix(key, "alt+") && !strings.HasPrefix(key, "shift+") && key != "tab" && key != "shift+tab" && key != "up" && key != "down") {
				m.settings.TextInsert(key)
			}
		}
		if apply {
			m.applySettings()
		}
		return m, nil
	}

	apply := false
	switch key {
	case "esc", "escape", ",":
		apply = true
		m.settings.Hide()
	case "tab":
		m.settings.NextTab()
	case "shift+tab":
		m.settings.PrevTab()
	case "up", "k":
		m.settings.PrevItem()
	case "down", "j":
		m.settings.NextItem()
	case "enter", "space":
		item := m.settings.CurrentItem()
		if item == nil {
			break
		}
		switch item.Type {
		case settings.TypeToggle:
			m.settings.Toggle()
			apply = true
		case settings.TypeText:
			m.settings.StartTextEdit()
		}
	case "left", "h":
		item := m.settings.CurrentItem()
		if item != nil && item.Type == settings.TypeText && len(item.Choices) > 0 {
			m.settings.CycleText(-1)
		} else {
			m.settings.CycleChoice(-1)
		}
		apply = true
	case "right", "l":
		item := m.settings.CurrentItem()
		if item != nil && item.Type == settings.TypeText && len(item.Choices) > 0 {
			m.settings.CycleText(1)
		} else {
			m.settings.CycleChoice(1)
		}
		apply = true
	}

	if apply {
		m.applySettings()
	}
	return m, nil
}

func (m Model) handleLauncherKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c", "ctrl+q":
		m.launcher.Hide()
		m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
		return m, nil
	case "esc", "escape", "ctrl+space", "ctrl+/":
		m.launcher.Hide()
		return m, nil
	case "up":
		m.launcher.MoveSelection(-1)
	case "down":
		m.launcher.MoveSelection(1)
	case "ctrl+up":
		m.launcher.PrevQuery()
	case "ctrl+down":
		m.launcher.NextQuery()
	case "tab":
		m.launcher.CompleteFromSelected()
	case "backspace":
		m.launcher.Backspace()
	case "ctrl+p":
		if m.launcher.ToggleFavorite() {
			m.notifications.Push("Favorited", m.launcher.SelectedEntry().Name, notification.Info)
		}
		m.saveLauncherState()
		return m, nil
	case "enter":
		query := m.launcher.Query
		entry := m.launcher.SelectedEntry()
		m.launcher.RecordQuery(query)
		m.launcher.Hide()
		// Query has args (spaces) → run as literal command line
		if strings.Contains(strings.TrimSpace(query), " ") {
			return m.launchCommandLine(query)
		}
		if entry != nil && entry.Source == "registry" {
			return m.launchAppFromRegistry(entry.Command)
		}
		if entry != nil && entry.Source == "exec" {
			return m.launchExecEntry(entry.Command, nil)
		}
		if strings.TrimSpace(query) != "" {
			return m.launchCommandLine(query)
		}
	default:
		// Type printable characters into the search
		k := tea.Key(msg)
		if k.Text != "" {
			for _, ch := range k.Text {
				m.launcher.TypeChar(ch)
			}
		}
	}
	return m, nil
}

func (m Model) handleMenuKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "escape", "f10":
		if m.menuBar.InSubMenu {
			m.menuBar.ExitSubMenu()
		} else {
			m.menuBar.CloseMenu()
		}
	case "up":
		m.menuBar.MoveHover(-1)
	case "down":
		m.menuBar.MoveHover(1)
	case "left":
		if m.menuBar.InSubMenu {
			m.menuBar.ExitSubMenu()
		} else {
			m.menuBar.MoveMenu(-1)
		}
	case "right":
		if !m.menuBar.InSubMenu && m.menuBar.HasSubMenu() {
			m.menuBar.EnterSubMenu()
		} else if !m.menuBar.InSubMenu {
			m.menuBar.MoveMenu(1)
		}
	case "enter", "space":
		if !m.menuBar.InSubMenu && m.menuBar.HasSubMenu() {
			m.menuBar.EnterSubMenu()
		} else {
			action := m.menuBar.SelectedAction()
			m.menuBar.CloseMenu()
			return m.executeMenuAction(action)
		}
	}
	return m, nil
}
