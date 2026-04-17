package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

const workspaceRestoreSizeSettle = 150 * time.Millisecond

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var updateStart time.Time
	if m.perf != nil && m.perf.enabled {
		updateStart = time.Now()
	}
	retModel, retCmd := m.handleUpdate(msg)

	// CRITICAL: Use retModel (post-handleUpdate state) for overlay checks.
	// Using the pre-update `m` would see stale overlay state — e.g. a context
	// menu that was just dismissed still shows as active, permanently hiding
	// images because hasActiveOverlay() returns true on the old state.
	ret := retModel.(Model)

	// --- Unified image suppression (Kitty + sixel/iTerm2) ---
	//
	// Images are hidden during any visual movement (drag, animation, scroll)
	// and re-shown once the UI settles. This prevents:
	//   - Kitty trail artifacts (old placement stays until replaced)
	//   - Sixel pixel corruption (no "delete" command for raw pixels)
	//   - Performance overhead from per-frame refresh during 60fps events
	//
	// Deferred refresh via ImageRefreshMsg ensures sixel/iTerm2 data arrives
	// AFTER the frame render (BT v2 flushes tea.Raw before renderer).
	// The refreshPending flag deduplicates: at most ONE ImageRefreshMsg queued.
	hasKitty := ret.kittyPass != nil && ret.kittyPass.HasPlacements()
	hasImages := ret.imagePass != nil && ret.imagePass.HasImagePlacements()

	if hasKitty || hasImages {
		shouldSuppress := ret.drag.Active ||
			ret.hasActiveAnimations() ||
			ret.inputMode == ModeCopy

		switch {
		case shouldSuppress:
			// Hide all images once when entering suppressed state.
			if !ret.imageState.hidden {
				ret.imageState.hidden = true
				if hasKitty {
					ret.kittyPass.HideAllPlacements()
					if out := ret.kittyPass.FlushPending(); len(out) > 0 {
						ret.kittyPending.data = append(ret.kittyPending.data, out...)
					}
				}
				// Sixel/iTerm2: just stop rendering (no erase command exists).
			}
			ret.imageState.refreshPending = false // cancel any pending

		case ret.imageState.hidden:
			// Transition from hidden → visible. Schedule ONE deferred refresh.
			ret.imageState.hidden = false
			ret.imageState.refreshPending = true
			retCmd = appendCmd(retCmd, func() tea.Msg { return ImageRefreshMsg{} })

		default:
			// Normal (visible) state — handle per-event refresh.
			switch msg.(type) {
			case ImageRefreshMsg:
				// Deferred refresh arrived. Refresh both protocols.
				ret.imageState.refreshPending = false
				if hasKitty {
					ret.refreshKittyPlacements()
				}
				if hasImages {
					ret.refreshImagePlacements()
				}
			case tea.RawMsg, CursorBlinkMsg, SystemStatsMsg, CleanupMsg,
				CustomWidgetResultMsg, BuiltinWidgetDataMsg,
				PtyOutputMsg,
				tea.MouseMotionMsg, tea.MouseWheelMsg,
				tea.MouseClickMsg, tea.MouseReleaseMsg,
				BellMsg, KittyFlushMsg,
				WorkspaceAutoSaveMsg, WorkspaceDiscoveryMsg,
				ResizeSettleTickMsg:
				// No-op events: no window position/size changes.
				// Mouse events and terminal output don't move windows,
				// so images don't need repositioning. Skipping these
				// prevents flicker (tea.Raw is flushed BEFORE the
				// renderer, so re-emitting images every frame causes
				// them to be painted then immediately overwritten).
			default:
				// Position-affecting event (window move, resize, focus,
				// new image arrival). Schedule ONE deferred refresh
				// if not already pending (prevents flooding).
				if !ret.imageState.refreshPending {
					ret.imageState.refreshPending = true
					retCmd = appendCmd(retCmd, func() tea.Msg { return ImageRefreshMsg{} })
				}
			}
		}
	}

	// Flush pending Kitty graphics through BT's synchronized output pipeline.
	// Writing to /dev/tty races with BT's async renderer (different fd, same
	// terminal device). By using tea.Raw, the output goes through p.outputBuf
	// → p.flush() → p.output, which is serialized with p.renderer.flush()
	// in the same ticker goroutine. No interleaving, no cursor corruption.
	if ret.kittyPending != nil && len(ret.kittyPending.data) > 0 {
		raw := tea.Raw(string(ret.kittyPending.data))
		ret.kittyPending.data = nil // release backing array (can be multi-MB for images)
		if retCmd != nil {
			retCmd = tea.Batch(retCmd, raw)
		} else {
			retCmd = raw
		}
	}

	// Flush pending sixel/iTerm2 image data through the same pipeline.
	if ret.imagePending != nil && len(ret.imagePending.data) > 0 {
		raw := tea.Raw(string(ret.imagePending.data))
		ret.imagePending.data = nil // release backing array (can be multi-MB for images)
		if retCmd != nil {
			retCmd = tea.Batch(retCmd, raw)
		} else {
			retCmd = raw
		}
	}
	if ret.perf != nil && ret.perf.enabled {
		ret.perf.recordUpdate(time.Since(updateStart))
	}
	return ret, retCmd
}

func (m Model) handleUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	dirty := true
	defer func() {
		if dirty {
			m.cache.updateGen++ // invalidate view cache for this Update cycle
		}
	}()

	// Update dock running indicators in Update(), not View().
	// Calling this in View() mutated shared state (dock is a pointer) on every
	// render frame, causing dock slice corruption during rapid animations.
	m.updateDockRunning()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		dbg("WindowSizeMsg: %dx%d", msg.Width, msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		m.lastWindowSizeAt = time.Now()
		m.wm.SetBounds(msg.Width, msg.Height)
		m.updateDockReserved()
		m.wm.ClampAllWindows()
		m.menuBar.SetWidth(msg.Width)
		m.dock.SetWidth(msg.Width)
		m.ready = true
		// Resize quake terminal to match new screen size
		if m.quakeTerminal != nil && m.quakeVisible {
			cols, rows := m.quakeContentSize()
			m.quakeTerminal.Resize(cols, rows)
			m.quakeTargetH = float64(m.quakeFullHeight())
			if !m.quakeAnimating() {
				m.quakeAnimH = m.quakeTargetH
			}
		}
		// Launch wallpaper program terminal if configured and not yet running
		if m.wallpaperMode == "program" && m.wallpaperProgram != "" && m.wallpaperTerminal == nil {
			m.launchWallpaperTerminal()
		} else {
			m.resizeWallpaperTerminal()
		}
		m.resizeAllTerminals()
		if m.inputMode == ModeCopy {
			m.copySnapshot = nil
			m.scrollOffset = 0
			m.selActive = false
			m.inputMode = ModeNormal
		}

		// Invalidate per-window render cache so stale buffers aren't reused.
		for k := range m.windowCache {
			delete(m.windowCache, k)
		}

		var cmds []tea.Cmd
		// Start a self-sustaining resize settle ticker that keeps triggering
		// View() calls for ~1s after resize. This ensures terminal content
		// updates as child apps respond to SIGWINCH. Same proven pattern as
		// AnimationTickMsg (tea.Tick works reliably, goroutines did not).
		cmds = append(cmds, tickResizeSettle())
		// Keep tiled workspaces in sync with the actual terminal size.
		if m.tilingMode && m.wm.VisibleCount() > 0 {
			m.applyTilingLayout()
			if m.animationsOn {
				cmds = append(cmds, tickAnimation())
			}
		}

		// Trigger project auto-start if not already done and no workspace restore pending.
		if m.projectConfig != nil && !m.autoStartTriggered && !m.workspaceRestorePending && len(m.projectConfig.AutoStart) > 0 {
			m.autoStartTriggered = true
			cmds = append(cmds, m.runProjectAutoStart())
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tea.FocusMsg:
		// Terminal regained focus (e.g. user switched back from another iTerm2 tab).
		// Exit copy mode so the user isn't stuck in vim-navigation after refocusing.
		if m.inputMode == ModeCopy {
			m.exitCopyMode()
		}
		return m, nil

	case tea.KeyPressMsg:
		dbg("key: %s mode=%s prefix=%v", msg.String(), m.inputMode, m.prefixPending)
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClick(tea.Mouse(msg))

	case tea.MouseMotionMsg:
		return m.handleMouseMotion(tea.Mouse(msg))

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(tea.Mouse(msg))

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(tea.Mouse(msg))

	case tea.PasteMsg:
		// Handle paste from external clipboard (Ctrl+Shift+V or terminal paste)
		if m.inputMode == ModeTerminal {
			if _, term := m.focusedTerminal(); term != nil {
				term.WriteInput([]byte(msg.Content))
			}
		}
		return m, nil

	case tea.RawMsg:
		// BT feeds back RawMsg after processing our tea.Raw() cmds.
		// Nothing to do — the raw data was already written by p.execute().
		dirty = false
		return m, nil

	case KittyFlushMsg:
		// Queue Kitty graphics APC data for flushing via tea.Raw() wrapper.
		if len(msg.Data) > 0 {
			m.kittyPending.data = append(m.kittyPending.data, msg.Data...)
		}
		return m, nil

	case ImageFlushMsg:
		// Queue sixel/iTerm2 image data for flushing via tea.Raw() wrapper.
		if len(msg.Data) > 0 {
			m.imagePending.data = append(m.imagePending.data, msg.Data...)
		}
		return m, nil

	case ImageRefreshMsg:
		// Deferred image refresh catch-up. Don't dirty the cache —
		// no frame redraw needed, just re-send images at current positions.
		dirty = false
		return m, nil

	case ImageClearScreenMsg:
		// Force full repaint to overwrite stale sixel pixels that were
		// injected via tea.Raw() and invisible to BT's diff renderer.
		return m, tea.ClearScreen

	case CustomWidgetResultMsg:
		for name, output := range msg.Results {
			if cw, ok := m.customWidgets[name]; ok {
				cw.SetOutput(output)
			}
		}
		return m, nil

	case BuiltinWidgetDataMsg:
		if m.widgetBar != nil {
			for _, w := range m.widgetBar.Widgets {
				switch ww := w.(type) {
				case *widget.DiskWidget:
					if msg.Refreshed["disk"] {
						ww.UsedPct = msg.DiskPct
					}
				case *widget.LoadWidget:
					if msg.Refreshed["load"] {
						ww.LoadAvg = msg.LoadAvg
						ww.NumCPU = msg.NumCPU
					}
				case *widget.GitBranchWidget:
					if msg.Refreshed["git"] {
						ww.Branch = msg.GitBranch
						ww.Dirty = msg.GitDirty
					}
				case *widget.DockerWidget:
					if msg.Refreshed["docker"] {
						ww.Count = msg.DockerCount
						ww.Available = msg.DockerAvail
					}
				case *widget.WeatherWidget:
					if msg.Refreshed["weather"] {
						ww.Text = msg.WeatherText
					}
				case *widget.MailWidget:
					if msg.Refreshed["mail"] {
						ww.Count = msg.MailCount
					}
				}
			}
		}
		return m, nil

	case BellMsg:
		// Bell rang in a terminal window.
		// Forward bell to parent terminal so the user's terminal/OS can notify.
		var cmds []tea.Cmd
		cmds = append(cmds, tea.Raw("\x07"))
		// Set HasBell on unfocused/minimized windows for visual indicator.
		fw := m.wm.FocusedWindow()
		if fw == nil || fw.ID != msg.WindowID {
			if w := m.wm.WindowByID(msg.WindowID); w != nil {
				w.HasBell = true
			}
		}
		return m, tea.Batch(cmds...)

	case PtyOutputMsg:
		// PTY produced output — just re-render (goroutine handles reads)
		// Quake terminal output — just redraw, no window state to update
		if msg.WindowID == quakeTermID {
			m.termHasOutput[quakeTermID] = true
			return m, nil
		}
		// Track first output for stuck terminal detection
		if !m.termHasOutput[msg.WindowID] {
			m.termHasOutput[msg.WindowID] = true
			// For split pane terminals, look up the parent window
			if w := m.windowForTerminal(msg.WindowID); w != nil && w.Stuck {
				w.Stuck = false
			}
		}
		// Mark unfocused windows as having notifications and activity
		// Use windowForTerminal to handle split pane terminals
		w := m.windowForTerminal(msg.WindowID)
		if fw := m.wm.FocusedWindow(); fw == nil || (w != nil && fw.ID != w.ID) {
			if w != nil {
				changed := false
				if !w.HasNotification {
					w.HasNotification = true
					changed = true
				}
				if !w.HasActivity {
					w.HasActivity = true
					changed = true
				}
				// If it's not visible and notification state didn't change, skip redraw.
				if !changed {
					if !w.Visible || w.Minimized {
						dirty = false
						return m, nil
					}
					wa := m.wm.WorkArea()
					frameRect := geometry.Rect{X: 0, Y: 0, Width: wa.Width, Height: wa.Height + wa.Y + m.wm.ReservedBottom()}
					if !w.Rect.Overlaps(frameRect) {
						dirty = false
						return m, nil
					}
				}
			}
		}
		return m, nil

	case SystemStatsMsg:
		// Auto-dismiss expired toast notifications
		m.notifications.Tick()
		// Detect stuck terminals — no output after 5 seconds
		for _, w := range m.wm.Windows() {
			hasOutput := m.termHasOutput[w.ID]
			if !hasOutput && w.IsSplit() {
				// Check if any pane in the split has output
				for _, id := range w.SplitRoot.AllTermIDs() {
					if m.termHasOutput[id] {
						hasOutput = true
						break
					}
				}
			}
			if w.Exited || hasOutput {
				if w.Stuck {
					w.Stuck = false
				}
				continue
			}
			if created, ok := m.termCreatedAt[w.ID]; ok {
				w.Stuck = time.Since(created) > 5*time.Second
			}
		}
		// Update widget bar widgets
		if m.widgetBar != nil {
			for _, w := range m.widgetBar.Widgets {
				switch ww := w.(type) {
				case *widget.CPUWidget:
					ww.Update(msg.CPU)
				case *widget.MemoryWidget:
					ww.UsedGB = msg.MemGB
					_, totalGB := widget.ReadMemoryInfo()
					ww.TotalGB = totalGB
				case *widget.BatteryWidget:
					ww.Pct = msg.BatPct
					ww.Charging = msg.BatCharging
					ww.Present = msg.BatPresent
				case *widget.NotificationWidget:
					ww.UnreadCount = m.notifications.UnreadCount()
				}
			}
		}
		// Refresh custom shell widgets asynchronously
		var cmds []tea.Cmd
		cmds = append(cmds, m.tickSystemStats())
		pending := make(map[string]string) // name → command
		for name, cw := range m.customWidgets {
			if cw.NeedsRefresh() {
				cw.MarkRun()
				pending[name] = cw.Command
			}
		}
		if len(pending) > 0 {
			cmds = append(cmds, func() tea.Msg {
				results := make(map[string]string)
				for name, command := range pending {
					if output, err := widget.RunCommand(command); err == nil {
						results[name] = output
					}
				}
				return CustomWidgetResultMsg{Results: results}
			})
		}
		// Refresh slow built-in widgets asynchronously (per-widget intervals).
		slowNeeded := make(map[string]bool)
		if m.widgetBar != nil {
			for _, w := range m.widgetBar.Widgets {
				switch ww := w.(type) {
				case *widget.DiskWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["disk"] = true
					}
				case *widget.LoadWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["load"] = true
					}
				case *widget.GitBranchWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["git"] = true
					}
				case *widget.DockerWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["docker"] = true
					}
				case *widget.WeatherWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["weather"] = true
					}
				case *widget.MailWidget:
					if ww.NeedsRefresh() {
						ww.MarkRefreshed()
						slowNeeded["mail"] = true
					}
				}
			}
		}
		if len(slowNeeded) > 0 {
			needed := slowNeeded // capture for closure
			cmds = append(cmds, func() tea.Msg {
				msg := BuiltinWidgetDataMsg{Refreshed: make(map[string]bool)}
				if needed["disk"] {
					msg.DiskPct = widget.ReadDiskInfo()
					msg.Refreshed["disk"] = true
				}
				if needed["load"] {
					msg.LoadAvg, msg.NumCPU = widget.ReadLoadAvg()
					msg.Refreshed["load"] = true
				}
				if needed["git"] {
					msg.GitBranch, msg.GitDirty = widget.ReadGitBranch()
					msg.Refreshed["git"] = true
				}
				if needed["docker"] {
					cnt := widget.ReadDockerCount()
					msg.DockerCount = cnt
					msg.DockerAvail = cnt >= 0
					if cnt < 0 {
						msg.DockerCount = 0
					}
					msg.Refreshed["docker"] = true
				}
				if needed["weather"] {
					msg.WeatherText = widget.ReadWeather()
					msg.Refreshed["weather"] = true
				}
				if needed["mail"] {
					msg.MailCount = widget.ReadMailCount()
					msg.Refreshed["mail"] = true
				}
				return msg
			})
		}
		return m, tea.Batch(cmds...)

	case WorkspaceRestoreMsg:
		if m.workspaceRestorePending {
			// Wait until we have the real terminal size; restoring against the
			// default bootstrap bounds (80x24) causes incorrect tiny layouts.
			if !m.ready {
				return m, tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg { return WorkspaceRestoreMsg{} })
			}
			// Debounce restore until size stabilizes. Some terminals/session
			// attaches emit an early small size followed by the real size.
			if !m.lastWindowSizeAt.IsZero() {
				elapsed := time.Since(m.lastWindowSizeAt)
				if elapsed < workspaceRestoreSizeSettle {
					wait := workspaceRestoreSizeSettle - elapsed
					return m, tea.Tick(wait, func(time.Time) tea.Msg { return WorkspaceRestoreMsg{} })
				}
			}
			m.workspaceRestorePending = false
			// Workspace loading priority:
			// 1. Project workspace (from .termdesk.toml project config)
			// 2. Last accessed workspace (from recent history)
			// 3. Global default workspace (~/.config/termdesk/workspace.toml)
			projectDir := ""
			if m.projectConfig != nil {
				projectDir = m.projectConfig.ProjectDir
			}
			restored := false

			// Priority 1: CWD workspace — always wins when projectDir is set.
			// Even if the workspace file is empty/missing, we claim this path
			// so auto-saves go to the right location (never fall through to
			// Priority 2/3 which would load a different project's workspace).
			if projectDir != "" {
				workspacePath := workspace.GetWorkspacePath(projectDir)
				m.activeWorkspacePath = workspacePath
				if state, err := workspace.LoadWorkspace(projectDir); err == nil && state != nil && len(state.Windows) > 0 {
					m.restoreWorkspace(state, projectDir)
					m.notifications.Push(
						"Workspace Restored",
						fmt.Sprintf("Restored %d window(s) from %s", len(state.Windows), filepath.Base(projectDir)),
						notification.Info,
					)
					recordWorkspaceAccess(workspacePath)
				}
				restored = true
			}

			// Priority 2: Last accessed workspace from history
			if !restored {
				cfg := config.LoadUserConfig()
				for _, entry := range cfg.RecentWorkspaces {
					if entry.Path == "" {
						continue
					}
					// Skip if it's the CWD workspace we already tried
					if projectDir != "" && entry.Path == workspace.GetWorkspacePath(projectDir) {
						continue
					}
					entryDir := workspace.ProjectDirFromPath(entry.Path)
					if state, err := workspace.LoadWorkspace(entryDir); err == nil && state != nil && len(state.Windows) > 0 {
						m.activeWorkspacePath = entry.Path
						m.restoreWorkspace(state, entryDir)
						label := "last session"
						if entryDir != "" {
							label = filepath.Base(entryDir)
						}
						m.notifications.Push(
							"Workspace Restored",
							fmt.Sprintf("Restored %d window(s) from %s", len(state.Windows), label),
							notification.Info,
						)
						recordWorkspaceAccess(entry.Path)
						restored = true
						break
					}
				}
			}

			// Priority 3: Global default workspace
			if !restored {
				workspacePath := workspace.GetWorkspacePath("")
				m.activeWorkspacePath = workspacePath
				if state, err := workspace.LoadWorkspace(""); err == nil && state != nil && len(state.Windows) > 0 {
					m.restoreWorkspace(state, "")
					m.notifications.Push(
						"Workspace Restored",
						fmt.Sprintf("Restored %d window(s) from saved session", len(state.Windows)),
						notification.Info,
					)
					recordWorkspaceAccess(workspacePath)
				}
			}

			// Trigger auto-start in a separate Update cycle so the restored
			// workspace state is fully committed before new windows are created.
			m.autoStartTriggered = true
			if m.projectConfig != nil && len(m.projectConfig.AutoStart) > 0 {
				return m, func() tea.Msg { return AutoStartMsg{} }
			}
		}
		return m, nil

	case AutoStartMsg:
		// Run project auto-start in its own Update cycle (after workspace restore).
		// Skips commands already running from the restored workspace.
		return m, m.runProjectAutoStart()

	case WorkspaceDiscoveryMsg:
		if m.workspacePickerVisible {
			if len(msg.Workspaces) == 0 {
				m.workspacePickerVisible = false
				m.notifications.Push(
					"No Workspaces Found",
					"No .termdesk-workspace.toml files found. Save a workspace first.",
					notification.Info,
				)
			} else {
				m.workspaceList = msg.Workspaces
				m.workspaceWindowCounts = msg.WindowCounts
			}
		}
		return m, nil

	case ResizeRedrawMsg:
		// Delayed repaint after terminal resize — terminal apps have had
		// time to respond to SIGWINCH and re-render their content.
		// Clear window cache and mark all terminals dirty so the next
		// View() re-snapshots the emulator with fresh content.
		for k := range m.windowCache {
			delete(m.windowCache, k)
		}
		for _, term := range m.terminals {
			term.MarkDirty()
		}
		return m, nil

	case ResizeSettleTickMsg:
		// Self-sustaining resize settle ticker. Keeps triggering View()
		// calls so terminal content updates as child apps respond to
		// SIGWINCH and redraw. Runs at 50ms intervals for ~1.2s after
		// the last resize event (same self-sustaining pattern as animations).
		if !m.lastWindowSizeAt.IsZero() && time.Since(m.lastWindowSizeAt) < resizeSettleDuration {
			// Still settling — invalidate caches and keep ticking.
			for k := range m.windowCache {
				delete(m.windowCache, k)
			}
			for _, term := range m.terminals {
				term.MarkDirty()
			}
			return m, tickResizeSettle()
		}
		// Resize settled — stop ticking. One final cache clear.
		for k := range m.windowCache {
			delete(m.windowCache, k)
		}
		for _, term := range m.terminals {
			term.MarkDirty()
		}
		return m, nil

	case WorkspaceAutoSaveMsg:
		// Auto-save based on configured interval
		interval := time.Duration(m.workspaceAutoSaveMin) * time.Minute
		if interval < time.Minute {
			interval = time.Minute // minimum 1 minute
		}
		// Two-phase approach to avoid blocking:
		// 1. Signal apps NOW (vim :mksession, SIGUSR1) — fast, non-blocking
		// 2. Capture state NOW — picks up responses from the PREVIOUS cycle's signals
		// Since auto-save runs every 60s, apps have had plenty of time to respond.
		m.signalAppsForCapture()
		// Check if enough time has passed (use slightly less to ensure we don't miss)
		if m.workspaceAutoSave && time.Since(m.lastWorkspaceSave) >= interval-5*time.Second {
			projectDir := ""
			if m.projectConfig != nil {
				projectDir = m.projectConfig.ProjectDir
			}
			state := m.captureWorkspaceState()
			go func() {
				_ = workspace.SaveWorkspace(state, projectDir)
			}()
			m.lastWorkspaceSave = time.Now()
		}
		return m, tickWorkspaceAutoSave()


	case CleanupMsg:
		// Periodic cleanup of old notifications (keep last 20)
		// Clipboard is self-limiting via ring buffer
		m.notifications.Cleanup(20)
		return m, tickCleanup()

	case AnimationTickMsg:
		if m.updateAnimations(msg.Time) {
			return m, tickAnimation()
		}
		return m, nil

	case CursorBlinkMsg:
		// Cursor blinking is now handled natively by tea.Cursor on the View.
		// No redraw needed — the terminal handles cursor blink itself.
		dirty = false
		return m, nil

	case launcher.ExecIndexReadyMsg:
		m.launcher.SetExecIndex([]string(msg))
		if m.launcher.Visible {
			m.launcher.RefreshResults()
		}
		return m, nil

	case PtyClosedMsg:
		// Wallpaper terminal — auto-restart if still in program mode.
		if msg.WindowID == wallpaperTermID {
			m.closeWallpaperTerminal()
			if m.wallpaperMode == "program" && m.wallpaperProgram != "" {
				m.launchWallpaperTerminal()
			}
			return m, nil
		}
		// Quake terminal — close and clean up. Next Ctrl+~ opens a fresh one.
		if msg.WindowID == quakeTermID {
			m.closeQuakeTerminal()
			return m, nil
		}
		// Resolve pane ID redirects from split-to-single reverts.
		// When a split reverts to single terminal, the surviving terminal's
		// PTY reader goroutine still has the old pane ID. Redirect it to
		// the window ID so the regular handler can mark it as exited.
		if redirect, ok := m.paneRedirect[msg.WindowID]; ok {
			delete(m.paneRedirect, msg.WindowID)
			msg.WindowID = redirect
		}
		// PTY exited — for split panes, auto-close the pane rather than holding open.
		errStr := ""
		if msg.Err != nil {
			errStr = msg.Err.Error()
		}
		// Check if this is a split pane terminal
		if parentWin := m.windowForTerminal(msg.WindowID); parentWin != nil && parentWin.IsSplit() && parentWin.SplitRoot.FindLeaf(msg.WindowID) != nil {
			// Auto-close the split pane
			if msg.Err != nil && !strings.Contains(errStr, "input/output error") && errStr != "EOF" {
				m.notifications.Push("Pane exited", msg.Err.Error(), notification.Warning)
			}
			// If this pane is focused, close it; otherwise just clean up the terminal
			if parentWin.FocusedPane == msg.WindowID {
				m.closeFocusedPane()
			} else {
				// Close unfocused pane
				m.closeTerminal(msg.WindowID)
				newRoot := parentWin.SplitRoot.RemoveLeaf(msg.WindowID)
				if newRoot == nil {
					m.wm.RemoveWindow(parentWin.ID)
					m.closeTerminal(parentWin.ID)
					m.inputMode = ModeNormal
				} else if newRoot.IsLeaf() {
					remainingID := newRoot.TermID
					if remainingID != parentWin.ID {
						m.terminals[parentWin.ID] = m.terminals[remainingID]
						delete(m.terminals, remainingID)
						delete(m.termCreatedAt, remainingID)
						delete(m.termHasOutput, remainingID)
						m.paneRedirect[remainingID] = parentWin.ID
					}
					parentWin.SplitRoot = nil
					parentWin.FocusedPane = ""
					m.resizeTerminalForWindow(parentWin)
				} else {
					parentWin.SplitRoot = newRoot
					ids := newRoot.AllTermIDs()
					if len(ids) > 0 {
						parentWin.FocusedPane = ids[0]
					}
					m.resizeAllPanes(parentWin)
				}
			}
			return m, nil
		}
		// Skip notification for normal PTY teardown errors (input/output error
		// from /dev/ptmx is expected when the child process exits).
		if msg.Err != nil && !strings.Contains(errStr, "input/output error") && errStr != "EOF" {
			title := "Process exited"
			if w := m.wm.WindowByID(msg.WindowID); w != nil {
				title = w.Title
			}
			m.notifications.Push(title, msg.Err.Error(), notification.Warning)
		}
		if w := m.wm.WindowByID(msg.WindowID); w != nil && !m.isAnimatingClose(msg.WindowID) {
			// Mark window as exited — keep it open with buffer visible.
			// The terminal/emulator stays alive so the user can read the output.
			if !w.Exited {
				w.Exited = true
				w.Title = w.Title + " [exited]"
			}
			// Clean up copy mode if the exiting window owns it.
			// Check both direct window ID and pane-to-window redirects
			// (a split pane terminal that was reverted to single).
			if m.inputMode == ModeCopy && m.copySnapshot != nil {
				copyWin := m.copySnapshot.WindowID
				if copyWin == msg.WindowID || copyWin == w.ID {
					m.exitCopyMode()
					m.inputMode = ModeNormal
				}
			}
			return m, nil
		}
		// Window already animating close or gone — clean up
		removed := m.wm.WindowByID(msg.WindowID) != nil
		m.closeTerminal(msg.WindowID)
		if removed {
			m.wm.RemoveWindow(msg.WindowID)
			m.updateDockReserved()
			// Prevent terminal mode on minimized/no windows after removal
			if m.inputMode == ModeTerminal {
				if fw := m.wm.FocusedWindow(); fw == nil || fw.Minimized {
					m.inputMode = ModeNormal
				}
			}
		}
		if removed && m.tilingMode {
			m.applyTilingLayout()
			return m, tickAnimation()
		}
		return m, nil
	}

	return m, nil
}

// appendCmd batches a new command with an existing one (which may be nil).
func appendCmd(existing tea.Cmd, cmd tea.Cmd) tea.Cmd {
	if existing != nil {
		return tea.Batch(existing, cmd)
	}
	return cmd
}

func (m Model) tickSystemStats() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		bat := widget.ReadBattery()
		usedGB, _ := widget.ReadMemoryInfo()
		return SystemStatsMsg{
			CPU:         widget.ReadCPUPercent(),
			MemGB:       usedGB,
			BatPct:      bat.Percent,
			BatCharging: bat.Charging,
			BatPresent:  bat.Present,
		}
	})
}
