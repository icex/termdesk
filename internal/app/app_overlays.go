package app

import (
	"fmt"
	"strings"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// helpOverlay returns a modal with keybinding help.
func (m *Model) helpOverlay() *ModalOverlay {
	kb := m.keybindings
	pfx := strings.ToUpper(kb.Prefix)

	sep := "───────────────────────────────────"
	general := HelpTab{
		Title: "\uf11c  General",
		Lines: []string{
			"NORMAL MODE",
			sep,
			"",
			fmt.Sprintf("  %-14s Quit", kb.Quit),
			fmt.Sprintf("  %-14s Terminal mode", kb.EnterTerminal+" / Enter"),
			fmt.Sprintf("  %-14s New Terminal", kb.NewTerminal+" / Ctrl+N"),
			fmt.Sprintf("  %-14s Close Window", kb.CloseWindow+" / Ctrl+W"),
			fmt.Sprintf("  %-14s Minimize to Dock", kb.Minimize),
			fmt.Sprintf("  %-14s Rename Window", kb.Rename),
			fmt.Sprintf("  %-14s Navigate Dock", kb.DockFocus),
			fmt.Sprintf("  %-14s Launcher", kb.Launcher+"/Ctrl+Sp"),
			"    Tab         Complete selected app/command",
			"    Ctrl+\u2191/\u2193    Prompt history prev/next",
			fmt.Sprintf("  %-14s Next Window", kb.NextWindow),
			fmt.Sprintf("  %-14s Quick Next/Prev", kb.QuickNextWindow+" / "+kb.QuickPrevWindow),
			fmt.Sprintf("  %-14s Show Desktop", kb.ShowDesktop),
			"  1-9           Focus Window #N",
			"",
			"GLOBAL (any mode)",
			sep,
			"",
			fmt.Sprintf("  %-14s Toggle Quake dropdown terminal", kb.QuakeTerminal),
			"",
			"MENUS & OTHER",
			sep,
			"",
			fmt.Sprintf("  %-14s File/Edit/Apps/View", kb.MenuFile+"/"+kb.MenuEdit+"/"+kb.MenuApps+"/"+kb.MenuView),
			fmt.Sprintf("  %-14s Settings", kb.Settings),
			fmt.Sprintf("  %-14s Show Keys", kb.ShowKeys),
			"  F1            Help",
			"  F9            Toggle Expos\u00e9",
			"  F10           Menu Bar",
		},
	}

	layout := HelpTab{
		Title: "\uf009  Layout",
		Lines: []string{
			"WINDOW LAYOUT",
			sep,
			"",
			fmt.Sprintf("  %-14s Snap Left / Right", kb.SnapLeft+" / "+kb.SnapRight),
			fmt.Sprintf("  %-14s Snap Top / Bottom", kb.SnapTop+" / "+kb.SnapBottom),
			fmt.Sprintf("  %-14s Maximize / Restore", kb.Maximize+" / "+kb.Restore),
			fmt.Sprintf("  %-14s Center Window", kb.Center),
			fmt.Sprintf("  %-14s Tile All", kb.TileAll),
			fmt.Sprintf("  %-14s Tile Cols / Rows", kb.TileColumns+" / "+kb.TileRows),
			fmt.Sprintf("  %-14s Cascade", kb.Cascade),
			fmt.Sprintf("  %-14s Tile Maximized", kb.TileMaximized),
			fmt.Sprintf("  %-14s Toggle Tiling", kb.ToggleTiling),
			fmt.Sprintf("  %-14s Swap L/R/U/D", kb.SwapLeft+"/"+kb.SwapRight+"/"+kb.SwapUp+"/"+kb.SwapDown),
			fmt.Sprintf("  %-14s Expos\u00e9", kb.Expose),
			fmt.Sprintf("  %-14s Move Window", "Shift+Arrows"),
			fmt.Sprintf("  %-14s Resize W / H", kb.ShrinkWidth+kb.GrowWidth+" / "+kb.ShrinkHeight+kb.GrowHeight),
			"",
			"DOCK",
			sep,
			"",
			"  \u2190/\u2192 h/l       Navigate items",
			"  Enter         Activate / Restore",
			"  Esc           Exit dock",
			"",
			"EXPOSE",
			sep,
			"",
			"  Tab / Arrows  Navigate",
			"  Enter         Select window",
			"  Esc           Cancel",
		},
	}

	panels := HelpTab{
		Title: "\uf2d2  Panels",
		Lines: []string{
			"CLIPBOARD",
			sep,
			"",
			fmt.Sprintf("  %-14s Clipboard History", kb.ClipboardHistory),
			"    Enter       Paste selected entry",
			"    v           View entry in window",
			"    n / N       Name / clear buffer name",
			"    Esc         Close clipboard",
			"",
			"NOTIFICATIONS",
			sep,
			"",
			fmt.Sprintf("  %-14s Notification Center", kb.NotificationCenter),
			"",
			"WORKSPACE",
			sep,
			"",
			fmt.Sprintf("  %-14s New Workspace", kb.NewWorkspace),
			fmt.Sprintf("  %-14s Save Workspace", kb.SaveWorkspace),
			fmt.Sprintf("  %-14s Load Workspace", kb.LoadWorkspace),
			fmt.Sprintf("  %-14s Workspace History", kb.ProjectPicker),
			"  Alt+1..9      Switch Workspace",
		},
	}

	terminal := HelpTab{
		Title: "\uf120  Terminal",
		Lines: []string{
			"TERMINAL MODE",
			sep,
			"",
			"  All keys are forwarded to the terminal",
			"  except the prefix key and global keys.",
			"",
			"GLOBAL (no prefix needed)",
			sep,
			"",
			fmt.Sprintf("  %-14s Quick Next/Prev", kb.QuickNextWindow+" / "+kb.QuickPrevWindow),
			fmt.Sprintf("  %-14s Toggle Quake terminal", kb.QuakeTerminal),
			"",
			fmt.Sprintf("  %-14s PREFIX key", pfx),
			"",
			fmt.Sprintf("PREFIX (%s) + ACTION", pfx),
			sep,
			"",
			fmt.Sprintf("  + Esc         Normal mode"),
			fmt.Sprintf("  + c           Copy mode"),
			fmt.Sprintf("  + y           Clipboard history"),
			fmt.Sprintf("  + d           Detach session"),
			fmt.Sprintf("  + %-11s Quit", kb.Quit),
			fmt.Sprintf("  + %-11s New Terminal", kb.NewTerminal),
			fmt.Sprintf("  + %-11s Close Window", kb.CloseWindow),
			fmt.Sprintf("  + %-11s Snap Left / Right", kb.SnapLeft+"/"+kb.SnapRight),
			fmt.Sprintf("  + %-11s Snap Top / Bottom", kb.SnapTop+"/"+kb.SnapBottom),
			fmt.Sprintf("  + %-11s Max / Restore", kb.Maximize+"/"+kb.Restore),
			fmt.Sprintf("  + %-11s Center", kb.Center),
			fmt.Sprintf("  + %-11s Tile All", kb.TileAll),
			fmt.Sprintf("  + %-11s Cols / Rows", kb.TileColumns+"/"+kb.TileRows),
			fmt.Sprintf("  + %-11s Cascade", kb.Cascade),
			fmt.Sprintf("  + %-11s Expos\u00e9", kb.Expose),
			fmt.Sprintf("  + %-11s Sync Panes", kb.SyncPanes),
			fmt.Sprintf("  + %-11s Clipboard", kb.ClipboardHistory),
			fmt.Sprintf("  + %-11s Notifications", kb.NotificationCenter),
			fmt.Sprintf("  + %-11s Settings", kb.Settings),
			fmt.Sprintf("  + %-11s Help", kb.Help),
			fmt.Sprintf("  + %-11s Menu Bar", kb.MenuBar),
			fmt.Sprintf("  + %-11s %s to terminal", pfx, pfx),
			"  + 1-9         Focus Window #N",
			"",
			"SPLIT PANES",
			sep,
			"",
			`  + %           Split Horizontal`,
			`  + "           Split Vertical`,
			"  + x           Close Pane (split) / Close Window",
			"  + o           Next Pane",
			"  + ;           Previous Pane",
			"  + Arrows      Focus Pane in Direction",
		},
	}

	copyMode := HelpTab{
		Title: "\uf0c5  Copy",
		Lines: []string{
			"COPY MODE",
			sep,
			"",
			"  Enter from Normal:     c",
			fmt.Sprintf("  Enter from Terminal:   %s + c", pfx),
			"  Shift+Click:           Quick entry + selection",
			"  Wheel up:              Enter + scroll (no mouse mode)",
			"",
			"SCROLLING",
			sep,
			"",
			"  Up / k         Scroll up one line",
			"  Down / j       Scroll down one line",
			"  PgUp           Scroll up one page",
			"  PgDown         Scroll down one page",
			"  Home / g       Scroll to oldest line",
			"  End / G        Scroll to newest (live)",
			"  3k / 2j       Count prefixes for motion",
			"  gg            Jump to oldest line",
			"",
			"SEARCH",
			sep,
			"",
			"  /             Search forward",
			"  ?             Search backward",
			"",
			"TEXT SELECTION",
			sep,
			"",
			"  v              Toggle visual selection",
			"  h / l          Move selection left/right",
			"  j / k          Move selection + scroll",
			"  y              Copy selection (OSC 52)",
			"  Enter          Copy + return to Terminal",
			"  Mouse drag     Select text with mouse",
			"  Esc            Clear selection",
			"",
			"EXIT",
			sep,
			"",
			"  Esc / q        Exit to Normal mode",
			"  i              Exit to Terminal mode",
		},
	}

	session := HelpTab{
		Title: "\uf127  Session",
		Lines: []string{
			"SESSION MANAGEMENT",
			sep,
			"",
			"  Prefix + d    Detach from session",
			"                Session keeps running",
			"                in the background",
			"",
			"RE-ATTACH",
			sep,
			"",
			"  termdesk      Auto-reconnects to",
			"                existing session",
			"",
			"  termdesk --new",
			"                Starts a fresh session",
			"",
			"QUIT",
			sep,
			"",
			"  q             Quit (from Normal mode)",
			"  Prefix + q    Quit (from Terminal)",
		},
	}

	return &ModalOverlay{
		Title:    "Help",
		Tabs:     []HelpTab{general, layout, panels, terminal, copyMode, session},
		HoverTab: -1,
	}
}

// aboutOverlay returns a modal with app info.

func (m *Model) aboutOverlay() *ModalOverlay {
	return &ModalOverlay{
		Title:    "About",
		HoverTab: -1,
		Lines: []string{
			"termdesk " + version,
			"",
			"A retro TUI desktop environment",
			"built with Go + Bubble Tea v2",
			"",
			"github.com/icex/termdesk",
		},
	}
}

// getTooltipAt returns tooltip text for the element at screen position (x,y).
func (m *Model) getTooltipAt(x, y int) string {
	// Dock items (bottom row)
	if y == m.height-1 {
		idx := m.dock.ItemAtX(x)
		if idx >= 0 && idx < len(m.dock.Items) {
			return m.dock.Items[idx].Label
		}
		return ""
	}

	// Menu bar (top row)
	if y == 0 {
		idx := m.menuBar.MenuAtX(x)
		if idx >= 0 && idx < len(m.menuBar.Menus) {
			return m.menuBar.Menus[idx].Label + " menu"
		}
		return ""
	}

	// Title bar buttons
	p := geometry.Point{X: x, Y: y}
	wins := m.wm.Windows()
	for i := len(wins) - 1; i >= 0; i-- {
		w := wins[i]
		if !w.Visible {
			continue
		}
		hit := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)
		switch hit {
		case window.HitCloseButton:
			return "Close window"
		case window.HitMaxButton:
			return "Maximize"
		case window.HitMinButton:
			return "Minimize"
		case window.HitTitleBar:
			return w.Title
		case window.HitContent:
			return ""
		}
		if hit != window.HitNone {
			break
		}
	}
	return ""
}

// showKeyboardTooltip shows a tooltip for the focused UI element via the ? key.
func (m *Model) showKeyboardTooltip() {
	if fw := m.wm.FocusedWindow(); fw != nil {
		// Show tooltip at the window title bar center
		cx := fw.Rect.X + fw.Rect.Width/2
		cy := fw.Rect.Y
		m.tooltipText = fw.Title
		m.tooltipX = cx
		m.tooltipY = cy
		return
	}
	// No focused window — show a hint
	m.tooltipText = "Press N for new terminal"
	m.tooltipX = m.width / 2
	m.tooltipY = m.height / 2
}

// saveTourCompleted marks the guided tour as completed in user config.
func (m *Model) saveTourCompleted() {
	cfg := config.LoadUserConfig()
	cfg.TourCompleted = true
	config.SaveUserConfig(cfg)
}
