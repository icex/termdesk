package app

func (m *Model) enterCopyModeForWindow(windowID string) {
	m.scrollOffset = 0
	m.inputMode = ModeCopy
	m.copyCount = 0
	m.copyLastKey = ""
	m.copySearchActive = false
	m.copySearchQuery = ""
	m.copySearchDir = 0
	m.copySearchMatchCount = 0
	m.copySearchMatchIdx = 0
	if term := m.terminals[windowID]; term != nil {
		m.copySnapshot = captureCopySnapshot(windowID, term)
	} else {
		m.copySnapshot = nil
	}
	// Initialize cursor at center of visible content area.
	// windowID may be a pane terminal ID — find pane rect via parent window.
	contentH := 0
	if w := m.wm.WindowByID(windowID); w != nil {
		contentH = w.ContentRect().Height
	} else if pw := m.windowForTerminal(windowID); pw != nil {
		contentH = m.paneRectForTerm(pw, windowID).Height
	}
	if contentH > 0 {
		sbLen := 0
		if term := m.terminals[windowID]; term != nil {
			sbLen = term.ScrollbackLen()
		}
		if m.copySnapshot != nil {
			sbLen = m.copySnapshot.ScrollbackLen()
		}
		midRow := contentH / 2
		m.copyCursorY = mouseToAbsLine(midRow, 0, sbLen, contentH)
		m.copyCursorX = 0
	}
}

func (m *Model) enterCopyModeForFocusedWindow() {
	// If quake is visible, enter copy mode for quake terminal
	if m.quakeVisible && m.quakeTerminal != nil {
		m.enterCopyModeForQuake()
		return
	}
	if fw := m.wm.FocusedWindow(); fw != nil {
		termID := fw.ID
		if fw.IsSplit() && fw.FocusedPane != "" {
			termID = fw.FocusedPane
		}
		m.enterCopyModeForWindow(termID)
		return
	}
	m.scrollOffset = 0
	m.inputMode = ModeCopy
	m.copySnapshot = nil
}

// enterCopyModeForQuake enters copy mode for the quake dropdown terminal.
func (m *Model) enterCopyModeForQuake() {
	m.scrollOffset = 0
	m.inputMode = ModeCopy
	m.copyCount = 0
	m.copyLastKey = ""
	m.copySearchActive = false
	m.copySearchQuery = ""
	m.copySearchDir = 0
	m.copySearchMatchCount = 0
	m.copySearchMatchIdx = 0
	if m.quakeTerminal != nil {
		m.copySnapshot = captureCopySnapshot(quakeTermID, m.quakeTerminal)
	} else {
		m.copySnapshot = nil
	}
	contentH := int(m.quakeAnimH) - 1
	if contentH < 1 {
		contentH = 1
	}
	sbLen := 0
	if m.quakeTerminal != nil {
		sbLen = m.quakeTerminal.ScrollbackLen()
	}
	if m.copySnapshot != nil {
		sbLen = m.copySnapshot.ScrollbackLen()
	}
	midRow := contentH / 2
	m.copyCursorY = mouseToAbsLine(midRow, 0, sbLen, contentH)
	m.copyCursorX = 0
}

// exitCopyMode cleanly exits copy mode, resetting all related state.
// Sets inputMode to ModeNormal as a safe default. Callers that need a
// different target mode (e.g. ModeTerminal) must override after calling.
func (m *Model) exitCopyMode() {
	m.scrollOffset = 0
	m.selActive = false
	m.inputMode = ModeNormal
	m.copySnapshot = nil
	m.copySearchActive = false
	m.copySearchQuery = ""
	m.copySearchDir = 0
	m.copySearchMatchCount = 0
	m.copySearchMatchIdx = 0
	m.copyCount = 0
	m.copyLastKey = ""
	m.copyCursorX = 0
	m.copyCursorY = 0
}

func (m *Model) clearCopySearch() {
	m.copySearchActive = false
	m.copySearchQuery = ""
	m.copySearchDir = 0
	m.copySearchMatchCount = 0
	m.copySearchMatchIdx = 0
}

func (m *Model) copySnapshotForWindow(windowID string) *CopySnapshot {
	if m.copySnapshot != nil && m.copySnapshot.WindowID == windowID {
		return m.copySnapshot
	}
	return nil
}

// ensureCursorVisible adjusts scrollOffset so the copy mode cursor is within the viewport.
func (m *Model) ensureCursorVisible(sbLen, contentH, maxScroll int) {
	row := absLineToContentRow(m.copyCursorY, m.scrollOffset, sbLen, contentH)
	if row < 0 {
		// Cursor is above viewport — scroll up
		// Set scroll so cursor is at row 0
		if m.copyCursorY < sbLen {
			m.scrollOffset = sbLen - m.copyCursorY
		} else {
			m.scrollOffset = 0
		}
	} else if row >= contentH {
		// Cursor is below viewport — scroll down
		diff := row - contentH + 1
		m.scrollOffset -= diff
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

