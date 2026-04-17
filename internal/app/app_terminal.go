package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/logging"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

// kittyWindowGeom caches window geometry for goroutine-safe access from
// the onKittyGraphics callback. Updated from the BT main loop; read from
// emuWriteLoop goroutine. Keyed by window ID.
var kittyWindowGeom sync.Map // map[string]*kittyGeomSnapshot

type kittyGeomSnapshot struct {
	windowX, windowY         int
	windowWidth, windowH     int
	contentOffX, contentOffY int
}

// updateKittyGeom snapshots a window's geometry for the kitty callback.
// Must be called from the BT main goroutine (e.g. during rendering or after resize).
func updateKittyGeom(windowID string, w *window.Window) {
	cr := w.ContentRect()
	kittyWindowGeom.Store(windowID, &kittyGeomSnapshot{
		windowX:     w.Rect.X,
		windowY:     w.Rect.Y,
		windowWidth: w.Rect.Width,
		windowH:     w.Rect.Height,
		contentOffX: cr.X - w.Rect.X,
		contentOffY: cr.Y - w.Rect.Y,
	})
}

// removeKittyGeom removes cached geometry for a closed window.
func removeKittyGeom(windowID string) {
	kittyWindowGeom.Delete(windowID)
}

// kittyDbg is a temporary debug logger for Kitty graphics integration.
func kittyDbg(format string, args ...any) {
	f, err := os.OpenFile("/tmp/termdesk-kitty.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] ", time.Now().Format("15:04:05.000"))
	fmt.Fprintf(f, format+"\n", args...)
}

// newWindowID generates a random window ID (16 hex chars).
func newWindowID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// TerminalWindowOpts controls how createTerminalWindow creates a window.
type TerminalWindowOpts struct {
	Command      string   // command to run; empty = default shell
	Args         []string // command arguments
	DisplayName  string   // window title; empty = derived from command
	WorkDir      string   // working directory for the shell/command
	Maximized    bool     // fill work area, set PreMaxRect for restore
	FixedSize    bool     // non-resizable, centered in work area
	Width        int      // only used when FixedSize is true
	Height       int      // only used when FixedSize is true
	SkipAutoTile bool     // create window without auto-applying tiling mode
}

const maxWindows = 9

// nextTerminalNumber returns the lowest positive integer N such that no existing
// window has the title "Terminal N" or "Window N". This allows reuse of numbers
// after windows are closed (e.g. close Terminal 2, next terminal is Terminal 2).
func (m *Model) nextTerminalNumber() int {
	used := make(map[int]bool)
	for _, w := range m.wm.Windows() {
		var n int
		if _, err := fmt.Sscanf(w.Title, "Terminal %d", &n); err == nil {
			used[n] = true
		} else if _, err := fmt.Sscanf(w.Title, "Window %d", &n); err == nil {
			used[n] = true
		}
	}
	for i := 1; ; i++ {
		if !used[i] {
			return i
		}
	}
}

// createTerminalWindow is the single entry point for opening a new terminal window.
// It handles cascaded (default), maximized, and fixed-size placement.
func (m *Model) createTerminalWindow(opts TerminalWindowOpts) tea.Cmd {
	if len(m.wm.Windows()) >= maxWindows {
		return nil
	}
	prevFocused := ""
	if fw := m.wm.FocusedWindow(); fw != nil {
		prevFocused = fw.ID
	}
	id := newWindowID()
	num := m.nextTerminalNumber()

	// Derive title
	title := fmt.Sprintf("Terminal %d", num)
	if opts.DisplayName != "" {
		title = opts.DisplayName
	} else if opts.Command != "" {
		title = commandDisplayName(opts.Command)
	}

	wa := m.wm.WorkArea()

	// Calculate window rect based on placement mode
	var finalRect geometry.Rect
	switch {
	case opts.Maximized:
		finalRect = wa
	case opts.FixedSize:
		w, h := opts.Width, opts.Height
		x := wa.X + (wa.Width-w)/2
		y := wa.Y + (wa.Height-h)/2
		if x < wa.X {
			x = wa.X
		}
		if y < wa.Y {
			y = wa.Y
		}
		finalRect = geometry.Rect{X: x, Y: y, Width: w, Height: h}
	default: // cascaded — 70% of work area with cascade offset
		w := wa.Width * 7 / 10
		h := wa.Height * 7 / 10
		if w < window.MinWindowWidth {
			w = window.MinWindowWidth
		}
		if h < window.MinWindowHeight {
			h = window.MinWindowHeight
		}
		offset := (num - 1) % 10
		x := wa.X + (wa.Width-w)/2 + offset*2
		y := wa.Y + (wa.Height-h)/2 + offset
		// Clamp to work area
		if x+w > wa.X+wa.Width {
			x = wa.X + wa.Width - w
		}
		if y+h > wa.Y+wa.Height {
			y = wa.Y + wa.Height - h
		}
		finalRect = geometry.Rect{X: x, Y: y, Width: w, Height: h}
	}

	win := window.NewWindow(id, title, finalRect, nil)
	win.Command = opts.Command
	win.Args = opts.Args
	win.WorkDir = opts.WorkDir
	win.TitleBarHeight = m.theme.TitleBarRows()
	win.Icon, win.IconColor = m.windowIcon(opts.Command)
	if opts.Maximized {
		win.PreMaxRect = &geometry.Rect{X: wa.X + 10, Y: wa.Y + 5, Width: 80, Height: 24}
	}
	if opts.FixedSize {
		win.Resizable = false
	}
	m.wm.AddWindow(win)

	autoTile := m.tilingMode && !opts.SkipAutoTile
	if autoTile {
		// Skip open animation; tiling mode will lay out after terminal creation.
	} else {
		// Animate window opening — grow from center
		centerX := finalRect.X + finalRect.Width/2
		centerY := finalRect.Y + finalRect.Height/2
		m.startWindowAnimation(id, AnimOpen,
			geometry.Rect{X: centerX, Y: centerY, Width: 1, Height: 1},
			finalRect)
	}

	// Create terminal sized to the window's content area
	cr := win.ContentRect()
	contentW := cr.Width
	contentH := cr.Height
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// When Kitty graphics passthrough is enabled, set env vars so child apps
	// (kitten icat, etc.) detect graphics support. Child apps talk to our VT
	// emulator which identifies as xterm — they need env hints to know graphics
	// are available. TERM=xterm-kitty is the most reliable signal (kitten checks
	// for "kitty" in TERM). TERM_PROGRAM provides secondary identification.
	var graphicsEnv []string
	if m.kittyPass != nil && m.kittyPass.IsEnabled() {
		if terminal.HasKittyTerminfo() {
			graphicsEnv = append(graphicsEnv, "TERM=xterm-kitty")
		}
		tp := terminal.HostTermProgram()
		if tp != "" {
			graphicsEnv = append(graphicsEnv, "TERM_PROGRAM="+tp)
		} else {
			// Host identity unknown (detected via active TTY query).
			// Set a graphics-capable identity so tools like kitten detect support.
			graphicsEnv = append(graphicsEnv, "TERM_PROGRAM=WezTerm")
		}
		kittyDbg("createTerminalWindow: graphics enabled, env=%v", graphicsEnv)
	} else {
		kittyDbg("createTerminalWindow: graphics NOT enabled (pass=%v)", m.kittyPass != nil)
	}
	// iTerm2 inline images: set env vars so imgcat detects support.
	// imgcat checks ITERM_SESSION_ID (primary) and LC_TERMINAL (secondary).
	// Safe because BT v2 checks TERM_PROGRAM (which we strip), not these.
	if m.imagePass != nil && m.imagePass.Iterm2Enabled() {
		graphicsEnv = append(graphicsEnv, "LC_TERMINAL=iTerm2")
		graphicsEnv = append(graphicsEnv, "ITERM_SESSION_ID=termdesk-"+id)
	}

	var term *terminal.Terminal
	var err error
	if opts.Command == "" {
		term, err = terminal.NewShell(contentW, contentH, m.cellPixelW, m.cellPixelH, opts.WorkDir, graphicsEnv...)
	} else {
		term, err = terminal.New(opts.Command, opts.Args, contentW, contentH, m.cellPixelW, m.cellPixelH, opts.WorkDir, graphicsEnv...)
	}
	if err != nil {
		logging.Error("createTerminalWindow: %v (cmd=%q)", err, opts.Command)
		return nil
	}

	// Set the emulator's default colors to match the theme so that:
	// 1. OSC 10/11 queries report the actual visual bg/fg (not hardcoded black/white)
	// 2. Apps like nvim that use "default background" render consistently
	c := m.theme.C()
	term.SetDefaultColors(c.DefaultFg, c.ContentBg)

	logging.Info("window created id=%s title=%q cmd=%q %dx%d", id, title, opts.Command, contentW, contentH)

	m.terminals[id] = term
	m.termCreatedAt[id] = time.Now()
	// Seed kitty geometry cache so the callback has valid data immediately.
	updateKittyGeom(id, win)
	m.spawnPTYReader(id, term)

	if m.defaultTerminalMode {
		m.inputMode = ModeTerminal
	}
	if autoTile {
		m.applyTileSpawnPreset(id, prevFocused)
		m.applyTilingLayout()
		return tickAnimation()
	}
	return tickAnimation()
}

func (m *Model) applyTileSpawnPreset(newWindowID, anchorID string) {
	preset := normalizeTileSpawnPreset(m.tileSpawnPreset)
	if preset == "auto" {
		return
	}
	if anchorID == "" || anchorID == newWindowID {
		return
	}
	baseSlot := m.wm.TilingSlotOf(anchorID)
	if baseSlot < 0 {
		return
	}
	target := baseSlot
	if preset == "right" || preset == "down" {
		target = baseSlot + 1
	}
	m.wm.PlaceWindowAtTilingSlot(newWindowID, target)
}

// openTerminalWindow creates a new window running the default shell.
func (m *Model) openTerminalWindow() tea.Cmd {
	cwd, _ := os.Getwd()
	return m.openTerminalWindowWith("", nil, "", cwd)
}

// openTerminalWindowWith creates a new cascaded terminal window. Thin wrapper around createTerminalWindow.
func (m *Model) openTerminalWindowWith(command string, args []string, displayName string, workDir string) tea.Cmd {
	return m.createTerminalWindow(TerminalWindowOpts{
		Command: command, Args: args, DisplayName: displayName, WorkDir: workDir,
	})
}

func (m *Model) openTerminalWindowWithOpts(command string, args []string, displayName string, workDir string, skipAutoTile bool) tea.Cmd {
	return m.createTerminalWindow(TerminalWindowOpts{
		Command: command, Args: args, DisplayName: displayName, WorkDir: workDir, SkipAutoTile: skipAutoTile,
	})
}

// openTerminalWindowMaximized creates a new maximized terminal window.
func (m *Model) openTerminalWindowMaximized(command string, args []string, displayName string, workDir string) tea.Cmd {
	return m.createTerminalWindow(TerminalWindowOpts{
		Command: command, Args: args, DisplayName: displayName, WorkDir: workDir,
		Maximized: true,
	})
}

// openFixedTerminalWindow creates a non-resizable terminal window with a fixed size.
func (m *Model) openFixedTerminalWindow(command string, args []string, w, h int, displayName string) tea.Cmd {
	return m.createTerminalWindow(TerminalWindowOpts{
		Command: command, Args: args, DisplayName: displayName,
		FixedSize: true, Width: w, Height: h,
	})
}

// focusedTerminal returns the focused window and its terminal, or nils if none.
func (m *Model) focusedTerminal() (*window.Window, *terminal.Terminal) {
	fw := m.wm.FocusedWindow()
	if fw == nil {
		return nil, nil
	}
	if fw.IsSplit() && fw.FocusedPane != "" {
		return fw, m.terminals[fw.FocusedPane]
	}
	term := m.terminals[fw.ID]
	return fw, term
}

// spawnPTYReader starts background goroutines that read PTY output and notify
// the Bubble Tea program for rendering. Uses a two-stage pipeline:
//
//  1. PTY read goroutine: reads from PTY → sends to terminal's emuCh channel
//  2. Terminal's emuWriteLoop: processes data → writes to emulator → calls onOutput
//
// The onOutput callback sends PtyOutputMsg AFTER the emulator has processed
// the data, ensuring View() always snapshots fresh content. Rate-limited to
// one message per 16ms (~60fps) to avoid flooding during burst output.
func (m *Model) spawnPTYReader(windowID string, term *terminal.Terminal) {
	if m.progRef == nil || m.progRef.p == nil {
		return
	}
	p := m.progRef.p

	// Set up rate-limited notification from emuWriteLoop (fires AFTER emulator
	// processes data — no more stale snapshot race condition).
	//
	// p.Send() blocks on BT's unbuffered message channel. If BT is busy
	// rendering (View → emu.Draw), p.Send blocks, which stalls emuWriteLoop,
	// which prevents emuCh from draining, which drops PTY data and hangs
	// heavy-output apps.
	//
	// Always dispatch p.Send() in a goroutine so emuWriteLoop never blocks.
	// The rate limiter coalesces notifications to ~60fps, so the goroutine
	// count stays bounded.
	var (
		mu       sync.Mutex
		lastSend time.Time
		pending  *time.Timer
	)
	const minInterval = 16 * time.Millisecond
	term.SetOnOutput(func() {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		if now.Sub(lastSend) >= minInterval {
			if pending != nil {
				pending.Stop()
				pending = nil
			}
			lastSend = now
			go p.Send(PtyOutputMsg{WindowID: windowID})
		} else if pending == nil {
			pending = time.AfterFunc(minInterval, func() {
				mu.Lock()
				pending = nil
				lastSend = time.Now()
				mu.Unlock()
				go p.Send(PtyOutputMsg{WindowID: windowID})
			})
		}
	})

	// Wire bell callback — sends BellMsg when child app rings the bell.
	// Use goroutine to avoid blocking emuWriteLoop on BT's unbuffered channel.
	term.SetOnBell(func() {
		go p.Send(BellMsg{WindowID: windowID})
	})

	// Wire Kitty graphics passthrough — intercepts APC sequences from child PTY
	// and forwards them to the real terminal via the shared KittyPassthrough.
	if m.kittyPass != nil && m.kittyPass.IsEnabled() {
		kp := m.kittyPass
		term.SetOnKittyGraphics(func(cmd *terminal.KittyCommand, rawData []byte) *terminal.PlacementResult {
			kittyDbg("onKittyGraphics callback: action=%c imageID=%d rawLen=%d", byte(cmd.Action), cmd.ImageID, len(rawData))
			// Queries don't need window position — respond immediately.
			// CRITICAL: Use WritePTYDirect (not WriteInput) for query responses.
			// kitten icat sends graphics queries followed by a DA1 sentinel (ESC[c]).
			// Our VT emulator's DA1 response goes through emu pipe → inputForwardLoop
			// → pty.Write. If we use the async WriteInput channel, the DA1 response
			// can arrive before our "OK", causing kitten to conclude no graphics.
			// WritePTYDirect writes synchronously to the PTY master before the
			// emulator even sees the DA1 query.
			if cmd.Action == terminal.KittyActionQuery {
				kp.ForwardCommand(cmd, rawData, windowID,
					0, 0, 0, 0, 0, 0, 0, 0, 0, false,
					func(data []byte) {
						kittyDbg("query response via WritePTYDirect (%d bytes): %q", len(data), data)
						term.WritePTYDirect(data)
					},
				)
				if out := kp.FlushPending(); len(out) > 0 {
					kittyDbg("flush pending after query: %d bytes", len(out))
					go p.Send(KittyFlushMsg{Data: out})
				}
				return nil
			}

			// Delete commands don't need window position — forward immediately.
			// Delete must not be gated behind WindowByID() because the stale
			// closure-captured m.wm may not find the window, silently skipping
			// the delete and leaving ghost images on screen.
			if cmd.Action == terminal.KittyActionDelete {
				kp.ForwardCommand(cmd, rawData, windowID,
					0, 0, 0, 0, 0, 0, 0, 0, 0, false, nil,
				)
				if out := kp.FlushPending(); len(out) > 0 {
					go p.Send(KittyFlushMsg{Data: out})
				}
				return nil
			}

			// Read cached window geometry — safe from any goroutine.
			// The snapshot is updated by the BT main loop during rendering.
			geomVal, ok := kittyWindowGeom.Load(windowID)
			if !ok {
				return nil
			}
			geom := geomVal.(*kittyGeomSnapshot)
			cx, cy := term.CursorPosition()
			scrollbackLen := term.ScrollbackLen()
			isAlt := term.IsAltScreen()

			result := kp.ForwardCommand(cmd, rawData, windowID,
				geom.windowX, geom.windowY, geom.windowWidth, geom.windowH,
				geom.contentOffX, geom.contentOffY,
				cx, cy, scrollbackLen, isAlt,
				func(data []byte) { term.WriteInput(data) },
			)

			if out := kp.FlushPending(); len(out) > 0 {
				go p.Send(KittyFlushMsg{Data: out})
			}
			return result
		})
	}

	// Wire image passthrough (sixel/iTerm2) — intercepts DCS/OSC sequences
	// from child PTY output BEFORE they reach the VT emulator.
	// Always set callback when imagePass exists (even if no protocol is enabled)
	// to prevent raw DCS/OSC data from reaching the emulator and being dumped
	// as ASCII text. ForwardImage/ForwardRawSequence check enablement internally.
	if m.imagePass != nil {
		ip := m.imagePass
		term.SetOnImageGraphics(func(rawData []byte, format terminal.ImageFormat, estimatedCellRows int) int {
			// Fire-and-forget: forward raw without placement storage.
			// iTerm2 images persist in the host terminal's image layer.
			// Sixel images persist in iTerm2's framebuffer.
			// CUP re-rendering from RefreshAllImages causes cursor/prompt
			// corruption when cellRows doesn't exactly match visual height.
			if estimatedCellRows == 0 {
				ip.ForwardRawSequence(rawData)
				if out := ip.FlushPending(); len(out) > 0 {
					go p.Send(ImageFlushMsg{Data: out})
				}
				return 0
			}

			cx, cy := term.CursorPosition()
			scrollbackLen := term.ScrollbackLen()
			isAlt := term.IsAltScreen()

			// Store placement for re-rendering after each BT frame (tmux-style).
			// No immediate render — RefreshAllImages handles it in the Update wrapper.
			result := ip.ForwardImage(rawData, format, windowID, cx, cy, scrollbackLen, isAlt, estimatedCellRows)

			// Notify the app so ImageRefreshMsg is scheduled to render
			// the new placement. ImageFlushMsg triggers image refresh
			// in the Update wrapper's event dispatch.
			if result > 0 {
				go p.Send(ImageFlushMsg{})
			}

			return result
		})
	}

	// Wire screen clear callback — clears graphics when child app sends CSI 2J/3J.
	{
		kp := m.kittyPass
		ip := m.imagePass
		term.SetOnScreenClear(func() {
			if kp != nil && kp.IsEnabled() {
				kp.ClearWindow(windowID)
				if out := kp.FlushPending(); len(out) > 0 {
					go p.Send(KittyFlushMsg{Data: out})
				}
			}
			if ip != nil && ip.ClearWindow(windowID) {
				// Force full screen repaint: sixel pixels were injected
				// via tea.Raw() — BT's diff renderer can't overwrite them.
				go p.Send(ImageClearScreenMsg{})
			}
		})
	}

	// PTY read loop — reads chunks and feeds to terminal's emuCh.
	// PtyOutputMsg is NOT sent here; it's sent from the onOutput callback
	// after the emulator has processed the data.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Only send close on panic — normal path sends its own
				p.Send(PtyClosedMsg{WindowID: windowID, Err: fmt.Errorf("internal panic: %v", r)})
			}
		}()
		readBuf := make([]byte, 32768)
		for {
			_, err := term.ReadOnce(readBuf)
			if err != nil {
				mu.Lock()
				if pending != nil {
					pending.Stop()
				}
				mu.Unlock()
				// Send final render notification and close message
				p.Send(PtyOutputMsg{WindowID: windowID})
				p.Send(PtyClosedMsg{WindowID: windowID, Err: err})
				return
			}
		}
	}()
}

// commandDisplayName extracts a human-readable name from a command path.
// "/home/user/go/bin/termdesk-calc" → "Calc", "htop" → "htop", "/usr/bin/nvim" → "nvim"
func commandDisplayName(command string) string {
	name := command
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasPrefix(name, "termdesk-") {
		name = name[len("termdesk-"):]
		// Capitalize first letter for termdesk apps
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
	}
	return name
}

// windowIcon returns a Nerd Font icon and color for a command.
// Checks the app registry first, then falls back to common command mappings.
func (m Model) windowIcon(command string) (icon, iconColor string) {
	// Extract base command name for matching
	base := command
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}

	// Check registry for manifest-based icons
	for _, entry := range m.registry {
		entryBase := entry.Command
		if i := strings.LastIndex(entryBase, "/"); i >= 0 {
			entryBase = entryBase[i+1:]
		}
		if entryBase == base && entry.Icon != "" {
			return entry.Icon, entry.IconColor
		}
	}

	// Fallback: common command → icon mapping
	switch {
	case base == "" || base == "$SHELL" || base == "bash" || base == "zsh" || base == "fish" || base == "sh":
		return "\uf120", "" //  terminal
	case base == "nvim" || base == "vim" || base == "vi":
		return "\ue62b", "#61AFEF" //  vim
	case base == "mc" || base == "spf" || base == "ranger" || base == "lf" || base == "yazi":
		return "\uf07b", "#E5C07B" //  files
	case base == "htop" || base == "btop" || base == "top":
		return "\uf200", "#E06C75" //  monitor
	case base == "python3" || base == "python" || base == "bc":
		return "\uf1ec", "#98C379" //  calc
	case base == "nano":
		return "\uf0f6", "#98C379" //  notepad
	default:
		return "\uf120", "" //  default terminal
	}
}

// openDemoWindow creates a demo window without a terminal (for testing).
func (m *Model) openDemoWindow() {
	id := newWindowID()
	num := m.nextTerminalNumber()
	title := fmt.Sprintf("Window %d", num)

	wa := m.wm.WorkArea()
	w := wa.Width * 7 / 10
	h := wa.Height * 7 / 10
	if w < window.MinWindowWidth {
		w = window.MinWindowWidth
	}
	if h < window.MinWindowHeight {
		h = window.MinWindowHeight
	}
	offset := (num - 1) % 10
	x := wa.X + (wa.Width-w)/2 + offset*2
	y := wa.Y + (wa.Height-h)/2 + offset
	if x+w > wa.X+wa.Width {
		x = wa.X + wa.Width - w
	}
	if y+h > wa.Y+wa.Height {
		y = wa.Y + wa.Height - h
	}

	win := window.NewWindow(id, title, geometry.Rect{X: x, Y: y, Width: w, Height: h}, nil)
	m.wm.AddWindow(win)
}

// runProjectAutoStart launches auto-start commands from project config,
// skipping any commands that are already running in existing windows
// (e.g. restored from a saved workspace).
func (m *Model) runProjectAutoStart() tea.Cmd {
	if m.projectConfig == nil {
		return nil
	}

	// Collect commands already running in existing windows
	running := make(map[string]bool)
	for _, w := range m.wm.Windows() {
		if w.Command != "" {
			running[w.Command] = true
		}
	}

	var cmds []tea.Cmd

	for _, item := range m.projectConfig.AutoStart {
		// Skip if this command is already running from workspace restore
		if running[item.Command] {
			continue
		}

		// Resolve working directory
		workDir := m.projectConfig.ProjectDir
		if item.Directory != "" && item.Directory != "." {
			workDir = filepath.Join(workDir, item.Directory)
		}

		// Determine title
		title := item.Title
		if title == "" {
			title = item.Command
		}

		// Launch terminal window
		cmd := m.openTerminalWindowWithOpts(item.Command, item.Args, title, workDir, len(m.wm.Windows()) > 0)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// focusOrLaunchApp focuses an existing window running the given command,
// or launches a new one if none exists. Used by widget clicks and dock.
// Matches by exact command, resolved path base name, or registry name.
func (m Model) focusOrLaunchApp(command string) (tea.Model, tea.Cmd) {
	// Skip dedup for default shell
	if command == "$SHELL" || command == "" {
		return m.launchAppFromRegistry(command)
	}
	// Check for existing window running this command
	for _, w := range m.wm.Windows() {
		if w.Exited {
			continue
		}
		// Match by exact command, or by base name (e.g. "htop" matches "/usr/bin/htop")
		if w.Command == command || filepath.Base(w.Command) == command {
			if w.Minimized {
				m.restoreMinimizedWindow(w)
				m.inputMode = ModeTerminal
				return m, tickAnimation()
			}
			m.wm.FocusWindow(w.ID)
			m.inputMode = ModeTerminal
			return m, nil
		}
	}
	return m.launchAppFromRegistry(command)
}

// launchAppFromRegistry launches a command using registry/manifest window preferences.
func (m Model) launchAppFromRegistry(command string) (tea.Model, tea.Cmd) {
	m.inputMode = ModeNormal
	m.launcher.RecordLaunch(command)
	m.saveLauncherState()

	// Handle default shell
	if command == "$SHELL" || command == "" {
		cmd := m.openTerminalWindow()
		return m, cmd
	}

	// Look up window preferences from registry
	for _, entry := range m.registry {
		if entry.Command == command {
			resolved := registry.FindBinary(m.exePath, command)
			name := entry.Name
			if entry.Window.IsFixedSize() && !entry.Window.IsResizable() {
				cmd := m.openFixedTerminalWindow(resolved, entry.Args, entry.Window.Width, entry.Window.Height, name)
				return m, cmd
			}
			if entry.Window.Maximized {
				cmd := m.openTerminalWindowMaximized(resolved, entry.Args, name, "")
				return m, cmd
			}
			cmd := m.openTerminalWindowWith(resolved, entry.Args, name, "")
			return m, cmd
		}
	}

	// Not in registry — launch with defaults
	resolved := registry.FindBinary(m.exePath, command)
	cmd := m.openTerminalWindowWith(resolved, nil, "", "")
	return m, cmd
}

// launchExecEntry launches a selected exec entry with explicit command and args.
func (m Model) launchExecEntry(command string, args []string) (tea.Model, tea.Cmd) {
	m.inputMode = ModeNormal
	m.launcher.RecordLaunch(command)
	m.saveLauncherState()

	resolved := registry.FindBinary(m.exePath, command)
	if resolved == command {
		if _, err := exec.LookPath(command); err != nil {
			m.notifications.Push("Command not found", command, notification.Warning)
			return m, nil
		}
	}
	return m, m.openTerminalWindowWith(resolved, args, command, "")
}

// launchCommandLine launches an arbitrary command line (PATH executable + args).
func (m Model) launchCommandLine(query string) (tea.Model, tea.Cmd) {
	m.inputMode = ModeNormal
	tokens := launcher.TokenizeCommand(query)
	if len(tokens) == 0 {
		return m, nil
	}
	cmd := tokens[0]
	args := tokens[1:]

	m.launcher.RecordLaunch(cmd)
	m.saveLauncherState()

	resolved := cmd
	if !strings.Contains(cmd, "/") {
		resolved = registry.FindBinary(m.exePath, cmd)
	}
	if resolved == cmd {
		if _, err := exec.LookPath(cmd); err != nil {
			m.notifications.Push("Command not found", cmd, notification.Warning)
			return m, nil
		}
	}
	return m, m.openTerminalWindowWith(resolved, args, cmd, "")
}

// saveLauncherState persists recent apps and favorites to config.
// Runs file I/O in a goroutine to avoid blocking the main BT event loop.
func (m *Model) saveLauncherState() {
	// Snapshot in-memory state before spawning goroutine.
	recentApps := make([]string, len(m.launcher.RecentApps))
	copy(recentApps, m.launcher.RecentApps)
	favs := make([]string, 0, len(m.launcher.Favorites))
	for cmd := range m.launcher.Favorites {
		favs = append(favs, cmd)
	}
	go func() {
		cfg := config.LoadUserConfig()
		cfg.RecentApps = recentApps
		cfg.Favorites = favs
		if err := config.SaveUserConfig(cfg); err != nil {
			logging.Error("config save failed: %v", err)
		}
	}()
}

// restartExitedWindow relaunches the command in an exited window, reusing the same window frame.
func (m Model) restartExitedWindow(windowID string) (tea.Model, tea.Cmd) {
	w := m.wm.WindowByID(windowID)
	if w == nil {
		return m, nil
	}

	// Close old terminal
	m.closeTerminal(windowID)

	// Reset window state
	w.Exited = false
	w.Stuck = false
	w.HasBell = false
	// Remove " [exited]" suffix from title
	w.Title = strings.TrimSuffix(w.Title, " [exited]")

	// Create new terminal sized to window's content area
	cr := w.ContentRect()
	contentW := cr.Width
	contentH := cr.Height
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
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

	var term *terminal.Terminal
	var err error
	if w.Command == "" {
		term, err = terminal.NewShell(contentW, contentH, m.cellPixelW, m.cellPixelH, w.WorkDir, graphicsEnv...)
	} else {
		term, err = terminal.New(w.Command, w.Args, contentW, contentH, m.cellPixelW, m.cellPixelH, w.WorkDir, graphicsEnv...)
	}
	if err != nil {
		return m, nil
	}

	c := m.theme.C()
	term.SetDefaultColors(c.DefaultFg, c.ContentBg)

	m.terminals[windowID] = term
	m.termCreatedAt[windowID] = time.Now()
	delete(m.termHasOutput, windowID)
	m.spawnPTYReader(windowID, term)

	return m, nil
}

// quakeTermID is the pseudo-ID used for the quake terminal in the terminals map.
const quakeTermID = "__quake__"

// quakeHeightPctOrDefault returns a valid quake height percentage.
func quakeHeightPctOrDefault(pct int) int {
	if pct < 10 || pct > 90 {
		return 40
	}
	return pct
}

// quakeFullHeight returns the full height for the quake terminal based on configured percentage.
func (m *Model) quakeFullHeight() int {
	pct := m.quakeHeightPct
	if pct <= 0 {
		pct = 40
	}
	h := m.height * pct / 100
	if h < 5 {
		h = 5
	}
	return h
}

// quakeContentSize returns the terminal content dimensions (no side borders, only bottom border).
func (m *Model) quakeContentSize() (int, int) {
	h := m.quakeFullHeight()
	return m.width, h - 1 // full width, minus 1 row for bottom border
}

// ensureQuakeTerminal creates the quake terminal if it doesn't exist yet.
func (m *Model) ensureQuakeTerminal() {
	if m.quakeTerminal != nil {
		return
	}
	cols, rows := m.quakeContentSize()
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 10
	}
	term, err := terminal.NewShell(cols, rows, m.cellPixelW, m.cellPixelH, "")
	if err != nil {
		return
	}
	c := m.theme.C()
	term.SetDefaultColors(c.DefaultFg, c.ContentBg)
	m.quakeTerminal = term
	m.terminals[quakeTermID] = term
	m.termCreatedAt[quakeTermID] = time.Now()
	m.spawnPTYReader(quakeTermID, term)
}

// toggleQuakeTerminal toggles the quake dropdown terminal visibility.
// The terminal is never destroyed — only shown/hidden with animation.
func (m *Model) toggleQuakeTerminal() tea.Cmd {
	m.ensureQuakeTerminal()
	if m.quakeTerminal == nil {
		return nil
	}

	fullH := float64(m.quakeFullHeight())

	if m.quakeVisible {
		// Hide: animate height → 0
		m.quakeVisible = false
		m.quakeTargetH = 0
		m.inputMode = ModeNormal
		if !m.animationsOn {
			m.quakeAnimH = 0
			return nil
		}
		return tickAnimation()
	}

	// Show: animate height → full
	m.quakeVisible = true
	m.quakeTargetH = fullH
	// Resize terminal to current screen size
	cols, rows := m.quakeContentSize()
	m.quakeTerminal.Resize(cols, rows)
	m.inputMode = ModeTerminal
	if !m.animationsOn {
		m.quakeAnimH = fullH
		return nil
	}
	return tickAnimation()
}

// resizeQuake adjusts the quake height by delta rows and resizes the terminal.
func (m *Model) resizeQuake(delta int) {
	newH := int(m.quakeAnimH) + delta
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
	// Update percentage to match the new absolute height
	m.quakeHeightPct = newH * 100 / m.height
	if m.quakeHeightPct < 10 {
		m.quakeHeightPct = 10
	}
	if m.quakeHeightPct > 90 {
		m.quakeHeightPct = 90
	}
	// Resize terminal emulator
	cols, _ := m.quakeContentSize()
	contentH := newH - 1 // minus bottom border
	if contentH < 1 {
		contentH = 1
	}
	m.quakeTerminal.Resize(cols, contentH)
}

// closeQuakeTerminal cleans up the quake terminal after PTY exit.
// The next Ctrl+~ toggle will create a fresh one via ensureQuakeTerminal().
func (m *Model) closeQuakeTerminal() {
	if m.quakeTerminal != nil {
		m.quakeTerminal.Close()
	}
	delete(m.terminals, quakeTermID)
	delete(m.termCreatedAt, quakeTermID)
	delete(m.termHasOutput, quakeTermID)
	m.quakeTerminal = nil
	m.quakeVisible = false
	m.quakeAnimH = 0
	m.quakeTargetH = 0
	m.quakeAnimVel = 0
	if m.inputMode == ModeTerminal {
		m.inputMode = ModeNormal
	}
}

// closeExitedWindow closes a window whose process has exited (hold-open mode).
func (m Model) closeExitedWindow(windowID string) (tea.Model, tea.Cmd) {
	if w := m.wm.WindowByID(windowID); w != nil {
		m.closeTerminal(windowID)
		centerX := w.Rect.X + w.Rect.Width/2
		centerY := w.Rect.Y + w.Rect.Height/2
		m.startWindowAnimation(windowID, AnimClose, w.Rect,
			geometry.Rect{X: centerX, Y: centerY, Width: 1, Height: 1})
		return m, tickAnimation()
	}
	return m, nil
}

// closeTerminal closes and removes a terminal by window ID.
func (m *Model) closeTerminal(windowID string) {
	if term, ok := m.terminals[windowID]; ok {
		logging.Info("window closed id=%s", windowID)
		term.Close()
		delete(m.terminals, windowID)
	}
	delete(m.termCreatedAt, windowID)
	delete(m.termHasOutput, windowID)
	removeKittyGeom(windowID)
	// Clean up Kitty graphics placements for this window.
	if m.kittyPass != nil {
		m.kittyPass.ClearWindow(windowID)
		if out := m.kittyPass.FlushPending(); len(out) > 0 {
			m.kittyPending.data = append(m.kittyPending.data, out...)
		}
	}
	// Clean up image (sixel/iTerm2) placements for this window.
	if m.imagePass != nil {
		m.imagePass.ClearWindow(windowID)
	}
	// Always clean up buffer (may exist from workspace restore even without terminal)
	delete(m.windowBuffers, windowID)
	delete(m.minimizedTileSlots, windowID)
}

// refreshKittyPlacements updates image positions based on current window/scroll state.
// Called from Update() on PtyOutputMsg, AnimationTickMsg, WindowSizeMsg, and drag
// release — any event that can change window positions or scroll state.
func (m *Model) refreshKittyPlacements() {
	if m.kittyPass == nil {
		return
	}

	// Hide all Kitty images when a fullscreen overlay is active (dialogs,
	// launcher, settings, etc.). Kitty images render at z=10 (above text)
	// and would cover UI overlays. Context menus excluded — they're small
	// title-bar popups that don't overlap with content-area images.
	if m.hasImageBlockingOverlay() {
		m.kittyPass.HideAllPlacements()
		if out := m.kittyPass.FlushPending(); len(out) > 0 {
			m.kittyPending.data = append(m.kittyPending.data, out...)
		}
		return
	}

	dragWindowID := ""
	if m.drag.Active {
		dragWindowID = m.drag.WindowID
	}
	focusedID := ""
	if fw := m.wm.FocusedWindow(); fw != nil {
		focusedID = fw.ID
	}

	// Build z-order index for occlusion checking (Kitty z=10 covers everything).
	allWindows := m.wm.Windows() // back-to-front
	windowZIndex := make(map[string]int, len(allWindows))
	for i, w := range allWindows {
		windowZIndex[w.ID] = i
	}

	// Update kitty geometry cache for all windows so the onKittyGraphics
	// callback (running in emuWriteLoop goroutine) has fresh data.
	for _, w := range allWindows {
		updateKittyGeom(w.ID, w)
	}

	m.kittyPass.RefreshAllPlacements(func() map[string]*terminal.KittyWindowInfo {
		infos := make(map[string]*terminal.KittyWindowInfo)
		for id, term := range m.terminals {
			w := m.wm.WindowByID(id)
			if w == nil {
				continue
			}
			cr := w.ContentRect()
			visible := w.Visible && !w.Minimized

			// Occlusion: hide Kitty images when another window is on top.
			// Kitty renders at z=10 (above all text), so images would
			// cover overlapping windows without this check.
			if visible {
				myZ := windowZIndex[id]
				for j := myZ + 1; j < len(allWindows); j++ {
					w2 := allWindows[j]
					if !w2.Visible || w2.Minimized {
						continue
					}
					if cr.Overlaps(w2.Rect) {
						visible = false
						break
					}
				}
			}

			scrollOff := 0
			if id == focusedID && m.inputMode == ModeCopy {
				scrollOff = m.scrollOffset
			}
			infos[id] = &terminal.KittyWindowInfo{
				WindowX:       w.Rect.X,
				WindowY:       w.Rect.Y,
				ContentOffX:   cr.X - w.Rect.X,
				ContentOffY:   cr.Y - w.Rect.Y,
				ContentWidth:  cr.Width,
				ContentHeight: cr.Height,
				Width:         w.Rect.Width,
				Height:        w.Rect.Height,
				Visible:       visible,
				ScrollbackLen: term.ScrollbackLen(),
				ScrollOffset:  scrollOff,
				IsManipulated: id == dragWindowID || m.hasActiveAnimation(id),
				IsAltScreen:   term.IsAltScreen(),
			}
		}
		return infos
	})
	if out := m.kittyPass.FlushPending(); len(out) > 0 {
		m.kittyPending.data = append(m.kittyPending.data, out...)
	}
}

// refreshImagePlacements re-renders stored sixel/iTerm2 images at their current
// host terminal positions. Called from the Update wrapper on ImageRefreshMsg
// (deferred from dirty events) to ensure images are sent AFTER the frame render.
func (m *Model) refreshImagePlacements() {
	if m.imagePass == nil || !m.imagePass.HasImagePlacements() {
		return
	}

	// Don't render during overlays — images would cover them.
	if m.hasImageBlockingOverlay() {
		return
	}

	focusedID := ""
	if fw := m.wm.FocusedWindow(); fw != nil {
		focusedID = fw.ID
	}

	// Build z-order index for occlusion checking.
	// Windows() returns back-to-front — higher index = on top.
	allWindows := m.wm.Windows()
	windowZIndex := make(map[string]int, len(allWindows))
	for i, w := range allWindows {
		windowZIndex[w.ID] = i
	}

	m.imagePass.RefreshAllImages(m.height, func() map[string]*terminal.ImageWindowInfo {
		infos := make(map[string]*terminal.ImageWindowInfo)
		for id, term := range m.terminals {
			w := m.wm.WindowByID(id)
			if w == nil {
				continue
			}
			cr := w.ContentRect()
			visible := w.Visible && !w.Minimized

			// Occlusion check: if any visible window ABOVE this one
			// overlaps its content area, mark as not visible. Prevents
			// sixel from rendering on top of overlapping windows.
			if visible {
				myZ := windowZIndex[id]
				for j := myZ + 1; j < len(allWindows); j++ {
					w2 := allWindows[j]
					if !w2.Visible || w2.Minimized {
						continue
					}
					if cr.Overlaps(w2.Rect) {
						visible = false
						break
					}
				}
			}

			scrollOff := 0
			if id == focusedID && m.inputMode == ModeCopy {
				scrollOff = m.scrollOffset
			}
			infos[id] = &terminal.ImageWindowInfo{
				WindowX:       w.Rect.X,
				WindowY:       w.Rect.Y,
				ContentOffX:   cr.X - w.Rect.X,
				ContentOffY:   cr.Y - w.Rect.Y,
				ContentWidth:  cr.Width,
				ContentHeight: cr.Height,
				Visible:       visible,
				ScrollbackLen: term.ScrollbackLen(),
				ScrollOffset:  scrollOff,
				IsAltScreen:   term.IsAltScreen(),
			}
		}
		return infos
	})
	if out := m.imagePass.FlushPending(); len(out) > 0 {
		m.imagePending.data = append(m.imagePending.data, out...)
	}
}

// hasActiveOverlay returns true when any overlay/dialog is blocking the view.
func (m *Model) hasActiveOverlay() bool {
	return m.modal != nil ||
		m.confirmClose != nil ||
		m.renameDialog != nil ||
		m.bufferNameDialog != nil ||
		m.newWorkspaceDialog != nil ||
		m.launcher.Visible ||
		m.exposeMode ||
		m.settings.Visible ||
		m.clipboard.Visible ||
		m.notifications.CenterVisible() ||
		m.workspacePickerVisible ||
		m.contextMenu != nil
}

// hasImageBlockingOverlay returns true for overlays that should hide images.
// Includes context menus because both Kitty (z=10) and sixel (re-rendered
// after frame) would appear ON TOP of the menu, making it unreadable.
func (m *Model) hasImageBlockingOverlay() bool {
	return m.modal != nil ||
		m.confirmClose != nil ||
		m.renameDialog != nil ||
		m.bufferNameDialog != nil ||
		m.newWorkspaceDialog != nil ||
		m.launcher.Visible ||
		m.exposeMode ||
		m.settings.Visible ||
		m.clipboard.Visible ||
		m.notifications.CenterVisible() ||
		m.workspacePickerVisible ||
		m.contextMenu != nil
}

// closeAllTerminals closes all active terminals.
func (m *Model) closeAllTerminals() {
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
	m.quakeTerminal = nil
	m.quakeVisible = false
	// Also close terminals in background workspaces
	for path, bg := range m.backgroundWorkspaces {
		for _, term := range bg.terminals {
			term.Close()
		}
		delete(m.backgroundWorkspaces, path)
	}
}

// resizeTerminalForWindow resizes the terminal to match its window's content area.
// For split windows, delegates to resizeAllPanes which handles all pane terminals.
func (m *Model) resizeTerminalForWindow(w *window.Window) {
	if w.IsSplit() {
		m.resizeAllPanes(w)
		return
	}
	if term, ok := m.terminals[w.ID]; ok {
		cr := w.ContentRect()
		if cr.Width > 0 && cr.Height > 0 {
			// Skip resize if dimensions haven't changed (pure window move).
			// Calling Resize on same size triggers SIGWINCH → shell redraws
			// with a brief blank flash, and sets dirtyUntil which forces
			// 800ms of cache misses.
			oldW, oldH := term.Width(), term.Height()
			if oldW == cr.Width && oldH == cr.Height {
				return
			}

			term.Resize(cr.Width, cr.Height)
			// NOTE: Do NOT recapture copy snapshot here. The snapshot is frozen
			// and must survive resize. The rendering code already handles
			// dimension mismatches (clips/fills). Recapturing after resize
			// would lose scrollback that was in the original snapshot.

			// HIDE image placements (don't destroy data).
			// Content reflow makes AbsoluteLine positions stale, but we
			// preserve image data so apps that don't re-emit after SIGWINCH
			// (one-shot tools like imgcat, img2sixel) keep their images.
			// For Kitty: d=a removes all visible placements, keeps data.
			// For sixel/iTerm2: keep stored DCS/OSC data —
			// RefreshAllImages recomputes visibility from current viewport.
			if m.kittyPass != nil && m.kittyPass.HasPlacements() {
				m.kittyPass.HideAllPlacements()
				if out := m.kittyPass.FlushPending(); len(out) > 0 {
					m.kittyPending.data = append(m.kittyPending.data, out...)
				}
			}
		}
	}
}

// resizeAllTerminals resizes all terminals to match their windows.
func (m *Model) resizeAllTerminals() {
	for _, w := range m.wm.Windows() {
		m.resizeTerminalForWindow(w)
	}
}

// openClipboardViewer opens a new terminal window with `less` displaying the text.
func (m *Model) openClipboardViewer(text string) tea.Cmd {
	// Write text to a temp file for `less` to display
	f, err := os.CreateTemp("", "termdesk-clip-*.txt")
	if err != nil {
		return nil
	}
	f.WriteString(text)
	f.Close()
	return m.openTerminalWindowWith("less", []string{f.Name()}, "", "")
}
