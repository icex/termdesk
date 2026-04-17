package app

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func (m Model) handleMouseClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	m.tabCycleCount = 0 // mouse click breaks tab cycle
	p := geometry.Point{X: mouse.X, Y: mouse.Y}

	// If modal is showing, handle tab clicks or dismiss
	if m.modal != nil {
		bounds := m.modal.Bounds(m.width, m.height)
		if mouse.X >= bounds.StartX && mouse.X < bounds.StartX+bounds.BoxW &&
			mouse.Y >= bounds.StartY && mouse.Y < bounds.StartY+bounds.BoxH {
			// Click inside modal — check for tab click
			if m.modal.Tabs != nil && len(m.modal.Tabs) > 0 {
				if mouse.Y == bounds.TabRow {
					// Compute which tab was clicked
					relX := mouse.X - bounds.StartX - 1 - bounds.HPad // -1 border, -hPad padding
					tabIdx := m.modal.TabAtX(relX)
					if tabIdx >= 0 && tabIdx < len(m.modal.Tabs) {
						m.modal.ActiveTab = tabIdx
						m.modal.ScrollY = 0
					}
					return m, nil
				}
			}
			// Click inside modal but not on a tab — absorb click (don't dismiss)
			return m, nil
		}
		// Click outside modal dismisses it
		m.modal = nil
		return m, nil
	}

	// Quake terminal absorbs clicks in its area
	if m.quakeVisible && m.quakeTerminal != nil && !m.quakeAnimating() {
		h := int(m.quakeAnimH)
		borderY := h - 1
		// Click on the bottom border starts drag resize
		if mouse.Y == borderY && borderY >= 0 {
			m.quakeDragActive = true
			m.quakeDragStartY = mouse.Y
			m.quakeDragStartH = h
			return m, nil
		}
		// Click inside content area
		if mouse.Y < borderY {
			isShift := mouse.Mod&tea.ModShift != 0
			if m.inputMode == ModeCopy || isShift {
				// Shift+click or click in copy mode: WM text selection
				if m.inputMode != ModeCopy {
					m.enterCopyModeForQuake()
				}
				contentH := borderY
				if contentH < 1 {
					contentH = 1
				}
				sbLen := m.quakeTerminal.ScrollbackLen()
				if snap := m.copySnapshotForWindow(quakeTermID); snap != nil {
					sbLen = snap.ScrollbackLen()
				}
				absLine := mouseToAbsLine(mouse.Y, m.scrollOffset, sbLen, contentH)
				m.selStart = geometry.Point{X: mouse.X, Y: absLine}
				m.selEnd = m.selStart
				m.copyCursorX = mouse.X
				m.copyCursorY = absLine
				m.selActive = true
				m.selDragging = true
			} else if m.quakeTerminal.HasMouseMode() {
				btn := teaToUvButton(mouse.Button)
				m.quakeTerminal.SendMouse(btn, mouse.X, mouse.Y, false)
			}
			return m, nil
		}
	}

	// Context menu: any click outside dismisses, click on item selects
	if m.contextMenu != nil && m.contextMenu.Visible {
		cm := m.contextMenu
		if cm.Contains(mouse.X, mouse.Y) {
			relY := mouse.Y - cm.Y - 1 // -1 for top border
			idx := cm.ItemAtY(relY)
			if idx >= 0 && idx < len(cm.Items) && !cm.Items[idx].Disabled {
				action := cm.Items[idx].Action
				cm.Hide()
				if action != "" {
					return m.executeMenuAction(action)
				}
			}
			return m, nil
		}
		cm.Hide()
		return m, nil
	}

	// Right-click: context menu depends on mode
	if mouse.Button == tea.MouseRight {
		// Copy mode: show copy/paste context menu
		if m.inputMode == ModeCopy {
			m.contextMenu = contextmenu.CopyModeMenu(mouse.X, mouse.Y)
			m.contextMenu.Clamp(m.width, m.height)
			return m, nil
		}
		if m.inputMode == ModeTerminal {
			if w := m.wm.FocusedWindow(); w != nil {
				if term, ok := m.terminals[w.ID]; ok {
					cr := w.ContentRect()
					if mouse.X >= cr.X && mouse.X < cr.X+cr.Width && mouse.Y >= cr.Y && mouse.Y < cr.Y+cr.Height {
						term.SendMouse(uv.MouseRight, mouse.X-cr.X, mouse.Y-cr.Y, false)
						return m, nil
					}
				}
			}
		}
		return m.handleRightClick(mouse)
	}

	// If confirm dialog is showing, check for button clicks
	if m.confirmClose != nil {
		// Match lipgloss dialog dimensions: innerW + 2 (border), 5 rows (border+title+sep+buttons+border)
		innerW := runeLen(m.confirmClose.Title) + 4
		if innerW < 26 {
			innerW = 26
		}
		boxW := innerW + 2 // +2 for rounded border
		boxH := 5
		startX := (m.width - boxW) / 2
		startY := (m.height - boxH) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		btnY := startY + 3 // row 3 = buttons (0=border, 1=title, 2=sep, 3=buttons)

		if mouse.Y == btnY && mouse.X > startX && mouse.X < startX+boxW-1 {
			midX := startX + boxW/2
			if mouse.X < midX {
				m.confirmClose.Selected = 0
				return m.confirmAccept()
			} else {
				m.confirmClose.Selected = 1
				m.confirmClose = nil
				return m, nil
			}
		}
		// Click outside the dialog dismisses it
		if mouse.X < startX || mouse.X >= startX+boxW || mouse.Y < startY || mouse.Y >= startY+boxH {
			m.confirmClose = nil
		}
		return m, nil
	}

	// Workspace picker: click on item loads it, click outside dismisses
	if m.workspacePickerVisible {
		bx, by, bw, bh := m.workspacePickerBounds()
		if mouse.X >= bx && mouse.X < bx+bw && mouse.Y >= by && mouse.Y < by+bh {
			itemY := mouse.Y - by - 2 // title(1) + border(1) = items start at relY=2
			if itemY >= 0 && itemY < len(m.workspaceList) {
				m.workspacePickerSelected = itemY
				m.workspacePickerVisible = false
				return m, m.loadSelectedWorkspace()
			}
			return m, nil
		}
		m.workspacePickerVisible = false
		return m, nil
	}

	// Settings panel mouse handling
	if m.settings.Visible {
		return m.handleSettingsClick(mouse)
	}

	// If exposé mode, click selects a window and maximizes it
	if m.exposeMode {
		if mouse.Y != m.height-1 && mouse.Y != 0 { // not dock or menubar
			m.selectExposeWindow(mouse.X, mouse.Y)
			m.exitExpose()
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()
	}

	// Launcher: click inside selects item, click outside dismisses
	if m.launcher.Visible {
		lx, ly, lw, lh := m.launcherBounds()
		if mouse.X >= lx && mouse.X < lx+lw && mouse.Y >= ly && mouse.Y < ly+lh {
			// Click inside launcher — check if on a result item
			idx := m.launcherResultIdx(mouse.Y, ly)
			if idx >= 0 {
				m.launcher.SelectedIdx = idx
				entry := m.launcher.SelectedEntry()
				m.launcher.Hide()
				if entry != nil {
					if entry.Source == "registry" {
						return m.launchAppFromRegistry(entry.Command)
					}
					return m.launchExecEntry(entry.Command, nil)
				}
			}
			return m, nil
		}
		m.launcher.Hide()
		return m, nil
	}

	// Check if click is on dock (bottom row) — skip when dock is hidden
	dockHidden := m.hideDockWhenMaximized && m.hasMaximizedWindow()
	if mouse.Y == m.height-1 && !dockHidden {
		idx := m.dock.ItemAtX(mouse.X)
		if idx < 0 || idx >= len(m.dock.Items) {
			// Clicked empty dock area — no mode change, just absorb
			return m, nil
		}
		if m.inputMode == ModeTerminal {
			m.inputMode = ModeNormal
		}
		item := m.dock.Items[idx]
		// Handle special dock items
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
			if w := m.wm.WindowByID(item.WindowID); w != nil {
				m.restoreMinimizedWindow(w)
				return m, tickAnimation()
			}
			return m, nil
		case "running":
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
		}
		// Shortcut — always launch (focus existing for non-shell apps)
		m.inputMode = ModeNormal
		m.startDockPulse(idx)
		mdl, cmd := m.focusOrLaunchApp(item.Command)
		m = mdl.(Model)
		return m, tea.Batch(cmd, tickAnimation())
	}

	// Check if click is on menu bar (y=0)
	if mouse.Y == 0 {
		if m.inputMode == ModeTerminal {
			m.inputMode = ModeNormal
		}
		// Check right-side zones first (clock, CPU, MEM, mode badge).
		// Use DisplayWidth to account for Nerd Font icons rendering 2 cells.
		mLabel := modeBadge(m.inputMode)
		if m.prefixPending {
			mLabel = " \uf11c PREFIX   "
		}
		mLabelW := widget.DisplayWidth(mLabel)
		effW := m.width - mLabelW
		if effW < 1 {
			effW = 1
		}
		// Check widget zones first, then mode badge = everything to the right
		zones := m.menuBar.RightZones(effW)
		for _, zone := range zones {
			if mouse.X >= zone.Start && mouse.X < zone.End {
				return m.handleMenuBarRightClick(zone.Type)
			}
		}
		// Mode badge: starts exactly where the badge is rendered
		modeStart := m.width - mLabelW
		if modeStart < 0 {
			modeStart = 0
		}
		if mouse.X >= modeStart {
			if m.inputMode == ModeTerminal {
				m.inputMode = ModeNormal
			} else {
				m.inputMode = ModeTerminal
			}
			return m, nil
		}
		idx := m.menuBar.MenuAtX(mouse.X)
		if idx >= 0 {
			if m.menuBar.IsOpen() && m.menuBar.OpenIndex == idx {
				m.menuBar.CloseMenu()
			} else {
				m.menuBar.OpenMenu(idx)
			}
		} else {
			m.menuBar.CloseMenu()
		}
		return m, nil
	}

	// Check if click is on open dropdown or submenu
	if m.menuBar.IsOpen() {
		positions := m.menuBar.MenuXPositions()
		dropX := positions[m.menuBar.OpenIndex]
		dropY := 1 // dropdown starts at row 1
		menu := m.menuBar.Menus[m.menuBar.OpenIndex]
		dropW, _ := m.menuBar.DropdownDimensions()

		// Check submenu click first (if visible)
		if m.menuBar.HasSubMenu() {
			parentItemY, parentWidth := m.menuBar.SubMenuParentIndex()
			subX := dropX + parentWidth
			subY := dropY + parentItemY + 1 // +1 for top border
			subItemIdx := m.menuBar.SubMenuItemAtY(mouse.Y - subY - 1)
			subW, _ := m.menuBar.SubMenuDimensions()
			if mouse.X >= subX && mouse.X < subX+subW && subItemIdx >= 0 {
				hovItem := menu.Items[m.menuBar.HoverIndex]
				if !hovItem.SubItems[subItemIdx].Disabled {
					m.menuBar.InSubMenu = true
					m.menuBar.SubHoverIndex = subItemIdx
					action := m.menuBar.SelectedAction()
					m.menuBar.CloseMenu()
					return m.executeMenuAction(action)
				}
				return m, nil
			}
		}

		itemIdx := m.menuBar.DropdownItemAtY(mouse.Y - dropY - 1) // -1 for top border
		if mouse.X >= dropX && mouse.X < dropX+dropW && itemIdx >= 0 {
			m.menuBar.HoverIndex = itemIdx
			item := menu.Items[itemIdx]
			// If clicking on a submenu parent, open the submenu
			if len(item.SubItems) > 0 {
				m.menuBar.EnterSubMenu()
				return m, nil
			}
			action := m.menuBar.SelectedAction()
			m.menuBar.CloseMenu()
			return m.executeMenuAction(action)
		}
		// Click outside dropdown closes it
		m.menuBar.CloseMenu()
	}

	// Clicking on toast notifications opens notification center
	if toasts := m.notifications.VisibleToasts(); len(toasts) > 0 {
		toastW := 44
		if toastW > m.width-4 {
			toastW = m.width - 4
		}
		toastX := m.width - toastW - 1
		toastY := 2
		toastEndY := toastY + len(toasts)*5 // each toast ~4 rows + 1 gap
		if mouse.X >= toastX && mouse.Y >= toastY && mouse.Y < toastEndY {
			m.notifications.ShowCenter()
			return m, nil
		}
	}

	// Find which window was clicked
	w := m.wm.WindowAt(p)
	if w == nil {
		// Clicked on desktop background
		if m.inputMode == ModeTerminal {
			m.inputMode = ModeNormal
		} else if m.inputMode == ModeCopy {
			m.exitCopyMode()
			m.inputMode = ModeNormal
		}
		return m, nil
	}

	// When clicking a different window while in copy mode, exit copy mode entirely.
	// This removes the scrollbar and overlays from the previous window.
	if m.inputMode == ModeCopy && m.copySnapshot != nil {
		copyWinID := m.copySnapshot.WindowID
		sameWindow := w.ID == copyWinID ||
			(w.IsSplit() && w.SplitRoot.FindLeaf(copyWinID) != nil)
		if !sameWindow {
			m.exitCopyMode()
		}
	}

	// Focus the clicked window
	m.wm.FocusWindow(w.ID)

	// If we clicked back on the copy window while snapshot is active, restore copy mode.
	if m.copySnapshot != nil && m.inputMode != ModeCopy {
		copyWinID := m.copySnapshot.WindowID
		sameWindow := w.ID == copyWinID ||
			(w.IsSplit() && w.SplitRoot.FindLeaf(copyWinID) != nil)
		if sameWindow {
			m.inputMode = ModeCopy
		}
	}

	// Determine what was clicked
	zone := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)

	// Exit copy mode before handling titlebar buttons. These bypass
	// executeAction(), so the copy mode exit there doesn't apply.
	if m.inputMode == ModeCopy && zone != window.HitContent && zone != window.HitTitleBar {
		m.exitCopyMode()
		m.inputMode = ModeNormal
	}

	switch zone {
	case window.HitCloseButton:
		if w.Exited {
			return m.closeExitedWindow(w.ID)
		}
		m.confirmClose = &ConfirmDialog{
			WindowID: w.ID,
			Title:    fmt.Sprintf("Close \"%s\"?", w.Title),
		}
		return m, nil

	case window.HitMinButton:
		m.minimizeWindow(w)
		return m, tickAnimation()

	case window.HitMaxButton:
		if w.Resizable {
			if !w.IsMaximized() {
				m.disablePersistentTiling()
			}
			from := w.Rect
			window.ToggleMaximize(w, m.wm.WorkArea())
			m.startWindowAnimation(w.ID, AnimMaximize, from, w.Rect)
			m.resizeTerminalForWindow(w)
		}
		return m, tickAnimation()

	case window.HitSnapLeftButton:
		if w.Resizable {
			from := w.Rect
			window.SnapLeft(w, m.wm.WorkArea())
			m.startWindowAnimation(w.ID, AnimSnap, from, w.Rect)
			m.resizeTerminalForWindow(w)
		}
		return m, tickAnimation()

	case window.HitSnapRightButton:
		if w.Resizable {
			from := w.Rect
			window.SnapRight(w, m.wm.WorkArea())
			m.startWindowAnimation(w.ID, AnimSnap, from, w.Rect)
			m.resizeTerminalForWindow(w)
		}
		return m, tickAnimation()

	case window.HitContent:
		isShift := mouse.Mod&tea.ModShift != 0
		if w.IsSplit() {
			// Split window: check separators first, then find pane
			cr := w.SplitContentRect()
			seps := w.SplitRoot.Separators(cr)
			for _, sep := range seps {
				if sep.Rect.Contains(p) {
					// Start separator drag
					m.drag = window.DragState{
						Active:     true,
						WindowID:   w.ID,
						Mode:       window.DragSeparator,
						StartMouse: p,
						StartRect:  w.Rect,
						SepNode:    sep.Node,
						SepDir:     sep.Dir,
					}
					return m, nil
				}
			}
			// Find which pane was clicked
			panes := w.SplitRoot.Layout(cr)
			for _, pane := range panes {
				if pane.Rect.Contains(p) {
					w.FocusedPane = pane.TermID
					if m.inputMode == ModeCopy || isShift {
						if term := m.terminals[pane.TermID]; term != nil {
							if m.inputMode != ModeCopy {
								m.enterCopyModeForWindow(pane.TermID)
							}
							localX := mouse.X - pane.Rect.X
							localY := mouse.Y - pane.Rect.Y
							sbLen := term.ScrollbackLen()
							if snap := m.copySnapshotForWindow(pane.TermID); snap != nil {
								sbLen = snap.ScrollbackLen()
							}
							absLine := mouseToAbsLine(localY, m.scrollOffset, sbLen, pane.Rect.Height)
							m.selStart = geometry.Point{X: localX, Y: absLine}
							m.selEnd = m.selStart
							m.copyCursorX = localX
							m.copyCursorY = absLine
							m.selActive = true
							m.selDragging = true
						}
					} else {
						m.inputMode = ModeTerminal
						if term := m.terminals[pane.TermID]; term != nil {
							localX := mouse.X - pane.Rect.X
							localY := mouse.Y - pane.Rect.Y
							btn := teaToUvButton(mouse.Button)
							term.SendMouse(btn, localX, localY, false)
						}
					}
					break
				}
			}
		} else if m.inputMode == ModeCopy || isShift {
			// Copy mode click or shift+click: WM text selection.
			// Shift+click enters copy mode from any mode (like tmux).
			if term, ok := m.terminals[w.ID]; ok {
				if m.inputMode != ModeCopy {
					m.enterCopyModeForWindow(w.ID)
				}
				contentRect := w.ContentRect()
				localX := mouse.X - contentRect.X
				localY := mouse.Y - contentRect.Y
				sbLen := term.ScrollbackLen()
				if snap := m.copySnapshotForWindow(w.ID); snap != nil {
					sbLen = snap.ScrollbackLen()
				}
				absLine := mouseToAbsLine(localY, m.scrollOffset, sbLen, contentRect.Height)
				m.selStart = geometry.Point{X: localX, Y: absLine}
				m.selEnd = m.selStart
				m.copyCursorX = localX
				m.copyCursorY = absLine
				m.selActive = true
				m.selDragging = true
			}
		} else {
			// Normal click: forward to terminal (let nvim handle its own selection).
			if _, ok := m.terminals[w.ID]; ok {
				m.inputMode = ModeTerminal
			}
			if term, ok := m.terminals[w.ID]; ok {
				contentRect := w.ContentRect()
				localX := mouse.X - contentRect.X // 0-indexed
				localY := mouse.Y - contentRect.Y
				btn := teaToUvButton(mouse.Button)
				term.SendMouse(btn, localX, localY, false)
			}
		}

	case window.HitTitleBar, window.HitBorderN, window.HitBorderS, window.HitBorderE,
		window.HitBorderW, window.HitBorderNE, window.HitBorderNW,
		window.HitBorderSE, window.HitBorderSW:
		dragMode := window.DragModeForZone(zone)
		canDrag := false
		if dragMode == window.DragMove {
			canDrag = w.Draggable
		} else if dragMode != window.DragNone {
			canDrag = w.Resizable
		}
		if canDrag {
			m.drag = window.DragState{
				Active:     true,
				WindowID:   w.ID,
				Mode:       dragMode,
				StartMouse: p,
				StartRect:  w.Rect,
			}
		}
	}

	return m, nil
}

func (m Model) handleMouseMotion(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	// Quake bottom border drag resize
	if m.quakeDragActive && m.quakeTerminal != nil {
		delta := mouse.Y - m.quakeDragStartY
		newH := m.quakeDragStartH + delta
		if newH < 5 {
			newH = 5
		}
		maxH := m.height * 9 / 10
		if newH > maxH {
			newH = maxH
		}
		m.quakeAnimH = float64(newH)
		m.quakeTargetH = float64(newH)
		m.quakeAnimVel = 0
		// Update percentage to match
		m.quakeHeightPct = newH * 100 / m.height
		if m.quakeHeightPct < 10 {
			m.quakeHeightPct = 10
		}
		if m.quakeHeightPct > 90 {
			m.quakeHeightPct = 90
		}
		return m, nil
	}

	// Copy mode: extend selection on drag
	if m.inputMode == ModeCopy && m.selDragging {
		// Quake terminal drag selection
		if m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID && m.quakeTerminal != nil {
			contentH := int(m.quakeAnimH) - 1
			if contentH < 1 {
				contentH = 1
			}
			localX := mouse.X
			localY := mouse.Y
			termW := m.quakeTerminal.Width()
			sbLen := m.quakeTerminal.ScrollbackLen()
			if snap := m.copySnapshotForWindow(quakeTermID); snap != nil {
				termW = snap.Width
				sbLen = snap.ScrollbackLen()
			}
			if localX < 0 {
				localX = 0
			}
			if localX >= termW {
				localX = termW - 1
			}
			if localY < 0 {
				localY = 0
			}
			if localY >= contentH {
				localY = contentH - 1
			}
			absLine := mouseToAbsLine(localY, m.scrollOffset, sbLen, contentH)
			m.selEnd = geometry.Point{X: localX, Y: absLine}
			m.copyCursorX = localX
			m.copyCursorY = absLine
			return m, nil
		}
		// Regular window drag selection (supports split panes)
		if fw := m.wm.FocusedWindow(); fw != nil {
			termID := fw.ID
			var cr geometry.Rect
			if fw.IsSplit() && fw.FocusedPane != "" {
				termID = fw.FocusedPane
				cr = m.paneRectForTerm(fw, fw.FocusedPane)
			} else {
				cr = fw.ContentRect()
			}
			if term, ok := m.terminals[termID]; ok {
				localX := mouse.X - cr.X
				localY := mouse.Y - cr.Y
				termW := term.Width()
				sbLen := term.ScrollbackLen()
				if snap := m.copySnapshotForWindow(termID); snap != nil {
					termW = snap.Width
					sbLen = snap.ScrollbackLen()
				}
				if localX < 0 {
					localX = 0
				}
				if localX >= termW {
					localX = termW - 1
				}
				if localY < 0 {
					localY = 0
				}
				if localY >= cr.Height {
					localY = cr.Height - 1
				}
				absLine := mouseToAbsLine(localY, m.scrollOffset, sbLen, cr.Height)
				m.selEnd = geometry.Point{X: localX, Y: absLine}
				m.copyCursorX = localX
				m.copyCursorY = absLine
			}
		}
		return m, nil
	}

	// Modal tab hover tracking
	if m.modal != nil && m.modal.Tabs != nil && len(m.modal.Tabs) > 0 {
		bounds := m.modal.Bounds(m.width, m.height)
		m.modal.HoverTab = -1
		if mouse.Y == bounds.TabRow &&
			mouse.X >= bounds.StartX+1+bounds.HPad &&
			mouse.X < bounds.StartX+bounds.BoxW-1 {
			relX := mouse.X - bounds.StartX - 1 - bounds.HPad
			m.modal.HoverTab = m.modal.TabAtX(relX)
		}
		return m, nil
	}

	// Workspace picker hover tracking
	if m.workspacePickerVisible {
		bx, by, bw, bh := m.workspacePickerBounds()
		if mouse.X >= bx && mouse.X < bx+bw && mouse.Y >= by && mouse.Y < by+bh {
			itemY := mouse.Y - by - 2
			if itemY >= 0 && itemY < len(m.workspaceList) {
				m.workspacePickerSelected = itemY
			}
		}
		return m, nil
	}

	// Context menu hover tracking
	if m.contextMenu != nil && m.contextMenu.Visible {
		if m.contextMenu.Contains(mouse.X, mouse.Y) {
			relY := mouse.Y - m.contextMenu.Y - 1
			m.contextMenu.HoverIndex = m.contextMenu.ItemAtY(relY)
		} else {
			m.contextMenu.HoverIndex = -1
		}
		return m, nil
	}

	// Settings panel hover tracking
	if m.settings.Visible {
		bounds := m.settings.Bounds(m.width, m.height)
		innerW := m.settings.InnerWidth(m.width)
		m.settings.HoverTab = -1
		if bounds.Contains(geometry.Point{X: mouse.X, Y: mouse.Y}) {
			relX := mouse.X - bounds.X - 1
			relY := mouse.Y - bounds.Y - 1
			if relY == settings.TabRow && relX >= 0 && relX < innerW {
				m.settings.HoverTab = m.settings.TabAtX(relX, innerW)
			}
			if idx := m.settings.ItemAtY(relY); idx >= 0 {
				m.settings.SetItem(idx)
			}
		}
		return m, nil
	}

	// Launcher hover tracking
	if m.launcher.Visible {
		lx, ly, lw, lh := m.launcherBounds()
		if mouse.X >= lx && mouse.X < lx+lw && mouse.Y >= ly && mouse.Y < ly+lh {
			if idx := m.launcherResultIdx(mouse.Y, ly); idx >= 0 {
				m.launcher.SelectedIdx = idx
			}
		}
		return m, nil
	}

	// Update dock hover + tooltip (skip when dock is hidden)
	dockHidden := m.hideDockWhenMaximized && m.hasMaximizedWindow()
	if mouse.Y == m.height-1 && !dockHidden {
		idx := m.dock.ItemAtX(mouse.X)
		m.dock.SetHover(idx)
		if idx >= 0 && idx < len(m.dock.Items) {
			m.tooltipText = m.dock.Items[idx].TooltipText()
			m.tooltipX = m.dock.ItemCenterX(idx)
			m.tooltipY = m.height - 2
		} else {
			m.tooltipText = ""
		}
	} else {
		m.dock.SetHover(-1)
		if m.tooltipText != "" {
			m.tooltipText = ""
		}
	}

	// Track menu label + widget hover on menu bar
	m.hoverMenuLabel = -1
	m.hoverWidgetName = ""
	if mouse.Y == 0 {
		// Check menu labels first
		m.hoverMenuLabel = m.menuBar.MenuAtX(mouse.X)

		// Check right-side widgets
		if m.menuBar.WidgetBar != nil {
			mLabel := modeBadge(m.inputMode)
			if m.prefixPending {
				mLabel = " \uf11c PREFIX   "
			}
			mLabelW := widget.DisplayWidth(mLabel)
			effW := m.width - mLabelW
			if effW < 1 {
				effW = 1
			}
			for _, zone := range m.menuBar.RightZones(effW) {
				if mouse.X >= zone.Start && mouse.X < zone.End {
					m.hoverWidgetName = zone.Type
					break
				}
			}
		}
	}

	// Dropdown + submenu hover tracking (when menu is open)
	if m.menuBar.IsOpen() {
		positions := m.menuBar.MenuXPositions()
		dropX := positions[m.menuBar.OpenIndex]
		dropY := 1 // dropdown starts at row 1
		menu := m.menuBar.Menus[m.menuBar.OpenIndex]
		dropW, _ := m.menuBar.DropdownDimensions()

		// Check submenu hover first
		if m.menuBar.HasSubMenu() {
			parentItemY, parentWidth := m.menuBar.SubMenuParentIndex()
			subX := dropX + parentWidth
			subY := dropY + parentItemY + 1 // +1 for top border
			subW, _ := m.menuBar.SubMenuDimensions()
			subItemIdx := m.menuBar.SubMenuItemAtY(mouse.Y - subY - 1)
			if mouse.X >= subX && mouse.X < subX+subW && subItemIdx >= 0 {
				m.menuBar.SubHoverIndex = subItemIdx
			}
		}

		// Check dropdown item hover
		itemIdx := m.menuBar.DropdownItemAtY(mouse.Y - dropY - 1)
		if mouse.X >= dropX && mouse.X < dropX+dropW && itemIdx >= 0 {
			m.menuBar.HoverIndex = itemIdx
			// Auto-enter submenu on hover if item has sub-items
			if len(menu.Items[itemIdx].SubItems) > 0 && !m.menuBar.InSubMenu {
				m.menuBar.EnterSubMenu()
			} else if len(menu.Items[itemIdx].SubItems) == 0 && m.menuBar.InSubMenu {
				m.menuBar.ExitSubMenu()
			}
		}
	}

	if !m.drag.Active {
		// Track button hover for title bar highlight
		p := geometry.Point{X: mouse.X, Y: mouse.Y}
		m.hoverButtonZone = window.HitNone
		m.hoverButtonWindowID = ""
		if w := m.wm.WindowAt(p); w != nil {
			zone := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)
			switch zone {
			case window.HitCloseButton, window.HitMinButton, window.HitMaxButton,
				window.HitSnapLeftButton, window.HitSnapRightButton:
				m.hoverButtonZone = zone
				m.hoverButtonWindowID = w.ID
			}
		}

		// Focus follows mouse: auto-focus window under cursor
		if m.focusFollowsMouse && !m.exposeMode && m.confirmClose == nil && m.modal == nil &&
			!m.launcher.Visible && !m.settings.Visible {
			if w := m.wm.WindowAt(p); w != nil && !w.Focused {
				m.wm.FocusWindow(w.ID)
				if m.defaultTerminalMode {
					m.inputMode = ModeTerminal
				}
			}
		}

		// Forward motion to focused terminal if over content area and app tracks motion
		if fw, term := m.focusedTerminal(); fw != nil && term != nil && term.HasMouseMotionMode() {
			var cr geometry.Rect
			if fw.IsSplit() && fw.FocusedPane != "" {
				cr = m.paneRectForTerm(fw, fw.FocusedPane)
			} else {
				cr = fw.ContentRect()
			}
			if cr.Contains(p) {
				localX := mouse.X - cr.X // 0-indexed
				localY := mouse.Y - cr.Y
				btn := teaToUvButton(mouse.Button)
				term.SendMouseMotion(btn, localX, localY)
			}
		}
		return m, nil
	}

	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	w := m.wm.WindowByID(m.drag.WindowID)
	if w == nil {
		m.drag = window.DragState{}
		return m, nil
	}

	// Separator drag: adjust split ratio
	if m.drag.Mode == window.DragSeparator && m.drag.SepNode != nil && w.IsSplit() {
		cr := w.SplitContentRect()
		node := m.drag.SepNode
		if m.drag.SepDir == window.SplitHorizontal {
			// Horizontal split: adjust ratio based on X position
			relX := p.X - cr.X
			totalW := cr.Width - 1 // subtract 1 for separator
			if totalW > 0 {
				newRatio := float64(relX) / float64(cr.Width)
				minR := float64(window.MinPaneSize) / float64(cr.Width)
				maxR := 1.0 - minR
				if newRatio < minR {
					newRatio = minR
				}
				if newRatio > maxR {
					newRatio = maxR
				}
				node.Ratio = newRatio
			}
		} else {
			// Vertical split: adjust ratio based on Y position
			relY := p.Y - cr.Y
			totalH := cr.Height - 1
			if totalH > 0 {
				newRatio := float64(relY) / float64(cr.Height)
				minR := float64(window.MinPaneSize) / float64(cr.Height)
				maxR := 1.0 - minR
				if newRatio < minR {
					newRatio = minR
				}
				if newRatio > maxR {
					newRatio = maxR
				}
				node.Ratio = newRatio
			}
		}
		// Don't resize terminals during drag — each resize sends SIGWINCH
		// which makes shells redraw their prompt, flooding the terminal.
		// The actual resize happens on mouse release in handleMouseRelease.
		// Invalidate the render cache so the separator position redraws.
		delete(m.windowCache, w.ID)
		return m, nil
	}

	// Un-maximize on drag move
	if m.drag.Mode == window.DragMove && w.IsMaximized() {
		window.Restore(w)
		m.drag.StartRect = w.Rect
	}

	newRect := window.ApplyDrag(m.drag, p, m.wm.WorkArea())
	w.Rect = newRect

	return m, nil
}

func (m Model) handleMouseRelease(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	// Finalize quake drag resize
	if m.quakeDragActive {
		m.quakeDragActive = false
		// Resize the terminal emulator to match the new height
		if m.quakeTerminal != nil {
			contentH := int(m.quakeAnimH) - 1 // minus bottom border
			if contentH < 1 {
				contentH = 1
			}
			m.quakeTerminal.Resize(m.width, contentH)
		}
		return m, nil
	}

	// Finalize selection drag in copy mode
	if m.selDragging {
		m.selDragging = false
		// If start == end (just a click with no drag), clear selection
		if m.selStart == m.selEnd {
			m.selActive = false
		}
		return m, nil
	}

	if m.drag.Active {
		// Resize terminal after drag completes.
		// Image refresh is handled by the Update wrapper's hidden→visible
		// transition (drag.Active becomes false → schedules ImageRefreshMsg).
		w := m.wm.WindowByID(m.drag.WindowID)
		if w != nil {
			m.resizeTerminalForWindow(w)
		}
		m.drag = window.DragState{}
		return m, nil
	}
	m.drag = window.DragState{}

	// Forward release to quake terminal
	if m.quakeVisible && m.quakeTerminal != nil && m.quakeTerminal.HasMouseMode() {
		h := int(m.quakeAnimH)
		if mouse.Y < h-1 {
			btn := teaToUvButton(mouse.Button)
			m.quakeTerminal.SendMouse(btn, mouse.X, mouse.Y, true)
			return m, nil
		}
	}

	// Forward release to focused terminal
	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	if fw, term := m.focusedTerminal(); fw != nil && term != nil {
		var cr geometry.Rect
		if fw.IsSplit() && fw.FocusedPane != "" {
			cr = m.paneRectForTerm(fw, fw.FocusedPane)
		} else {
			cr = fw.ContentRect()
		}
		if cr.Contains(p) {
			localX := mouse.X - cr.X // 0-indexed
			localY := mouse.Y - cr.Y
			btn := teaToUvButton(mouse.Button)
			term.SendMouse(btn, localX, localY, true)
		}
	}
	return m, nil
}

func (m Model) handleMouseWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	// Quake terminal absorbs wheel events in its area
	if m.quakeVisible && m.quakeTerminal != nil && !m.quakeAnimating() {
		h := int(m.quakeAnimH)
		if mouse.Y < h {
			isWheelUp := mouse.Button == tea.MouseWheelUp
			isWheelDown := mouse.Button == tea.MouseWheelDown

			// App has mouse mode — forward wheel to terminal
			if m.quakeTerminal.HasMouseMode() {
				btn := teaToUvButton(mouse.Button)
				m.quakeTerminal.SendMouseWheel(btn, mouse.X, mouse.Y)
				return m, nil
			}

			// Wheel-up with scrollback: enter copy mode (like tmux)
			if isWheelUp && m.quakeTerminal.ScrollbackLen() > 0 {
				if m.inputMode != ModeCopy {
					m.enterCopyModeForQuake()
				}
				maxScroll := m.quakeTerminal.ScrollbackLen()
				if snap := m.copySnapshotForWindow(quakeTermID); snap != nil {
					maxScroll = snap.ScrollbackLen()
				}
				m.scrollOffset += 3
				if m.scrollOffset > maxScroll {
					m.scrollOffset = maxScroll
				}
				return m, nil
			}

			// Wheel-down in copy mode: scroll down or exit at bottom
			if isWheelDown && m.inputMode == ModeCopy && m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID {
				prev := m.scrollOffset
				m.scrollOffset -= 3
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
				if !m.selActive && m.scrollOffset == 0 && prev == 0 {
					m.exitCopyMode()
				}
				return m, nil
			}

			return m, nil
		}
	}
	// Copy mode: mouse wheel scrolls the scrollback buffer
	if m.inputMode == ModeCopy {
		var term *terminal.Terminal
		var windowID string
		if m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID {
			term = m.quakeTerminal
			windowID = quakeTermID
		} else {
			fw, t := m.focusedTerminal()
			term = t
			if fw != nil {
				if fw.IsSplit() && fw.FocusedPane != "" {
					windowID = fw.FocusedPane
				} else {
					windowID = fw.ID
				}
			}
		}
		if term == nil {
			return m, nil
		}
		maxScroll := term.ScrollbackLen()
		if snap := m.copySnapshotForWindow(windowID); snap != nil {
			maxScroll = snap.ScrollbackLen()
		}
		switch mouse.Button {
		case tea.MouseWheelUp:
			m.scrollOffset += 3
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
		case tea.MouseWheelDown:
			prev := m.scrollOffset
			m.scrollOffset -= 3
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			if !m.selActive && m.scrollOffset == 0 && prev == 0 {
				m.exitCopyMode()
			}
		}
		return m, nil
	}

	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	w := m.wm.WindowAt(p)
	if w == nil {
		return m, nil
	}

	// Find the terminal and content rect under the mouse
	var term *terminal.Terminal
	var cr geometry.Rect
	termID := w.ID
	if w.IsSplit() {
		// Find which pane was wheeled over
		scr := w.SplitContentRect()
		panes := w.SplitRoot.Layout(scr)
		for _, pane := range panes {
			if pane.Rect.Contains(p) {
				termID = pane.TermID
				term = m.terminals[pane.TermID]
				cr = pane.Rect
				break
			}
		}
	} else {
		cr = w.ContentRect()
		term = m.terminals[w.ID]
	}

	if term != nil && cr.Contains(p) {
		localX := mouse.X - cr.X // 0-indexed
		localY := mouse.Y - cr.Y
		btn := teaToUvButton(mouse.Button)

		isWheelUp := mouse.Button == tea.MouseWheelUp
		isWheelDown := mouse.Button == tea.MouseWheelDown
		hasScrollback := term.ScrollbackLen() > 0

		// Wheel up with scrollback: enter copy mode and scroll (like tmux).
		// Skip if the app has mouse mode enabled (nvim, htop, etc.) — forward to app instead.
		if isWheelUp && hasScrollback && !term.HasMouseMode() {
			if m.inputMode != ModeCopy {
				m.enterCopyModeForWindow(termID)
			}
			maxScroll := term.ScrollbackLen()
			if snap := m.copySnapshotForWindow(termID); snap != nil {
				maxScroll = snap.ScrollbackLen()
			}
			m.scrollOffset += 3
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
			return m, nil
		}

		// Wheel down in copy mode: scroll down or exit at bottom.
		if isWheelDown && m.inputMode == ModeCopy {
			m.scrollOffset -= 3
			if m.scrollOffset <= 0 {
				m.exitCopyMode()
				m.inputMode = ModeNormal
			}
			return m, nil
		}

		// Forward wheel to terminal (for apps like nvim with mouse mode).
		term.SendMouseWheel(btn, localX, localY)
	}
	return m, nil
}

func (m Model) handleRightClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	p := geometry.Point{X: mouse.X, Y: mouse.Y}

	// Check if right-click is on a window title bar (iterate front→back)
	wins := m.wm.Windows()
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		if !w.Visible {
			continue
		}
		hit := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)
		if hit == window.HitTitleBar {
			m.wm.FocusWindow(w.ID)
			m.contextMenu = contextmenu.TitleBarMenu(mouse.X, mouse.Y, w.Resizable, w.IsSplit(), ctxMenuKB(m.keybindings))
			m.contextMenu.Clamp(m.width, m.height)
			return m, nil
		}
		// If click is inside window content, it's not on desktop
		if hit != window.HitNone {
			break
		}
	}

	// Dock right-click: show context menu for dock item
	dockHidden := m.hideDockWhenMaximized && m.hasMaximizedWindow()
	if mouse.Y == m.height-1 && !dockHidden {
		idx := m.dock.ItemAtX(mouse.X)
		if idx >= 0 && idx < len(m.dock.Items) {
			item := m.dock.Items[idx]
			if item.WindowID != "" {
				// Right-click on a running/minimized window in dock
				m.wm.FocusWindow(item.WindowID)
				m.contextMenu = contextmenu.DockItemMenu(mouse.X, mouse.Y-1)
				m.contextMenu.Clamp(m.width, m.height)
				m.tooltipText = ""
				m.dock.SetHover(-1)
				return m, nil
			}
		}
	}

	// Desktop background right-click
	if mouse.Y > 0 && mouse.Y < m.height-1 {
		m.contextMenu = contextmenu.DesktopMenu(mouse.X, mouse.Y, ctxMenuKB(m.keybindings))
		m.contextMenu.Clamp(m.width, m.height)
		return m, nil
	}

	return m, nil
}

// handleMenuBarRightClick handles clicks on the right side of the menu bar.
func (m Model) handleMenuBarRightClick(zoneType string) (tea.Model, tea.Cmd) {
	switch zoneType {
	case "cpu", "mem":
		return m.focusOrLaunchApp("htop")
	case "clock":
		m.modal = &ModalOverlay{
			Title:    "Date & Time",
			HoverTab: -1,
			Lines: []string{
				time.Now().Format("Monday, January 2, 2006"),
				time.Now().Format("03:04:05 PM MST"),
			},
		}
	case "notification":
		m.notifications.ToggleCenter()
	case "workspace":
		cmd := m.toggleWorkspacePicker()
		return m, cmd
	case "user":
		home, _ := os.UserHomeDir()
		cmd := m.createTerminalWindow(TerminalWindowOpts{WorkDir: home})
		return m, cmd
	case "git":
		return m.focusOrLaunchApp("lazygit")
	case "docker":
		return m.focusOrLaunchApp("lazydocker")
	default:
		// Check custom widgets for onClick handler
		if cw, ok := m.customWidgets[zoneType]; ok && cw.OnClick != "" {
			return m.focusOrLaunchApp(cw.OnClick)
		}
	}
	return m, nil
}

// workspacePickerBounds returns the screen bounds of the workspace picker overlay.
// Must match the layout logic in renderWorkspacePicker().
func (m Model) workspacePickerBounds() (x, y, w, h int) {
	maxW := 70
	if m.width < maxW {
		maxW = m.width - 4
	}
	maxH := len(m.workspaceList) + 4
	if maxH > m.height-4 {
		maxH = m.height - 4
	}
	startX := (m.width - maxW) / 2
	startY := (m.height - maxH) / 2
	return startX, startY, maxW, maxH
}

// launcherResultIdx returns which result index the mouse is on, or -1 if not on a result.
// mouseY is the screen Y, boundsY is the launcher top edge.
func (m Model) launcherResultIdx(mouseY, boundsY int) int {
	relY := mouseY - boundsY - 7 // border(1) + 6 header lines
	maxResults := min(8, len(m.launcher.Results))
	if relY >= 0 && relY < maxResults {
		return relY
	}
	return -1
}

// launcherBounds returns the approximate screen bounds of the launcher modal.
// Must match the layout logic in RenderLauncher().
func (m Model) launcherBounds() (x, y, w, h int) {
	boxW := 56
	if boxW > m.width-4 {
		boxW = m.width - 4
	}
	// Inner content lines: spacer, title, sep, search, sep, spacer, results..., spacer, sep, footer, spacer
	maxResults := min(8, len(m.launcher.Results))
	if maxResults == 0 {
		maxResults = 1 // "No results" line
	}
	innerH := 6 + maxResults + 3 // 6 header + results + 3 footer (spacer+sep+footer+spacer → 4, minus overlap)
	if len(m.launcher.Suggestions) > 0 {
		innerH += 2 // sep + suggestions
	}
	innerH += 1 // extra spacer
	// Add 2 for border
	totalH := innerH + 2
	totalW := boxW + 2 // border adds 2
	startX := (m.width - totalW) / 2
	startY := (m.height - totalH) / 3
	if startX < 0 {
		startX = 0
	}
	if startY < 1 {
		startY = 1
	}
	return startX, startY, totalW, totalH
}

// teaToUvButton converts a Bubble Tea mouse button to an ultraviolet MouseButton.
func teaToUvButton(b tea.MouseButton) uv.MouseButton {
	switch b {
	case tea.MouseLeft:
		return uv.MouseLeft
	case tea.MouseMiddle:
		return uv.MouseMiddle
	case tea.MouseRight:
		return uv.MouseRight
	case tea.MouseWheelUp:
		return uv.MouseWheelUp
	case tea.MouseWheelDown:
		return uv.MouseWheelDown
	default:
		return uv.MouseNone
	}
}
