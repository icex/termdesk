# Termdesk Development Notes

Internal notes for people hacking on termdesk. Mostly gotchas I want future-me
to remember, plus a rough map of the codebase.

## Build

```bash
go build -o bin/termdesk ./cmd/termdesk
./bin/termdesk

# Tests
go test ./...

# Race detector
go test ./... -race

# Android / Termux — must build, not `go run`
GOOS=android go build -o bin/termdesk ./cmd/termdesk

# Performance overlay (FPS/timing + ~/.local/share/termdesk/perf.log)
TERMDESK_PERF=1 ./bin/termdesk
```

## Dependencies

- Go 1.25+
- `charm.land/bubbletea/v2` (v2 RC)
- `github.com/charmbracelet/x/vt` — VT emulator
- `github.com/creack/pty` — PTY
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/harmonica` — spring physics

## Layout

```
cmd/termdesk/       entry point, CLI, session bootstrap
internal/
  app/              root Model, Update, View, rendering, animations
  config/           TOML config, themes, keybindings, project config
  workspace/        workspace save/restore
  window/           window state, manager, hit testing, drag, tiling, splits
  terminal/         PTY, VT bridge, input/mouse, image passthrough
  menubar/          top menu bar + widgets
  dock/             bottom dock (shortcuts + window list)
  launcher/         command palette
  clipboard/        clipboard ring buffer + history overlay
  notification/     toasts + notification center
  session/          dtach-style client/server over Unix sockets
  settings/         settings panel
  widget/           built-in + custom widgets
  appmanifest/      TOML app manifests + registry
  contextmenu/      right-click menus
  tour/             first-launch tour
  help/             help overlay content
  apps/registry/    app registry used by dock/launcher/menubar
pkg/
  geometry/         Rect / Point
  ringbuf/          generic ring buffer
apps/
  calc/ minesweeper/ snake/ solitaire/ tetris/
```

## Bubble Tea v2 notes

- `View()` returns `tea.View` (a struct), not `string` — use `tea.NewView(s)` or `v.SetContent(s)`.
- `tea.KeyPressMsg` is `type KeyPressMsg Key`; build with `tea.Key{Code: 'q'}`.
- No `tea.WithAltScreen()` / `tea.WithMouseAllMotion()` — these are fields on `tea.View`.
- Mouse modes: `MouseModeNone`, `MouseModeCellMotion`, `MouseModeAllMotion`.
- Force TrueColor with `tea.WithColorProfile(colorprofile.TrueColor)`.
- Don't set `v.BackgroundColor` / `v.ForegroundColor` — they emit sticky OSC 10/11.

## State

- `Update()` returns a `tea.Model` value — pointer fields (`*renderCache`, `*programRef`)
  are shared across copies, plain fields are not.
- Don't mutate model state in `View()` — causes corruption.
- `bufio.Writer` is not safe for concurrent writes; protect with a mutex.

## Rendering pipeline

1. Composite windows into a cell `Buffer` (painter's algorithm).
2. `BufferToString()` emits ANSI — pre-size the builder (`sb.Grow(width*height*50)`).
3. Wrap in `StyledString` for `tea.View.SetContent()`.
4. `stampANSI()` parses lipgloss output back into the cell buffer (needs `\r\n`).

## Input modes

- **Normal** — window-management keys (h/j/k/l, n, w, …).
- **Terminal** — everything goes to the PTY; prefix key (default `Ctrl+A`) gates WM actions.
- **Copy** — vim-style scrollback. Uses a frozen snapshot while active (tmux-like).

## Sessions

- dtach-style client/server over Unix sockets.
- TLV framing: `I` input, `O` output, `R` resize, `D` detach, `S` redraw.
- Detach via prefix+d → server writes OSC `\x1b]666;detach\x07`.
- Don't strip `SSH_TTY` — BT v2 sync output (mode 2026) needs it to wrap through the PTY proxy.
- Client picks one of two read loops at attach time based on a DECRQM probe
  (`probeSync2026()`):
  - Mode-2026 terminals (iTerm2, kitty, WezTerm): `clientReadLoopSync` — 8ms tick,
    strips per-frame sync markers, wraps coalesced data in one sync block.
  - Everyone else (Termux, plain xterm, …): `clientReadLoopBasic` — rate-limited
    using the sync markers as frame boundaries, 30ms min interval (~33fps cap).
- Server injects a DECRQM reply after child start so BT enables sync wrapping
  even when `SSH_TTY` is set.

## Quake terminal

- Dropdown terminal that slides from the top.
- Default toggle `Ctrl+Backtick` (`quake_terminal` binding).
- Height via `quake_height_percent` (10–90, default 40).
- Single persistent terminal; bottom border is draggable for resize.
- Copy mode works — uses `quakeTermID = "__quake__"` sentinel.
- Regular windows are visually dimmed while the quake is visible.
- Impl: `app_terminal.go` (lifecycle) + `render_quake.go` (rendering).

## Split panes

- Tmux-style binary tree inside a window. `SplitRoot` + `FocusedPane` on the window.
- `SplitHorizontal` = vertical divider (left|right), `SplitVertical` = horizontal divider (top/bottom).
- Each pane owns its own PTY.
- Bindings: `Prefix+%` split H, `Prefix+"` split V, `Prefix+x` close,
  `Prefix+Arrow` navigate, `Prefix+Tab` cycle.
- Dividers are draggable; reflow is applied on mouse release, not during drag.
- 5-cell minimum pane size.
- Impl: `app_split.go`, `window/split.go`, `render_split.go`.

## Dock

- Two zones: launcher shortcuts (left) and window list (right, after Expose).
- Shortcuts always launch via `focusOrLaunchApp()` — Terminal opens new, others focus existing.
- Window list shows running/minimized windows as title-only entries (no icon).
- `MinimizeToDock` config controls whether minimized windows show up.
- `BaseItems` keeps the original items so we can rebuild after `hideDockApps` toggles.
- Project-specific entries come from `.termdesk.toml` `[[dock]]` (optional `position`).

## Widgets

- Built-ins: CPU, Memory, Battery, Notification, Workspace, Clock, Hostname, User.
- `enabled_widgets` config picks which ones show and in what order.
- Custom shell widgets via `[[custom_widgets]]` in `config.toml`.
- Custom widgets support `onClick` (e.g. `onClick = "lazygit"`).
- Settings panel "Widgets" tab for toggling.

## Wallpaper

Four modes: `theme` (default pattern), `color` (solid hex), `pattern`
(custom char + fg/bg), `program` (live terminal command — `cmatrix`, `pipes.sh`,
`asciiquarium`, …).

Program mode runs a headless `*terminal.Terminal` behind all windows. Auto-restarts
on exit. Uses `wallpaperTermID = "__wallpaper__"` sentinel.

Config keys: `wallpaper_mode`, `wallpaper_color`, `wallpaper_pattern`,
`wallpaper_pattern_fg`, `wallpaper_pattern_bg`, `wallpaper_program`.

Impl: `wallpaper.go` + `renderWallpaperTerminal` in `render_frame.go`.
`WallpaperConfig` is threaded through the rendering pipeline.

## Image passthrough

Three protocols, all optional and detected at startup:

- **Kitty graphics (APC)** — placements persist across frames in Kitty's
  separate layer. Deferred placements (`Hidden: true`) are shown on the next
  Update via `RefreshAllPlacements`. Guest image IDs are remapped to host IDs
  to avoid collisions between windows.
- **Sixel (DCS)** — fire-and-forget: BT's diff renderer overwrites pixels each
  frame, so we re-emit visible placements every render cycle via
  `RefreshAllImages`. Always truncate to available space via
  `truncateSixelBands()` — raster attributes can be wrong. Safety margin:
  `safeBottom = hostTermHeight - 2`, one extra band off.
- **iTerm2 (OSC 1337)** — same fire-and-forget model as sixel. Multipart support
  for `File=`, `MultipartFile=`, `FilePart=`, `FileEnd`.

Cursor advancement after an image must use newlines (`\n`), not CUD (`\x1b[NB`) —
CUD stops at the bottom margin without scrolling, and we need text to move up
to make room. Don't use DECSC/DECRC (`\x1b7`/`\x1b8`) inside `tea.Raw()` —
they conflict with BT's cursor state.

When placements are cleared (e.g. `clear`), send `ImageClearScreenMsg` so the
Update loop triggers `tea.ClearScreen` — BT's diff renderer can't see stale
sixel pixels injected via `tea.Raw()`.

8-bit DCS/ST (`\x90`, `\x9c`) are handled alongside the 7-bit variants.
Debug log: `/tmp/termdesk-image.log`.

## Workspaces

- Auto-save every 60s via `tea.Tick`.
- Per-project file: `{project}/.termdesk-workspace.toml`.
- Global: `~/.config/termdesk/workspace.toml`.
- Project auto-start: `.termdesk.toml` with `[[autostart]]` and `[[dock]]` sections.
- Auto-start dedupes by command name and fires in a separate Update cycle
  (`AutoStartMsg`) after workspace restore.

## Gotchas collected the hard way

- `range string` yields byte indices, not display positions. Use a counter.
- `len(string)` is a byte count. For display width use `utf8.RuneCountInString()`
  or, for Nerd Font icons and other multi-byte runes, `len([]rune(s))`.
- `Cell.Style.Bg` can be nil (meaning "default"). `renderTerminalContent` takes
  `defaultFg, defaultBg` precisely for this case.
- Caps lock: `msg.String()` returns uppercase. Normalize with `strings.ToLower(key)`
  outside copy mode; copy mode deliberately keeps the uppercase form because
  it matters (Y vs y, N vs n, …).
- VT emulator `SendKey` / `SendMouse` write to an `io.Pipe` — you need the
  `inputForwardLoop` goroutine running or writes block.
- `SendMouse` checks mouse mode first. Don't fall back to arrow keys for
  scroll — it double-scrolls under apps that already handle the wheel.
- `hexToColor`: don't use `fmt.Sscanf`; the hand-rolled nibble parser is ~10×
  faster and this is on hot paths.
- Battery paths: Android/Termux uses `/sys/class/power_supply/battery`, not
  `BAT0`/`BAT1` — discover dynamically.
- On Termux, `go run` fails with `unexpected e_type: 2`. Build with
  `GOOS=android` (Bionic linker needs 64-byte TLS alignment).
- Minimum window size is 40×10, enforced in `window/drag.go` `ApplyDrag`.
- Minimize animation: set `w.Minimized = true` *before* starting the animation
  so the dock entry appears immediately.
- Sixel cursor advance uses `\n`, not CUD — see image passthrough section.
- Don't emit DECSC/DECRC in `tea.Raw()` output; BT maintains its own cursor
  state and they fight.
- `BufferToString` must skip width-0 continuation cells — otherwise wide
  characters (CJK, double-width emoji) push later cells off the line.
- OSC 0/2 title intercept: `charmbracelet/x/ansi`'s parser treats byte `0x9C`
  as C1 ST inside OSC sequences, but `0x9C` is a valid UTF-8 continuation byte
  (e.g. `✳` U+2733 = `\xe2\x9c\xb3`). `extractOSCTitles()` in `terminal.go`
  intercepts OSC 0/2 before the emulator sees them and strips the sequence
  cleanly.

## Config

- User config: `~/.config/termdesk/config.toml`.
- Parser is hand-rolled (`[section]` support, no external TOML dep).
- Adding a setting: touch `UserConfig` struct, `parseConfig()`, `SaveUserConfig()`,
  read it in `internal/app/app.go` `New()`, add a settings-panel entry if useful.
- Themes: `ParseColors()` pre-parses hex → `color.Color`; access via `theme.C()`.
  Currently 14 themes (see `internal/config/theme.go`).

## Tests

```bash
go test ./...                       # everything
go test ./internal/app/ -v
go test ./internal/workspace/ -v
```

- PTY-interaction tests use real binaries (e.g. `/bin/echo`).
- Close-animation tests must call the `completeAnimations()` helper to drain
  the spring scheduler.
- Workspace tests use `t.TempDir()` for isolation.
- The whole suite honours `TERMDESK_CONFIG_PATH` / `TERMDESK_HOME` so it never
  touches a real user config.

## Adding a keybinding

1. Field on `KeyBindings` in `internal/config/userconfig.go`.
2. Default in `DefaultKeyBindings()`.
3. Case in `parseKeybinding()`.
4. Save path in `SaveUserConfig()` (skip if equal to default).
5. Handler in the appropriate `Update()` branch in `app.go`.
