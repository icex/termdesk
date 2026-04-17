package app

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// splitPane splits the focused pane in the given direction.
func (m *Model) splitPane(dir window.SplitDir) tea.Cmd {
	fw := m.wm.FocusedWindow()
	if fw == nil || fw.Minimized || fw.Exited {
		return nil
	}
	if m.totalTerminalCount() >= maxWindows {
		return nil
	}

	// Determine which terminal to split
	currentTermID := fw.ID
	if fw.IsSplit() && fw.FocusedPane != "" {
		currentTermID = fw.FocusedPane
	}

	// Generate ID for the new pane terminal
	newTermID := newWindowID()

	if fw.SplitRoot == nil {
		// First split: create tree with existing terminal and new terminal
		fw.SplitRoot = &window.SplitNode{
			Dir:   dir,
			Ratio: 0.5,
			Children: [2]*window.SplitNode{
				{TermID: currentTermID},
				{TermID: newTermID},
			},
		}
		fw.FocusedPane = currentTermID
	} else {
		// Recursive split: replace the focused leaf
		newNode := &window.SplitNode{
			Dir:   dir,
			Ratio: 0.5,
			Children: [2]*window.SplitNode{
				{TermID: currentTermID},
				{TermID: newTermID},
			},
		}
		fw.SplitRoot.ReplaceLeaf(currentTermID, newNode)
	}

	// Compute pane layouts to size terminals
	cr := fw.SplitContentRect()
	panes := fw.SplitRoot.Layout(cr)

	// Find the new pane's rect for terminal sizing
	var newRect geometry.Rect
	for _, p := range panes {
		if p.TermID == newTermID {
			newRect = p.Rect
			break
		}
	}
	if newRect.Width <= 0 || newRect.Height <= 0 {
		// Area too small to split — revert
		fw.SplitRoot = nil
		fw.FocusedPane = ""
		return nil
	}

	// Create the new terminal
	var graphicsEnv []string
	if m.kittyPass != nil && m.kittyPass.IsEnabled() {
		if terminal.HasKittyTerminfo() {
			graphicsEnv = append(graphicsEnv, "TERM=xterm-kitty")
		}
		tp := terminal.HostTermProgram()
		if tp != "" {
			graphicsEnv = append(graphicsEnv, "TERM_PROGRAM="+tp)
		} else {
			graphicsEnv = append(graphicsEnv, "TERM_PROGRAM=WezTerm")
		}
	}
	if m.imagePass != nil && m.imagePass.Iterm2Enabled() {
		graphicsEnv = append(graphicsEnv, "LC_TERMINAL=iTerm2")
		graphicsEnv = append(graphicsEnv, "ITERM_SESSION_ID=termdesk-"+newTermID)
	}
	workDir := fw.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	term, err := terminal.NewShell(newRect.Width, newRect.Height,
		m.cellPixelW, m.cellPixelH, workDir, graphicsEnv...)
	if err != nil {
		// Revert split
		fw.SplitRoot = nil
		fw.FocusedPane = ""
		return nil
	}

	c := m.theme.C()
	term.SetDefaultColors(c.DefaultFg, c.ContentBg)
	m.terminals[newTermID] = term
	m.termCreatedAt[newTermID] = time.Now()
	m.spawnPTYReader(newTermID, term)

	// Resize the existing terminal to its new (smaller) pane rect
	for _, p := range panes {
		if p.TermID == currentTermID {
			if t := m.terminals[currentTermID]; t != nil {
				t.Resize(p.Rect.Width, p.Rect.Height)
			}
			break
		}
	}

	// Focus the new pane
	fw.FocusedPane = newTermID

	// Enter terminal mode if not already
	if m.inputMode == ModeNormal {
		m.inputMode = ModeTerminal
	}

	return nil
}

// closeFocusedPane closes the currently focused pane in a split window.
func (m *Model) closeFocusedPane() {
	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		return
	}

	termID := fw.FocusedPane
	if termID == "" {
		return
	}

	// Close the terminal
	m.closeTerminal(termID)

	// Remove the leaf from the tree
	newRoot := fw.SplitRoot.RemoveLeaf(termID)

	if newRoot == nil {
		// All panes gone — close the window
		m.wm.RemoveWindow(fw.ID)
		m.closeTerminal(fw.ID)
		m.inputMode = ModeNormal
		return
	}

	if newRoot.IsLeaf() {
		// Revert to single-terminal mode
		remainingID := newRoot.TermID
		if remainingID != fw.ID {
			// Remap: the remaining pane terminal must be keyed by window ID
			// for compatibility with all existing code paths
			m.terminals[fw.ID] = m.terminals[remainingID]
			delete(m.terminals, remainingID)
			delete(m.termCreatedAt, remainingID)
			delete(m.termHasOutput, remainingID)
			// Track redirect so PtyClosedMsg from old goroutine finds the window
			m.paneRedirect[remainingID] = fw.ID
			// Update any active copy snapshot
			if m.copySnapshot != nil && m.copySnapshot.WindowID == remainingID {
				m.copySnapshot.WindowID = fw.ID
			}
		}
		fw.SplitRoot = nil
		fw.FocusedPane = ""
		m.resizeTerminalForWindow(fw)
		return
	}

	// Multiple panes remain
	fw.SplitRoot = newRoot
	// Focus the first remaining pane
	ids := newRoot.AllTermIDs()
	if len(ids) > 0 {
		fw.FocusedPane = ids[0]
	}
	m.resizeAllPanes(fw)
}

// focusNextPane cycles focus to the next pane in the split window.
func (m *Model) focusNextPane() {
	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		return
	}
	ids := fw.SplitRoot.AllTermIDs()
	if len(ids) < 2 {
		return
	}
	current := fw.FocusedPane
	for i, id := range ids {
		if id == current {
			fw.FocusedPane = ids[(i+1)%len(ids)]
			return
		}
	}
	fw.FocusedPane = ids[0]
}

// focusPrevPane cycles focus to the previous pane in the split window.
func (m *Model) focusPrevPane() {
	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		return
	}
	ids := fw.SplitRoot.AllTermIDs()
	if len(ids) < 2 {
		return
	}
	current := fw.FocusedPane
	for i, id := range ids {
		if id == current {
			fw.FocusedPane = ids[(i-1+len(ids))%len(ids)]
			return
		}
	}
	fw.FocusedPane = ids[0]
}

// focusPaneInDirection moves focus to the pane in the given direction.
func (m *Model) focusPaneInDirection(action string) {
	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.IsSplit() {
		return
	}
	cr := fw.SplitContentRect()
	panes := fw.SplitRoot.Layout(cr)
	if len(panes) < 2 {
		return
	}

	// Find current pane's rect center
	var curRect geometry.Rect
	for _, p := range panes {
		if p.TermID == fw.FocusedPane {
			curRect = p.Rect
			break
		}
	}
	if curRect.Width == 0 {
		return
	}
	cx := curRect.X + curRect.Width/2
	cy := curRect.Y + curRect.Height/2

	// Find the best candidate in the given direction
	bestID := ""
	bestDist := 1<<31 - 1
	for _, p := range panes {
		if p.TermID == fw.FocusedPane {
			continue
		}
		px := p.Rect.X + p.Rect.Width/2
		py := p.Rect.Y + p.Rect.Height/2
		dx := px - cx
		dy := py - cy

		match := false
		switch action {
		case "pane_left":
			match = dx < 0
		case "pane_right":
			match = dx > 0
		case "pane_up":
			match = dy < 0
		case "pane_down":
			match = dy > 0
		}
		if !match {
			continue
		}
		dist := dx*dx + dy*dy
		if dist < bestDist {
			bestDist = dist
			bestID = p.TermID
		}
	}
	if bestID != "" {
		fw.FocusedPane = bestID
	}
}

// resizeAllPanes resizes all pane terminals in a split window to match their layout rects.
func (m *Model) resizeAllPanes(w *window.Window) {
	if !w.IsSplit() {
		return
	}
	cr := w.SplitContentRect()
	panes := w.SplitRoot.Layout(cr)
	for _, p := range panes {
		if t := m.terminals[p.TermID]; t != nil {
			if p.Rect.Width > 0 && p.Rect.Height > 0 &&
				(t.Width() != p.Rect.Width || t.Height() != p.Rect.Height) {
				t.Resize(p.Rect.Width, p.Rect.Height)
			}
		}
	}
}

// totalTerminalCount returns the total number of terminals (including split panes).
func (m *Model) totalTerminalCount() int {
	count := 0
	for _, w := range m.wm.Windows() {
		if w.IsSplit() {
			count += w.SplitRoot.PaneCount()
		} else {
			count++
		}
	}
	return count
}

// windowForTerminal finds the window that owns a terminal (by terminal ID).
// For unsplit windows, termID == windowID. For split panes, searches the split tree.
func (m *Model) windowForTerminal(termID string) *window.Window {
	if w := m.wm.WindowByID(termID); w != nil {
		return w
	}
	for _, w := range m.wm.Windows() {
		if w.IsSplit() && w.SplitRoot.FindLeaf(termID) != nil {
			return w
		}
	}
	return nil
}

// paneRectForTerm returns the screen rect for a pane terminal in a split window.
func (m *Model) paneRectForTerm(w *window.Window, termID string) geometry.Rect {
	if !w.IsSplit() {
		return w.ContentRect()
	}
	return w.SplitRoot.PaneRectForTerm(termID, w.SplitContentRect())
}
