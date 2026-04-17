package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/logging"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

const envHomeDir = "TERMDESK_HOME"

func appHomeDir() string {
	if override := strings.TrimSpace(os.Getenv(envHomeDir)); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// isAppStateCapable returns true if the command supports SIGUSR1/OSC 667.
// Only termdesk-* apps implement this protocol. Sending SIGUSR1 to a regular
// shell (bash/zsh) would kill it since the default action is termination.
func isAppStateCapable(command string) bool {
	base := filepath.Base(command)
	return strings.HasPrefix(base, "termdesk-")
}

// isVimEditor returns true if the command is nvim or vim.
func isVimEditor(command string) bool {
	base := filepath.Base(command)
	return base == "nvim" || base == "vim"
}

// vimSessionsDir returns the directory for vim session files.
func vimSessionsDir() string {
	home := appHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "termdesk", "sessions")
}

// vimSessionPath returns the session file path for a given window ID.
func vimSessionPath(windowID string) string {
	dir := vimSessionsDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("vim-%s.vim", windowID))
}

// signalAppsForCapture sends vim :mksession and SIGUSR1/OSC 667 signals
// to prepare apps for workspace state capture. This is fast (no blocking).
// Call this before captureWorkspaceState() — for auto-save, responses from
// the previous cycle's signals are already available.
func (m *Model) signalAppsForCapture() {
	// Build a map of foreground commands per window (for detecting nvim in shells).
	type fgInfo struct {
		command string
		isVim   bool
	}
	fgCommands := make(map[string]fgInfo)
	for _, w := range m.wm.Windows() {
		if term, ok := m.terminals[w.ID]; ok {
			cmd := term.ForegroundCommand()
			fgCommands[w.ID] = fgInfo{command: cmd, isVim: isVimEditor(cmd)}
		}
	}

	// Save vim sessions — check both stored command AND foreground process.
	sessDir := vimSessionsDir()
	for _, w := range m.wm.Windows() {
		fg := fgCommands[w.ID]
		if isVimEditor(w.Command) || fg.isVim {
			if term, ok := m.terminals[w.ID]; ok {
				if sessDir != "" {
					os.MkdirAll(sessDir, 0755)
					sessPath := vimSessionPath(w.ID)
					term.WriteInput([]byte("\x1b:silent! wall\r:mksession! " + sessPath + "\r"))
				}
			}
		}
	}

	// Request app state from terminals that support the protocol.
	for _, w := range m.wm.Windows() {
		if isAppStateCapable(w.Command) {
			if term, ok := m.terminals[w.ID]; ok {
				_ = term.RequestAppState()
			}
		}
	}
}

// captureWorkspaceState creates a WorkspaceState from current Model state.
// Does NOT block — call signalAppsForCapture() ahead of time (or rely on
// previous auto-save cycle's signals) to ensure vim/app state is fresh.
func (m *Model) captureWorkspaceState() workspace.WorkspaceState {
	focusedWin := m.wm.FocusedWindow()
	focusedID := ""
	if focusedWin != nil {
		focusedID = focusedWin.ID
	}
	state := workspace.WorkspaceState{
		FocusedID: focusedID,
	}

	// Build a map of foreground commands per terminal (for detecting nvim in shells).
	type fgInfo struct {
		command string // foreground process name (e.g. "nvim")
		isVim   bool
	}
	fgCommands := make(map[string]fgInfo)
	for termID, term := range m.terminals {
		cmd := term.ForegroundCommand()
		fgCommands[termID] = fgInfo{command: cmd, isVim: isVimEditor(cmd)}
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// captureTermState captures command, workdir, and buffer for a single terminal.
	captureTermState := func(termID string, w *window.Window) (cmd string, args []string, workDir, bufFile string, bufRows, bufCols int) {
		cmd = w.Command
		if cmd == "" {
			cmd = shell
		}
		args = w.Args

		// If foreground process is vim/nvim but stored command is a shell,
		// override to launch vim directly on restore (with session file).
		fg := fgCommands[termID]
		if fg.isVim && !isVimEditor(cmd) {
			cmd = fg.command
			args = nil
		}

		workDir = w.WorkDir
		if term, ok := m.terminals[termID]; ok {
			if liveCWD := term.GetCWD(); liveCWD != "" {
				workDir = liveCWD
			}
			bufContent := term.CaptureFullBuffer(1000)
			if id := workspace.SaveBuffer(termID, bufContent); id != "" {
				bufFile = id
			}
			bufRows = term.Height()
			bufCols = term.Width()
		}
		return
	}

	// Capture windows (skip exited — quake terminal is standalone, not a window)
	for _, w := range m.wm.Windows() {
		if w.Exited {
			continue
		}

		cmd, args, workDir, _, _, _ := captureTermState(w.ID, w)

		ws := workspace.WindowState{
			ID:        w.ID,
			Title:     w.Title,
			Command:   cmd,
			Args:      args,
			WorkDir:   workDir,
			X:         w.Rect.X,
			Y:         w.Rect.Y,
			Width:     w.Rect.Width,
			Height:    w.Rect.Height,
			ZIndex:    w.ZIndex,
			Minimized: w.Minimized,
			Resizable: w.Resizable,
		}

		if w.PreMaxRect != nil {
			ws.PreMaxRect = &geometry.Rect{
				X:      w.PreMaxRect.X,
				Y:      w.PreMaxRect.Y,
				Width:  w.PreMaxRect.Width,
				Height: w.PreMaxRect.Height,
			}
		}

		if w.IsSplit() {
			// Capture split layout and per-pane terminal data
			ws.SplitTree = window.EncodeSplitTree(w.SplitRoot)
			ws.FocusedPane = w.FocusedPane
			paneIDs := w.SplitRoot.AllTermIDs()
			for _, paneID := range paneIDs {
				pCmd, pArgs, pWorkDir, pBufFile, pBufRows, pBufCols := captureTermState(paneID, w)
				// Per-pane command: use the terminal's own ForegroundCommand or shell
				if term, ok := m.terminals[paneID]; ok {
					fgCmd := term.ForegroundCommand()
					if fgCmd != "" && fgCmd != shell {
						pCmd = fgCmd
						pArgs = nil
					}
				}
				ws.Panes = append(ws.Panes, workspace.PaneState{
					TermID:     paneID,
					Command:    pCmd,
					Args:       pArgs,
					WorkDir:    pWorkDir,
					BufferFile: pBufFile,
					BufferRows: pBufRows,
					BufferCols: pBufCols,
				})
			}
		} else {
			// Single terminal — capture buffer and app state
			if term, ok := m.terminals[w.ID]; ok {
				bufContent := term.CaptureFullBuffer(1000)
				if id := workspace.SaveBuffer(w.ID, bufContent); id != "" {
					ws.BufferFile = id
				}
				ws.BufferRows = term.Height()
				ws.BufferCols = term.Width()
				if appState := term.AppState(); appState != "" {
					ws.AppStateData = appState
				}
			}
		}

		state.Windows = append(state.Windows, ws)
	}

	// Capture clipboard history (limit item length to avoid huge files)
	if m.clipboard != nil {
		rawHistory := m.clipboard.GetHistory()
		for _, item := range rawHistory {
			// Limit each clipboard item to 1000 characters
			if len(item) > 1000 {
				item = item[:1000] + "... [truncated]"
			}
			state.Clipboard = append(state.Clipboard, item)
		}
	}

	// Note: We don't save notifications - they're transient UI state

	// Cleanup stale buffer files (windows that no longer exist).
	activeIDs := make(map[string]bool, len(state.Windows))
	for _, ws := range state.Windows {
		if ws.BufferFile != "" {
			activeIDs[ws.BufferFile] = true
		}
		for _, p := range ws.Panes {
			if p.BufferFile != "" {
				activeIDs[p.BufferFile] = true
			}
		}
	}
	workspace.CleanupBuffers(activeIDs)

	return state
}

// updateWorkspaceWidget sets the workspace widget name from the given directory.
func (m *Model) updateWorkspaceWidget(dir string) {
	if m.workspaceWidget == nil {
		return
	}
	if dir == "" {
		m.workspaceWidget.DisplayName = "Default"
		return
	}
	m.workspaceWidget.DisplayName = filepath.Base(dir)
}

// restoreWorkspace restores windows and state from saved WorkspaceState.
// If projectDir is non-empty, changes the working directory to that path.
func (m *Model) restoreWorkspace(state *workspace.WorkspaceState, projectDir string) {
	m.updateWorkspaceWidget(projectDir)

	// Change to workspace directory if specified
	if projectDir != "" {
		if err := os.Chdir(projectDir); err != nil {
			dbg("Failed to chdir to workspace directory %s: %v", projectDir, err)
		} else {
			dbg("Changed directory to workspace: %s", projectDir)
		}
	}

	// Restore windows
	for _, ws := range state.Windows {
		// Create window
		rect := geometry.Rect{
			X:      ws.X,
			Y:      ws.Y,
			Width:  ws.Width,
			Height: ws.Height,
		}

		win := window.NewWindow(ws.ID, ws.Title, rect, nil)
		win.Command = ws.Command
		win.Args = ws.Args
		win.WorkDir = ws.WorkDir
		win.ZIndex = ws.ZIndex
		win.Minimized = ws.Minimized
		win.Resizable = ws.Resizable
		win.TitleBarHeight = m.theme.TitleBarRows()
		win.Icon, win.IconColor = m.windowIcon(ws.Command)
		if ws.PreMaxRect != nil {
			win.PreMaxRect = &geometry.Rect{
				X:      ws.PreMaxRect.X,
				Y:      ws.PreMaxRect.Y,
				Width:  ws.PreMaxRect.Width,
				Height: ws.PreMaxRect.Height,
			}
		}

		m.wm.AddWindow(win)

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		if ws.SplitTree != "" && len(ws.Panes) > 0 {
			// Restore split window: reconstruct tree and create per-pane terminals
			win.SplitRoot = window.DecodeSplitTree(ws.SplitTree)
			win.FocusedPane = ws.FocusedPane
			if win.SplitRoot == nil {
				continue // corrupted tree — skip
			}
			cr := win.SplitContentRect()
			panes := win.SplitRoot.Layout(cr)

			// Build lookup for pane data by term ID
			paneData := make(map[string]*workspace.PaneState, len(ws.Panes))
			for i := range ws.Panes {
				paneData[ws.Panes[i].TermID] = &ws.Panes[i]
			}

			for _, pr := range panes {
				pd := paneData[pr.TermID]
				if pd == nil {
					continue
				}
				pCmd := pd.Command
				if pCmd == "" {
					pCmd = shell
				}
				pWorkDir := pd.WorkDir
				if pWorkDir == "" {
					pWorkDir = ws.WorkDir
				}

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

				var paneTerm *terminal.Terminal
				var pErr error
				if pCmd == shell {
					paneTerm, pErr = terminal.NewShell(pr.Rect.Width, pr.Rect.Height,
						m.cellPixelW, m.cellPixelH, pWorkDir, graphicsEnv...)
				} else {
					paneTerm, pErr = terminal.New(pCmd, pd.Args, pr.Rect.Width, pr.Rect.Height,
						m.cellPixelW, m.cellPixelH, pWorkDir, graphicsEnv...)
				}
				if pErr != nil || paneTerm == nil {
					continue
				}
				tc := m.theme.C()
				paneTerm.SetDefaultColors(tc.DefaultFg, tc.ContentBg)
				m.terminals[pr.TermID] = paneTerm
				m.termCreatedAt[pr.TermID] = time.Now()

				// Restore buffer
				if pd.BufferFile != "" {
					if bufContent := workspace.LoadBuffer(pd.BufferFile); bufContent != "" {
						m.windowBuffers[pr.TermID] = bufContent
						paneTerm.RestoreBuffer(bufContent)
					}
				}
				m.spawnPTYReader(pr.TermID, paneTerm)
			}
		} else {
			// Restore single terminal window
			contentW := win.Rect.Width - 2
			contentH := win.Rect.Height - win.TitleBarHeight - 1

			var term *terminal.Terminal
			var err error
			// Pass app state via env var if available (SIGUSR1/OSC 667 protocol)
			var extraEnv []string
			if ws.AppStateData != "" {
				extraEnv = append(extraEnv, "TERMDESK_APP_STATE="+ws.AppStateData)
			}

			if ws.Command == "" || ws.Command == shell {
				term, err = terminal.NewShell(contentW, contentH, m.cellPixelW, m.cellPixelH, ws.WorkDir, extraEnv...)
			} else {
				term, err = terminal.New(ws.Command, ws.Args, contentW, contentH, m.cellPixelW, m.cellPixelH, ws.WorkDir, extraEnv...)
			}

			if err == nil && term != nil {
				tc := m.theme.C()
				term.SetDefaultColors(tc.DefaultFg, tc.ContentBg)
				m.terminals[ws.ID] = term
				m.termCreatedAt[ws.ID] = time.Now()

				// Restore buffer content from external file to emulator (visual continuity).
				if ws.BufferFile != "" {
					if bufContent := workspace.LoadBuffer(ws.BufferFile); bufContent != "" {
						m.windowBuffers[ws.ID] = bufContent
						term.RestoreBuffer(bufContent)
					}
				}

				m.spawnPTYReader(ws.ID, term)

				// For vim/nvim: restore session via delayed :source command.
				if isVimEditor(ws.Command) {
					sessPath := vimSessionPath(ws.ID)
					if sessPath != "" {
						if _, err := os.Stat(sessPath); err == nil {
							t := term // capture for goroutine
							go func() {
								time.Sleep(500 * time.Millisecond)
								t.WriteInput([]byte("\x1b:source " + sessPath + "\r"))
							}()
						}
					}
				}
			}
		}
	}

	// Restore focused window — prefer a non-minimized window.
	// Terminal mode must never be active on a minimized window.
	if state.FocusedID != "" {
		m.wm.FocusWindow(state.FocusedID)
	}
	if fw := m.wm.FocusedWindow(); fw != nil && fw.Minimized {
		// The focused window is minimized — find a visible non-minimized window instead.
		found := false
		for _, w := range m.wm.Windows() {
			if w.Visible && !w.Minimized {
				m.wm.FocusWindow(w.ID)
				found = true
				break
			}
		}
		if !found {
			m.inputMode = ModeNormal
		}
	}

	// Restore clipboard
	if m.clipboard != nil && len(state.Clipboard) > 0 {
		m.clipboard.RestoreHistory(state.Clipboard)
	}

	// Note: We don't restore notifications - they're transient UI state, not part of the workspace
	// nextTerminalNumber() dynamically computes the next available number from window titles.
}

// switchToProject switches to a different project workspace.
func (m *Model) switchToProject(projectPath string) tea.Cmd {
	// Save current workspace to project-specific file
	currentProjectDir := ""
	if m.projectConfig != nil {
		currentProjectDir = m.projectConfig.ProjectDir
	}
	state := m.captureWorkspaceState()
	go func() {
		if err := workspace.SaveWorkspace(state, currentProjectDir); err != nil {
			logging.Error("workspace save failed: %v", err)
		}
	}()

	// Release cached render buffers before switching workspace
	for id, entry := range m.windowCache {
		if entry != nil && entry.buf != nil {
			ReleaseBuffer(entry.buf)
		}
		delete(m.windowCache, id)
	}

	// Close all windows and terminals
	for id := range m.terminals {
		if term, ok := m.terminals[id]; ok {
			term.Close()
		}
		delete(m.terminals, id)
	}
	m.wm = window.NewManager(m.width, m.height)
	m.wm.SetBounds(m.width, m.height)
	m.wm.SetReserved(1, 1)

	// Load new project config
	newConfig, _ := config.FindProjectConfig(projectPath)
	m.projectConfig = newConfig
	m.autoStartTriggered = false

	// Load workspace from new project
	if ws, err := workspace.LoadWorkspace(projectPath); err == nil && ws != nil {
		m.restoreWorkspace(ws, projectPath)
	}
	m.updateDockReserved()

	// Trigger auto-start for new project
	if m.projectConfig != nil && len(m.projectConfig.AutoStart) > 0 {
		m.autoStartTriggered = true
		return m.runProjectAutoStart()
	}

	return nil
}

// saveWorkspaceNow manually saves the current workspace immediately.
// Signals apps and captures state non-blocking — relies on auto-save
// having already sent signals in a previous cycle.
func (m *Model) saveWorkspaceNow() {
	projectDir := ""
	if m.projectConfig != nil {
		projectDir = m.projectConfig.ProjectDir
	}

	m.signalAppsForCapture()
	state := m.captureWorkspaceState()
	wsName := "Global Workspace"
	if projectDir != "" {
		wsName = filepath.Base(projectDir)
	}
	m.notifications.Push("Workspace Saved", fmt.Sprintf("Saved workspace: %s", wsName), notification.Info)
	m.lastWorkspaceSave = time.Now()
	go func() {
		if err := workspace.SaveWorkspace(state, projectDir); err != nil {
			logging.Error("workspace save failed: %v", err)
		}
	}()
}

// toggleWorkspacePicker shows/hides the workspace picker.
// Discovery runs asynchronously via tea.Cmd to avoid blocking the event loop.
func (m *Model) toggleWorkspacePicker() tea.Cmd {
	m.workspacePickerVisible = !m.workspacePickerVisible
	if m.workspacePickerVisible {
		m.workspacePickerSelected = 0
		m.workspaceList = nil
		m.workspaceWindowCounts = nil
		// Run discovery in background goroutine
		return func() tea.Msg {
			paths := discoverWorkspaces()
			counts := make([]int, len(paths))
			for i, wsPath := range paths {
				dir := filepath.Dir(wsPath)
				if ws, err := workspace.LoadWorkspace(dir); err == nil && ws != nil {
					counts[i] = len(ws.Windows)
				}
			}
			return WorkspaceDiscoveryMsg{Workspaces: paths, WindowCounts: counts}
		}
	}
	return nil
}

// loadSelectedWorkspace loads the workspace selected in the picker.
func (m *Model) loadSelectedWorkspace() tea.Cmd {
	if m.workspacePickerSelected < 0 || m.workspacePickerSelected >= len(m.workspaceList) {
		return nil
	}
	return m.switchToWorkspace(m.workspaceList[m.workspacePickerSelected])
}

// stashCurrentWorkspace saves the current workspace state to disk and stashes
// the window manager + terminals in backgroundWorkspaces so processes stay alive.
func (m *Model) stashCurrentWorkspace() {
	currentProjectDir := ""
	if m.projectConfig != nil {
		currentProjectDir = m.projectConfig.ProjectDir
	}

	// Save workspace state to disk
	state := m.captureWorkspaceState()
	go func() {
		if err := workspace.SaveWorkspace(state, currentProjectDir); err != nil {
			logging.Error("workspace save failed: %v", err)
		}
	}()

	// Release cached render buffers before stashing workspace
	for id, entry := range m.windowCache {
		if entry != nil && entry.buf != nil {
			ReleaseBuffer(entry.buf)
		}
		delete(m.windowCache, id)
	}

	// Stash WM + terminals in background (processes stay alive)
	wsPath := m.activeWorkspacePath
	if wsPath != "" {
		m.backgroundWorkspaces[wsPath] = &backgroundWorkspace{
			wm:        m.wm,
			terminals: m.terminals,
		}
	} else {
		// No active path means global workspace — close terminals
		for _, term := range m.terminals {
			term.Close()
		}
	}
}

// switchToWorkspace handles the full workspace switch:
// 1. Stash current workspace (keep processes alive)
// 2. Restore from background if already opened, else load from disk
func (m *Model) switchToWorkspace(workspacePath string) tea.Cmd {
	projectDir := filepath.Dir(workspacePath)
	wsName := "Global Workspace"
	if projectDir != "" {
		wsName = filepath.Base(projectDir)
	}

	// Don't switch to the same workspace
	if workspacePath == m.activeWorkspacePath {
		m.notifications.Push("Already Active", fmt.Sprintf("%s is already the active workspace", wsName), notification.Info)
		return nil
	}

	// Stash current workspace
	m.stashCurrentWorkspace()

	// Change to workspace directory so new terminals get the right CWD
	if err := os.Chdir(projectDir); err != nil {
		dbg("switchToWorkspace: failed to chdir to %s: %v", projectDir, err)
	}

	// Check if target workspace is already in background (previously opened)
	if bg, ok := m.backgroundWorkspaces[workspacePath]; ok {
		// Restore from background — processes are still alive
		delete(m.backgroundWorkspaces, workspacePath)
		m.wm = bg.wm
		m.terminals = bg.terminals
		m.wm.SetBounds(m.width, m.height)
		m.updateDockReserved()
		m.wm.ClampAllWindows()
		m.resizeAllTerminals()
		m.activeWorkspacePath = workspacePath
		m.notifications.Push("Workspace Resumed", fmt.Sprintf("Resumed: %s (%d window(s))", wsName, len(m.wm.Windows())), notification.Info)
		recordWorkspaceAccess(workspacePath)
	} else {
		// Fresh load from disk
		m.wm = window.NewManager(m.width, m.height)
		m.wm.SetBounds(m.width, m.height)
		m.wm.SetReserved(1, 1)
		m.terminals = make(map[string]*terminal.Terminal)
		m.paneRedirect = make(map[string]string)
		m.activeWorkspacePath = workspacePath

		if ws, err := workspace.LoadWorkspace(projectDir); err == nil && ws != nil {
			m.restoreWorkspace(ws, projectDir)
			m.notifications.Push("Workspace Loaded", fmt.Sprintf("Loaded: %s (%d window(s))", wsName, len(ws.Windows)), notification.Info)
			recordWorkspaceAccess(workspacePath)
		} else if err != nil {
			m.notifications.Push("Load Failed", fmt.Sprintf("Could not load workspace %s: %s", wsName, err.Error()), notification.Error)
		}
	}

	m.updateDockReserved()

	// Update project config if this directory has one
	newConfig, _ := config.FindProjectConfig(projectDir)
	m.projectConfig = newConfig
	m.autoStartTriggered = false

	// Update workspace widget
	if m.workspaceWidget != nil {
		m.workspaceWidget.DisplayName = wsName
	}

	// Trigger auto-start for the project (dedup skips already-running commands)
	if m.projectConfig != nil && len(m.projectConfig.AutoStart) > 0 {
		m.autoStartTriggered = true
		return m.runProjectAutoStart()
	}

	return nil
}

// recordWorkspaceAccess adds a workspace to the access history in user config.
func recordWorkspaceAccess(workspacePath string) {
	cfg := config.LoadUserConfig()

	// Remove existing entry for this path
	var filtered []config.WorkspaceHistoryEntry
	for _, entry := range cfg.RecentWorkspaces {
		if entry.Path != workspacePath {
			filtered = append(filtered, entry)
		}
	}

	// Prepend new entry at the top
	newEntry := config.WorkspaceHistoryEntry{
		Path:       workspacePath,
		LastAccess: time.Now(),
	}
	cfg.RecentWorkspaces = append([]config.WorkspaceHistoryEntry{newEntry}, filtered...)

	// Cap at 20 workspaces
	if len(cfg.RecentWorkspaces) > 20 {
		cfg.RecentWorkspaces = cfg.RecentWorkspaces[:20]
	}

	// Save config
	if err := config.SaveUserConfig(cfg); err != nil {
		logging.Error("config save failed: %v", err)
	}
}

// loadWorkspaceFromHistory loads a workspace from the history picker selection.
func (m *Model) loadWorkspaceFromHistory(workspacePath string) tea.Cmd {
	return m.switchToWorkspace(workspacePath)
}

// loadRecentWorkspace loads a workspace by index from recent history (0-based).
func (m *Model) loadRecentWorkspace(index int) tea.Cmd {
	cfg := config.LoadUserConfig()
	if index < 0 || index >= len(cfg.RecentWorkspaces) {
		m.notifications.Push(
			"Workspace Shortcut",
			fmt.Sprintf("No workspace bound to slot %d", index+1),
			notification.Info,
		)
		return nil
	}
	return m.loadWorkspaceFromHistory(cfg.RecentWorkspaces[index].Path)
}

// createNewWorkspace creates a new blank workspace at the specified location.
// Saves the current workspace first, then closes all windows before switching.
func (m *Model) createNewWorkspace(name, path string) {
	// Ensure path is absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		m.notifications.Push("Invalid Path", fmt.Sprintf("Could not resolve path: %s", err.Error()), notification.Error)
		return
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(absPath, 0755); err != nil {
		m.notifications.Push("Create Failed", fmt.Sprintf("Could not create directory: %s", err.Error()), notification.Error)
		return
	}

	// Create blank workspace state
	blankState := workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		Windows:   []workspace.WindowState{},
		FocusedID: "",
		Clipboard: []string{},
	}

	// Save workspace file
	if err := workspace.SaveWorkspace(blankState, absPath); err != nil {
		m.notifications.Push("Save Failed", fmt.Sprintf("Could not save workspace: %s", err.Error()), notification.Error)
		return
	}

	// Optionally create a .termdesk.toml project config file with the name
	projectConfigPath := filepath.Join(absPath, ".termdesk.toml")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		configContent := fmt.Sprintf("# Termdesk project configuration\n# Project: %s\n\n# Uncomment to add auto-start commands:\n# [[autostart]]\n# command = \"your-command\"\n# args = []\n# directory = \".\"\n# title = \"Window Title\"\n", name)
		os.WriteFile(projectConfigPath, []byte(configContent), 0644)
	}

	// Save old workspace and close all existing windows before switching.
	// Without this, old windows persist and get auto-saved onto the new workspace.
	m.stashCurrentWorkspace()
	m.wm = window.NewManager(m.width, m.height)
	m.wm.SetBounds(m.width, m.height)
	m.wm.SetReserved(1, 1)
	m.terminals = make(map[string]*terminal.Terminal)

	// Switch to new workspace directory
	if err := os.Chdir(absPath); err != nil {
		dbg("createNewWorkspace: failed to chdir to %s: %v", absPath, err)
	}

	m.activeWorkspacePath = workspace.GetWorkspacePath(absPath)
	m.updateDockReserved()

	// Update project config
	newConfig, _ := config.FindProjectConfig(absPath)
	m.projectConfig = newConfig
	m.autoStartTriggered = false

	m.updateWorkspaceWidget(absPath)
	recordWorkspaceAccess(m.activeWorkspacePath)
	m.notifications.Push("Workspace Created", fmt.Sprintf("Created new workspace: %s", name), notification.Info)
}

// discoverWorkspaces finds all .termdesk-workspace.toml files starting from home directory.
func discoverWorkspaces() []string {
	var workspaces []string

	home := appHomeDir()
	if home == "" {
		return workspaces
	}

	// Add global workspace
	globalPath := filepath.Join(home, ".config", "termdesk", "workspace.toml")
	if _, err := os.Stat(globalPath); err == nil {
		workspaces = append(workspaces, globalPath)
	}

	// Search for project workspaces in common directories.
	// On case-insensitive filesystems (macOS), "Projects" and "projects"
	// resolve to the same dir — deduplicate search dirs by real path.
	searchDirs := []string{
		filepath.Join(home, "Projects"),
		filepath.Join(home, "projects"),
		filepath.Join(home, "Sites"),
		filepath.Join(home, "sites"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "code"),
		filepath.Join(home, "Dev"),
		filepath.Join(home, "dev"),
		filepath.Join(home, "src"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "git"),
	}

	// Deduplicate search dirs and results by device+inode.
	// On macOS (case-insensitive FS), "Sites" and "sites" are the same
	// directory but resolve to different strings — inode comparison is reliable.
	seenDirs := make(map[string]bool)
	seenResults := make(map[string]bool)

	fileKey := func(fi os.FileInfo) string {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			return fmt.Sprintf("%d:%d", st.Dev, st.Ino)
		}
		return ""
	}

	// Heavy directories that can never contain workspace files
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, "target": true,
		"build": true, "dist": true, "__pycache__": true,
		".cache": true, ".npm": true, ".cargo": true,
	}

	for _, dir := range searchDirs {
		fi, err := os.Stat(dir)
		if err != nil {
			continue
		}
		key := fileKey(fi)
		if key == "" || seenDirs[key] {
			continue
		}
		seenDirs[key] = true
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if info.IsDir() {
				name := info.Name()
				// Skip hidden directories
				if strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				// Skip known heavy directories
				if skipDirs[name] {
					return filepath.SkipDir
				}
			}
			// Limit depth to avoid scanning too deep
			relPath, _ := filepath.Rel(dir, path)
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth > 3 {
				return filepath.SkipDir
			}
			if !info.IsDir() && info.Name() == ".termdesk-workspace.toml" {
				rk := fileKey(info)
				if rk == "" || !seenResults[rk] {
					if rk != "" {
						seenResults[rk] = true
					}
					workspaces = append(workspaces, path)
				}
				// Project root found — skip remaining entries in this
				// directory (subdirectories). Projects don't nest, so
				// there's nothing useful deeper inside.
				// .termdesk-workspace.toml sorts before most dir names
				// (starts with '.'), so Walk hasn't entered subdirs yet.
				return filepath.SkipDir
			}
			return nil
		})
	}

	return workspaces
}

// renderWorkspacePicker draws the workspace picker overlay.
func (m *Model) renderWorkspacePicker(buf *Buffer) {
	workspaces := m.workspaceList

	if len(workspaces) == 0 {
		return
	}

	// Calculate dimensions (centered overlay)
	maxWidth := 70
	if m.width < maxWidth {
		maxWidth = m.width - 4
	}
	maxHeight := len(workspaces) + 4
	if maxHeight > m.height-4 {
		maxHeight = m.height - 4
	}

	x := (m.width - maxWidth) / 2
	y := (m.height - maxHeight) / 2

	// Parse theme colors
	bgColor := hexToColor(m.theme.ActiveBorderBg)
	fgColor := hexToColor(m.theme.MenuBarFg)
	borderColor := hexToColor(m.theme.ActiveBorderFg)
	subtleColor := hexToColor(m.theme.SubtleFg)

	// Draw semi-transparent backdrop
	for dy := 0; dy < maxHeight; dy++ {
		for dx := 0; dx < maxWidth; dx++ {
			buf.SetCell(x+dx, y+dy, ' ', fgColor, bgColor, 0)
		}
	}

	// Title
	title := " Load Workspace "
	titleX := x + (maxWidth-len(title))/2
	for i, ch := range title {
		buf.SetCell(titleX+i, y, ch, fgColor, bgColor, 0)
	}

	// Draw border
	for dx := 0; dx < maxWidth; dx++ {
		buf.SetCell(x+dx, y+1, '\u2500', borderColor, bgColor, 0)
	}

	// Workspace list
	displayCount := maxHeight - 4
	if displayCount > len(workspaces) {
		displayCount = len(workspaces)
	}

	for i := 0; i < displayCount; i++ {
		wsPath := workspaces[i]
		// Extract meaningful name from path
		var displayName string
		if strings.Contains(wsPath, ".config/termdesk") {
			displayName = "Global Workspace"
		} else {
			dir := filepath.Dir(wsPath)
			displayName = filepath.Base(dir)
			// Show parent directory for context
			parent := filepath.Base(filepath.Dir(dir))
			if parent != "." && parent != "/" {
				displayName = parent + "/" + displayName
			}
		}

		// Append window count if available
		if i < len(m.workspaceWindowCounts) {
			n := m.workspaceWindowCounts[i]
			if n == 1 {
				displayName += " (1 win)"
			} else {
				displayName += fmt.Sprintf(" (%d wins)", n)
			}
		}

		lineY := y + 2 + i
		lineText := "  " + displayName

		// Truncate if needed
		maxTextLen := maxWidth - 2
		if len(lineText) > maxTextLen {
			lineText = lineText[:maxTextLen-3] + "..."
		}

		fg := fgColor
		bg := bgColor
		if i == m.workspacePickerSelected {
			// Highlight selected
			fg = bgColor
			bg = fgColor
			lineText = ">" + lineText[1:]
		}

		for j, ch := range lineText {
			buf.SetCell(x+j, lineY, ch, fg, bg, 0)
		}
		// Fill rest of line with background
		for j := len(lineText); j < maxWidth; j++ {
			buf.SetCell(x+j, lineY, ' ', fg, bg, 0)
		}
	}

	// Footer hint
	hint := "\u2191\u2193: Navigate  Enter: Load  Esc: Cancel"
	hintY := y + maxHeight - 1
	hintX := x + (maxWidth-len(hint))/2
	for i, ch := range hint {
		buf.SetCell(hintX+i, hintY, ch, subtleColor, bgColor, 0)
	}
}
