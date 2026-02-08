package app

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"

	"github.com/mattn/go-runewidth"
	tea "charm.land/bubbletea/v2"
)

const version = "0.1.0"

// programRef holds a shared reference to the tea.Program.
// Using a pointer so copies of Model (value receivers) share the same ref.
type programRef struct {
	p *tea.Program
}

// Model is the root model for the termdesk application.
type Model struct {
	width     int
	height    int
	ready     bool
	wm        *window.Manager
	theme     config.Theme
	drag      window.DragState
	nextWID   int
	terminals map[string]*terminal.Terminal
	menuBar   *menubar.MenuBar
	dock      *dock.Dock
	launcher     *launcher.Launcher
	progRef      *programRef // shared program reference for goroutine messaging
	exposeMode   bool        // exposé overview mode
	inputMode    InputMode   // current input mode (Normal/Terminal/Copy)
	dockFocused  bool        // true when dock has keyboard focus in Normal mode
	confirmClose *ConfirmDialog
	renameDialog *RenameDialog
	cpuPct       float64
	memUsedGB    float64
	animations   []Animation     // active animations
	modal        *ModalOverlay   // help, about, or other modal
	keybindings  config.KeyBindings
	actionMap    map[string]string // key string → action name (reverse lookup)
	prefixPending bool            // true when prefix key pressed, waiting for action
	cursorVisible bool            // cursor blink state (toggled every 500ms)
	cursorBlinkAt time.Time       // last cursor blink toggle time
	scrollOffset  int             // scrollback offset in Copy mode (0 = live)
}

// ConfirmDialog represents a confirmation dialog overlay.
type ConfirmDialog struct {
	WindowID string // window to close, or "" for quit
	Title    string
	IsQuit   bool // true = quit application, false = close window
	Selected int  // 0 = Yes, 1 = No
}

// RenameDialog represents a text input dialog for renaming a window.
type RenameDialog struct {
	WindowID string
	Text     []rune
	Cursor   int
}

// ModalOverlay represents a modal text overlay (help, about, etc.)
type ModalOverlay struct {
	Title    string
	Lines    []string
	ScrollY  int
}

// InputMode represents the current interaction mode.
type InputMode int

const (
	// ModeNormal is window management mode — single-letter WM keys work.
	ModeNormal InputMode = iota
	// ModeTerminal passes all input to the focused terminal.
	ModeTerminal
	// ModeCopy enables vim-style scrollback navigation and selection.
	ModeCopy
)

// String returns the display name for the input mode.
func (m InputMode) String() string {
	switch m {
	case ModeTerminal:
		return "TERMINAL"
	case ModeCopy:
		return "COPY"
	default:
		return "NORMAL"
	}
}

// cycleInputMode cycles through Normal → Terminal → Copy → Normal.
func (m *Model) cycleInputMode() {
	switch m.inputMode {
	case ModeNormal:
		m.inputMode = ModeTerminal
	case ModeTerminal:
		m.inputMode = ModeCopy
	case ModeCopy:
		m.inputMode = ModeNormal
	}
}

// SystemStatsMsg carries updated system statistics.
type SystemStatsMsg struct {
	CPU         float64
	MemGB       float64
	BatPct      float64
	BatCharging bool
	BatPresent  bool
}

// BuildActionMap creates a reverse lookup from KeyBindings (key → action).
// Includes configurable primary keys and hardcoded alternate keys.
func BuildActionMap(kb config.KeyBindings) map[string]string {
	am := map[string]string{}

	// Primary bindings from config
	am[kb.Quit] = "quit"
	am[kb.NewTerminal] = "new_terminal"
	am[kb.CloseWindow] = "close_window"
	am[kb.EnterTerminal] = "enter_terminal"
	am[kb.Minimize] = "minimize"
	am[kb.Rename] = "rename"
	am[kb.DockFocus] = "dock_focus"
	am[kb.Launcher] = "launcher"
	am[kb.SnapLeft] = "snap_left"
	am[kb.SnapRight] = "snap_right"
	am[kb.Maximize] = "maximize"
	am[kb.Restore] = "restore"
	am[kb.TileAll] = "tile_all"
	am[kb.Expose] = "expose"
	am[kb.NextWindow] = "next_window"
	am[kb.PrevWindow] = "prev_window"
	am[kb.Help] = "help"
	am[kb.ToggleExpose] = "toggle_expose"
	am[kb.MenuBar] = "menu_bar"
	am[kb.MenuFile] = "menu_file"
	am[kb.MenuApps] = "menu_apps"
	am[kb.MenuView] = "menu_view"

	// Hardcoded alternates (always work alongside configurable keys)
	am["ctrl+q"] = "quit"
	am["ctrl+c"] = "quit"
	am["ctrl+n"] = "new_terminal"
	am["ctrl+w"] = "close_window"
	am["enter"] = "enter_terminal"
	am["ctrl+space"] = "launcher"
	am["ctrl+/"] = "launcher"
	am["left"] = "snap_left"
	am["right"] = "snap_right"
	am["up"] = "maximize"
	am["down"] = "restore"
	am["ctrl+]"] = "next_window"
	am["ctrl+["] = "prev_window"

	return am
}

// New creates a new root Model.
func New() Model {
	userCfg := config.LoadUserConfig()
	theme := config.GetTheme(userCfg.Theme)
	mb := menubar.New(80)
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	hostname, _ := os.Hostname()
	if hostname != "" {
		mb.Username = username + "@" + hostname
	} else {
		mb.Username = username
	}
	d := dock.New(80)
	d.IconsOnly = userCfg.IconsOnly
	am := BuildActionMap(userCfg.Keys)
	return Model{
		wm:            window.NewManager(80, 24),
		theme:         theme,
		terminals:     make(map[string]*terminal.Terminal),
		menuBar:       mb,
		dock:          d,
		launcher:      launcher.New(),
		progRef:       &programRef{},
		keybindings:   userCfg.Keys,
		actionMap:     am,
		cursorVisible: true,
		cursorBlinkAt: time.Now(),
	}
}

// SetProgram sets the tea.Program reference for background goroutine messaging.
// Must be called before Run(). The reference is shared across Model copies
// (Bubble Tea uses value receivers, so Model gets copied).
func (m *Model) SetProgram(p *tea.Program) {
	m.progRef.p = p
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.tickSystemStats(), tickCursorBlink())
}

func (m Model) tickSystemStats() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		bat := menubar.ReadBattery()
		return SystemStatsMsg{
			CPU:         menubar.ReadCPUPercent(),
			MemGB:       menubar.ReadMemoryGB(),
			BatPct:      bat.Percent,
			BatCharging: bat.Charging,
			BatPresent:  bat.Present,
		}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.wm.SetBounds(msg.Width, msg.Height)
		m.wm.SetReserved(1, 1) // 1 row for menu bar at top, 1 for dock at bottom
		m.menuBar.SetWidth(msg.Width)
		m.dock.SetWidth(msg.Width)
		m.ready = true
		m.resizeAllTerminals()

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClick(tea.Mouse(msg))

	case tea.MouseMotionMsg:
		return m.handleMouseMotion(tea.Mouse(msg))

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(tea.Mouse(msg))

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(tea.Mouse(msg))

	case PtyOutputMsg:
		// PTY produced output — just re-render (goroutine handles reads)
		// Mark unfocused windows as having notifications
		if fw := m.wm.FocusedWindow(); fw == nil || fw.ID != msg.WindowID {
			if w := m.wm.WindowByID(msg.WindowID); w != nil {
				w.HasNotification = true
			}
		}
		return m, nil

	case SystemStatsMsg:
		m.cpuPct = msg.CPU
		m.memUsedGB = msg.MemGB
		m.menuBar.CPUPct = msg.CPU
		m.menuBar.MemGB = msg.MemGB
		m.menuBar.BatPct = msg.BatPct
		m.menuBar.BatCharging = msg.BatCharging
		m.menuBar.BatPresent = msg.BatPresent
		// Push CPU to rolling history (max 20 samples)
		m.menuBar.CPUHistory = append(m.menuBar.CPUHistory, msg.CPU)
		if len(m.menuBar.CPUHistory) > 20 {
			m.menuBar.CPUHistory = m.menuBar.CPUHistory[1:]
		}
		return m, m.tickSystemStats()

	case AnimationTickMsg:
		if m.updateAnimations(msg.Time) {
			return m, tickAnimation()
		}
		return m, nil

	case CursorBlinkMsg:
		if m.inputMode == ModeTerminal {
			m.cursorVisible = !m.cursorVisible
		} else {
			m.cursorVisible = true
		}
		return m, tickCursorBlink()

	case PtyClosedMsg:
		// PTY exited — animate close, then remove
		if w := m.wm.WindowByID(msg.WindowID); w != nil && !m.isAnimatingClose(msg.WindowID) {
			m.closeTerminal(msg.WindowID)
			centerX := w.Rect.X + w.Rect.Width/2
			centerY := w.Rect.Y + w.Rect.Height/2
			m.startWindowAnimation(msg.WindowID, AnimClose, w.Rect,
				geometry.Rect{X: centerX, Y: centerY, Width: 1, Height: 1})
			return m, tickAnimation()
		}
		m.closeTerminal(msg.WindowID)
		m.wm.RemoveWindow(msg.WindowID)
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Normalize single uppercase letters to lowercase so hotkeys work with caps lock.
	if len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' {
		key = strings.ToLower(key)
	}

	// ── Layer 1: UI overlays always take precedence ──

	if m.modal != nil {
		switch key {
		case "esc", "escape", "enter", "q":
			m.modal = nil
		case "up", "k":
			if m.modal.ScrollY > 0 {
				m.modal.ScrollY--
			}
		case "down", "j":
			m.modal.ScrollY++
		}
		return m, nil
	}

	if m.confirmClose != nil {
		switch key {
		case "y":
			m.confirmClose.Selected = 0 // select Yes
			return m.confirmAccept()
		case "n", "esc", "escape":
			m.confirmClose = nil
			return m, nil
		case "enter":
			return m.confirmAccept()
		case "left", "right", "tab", "shift+tab", "h", "l":
			// Toggle selected button
			if m.confirmClose.Selected == 0 {
				m.confirmClose.Selected = 1
			} else {
				m.confirmClose.Selected = 0
			}
		}
		return m, nil
	}

	if m.renameDialog != nil {
		switch key {
		case "enter":
			if w := m.wm.WindowByID(m.renameDialog.WindowID); w != nil {
				newTitle := string(m.renameDialog.Text)
				if newTitle != "" {
					w.Title = newTitle
				}
			}
			m.renameDialog = nil
		case "esc", "escape":
			m.renameDialog = nil
		case "backspace":
			if m.renameDialog.Cursor > 0 {
				m.renameDialog.Text = append(m.renameDialog.Text[:m.renameDialog.Cursor-1], m.renameDialog.Text[m.renameDialog.Cursor:]...)
				m.renameDialog.Cursor--
			}
		case "delete":
			if m.renameDialog.Cursor < len(m.renameDialog.Text) {
				m.renameDialog.Text = append(m.renameDialog.Text[:m.renameDialog.Cursor], m.renameDialog.Text[m.renameDialog.Cursor+1:]...)
			}
		case "left":
			if m.renameDialog.Cursor > 0 {
				m.renameDialog.Cursor--
			}
		case "right":
			if m.renameDialog.Cursor < len(m.renameDialog.Text) {
				m.renameDialog.Cursor++
			}
		case "home", "ctrl+a":
			m.renameDialog.Cursor = 0
		case "end", "ctrl+e":
			m.renameDialog.Cursor = len(m.renameDialog.Text)
		case "ctrl+u":
			m.renameDialog.Text = m.renameDialog.Text[m.renameDialog.Cursor:]
			m.renameDialog.Cursor = 0
		default:
			k := tea.Key(msg)
			if k.Text != "" {
				for _, ch := range k.Text {
					m.renameDialog.Text = append(m.renameDialog.Text, 0)
					copy(m.renameDialog.Text[m.renameDialog.Cursor+1:], m.renameDialog.Text[m.renameDialog.Cursor:])
					m.renameDialog.Text[m.renameDialog.Cursor] = ch
					m.renameDialog.Cursor++
				}
			}
		}
		return m, nil
	}

	if m.exposeMode {
		switch key {
		case "esc", "escape":
			m.exitExpose()
			m.inputMode = ModeNormal
			return m, tickAnimation()
		case "tab", "right", "down", "j", "l":
			m.cycleExposeWindow(1)
			return m, tickAnimation()
		case "shift+tab", "left", "up", "k", "h":
			m.cycleExposeWindow(-1)
			return m, tickAnimation()
		case "enter":
			m.maximizeFocusedWindow()
			m.exitExpose()
			m.inputMode = ModeNormal
			return m, tickAnimation()
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(key[0] - '1') // 0-based
			m.selectExposeByIndex(idx)
			m.maximizeFocusedWindow()
			m.exitExpose()
			m.inputMode = ModeNormal
			return m, tickAnimation()
		}
		return m, nil
	}

	if m.launcher.Visible {
		return m.handleLauncherKey(msg, key)
	}

	if m.menuBar.IsOpen() {
		return m.handleMenuKey(key)
	}

	// ── Layer 2: Prefix pending state (any mode) ──

	if m.prefixPending {
		m.prefixPending = false
		return m.handlePrefixAction(msg, key)
	}

	// ── Layer 3: Global hotkeys — only in non-Terminal modes ──

	if m.inputMode != ModeTerminal {
		switch key {
		case "ctrl+q":
			m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
			return m, nil
		case "f1":
			m.modal = m.helpOverlay()
			return m, nil
		case "f10":
			m.menuBar.OpenMenu(0)
			return m, nil
		case "f9":
			if m.exposeMode {
				m.exitExpose()
			} else {
				m.enterExpose()
			}
			return m, tickAnimation()
		}
	}

	// ── Layer 4: Mode-specific dispatch ──

	switch m.inputMode {
	case ModeTerminal:
		return m.handleTerminalModeKey(msg, key)
	case ModeCopy:
		return m.handleCopyModeKey(msg, key)
	default:
		return m.handleNormalModeKey(msg, key)
	}
}

// handleTerminalModeKey handles keys when in Terminal mode.
// Only the prefix key is intercepted; everything else goes to the terminal.
func (m Model) handleTerminalModeKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// Prefix key → enter prefix pending state
	if key == m.keybindings.Prefix {
		m.prefixPending = true
		return m, nil
	}

	// Forward everything else to the focused terminal
	if fw := m.wm.FocusedWindow(); fw != nil {
		if term, ok := m.terminals[fw.ID]; ok {
			k := tea.Key(msg)
			term.SendKey(k.Code, k.Mod, k.Text)
			return m, nil
		}
	}

	// No terminal focused — switch to Normal mode (key is NOT reprocessed)
	m.inputMode = ModeNormal
	return m, nil
}

// handlePrefixAction dispatches an action after the prefix key was pressed.
func (m Model) handlePrefixAction(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// Double-prefix: send prefix key to terminal (like tmux Ctrl+b Ctrl+b)
	if key == m.keybindings.Prefix {
		if fw := m.wm.FocusedWindow(); fw != nil {
			if term, ok := m.terminals[fw.ID]; ok {
				k := tea.Key(msg)
				term.SendKey(k.Code, k.Mod, k.Text)
			}
		}
		return m, nil
	}

	// Esc after prefix → exit terminal mode
	if key == "esc" || key == "escape" {
		m.inputMode = ModeNormal
		return m, nil
	}

	// Look up action
	action := m.actionMap[key]

	// Handle 1-9 for window-by-index
	if action == "" && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		idx := int(key[0]-'0') - 1
		windows := m.wm.Windows()
		if idx < len(windows) {
			m.wm.FocusWindow(windows[idx].ID)
		}
		return m, nil
	}

	// Unrecognized key — forward to terminal (prefix consumed, matches tmux behavior)
	if action == "" {
		if fw := m.wm.FocusedWindow(); fw != nil {
			if term, ok := m.terminals[fw.ID]; ok {
				k := tea.Key(msg)
				term.SendKey(k.Code, k.Mod, k.Text)
			}
		}
		return m, nil
	}

	// Execute the action
	return m.executeAction(action, msg, key)
}

// Actions that keep the user in Terminal mode after prefix.
var terminalStayActions = map[string]bool{
	"snap_left": true, "snap_right": true,
	"maximize": true, "restore": true,
	"tile_all": true, "new_terminal": true,
	"next_window": true, "prev_window": true,
}

// executeAction runs a named WM action. Called from both Normal mode (direct)
// and Terminal mode (via prefix). When invoked via prefix, non-geometry actions
// switch to Normal mode so overlays/dialogs work.
func (m Model) executeAction(action string, msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	wasTerminal := m.inputMode == ModeTerminal

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

	case "close_window":
		if fw := m.wm.FocusedWindow(); fw != nil {
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
		if fw := m.wm.FocusedWindow(); fw != nil {
			if _, ok := m.terminals[fw.ID]; ok {
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
		return m, nil

	case "snap_left":
		if fw := m.wm.FocusedWindow(); fw != nil {
			from := fw.Rect
			window.SnapLeft(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "snap_right":
		if fw := m.wm.FocusedWindow(); fw != nil {
			from := fw.Rect
			window.SnapRight(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimSnap, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "maximize":
		if fw := m.wm.FocusedWindow(); fw != nil {
			from := fw.Rect
			window.Maximize(fw, m.wm.WorkArea())
			m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "restore":
		if fw := m.wm.FocusedWindow(); fw != nil {
			from := fw.Rect
			window.Restore(fw)
			m.startWindowAnimation(fw.ID, AnimRestore, from, fw.Rect)
			m.resizeTerminalForWindow(fw)
		}
		return m, tickAnimation()

	case "tile_all":
		m.animateTileAll()
		return m, tickAnimation()

	case "expose":
		m.enterExpose()
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()

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

	case "menu_apps":
		m.menuBar.OpenMenu(1)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil

	case "menu_view":
		m.menuBar.OpenMenu(2)
		if wasTerminal {
			m.inputMode = ModeNormal
		}
		return m, nil
	}

	return m, nil
}

// handleNormalModeKey handles keys when in Normal (window management) mode.
func (m Model) handleNormalModeKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	// ── Dock navigation sub-mode ──
	if m.dockFocused {
		return m.handleDockNav(key)
	}

	// Prefix key works in Normal mode too — activates PREFIX badge
	if key == m.keybindings.Prefix {
		m.prefixPending = true
		return m, nil
	}

	// Esc in normal mode — exit dock focus or no-op
	if key == "esc" || key == "escape" {
		m.dockFocused = false
		m.dock.SetHover(-1)
		return m, nil
	}

	// Focus window by number (1-9)
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		idx := int(key[0]-'0') - 1
		windows := m.wm.Windows()
		if idx < len(windows) {
			m.wm.FocusWindow(windows[idx].ID)
		}
		return m, nil
	}

	// Look up action in keymap
	action := m.actionMap[key]
	if action != "" {
		return m.executeAction(action, msg, key)
	}

	return m, nil
}

// handleDockNav handles keyboard navigation of the dock bar.
func (m Model) handleDockNav(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "left", "h":
		idx := m.dock.HoverIndex - 1
		if idx < 0 {
			idx = m.dock.ItemCount() - 1
		}
		m.dock.SetHover(idx)
		return m, nil
	case "right", "l":
		idx := m.dock.HoverIndex + 1
		if idx >= m.dock.ItemCount() {
			idx = 0
		}
		m.dock.SetHover(idx)
		return m, nil
	case "enter":
		return m.activateDockItem(m.dock.HoverIndex)
	case "esc", "escape", "d", "up", "k":
		m.dockFocused = false
		m.dock.SetHover(-1)
		return m, nil
	}
	return m, nil
}

// activateDockItem launches or restores the dock item at the given index.
func (m Model) activateDockItem(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.dock.Items) {
		return m, nil
	}
	item := m.dock.Items[idx]
	m.dockFocused = false
	m.dock.SetHover(-1)

	switch item.Special {
	case "launcher":
		m.launcher.Toggle()
		return m, nil
	case "expose":
		m.enterExpose()
		return m, tickAnimation()
	case "minimized":
		// Restore the minimized window
		if w := m.wm.WindowByID(item.WindowID); w != nil {
			m.restoreMinimizedWindow(w)
			return m, tickAnimation()
		}
		return m, nil
	default:
		// Launch the app
		if item.Command != "" {
			m.inputMode = ModeNormal
			cmd := m.openTerminalWindowWith(item.Command, item.Args)
			return m, cmd
		}
	}
	return m, nil
}

// handleCopyModeKey handles keys when in Copy mode (vim-style scrollback).
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
	return m, nil
}

func (m Model) handleCopyModeKey(_ tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	fw := m.wm.FocusedWindow()
	if fw == nil {
		m.scrollOffset = 0
		m.inputMode = ModeNormal
		return m, nil
	}
	term := m.terminals[fw.ID]
	if term == nil {
		m.scrollOffset = 0
		m.inputMode = ModeNormal
		return m, nil
	}
	maxScroll := term.ScrollbackLen()

	switch key {
	case "esc", "escape", "q":
		m.scrollOffset = 0
		m.inputMode = ModeNormal
	case "i":
		m.scrollOffset = 0
		m.inputMode = ModeTerminal
	case "up", "k":
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}
	case "down", "j":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case "pgup":
		page := term.Height()
		m.scrollOffset += page
		if m.scrollOffset > maxScroll {
			m.scrollOffset = maxScroll
		}
	case "pgdown":
		page := term.Height()
		m.scrollOffset -= page
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	case "home", "g":
		m.scrollOffset = maxScroll
	case "end", "shift+g":
		m.scrollOffset = 0
	}
	return m, nil
}

func (m Model) handleLauncherKey(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c", "ctrl+q":
		m.launcher.Hide()
		m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
		return m, nil
	case "esc", "escape", "ctrl+space", "ctrl+/":
		m.launcher.Hide()
		return m, nil
	case "up":
		m.launcher.MoveSelection(-1)
	case "down":
		m.launcher.MoveSelection(1)
	case "backspace":
		m.launcher.Backspace()
	case "enter":
		entry := m.launcher.SelectedEntry()
		m.launcher.Hide()
		if entry != nil {
			return m.launchApp(entry.Command, entry.Args)
		}
	default:
		// Type printable characters into the search
		k := tea.Key(msg)
		if k.Text != "" {
			for _, ch := range k.Text {
				m.launcher.TypeChar(ch)
			}
		}
	}
	return m, nil
}

func (m Model) handleMenuKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "escape", "f10":
		m.menuBar.CloseMenu()
	case "up":
		m.menuBar.MoveHover(-1)
	case "down":
		m.menuBar.MoveHover(1)
	case "left":
		m.menuBar.MoveMenu(-1)
	case "right":
		m.menuBar.MoveMenu(1)
	case "enter":
		action := m.menuBar.SelectedAction()
		m.menuBar.CloseMenu()
		return m.executeMenuAction(action)
	}
	return m, nil
}

func (m Model) executeMenuAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "new_terminal":
		cmd := m.openTerminalWindow()
		return m, cmd
	case "quit":
		m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}
		return m, nil
	case "tile_all":
		window.TileAll(m.wm.Windows(), m.wm.WorkArea())
		m.resizeAllTerminals()
	case "snap_left":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapLeft(fw, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
	case "snap_right":
		if fw := m.wm.FocusedWindow(); fw != nil {
			window.SnapRight(fw, m.wm.WorkArea())
			m.resizeTerminalForWindow(fw)
		}
	case "minimize":
		if fw := m.wm.FocusedWindow(); fw != nil {
			m.minimizeWindow(fw)
			return m, tickAnimation()
		}
	case "toggle_icons_only":
		m.dock.IconsOnly = !m.dock.IconsOnly
		cfg := config.LoadUserConfig()
		cfg.IconsOnly = m.dock.IconsOnly
		config.SaveUserConfig(cfg)
	case "help_keys":
		m.modal = m.helpOverlay()
	case "about":
		m.modal = m.aboutOverlay()
	case "launch_terminal":
		cmd := m.openTerminalWindow()
		return m, cmd
	case "launch_nvim":
		cmd := m.openTerminalWindowMaximized("nvim", nil)
		return m, cmd
	case "launch_files":
		cmd := m.openTerminalWindowMaximized("mc", nil)
		return m, cmd
	case "launch_htop":
		cmd := m.openTerminalWindowMaximized("htop", nil)
		return m, cmd
	case "launch_calc":
		cmd := m.openTerminalWindowWith("python3", nil)
		return m, cmd
	case "detach":
		// Write a message hinting the user to use the detach key combo.
		// The actual detach is handled client-side (Ctrl+\ then d).
		m.modal = &ModalOverlay{
			Title: "Detach",
			Lines: []string{
				"Press F12 to detach.",
				"Session will keep running in the background.",
				"",
				"Re-attach with: termdesk",
			},
		}
		return m, nil
	case "theme_retro", "theme_modern", "theme_tokyonight", "theme_catppuccin":
		name := action[6:] // strip "theme_" prefix
		m.theme = config.GetTheme(name)
		// Update terminal default background for new theme (Termux fix)
		if bg := m.theme.DesktopBg; len(bg) == 7 && bg[0] == '#' {
			fmt.Fprintf(os.Stdout, "\x1b]11;rgb:%s/%s/%s\x07", bg[1:3], bg[3:5], bg[5:7])
		}
		// Persist theme choice
		cfg := config.LoadUserConfig()
		cfg.Theme = name
		config.SaveUserConfig(cfg)
	}
	return m, nil
}

func (m Model) handleMouseClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	p := geometry.Point{X: mouse.X, Y: mouse.Y}

	// If modal is showing, any click dismisses it
	if m.modal != nil {
		m.modal = nil
		return m, nil
	}

	// If confirm dialog is showing, check for button clicks
	if m.confirmClose != nil {
		title := m.confirmClose.Title
		boxW := runeLen(title) + 6
		if boxW < 28 {
			boxW = 28
		}
		boxH := 6
		startX := (m.width - boxW) / 2
		startY := (m.height - boxH) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		yesLabel := " Yes "
		noLabel := "  No "
		gap := boxW - 2 - len(yesLabel) - len(noLabel)
		if gap < 4 {
			gap = 4
		}
		yesX := startX + 1 + (gap / 3)
		noX := startX + boxW - 1 - len(noLabel) - (gap / 3)
		btnY := startY + 4

		if mouse.Y == btnY {
			if mouse.X >= yesX && mouse.X < yesX+len(yesLabel) {
				m.confirmClose.Selected = 0
				return m.confirmAccept()
			}
			if mouse.X >= noX && mouse.X < noX+len(noLabel) {
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

	// If exposé mode, click selects a window and maximizes it
	if m.exposeMode {
		if mouse.Y != m.height-1 && mouse.Y != 0 { // not dock or menubar
			m.selectExposeWindow(mouse.X, mouse.Y)
			m.maximizeFocusedWindow()
			m.exitExpose()
			m.inputMode = ModeNormal
		}
		return m, tickAnimation()
	}

	// If launcher is open, dismiss on click outside
	if m.launcher.Visible {
		m.launcher.Hide()
		return m, nil
	}

	// Check if click is on dock (bottom row)
	if mouse.Y == m.height-1 {
		idx := m.dock.ItemAtX(mouse.X)
		if idx >= 0 && idx < len(m.dock.Items) {
			item := m.dock.Items[idx]
			// Handle special dock items
			switch item.Special {
			case "launcher":
				m.launcher.Toggle()
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
			}
			// Regular app launch — with dock pulse animation
			m.inputMode = ModeNormal
			m.startDockPulse(idx)
			if item.Command == "$SHELL" || item.Command == "" {
				cmd := m.openTerminalWindow()
				return m, tea.Batch(cmd, tickAnimation())
			}
			// Launch space-hungry apps maximized
			var cmd tea.Cmd
			switch item.Command {
			case "nvim", "mc", "htop", "btop", "spf", "ranger", "lf", "yazi":
				cmd = m.openTerminalWindowMaximized(item.Command, item.Args)
			default:
				cmd = m.openTerminalWindowWith(item.Command, item.Args)
			}
			return m, tea.Batch(cmd, tickAnimation())
		}
		return m, nil
	}

	// Check if click is on menu bar (y=0)
	if mouse.Y == 0 {
		// Check right-side zones first (clock, CPU, MEM, mode badge)
		// Use effective width (same as rendering — leave room for mode badge)
		mLabel := modeBadge(m.inputMode)
		mLabelW := runewidth.StringWidth(mLabel)
		effW := m.width - mLabelW
		if effW < 1 {
			effW = 1
		}
		// Check if click is on the mode badge (far right)
		modeX := m.width - mLabelW
		if mouse.X >= modeX && mouse.X < m.width {
			m.cycleInputMode()
			return m, nil
		}
		for _, zone := range m.menuBar.RightZones(effW) {
			if mouse.X >= zone.Start && mouse.X < zone.End {
				return m.handleMenuBarRightClick(zone.Type)
			}
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

	// Check if click is on open dropdown
	if m.menuBar.IsOpen() {
		positions := m.menuBar.MenuXPositions()
		dropX := positions[m.menuBar.OpenIndex]
		dropY := 1 // dropdown starts at row 1
		itemIdx := m.menuBar.DropdownItemAtY(mouse.Y - dropY - 1) // -1 for top border
		menu := m.menuBar.Menus[m.menuBar.OpenIndex]
		dropWidth := 0
		for _, item := range menu.Items {
			w := len(item.Label) + 4
			if item.Shortcut != "" {
				w += len(item.Shortcut) + 2
			}
			if w > dropWidth {
				dropWidth = w
			}
		}
		if mouse.X >= dropX && mouse.X < dropX+dropWidth && itemIdx >= 0 {
			m.menuBar.HoverIndex = itemIdx
			action := m.menuBar.SelectedAction()
			m.menuBar.CloseMenu()
			return m.executeMenuAction(action)
		}
		// Click outside dropdown closes it
		m.menuBar.CloseMenu()
	}

	// Find which window was clicked
	w := m.wm.WindowAt(p)
	if w == nil {
		return m, nil
	}

	// Focus the clicked window
	m.wm.FocusWindow(w.ID)

	// Determine what was clicked
	zone := window.HitTestWithTheme(w, p, m.theme.CloseButton, m.theme.MaxButton)

	switch zone {
	case window.HitCloseButton:
		m.confirmClose = &ConfirmDialog{
			WindowID: w.ID,
			Title:    fmt.Sprintf("Close \"%s\"?", w.Title),
		}
		return m, nil

	case window.HitMinButton:
		m.minimizeWindow(w)
		return m, tickAnimation()

	case window.HitMaxButton:
		from := w.Rect
		window.ToggleMaximize(w, m.wm.WorkArea())
		m.startWindowAnimation(w.ID, AnimMaximize, from, w.Rect)
		m.resizeTerminalForWindow(w)
		return m, tickAnimation()

	case window.HitSnapLeftButton:
		from := w.Rect
		window.SnapLeft(w, m.wm.WorkArea())
		m.startWindowAnimation(w.ID, AnimSnap, from, w.Rect)
		m.resizeTerminalForWindow(w)
		return m, tickAnimation()

	case window.HitSnapRightButton:
		from := w.Rect
		window.SnapRight(w, m.wm.WorkArea())
		m.startWindowAnimation(w.ID, AnimSnap, from, w.Rect)
		m.resizeTerminalForWindow(w)
		return m, tickAnimation()

	case window.HitContent:
		// Clicking on terminal content enters Terminal mode
		if _, ok := m.terminals[w.ID]; ok {
			m.inputMode = ModeTerminal
		}
		// Forward mouse click to terminal via emulator (respects mouse mode)
		if term, ok := m.terminals[w.ID]; ok {
			cr := w.ContentRect()
			localX := mouse.X - cr.X // 0-indexed
			localY := mouse.Y - cr.Y
			btn := teaToUvButton(mouse.Button)
			term.SendMouse(btn, localX, localY, false)
		}

	case window.HitTitleBar, window.HitBorderN, window.HitBorderS, window.HitBorderE,
		window.HitBorderW, window.HitBorderNE, window.HitBorderNW,
		window.HitBorderSE, window.HitBorderSW:
		dragMode := window.DragModeForZone(zone)
		if dragMode != window.DragNone && (w.Draggable || zone != window.HitTitleBar) {
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
	// Update dock hover
	if mouse.Y == m.height-1 {
		m.dock.SetHover(m.dock.ItemAtX(mouse.X))
	} else {
		m.dock.SetHover(-1)
	}

	if !m.drag.Active {
		// Forward motion to focused terminal if over content area
		if fw := m.wm.FocusedWindow(); fw != nil {
			cr := fw.ContentRect()
			p := geometry.Point{X: mouse.X, Y: mouse.Y}
			if cr.Contains(p) {
				if term, ok := m.terminals[fw.ID]; ok {
					localX := mouse.X - cr.X // 0-indexed
					localY := mouse.Y - cr.Y
					btn := teaToUvButton(mouse.Button)
					term.SendMouseMotion(btn, localX, localY)
				}
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
	if m.drag.Active {
		// Resize terminal after drag completes
		w := m.wm.WindowByID(m.drag.WindowID)
		if w != nil {
			m.resizeTerminalForWindow(w)
		}
		m.drag = window.DragState{}
		return m, nil
	}
	m.drag = window.DragState{}

	// Forward release to focused terminal
	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	if fw := m.wm.FocusedWindow(); fw != nil {
		cr := fw.ContentRect()
		if cr.Contains(p) {
			if term, ok := m.terminals[fw.ID]; ok {
				localX := mouse.X - cr.X // 0-indexed
				localY := mouse.Y - cr.Y
				btn := teaToUvButton(mouse.Button)
				term.SendMouse(btn, localX, localY, true)
			}
		}
	}
	return m, nil
}

func (m Model) handleMouseWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	p := geometry.Point{X: mouse.X, Y: mouse.Y}
	w := m.wm.WindowAt(p)
	if w == nil {
		return m, nil
	}
	cr := w.ContentRect()
	if cr.Contains(p) {
		if term, ok := m.terminals[w.ID]; ok {
			localX := mouse.X - cr.X // 0-indexed
			localY := mouse.Y - cr.Y
			btn := teaToUvButton(mouse.Button)
			// SGR mouse encoding — works if terminal app enabled mouse mode.
			// Apps without mouse mode won't receive the event (emulator drops silently).
			term.SendMouseWheel(btn, localX, localY)
		}
	}
	return m, nil
}

// launchApp launches a command in a new terminal window.
func (m Model) launchApp(command string, args []string) (tea.Model, tea.Cmd) {
	m.inputMode = ModeNormal
	cmd := m.openTerminalWindowWith(command, args)
	return m, cmd
}

// openTerminalWindow creates a new window running the default shell.
func (m *Model) openTerminalWindow() tea.Cmd {
	return m.openTerminalWindowWith("", nil)
}

const maxWindows = 9

// openTerminalWindowWith creates a new window running the specified command.
// If command is empty, it launches the default shell.
func (m *Model) openTerminalWindowWith(command string, args []string) tea.Cmd {
	if len(m.wm.Windows()) >= maxWindows {
		return nil
	}
	m.nextWID++
	id := fmt.Sprintf("win-%d", m.nextWID)
	title := fmt.Sprintf("Terminal %d", m.nextWID)
	if command != "" {
		title = command
	}

	wa := m.wm.WorkArea()
	offset := (m.nextWID - 1) % 10
	x := wa.X + 2 + offset*2
	y := wa.Y + 1 + offset
	w := min(80, wa.Width-x)
	h := min(24, wa.Height-y)
	if w < window.MinWindowWidth {
		w = window.MinWindowWidth
	}
	if h < window.MinWindowHeight {
		h = window.MinWindowHeight
	}

	finalRect := geometry.Rect{X: x, Y: y, Width: w, Height: h}
	win := window.NewWindow(id, title, finalRect, nil)
	win.Command = command
	win.TitleBarHeight = m.theme.TitleBarRows()
	m.wm.AddWindow(win)

	// Animate window opening — grow from center
	centerX := x + w/2
	centerY := y + h/2
	m.startWindowAnimation(id, AnimOpen,
		geometry.Rect{X: centerX, Y: centerY, Width: 1, Height: 1},
		finalRect)

	// Use window's actual content rect (accounts for title bar height)
	cr := win.ContentRect()
	contentW := cr.Width
	contentH := cr.Height
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	var term *terminal.Terminal
	var err error
	if command == "" {
		term, err = terminal.NewShell(contentW, contentH)
	} else {
		term, err = terminal.New(command, args, contentW, contentH)
	}
	if err != nil {
		// Fall back to a window with no terminal
		return nil
	}

	m.terminals[id] = term

	// Spawn background reader goroutine — reads PTY output and sends messages
	// via program.Send(). Rate-limited: at most one PtyOutputMsg per 8ms (~120fps)
	// to avoid flooding Bubble Tea with re-renders during burst output (e.g. dmesg).
	// Uses time.AfterFunc which fires in its own goroutine — unlike time.NewTimer,
	// its callback runs even while ReadOnce blocks on PTY read.
	if m.progRef != nil && m.progRef.p != nil {
		p := m.progRef.p
		go func() {
			defer func() {
				recover() // prevent panics from crashing the entire app
				p.Send(PtyClosedMsg{WindowID: id})
			}()
			var (
				mu       sync.Mutex
				lastSend time.Time
				pending  *time.Timer
			)
			const minInterval = 16 * time.Millisecond
			readBuf := make([]byte, 32768)
			for {
				n, err := term.ReadOnce(readBuf)
				if n > 0 {
					mu.Lock()
					now := time.Now()
					if now.Sub(lastSend) >= minInterval {
						if pending != nil {
							pending.Stop()
							pending = nil
						}
						lastSend = now
						mu.Unlock()
						p.Send(PtyOutputMsg{WindowID: id})
					} else if pending == nil {
						pending = time.AfterFunc(minInterval, func() {
							mu.Lock()
							pending = nil
							lastSend = time.Now()
							mu.Unlock()
							p.Send(PtyOutputMsg{WindowID: id})
						})
						mu.Unlock()
					} else {
						mu.Unlock()
					}
				}
				if err != nil {
					mu.Lock()
					if pending != nil {
						pending.Stop()
					}
					mu.Unlock()
					p.Send(PtyOutputMsg{WindowID: id})
					p.Send(PtyClosedMsg{WindowID: id, Err: err})
					return
				}
			}
		}()
	}
	return tickAnimation()
}

// openTerminalWindowMaximized creates a new maximized terminal window.
func (m *Model) openTerminalWindowMaximized(command string, args []string) tea.Cmd {
	cmd := m.openTerminalWindowWith(command, args)
	// Find the window we just created and maximize it
	if fw := m.wm.FocusedWindow(); fw != nil {
		from := fw.Rect
		window.Maximize(fw, m.wm.WorkArea())
		m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
		m.resizeTerminalForWindow(fw)
	}
	return cmd
}

// openDemoWindow creates a demo window without a terminal (for testing).
func (m *Model) openDemoWindow() {
	m.nextWID++
	id := fmt.Sprintf("win-%d", m.nextWID)
	title := fmt.Sprintf("Window %d", m.nextWID)

	wa := m.wm.WorkArea()
	offset := (m.nextWID - 1) % 10
	x := wa.X + 2 + offset*2
	y := wa.Y + 1 + offset
	w := min(60, wa.Width-x)
	h := min(15, wa.Height-y)
	if w < window.MinWindowWidth {
		w = window.MinWindowWidth
	}
	if h < window.MinWindowHeight {
		h = window.MinWindowHeight
	}

	win := window.NewWindow(id, title, geometry.Rect{X: x, Y: y, Width: w, Height: h}, nil)
	m.wm.AddWindow(win)
}

// enterExpose transitions into exposé mode with animations.
func (m *Model) enterExpose() {
	m.exposeMode = true
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(visible) == 0 {
		return
	}
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Animate focused window to center
	focTarget := exposeTargetRect(focusedWin, wa, true)
	m.startExposeAnimation(focusedWin.ID, AnimExpose, focusedWin.Rect, focTarget)

	// Animate unfocused windows to bottom strip
	bgTargets := exposeBgTargets(visible, focusedWin.ID, wa)
	for id, target := range bgTargets {
		if w := m.wm.WindowByID(id); w != nil {
			m.startExposeAnimation(id, AnimExpose, w.Rect, target)
		}
	}
}

// exitExpose transitions out of exposé mode with animations.
func (m *Model) exitExpose() {
	m.exposeMode = false
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(visible) == 0 {
		return
	}
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Animate focused window from center back to its real position
	focFrom := exposeTargetRect(focusedWin, wa, true)
	m.startExposeAnimation(focusedWin.ID, AnimExposeExit, focFrom, focusedWin.Rect)

	// Animate unfocused windows from bottom strip back to their positions
	bgTargets := exposeBgTargets(visible, focusedWin.ID, wa)
	for id, from := range bgTargets {
		if w := m.wm.WindowByID(id); w != nil {
			m.startExposeAnimation(id, AnimExposeExit, from, w.Rect)
		}
	}
}

// cycleExposeWindow animates switching focus in exposé mode.
// direction: +1 = forward, -1 = backward.
func (m *Model) cycleExposeWindow(direction int) {
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var oldFocused *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				oldFocused = w
			}
		}
	}
	if len(visible) < 2 || oldFocused == nil {
		if direction > 0 {
			m.wm.CycleForward()
		} else {
			m.wm.CycleBackward()
		}
		return
	}

	// Compute old focused window's center target rect (where it currently is)
	oldCenterRect := exposeTargetRect(oldFocused, wa, true)

	// Cycle focus
	if direction > 0 {
		m.wm.CycleForward()
	} else {
		m.wm.CycleBackward()
	}

	var newFocused *window.Window
	for _, w := range visible {
		if w.Focused {
			newFocused = w
			break
		}
	}
	if newFocused == nil || newFocused.ID == oldFocused.ID {
		return
	}

	// Where the new focused window currently is in the bg strip
	bgTargets := exposeBgTargets(visible, newFocused.ID, wa)
	newBgRect, ok := bgTargets[oldFocused.ID]
	if !ok {
		// Old focused goes to some position in the strip — recalculate with new focus
		bgTargets = exposeBgTargets(visible, newFocused.ID, wa)
		newBgRect = bgTargets[oldFocused.ID]
	}

	// Where the new focused window was in the old bg strip
	oldBgTargets := exposeBgTargets(visible, oldFocused.ID, wa)
	newOldBgRect := oldBgTargets[newFocused.ID]

	// Animate: old focused → shrink to its new bg position
	m.startExposeAnimation(oldFocused.ID, AnimExpose, oldCenterRect, newBgRect)

	// Animate: new focused → grow from its old bg position to center
	newCenterRect := exposeTargetRect(newFocused, wa, true)
	m.startExposeAnimation(newFocused.ID, AnimExpose, newOldBgRect, newCenterRect)

	// Animate all other windows repositioning in the bg strip
	for _, w := range visible {
		if w.ID == oldFocused.ID || w.ID == newFocused.ID {
			continue
		}
		oldPos := oldBgTargets[w.ID]
		newPos := bgTargets[w.ID]
		if oldPos != newPos {
			m.startExposeAnimation(w.ID, AnimExpose, oldPos, newPos)
		}
	}
}

// exposeTargetRect calculates the exposé display rect for a window.
// If focused=true, returns a large centered rect; otherwise a small thumbnail.
func exposeTargetRect(w *window.Window, wa geometry.Rect, focused bool) geometry.Rect {
	if focused {
		// Centered, up to 70% of work area
		focW := w.Rect.Width * 7 / 10
		focH := w.Rect.Height * 7 / 10
		maxW := wa.Width * 7 / 10
		maxH := wa.Height * 7 / 10
		if focW > maxW {
			focW = maxW
		}
		if focH > maxH {
			focH = maxH
		}
		if focW < 16 {
			focW = 16
		}
		if focH < 8 {
			focH = 8
		}
		return geometry.Rect{
			X:      wa.X + (wa.Width-focW)/2,
			Y:      wa.Y + (wa.Height-focH)/3,
			Width:  focW,
			Height: focH,
		}
	}
	// Small thumbnail — used for bg targets calculation
	return geometry.Rect{X: wa.X, Y: wa.Y, Width: 16, Height: 6}
}

// exposeBgTargets returns a map of window ID → target rect for unfocused windows
// arranged along the bottom of the work area.
func exposeBgTargets(visible []*window.Window, focusedID string, wa geometry.Rect) map[string]geometry.Rect {
	targets := make(map[string]geometry.Rect)
	bgCount := 0
	for _, w := range visible {
		if w.ID != focusedID {
			bgCount++
		}
	}
	if bgCount == 0 {
		return targets
	}

	thumbW := 16
	thumbH := 6
	totalThumbW := bgCount * (thumbW + 1)
	if totalThumbW > wa.Width-2 {
		thumbW = (wa.Width - 2 - bgCount) / bgCount
		if thumbW < 8 {
			thumbW = 8
		}
	}
	startX := wa.X + (wa.Width-bgCount*(thumbW+1)+1)/2
	thumbY := wa.Y + wa.Height - thumbH - 1

	idx := 0
	for _, w := range visible {
		if w.ID == focusedID {
			continue
		}
		x := startX + idx*(thumbW+1)
		targets[w.ID] = geometry.Rect{X: x, Y: thumbY, Width: thumbW, Height: thumbH}
		idx++
	}
	return targets
}

// selectExposeWindow finds which mini-window was clicked in exposé mode and focuses it.
// Layout matches RenderExpose: focused window centered, others in bottom strip.
func (m *Model) selectExposeWindow(mouseX, mouseY int) {
	wa := m.wm.WorkArea()
	var visible []*window.Window
	var focusedWin *window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
			if w.Focused {
				focusedWin = w
			}
		}
	}
	if len(visible) == 0 {
		return
	}
	if focusedWin == nil {
		focusedWin = visible[0]
	}

	// Check focused center window first
	focW := focusedWin.Rect.Width * 7 / 10
	focH := focusedWin.Rect.Height * 7 / 10
	maxW := wa.Width * 7 / 10
	maxH := wa.Height * 7 / 10
	if focW > maxW {
		focW = maxW
	}
	if focH > maxH {
		focH = maxH
	}
	if focW < 16 {
		focW = 16
	}
	if focH < 8 {
		focH = 8
	}
	focX := wa.X + (wa.Width-focW)/2
	focY := wa.Y + (wa.Height-focH)/3
	if mouseX >= focX && mouseX < focX+focW && mouseY >= focY && mouseY < focY+focH {
		// Already focused — just exit exposé
		return
	}

	// Check background thumbnails (bottom strip)
	bgCount := len(visible) - 1
	if bgCount <= 0 {
		return
	}
	thumbW := 16
	thumbH := 6
	totalThumbW := bgCount * (thumbW + 1)
	if totalThumbW > wa.Width-2 {
		thumbW = (wa.Width - 2 - bgCount) / bgCount
		if thumbW < 8 {
			thumbW = 8
		}
	}
	startX := wa.X + (wa.Width-bgCount*(thumbW+1)+1)/2
	thumbY := wa.Y + wa.Height - thumbH - 1

	idx := 0
	for _, w := range visible {
		if w.ID == focusedWin.ID {
			continue
		}
		x := startX + idx*(thumbW+1)
		if mouseX >= x && mouseX < x+thumbW && mouseY >= thumbY && mouseY < thumbY+thumbH {
			m.wm.FocusWindow(w.ID)
			return
		}
		idx++
	}
}

// selectExposeByIndex selects the Nth visible window (0-based) in exposé mode.
func (m *Model) selectExposeByIndex(idx int) {
	var visible []*window.Window
	for _, w := range m.wm.Windows() {
		if w.Visible && !w.Minimized {
			visible = append(visible, w)
		}
	}
	if idx >= 0 && idx < len(visible) {
		m.wm.FocusWindow(visible[idx].ID)
	}
}

// maximizeFocusedWindow maximizes the currently focused window.
func (m *Model) maximizeFocusedWindow() {
	if fw := m.wm.FocusedWindow(); fw != nil && !fw.IsMaximized() {
		from := fw.Rect
		window.Maximize(fw, m.wm.WorkArea())
		m.startWindowAnimation(fw.ID, AnimMaximize, from, fw.Rect)
		m.resizeTerminalForWindow(fw)
	}
}

// closeTerminal closes and removes a terminal by window ID.
func (m *Model) closeTerminal(windowID string) {
	if term, ok := m.terminals[windowID]; ok {
		term.Close()
		delete(m.terminals, windowID)
	}
}

// closeAllTerminals closes all active terminals.
func (m *Model) closeAllTerminals() {
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

// resizeTerminalForWindow resizes the terminal to match its window's content area.
func (m *Model) resizeTerminalForWindow(w *window.Window) {
	if term, ok := m.terminals[w.ID]; ok {
		cr := w.ContentRect()
		if cr.Width > 0 && cr.Height > 0 {
			term.Resize(cr.Width, cr.Height)
		}
	}
}

// animateTileAll tiles all windows with animations.
func (m *Model) animateTileAll() {
	windows := m.wm.Windows()
	// Save current rects
	fromRects := make(map[string]geometry.Rect, len(windows))
	for _, w := range windows {
		fromRects[w.ID] = w.Rect
	}
	// Apply tiling
	window.TileAll(windows, m.wm.WorkArea())
	// Create animations
	for _, w := range windows {
		if from, ok := fromRects[w.ID]; ok {
			m.startWindowAnimation(w.ID, AnimTile, from, w.Rect)
		}
	}
	m.resizeAllTerminals()
}

// resizeAllTerminals resizes all terminals to match their windows.
func (m *Model) resizeAllTerminals() {
	for _, w := range m.wm.Windows() {
		m.resizeTerminalForWindow(w)
	}
}

// handleMenuBarRightClick handles clicks on the right side of the menu bar.
func (m Model) handleMenuBarRightClick(zoneType string) (tea.Model, tea.Cmd) {
	switch zoneType {
	case "cpu", "mem":
		cmd := m.openTerminalWindowMaximized("htop", nil)
		return m, cmd
	case "clock":
		m.modal = &ModalOverlay{
			Title: "Date & Time",
			Lines: []string{
				time.Now().Format("Monday, January 2, 2006"),
				time.Now().Format("03:04:05 PM MST"),
			},
		}
	}
	return m, nil
}

// helpOverlay returns a modal with keybinding help.
func (m *Model) helpOverlay() *ModalOverlay {
	kb := m.keybindings
	pfx := strings.ToUpper(kb.Prefix)
	return &ModalOverlay{
		Title: "Keybindings",
		Lines: []string{
			"NORMAL mode (window management):",
			fmt.Sprintf("  %-14s Quit", kb.Quit),
			fmt.Sprintf("  %-14s \u2192 Terminal mode", kb.EnterTerminal+" / Enter"),
			fmt.Sprintf("  %-14s New Terminal", kb.NewTerminal+" / Ctrl+N"),
			fmt.Sprintf("  %-14s Close Window", kb.CloseWindow+" / Ctrl+W"),
			fmt.Sprintf("  %-14s Minimize to Dock", kb.Minimize),
			fmt.Sprintf("  %-14s Rename Window", kb.Rename),
			fmt.Sprintf("  %-14s Navigate Dock", kb.DockFocus),
			fmt.Sprintf("  %-14s Launcher", kb.Launcher+"/Ctrl+Sp"),
			fmt.Sprintf("  %-14s Next Window", kb.NextWindow),
			fmt.Sprintf("  %-14s Snap Left/Right", kb.SnapLeft+" / "+kb.SnapRight),
			fmt.Sprintf("  %-14s Restore / Maximize", kb.Restore+" / "+kb.Maximize),
			fmt.Sprintf("  %-14s Tile All", kb.TileAll),
			fmt.Sprintf("  %-14s Expos\u00e9", kb.Expose),
			fmt.Sprintf("  %-14s File / Apps / View", kb.MenuFile+" / "+kb.MenuApps+" / "+kb.MenuView),
			"  1-9           Focus Window #N",
			"",
			"TERMINAL mode (prefix system):",
			fmt.Sprintf("  %-14s PREFIX key", pfx),
			"  F2            Exit to Normal (hardcoded)",
			"",
			fmt.Sprintf("PREFIX (%s) + action:", pfx),
			fmt.Sprintf("  + Esc         \u2192 Normal mode"),
			fmt.Sprintf("  + %-11s Quit", kb.Quit),
			fmt.Sprintf("  + %-11s New Terminal", kb.NewTerminal),
			fmt.Sprintf("  + %-11s Close Window", kb.CloseWindow),
			fmt.Sprintf("  + %-11s Snap Left/Right", kb.SnapLeft+"/"+kb.SnapRight),
			fmt.Sprintf("  + %-11s Max / Restore", kb.Maximize+"/"+kb.Restore),
			fmt.Sprintf("  + %-11s Tile All", kb.TileAll),
			fmt.Sprintf("  + %-11s Expos\u00e9", kb.Expose),
			fmt.Sprintf("  + %-11s Help", kb.Help),
			fmt.Sprintf("  + %-11s Menu Bar", kb.MenuBar),
			fmt.Sprintf("  + %-11s %s to terminal", pfx, pfx),
			"",
			"DOCK (d):",
			"  \u2190/\u2192 h/l       Navigate items",
			"  Enter         Activate / Restore",
			"  Esc           Exit dock",
			"",
			"SESSION:",
			"  F12           Detach (session persists)",
			"",
			"Expos\u00e9:",
			"  Tab / Arrows  Navigate",
			"  Enter         Select window",
			"  Esc           Cancel",
		},
	}
}

// aboutOverlay returns a modal with app info.
func (m *Model) aboutOverlay() *ModalOverlay {
	return &ModalOverlay{
		Title: "About",
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

// minimizeWindow animates a window shrinking to the dock area.
func (m *Model) minimizeWindow(w *window.Window) {
	if w.Minimized {
		return
	}
	// Animate shrink to dock bar center
	dockY := m.height - 1
	dockX := m.width / 2
	m.startWindowAnimation(w.ID, AnimMinimize, w.Rect,
		geometry.Rect{X: dockX, Y: dockY, Width: 1, Height: 1})

	// Unfocus the minimized window and focus next visible one
	m.wm.FocusNextVisible(w.ID)
	m.inputMode = ModeNormal
}

// restoreMinimizedWindow animates a minimized window back to its original rect.
func (m *Model) restoreMinimizedWindow(w *window.Window) {
	if !w.Minimized {
		return
	}
	w.Minimized = false
	m.wm.FocusWindow(w.ID)
	m.inputMode = ModeNormal
	// Animate from dock to original position
	dockY := m.height - 1
	dockX := m.width / 2
	m.startWindowAnimation(w.ID, AnimRestore2,
		geometry.Rect{X: dockX, Y: dockY, Width: 1, Height: 1},
		w.Rect)
}

// updateDockRunning syncs the dock's running indicators and minimized windows.
func (m *Model) updateDockRunning() {
	running := make(map[string]bool)
	for _, w := range m.wm.Windows() {
		if !w.Minimized {
			if w.Command != "" {
				running[w.Command] = true
			} else {
				running["$SHELL"] = true
			}
		}
	}
	m.dock.RunningCommands = running

	// Remove stale minimized items, then add current minimized windows
	var baseItems []dock.DockItem
	for _, item := range m.dock.Items {
		if item.Special != "minimized" {
			baseItems = append(baseItems, item)
		}
	}
	// Insert minimized windows before the Exposé button (last item)
	// Collect minimized windows
	var minimized []*window.Window
	for _, w := range m.wm.Windows() {
		if w.Minimized {
			minimized = append(minimized, w)
		}
	}

	// Build minimized dock items with full names first
	var minItems []dock.DockItem
	for _, w := range minimized {
		icon, _ := minimizedDockLabel(w)
		minItems = append(minItems, dock.DockItem{
			Icon:     icon,
			Label:    w.Title,
			Special:  "minimized",
			WindowID: w.ID,
		})
	}

	// Check if full names fit — estimate total dock width
	totalW := 0
	for _, item := range baseItems {
		totalW += utf8.RuneCountInString(item.Icon) + 1 + utf8.RuneCountInString(item.Label) + 3 // icon+space+label+sep
	}
	for _, item := range minItems {
		totalW += utf8.RuneCountInString(item.Icon) + 1 + utf8.RuneCountInString(item.Label) + 3
	}

	// If too wide, shorten minimized labels progressively
	if totalW > m.width-4 && len(minItems) > 0 {
		// Try truncating to 8 chars
		for i, item := range minItems {
			titleRunes := []rune(item.Label)
			if len(titleRunes) > 8 {
				minItems[i].Label = string(titleRunes[:8])
			}
		}
		// Recalculate
		totalW = 0
		for _, item := range baseItems {
			totalW += utf8.RuneCountInString(item.Icon) + 1 + utf8.RuneCountInString(item.Label) + 3
		}
		for _, item := range minItems {
			totalW += utf8.RuneCountInString(item.Icon) + 1 + utf8.RuneCountInString(item.Label) + 3
		}
		// If still too wide, use short abbreviations
		if totalW > m.width-4 {
			for i := range minItems {
				_, shortLabel := minimizedDockLabel(minimized[i])
				minItems[i].Label = shortLabel
			}
		}
	}

	// Insert minimized items before the Exposé button
	var result []dock.DockItem
	for _, item := range baseItems {
		if item.Special == "expose" {
			result = append(result, minItems...)
		}
		result = append(result, item)
	}
	m.dock.Items = result
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

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion // CellMotion (1002) is more compatible with Termux than AllMotion (1003)
	c := m.theme.C()
	v.BackgroundColor = c.DesktopBg
	v.ForegroundColor = c.DefaultFg

	if !m.ready {
		v.SetContent("Starting termdesk...")
		return v
	}

	// Update dock running indicators from window manager state
	m.updateDockRunning()

	// Check for expose enter/exit animations
	if m.hasExposeAnimations() {
		buf := RenderExposeTransition(m.wm, m.theme, m.animations)
		RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending)
		RenderDock(buf, m.dock, m.theme, nil)
		v.SetContent(buf) // Direct cell transfer — no ANSI round-trip
		return v
	}

	if m.exposeMode {
		buf := RenderExpose(m.wm, m.theme)
		RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending)
		RenderDock(buf, m.dock, m.theme, nil)
		v.SetContent(buf)
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
	// Cursor blinks in Terminal mode, always visible otherwise
	showCursor := m.inputMode != ModeTerminal || m.cursorVisible
	// Only pass scrollOffset when in Copy mode
	scrollOff := 0
	if m.inputMode == ModeCopy {
		scrollOff = m.scrollOffset
	}
	buf := RenderFrame(m.wm, m.theme, m.terminals, animRects, showCursor, scrollOff)
	RenderMenuBar(buf, m.menuBar, m.theme, m.inputMode, m.prefixPending)
	RenderDock(buf, m.dock, m.theme, m.animations)
	if m.launcher.Visible {
		RenderLauncher(buf, m.launcher, m.theme)
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
	v.SetContent(buf) // Direct cell transfer — no ANSI round-trip
	return v
}
