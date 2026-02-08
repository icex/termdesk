# Termdesk

A retro terminal desktop environment inspired by Windows 1.0, DESQview, and Mac System 1.0 — built with Go and the Charmbracelet TUI stack (Bubble Tea v2).

## Features

- **Overlapping windows** with drag, resize, maximize, snap (left/right), tile, and minimize
- **PTY terminal windows** running real shells and TUI apps (nvim, htop, mc, etc.)
- **VT100/ANSI terminal emulation** via charmbracelet/x/vt with full keyboard and mouse forwarding
- **Menu bar** with dropdowns, live CPU/MEM/battery indicators, clock (click for date popup, click CPU/MEM for btop), logged-in username
- **Dock** with colorful Nerd Font icons, running indicators, minimized window tabs, launch pulse animation
- **Command palette** (Ctrl+Space) with fuzzy search across installed apps
- **4 themes**: Retro (Win 1.0 blue), Modern (OneDark), Tokyo Night, Catppuccin Mocha — switchable via View menu
- **Expose mode** for window overview with animated cycling (focused window centered, thumbnails in strip)
- **Window animations** for open, close, maximize, restore, snap, tile, minimize, and expose transitions
- **Input modes**: Normal (WM keys), Terminal (passthrough to shell), Copy (vim-style scrollback)
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
- Recommended apps: `btop`, `nvim`, `python3` (used by dock/launcher)

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
sudo apt install golang neovim btop python3

# Or on Arch
sudo pacman -S go neovim btop python3

# Then build and install
./install.sh
```

### macOS

```bash
# Install with Homebrew
brew install go neovim btop python3
brew install --cask font-jetbrains-mono-nerd-font

# Then build and install
./install.sh
```

### Android (Termux)

```bash
# Install dependencies
pkg install golang neovim btop python

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
go build -o bin/termdesk ./cmd/termdesk
./bin/termdesk
```

## Quick Start

```bash
termdesk            # if installed via install.sh
# or
make run            # build and run from source
```

## Configuration

Settings are saved to `~/.config/termdesk/config.toml`:

```toml
theme = "modern"       # retro, modern, tokyonight, catppuccin
icons_only = false     # dock shows icons only (no labels)
```

Theme and dock mode changes are persisted automatically via the View menu.

## Development

```bash
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run linter
make run            # Build and run
```

## Keybindings

### Window Management (Normal Mode)

| Key | Action |
|-----|--------|
| `Ctrl+N` | New terminal window |
| `Ctrl+W` | Close focused window (with confirmation) |
| `Ctrl+Space` | Open command palette |
| `Alt+Tab` | Cycle windows forward |
| `Alt+Shift+Tab` | Cycle windows backward |
| `Ctrl+Left` | Snap window left |
| `Ctrl+Right` | Snap window right |
| `Ctrl+Up` | Maximize window |
| `Ctrl+Down` | Restore window |
| `Ctrl+T` | Tile all windows |
| `R` | Rename focused window |
| `Q` | Quit (in Normal mode, no window focused) |
| `F1` | Help overlay |
| `F10` | Toggle menu bar |
| `Escape` | Exit current mode / dismiss overlay |

### Input Modes

| Key | Action |
|-----|--------|
| `Enter` (on focused window) | Switch to Terminal mode |
| `Escape` (in Terminal mode) | Return to Normal mode |

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
- **Click** menu bar items, CPU/MEM/battery (opens btop), clock (shows date)
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
