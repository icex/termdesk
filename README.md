# Termdesk

A retro terminal desktop environment inspired by Windows 1.0, DESQview, and Mac System 1.0 — built with modern Go and the Charmbracelet TUI stack.

## Features (Planned)

- Overlapping, draggable, resizable windows with tiling support
- Menu bar with dropdowns, clock, and system indicators
- Dock with configurable app shortcuts (Nerd Font icons)
- PTY terminal windows running real shells and apps
- Keyboard-driven command palette / app launcher
- Clipboard with history (5-item ring buffer)
- Desk accessories: calculator, notepad, clock
- Built-in contextual help system
- TOML configuration with live reload
- Session persistence across restarts
- Integration with dotfiles (nvim, superfile, zsh, tmux)

## Requirements

- Go 1.23+
- A terminal emulator with mouse support
- Nerd Font installed (for icons)

## Quick Start

```bash
make build
./bin/termdesk
```

## Development

```bash
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run linter
make run            # Build and run
```

## Keybindings

| Key | Action |
|-----|--------|
| `Ctrl+N` | New terminal window |
| `Alt+Tab` | Cycle windows forward |
| `Alt+Shift+Tab` | Cycle windows backward |
| `Ctrl+Left` | Snap window left |
| `Ctrl+Right` | Snap window right |
| `Ctrl+Up` | Maximize window |
| `Ctrl+W` | Close window |
| `Ctrl+Space` | Open launcher |
| `Ctrl+C` or `q` | Quit (when no window focused) |

## License

MIT
