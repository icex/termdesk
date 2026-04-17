package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/logging"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

// scanDirEntries returns sorted subdirectory names for the given path.
// Hidden directories (starting with ".") are excluded.
func scanDirEntries(dirPath string) []string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// makeNewWorkspaceDialog creates a NewWorkspaceDialog starting at dirPath.
func makeNewWorkspaceDialog(dirPath string) *NewWorkspaceDialog {
	abs, err := os.Getwd()
	if dirPath != "" {
		abs = dirPath
	}
	if err != nil {
		abs = "/"
	}
	return &NewWorkspaceDialog{
		Name:       []rune{},
		TextCursor: 0,
		DirPath:    abs,
		DirEntries: scanDirEntries(abs),
		DirSelect:  0,
		Cursor:     0,
		Selected:   0,
	}
}

// Actions that keep the user in Terminal mode after prefix.
var terminalStayActions = map[string]bool{
	"snap_left": true, "snap_right": true,
	"maximize": true, "restore": true,
	"tile_all": true, "new_terminal": true,
	"next_window": true, "prev_window": true,
	"snap_top": true, "snap_bottom": true, "center": true,
	"tile_columns": true, "tile_rows": true, "cascade": true, "tile_maximized": true, "show_desktop": true,
	"move_left": true, "move_right": true, "move_up": true, "move_down": true,
	"grow_width": true, "shrink_width": true, "grow_height": true, "shrink_height": true,
	"clipboard_history":   true,
	"notification_center": true,
	"settings":            true,
	"show_keys":           true,
	"toggle_tiling":       true,
	"tile_spawn_cycle":    true,
	"swap_left":           true,
	"swap_right":          true,
	"swap_up":             true,
	"swap_down":           true,
	"split_horizontal": true,
	"split_vertical":   true,
	"close_pane":       true,
	"next_pane":        true,
	"prev_pane":        true,
	"pane_left":        true,
	"pane_right":       true,
	"pane_up":          true,
	"pane_down":        true,
}

func (m *Model) persistTilingSettings() {
	cfg := config.LoadUserConfig()
	cfg.TilingMode = m.tilingMode
	cfg.TilingLayout = normalizeTilingLayout(m.tilingLayout)
	if err := config.SaveUserConfig(cfg); err != nil {
		logging.Error("config save failed: %v", err)
	}
}

func (m *Model) syncTilingMenuLabel() {
	if m.menuBar == nil {
		return
	}
	m.menuBar.SetToggleTilingLabel(m.tilingMode, tilingLayoutLabel(m.tilingLayout))
	m.menuBar.SetTileSpawnLabel(tileSpawnPresetLabel(m.tileSpawnPreset))
}

func (m *Model) setTilingLayout(layout string) {
	m.tilingLayout = normalizeTilingLayout(layout)
	m.syncTilingMenuLabel()
	m.persistTilingSettings()
}

func (m *Model) cycleTileSpawnPreset() {
	switch normalizeTileSpawnPreset(m.tileSpawnPreset) {
	case "auto":
		m.tileSpawnPreset = "left"
	case "left":
		m.tileSpawnPreset = "right"
	case "right":
		m.tileSpawnPreset = "up"
	case "up":
		m.tileSpawnPreset = "down"
	default:
		m.tileSpawnPreset = "auto"
	}
	m.syncTilingMenuLabel()
}

func (m *Model) disablePersistentTiling() {
	if !m.tilingMode {
		return
	}
	m.tilingMode = false
	m.syncTilingMenuLabel()
	m.persistTilingSettings()
}

// executeAction runs a named WM action. Called from both Normal mode (direct)
// and Terminal mode (via prefix). When invoked via prefix, non-geometry actions
// switch to Normal mode so overlays/dialogs work.
func (m Model) executeAction(action string, msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	wasTerminal := m.inputMode == ModeTerminal || m.inputMode == ModeCopy
	// Exit copy mode before handling any prefix-gated action.
	// This clears scrollbar, selection, snapshot state that would otherwise
	// leak visually during close/minimize animations.
	if m.inputMode == ModeCopy {
		m.exitCopyMode()
		// exitCopyMode sets ModeTerminal; the wasTerminal checks below
		// will set ModeNormal where needed for overlays/dialogs.
	}

	switch action {
	case "quit":
		m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "new_terminal":
		cmd := m.openTerminalWindow()
		return m, cmd

	case "new_workspace":
		cwd, _ := os.Getwd()
		m.newWorkspaceDialog = makeNewWorkspaceDialog(cwd)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "close_window":
		if fw := m.wm.FocusedWindow(); fw != nil && !m.isAnimatingClose(fw.ID) {
			if fw.Exited {
				// No confirmation needed for exited processes
				return m.closeExitedWindow(fw.ID)
			}
			m.confirmClose = &ConfirmDialog{
				WindowID: fw.ID,
				Title:    fmt.Sprintf("Close \"%s\"?", fw.Title),
			}
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "enter_terminal":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			if fw.IsSplit() {
				if fw.FocusedPane != "" {
					if _, ok := m.terminals[fw.FocusedPane]; ok {
						m.inputMode = ModeTerminal
						return m, nil
					}
				}
			} else if _, ok := m.terminals[fw.ID]; ok {
				m.inputMode = ModeTerminal
				return m, nil
			}
		}
		return m, nil

	case "minimize":
		if fw := m.wm.FocusedWindow(); fw != nil {
			m.minimizeWindow(fw)
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()

	case "rename":
		if fw := m.wm.FocusedWindow(); fw != nil {
			text := []rune(fw.Title)
			m.renameDialog = &RenameDialog{
				WindowID: fw.ID,
				Text:     text,
				Cursor:   len(text),
			}
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "dock_focus":
		m.dockFocused = true
		if m.dock.HoverIndex < 0 {
			m.dock.SetHover(0)
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "launcher":
		m.launcher.Toggle()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		if m.launcher.Visible && m.launcher.NeedsExecScan() {
			m.launcher.MarkExecLoading()
			return m, launcher.ScanExecIndex()
		}
		return m, nil

	case "snap_left":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			from := fw.Rect
			window.SnapLeft(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "snap_right":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			from := fw.Rect
			window.SnapRight(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "maximize":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			m.disablePersistentTiling()
			from := fw.Rect
			window.Maximize(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			m.updateDockReserved()
		}
		return m, tickAnimation()

	case "restore":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			from := fw.Rect
			window.Restore(fw)
			m.startWindowAnimation(fw.ID, AnimRestore, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			m.updateDockReserved()
		}
		return m, tickAnimation()

	case "tile_all":
		m.setTilingLayout("all")
		m.animateTileAll()
		return m, tickAnimation()

	case "tile_columns":
		m.setTilingLayout("columns")
		m.animateTileColumns()
		return m, tickAnimation()

	case "tile_rows":
		m.setTilingLayout("rows")
		m.animateTileRows()
		return m, tickAnimation()

	case "cascade":
		m.disablePersistentTiling()
		m.animateCascade()
		return m, tickAnimation()

	case "tile_maximized":
		m.disablePersistentTiling()
		m.animateTileMaximized()
		return m, tickAnimation()

	case "show_desktop":
		m.showDesktop()
		return m, tickAnimation()

	case "split_horizontal":
		cmd := m.splitPane(window.SplitHorizontal)
		return m, cmd

	case "split_vertical":
		cmd := m.splitPane(window.SplitVertical)
		return m, cmd

	case "close_pane":
		m.closeFocusedPane()
		return m, nil

	case "next_pane":
		m.focusNextPane()
		return m, nil

	case "prev_pane":
		m.focusPrevPane()
		return m, nil

	case "pane_left", "pane_right", "pane_up", "pane_down":
		m.focusPaneInDirection(action)
		return m, nil

	case "snap_top":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			from := fw.Rect
			window.SnapTop(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "snap_bottom":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			from := fw.Rect
			window.SnapBottom(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "center":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			from := fw.Rect
			window.CenterWindow(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "move_left":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			window.MoveWindow(fw, -window.MoveStep, 0, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "move_right":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			window.MoveWindow(fw, window.MoveStep, 0, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "move_up":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			window.MoveWindow(fw, 0, -window.MoveStep, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "move_down":
		if fw := m.wm.FocusedWindow(); fw != nil && !fw.Minimized {
			window.MoveWindow(fw, 0, window.MoveStep, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "grow_width":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			window.ResizeWindow(fw, window.ResizeStepW, 0, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "shrink_width":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			window.ResizeWindow(fw, -window.ResizeStepW, 0, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "grow_height":
		if m.quakeVisible && m.quakeTerminal != nil {
			m.resizeQuake(3)
			return m, nil
		}
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			window.ResizeWindow(fw, 0, window.ResizeStepH, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "shrink_height":
		if m.quakeVisible && m.quakeTerminal != nil {
			m.resizeQuake(-3)
			return m, nil
		}
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable && !fw.Minimized {
			window.ResizeWindow(fw, 0, -window.ResizeStepH, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
		return m, nil

	case "expose":
		m.enterExpose()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()

	case "clipboard_history":
		m.clipboard.ToggleHistory()
		return m, nil

	case "notification_center":
		m.notifications.ToggleCenter()
		return m, nil

	case "settings":
		if m.settings.Visible {
			m.settings.Hide()
		} else {
			m.settings.Show()
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil
	case "toggle_tiling":
		m.tilingMode = !m.tilingMode
		m.syncTilingMenuLabel()
		m.persistTilingSettings()
		if m.tilingMode {
			m.applyTilingLayout()
			return m, tickAnimation()
		}
		return m, nil
	case "tile_spawn_cycle":
		m.cycleTileSpawnPreset()
		return m, nil
	case "swap_left":
		return m.swapFocusedWindow(swapLeft), tickAnimation()
	case "swap_right":
		return m.swapFocusedWindow(swapRight), tickAnimation()
	case "swap_up":
		return m.swapFocusedWindow(swapUp), tickAnimation()
	case "swap_down":
		return m.swapFocusedWindow(swapDown), tickAnimation()

	case "save_workspace":
		m.saveWorkspaceNow()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "load_workspace":
		cmd := m.toggleWorkspacePicker()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, cmd
	case "project_picker":
		cmd := m.toggleWorkspacePicker()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, cmd

	case "next_window":
		m.wm.CycleForward()
		return m, nil

	case "prev_window":
		m.wm.CycleBackward()
		return m, nil

	case "help":
		m.modal = m.helpOverlay()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "toggle_expose":
		if m.exposeMode {
			m.exitExpose()
		} else {
			m.enterExpose()
		}
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()

	case "menu_bar":
		m.menuBar.OpenMenu(0)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "menu_file":
		m.menuBar.OpenMenu(0)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "menu_edit":
		m.menuBar.OpenMenu(1)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "menu_apps":
		m.menuBar.OpenMenu(2)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "menu_view":
		m.menuBar.OpenMenu(3)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil
	}

	return m, nil
}

// confirmAccept processes the confirm dialog based on the selected button.
func (m Model) confirmAccept() (tea.Model, tea.Cmd) {
	if m.confirmClose == nil {
		return m, nil
	}
	if m.confirmClose.Selected == 1 { // No
		m.confirmClose = nil
		return m, nil
	}
	// Yes
	if m.confirmClose.IsQuit {
		// Signal apps and save workspace before quitting.
		// Brief sleep is acceptable on quit — one-time user action.
		projectDir := ""
		if m.projectConfig != nil {
			projectDir = m.projectConfig.ProjectDir
		}
		m.signalAppsForCapture()
		time.Sleep(150 * time.Millisecond)
		state := m.captureWorkspaceState()
		if err := workspace.SaveWorkspace(state, projectDir); err != nil {
			logging.Error("workspace save failed: %v", err)
		}

		m.closeAllTerminals()
		return m, tea.Quit
	}
	wid := m.confirmClose.WindowID
	m.confirmClose = nil
	if w := m.wm.WindowByID(wid); w != nil {
		centerX := w.Rect.X + w.Rect.Width/2
		centerY := w.Rect.Y + w.Rect.Height/2
		m.startWindowAnimation(wid, AnimClose, w.Rect,
			geometry.Rect{X: centerX, Y: centerY, Width: 1, Height: 1})
		return m, tickAnimation()
	}
	m.closeTerminal(wid)
	m.wm.RemoveWindow(wid)
	m.updateDockReserved()
	// Prevent terminal mode on minimized/no windows after removal
	if m.inputMode == ModeTerminal {
		if fw := m.wm.FocusedWindow(); fw == nil || fw.Minimized {
			m.inputMode = ModeNormal
		}
	}
	if m.tilingMode {
		m.applyTilingLayout()
		return m, tickAnimation()
	}
	return m, nil
}

func (m Model) executeMenuAction(action string) (tea.Model, tea.Cmd) {
	// Exit copy mode before handling any menu action.
	if m.inputMode == ModeCopy {
		m.exitCopyMode()
		m.inputMode = ModeNormal
	}
	switch action {
	case "new_terminal":
		cmd := m.openTerminalWindow()
		return m, cmd
	case "new_workspace":
		cwd, _ := os.Getwd()
		m.newWorkspaceDialog = makeNewWorkspaceDialog(cwd)
		return m, nil
	case "save_workspace":
		m.saveWorkspaceNow()
		return m, nil
	case "load_workspace":
		return m, m.toggleWorkspacePicker()
	case "toggle_tiling":
		m.tilingMode = !m.tilingMode
		m.syncTilingMenuLabel()
		m.persistTilingSettings()
		if m.tilingMode {
			m.applyTilingLayout()
		}
		return m, nil
	case "tile_spawn_cycle":
		m.cycleTileSpawnPreset()
		return m, nil
	case "quit":
		m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
		return m, nil
	case "tile_all":
		m.setTilingLayout("all")
		m.animateTileAll()
		return m, tickAnimation()
	case "dock_focus_window":
		if fw := m.wm.FocusedWindow(); fw != nil {
			if fw.Minimized {
				fw.Minimized = false
			}
			m.wm.FocusWindow(fw.ID)
			m.inputMode = ModeTerminal
		}
	case "close_window":
		if fw := m.wm.FocusedWindow(); fw != nil {
			if fw.Exited {
				return m.closeExitedWindow(fw.ID)
			}
			m.confirmClose = &ConfirmDialog{Title: "Close " + fw.Title + "?", WindowID: fw.ID}
		}
	case "maximize":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			if fw.IsMaximized() {
				window.Restore(fw)
				m.startWindowAnimation(fw.ID, AnimRestore, from, fw.Rect)
			} else {
				m.disablePersistentTiling()
				window.Maximize(fw, m.wm.WorkArea())
				m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
			}
			m.resizeTerminalForWindow(fw)
			m.updateDockReserved()
			return m, tickAnimation()
		}
	case "snap_left":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			window.SnapLeft(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			return m, tickAnimation()
		}
	case "snap_right":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			window.SnapRight(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			return m, tickAnimation()
		}
	case "snap_top":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			window.SnapTop(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			return m, tickAnimation()
		}
	case "snap_bottom":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			window.SnapBottom(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			return m, tickAnimation()
		}
	case "center":
		if fw := m.wm.FocusedWindow(); fw != nil && fw.Resizable {
			from := fw.Rect
			window.CenterWindow(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
			return m, tickAnimation()
		}
	case "swap_left":
		m = m.swapFocusedWindow(swapLeft)
		return m, tickAnimation()
	case "swap_right":
		m = m.swapFocusedWindow(swapRight)
		return m, tickAnimation()
	case "swap_up":
		m = m.swapFocusedWindow(swapUp)
		return m, tickAnimation()
	case "swap_down":
		m = m.swapFocusedWindow(swapDown)
		return m, tickAnimation()
	case "tile_columns":
		m.setTilingLayout("columns")
		m.animateTileColumns()
		return m, tickAnimation()
	case "tile_rows":
		m.setTilingLayout("rows")
		m.animateTileRows()
		return m, tickAnimation()
	case "cascade":
		m.disablePersistentTiling()
		m.animateCascade()
		return m, tickAnimation()
	case "tile_maximized":
		m.disablePersistentTiling()
		m.animateTileMaximized()
		return m, tickAnimation()
	case "show_desktop":
		for _, w := range m.wm.Windows() {
			if w.Visible && !w.Minimized {
				m.minimizeWindow(w)
			}
		}
		return m, tickAnimation()
	case "minimize":
		if fw := m.wm.FocusedWindow(); fw != nil {
			m.minimizeWindow(fw)
			return m, tickAnimation()
		}
	case "clipboard_history":
		m.clipboard.ToggleHistory()
	case "copy_mode":
		if fw := m.wm.FocusedWindow(); fw != nil {
			if _, ok := m.terminals[fw.ID]; ok {
				m.enterCopyModeForWindow(fw.ID)
			}
		}
	case "copy_search_forward":
		if fw := m.wm.FocusedWindow(); fw != nil {
			if _, ok := m.terminals[fw.ID]; ok {
				if m.inputMode != ModeCopy {
					m.enterCopyModeForWindow(fw.ID)
				}
				m.copySearchActive = true
				m.copySearchQuery = ""
				m.copySearchDir = 1
				m.copySearchMatchCount = 0
				m.copySearchMatchIdx = 0
			}
		}
	case "copy_search_backward":
		if fw := m.wm.FocusedWindow(); fw != nil {
			if _, ok := m.terminals[fw.ID]; ok {
				if m.inputMode != ModeCopy {
					m.enterCopyModeForWindow(fw.ID)
				}
				m.copySearchActive = true
				m.copySearchQuery = ""
				m.copySearchDir = -1
				m.copySearchMatchCount = 0
				m.copySearchMatchIdx = 0
			}
		}
	case "copy_selection":
		if m.selActive {
			fw, term := m.focusedTerminal()
			if term != nil {
				var snap *CopySnapshot
				if fw != nil {
					termID := fw.ID
					if fw.IsSplit() && fw.FocusedPane != "" {
						termID = fw.FocusedPane
					}
					snap = m.copySnapshotForWindow(termID)
				}
				text := extractSelTextWithSnapshot(term, snap, m.selStart, m.selEnd)
				if text != "" {
					writeOSC52(text)
					m.clipboard.Copy(text)
				}
			}
		}
	case "select_all":
		fw, term := m.focusedTerminal()
		if fw != nil && term != nil {
			termID := fw.ID
			if fw.IsSplit() && fw.FocusedPane != "" {
				termID = fw.FocusedPane
			}
			sbLen := term.ScrollbackLen()
			termH := term.Height()
			termW := term.Width()
			if snap := m.copySnapshotForWindow(termID); snap != nil {
				sbLen = snap.ScrollbackLen()
				termH = snap.Height
				termW = snap.Width
			}
			m.selActive = true
			m.selStart = geometry.Point{X: 0, Y: 0}
			m.selEnd = geometry.Point{X: termW - 1, Y: sbLen + termH - 1}
		}
	case "clear_selection":
		m.selActive = false
	case "paste":
		text := m.clipboard.Paste()
		if text != "" {
			if _, term := m.focusedTerminal(); term != nil {
				term.WriteInput([]byte(text))
			}
		}
		m.inputMode = ModeTerminal
	case "settings":
		m.settings.Show()
	case "show_keys":
		m.showKeys = !m.showKeys
		if !m.showKeys {
			m.showKeysEvents = nil
		}
		cfg := config.LoadUserConfig()
		cfg.ShowKeys = m.showKeys
		if err := config.SaveUserConfig(cfg); err != nil {
			logging.Error("config save failed: %v", err)
		}
	case "quake_terminal":
		cmd := m.toggleQuakeTerminal()
		return m, cmd
	case "split_horizontal":
		cmd := m.splitPane(window.SplitHorizontal)
		return m, cmd
	case "split_vertical":
		cmd := m.splitPane(window.SplitVertical)
		return m, cmd
	case "close_pane":
		m.closeFocusedPane()
		return m, nil
	case "next_pane":
		m.focusNextPane()
		return m, nil
	case "prev_pane":
		m.focusPrevPane()
		return m, nil
	case "sync_panes":
		m.syncPanes = !m.syncPanes
		m.menuBar.SetSyncPanesLabel(m.syncPanes)
	case "help_keys":
		m.modal = m.helpOverlay()
	case "about":
		m.modal = m.aboutOverlay()
	case "detach":
		// Send OSC detach sequence — server detects this and disconnects client
		os.Stdout.Write([]byte("\x1b]666;detach\x07"))
		return m, nil
	default:
		// Handle dynamic app launch actions from the Apps menu (e.g. "launch:nvim")
		if strings.HasPrefix(action, "launch:") {
			return m.launchAppFromRegistry(action[7:])
		}
	}
	return m, nil
}

func (m *Model) applySettings() {
	// Load full config first to preserve fields not managed by settings panel
	// (TourCompleted, RecentApps, Favorites, RecentWorkspaces, etc.)
	cfg := config.LoadUserConfig()

	// Update fields from current model state
	cfg.Theme = m.theme.Name
	cfg.IconsOnly = m.dock.IconsOnly
	cfg.Animations = m.animationsOn
	cfg.AnimationSpeed = m.animationSpeed
	cfg.AnimationStyle = m.animationStyle
	cfg.ShowDeskClock = m.showDeskClock
	cfg.ShowKeys = m.showKeys
	cfg.TilingMode = m.tilingMode
	cfg.TilingLayout = m.tilingLayout
	cfg.MinimizeToDock = m.dock.MinimizeToDock
	cfg.HideDockWhenMaximized = m.hideDockWhenMaximized
	cfg.WorkspaceAutoSave = m.workspaceAutoSave
	cfg.ShowResizeIndicator = m.showResizeIndicator
	cfg.QuakeHeightPercent = m.quakeHeightPct
	cfg.Keys = m.keybindings

	// Apply settings panel overrides
	m.settings.ApplyTo(&cfg)

	// Sync keybindings back from config (settings panel may have changed prefix, etc.)
	m.keybindings = cfg.Keys
	m.actionMap = BuildActionMap(m.keybindings)

	if cfg.Theme != m.theme.Name {
		m.theme = config.GetTheme(cfg.Theme)
	}
	m.dock.IconsOnly = cfg.IconsOnly
	m.dock.MinimizeToDock = cfg.MinimizeToDock
	m.animationsOn = cfg.Animations
	m.animationSpeed = cfg.AnimationSpeed
	m.animationStyle = cfg.AnimationStyle
	m.springs = newSpringCache(cfg.AnimationSpeed, cfg.AnimationStyle)
	m.showDeskClock = cfg.ShowDeskClock
	m.showKeys = cfg.ShowKeys
	if !m.showKeys {
		m.showKeysEvents = nil
	}
	prevTiling := m.tilingMode
	m.tilingMode = cfg.TilingMode
	m.tilingLayout = normalizeTilingLayout(cfg.TilingLayout)
	m.syncTilingMenuLabel()
	m.hideDockWhenMaximized = cfg.HideDockWhenMaximized
	m.hideDockApps = cfg.HideDockApps
	m.defaultTerminalMode = cfg.DefaultTerminalMode
	m.focusFollowsMouse = cfg.FocusFollowsMouse
	m.workspaceAutoSave = cfg.WorkspaceAutoSave
	m.workspaceAutoSaveMin = cfg.WorkspaceAutoSaveMin
	m.quakeHeightPct = quakeHeightPctOrDefault(cfg.QuakeHeightPercent)
	m.menuBar.SetMenuItemShortcut("quake_terminal", cfg.Keys.QuakeTerminal)
	// Rebuild widget bar if enabled widgets changed
	if !slicesEqual(m.enabledWidgets, cfg.EnabledWidgets) {
		m.enabledWidgets = cfg.EnabledWidgets
		m.widgetBar = widget.NewBar(m.widgetRegistry, m.enabledWidgets,
			m.barUsername, m.barDisplayName, m.customWidgets)
		m.menuBar.WidgetBar = m.widgetBar
		// Re-extract workspace widget reference
		m.workspaceWidget = nil
		for _, w := range m.widgetBar.Widgets {
			if ww, ok := w.(*widget.WorkspaceWidget); ok {
				m.workspaceWidget = ww
				break
			}
		}
	}
	// Wallpaper settings
	prevWpMode := m.wallpaperMode
	prevWpProg := m.wallpaperProgram
	m.wallpaperMode = cfg.WallpaperMode
	m.wallpaperColor = cfg.WallpaperColor
	m.wallpaperPattern = cfg.WallpaperPattern
	m.wallpaperPatternFg = cfg.WallpaperPatternFg
	m.wallpaperPatternBg = cfg.WallpaperPatternBg
	m.wallpaperProgram = cfg.WallpaperProgram
	m.wallpaperConfig = m.buildWallpaperConfig()
	// If wallpaper program changed or mode switched away from program, close old terminal
	if m.wallpaperTerminal != nil && (m.wallpaperMode != "program" || m.wallpaperProgram != prevWpProg) {
		m.closeWallpaperTerminal()
	}
	// If switched to program mode or program changed, launch new wallpaper terminal
	if m.wallpaperMode == "program" && m.wallpaperProgram != "" && (prevWpMode != "program" || m.wallpaperProgram != prevWpProg) {
		m.launchWallpaperTerminal()
	}

	if err := config.SaveUserConfig(cfg); err != nil {
		logging.Error("config save failed: %v", err)
	}
	m.updateDockReserved()
	m.updateDockRunning()
	if !prevTiling && m.tilingMode {
		m.applyTilingLayout()
	}
}

// hasMaximizedWindow returns true if any visible window is maximized.
func (m *Model) hasMaximizedWindow() bool {
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized && w.IsMaximized() {
			return true
		}
	}
	return false
}

// updateDockReserved adjusts the bottom reserved space based on dock visibility.
// When the reserved space changes, maximized windows are re-expanded to match the new work area.
func (m *Model) updateDockReserved() {
	bottom := 1
	if m.hideDockWhenMaximized && m.hasMaximizedWindow() {
		bottom = 0
	}
	m.wm.SetReserved(1, bottom)
	// Re-expand maximized windows to the new work area
	wa := m.wm.WorkArea()
	for _, w := range m.wm.Windows() {
		if w.IsMaximized() && w.Rect != wa {
			w.Rect = wa
			m.resizeTerminalForWindow(w)
		}
	}
}

func (m Model) handleSettingsClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	bounds := m.settings.Bounds(m.width, m.height)
	innerW := m.settings.InnerWidth(m.width)

	// Click outside panel dismisses it
	if !bounds.Contains(geometry.Point{X: mouse.X, Y: mouse.Y}) {
		m.settings.Hide()
		m.applySettings()
		return m, nil
	}

	// Relative position inside the border
	relX := mouse.X - bounds.X - 1 // -1 for left border
	relY := mouse.Y - bounds.Y - 1 // -1 for top border
	if relX < 0 || relX >= innerW || relY < 0 {
		return m, nil
	}

	// Tab row click
	if relY == settings.TabRow {
		idx := m.settings.TabAtX(relX, innerW)
		if idx >= 0 {
			m.settings.SetTab(idx)
		}
		return m, nil
	}

	// Item click
	itemRow := m.settings.ItemAtY(relY)
	if itemRow >= 0 {
		m.settings.SetItem(itemRow)
		item := m.settings.CurrentItem()
		if item != nil {
			if item.Type == settings.TypeToggle {
				m.settings.Toggle()
			} else if item.Type == settings.TypeChoice {
				// Compute where the value string starts (must match render_settings.go layout)
				valStr := "◀ " + item.StrVal + " ▶"
				valW := utf8.RuneCountInString(valStr)
				labelW := innerW - 8 - valW
				if labelW < 10 {
					labelW = 10
				}
				valStart := 2 + labelW + 2 // left pad + label + middle pad
				valMid := valStart + valW/2
				if relX < valMid {
					m.settings.CycleChoice(-1)
				} else {
					m.settings.CycleChoice(1)
				}
			}
			m.applySettings()
		}
		return m, nil
	}

	return m, nil
}

// slicesEqual returns true if two string slices have the same elements in the same order.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
