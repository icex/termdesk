# Changelog

All notable changes to this project will be documented in this file.

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
