package app

import (
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// sanitizeTitle strips invisible Unicode characters (variation selectors,
// zero-width joiners/spaces, BOM) from a window title. Keeps all visible
// characters including Unicode symbols like ✳ and braille patterns.
func sanitizeTitle(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\uFE0E' || r == '\uFE0F': // variation selectors
			continue
		case r == '\u200B' || r == '\u200C' || r == '\u200D': // zero-width space/joiner
			continue
		case r == '\uFEFF': // BOM / zero-width no-break space
			continue
		case !unicode.IsPrint(r) && !unicode.IsSpace(r): // non-printable
			continue
		}
		b.WriteRune(r)
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return s
	}
	return result
}

// isLocalShellTitle returns true if the title looks like a local shell prompt
// (e.g., "user@hostname:~", "user@hostname:/path"). These are set by default
// shell prompt configurations (PS1 with \u@\h) and should not override the
// "Terminal N" window title.
func isLocalShellTitle(title string) bool {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	if user == "" {
		return false
	}
	hostname, _ := os.Hostname()
	// Check common patterns: "user@hostname", "user@hostname:path"
	prefix := user + "@"
	if !strings.HasPrefix(title, prefix) {
		return false
	}
	rest := title[len(prefix):]
	// Short hostname match (before first dot)
	shortHost := hostname
	if idx := strings.IndexByte(hostname, '.'); idx > 0 {
		shortHost = hostname[:idx]
	}
	// Title starts with user@hostname or user@shorthost
	return strings.HasPrefix(rest, hostname) || strings.HasPrefix(rest, shortHost)
}

// animateLayout saves current rects, applies a layout function, then animates
// all windows from their old position to the new one.
func (m *Model) animateLayout(animType AnimationType, layoutFn func([]*window.Window, geometry.Rect)) {
	windows := m.wm.Windows()
	fromRects := make(map[string]geometry.Rect, len(windows))
	for _, w := range windows {
		fromRects[w.ID] = w.Rect
	}
	layoutFn(windows, m.wm.WorkArea())
	for _, w := range windows {
		if from, ok := fromRects[w.ID]; ok {
			m.startWindowAnimation(w.ID, animType, from, w.Rect)
		}
	}
	m.resizeAllTerminals()
}

func (m *Model) animateTileAll()       { m.animateLayout(AnimTile, window.TileAll) }
func (m *Model) animateTileColumns()   { m.animateLayout(AnimTile, window.TileColumns) }
func (m *Model) animateTileRows()      { m.animateLayout(AnimTile, window.TileRows) }
func (m *Model) animateCascade()       { m.animateLayout(AnimTile, window.Cascade) }
func (m *Model) animateTileMaximized() { m.animateLayout(AnimMaximize, window.MaximizeAll) }

func (m *Model) applyTilingLayout() {
	m.tilingLayout = normalizeTilingLayout(m.tilingLayout)
	if m.animationsOn {
		switch m.tilingLayout {
		case "rows":
			m.animateTileRows()
		case "all":
			m.animateTileAll()
		default:
			m.animateTileColumns()
		}
		return
	}
	switch m.tilingLayout {
	case "rows":
		window.TileRows(m.wm.Windows(), m.wm.WorkArea())
	case "all":
		window.TileAll(m.wm.Windows(), m.wm.WorkArea())
	default:
		window.TileColumns(m.wm.Windows(), m.wm.WorkArea())
	}
	m.resizeAllTerminals()
}

// currentTilingSlotByRect returns a stable tile slot based on on-screen geometry
// for the currently selected tiling layout, independent of z-order/focus raises.
func (m *Model) currentTilingSlotByRect(windowID string) int {
	layout := normalizeTilingLayout(m.tilingLayout)
	wins := m.wm.Windows()
	visible := make([]*window.Window, 0, len(wins))
	for _, w := range wins {
		if w.Visible && !w.Minimized && w.Resizable {
			visible = append(visible, w)
		}
	}
	switch layout {
	case "columns":
		sort.SliceStable(visible, func(i, j int) bool {
			if visible[i].Rect.X != visible[j].Rect.X {
				return visible[i].Rect.X < visible[j].Rect.X
			}
			if visible[i].Rect.Y != visible[j].Rect.Y {
				return visible[i].Rect.Y < visible[j].Rect.Y
			}
			return visible[i].ID < visible[j].ID
		})
	default: // rows and tile-all use row-major order
		sort.SliceStable(visible, func(i, j int) bool {
			if visible[i].Rect.Y != visible[j].Rect.Y {
				return visible[i].Rect.Y < visible[j].Rect.Y
			}
			if visible[i].Rect.X != visible[j].Rect.X {
				return visible[i].Rect.X < visible[j].Rect.X
			}
			return visible[i].ID < visible[j].ID
		})
	}
	for i, w := range visible {
		if w.ID == windowID {
			return i
		}
	}
	return -1
}

func normalizeTilingLayout(layout string) string {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "rows":
		return "rows"
	case "all", "tile_all", "grid":
		return "all"
	default:
		return "columns"
	}
}

func tilingLayoutLabel(layout string) string {
	switch normalizeTilingLayout(layout) {
	case "rows":
		return "Rows"
	case "all":
		return "Tile All"
	default:
		return "Columns"
	}
}

func normalizeTileSpawnPreset(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "left":
		return "left"
	case "right":
		return "right"
	case "up":
		return "up"
	case "down":
		return "down"
	default:
		return "auto"
	}
}

func tileSpawnPresetLabel(v string) string {
	switch normalizeTileSpawnPreset(v) {
	case "left":
		return "Left"
	case "right":
		return "Right"
	case "up":
		return "Up"
	case "down":
		return "Down"
	default:
		return "Auto"
	}
}

type swapDirection int

const (
	swapLeft swapDirection = iota
	swapRight
	swapUp
	swapDown
)

func (m Model) swapFocusedWindow(dir swapDirection) Model {
	fw := m.wm.FocusedWindow()
	if fw == nil {
		return m
	}
	candidates := m.wm.Windows()
	if len(candidates) < 2 {
		return m
	}
	fx := fw.Rect.X + fw.Rect.Width/2
	fy := fw.Rect.Y + fw.Rect.Height/2

	bestIdx := -1
	bestPrimary := 0
	bestSecondary := 0

	for i, w := range candidates {
		if w.ID == fw.ID || !w.Visible || w.Minimized {
			continue
		}
		wx := w.Rect.X + w.Rect.Width/2
		wy := w.Rect.Y + w.Rect.Height/2
		dx := wx - fx
		dy := wy - fy

		var primary, secondary int
		switch dir {
		case swapLeft:
			if dx >= 0 {
				continue
			}
			primary = -dx
			secondary = absInt(dy)
		case swapRight:
			if dx <= 0 {
				continue
			}
			primary = dx
			secondary = absInt(dy)
		case swapUp:
			if dy >= 0 {
				continue
			}
			primary = -dy
			secondary = absInt(dx)
		case swapDown:
			if dy <= 0 {
				continue
			}
			primary = dy
			secondary = absInt(dx)
		}

		if bestIdx == -1 || primary < bestPrimary || (primary == bestPrimary && secondary < bestSecondary) {
			bestIdx = i
			bestPrimary = primary
			bestSecondary = secondary
		}
	}

	if bestIdx == -1 {
		m.notifications.Push("Swap", "No window in that direction", notification.Info)
		return m
	}

	other := candidates[bestIdx]
	fromA := fw.Rect
	fromB := other.Rect
	fw.Rect, other.Rect = fromB, fromA
	m.startWindowAnimation(fw.ID, AnimTile, fromA, fw.Rect)
	m.startWindowAnimation(other.ID, AnimTile, fromB, other.Rect)
	m.resizeTerminalForWindow(fw)
	m.resizeTerminalForWindow(other)
	return m
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// showDesktop minimizes all visible windows with animations.
func (m *Model) showDesktop() {
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			m.minimizeWindow(w)
		}
	}
	m.inputMode = ModeNormal
}

// maximizeFocusedWindow maximizes the currently focused window.
func (m *Model) maximizeFocusedWindow() {
	if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.IsMaximized() {
		m.disablePersistentTiling()
		from := fw.Rect
		window.Maximize(fw, m.wm.WorkArea())
		m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
		m.resizeTerminalForWindow(fw)
	}
}

// minimizeWindow animates a window shrinking to the dock area.
func (m *Model) minimizeWindow(w *window.Window) {
	if w.Minimized {
		return
	}
	// Exit copy mode if minimizing the copy-mode window. Clears scrollbar,
	// selection, and snapshot state that would otherwise leak during animation.
	if m.inputMode == ModeCopy && m.copySnapshot != nil && m.copySnapshot.WindowID == w.ID {
		m.exitCopyMode()
		m.inputMode = ModeNormal
	}
	// Mark minimized immediately so the dock shows the window item without delay.
	// The animation still plays visually; RenderWindow skips Minimized windows.
	if m.tilingMode {
		if slot := m.currentTilingSlotByRect(w.ID); slot >= 0 {
			m.minimizedTileSlots[w.ID] = slot
		}
	}
	w.Minimized = true
	// Save original rect before animation overwrites it
	savedRect := w.Rect

	// Animate shrink to dock bar center
	dockY := m.height - 1
	dockX := m.width / 2
	m.startWindowAnimation(w.ID, AnimMinimize, savedRect,
		geometry.Rect{X: dockX, Y: dockY, Width: 1, Height: 1})

	// Unfocus the minimized window and focus next visible one.
	// In tiling mode, keep z-order stable so tile slot ordering is preserved.
	if m.tilingMode {
		m.wm.FocusNextVisibleNoRaise(w.ID)
	} else {
		m.wm.FocusNextVisible(w.ID)
	}
	if m.tilingMode {
		m.applyTilingLayout()
	}
	m.inputMode = ModeNormal
	m.updateDockReserved()
}

// restoreMinimizedWindow animates a minimized window back to its original rect.
func (m *Model) restoreMinimizedWindow(w *window.Window) {
	if !w.Minimized {
		return
	}
	w.Minimized = false
	m.inputMode = ModeNormal
	if m.tilingMode {
		if slot, ok := m.minimizedTileSlots[w.ID]; ok {
			m.wm.PlaceWindowAtTilingSlot(w.ID, slot)
			delete(m.minimizedTileSlots, w.ID)
		}
		// Match manual tiling behavior: keep slot ordering and just reapply layout.
		m.wm.FocusWindowNoRaise(w.ID)
		// In tiling mode, restoring should participate in the tiled layout.
		// Running a dock-restore animation here fights with tile animations and
		// can leave stale geometry gaps after finalize.
		m.applyTilingLayout()
		return
	}
	m.wm.FocusWindow(w.ID)
	// Animate from dock to original position
	dockY := m.height - 1
	dockX := m.width / 2
	m.startWindowAnimation(w.ID, AnimRestore2,
		geometry.Rect{X: dockX, Y: dockY, Width: 1, Height: 1},
		w.Rect)
	m.updateDockReserved()
}

// updateDockRunning syncs the dock's running indicators and minimized markers.
// Windows are associated with their matching dock shortcut (by command) instead
// of creating separate entries. Only unmatched windows get separate dock items.
func (m *Model) updateDockRunning() {
	// Track focused window for active indicator in dock
	if fw := m.wm.FocusedWindow(); fw != nil {
		m.dock.FocusedWindowID = fw.ID
	} else {
		m.dock.FocusedWindowID = ""
	}

	// Propagate VT title (OSC 0/2) to window title — apps like nvim, htop, etc.
	for _, w := range m.wm.Windows() {
		if w.Exited || w.TitleLocked {
			continue
		}
		if term, ok := m.terminals[w.ID]; ok {
			if vtTitle := term.Title(); vtTitle != "" && !isLocalShellTitle(vtTitle) {
					w.Title = sanitizeTitle(vtTitle)
			}
		}
	}

	// Rebuild from preserved BaseItems (not Items, which may have been filtered)
	// When hideDockApps is enabled, skip regular app shortcuts (keep launcher+expose only)
	var baseItems []dock.DockItem
	for _, item := range m.dock.BaseItems {
		if m.hideDockApps && item.Special == "" {
			continue
		}
		baseItems = append(baseItems, item)
	}

	// Collect windows for the right-side window list
	var dockWindows []*window.Window
	for _, w := range m.wm.Windows() {
		if w.Minimized {
			if m.dock.MinimizeToDock {
				dockWindows = append(dockWindows, w)
			}
		} else if w.Visible {
			dockWindows = append(dockWindows, w)
		}
	}

	// Build window list entries (title-only, after Exposé)
	var dynItems []dock.DockItem
	var windowItems []*window.Window
	seenIDs := make(map[string]bool)

	// First pass: preserve order of existing dynamic items
	for _, item := range m.dock.Items {
		if item.Special == "minimized" || item.Special == "running" {
			wID := item.WindowID
			w := findWindow(dockWindows, wID)
			if w == nil {
				continue
			}
			seenIDs[wID] = true
			special := "running"
			if w.Minimized {
				special = "minimized"
			}
			dynItems = append(dynItems, dock.DockItem{
				Label:       w.Title,
				FullTitle:   w.Title,
				Special:     special,
				WindowID:    w.ID,
				Stuck:       w.Stuck || w.Exited,
				HasBell:     w.HasBell,
				HasActivity: w.HasActivity,
			})
			windowItems = append(windowItems, w)
		}
	}

	// Second pass: add new windows
	for _, w := range dockWindows {
		if seenIDs[w.ID] {
			continue
		}
		special := "running"
		if w.Minimized {
			special = "minimized"
		}
		dynItems = append(dynItems, dock.DockItem{
			Label:       w.Title,
			FullTitle:   w.Title,
			Special:     special,
			WindowID:    w.ID,
			Stuck:       w.Stuck || w.Exited,
			HasBell:     w.HasBell,
			HasActivity: w.HasActivity,
		})
		windowItems = append(windowItems, w)
	}

	// Progressively shorten dynamic labels to fit the dock width
	dockItemWidth := func(items []dock.DockItem) int {
		w := 0
		sep := 1 // "│"
		if m.dock.IconsOnly {
			sep = 0 // padding is part of the item in icons-only mode
		}
		for _, item := range items {
			if item.Icon == "" {
				// " label " + separator
				w += utf8.RuneCountInString(item.Label) + 2 + sep
			} else if m.dock.IconsOnly {
				// " icon  " = runeCount + 3 + separator (0)
				w += utf8.RuneCountInString(item.Icon) + 3 + sep
			} else {
				// " icon  label " + separator
				w += utf8.RuneCountInString(item.Icon) + utf8.RuneCountInString(item.Label) + 4 + sep
			}
		}
		return w
	}
	truncateDynTo := func(maxChars int) {
		for i, item := range dynItems {
			titleRunes := []rune(item.Label)
			if len(titleRunes) > maxChars {
				dynItems[i].Label = string(titleRunes[:maxChars])
			}
		}
	}

	totalW := dockItemWidth(baseItems) + dockItemWidth(dynItems)
	if totalW > m.width-4 && len(dynItems) > 0 {
		truncateDynTo(12)
		totalW = dockItemWidth(baseItems) + dockItemWidth(dynItems)
	}
	if totalW > m.width-4 && len(dynItems) > 0 {
		truncateDynTo(8)
		totalW = dockItemWidth(baseItems) + dockItemWidth(dynItems)
	}
	if totalW > m.width-4 && len(dynItems) > 0 {
		truncateDynTo(4)
		totalW = dockItemWidth(baseItems) + dockItemWidth(dynItems)
		if totalW > m.width-4 {
			for i := range dynItems {
				_, shortLabel := minimizedDockLabel(windowItems[i])
				dynItems[i].Label = shortLabel
			}
		}
	}

	// Insert dynamic items after the Expose button
	var result []dock.DockItem
	for _, item := range baseItems {
		result = append(result, item)
		if item.Special == "expose" {
			result = append(result, dynItems...)
		}
	}
	m.dock.Items = result
}

// findWindow returns the window with the given ID from the list, or nil.
func findWindow(windows []*window.Window, id string) *window.Window {
	for _, w := range windows {
		if w.ID == id {
			return w
		}
	}
	return nil
}

// minimizedDockLabel returns a short icon and label for a minimized window in the dock.
// Maps common commands to recognizable abbreviations (e.g. Terminal → "T", nvim → "V").
func minimizedDockLabel(w *window.Window) (icon, label string) {
	cmd := w.Command
	switch {
	case cmd == "" || cmd == "$SHELL":
		return "\uf120", "T" //  terminal
	case cmd == "nvim" || cmd == "vim" || cmd == "vi":
		return "\ue62b", "V" //  vim
	case cmd == "mc" || cmd == "spf" || cmd == "ranger" || cmd == "lf" || cmd == "yazi":
		return "\uf07b", "F" //  files
	case cmd == "htop" || cmd == "btop" || cmd == "top":
		return "\uf200", "M" //  monitor
	case cmd == "python3" || cmd == "python" || cmd == "bc":
		return "\uf1ec", "C" //  calc
	default:
		// First letter of command, uppercased
		if len(cmd) > 0 {
			return "\uf2d0", strings.ToUpper(cmd[:1])
		}
		return "\uf2d0", "?"
	}
}
