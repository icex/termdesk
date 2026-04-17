package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/clipboard"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/tour"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

const version = "0.1.0"

// programRef holds a shared reference to the tea.Program.
// Using a pointer so copies of Model (value receivers) share the same ref.
type programRef struct {
	p *tea.Program
}

// renderCache caches the last View() result. Stored as a pointer on Model so
// it survives BT v2's value-receiver copies (both copies share the same cache).
type renderCache struct {
	updateGen uint64   // incremented each Update()
	viewGen   uint64   // set to updateGen when View() renders
	view      tea.View // cached result
}

// backgroundWorkspace holds a stashed workspace's window manager and terminals
// so processes stay alive when switching between workspaces.
type backgroundWorkspace struct {
	wm        *window.Manager
	terminals map[string]*terminal.Terminal
}

// Model is the root model for the termdesk application.
type Model struct {
	width                   int
	height                  int
	ready                   bool
	wm                      *window.Manager
	theme                   config.Theme
	drag                    window.DragState
	terminals               map[string]*terminal.Terminal
	menuBar                 *menubar.MenuBar
	dock                    *dock.Dock
	launcher                *launcher.Launcher
	clipboard               *clipboard.Clipboard
	notifications           *notification.Manager
	settings                *settings.Panel
	progRef                 *programRef // shared program reference for goroutine messaging
	exposeMode              bool        // exposé overview mode
	exposeFilter            string      // exposé search filter (case-insensitive title match)
	inputMode               InputMode   // current input mode (Normal/Terminal/Copy)
	dockFocused             bool        // true when dock has keyboard focus in Normal mode
	menuBarFocused          bool        // true when menu bar has keyboard focus via Tab cycling
	menuBarFocusIdx         int         // focused menu label index during Tab cycling
	tabCycleCount           int         // counts consecutive Tab presses in one direction
	tabCycleDir             int         // +1 for Tab, -1 for Shift+Tab; reset on direction change
	contextMenu             *contextmenu.Menu
	confirmClose            *ConfirmDialog
	renameDialog            *RenameDialog
	bufferNameDialog        *BufferNameDialog
	newWorkspaceDialog      *NewWorkspaceDialog
	registry                []registry.RegistryEntry
	widgetBar               *widget.Bar
	widgetRegistry          *widget.Registry
	customWidgets           map[string]*widget.ShellWidget
	enabledWidgets          []string
	barUsername              string // preserved for widget bar rebuilds
	barDisplayName          string // preserved for widget bar rebuilds
	exePath                 string
	animations              []Animation   // active animations
	modal                   *ModalOverlay // help, about, or other modal
	keybindings             config.KeyBindings
	actionMap               map[string]string             // key string → action name (reverse lookup)
	prefixPending           bool                          // true when prefix key pressed, waiting for action
	// Cursor is rendered natively via tea.Cursor — no manual blink tracking needed.
	scrollOffset            int                           // scrollback offset in Copy mode (0 = live)
	copyCursorX             int                           // copy mode cursor column (always visible in copy mode)
	copyCursorY             int                           // copy mode cursor absolute line (0=oldest scrollback)
	copyCount               int                           // numeric prefix for copy mode actions
	copyLastKey             string                        // last key in copy mode (for sequences like gg)
	copySearchActive        bool                          // copy mode search prompt active
	copySearchQuery         string                        // current copy mode search query
	copySearchDir           int                           // search direction: 1 forward, -1 backward
	copySearchMatchCount    int                           // total search matches in buffer
	copySearchMatchIdx      int                           // current match index (0-based)
	copySnapshot            *CopySnapshot                 // frozen terminal snapshot for copy mode
	selActive               bool                          // selection in progress
	selDragging             bool                          // mouse drag selection in progress
	selStart                geometry.Point                // selection start (X=col, Y=absLine)
	selEnd                  geometry.Point                // selection end (X=col, Y=absLine)
	cache                   *renderCache                  // shared view cache (survives value-receiver copies)
	animationsOn            bool                          // animations enabled (persisted in config)
	animationSpeed          string                        // animation speed: "slow", "normal", "fast"
	animationStyle          string                        // animation style: "smooth", "snappy", "bouncy"
	springs                 *springCache                  // cached spring presets (rebuilt on speed/style change)
	showDeskClock           bool                          // desktop clock enabled (persisted in config)
	showKeys                bool                          // show key press overlay (persisted in config)
	tilingMode              bool                          // tiling mode enabled (persisted in config)
	tilingLayout            string                        // persistent tiling layout: "columns", "rows", "all"
	hideDockWhenMaximized   bool                          // hide dock when a window is maximized
	hideDockApps            bool                          // hide static app shortcuts in dock
	showResizeIndicator     bool                          // show resize dimensions overlay (debug)
	defaultTerminalMode     bool                          // start in Terminal mode on new window focus
	tour                    *tour.Tour                    // first-run guided tour
	tooltipText             string                        // hover tooltip text (empty = hidden)
	tooltipX                int                           // tooltip screen X
	tooltipY                int                           // tooltip screen Y
	hoverX                  int                           // last mouse X for hover tracking
	hoverY                  int                           // last mouse Y for hover tracking
	hoverTime               time.Time                     // when mouse stopped moving
	hoverButtonZone         window.HitZone                // which title bar button the mouse is over
	hoverButtonWindowID     string                        // which window the hovered button belongs to
	hoverMenuLabel          int                           // which menu bar label is hovered (-1=none)
	hoverWidgetName         string                        // which menubar widget is hovered (empty=none)
	workspaceAutoSave       bool                          // whether auto-save is enabled
	workspaceAutoSaveMin    int                           // auto-save interval in minutes
	lastWorkspaceSave       time.Time                     // last save timestamp
	projectConfig           *config.ProjectConfig         // project-specific config
	autoStartTriggered      bool                          // whether autostart has run
	workspaceRestorePending bool                          // whether workspace needs to be restored
	workspacePickerVisible  bool                          // whether workspace picker is shown
	workspacePickerSelected int                           // selected index in workspace picker
	workspaceList           []string                      // discovered workspace files
	workspaceWindowCounts   []int                         // window counts per workspace (for picker display)
	workspaceWidget         *widget.WorkspaceWidget       // reference to workspace widget in bar
	windowBuffers           map[string]string             // window ID → last buffer content (plain text)
	windowCache             map[string]*windowRenderCache // window ID → cached window buffer
	backgroundWorkspaces    map[string]*backgroundWorkspace // workspace path → stashed WM + terminals
	activeWorkspacePath     string                        // currently active workspace path (for stashing)
	showKeysEvents          []showKeyEvent                // recent key events for show-keys overlay
	minimizedTileSlots      map[string]int                // window ID -> prior tile slot before minimize
	termCreatedAt           map[string]time.Time          // window ID → terminal creation time (for stuck detection)
	termHasOutput           map[string]bool               // window ID → true once first PtyOutputMsg arrives
	paneRedirect            map[string]string             // old pane ID → window ID (after split-to-single revert)
	lastWindowSizeAt        time.Time                     // timestamp of latest WindowSizeMsg
	focusFollowsMouse       bool                          // auto-focus window under mouse cursor
	syncPanes               bool                          // broadcast input to all visible terminals
	quakeTerminal           *terminal.Terminal             // standalone quake dropdown terminal (not a window)
	quakeVisible            bool                          // whether quake terminal is currently shown
	quakeAnimH              float64                       // current animated height (0 = hidden)
	quakeAnimVel            float64                       // animation velocity for spring physics
	quakeTargetH            float64                       // target height (0 or full quake height)
	quakeHeightPct          int                           // quake height as % of screen (default 40, configurable)
	quakeDragActive         bool                          // true while dragging quake bottom border
	quakeDragStartY         int                           // mouse Y at drag start
	quakeDragStartH         int                           // quake height at drag start
	tileSpawnPreset         string                        // next tiled-window placement hint: auto,left,right,up,down
	cellPixelW              int                           // cell width in pixels (from real terminal)
	cellPixelH              int                           // cell height in pixels (from real terminal)
	kittyPass               *terminal.KittyPassthrough    // Kitty graphics passthrough (shared across windows)
	kittyPending            *kittyPendingBuf              // pending Kitty output flushed via tea.Raw() (shared pointer)
	imagePass               *terminal.ImagePassthrough    // Sixel/iTerm2 image passthrough (shared across windows)
	imagePending            *imagePendingBuf              // pending image output flushed via tea.Raw() (shared pointer)
	imageState              *imageSuppressionState         // shared pointer for image hide/show state
	perf                    *perfTracker                   // render performance tracker (TERMDESK_PERF=1)
	wallpaperMode           string                         // "theme", "color", "pattern", "program"
	wallpaperColor          string                         // hex color for solid color mode
	wallpaperPattern        string                         // pattern char(s) for pattern mode
	wallpaperPatternFg      string                         // pattern foreground hex
	wallpaperPatternBg      string                         // pattern background hex
	wallpaperProgram        string                         // command for program wallpaper
	wallpaperTerminal       *terminal.Terminal             // headless terminal for program wallpaper
	wallpaperConfig         *WallpaperConfig               // pre-computed wallpaper config for rendering
}

// kittyPendingBuf accumulates Kitty graphics output during Update().
// Flushed via tea.Raw() in the Update() wrapper — routes through BT's
// synchronized output pipeline (p.outputBuf → p.flush() → p.output),
// serialized with frame rendering. Shared pointer — survives BT v2 copies.
type kittyPendingBuf struct {
	data []byte
}

// imagePendingBuf accumulates sixel/iTerm2 image output during Update().
// Same flush pattern as kittyPendingBuf.
type imagePendingBuf struct {
	data []byte
}

// imageSuppressionState tracks whether images are currently hidden (during
// drag, animation, scroll) and prevents flooding by deduplicating refresh
// requests. Shared pointer — survives BT v2 value copies.
type imageSuppressionState struct {
	hidden         bool // images are currently hidden (drag/animation/scroll)
	refreshPending bool // an ImageRefreshMsg is already queued
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
	Selected int // 0 = OK, 1 = Cancel
}

// BufferNameDialog captures a name for the selected clipboard buffer.
type BufferNameDialog struct {
	Text   []rune
	Cursor int
}

// NewWorkspaceDialog represents a dialog for creating a new workspace.
type NewWorkspaceDialog struct {
	Name       []rune   // workspace name text input
	TextCursor int      // cursor position in name field
	DirPath    string   // current browsed directory (absolute)
	DirEntries []string // subdirectory names (sorted, not including "..")
	DirScroll  int      // scroll offset for directory list
	DirSelect  int      // selected index (0 = "..", 1+ = DirEntries[i-1])
	Cursor     int      // section: 0=name, 1=browser, 2=buttons
	Selected   int      // button: 0=Create, 1=Cancel
}

// HelpTab represents a single tab in a tabbed help overlay.
type HelpTab struct {
	Title string
	Lines []string
}

// ModalOverlay represents a modal text overlay (help, about, etc.)
type ModalOverlay struct {
	Title     string
	Lines     []string  // simple modals (about, etc.)
	Tabs      []HelpTab // tabbed help (nil for simple modals)
	ActiveTab int
	HoverTab  int // hovered tab index (-1 = none)
	ScrollY   int
}

// TabLabel returns the display label for tab at index i (e.g. " General [1]").
func (mo *ModalOverlay) TabLabel(i int) string {
	if i < 0 || i >= len(mo.Tabs) {
		return ""
	}
	return fmt.Sprintf("%s [%d]", mo.Tabs[i].Title, i+1)
}

// ModalBounds holds computed modal layout dimensions for click/hover testing.
type ModalBounds struct {
	StartX, StartY int
	BoxW, BoxH     int
	HPad           int
	TabRow         int // row (screen Y) where tab bar is rendered (-1 if no tabs)
	InnerW         int
}

// Bounds computes the on-screen bounds and layout info for the modal.
// Must match the layout logic in RenderModal().
func (mo *ModalOverlay) Bounds(screenW, screenH int) ModalBounds {
	hasTabBar := mo.Tabs != nil && len(mo.Tabs) > 0

	// Calculate stable dimensions (same as RenderModal)
	maxLineW := runeLen(mo.Title)
	if hasTabBar {
		tabBarW := 0
		for i := range mo.Tabs {
			tabBarW += runeLen(mo.TabLabel(i)) + 2
		}
		if tabBarW > maxLineW {
			maxLineW = tabBarW
		}
		for _, tab := range mo.Tabs {
			for _, line := range tab.Lines {
				if w := runeLen(line); w > maxLineW {
					maxLineW = w
				}
			}
		}
	} else {
		for _, line := range mo.Lines {
			if w := runeLen(line); w > maxLineW {
				maxLineW = w
			}
		}
	}

	hPad := 3
	innerW := maxLineW + 2*hPad
	if innerW > screenW-6 {
		innerW = screenW - 6
	}

	// Compute visible lines for height
	lines := mo.Lines
	if hasTabBar {
		if mo.ActiveTab < len(mo.Tabs) {
			lines = mo.Tabs[mo.ActiveTab].Lines
		}
	}
	maxTabLines := len(lines)
	if hasTabBar {
		for _, tab := range mo.Tabs {
			if len(tab.Lines) > maxTabLines {
				maxTabLines = len(tab.Lines)
			}
		}
	}
	visibleLines := screenH - 14
	if visibleLines < 3 {
		visibleLines = 3
	}
	if visibleLines > maxTabLines {
		visibleLines = maxTabLines
	}

	// Count inner rows: spacer + title + sep [+ spacer + tabBar] + spacer + content + spacer + footerSep + footer + spacer
	innerRows := 3 + visibleLines + 4
	if hasTabBar {
		innerRows += 2 // spacer + tabBar
	}

	boxW := innerW + 2
	boxH := innerRows + 2
	startX := (screenW - boxW) / 2
	startY := (screenH - boxH) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	tabRow := -1
	if hasTabBar {
		tabRow = startY + 5
	}

	return ModalBounds{
		StartX: startX, StartY: startY,
		BoxW: boxW, BoxH: boxH,
		HPad: hPad, TabRow: tabRow,
		InnerW: innerW,
	}
}

// TabAtX returns which tab index is at relative X position within the tab bar,
// or -1 if not on a tab. relX is relative to the left padding start.
func (mo *ModalOverlay) TabAtX(relX int) int {
	if mo.Tabs == nil {
		return -1
	}
	x := 0
	for i := range mo.Tabs {
		tabW := runeLen(mo.TabLabel(i)) + 2
		if relX >= x && relX < x+tabW {
			return i
		}
		x += tabW
	}
	return -1
}

type showKeyEvent struct {
	Key    string
	Action string
	At     time.Time
}

const showKeysMaxEvents = 8

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
		m.enterCopyModeForFocusedWindow()
	case ModeCopy:
		m.copySnapshot = nil
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
	am[kb.MenuEdit] = "menu_edit"
	am[kb.MenuApps] = "menu_apps"
	am[kb.MenuView] = "menu_view"
	am[kb.ClipboardHistory] = "clipboard_history"
	am[kb.NotificationCenter] = "notification_center"
	am[kb.Settings] = "settings"
	am[kb.MoveLeft] = "move_left"
	am[kb.MoveRight] = "move_right"
	am[kb.MoveUp] = "move_up"
	am[kb.MoveDown] = "move_down"
	am[kb.GrowWidth] = "grow_width"
	am[kb.ShrinkWidth] = "shrink_width"
	am[kb.GrowHeight] = "grow_height"
	am[kb.ShrinkHeight] = "shrink_height"
	am[kb.SnapTop] = "snap_top"
	am[kb.SnapBottom] = "snap_bottom"
	am[kb.Center] = "center"
	am[kb.TileColumns] = "tile_columns"
	am[kb.TileRows] = "tile_rows"
	am[kb.Cascade] = "cascade"
	if kb.TileMaximized != "" {
		am[kb.TileMaximized] = "tile_maximized"
	}
	if kb.ShowDesktop != "" {
		am[kb.ShowDesktop] = "show_desktop"
	}
	if kb.SaveWorkspace != "" {
		am[kb.SaveWorkspace] = "save_workspace"
	}
	if kb.LoadWorkspace != "" {
		am[kb.LoadWorkspace] = "load_workspace"
	}
	if kb.NewWorkspace != "" {
		am[kb.NewWorkspace] = "new_workspace"
	}
	if kb.ProjectPicker != "" {
		am[kb.ProjectPicker] = "project_picker"
	}
	if kb.ShowKeys != "" {
		am[kb.ShowKeys] = "show_keys"
	}
	if kb.ToggleTiling != "" {
		am[kb.ToggleTiling] = "toggle_tiling"
	}
	if kb.SwapLeft != "" {
		am[kb.SwapLeft] = "swap_left"
	}
	if kb.SwapRight != "" {
		am[kb.SwapRight] = "swap_right"
	}
	if kb.SwapUp != "" {
		am[kb.SwapUp] = "swap_up"
	}
	if kb.SwapDown != "" {
		am[kb.SwapDown] = "swap_down"
	}
	if kb.SyncPanes != "" {
		am[kb.SyncPanes] = "sync_panes"
	}
	if kb.QuakeTerminal != "" {
		am[kb.QuakeTerminal] = "quake_terminal"
	}

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

func (m *Model) recordShowKey(key, action string) {
	if !m.showKeys || key == "" {
		return
	}
	evt := showKeyEvent{Key: key, Action: action, At: time.Now()}
	m.showKeysEvents = append(m.showKeysEvents, evt)
	if len(m.showKeysEvents) > showKeysMaxEvents {
		m.showKeysEvents = m.showKeysEvents[len(m.showKeysEvents)-showKeysMaxEvents:]
	}
}

// New creates a new root Model.
