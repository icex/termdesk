package app

import (
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/clipboard"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/settings"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/tour"
	"github.com/icex/termdesk/internal/widget"
	"github.com/icex/termdesk/internal/window"
)

func New() Model {
	dbg("New() called")
	userCfg := config.LoadUserConfig()
	theme := config.GetTheme(userCfg.Theme)

	// Load project config if present
	projectCfg, _ := config.FindProjectConfig("")

	// Resolve our own executable path for manifest discovery
	exePath, _ := os.Executable()
	appRegistry := registry.BuildRegistry(exePath)
	dockEntries := registry.DockEntries(appRegistry)

	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	displayName := username
	hostname, _ := os.Hostname()
	if hostname != "" {
		displayName = username + "@" + hostname
	}

	// Build widget registry and bar
	widgetReg := widget.DefaultRegistry()
	customWidgets := make(map[string]*widget.ShellWidget)
	for _, cw := range userCfg.CustomWidgets {
		if cw.Name == "" || cw.Command == "" {
			continue
		}
		widgetReg.Register(widget.WidgetMeta{Name: cw.Name, Label: cw.Label, Builtin: false})
		customWidgets[cw.Name] = &widget.ShellWidget{
			WidgetName: cw.Name,
			Label:      cw.Label,
			Icon:       cw.Icon,
			Command:    cw.Command,
			Interval:   cw.Interval,
			OnClick:    cw.OnClick,
		}
	}
	enabledWidgets := userCfg.EnabledWidgets
	if len(enabledWidgets) == 0 {
		enabledWidgets = widget.DefaultEnabledWidgets()
	}
	wb := widget.NewBar(widgetReg, enabledWidgets, username, displayName, customWidgets)
	// Find workspace widget reference for dynamic updates
	var wsWidget *widget.WorkspaceWidget
	for _, w := range wb.Widgets {
		if ww, ok := w.(*widget.WorkspaceWidget); ok {
			wsWidget = ww
			break
		}
	}
	mb := menubar.New(80, userCfg.Keys, appRegistry, wb)

	// Add project-specific dock shortcuts from .termdesk.toml
	if projectCfg != nil {
		for _, de := range projectCfg.DockItems {
			entry := registry.RegistryEntry{
				Name:      de.Name,
				Icon:      de.Icon,
				IconColor: de.IconColor,
				Command:   de.Command,
				Args:      de.Args,
			}
			if de.Position > 0 && de.Position <= len(dockEntries) {
				// Insert at specified position (1-based among app items)
				idx := de.Position - 1
				dockEntries = append(dockEntries, registry.RegistryEntry{})
				copy(dockEntries[idx+1:], dockEntries[idx:])
				dockEntries[idx] = entry
			} else {
				dockEntries = append(dockEntries, entry)
			}
		}
	}

	d := dock.New(dockEntries, 80)
	d.IconsOnly = userCfg.IconsOnly
	d.MinimizeToDock = userCfg.MinimizeToDock
	// Query real terminal cell pixel size for Kitty graphics protocol support.
	// Apps use TIOCGWINSZ pixel dimensions and CSI 14t/16t queries to detect
	// terminal pixel size. Without this, image viewers fall back to pixelated ANSI.
	cellPxW, cellPxH := queryCellPixelSize()

	am := BuildActionMap(userCfg.Keys)
	m := Model{
		wm:                    window.NewManager(80, 24),
		theme:                 theme,
		terminals:             make(map[string]*terminal.Terminal),
		menuBar:               mb,
		dock:                  d,
		launcher:              launcher.New(appRegistry),
		clipboard:             clipboard.New(),
		notifications:         notification.New(),
		settings:              settings.New(userCfg, widgetReg.All(), enabledWidgets),
		progRef:               &programRef{},
		hoverMenuLabel:        -1,
		keybindings:           userCfg.Keys,
		actionMap:             am,
		registry:              appRegistry,
		widgetBar:             wb,
		widgetRegistry:        widgetReg,
		customWidgets:         customWidgets,
		enabledWidgets:        enabledWidgets,
		barUsername:            username,
		barDisplayName:         displayName,
		workspaceWidget:       wsWidget,
		exePath:               exePath,
		cache:                 &renderCache{updateGen: 1},
		animationsOn:          userCfg.Animations,
		animationSpeed:        userCfg.AnimationSpeed,
		animationStyle:        userCfg.AnimationStyle,
		springs:               newSpringCache(userCfg.AnimationSpeed, userCfg.AnimationStyle),
		showDeskClock:         userCfg.ShowDeskClock,
		showKeys:              userCfg.ShowKeys,
		tilingMode:            userCfg.TilingMode,
		tilingLayout:          normalizeTilingLayout(userCfg.TilingLayout),
		hideDockWhenMaximized: userCfg.HideDockWhenMaximized,
		hideDockApps:          userCfg.HideDockApps,
		focusFollowsMouse:     userCfg.FocusFollowsMouse,
		showResizeIndicator:   userCfg.ShowResizeIndicator,
		defaultTerminalMode:   userCfg.DefaultTerminalMode,
		tour:                  tour.New(userCfg.TourCompleted),
		workspaceAutoSave:     userCfg.WorkspaceAutoSave,
		workspaceAutoSaveMin:  userCfg.WorkspaceAutoSaveMin,
		lastWorkspaceSave:     time.Now(),
		projectConfig:         projectCfg,
		autoStartTriggered:    false,
		windowBuffers:         make(map[string]string),
		windowCache:           make(map[string]*windowRenderCache),
		backgroundWorkspaces:  make(map[string]*backgroundWorkspace),
		minimizedTileSlots:    make(map[string]int),
		termCreatedAt:         make(map[string]time.Time),
		termHasOutput:         make(map[string]bool),
		paneRedirect:          make(map[string]string),
		quakeHeightPct:        quakeHeightPctOrDefault(userCfg.QuakeHeightPercent),
		tileSpawnPreset:       "auto",
		cellPixelW:            cellPxW,
		cellPixelH:            cellPxH,
		kittyPass:             terminal.NewKittyPassthrough(cellPxW, cellPxH),
		kittyPending:          &kittyPendingBuf{},
		imagePass:             terminal.NewImagePassthrough(cellPxW, cellPxH),
		imagePending:          &imagePendingBuf{},
		imageState:            &imageSuppressionState{},
		perf:                  newPerfTracker(),
		wallpaperMode:         userCfg.WallpaperMode,
		wallpaperColor:        userCfg.WallpaperColor,
		wallpaperPattern:      userCfg.WallpaperPattern,
		wallpaperPatternFg:    userCfg.WallpaperPatternFg,
		wallpaperPatternBg:    userCfg.WallpaperPatternBg,
		wallpaperProgram:      userCfg.WallpaperProgram,
	}
	// Load recent apps and favorites into launcher
	m.launcher.RecentApps = userCfg.RecentApps
	for _, cmd := range userCfg.Favorites {
		m.launcher.Favorites[cmd] = true
	}

	// Set workspace widget name from project config
	if m.workspaceWidget != nil && projectCfg != nil && projectCfg.ProjectDir != "" {
		m.workspaceWidget.DisplayName = filepath.Base(projectCfg.ProjectDir)
	}

	// Mark that workspace should be restored after program is set
	m.workspaceRestorePending = true
	m.syncTilingMenuLabel()

	// Build wallpaper config from settings
	m.wallpaperConfig = m.buildWallpaperConfig()

	return m
}

// SetProgram sets the tea.Program reference for background goroutine messaging.
// Must be called before Run(). The reference is shared across Model copies
// (Bubble Tea uses value receivers, so Model gets copied).
func (m *Model) SetProgram(p *tea.Program) {
	m.progRef.p = p
}

// queryCellPixelSize returns the real terminal's cell pixel dimensions.
// First checks TERMDESK_CELL_PX env var (propagated through session system),
// then queries TIOCGWINSZ on stdout. Returns (10, 20) as fallback.
func queryCellPixelSize() (int, int) {
	// Session system propagation: client queried the real terminal and set this.
	if px := os.Getenv("TERMDESK_CELL_PX"); px != "" {
		// Parse "WxH" format
		for i := 0; i < len(px); i++ {
			if px[i] == 'x' {
				w, h := 0, 0
				for _, c := range px[:i] {
					if c < '0' || c > '9' {
						goto directQuery
					}
					w = w*10 + int(c-'0')
				}
				for _, c := range px[i+1:] {
					if c < '0' || c > '9' {
						goto directQuery
					}
					h = h*10 + int(c-'0')
				}
				if w > 0 && h > 0 {
					return w, h
				}
			}
		}
	}
directQuery:
	ws, err := pty.GetsizeFull(os.Stdout)
	if err == nil && ws.X > 0 && ws.Y > 0 && ws.Cols > 0 && ws.Rows > 0 {
		return int(ws.X) / int(ws.Cols), int(ws.Y) / int(ws.Rows)
	}
	return 10, 20 // common default cell size
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.tickSystemStats(), tickWorkspaceAutoSave(), tickCleanup()}
	// Trigger workspace restoration after program is set up
	if m.workspaceRestorePending {
		cmds = append(cmds, func() tea.Msg { return WorkspaceRestoreMsg{} })
	}
	return tea.Batch(cmds...)
}
