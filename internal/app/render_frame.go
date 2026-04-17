package app

import (
	"image/color"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// RenderFrame composites all windows using the painter's algorithm.
// Windows are drawn back-to-front in z-order.
// animRects provides animated rect overrides for windows currently animating.
// SelectionInfo holds the current copy-mode selection state for rendering.
type SelectionInfo struct {
	Active           bool
	Start            geometry.Point // X=col, Y=absLine
	End              geometry.Point // X=col, Y=absLine
	CopyMode         bool           // true when in copy mode (for scrollbar)
	CopySnap         *CopySnapshot  // frozen copy-mode snapshot for focused window
	SearchQuery      string         // current search query for match highlighting
	CopyCursorX      int            // copy mode cursor column
	CopyCursorY      int            // copy mode cursor absolute line
	CopyWindowID     string         // window ID that owns the copy mode
	QuakeUnfocusAll  bool           // when true, render all windows as unfocused (quake visible)
}

type windowRenderCache struct {
	buf             *Buffer
	rect            geometry.Rect
	focused         bool
	title           string
	resizable       bool
	maximized       bool
	titleBarHeight  int
	hasNotification bool
	hasBell         bool
	exited          bool
	stuck           bool
	hoverZone       window.HitZone
	themeName       string
	scrollOffset    int
	cursorShown     bool
	hasCopySnap     bool
	copyMode        bool
	selActive       bool
	selStart        geometry.Point
	selEnd          geometry.Point
	searchQuery     string
	copyCursorX     int
	copyCursorY     int
	focusedPane     string // split pane focus tracking
	dirtyGen        uint64 // terminal dirty generation at time of cache
}

func (c *windowRenderCache) matches(w *window.Window, rect geometry.Rect, theme config.Theme, hoverZone window.HitZone, scrollOffset int, cursorShown bool, hasCopySnap bool, copyMode bool, sel SelectionInfo) bool {
	if c == nil {
		return false
	}
	return c.buf != nil &&
		c.rect == rect &&
		c.focused == w.Focused &&
		c.title == w.Title &&
		c.resizable == w.Resizable &&
		c.maximized == w.IsMaximized() &&
		c.titleBarHeight == w.TitleBarHeight &&
		c.hasNotification == w.HasNotification &&
		c.hasBell == w.HasBell &&
		c.exited == w.Exited &&
		c.stuck == w.Stuck &&
		c.hoverZone == hoverZone &&
		c.themeName == theme.Name &&
		c.scrollOffset == scrollOffset &&
		c.cursorShown == cursorShown &&
		c.hasCopySnap == hasCopySnap &&
		c.copyMode == copyMode &&
		c.selActive == sel.Active &&
		c.selStart == sel.Start &&
		c.selEnd == sel.End &&
		c.searchQuery == sel.SearchQuery &&
		c.copyCursorX == sel.CopyCursorX &&
		c.copyCursorY == sel.CopyCursorY &&
		c.focusedPane == w.FocusedPane
}

// renderWallpaperTerminal renders a headless terminal's output as the desktop background.
// The terminal output fills the work area (below menu bar, above dock).
func renderWallpaperTerminal(buf *Buffer, term *terminal.Terminal, theme config.Theme, wa geometry.Rect) {
	if term == nil {
		return
	}
	defaultFg := theme.C().DefaultFg
	defaultBg := theme.C().DesktopBg
	if defaultBg == nil {
		defaultBg = color.RGBA{A: 255}
	}
	area := geometry.Rect{X: 0, Y: wa.Y, Width: wa.Width, Height: wa.Height}
	renderTerminalContent(buf, area, term, defaultFg, defaultBg, 0)
}

// renderDeskClock draws a big HH:MM clock at the bottom-right of the work area.
func renderDeskClock(buf *Buffer, theme config.Theme) {
	now := time.Now()
	timeStr := now.Format("15:04")
	tw := bigTextWidth(timeStr)
	startX := buf.Width - tw - 3
	startY := buf.Height - 3 - bigOutRows
	if startX < 0 {
		startX = 0
	}
	if startY < 2 {
		startY = 2
	}
	fg := theme.C().SubtleFg
	bg := theme.C().DesktopBg
	renderBigText(buf, startX, startY, timeStr, fg, bg)
}

// frameStats collects per-frame render statistics. Reset each frame.
// Only used when TERMDESK_PERF=1; safe for single-goroutine BT main loop.
var frameStats struct {
	winCount    int
	cacheHits   int
	cacheMisses int
}

func RenderFrame(wm *window.Manager, theme config.Theme, terminals map[string]*terminal.Terminal, animRects map[string]geometry.Rect, showCursor bool, scrollOffset int, sel SelectionInfo, showDeskClock bool, hoverWindowID string, hoverZone window.HitZone, cache map[string]*windowRenderCache, wp *WallpaperConfig, wpTerm *terminal.Terminal) *Buffer {
	frameStats.winCount = 0
	frameStats.cacheHits = 0
	frameStats.cacheMisses = 0
	wa := wm.WorkArea()
	// Use full terminal bounds for the buffer (includes reserved rows for menu/dock)
	fullWidth := wa.Width
	fullHeight := wa.Height + wa.Y + wm.ReservedBottom()
	if fullWidth <= 0 || fullHeight <= 0 {
		return AcquireThemedBuffer(1, 1, theme)
	}

	buf := AcquireWallpaperBuffer(fullWidth, fullHeight, theme, wp)
	frameRect := geometry.Rect{X: 0, Y: 0, Width: fullWidth, Height: fullHeight}
	seen := make(map[string]bool)

	// Render wallpaper program terminal output before everything else
	if wp != nil && wp.Mode == "program" && wpTerm != nil {
		renderWallpaperTerminal(buf, wpTerm, theme, wa)
	}

	// Desktop clock drawn before windows so terminals paint over it
	if showDeskClock {
		renderDeskClock(buf, theme)
	}

	// Draw windows back-to-front (painter's algorithm)
	for _, w := range wm.Windows() {
		// Use animated rect if available when deciding visibility.
		animRect, hasAnim := animRects[w.ID]
		if !w.Visible {
			continue
		}
		if w.Minimized && !hasAnim {
			continue
		}
		drawRect := w.Rect
		if hasAnim {
			drawRect = animRect
		}
		if !drawRect.Overlaps(frameRect) {
			continue
		}
		frameStats.winCount++
		if cache != nil {
			seen[w.ID] = true
		}
		// Only pass hover zone for the window being hovered
		winHover := window.HitNone
		if w.ID == hoverWindowID {
			winHover = hoverZone
		}

		// Split windows use their own rendering path
		if w.IsSplit() {
			splitCopyMode := sel.CopyMode && sel.CopyWindowID != "" && w.SplitRoot.FindLeaf(sel.CopyWindowID) != nil
			if hasAnim {
				if animRect.Width < 1 {
					animRect.Width = 1
				}
				if animRect.Height < 1 {
					animRect.Height = 1
				}
				// Use a local copy to avoid mutating shared *Window state
				// (concurrent goroutines like onKittyGraphics read these fields).
				wcopy := *w
				wcopy.Rect = animRect
				wcopy.Minimized = false
				if sel.QuakeUnfocusAll {
					wcopy.Focused = false
				}
				renderSplitWindow(buf, &wcopy, theme, terminals, showCursor, scrollOffset, sel, winHover)
				if splitCopyMode {
					renderSplitCopyOverlays(buf, &wcopy, terminals, sel, scrollOffset, theme)
				}
			} else {
				// Split windows bypass the single-terminal cache — check if any pane is dirty
				anyDirty := false
				var splitGen uint64
				for _, id := range w.SplitRoot.AllTermIDs() {
					if t := terminals[id]; t != nil {
						if t.ConsumeDirty() {
							anyDirty = true
						}
						splitGen += t.DirtyGen()
					}
				}
				if cache != nil {
					entry := cache[w.ID]
					// Secondary gen-counter check for split windows
					if !anyDirty && entry != nil && splitGen != entry.dirtyGen {
						anyDirty = true
					}
					if entry != nil && !anyDirty && entry.matches(w, drawRect, theme, winHover, scrollOffset, showCursor, sel.CopySnap != nil, splitCopyMode, sel) {
						frameStats.cacheHits++
						buf.Blit(drawRect.X, drawRect.Y, entry.buf)
					} else {
						frameStats.cacheMisses++
						if entry != nil && entry.buf != nil {
							ReleaseBuffer(entry.buf)
						}
						tmp := AcquireThemedBuffer(drawRect.Width, drawRect.Height, theme)
						wcopy := *w
						wcopy.Rect = geometry.Rect{X: 0, Y: 0, Width: drawRect.Width, Height: drawRect.Height}
						if sel.QuakeUnfocusAll {
							wcopy.Focused = false
						}
						renderSplitWindow(tmp, &wcopy, theme, terminals, showCursor, scrollOffset, sel, winHover)
						// Render copy overlays into the cached buffer
						if splitCopyMode {
							renderSplitCopyOverlays(tmp, &wcopy, terminals, sel, scrollOffset, theme)
						}
						effectiveFocused := w.Focused
						if sel.QuakeUnfocusAll {
							effectiveFocused = false
						}
						cache[w.ID] = &windowRenderCache{
							buf:             tmp,
							rect:            drawRect,
							focused:         effectiveFocused,
							title:           w.Title,
							resizable:       w.Resizable,
							maximized:       w.IsMaximized(),
							titleBarHeight:  w.TitleBarHeight,
							hasNotification: w.HasNotification,
							hasBell:         w.HasBell,
							exited:          w.Exited,
							stuck:           w.Stuck,
							hoverZone:       winHover,
							themeName:       theme.Name,
							scrollOffset:    scrollOffset,
							cursorShown:     showCursor,
							hasCopySnap:     sel.CopySnap != nil,
							copyMode:        splitCopyMode,
							selActive:       sel.Active,
							selStart:        sel.Start,
							selEnd:          sel.End,
							searchQuery:     sel.SearchQuery,
							copyCursorX:     sel.CopyCursorX,
							copyCursorY:     sel.CopyCursorY,
							focusedPane:     w.FocusedPane,
							dirtyGen:        splitGen,
						}
						buf.Blit(drawRect.X, drawRect.Y, tmp)
					}
				} else {
					rw := w
					if sel.QuakeUnfocusAll && w.Focused {
						wcopy := *w
						wcopy.Focused = false
						rw = &wcopy
					}
					renderSplitWindow(buf, rw, theme, terminals, showCursor, scrollOffset, sel, winHover)
					if splitCopyMode {
						renderSplitCopyOverlays(buf, rw, terminals, sel, scrollOffset, theme)
					}
				}
			}
		} else {
			// Single-terminal window rendering
			var term *terminal.Terminal
			if terminals != nil {
				term = terminals[w.ID]
			}
			// Apply scrollOffset only to the copy mode window (not necessarily focused)
			winScroll := 0
			if sel.CopyWindowID != "" && w.ID == sel.CopyWindowID {
				winScroll = scrollOffset
			} else if sel.CopyWindowID == "" && w.Focused {
				winScroll = scrollOffset
			}
			cursorShown := w.Focused && showCursor && term != nil && !term.IsCursorHidden() && winScroll == 0
			copySnap := (*CopySnapshot)(nil)
			winCopyMode := sel.CopyMode && sel.CopyWindowID != "" && w.ID == sel.CopyWindowID
			if !winCopyMode {
				winCopyMode = w.Focused && sel.CopyMode && sel.CopyWindowID == ""
			}
			if winCopyMode {
				copySnap = sel.CopySnap
			}

			// Use animated rect if available
			if hasAnim {
				// Clamp animated rect to valid range (spring can overshoot)
				if animRect.Width < 1 {
					animRect.Width = 1
				}
				if animRect.Height < 1 {
					animRect.Height = 1
				}
				// Use a local copy to avoid mutating shared *Window state
				// (concurrent goroutines like onKittyGraphics read these fields).
				wcopy := *w
				wcopy.Rect = animRect
				wcopy.Minimized = false
				if sel.QuakeUnfocusAll {
					wcopy.Focused = false
				}
				renderWindowChrome(buf, &wcopy, theme, winHover)
				if animRect.Width > 3 && animRect.Height > 3 {
					renderWindowTerminalContentWithSnapshot(buf, &wcopy, theme, term, showCursor, winScroll, copySnap)
					if wcopy.Exited && !winCopyMode {
						renderExitedOverlay(buf, &wcopy, theme)
					}
					if winCopyMode {
						renderCopyOverlays(buf, wcopy.ContentRect(), &wcopy, term, sel, scrollOffset, theme)
					}
				}
			} else {
				if cache != nil {
					entry := cache[w.ID]
					termDirty := term != nil && term.ConsumeDirty()
					// Secondary check: compare dirty generation counter.
					// This catches cases where ConsumeDirty() was consumed by an
					// intervening View() (mouse move, timer) but the terminal
					// has new data since the cache entry was created.
					var curGen uint64
					if !termDirty && term != nil && entry != nil {
						curGen = term.DirtyGen()
						if curGen != entry.dirtyGen {
							termDirty = true
						}
					}
					if entry != nil && !termDirty && entry.matches(w, drawRect, theme, winHover, winScroll, cursorShown, copySnap != nil, winCopyMode, sel) {
						frameStats.cacheHits++
						buf.Blit(drawRect.X, drawRect.Y, entry.buf)
					} else {
						frameStats.cacheMisses++
						if entry != nil && entry.buf != nil {
							ReleaseBuffer(entry.buf)
						}
						// Capture gen for the new cache entry
						if term != nil && curGen == 0 {
							curGen = term.DirtyGen()
						}
						tmp := AcquireThemedBuffer(drawRect.Width, drawRect.Height, theme)
						wcopy := *w
						wcopy.Rect = geometry.Rect{X: 0, Y: 0, Width: drawRect.Width, Height: drawRect.Height}
						if sel.QuakeUnfocusAll {
							wcopy.Focused = false
						}
						renderWindowChrome(tmp, &wcopy, theme, winHover)
						renderWindowTerminalContentWithSnapshot(tmp, &wcopy, theme, term, showCursor, winScroll, copySnap)
						if wcopy.Exited && !winCopyMode {
							renderExitedOverlay(tmp, &wcopy, theme)
						}
						// Render copy overlays INTO the cached buffer so they
						// are part of the window's rendering and naturally
						// follow z-order via the painter's algorithm.
						if winCopyMode {
							renderCopyOverlays(tmp, wcopy.ContentRect(), &wcopy, term, sel, scrollOffset, theme)
						}
						effectiveFocused := w.Focused
						if sel.QuakeUnfocusAll {
							effectiveFocused = false
						}
						cache[w.ID] = &windowRenderCache{
							buf:             tmp,
							rect:            drawRect,
							focused:         effectiveFocused,
							title:           w.Title,
							resizable:       w.Resizable,
							maximized:       w.IsMaximized(),
							titleBarHeight:  w.TitleBarHeight,
							hasNotification: w.HasNotification,
							hasBell:         w.HasBell,
							exited:          w.Exited,
							stuck:           w.Stuck,
							hoverZone:       winHover,
							themeName:       theme.Name,
							scrollOffset:    winScroll,
							cursorShown:     cursorShown,
							hasCopySnap:     copySnap != nil,
							copyMode:        winCopyMode,
							selActive:       sel.Active,
							selStart:        sel.Start,
							selEnd:          sel.End,
							searchQuery:     sel.SearchQuery,
							copyCursorX:     sel.CopyCursorX,
							copyCursorY:     sel.CopyCursorY,
							dirtyGen:        curGen,
						}
						buf.Blit(drawRect.X, drawRect.Y, tmp)
					}
				} else {
					rw := w
					if sel.QuakeUnfocusAll && w.Focused {
						wcopy := *w
						wcopy.Focused = false
						rw = &wcopy
					}
					renderWindowChrome(buf, rw, theme, winHover)
					renderWindowTerminalContentWithSnapshot(buf, rw, theme, term, showCursor, winScroll, copySnap)
					if rw.Exited && !winCopyMode {
						renderExitedOverlay(buf, rw, theme)
					}
					if winCopyMode {
						renderCopyOverlays(buf, rw.ContentRect(), rw, term, sel, scrollOffset, theme)
					}
				}
			}
		}
	}

	if cache != nil {
		for id, entry := range cache {
			if !seen[id] {
				if entry != nil && entry.buf != nil {
					ReleaseBuffer(entry.buf)
				}
				delete(cache, id)
			}
		}
	}

	return buf
}

// renderCopyOverlaysCore draws copy mode overlays (search highlights, selection,
// cursor) into the given buffer. Does NOT draw scrollbar — caller handles that
// since scrollbar geometry differs between window types.
func renderCopyOverlaysCore(buf *Buffer, cr geometry.Rect, term *terminal.Terminal, sel SelectionInfo, scrollOffset int, theme config.Theme) {
	if term == nil {
		return
	}
	sbLen := term.ScrollbackLen()
	if sel.CopySnap != nil {
		sbLen = sel.CopySnap.ScrollbackLen()
	}
	if sel.SearchQuery != "" {
		renderSearchHighlights(buf, cr, term, sel.CopySnap, scrollOffset, sbLen, sel.SearchQuery, theme)
	}
	if sel.Active {
		renderSelection(buf, cr, sel.Start, sel.End, scrollOffset, sbLen, cr.Height)
	}
	renderCopyCursor(buf, cr, sel.CopyCursorX, sel.CopyCursorY, scrollOffset, sbLen, cr.Height, theme)
}

// renderCopyOverlays draws copy mode overlays for a single-terminal window.
func renderCopyOverlays(buf *Buffer, cr geometry.Rect, w *window.Window, term *terminal.Terminal, sel SelectionInfo, scrollOffset int, theme config.Theme) {
	renderCopyOverlaysCore(buf, cr, term, sel, scrollOffset, theme)
	if term != nil {
		renderScrollbarWithSnapshot(buf, w, theme, term, scrollOffset, sel.CopySnap)
	}
}

// renderSplitCopyOverlays draws copy mode overlays for a split pane window.
func renderSplitCopyOverlays(buf *Buffer, w *window.Window, terminals map[string]*terminal.Terminal, sel SelectionInfo, scrollOffset int, theme config.Theme) {
	termID := sel.CopyWindowID
	if termID == "" || w.SplitRoot == nil {
		return
	}
	term := terminals[termID]
	if term == nil {
		return
	}
	cr := w.SplitRoot.PaneRectForTerm(termID, w.SplitContentRect())
	renderCopyOverlaysCore(buf, cr, term, sel, scrollOffset, theme)
	renderPaneScrollbar(buf, cr, theme, term, scrollOffset, sel.CopySnap)
}
