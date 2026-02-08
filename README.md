# Termdesk

A retro terminal desktop environment inspired by Windows 1.0, DESQview, and Mac System 1.0 — built with Go and the Charmbracelet TUI stack (Bubble Tea v2).

## Features

- **Session attach/detach** — tmux-like persistent sessions (detach with `F12`, reattach with `termdesk`)
- **Overlapping windows** with drag, resize, maximize, snap (left/right), tile, and minimize
- **PTY terminal windows** running real shells and TUI apps (nvim, htop, mc, etc.)
- **VT100/ANSI terminal emulation** via charmbracelet/x/vt with full keyboard and mouse forwarding
- **Menu bar** with dropdowns, live CPU/MEM/battery indicators, clock (click for date popup, click CPU/MEM for btop), logged-in username
- **Dock** with colorful Nerd Font icons, running indicators, minimized window tabs, launch pulse animation
- **Command palette** (Ctrl+Space) with fuzzy search across installed apps
- **4 themes**: Retro (Win 1.0 blue), Modern (OneDark), Tokyo Night, Catppuccin Mocha — switchable via View menu
- **Expose mode** for window overview with animated cycling (focused window centered, thumbnails in strip)
- **Window animations** for open, close, maximize, restore, snap, tile, minimize, and expose transitions
- **Input modes**: Normal (WM keys), Terminal (prefix-gated, like tmux), Copy (vim-style scrollback)
- **Configurable keybindings** via TOML config with tmux-style prefix key (`Ctrl+G`)
- **Confirm dialogs** with clickable styled buttons (green Yes / red No)
- **Bell/notification highlighting** on unfocused terminal windows
- **Help overlay** (F1) and About dialog
- **Unfocused window content fading** for visual depth
- **Configurable title bar height** via TOML config
- **TOML configuration** for themes, dock icons-only mode, and settings
- **Termux (Android) compatible** — tested with mouse, background colors, and battery widget

## Requirements

- Go 1.23+
- A terminal emulator with truecolor and mouse support
- [Nerd Font](https://www.nerdfonts.com/) (for dock and UI icons)
- Recommended apps: `htop`, `mc`, `nvim`, `python3` (used by dock/launcher)

## Install

The install script handles everything: checks Go, installs recommended apps, builds, and installs the binary.

```bash
git clone https://github.com/icex/termdesk.git
cd termdesk
./install.sh
```

### Linux

```bash
# Install dependencies first (Debian/Ubuntu)
sudo apt install golang neovim htop mc python3

# Or on Arch
sudo pacman -S go neovim htop mc python3

# Then build and install
./install.sh
```

### macOS

```bash
# Install with Homebrew
brew install go neovim htop mc python3
brew install --cask font-jetbrains-mono-nerd-font

# Then build and install
./install.sh
```

### Android (Termux)

```bash
# Install dependencies
pkg install golang neovim htop mc python

# Install a Nerd Font
curl -fLo ~/.termux/font.ttf \
  https://github.com/ryanoasis/nerd-fonts/raw/HEAD/patched-fonts/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf
termux-reload-settings

# Clone and install
git clone https://github.com/icex/termdesk.git
cd termdesk
./install.sh
```

### Manual Build (no install script)

```bash
# Linux / macOS
go build -o bin/termdesk ./cmd/termdesk
./bin/termdesk

# Android (Termux) — must use GOOS=android for Bionic TLS alignment
GOOS=android go build -o bin/termdesk ./cmd/termdesk
./bin/termdesk
```

> **Termux note**: `go run` does not work on Android — it produces an `unexpected e_type: 2` error. Termux reports `GOOS=linux` but runs on Android's Bionic linker which requires 64-byte TLS alignment. Building with `GOOS=android` makes Go use the correct alignment. No C compiler needed.

## Quick Start

```bash
termdesk            # if installed via install.sh
# or
make run            # build and run from source
```

## Sessions (tmux-like)

Termdesk runs as a persistent background server. Close your terminal and reattach later — all windows and shells survive.

```bash
termdesk              # Start or attach to default session
termdesk new work     # Create a named session
termdesk ls           # List active sessions
termdesk attach work  # Reattach to a named session
termdesk kill work    # Kill a session
```

**Detach**: Press `F12` to detach (or use File > Detach menu). The session keeps running in the background.

## Configuration

Settings are saved to `~/.config/termdesk/config.toml`:

```toml
theme = "modern"       # retro, modern, tokyonight, catppuccin
icons_only = false     # dock shows icons only (no labels)

[keybindings]
prefix = "ctrl+g"      # prefix key for Terminal mode (like tmux)
# quit = "q"
# new_terminal = "n"
# close_window = "w"
# snap_left = "h"
# snap_right = "l"
# maximize = "k"
# restore = "j"
# tile_all = "t"
# expose = "x"
# help = "f1"
# menu_bar = "f10"
```

Theme and dock mode changes are persisted automatically via the View menu. Keybindings are fully customizable — uncomment and change any binding above.

## Development

```bash
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run linter
make run            # Build and run
```

## Keybindings

### Normal Mode (Window Management)

| Key | Action |
|-----|--------|
| `q` | Quit |
| `i` / `Enter` | Enter Terminal mode |
| `n` / `Ctrl+N` | New terminal window |
| `w` / `Ctrl+W` | Close focused window |
| `m` | Minimize to dock |
| `r` | Rename window |
| `d` | Navigate dock |
| `Space` / `Ctrl+Space` | Open launcher |
| `Tab` | Next window |
| `h` / `l` | Snap left / right |
| `k` / `j` | Maximize / restore |
| `t` | Tile all windows |
| `x` | Expose mode |
| `f` / `a` / `v` | File / Apps / View menu |
| `1`-`9` | Focus window by number |
| `F1` | Help overlay |

### Terminal Mode (Prefix System)

In Terminal mode, all keys go to the terminal app. Use the **prefix key** (`Ctrl+G` by default) to access WM commands — just like tmux.

| Key | Action |
|-----|--------|
| `Ctrl+G` | Activate prefix (badge shows PREFIX) |
| `Ctrl+G` then action | Execute WM action (same keys as Normal mode) |
| `Ctrl+G` then `Esc` | Exit to Normal mode |
| `Ctrl+G` then `Ctrl+G` | Send `Ctrl+G` to terminal |
| `F2` | Exit to Normal mode (hardcoded fallback) |

### Session

| Key | Action |
|-----|--------|
| `F12` | Detach from session (keeps running) |

### Expose Mode

| Key | Action |
|-----|--------|
| `Tab` / `Arrow keys` | Cycle through windows |
| `1`-`9` | Select window by number |
| `Enter` | Select focused window (maximizes it) |
| `Escape` | Exit expose |

## Mouse Support

- **Click** title bar to focus and bring window to front
- **Drag** title bar to move window
- **Drag** borders/corners to resize
- **Click** close [X], maximize [box], minimize [_] buttons
- **Click** dock items to launch apps or restore minimized windows
- **Click** menu bar items, CPU/MEM/battery (opens htop), clock (shows date)
- **Click** expose thumbnails to select
- **Scroll wheel** in terminal windows
- **Mouse forwarding** to TUI apps (vim, htop, mc)

## Themes

Switch themes via the **View** menu:

- **Retro** — Win 1.0 inspired: white on navy blue, square box borders
- **Modern** — OneDark color scheme, rounded corners, cyan accent
- **Tokyo Night** — Dark purple-blue with soft blue highlights
- **Catppuccin Mocha** — Warm dark theme with lavender-blue accent

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| Linux | Fully supported | All features including CPU/MEM/battery indicators |
| macOS | Supported | Install via `install.sh`, battery requires `/sys` or pmset |
| Android (Termux) | Fully supported | Mouse, background colors, battery widget all working |

## Architecture

```
cmd/termdesk/         Main entry point
internal/
  app/                Root app model, rendering, animations, big text fonts
  config/             TOML configuration and theme definitions
  dock/               Dock bar with icons and running indicators
  launcher/           Command palette with fuzzy search
  menubar/            Menu bar with dropdowns and system indicators
  terminal/           PTY management and VT emulation bridge
  window/             Window struct, hit testing, manager with z-ordering
  clipboard/          Clipboard with ring buffer history
pkg/
  geometry/           Rect and Point primitives
  ringbuf/            Generic ring buffer
```

## License

MIT
