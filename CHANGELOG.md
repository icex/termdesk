# Changelog

All notable changes to this project will be documented in this file.

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
- Minimum window size enforcement during drag (12×4)
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
- Unicode box-drawing borders with title bar, close [X] and maximize [□] buttons
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
