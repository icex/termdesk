package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/window"
)

// dirBrowserVisibleRows is the max number of directory entries visible at once.
const dirBrowserVisibleRows = 8

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Normalize single uppercase letters to lowercase so hotkeys work with caps lock.
	// Skip normalization in modes that distinguish uppercase letters (e.g. copy mode:
	// N=prev search, Y=copy line; clipboard: N=remove name; notification center: D=clear).
	skipNormalize := m.inputMode == ModeCopy || m.clipboard.Visible || m.notifications.CenterVisible()
	if !skipNormalize && len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' {
		key = strings.ToLower(key)
	}
	// Reset tab cycle counter on any non-tab key so stale counts don't cause
	// premature dock/menubar transitions on the next tab cycle.
	if key != "tab" && key != "shift+tab" {
		m.tabCycleCount = 0
		m.tabCycleDir = 0
	}
	// Quake dropdown terminal — global hotkey, works in all modes
	if key == m.keybindings.QuakeTerminal {
		cmd := m.toggleQuakeTerminal()
		return m, cmd
	}
	// Discard all keys while quake is animating to prevent
	// stale prefixPending state from triggering unintended actions.
	if m.quakeVisible && m.quakeAnimating() && m.inputMode == ModeTerminal {
		return m, nil
	}
	// When quake is visible in Terminal mode, forward keys to quake terminal
	// (like handleTerminalModeKey but using quakeTerminal instead of focused window)
	if m.quakeVisible && m.quakeTerminal != nil && !m.quakeAnimating() && m.inputMode == ModeTerminal {
		// Prefix key → enter prefix pending state
		if key == m.keybindings.Prefix {
			m.prefixPending = true
			return m, nil
		}
		if !m.prefixPending {
			k := tea.Key(msg)
			m.quakeTerminal.SendKey(k.Code, k.Mod, k.Text)
			return m, nil
		}
		// Fall through to normal prefix handling below
	}
	actionHint := ""
	if m.inputMode != ModeTerminal || m.prefixPending || key == m.keybindings.Prefix {
		actionHint = m.actionMap[key]
	}
	if key == m.keybindings.Prefix && m.inputMode == ModeTerminal {
		actionHint = "prefix"
	}
	if m.prefixPending {
		switch key {
		case "c":
			actionHint = "copy_mode"
		case "y":
			actionHint = "clipboard_history"
		case "d":
			actionHint = "detach"
		case "\"":
			actionHint = "split_vertical"
		case "%":
			actionHint = "split_horizontal"
		case "x":
			// In split mode, close pane; otherwise close window
			if fw := m.wm.FocusedWindow(); fw != nil && fw.IsSplit() {
				actionHint = "close_pane"
			} else {
				actionHint = "close_window"
			}
		case "o":
			actionHint = "next_pane"
		case ";":
			actionHint = "prev_pane"
		case "left":
			actionHint = "pane_left"
		case "right":
			actionHint = "pane_right"
		case "up":
			actionHint = "pane_up"
		case "down":
			actionHint = "pane_down"
		}
	}
	if m.inputMode == ModeNormal && key == "c" {
		actionHint = "copy_mode"
	}
	m.recordShowKey(key, actionHint)

	// ── Layer 1: UI overlays always take precedence ──

	if m.workspacePickerVisible {
		workspaces := m.workspaceList
		switch key {
		case "esc", "escape", "q":
			m.workspacePickerVisible = false
		case "up", "k":
			if len(workspaces) > 0 {
				m.workspacePickerSelected--
				if m.workspacePickerSelected < 0 {
					m.workspacePickerSelected = len(workspaces) - 1
				}
			}
		case "down", "j":
			if len(workspaces) > 0 {
				m.workspacePickerSelected++
				if m.workspacePickerSelected >= len(workspaces) {
					m.workspacePickerSelected = 0
				}
			}
		case "enter", "space":
			if m.workspacePickerSelected >= 0 && m.workspacePickerSelected < len(workspaces) {
				m.workspacePickerVisible = false
				return m, m.loadSelectedWorkspace()
			}
		}
		return m, nil
	}

	if m.modal != nil {
		switch key {
		case "esc", "escape", "q":
			m.modal = nil
		case "up", "k":
			if m.modal.ScrollY > 0 {
				m.modal.ScrollY--
			}
		case "down", "j":
			m.modal.ScrollY++
		case "tab", "right", "l":
			if m.modal.Tabs != nil {
				m.modal.ActiveTab = (m.modal.ActiveTab + 1) % len(m.modal.Tabs)
				m.modal.ScrollY = 0
			}
		case "shift+tab", "left", "h":
			if m.modal.Tabs != nil {
				m.modal.ActiveTab = (m.modal.ActiveTab - 1 + len(m.modal.Tabs)) % len(m.modal.Tabs)
				m.modal.ScrollY = 0
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.modal.Tabs != nil {
				idx := int(key[0]-'0') - 1
				if idx < len(m.modal.Tabs) {
					m.modal.ActiveTab = idx
					m.modal.ScrollY = 0
				}
			}
		}
		return m, nil
	}

	if m.confirmClose != nil {
		switch key {
		case "y":
			m.confirmClose.Selected = 0 // select Yes
			return m.confirmAccept()
		case "n", "esc", "escape":
			m.confirmClose = nil
			return m, nil
		case "enter", "space":
			return m.confirmAccept()
		case "left", "right", "tab", "shift+tab", "h", "l":
			// Toggle selected button
			if m.confirmClose.Selected == 0 {
				m.confirmClose.Selected = 1
			} else {
				m.confirmClose.Selected = 0
			}
		}
		return m, nil
	}

	if m.bufferNameDialog != nil {
		switch key {
		case "enter":
			name := strings.TrimSpace(string(m.bufferNameDialog.Text))
			if name != "" {
				m.clipboard.SetSelectedName(m.clipboard.SelectedIdx, name)
			}
			m.bufferNameDialog = nil
		case "esc", "escape":
			m.bufferNameDialog = nil
		case "backspace":
			if m.bufferNameDialog.Cursor > 0 {
				m.bufferNameDialog.Text = append(m.bufferNameDialog.Text[:m.bufferNameDialog.Cursor-1], m.bufferNameDialog.Text[m.bufferNameDialog.Cursor:]...)
				m.bufferNameDialog.Cursor--
			}
		case "delete":
			if m.bufferNameDialog.Cursor < len(m.bufferNameDialog.Text) {
				m.bufferNameDialog.Text = append(m.bufferNameDialog.Text[:m.bufferNameDialog.Cursor], m.bufferNameDialog.Text[m.bufferNameDialog.Cursor+1:]...)
			}
		case "left":
			if m.bufferNameDialog.Cursor > 0 {
				m.bufferNameDialog.Cursor--
			}
		case "right":
			if m.bufferNameDialog.Cursor < len(m.bufferNameDialog.Text) {
				m.bufferNameDialog.Cursor++
			}
		case "home":
			m.bufferNameDialog.Cursor = 0
		case "end":
			m.bufferNameDialog.Cursor = len(m.bufferNameDialog.Text)
		default:
			k := tea.Key(msg)
			if k.Text != "" {
				for _, ch := range k.Text {
					m.bufferNameDialog.Text = append(m.bufferNameDialog.Text, 0)
					copy(m.bufferNameDialog.Text[m.bufferNameDialog.Cursor+1:], m.bufferNameDialog.Text[m.bufferNameDialog.Cursor:])
					m.bufferNameDialog.Text[m.bufferNameDialog.Cursor] = ch
					m.bufferNameDialog.Cursor++
				}
			}
		}
		return m, nil
	}

	if m.newWorkspaceDialog != nil {
		d := m.newWorkspaceDialog
		switch d.Cursor {
		case 0: // Name text field
			switch key {
			case "enter", "tab":
				d.Cursor = 1
			case "shift+tab":
				d.Cursor = 2
				d.Selected = 0
			case "esc", "escape":
				m.newWorkspaceDialog = nil
			case "left":
				if d.TextCursor > 0 {
					d.TextCursor--
				}
			case "right":
				if d.TextCursor < len(d.Name) {
					d.TextCursor++
				}
			case "home", "ctrl+a":
				d.TextCursor = 0
			case "end", "ctrl+e":
				d.TextCursor = len(d.Name)
			case "backspace":
				if d.TextCursor > 0 {
					d.Name = append(d.Name[:d.TextCursor-1], d.Name[d.TextCursor:]...)
					d.TextCursor--
				}
			case "delete":
				if d.TextCursor < len(d.Name) {
					d.Name = append(d.Name[:d.TextCursor], d.Name[d.TextCursor+1:]...)
				}
			case "ctrl+u":
				d.Name = d.Name[d.TextCursor:]
				d.TextCursor = 0
			default:
				k := tea.Key(msg)
				if k.Text != "" {
					for _, ch := range k.Text {
						d.Name = append(d.Name, 0)
						copy(d.Name[d.TextCursor+1:], d.Name[d.TextCursor:])
						d.Name[d.TextCursor] = ch
						d.TextCursor++
					}
				}
			}

		case 1: // Directory browser
			totalEntries := 1 + len(d.DirEntries) // ".." + entries
			switch key {
			case "up", "k":
				if d.DirSelect > 0 {
					d.DirSelect--
				}
				if d.DirSelect < d.DirScroll {
					d.DirScroll = d.DirSelect
				}
			case "down", "j":
				if d.DirSelect < totalEntries-1 {
					d.DirSelect++
				}
				if d.DirSelect >= d.DirScroll+dirBrowserVisibleRows {
					d.DirScroll = d.DirSelect - dirBrowserVisibleRows + 1
				}
			case "enter", "space":
				// Navigate into selected directory
				var target string
				if d.DirSelect == 0 {
					// ".." — go to parent
					target = filepath.Dir(d.DirPath)
				} else {
					target = filepath.Join(d.DirPath, d.DirEntries[d.DirSelect-1])
				}
				d.DirPath = target
				d.DirEntries = scanDirEntries(target)
				d.DirSelect = 0
				d.DirScroll = 0
			case "tab":
				d.Cursor = 2
				d.Selected = 0
			case "shift+tab":
				d.Cursor = 0
				d.TextCursor = len(d.Name)
			case "esc", "escape":
				m.newWorkspaceDialog = nil
			}

		case 2: // Buttons
			switch key {
			case "left", "right":
				if d.Selected == 0 {
					d.Selected = 1
				} else {
					d.Selected = 0
				}
			case "tab":
				d.Cursor = 0
				d.TextCursor = len(d.Name)
			case "shift+tab":
				d.Cursor = 1
			case "enter", "space":
				if d.Selected == 1 { // Cancel
					m.newWorkspaceDialog = nil
				} else { // Create
					name := string(d.Name)
					dirPath := d.DirPath
					if name != "" && dirPath != "" {
						wsFile := filepath.Join(dirPath, ".termdesk-workspace.toml")
						if _, err := os.Stat(wsFile); err == nil {
							m.newWorkspaceDialog = nil
							m.notifications.Push(
								"Workspace Exists",
								fmt.Sprintf("A workspace already exists at %s. Use the picker to load it.", filepath.Base(dirPath)),
								notification.Info,
							)
							return m, nil
						}
						m.createNewWorkspace(name, dirPath)
					}
					m.newWorkspaceDialog = nil
				}
			case "esc", "escape":
				m.newWorkspaceDialog = nil
			}
		}
		return m, nil
	}

	if m.renameDialog != nil {
		switch key {
		case "enter":
			if m.renameDialog.Selected == 1 { // Cancel
				m.renameDialog = nil
			} else { // OK
				if w := m.wm.WindowByID(m.renameDialog.WindowID); w != nil {
					newTitle := string(m.renameDialog.Text)
					if newTitle != "" {
						w.Title = newTitle
						w.TitleLocked = true // prevent VT title from overriding user rename
					}
				}
				m.renameDialog = nil
			}
		case "esc", "escape":
			m.renameDialog = nil
		case "tab", "shift+tab":
			if m.renameDialog.Selected == 0 {
				m.renameDialog.Selected = 1
			} else {
				m.renameDialog.Selected = 0
			}
		case "backspace":
			if m.renameDialog.Cursor > 0 {
				m.renameDialog.Text = append(m.renameDialog.Text[:m.renameDialog.Cursor-1], m.renameDialog.Text[m.renameDialog.Cursor:]...)
				m.renameDialog.Cursor--
			}
		case "delete":
			if m.renameDialog.Cursor < len(m.renameDialog.Text) {
				m.renameDialog.Text = append(m.renameDialog.Text[:m.renameDialog.Cursor], m.renameDialog.Text[m.renameDialog.Cursor+1:]...)
			}
		case "left":
			if m.renameDialog.Cursor > 0 {
				m.renameDialog.Cursor--
			}
		case "right":
			if m.renameDialog.Cursor < len(m.renameDialog.Text) {
				m.renameDialog.Cursor++
			}
		case "home", "ctrl+a":
			m.renameDialog.Cursor = 0
		case "end", "ctrl+e":
			m.renameDialog.Cursor = len(m.renameDialog.Text)
		case "ctrl+u":
			m.renameDialog.Text = m.renameDialog.Text[m.renameDialog.Cursor:]
			m.renameDialog.Cursor = 0
		default:
			k := tea.Key(msg)
			if k.Text != "" {
				for _, ch := range k.Text {
					m.renameDialog.Text = append(m.renameDialog.Text, 0)
					copy(m.renameDialog.Text[m.renameDialog.Cursor+1:], m.renameDialog.Text[m.renameDialog.Cursor:])
					m.renameDialog.Text[m.renameDialog.Cursor] = ch
					m.renameDialog.Cursor++
				}
			}
		}
		return m, nil
	}

	if m.exposeMode {
		switch key {
		case "esc", "escape":
			if m.exposeFilter != "" {
				m.exposeFilter = ""
				m.relayoutExpose()
				return m, tickAnimation()
			}
			m.exitExpose()
			m.inputMode = ModeNormal
			return m, tickAnimation()
		case "tab", "down":
			m.cycleExposeWindow(1)
			return m, tickAnimation()
		case "shift+tab", "up":
			m.cycleExposeWindow(-1)
			return m, tickAnimation()
		case "enter", "space":
			m.exitExpose()
			m.inputMode = ModeNormal
			return m, tickAnimation()
		case "backspace":
			if len(m.exposeFilter) > 0 {
				runes := []rune(m.exposeFilter)
				m.exposeFilter = string(runes[:len(runes)-1])
				m.relayoutExpose()
			}
			return m, tickAnimation()
		default:
			// Number keys select windows directly when no filter is active
			if m.exposeFilter == "" && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
				idx := int(key[0] - '1')
				m.selectExposeByIndex(idx)
				m.exitExpose()
				m.inputMode = ModeNormal
				return m, tickAnimation()
			}
			// Single printable character → append to filter
			runes := []rune(key)
			if len(runes) == 1 && runes[0] > 31 {
				m.exposeFilter += key
				m.relayoutExpose()
				return m, tickAnimation()
			}
		}
		return m, nil
	}

	// Tour overlay intercepts all input
	if m.tour.Active {
		return m.handleTourKey(key)
	}

	if m.contextMenu != nil && m.contextMenu.Visible {
		return m.handleContextMenuKey(key)
	}

	if m.clipboard.Visible {
		return m.handleClipboardKey(key)
	}

	if m.notifications.CenterVisible() {
		return m.handleNotificationCenterKey(key)
	}

	if m.settings.Visible {
		return m.handleSettingsKey(key)
	}

	if m.launcher.Visible {
		return m.handleLauncherKey(msg, key)
	}

	if m.menuBar.IsOpen() {
		return m.handleMenuKey(key)
	}

	// ── Layer 2: Prefix pending state (any mode) ──

	if m.prefixPending {
		m.prefixPending = false
		return m.handlePrefixAction(msg, key)
	}

	// ── Layer 3: Global hotkeys — only in non-Terminal modes ──

	if m.inputMode != ModeTerminal {
		switch key {
		case "ctrl+q":
			m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
			return m, nil
		case "f1":
			m.modal = m.helpOverlay()
			return m, nil
		case "f10":
			m.menuBar.OpenMenu(0)
			return m, nil
		case "f9":
			if m.exposeMode {
				m.exitExpose()
			} else {
				m.enterExpose()
			}
			return m, tickAnimation()
		}
	}

	// ── Layer 3.25: Workspace hotkeys (Alt+1..9) ──
	if m.inputMode != ModeTerminal {
		if idx, ok := parseAltNumber(key); ok {
			return m, m.loadRecentWorkspace(idx)
		}
	}

	// ── Layer 3.5: Global quick window switching (works in ALL modes) ──
	if key == m.keybindings.QuickNextWindow {
		if m.inputMode == ModeCopy {
			m.inputMode = ModeTerminal // switch to terminal mode for new window
		}
		m.wm.CycleForward()
		// If we cycled back to the copy window, restore copy mode
		if m.copySnapshot != nil {
			if fw := m.wm.FocusedWindow(); fw != nil {
				if fw.ID == m.copySnapshot.WindowID ||
					(fw.IsSplit() && fw.SplitRoot.FindLeaf(m.copySnapshot.WindowID) != nil) {
					m.inputMode = ModeCopy
				}
			}
		}
		return m, nil
	}
	if key == m.keybindings.QuickPrevWindow {
		if m.inputMode == ModeCopy {
			m.inputMode = ModeTerminal
		}
		m.wm.CycleBackward()
		if m.copySnapshot != nil {
			if fw := m.wm.FocusedWindow(); fw != nil {
				if fw.ID == m.copySnapshot.WindowID ||
					(fw.IsSplit() && fw.SplitRoot.FindLeaf(m.copySnapshot.WindowID) != nil) {
					m.inputMode = ModeCopy
				}
			}
		}
		return m, nil
	}

	// ── Layer 4: Mode-specific dispatch ──

	switch m.inputMode {
	case ModeTerminal:
		return m.handleTerminalModeKey(msg, key)
	case ModeCopy:
		return m.handleCopyModeKey(msg, key)
	default:
		return m.handleNormalModeKey(msg, key)
	}
}

// handleTerminalModeKey handles keys when in Terminal mode.
// Only the prefix key is intercepted; everything else goes to the terminal.
func (m Model) handleTerminalModeKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// Guard: if the focused window is minimized, exit terminal mode immediately.
	if fw := m.wm.FocusedWindow(); fw != nil && fw.Minimized {
		m.inputMode = ModeNormal
		return m, nil
	}

	// Prefix key → enter prefix pending state
	if key == m.keybindings.Prefix {
		m.prefixPending = true
		return m, nil
	}

	// Exited window: 'r' to restart, 'q'/'enter'/'space' to close
	if w, _ := m.focusedTerminal(); w != nil && w.Exited {
		switch strings.ToLower(key) {
		case "r":
			return m.restartExitedWindow(w.ID)
		case "q", "enter", "space":
			return m.closeExitedWindow(w.ID)
		}
		return m, nil // swallow other keys
	}

	// Forward everything else to the focused terminal
	if fw, term := m.focusedTerminal(); fw != nil && term != nil {
		k := tea.Key(msg)
		// Handle Home/End explicitly for terminals that don't map these reliably.
		if k.Code == tea.KeyHome || k.Code == tea.KeyKpHome {
			homeSeq, _ := homeEndSeq(term.ForegroundCommand())
			term.WriteInput([]byte(homeSeq))
			m.syncPanesForward(fw.ID, k)
			return m, nil
		}
		if k.Code == tea.KeyEnd || k.Code == tea.KeyKpEnd {
			_, endSeq := homeEndSeq(term.ForegroundCommand())
			term.WriteInput([]byte(endSeq))
			m.syncPanesForward(fw.ID, k)
			return m, nil
		}
		term.SendKey(k.Code, k.Mod, k.Text)
		m.syncPanesForward(fw.ID, k)
		return m, nil
	}

	// No terminal focused — switch to Normal mode (key is NOT reprocessed)
	m.inputMode = ModeNormal
	return m, nil
}

// syncPanesForward broadcasts a key to all visible terminals except the focused one.
func (m *Model) syncPanesForward(focusID string, k tea.Key) {
	if !m.syncPanes {
		return
	}
	for _, w := range m.wm.Windows() {
		if w.ID == focusID || !w.Visible || w.Minimized {
			continue
		}
		if t := m.terminals[w.ID]; t != nil {
			t.SendKey(k.Code, k.Mod, k.Text)
		}
	}
}

func isShellProcess(cmd string) bool {
	if cmd == "" {
		return true
	}
	base := filepath.Base(cmd)
	switch base {
	case "bash", "zsh", "fish", "sh", "dash", "ksh", "tcsh", "csh":
		return true
	default:
		return false
	}
}

func homeEndSeq(cmd string) (string, string) {
	if !isShellProcess(cmd) {
		return "\x1b[H", "\x1b[F"
	}
	base := filepath.Base(cmd)
	switch base {
	case "zsh", "tcsh", "csh":
		// zsh defaults often bind Home/End to SS3 ESC O H/F.
		return "\x1bOH", "\x1bOF"
	case "bash", "rbash":
		// readline bash defaults usually bind to CSI 1~/4~.
		return "\x1b[1~", "\x1b[4~"
	case "sh", "dash", "ksh":
		// Minimal shells often don't bind Home/End; use Ctrl+A/Ctrl+E.
		return "\x01", "\x05"
	default:
		// Fallback to common ANSI Home/End.
		return "\x1b[H", "\x1b[F"
	}
}

func parseAltNumber(key string) (int, bool) {
	if !strings.HasPrefix(key, "alt+") {
		return 0, false
	}
	rest := key[len("alt+"):]
	if len(rest) != 1 {
		return 0, false
	}
	ch := rest[0]
	if ch < '1' || ch > '9' {
		return 0, false
	}
	return int(ch - '1'), true
}

// handlePrefixAction dispatches an action after the prefix key was pressed.
func (m Model) handlePrefixAction(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// Double-prefix: send prefix key to terminal (like tmux Ctrl+b Ctrl+b)
	if key == m.keybindings.Prefix {
		if _, term := m.focusedTerminal(); term != nil {
			k := tea.Key(msg)
			term.SendKey(k.Code, k.Mod, k.Text)
		}
		return m, nil
	}

	// Esc, Tab, or Shift+Tab after prefix → exit terminal mode
	if key == "esc" || key == "escape" || key == "tab" || key == "shift+tab" {
		m.inputMode = ModeNormal
		return m, nil
	}

	// prefix+c → enter copy mode (scrollback navigation)
	if key == "c" {
		m.enterCopyModeForFocusedWindow()
		return m, nil
	}

	// prefix+y → show clipboard history (like tmux prefix+#)
	if key == "y" {
		m.clipboard.ShowHistory()
		return m, nil
	}

	// prefix+d → detach session (like tmux prefix+d)
	if key == "d" {
		os.Stdout.Write([]byte("\x1b]666;detach\x07"))
		return m, nil
	}

	// Split pane keys (tmux-style, prefix-gated)
	switch key {
	case "\"":
		cmd := m.splitPane(window.SplitVertical)
		return m, cmd
	case "%":
		cmd := m.splitPane(window.SplitHorizontal)
		return m, cmd
	case "x":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.IsSplit() {
			m.closeFocusedPane()
			return m, nil
		}
		// Fall through to actionMap for close_window
	case "o":
		m.focusNextPane()
		return m, nil
	case ";":
		m.focusPrevPane()
		return m, nil
	case "left", "right", "up", "down":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.IsSplit() {
			m.focusPaneInDirection("pane_" + key)
			return m, nil
		}
		// Fall through to actionMap for non-split windows
	}

	// Look up action
	action := m.actionMap[key]

	// Handle 1-9 for window-by-index
	if action == "" && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		idx := int(key[0]-'0') - 1
		windows := m.wm.Windows()
		if idx < len(windows) {
			m.wm.FocusWindow(windows[idx].ID)
		}
		return m, nil
	}

	// Unrecognized key — forward to terminal (prefix consumed, matches tmux behavior)
	if action == "" {
		if _, term := m.focusedTerminal(); term != nil {
			k := tea.Key(msg)
			term.SendKey(k.Code, k.Mod, k.Text)
		}
		return m, nil
	}

	// Execute the action
	return m.executeAction(action, msg, key)
}

// handleNormalModeKey handles keys when in Normal (window management) mode.
func (m Model) handleNormalModeKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// ── Menu bar navigation sub-mode ──
	if m.menuBarFocused {
		ret, cmd, handled := m.handleMenuBarFocusNav(key)
		if handled {
			return ret, cmd
		}
		m = ret.(Model)
		// Fall through to normal key handling below
	}

	// ── Dock navigation sub-mode ──
	if m.dockFocused {
		ret, cmd, handled := m.handleDockNav(key)
		if handled {
			return ret, cmd
		}
		m = ret.(Model)
		// Fall through to normal key handling below
	}

	// Prefix key works in Normal mode too — activates PREFIX badge
	if key == m.keybindings.Prefix {
		m.prefixPending = true
		return m, nil
	}

	// Esc in normal mode — exit dock/menubar focus or no-op
	if key == "esc" || key == "escape" {
		m.dockFocused = false
		m.menuBarFocused = false
		m.menuBar.FocusIndex = -1
		m.dock.SetHover(-1)
		return m, nil
	}

	// Tab / Shift+Tab — cycle focus: windows → dock → menu bar → windows
	if key == "tab" {
		return m.tabCycleForward()
	}
	if key == "shift+tab" {
		return m.tabCycleBackward()
	}

	// c → enter copy mode (scrollback navigation)
	if key == "c" {
		m.enterCopyModeForFocusedWindow()
		return m, nil
	}

	// Exited window: q/enter/space to close, r to restart
	if fw := m.wm.FocusedWindow(); fw != nil && fw.Exited {
		switch key {
		case "q", "enter", "space":
			return m.closeExitedWindow(fw.ID)
		case "r":
			return m.restartExitedWindow(fw.ID)
		}
	}

	// Focus window by number (1-9)
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		idx := int(key[0]-'0') - 1
		windows := m.wm.Windows()
		if idx < len(windows) {
			m.wm.FocusWindow(windows[idx].ID)
		}
		return m, nil
	}

	// Look up action in keymap
	action := m.actionMap[key]
	if action != "" {
		return m.executeAction(action, msg, key)
	}

	return m, nil
}

// tabCycleForward cycles Tab focus: windows → dock → menu bar → windows.
// Uses a stable window order (sorted by ID) so the cycle is deterministic.
// Direction changes reset the counter so Tab/Shift+Tab back-and-forth
// never accidentally transitions to dock or menu bar.
func (m Model) tabCycleForward() (tea.Model, tea.Cmd) {
	// Reset counter on direction change
	if m.tabCycleDir != 1 {
		m.tabCycleCount = 0
		m.tabCycleDir = 1
	}

	if m.menuBarFocused {
		// From menu bar → first window (or dock if no windows)
		m.menuBarFocused = false
		m.menuBar.FocusIndex = -1
		m.tabCycleCount = 0
		visible := m.wm.VisibleWindowsByID()
		if len(visible) > 0 {
			m.wm.FocusWindow(visible[0].ID)
			m.tabCycleCount = 1
		} else {
			m.enterDockFocus(0)
		}
		return m, nil
	}

	if m.dockFocused {
		// From dock → menu bar (first item)
		m.dockFocused = false
		m.dock.SetHover(-1)
		m.tooltipText = ""
		m.enterMenuBarFocus(0)
		return m, nil
	}

	visible := m.wm.VisibleWindowsByID()
	if len(visible) == 0 {
		m.enterDockFocus(0)
		return m, nil
	}

	m.tabCycleCount++
	if m.tabCycleCount > len(visible) {
		// Visited all windows → enter dock
		m.tabCycleCount = 0
		m.tabCycleDir = 0
		m.enterDockFocus(0)
		return m, nil
	}

	// Find current focused in stable order, advance to next
	next := m.nextWindowInStableOrder(visible, +1)
	m.wm.FocusWindow(next)
	return m, nil
}

// tabCycleBackward cycles Shift+Tab focus: windows → menu bar → dock → windows.
// Uses a stable window order (sorted by ID) for deterministic cycling.
func (m Model) tabCycleBackward() (tea.Model, tea.Cmd) {
	// Reset counter on direction change
	if m.tabCycleDir != -1 {
		m.tabCycleCount = 0
		m.tabCycleDir = -1
	}

	if m.menuBarFocused {
		// From menu bar → dock (first item)
		m.menuBarFocused = false
		m.menuBar.FocusIndex = -1
		m.enterDockFocus(0)
		return m, nil
	}

	if m.dockFocused {
		// From dock → last window (or menu bar if no windows)
		m.dockFocused = false
		m.dock.SetHover(-1)
		m.tooltipText = ""
		m.tabCycleCount = 0
		visible := m.wm.VisibleWindowsByID()
		if len(visible) > 0 {
			m.wm.FocusWindow(visible[len(visible)-1].ID)
			m.tabCycleCount = 1
		} else {
			m.enterMenuBarFocus(0)
		}
		return m, nil
	}

	visible := m.wm.VisibleWindowsByID()
	if len(visible) == 0 {
		m.enterMenuBarFocus(0)
		return m, nil
	}

	m.tabCycleCount++
	if m.tabCycleCount > len(visible) {
		// Visited all windows → enter menu bar
		m.tabCycleCount = 0
		m.tabCycleDir = 0
		m.enterMenuBarFocus(0)
		return m, nil
	}

	// Find current focused in stable order, go to previous
	prev := m.nextWindowInStableOrder(visible, -1)
	m.wm.FocusWindow(prev)
	return m, nil
}

// nextWindowInStableOrder finds the current focused window in a stable-sorted
// list and returns the ID of the window at offset +1 (forward) or -1 (backward).
func (m *Model) nextWindowInStableOrder(sorted []*window.Window, dir int) string {
	fw := m.wm.FocusedWindow()
	currentIdx := -1
	if fw != nil {
		for i, w := range sorted {
			if w.ID == fw.ID {
				currentIdx = i
				break
			}
		}
	}
	nextIdx := 0
	if currentIdx >= 0 {
		nextIdx = (currentIdx + dir + len(sorted)) % len(sorted)
	}
	return sorted[nextIdx].ID
}

// enterDockFocus activates dock keyboard navigation at the given item index.
func (m *Model) enterDockFocus(idx int) {
	m.dockFocused = true
	m.menuBarFocused = false
	if idx < 0 {
		idx = 0
	}
	if idx >= m.dock.ItemCount() {
		idx = m.dock.ItemCount() - 1
	}
	m.dock.SetHover(idx)
	if idx >= 0 && idx < len(m.dock.Items) {
		m.tooltipText = m.dock.Items[idx].TooltipText()
		m.tooltipX = m.dock.ItemCenterX(idx)
		m.tooltipY = m.height - 2
	}
}

// enterMenuBarFocus activates menu bar keyboard focus at the given label index.
func (m *Model) enterMenuBarFocus(idx int) {
	m.menuBarFocused = true
	m.dockFocused = false
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.menuBar.Menus) {
		idx = len(m.menuBar.Menus) - 1
	}
	m.menuBarFocusIdx = idx
	m.menuBar.FocusIndex = idx
	m.tooltipText = ""
}
