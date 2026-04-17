package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/pkg/geometry"
)

func (m Model) View() tea.View {
	// BT v2 calls View() twice per Update(). Return cached result on second call.
	// cache is a shared pointer — survives value-receiver copies.
	if m.cache.viewGen == m.cache.updateGen {
		return m.cache.view
	}

	var viewStart time.Time
	if m.perf != nil && m.perf.enabled {
		viewStart = time.Now()
	}

	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	v.ReportFocus = true

	if !m.ready {
		v.SetContent("Starting termdesk...")
		m.cache.viewGen = m.cache.updateGen
		m.cache.view = v
		return v
	}

	// Determine if dock should be hidden (maximized window + setting enabled)
	dockHidden := m.hideDockWhenMaximized && m.hasMaximizedWindow()

	// Check for expose enter/exit animations
	if m.hasExposeAnimations() {
		buf := RenderExposeTransition(m.wm, m.theme, m.animations, m.exposeFilter)
		RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending, m.hoverWidgetName, m.hoverMenuLabel)
		if !dockHidden {
			RenderDock(buf, m.dock, m.theme, nil)
		}
		v.SetContent(BufferToString(buf))
		ReleaseBuffer(buf)
		m.cache.viewGen = m.cache.updateGen
		m.cache.view = v
		return v
	}

	if m.exposeMode {
		buf := RenderExpose(m.wm, m.theme, m.exposeFilter)
		RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending, m.hoverWidgetName, m.hoverMenuLabel)
		if !dockHidden {
			RenderDock(buf, m.dock, m.theme, nil)
		}
		v.SetContent(BufferToString(buf))
		ReleaseBuffer(buf)
		m.cache.viewGen = m.cache.updateGen
		m.cache.view = v
		return v
	}

	// Build animated rect overrides for windows currently animating
	var animRects map[string]geometry.Rect
	if m.hasActiveAnimations() {
		animRects = make(map[string]geometry.Rect)
		for _, w := range m.wm.Windows() {
			if r, ok := m.animatedRect(w.ID); ok {
				animRects[w.ID] = r
			}
		}
	}
	// Cursor is rendered natively via tea.Cursor (set below), not in the buffer.
	showCursor := false
	// Pass scrollOffset when copy mode is active (even if another window is focused)
	scrollOff := 0
	copyModeActive := m.copySnapshot != nil
	if copyModeActive {
		scrollOff = m.scrollOffset
	}
	copyWindowID := ""
	if m.copySnapshot != nil {
		copyWindowID = m.copySnapshot.WindowID
	}
	sel := SelectionInfo{
		Active:       m.selActive,
		Start:        m.selStart,
		End:          m.selEnd,
		CopyMode:     copyModeActive,
		CopySnap:     m.copySnapshot,
		SearchQuery:  m.copySearchQuery,
		CopyCursorX:  m.copyCursorX,
		CopyCursorY:  m.copyCursorY,
		CopyWindowID: copyWindowID,
	}
	// When quake terminal is visible, all windows lose focus visually
	// (they appear inactive/dimmed behind the quake). Use QuakeUnfocusAll
	// flag instead of mutating shared *Window pointers to avoid data races
	// with concurrent goroutines (e.g. onKittyGraphics callbacks).
	if m.quakeVisible && m.quakeTerminal != nil {
		sel.QuakeUnfocusAll = true
	}
	var frameStart time.Time
	if m.perf != nil && m.perf.enabled {
		frameStart = time.Now()
	}
	buf := RenderFrame(m.wm, m.theme, m.terminals, animRects, showCursor, scrollOff, sel, m.showDeskClock, m.hoverButtonWindowID, m.hoverButtonZone, m.windowCache, m.wallpaperConfig, m.wallpaperTerminal)
	if m.perf != nil && m.perf.enabled {
		m.perf.recordFrame(time.Since(frameStart), frameStats.winCount, frameStats.cacheHits, frameStats.cacheMisses)
	}
	if m.copySnapshot != nil && m.copySearchActive && m.copySnapshot.WindowID != quakeTermID {
		copyTermID := m.copySnapshot.WindowID
		if cw := m.wm.WindowByID(copyTermID); cw != nil {
			renderCopySearchBar(buf, cw, m.theme, m.copySearchQuery, m.copySearchDir, m.copySearchMatchIdx, m.copySearchMatchCount)
		} else if pw := m.windowForTerminal(copyTermID); pw != nil && pw.Visible && !pw.Minimized {
			// Split pane: render search bar inside the pane rect
			cr := m.paneRectForTerm(pw, copyTermID)
			renderCopySearchBarInRect(buf, cr, m.theme, m.copySearchQuery, m.copySearchDir, m.copySearchMatchIdx, m.copySearchMatchCount)
		}
	}
	// Contextual keyboard hints in the focused window's bottom border
	// Hide during animations so the sliding border looks clean.
	if fw := m.wm.FocusedWindow(); fw != nil && !m.hasActiveAnimations() {
		renderWindowHints(buf, fw, m.theme, m.inputMode, m.keybindings, m.selActive, m.copySearchQuery)
	}
	RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending, m.hoverWidgetName, m.hoverMenuLabel)
	if !dockHidden {
		RenderDock(buf, m.dock, m.theme, m.animations)
	}
	// Quake terminal — renders on top of everything including menu bar
	if h := int(m.quakeAnimH); h > 0 && m.quakeTerminal != nil {
		quakeScroll := 0
		var quakeCopySnap *CopySnapshot
		if m.copySnapshot != nil && m.copySnapshot.WindowID == quakeTermID {
			quakeScroll = m.scrollOffset
			quakeCopySnap = m.copySnapshot
		}
		renderQuakeTerminal(buf, m.quakeTerminal, m.theme, h, m.width, quakeScroll, quakeCopySnap)
		// Copy mode overlays for quake (search highlights, selection, cursor, search bar)
		if quakeCopySnap != nil {
			contentH := h - 1
			if contentH < 1 {
				contentH = 1
			}
			cr := geometry.Rect{X: 0, Y: 0, Width: m.width, Height: contentH}
			quakeSel := SelectionInfo{
				Active:      m.selActive,
				Start:       m.selStart,
				End:         m.selEnd,
				CopySnap:    quakeCopySnap,
				SearchQuery: m.copySearchQuery,
				CopyCursorX: m.copyCursorX,
				CopyCursorY: m.copyCursorY,
			}
			renderCopyOverlaysCore(buf, cr, m.quakeTerminal, quakeSel, quakeScroll, m.theme)
			if m.copySearchActive {
				renderQuakeCopySearchBar(buf, m.theme, m.width, m.copySearchQuery, m.copySearchDir, m.copySearchMatchIdx, m.copySearchMatchCount)
			}
		}
	}
	// Sync panes indicator — highlight synced window borders with accent color
	if m.syncPanes {
		renderSyncPanesIndicator(buf, m.wm, m.theme)
	}
	if m.launcher.Visible {
		RenderLauncher(buf, m.launcher, m.theme)
	}
	if m.clipboard.Visible {
		RenderClipboardHistory(buf, m.clipboard, m.theme)
	}
	// Notification center overlay
	if m.notifications.CenterVisible() {
		RenderNotificationCenter(buf, m.notifications, m.theme)
	}
	// Settings panel overlay
	if m.settings.Visible {
		RenderSettingsPanel(buf, m.settings, m.theme)
	}
	// Context menu overlay
	if m.contextMenu != nil && m.contextMenu.Visible {
		RenderContextMenu(buf, m.contextMenu, m.theme)
	}
	// Toast notifications — always rendered (top-right corner)
	if len(m.notifications.VisibleToasts()) > 0 {
		RenderToasts(buf, m.notifications, m.theme)
	}
	if m.confirmClose != nil {
		RenderConfirmDialog(buf, m.confirmClose, m.theme)
	}
	if m.modal != nil {
		RenderModal(buf, m.modal, m.theme)
	}
	if m.renameDialog != nil {
		RenderRenameDialog(buf, m.renameDialog, m.theme)
	}
	if m.bufferNameDialog != nil {
		RenderBufferNameDialog(buf, m.bufferNameDialog, m.theme)
	}
	if m.newWorkspaceDialog != nil {
		RenderNewWorkspaceDialog(buf, m.newWorkspaceDialog, m.theme)
	}
	if m.workspacePickerVisible {
		m.renderWorkspacePicker(buf)
	}
	// Tooltip overlay (dock hover, keyboard ?, exposé)
	if m.tooltipText != "" {
		RenderTooltip(buf, m.tooltipText, m.tooltipX, m.tooltipY, m.theme)
	}
	if m.showKeys {
		RenderShowKeys(buf, m.showKeysEvents, m.theme)
	}
	// Tour overlay (drawn last, on top of everything)
	if m.tour.Active {
		RenderTour(buf, m.tour, m.theme)
	}
	// Resize debug indicator — show terminal dimensions for 2s after resize.
	// Off by default; enable via show_resize_indicator=true in config.toml.
	if m.showResizeIndicator && !m.lastWindowSizeAt.IsZero() && time.Since(m.lastWindowSizeAt) < 2*time.Second {
		sizeStr := fmt.Sprintf(" %dx%d ", m.width, m.height)
		c := m.theme.C()
		x := 1
		y := 1
		for i, ch := range sizeStr {
			if x+i < buf.Width && y < buf.Height {
				buf.SetCell(x+i, y, ch, c.AccentFg, c.AccentColor, 0)
			}
		}
	}
	// Performance overlay — drawn on top of everything, just before ANSI conversion.
	renderPerfOverlay(buf, m.perf, m.theme)
	var ansiStart time.Time
	if m.perf != nil && m.perf.enabled {
		ansiStart = time.Now()
	}
	s := BufferToString(buf)
	if m.perf != nil && m.perf.enabled {
		m.perf.recordANSI(time.Since(ansiStart), len(s))
		m.perf.recordView(time.Since(viewStart))
	}
	dbg("View: buf=%dx%d ansi=%d wins=%d anims=%d dock=%d",
		buf.Width, buf.Height, len(s), len(m.wm.Windows()), len(m.animations), len(m.dock.Items))
	ReleaseBuffer(buf)
	v.SetContent(s)
	// Native cursor — let the terminal handle shape, blink, and rendering.
	v.Cursor = m.getNativeCursor()
	// Kitty graphics output is flushed via tea.Raw() in the Update() wrapper,
	// not here. tea.Raw routes through BT's synchronized output pipeline
	// (p.outputBuf → p.flush() → p.output), serialized with the frame render
	// in the same ticker goroutine. No /dev/tty writes needed.
	m.cache.viewGen = m.cache.updateGen
	m.cache.view = v
	return v
}

// getNativeCursor returns a native tea.Cursor for the focused terminal window,
// or nil if no cursor should be shown (e.g. copy mode, no terminal, cursor hidden,
// animations active).
func (m Model) getNativeCursor() *tea.Cursor {
	if m.inputMode != ModeTerminal {
		return nil
	}
	// Suppress cursor when overlays are visible — cursor shouldn't bleed through.
	if m.contextMenu != nil && m.contextMenu.Visible {
		return nil
	}
	if m.confirmClose != nil || m.modal != nil || m.renameDialog != nil {
		return nil
	}
	// Suppress cursor during any animation — the window rect is in flux and
	// would place the cursor at wrong screen coordinates.
	if m.hasActiveAnimations() {
		return nil
	}

	// Quake terminal cursor — positioned within the quake content area
	if m.quakeVisible && m.quakeTerminal != nil {
		if m.quakeTerminal.IsCursorHidden() {
			return nil
		}
		cx, cy := m.quakeTerminal.CursorPosition()
		contentH := int(m.quakeAnimH) - 1 // minus bottom border
		if contentH < 1 {
			return nil
		}
		// Cursor must be within the visible quake content area
		if cx < 0 || cx >= m.width || cy < 0 || cy >= contentH {
			return nil
		}
		cursor := tea.NewCursor(cx, cy)
		cursor.Color = m.theme.C().AccentColor
		cursor.Blink = true
		return cursor
	}

	fw := m.wm.FocusedWindow()
	if fw == nil || !fw.Visible || fw.Minimized {
		return nil
	}

	// For split windows, use the focused pane's terminal and rect
	var term *terminal.Terminal
	var cr geometry.Rect
	if fw.IsSplit() && fw.FocusedPane != "" {
		term = m.terminals[fw.FocusedPane]
		cr = fw.SplitRoot.PaneRectForTerm(fw.FocusedPane, fw.SplitContentRect())
	} else {
		term = m.terminals[fw.ID]
		cr = fw.ContentRect()
	}

	if term == nil || term.IsCursorHidden() {
		return nil
	}
	cx, cy := term.CursorPosition()
	sx := cr.X + cx
	sy := cr.Y + cy
	if sx < cr.X || sx >= cr.Right() || sy < cr.Y || sy >= cr.Bottom() {
		return nil
	}
	cursor := tea.NewCursor(sx, sy)
	cursor.Color = m.theme.C().AccentColor
	cursor.Blink = true
	return cursor
}
