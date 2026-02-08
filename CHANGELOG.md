# Changelog

All notable changes to this project will be documented in this file.

## [0.9.0] - 2026-02-08

### Added
- **Expose mode**: focused window centered at 70% work area, unfocused as thumbnails in bottom strip
- Animated expose cycling with easeInOutCubic easing (350ms transitions)
- Expose titles: focused uses big half-block ASCII font, unfocused uses bold text
- **4 themes**: Retro, Modern, Tokyo Night, Catppuccin Mocha — switchable via View menu
- Theme selector in View menu dropdown
- **Window snap/tile buttons** in title bar: [snap-left][snap-right][maximize][close]
- **Minimize button** [_] in title bar with dock integration
- **Unfocused window content fading** (configurable fade amount per theme)
- **Confirm dialogs** with styled buttons (green Yes / red No) — clickable with mouse
- **Bell/notification highlighting** on unfocused terminal windows (yellow border)
- **Live CPU/MEM indicators** in menu bar (reads /proc/stat and /proc/meminfo)
- **Interactive menu bar**: click clock for date popup, click CPU/MEM to launch btop
- **Help overlay** (F1) with keybinding reference
- **About dialog** (Help > About) with version info
- **Input mode system**: Normal/Terminal/Copy with colored badge in menu bar
- **Configurable title bar height** via TOML config
- Dock icons-only mode with running indicators and launch pulse animation
- Dock shows minimized windows with progressive name shortening
- Pin to Dock via File menu
- Window command tracking (Command field on Window struct)
- Big text font (4x5 pixel, half-block rendered) and mini font (3x4 pixel) for expose
- Content background color separate from border background for visual depth

### Fixed
- **Termux background bleed-through**: NewBuffer now sets explicit Fg and Bg on all cells; BufferToString uses targeted SGR sequences instead of full `\x1b[0m` resets to prevent terminal default colors from showing through
- Terminal default foreground/background: renderTerminalContent uses explicit fallback colors for nil VT cells
- Menu dropdown X position offset (was +3, now +2 matching rendered width)
- Calc dock item changed from nonexistent `calc` to `bc`

### Changed
- PTY architecture: background goroutines per terminal with non-blocking buffered writes (replaces blocking Cmd chain)
- Mouse forwarding: SGR encoding with mouse mode detection (prevents "weird text" in non-mouse apps)
- Animation system: expose uses easeInOutCubic (350ms) for smoother transitions; other animations use easeOutCubic (200ms)
- BufferToString: removed per-line ANSI resets, uses combined SGR sequences for attr+color changes

## [0.8.1] - 2026-02-07

### Fixed
- Terminal responsiveness: replaced blocking PTY read loop with event-driven ReadOnce chain
- Exit command now closes window: PtyClosedMsg handler removes the window
- Buffer height includes dock row (was missing 1 row)
- Optimized ANSI color output with direct string building (no fmt.Sprintf per cell)

## [0.8.0] - 2026-02-07

### Added
- App launcher / command palette overlay (Ctrl+Space to toggle)
- Fuzzy search with prefix bonus scoring across all registered apps
- Keyboard navigation: Up/Down to move selection, Enter to launch, Escape to dismiss
- $PATH scanning for known tools (nvim, spf, claude, tetrigo, mc, htop, btop)
- Default app registry: Terminal, nvim, Files (spf), Calculator
- Launcher renders as centered modal overlay with box-drawing borders
- Click outside launcher to dismiss
- Dock items now launch specific commands (nvim, spf, calc) instead of always default shell
- openTerminalWindowWith() supports launching arbitrary commands in terminal windows

## [0.7.0] - 2026-02-07

### Added
- Bottom dock bar with Nerd Font icons: Terminal, nvim, Files, Calc
- Dock items centered with separators
- Mouse hover highlighting on dock items (bracket indicator)
- Click dock items to launch terminal windows
- Reserved bottom row for dock (windows can't overlap)

## [0.6.0] - 2026-02-07

### Added
- Menu bar at top of screen with File, Edit, View, Help menus
- Dropdown menus with keyboard navigation (arrows, Enter, Escape)
- F10 to toggle menu bar
- Mouse click on menu labels opens/closes dropdowns
- Clock display (HH:MM AM/PM) and CPU/MEM placeholders in menu bar
- Menu actions: New Terminal, Quit, Tile All, Snap Left/Right
- Reserved space system: windows can't overlap menu bar
- FormatCPU, FormatMemory helpers for system indicators

## [0.5.0] - 2026-02-07

### Added
- PTY terminal windows: Ctrl+N opens a real shell (zsh/sh) inside a window
- VT100/ANSI terminal emulation via charmbracelet/x/vt SafeEmulator
- Full keyboard input forwarding to PTY (printable chars, Ctrl combos, arrows, function keys)
- Terminal resize propagation when windows are snapped, maximized, tiled, or dragged
- ANSI escape sequence stripping for terminal content rendering
- Terminal cleanup on window close and application quit
- PTY read loop with async message passing

## [0.4.0] - 2026-02-07

### Added
- Mouse support: click to focus windows, drag title bar to move, drag borders/corners to resize
- Hit testing for window zones: title bar, close/maximize buttons, borders, corners, content
- Drag state machine with 9 resize modes (N/S/E/W/NE/NW/SE/SW) and move
- Minimum window size enforcement during drag (12x4)
- Close button click removes window, maximize button toggles maximize/restore
- Dragging a maximized window auto-restores it
- Keyboard window management: Ctrl+Left/Right snap, Ctrl+Up maximize, Ctrl+Down restore
- Alt+Tab / Alt+Shift+Tab to cycle between windows
- Ctrl+W to close focused window, Ctrl+T to tile all windows
- Window tiling: SnapLeft, SnapRight, Maximize, Restore, ToggleMaximize, TileAll (grid layout)
- Stale drag cleanup when window removed mid-drag

## [0.3.0] - 2026-02-07

### Added
- Window rendering with painter's algorithm compositor
- Retro theme (Win 1.0 inspired: white-on-blue) and Modern theme (OneDark)
- Unicode box-drawing borders with title bar, close [X] and maximize [box] buttons
- Active/inactive window color differentiation
- Cell buffer with SetString, FillRect operations (proper Unicode rune handling)
- Ctrl+N to open demo windows with cascading positions
- Theme system with GetTheme() name lookup

## [0.2.0] - 2026-02-07

### Added
- Geometry primitives: Rect (Contains, Intersect, Union, Move, Resize, Clamp, Overlaps) and Point (Add, Sub, In)
- Window data structure with title bar, content rect, button positions, maximize state
- Window Manager with z-order stack, focus management (bring-to-front), hit testing, cycle forward/backward

## [0.1.0] - 2026-02-07

### Added
- Project scaffolding with Go module, Bubble Tea v2 RC, Makefile
- Root application model with alt-screen and quit handling
- Default TOML configuration skeleton
- Initial test suite
