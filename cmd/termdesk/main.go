package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/creack/pty"
	"github.com/icex/termdesk/internal/app"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/logging"
	"github.com/icex/termdesk/internal/session"
	"github.com/icex/termdesk/internal/terminal"

	tea "charm.land/bubbletea/v2"
	goterm "github.com/charmbracelet/x/term"
)

func main() {
	// Parse --log-level flag from anywhere in args (strip it before subcommand dispatch).
	// Priority: CLI flag > env var > config file.
	args := os.Args[1:]
	var cliLogLevel string
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--log-level" && i+1 < len(args) {
			cliLogLevel = args[i+1]
			i++ // skip value
		} else if strings.HasPrefix(args[i], "--log-level=") {
			cliLogLevel = strings.TrimPrefix(args[i], "--log-level=")
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}
	os.Args = append([]string{os.Args[0]}, filteredArgs...)

	// Initialize logging: CLI flag takes priority, then env, then config.
	initLogging(cliLogLevel)

	sess := session.NewAdapter()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--app":
			runApp()
			return
		case "--server":
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			runServer(sess, name)
			return
		case "ls", "list":
			listSessions(sess)
			return
		case "keys", "keybindings":
			if len(os.Args) > 2 && (os.Args[2] == "edit" || os.Args[2] == "--edit" || os.Args[2] == "-e") {
				editKeybindings()
				return
			}
			listKeybindings()
			return
		case "new":
			checkNesting()
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			createAndAttach(sess, name)
			return
		case "attach", "a":
			checkNesting()
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			attachSession(sess, name)
			return
		case "kill":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: termdesk kill NAME")
				os.Exit(1)
			}
			killSession(sess, os.Args[2])
			return
		case "--help", "-h", "help":
			printUsage()
			return
		default:
			// If the argument is a directory path, chdir to it and use a
			// project-specific session name (so each project gets its own session).
			// This allows: termdesk /path/to/project  or  termdesk ../relative/path
			if info, err := os.Stat(os.Args[1]); err == nil && info.IsDir() {
				absPath, err := filepath.Abs(os.Args[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "termdesk: cannot resolve path %s: %v\n", os.Args[1], err)
					os.Exit(1)
				}
				if err := os.Chdir(absPath); err != nil {
					fmt.Fprintf(os.Stderr, "termdesk: cannot chdir to %s: %v\n", absPath, err)
					os.Exit(1)
				}
				checkNesting()
				// Use directory name as session name so projects don't collide
				name := filepath.Base(absPath)
				createOrAttach(sess, name)
				return
			}
			fmt.Fprintf(os.Stderr, "termdesk: unknown command %q\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}

	// Default: attach to existing "default" session, or create one
	checkNesting()
	createOrAttach(sess, session.DefaultSession)
}

// bracketedPasteFilter manually parses bracketed paste sequences when SSH_TTY disables BT's parser.
// The session server enables bracketed paste mode (\x1b[?2004h), but BT skips parsing when SSH_TTY is set.
// This filter intercepts key messages containing paste sequences and converts them to PasteMsg.
func bracketedPasteFilter(m tea.Model, msg tea.Msg) tea.Msg {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return msg
	}

	// Bracketed paste format: \x1b[200~...content...\x1b[201~
	// The entire sequence arrives as a single key message with Text containing the full sequence
	text := keyMsg.Text
	if len(text) < 12 {
		return msg // too short to contain bracketed paste
	}

	// Check for start marker
	if !strings.HasPrefix(text, "\x1b[200~") {
		return msg
	}

	// Find end marker
	endIdx := strings.Index(text, "\x1b[201~")
	if endIdx == -1 {
		return msg // incomplete bracketed paste sequence
	}

	// Extract pasted content (between start and end markers)
	content := text[6:endIdx] // skip "\x1b[200~"

	return tea.PasteMsg{Content: content}
}

// runApp runs the Bubble Tea app directly (used by the server on a slave PTY).
func runApp() {
	m := app.New()
	// The app runs behind a PTY proxy (session system), so BT v2 features
	// that require direct terminal communication (Kitty keyboard, mouse
	// mode queries) don't work reliably. Setting SSH_TTY tells BT we're
	// behind a PTY proxy — it skips those features. Synchronized output
	// (mode 2026) is enabled separately: the server injects a fake DECRQM
	// response into the PTY so BT's input handler enables sync output.
	// Bracketed paste mode is enabled by the session server in the
	// terminal, so we use a filter to manually parse those sequences.
	// Color profile is auto-detected from COLORTERM env var set by the
	// session server (preserves the connecting client's capabilities).
	opts := []tea.ProgramOption{
		tea.WithEnvironment(append(os.Environ(), "SSH_TTY=/dev/pts/proxy")),
		tea.WithFilter(bracketedPasteFilter),
	}
	// Force TrueColor only when COLORTERM confirms support; otherwise let
	// Bubble Tea auto-detect from TERM/COLORTERM (supports 256-color etc).
	ct := os.Getenv("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
	}
	p := tea.NewProgram(m, opts...)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

// runServer starts the server process for a named session.
func runServer(sess session.Adapter, name string) {
	cols, rows := 80, 24
	selfExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "termdesk server: %v\n", err)
		os.Exit(1)
	}
	opts := session.ServerOptions{
		AppCommand: selfExe,
		AppArgs:    []string{"--app"},
		AppEnv:     session.BuildAppEnv(),
	}
	srv, err := sess.NewServer(name, cols, rows, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "termdesk server: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk server: %v\n", err)
		os.Exit(1)
	}
}

// createOrAttach attaches to an existing session, or creates a new one.
func createOrAttach(sess session.Adapter, name string) {
	// Probe host terminal to detect actual emoji character width.
	// Some terminals render emoji-capable chars (☀ U+2600 etc.) as 2 cells
	// even without VS16, while others use 1 cell. The VT emulator and Go
	// width libraries can't predict this, so we ask the terminal directly.
	probeEmojiWidth()

	if sess.SessionExists(name) {
		attachSession(sess, name)
	} else {
		createAndAttach(sess, name)
	}
}

// probeEmojiWidth queries the host terminal to determine whether ☀ (U+2600)
// renders as 1 or 2 cells. Sets TERMDESK_EMOJI_WIDTH=2 if wide, so the
// rendering pipeline can compensate. Uses DSR (cursor position report) to
// measure actual cursor advancement.
func probeEmojiWidth() {
	fd := os.Stdin.Fd()
	if !goterm.IsTerminal(fd) {
		return
	}
	oldState, err := goterm.MakeRaw(fd)
	if err != nil {
		return
	}
	defer goterm.Restore(fd, oldState)

	// Save cursor, move to col 1, print test char, query position
	os.Stdout.Write([]byte("\x1b7\x1b[1G☀\x1b[6n"))

	// Read DSR response: \x1b[row;colR
	var resp []byte
	buf := make([]byte, 32)
	deadline := time.Now().Add(100 * time.Millisecond)
	os.Stdin.SetReadDeadline(deadline)
	for time.Now().Before(deadline) {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			resp = append(resp, buf[:n]...)
			if bytes.ContainsRune(resp, 'R') {
				break
			}
		}
		if err != nil {
			break
		}
	}
	os.Stdin.SetReadDeadline(time.Time{})

	// Restore cursor and clear the test character
	os.Stdout.Write([]byte("\x1b8\x1b[K"))

	// Parse response: \x1b[row;colR → extract col
	if idx := bytes.IndexByte(resp, ';'); idx >= 0 {
		if end := bytes.IndexByte(resp[idx:], 'R'); end >= 0 {
			colStr := string(resp[idx+1 : idx+end])
			if col, err := strconv.Atoi(colStr); err == nil {
				// col is 1-indexed. If ☀ took 2 cells, cursor is at col 3.
				// If 1 cell, cursor is at col 2.
				logging.Info("probeEmojiWidth: ☀ cursor at col %d (resp=%q)", col, string(resp))
				if col >= 3 {
					os.Setenv("TERMDESK_EMOJI_WIDTH", "2")
					logging.Info("probeEmojiWidth: host terminal renders emoji as 2 cells")
				}
			}
		}
	}
}

// createAndAttach starts a server in the background, then attaches.
func createAndAttach(sess session.Adapter, name string) {
	if sess.SessionExists(name) {
		fmt.Fprintf(os.Stderr, "session %q already exists, attaching...\n", name)
		attachSession(sess, name)
		return
	}

	// Query cell pixel size from the real terminal before detaching.
	// The server process inherits this env var; BuildAppEnv() passes it
	// through to the --app child for Kitty graphics pixel size reporting.
	if ws, err := pty.GetsizeFull(os.Stdout); err == nil &&
		ws.X > 0 && ws.Y > 0 && ws.Cols > 0 && ws.Rows > 0 {
		cellW := int(ws.X) / int(ws.Cols)
		cellH := int(ws.Y) / int(ws.Rows)
		os.Setenv("TERMDESK_CELL_PX", strconv.Itoa(cellW)+"x"+strconv.Itoa(cellH))
	}

	// Detect Kitty graphics via active TTY query. Skips terminals already
	// known to support Kitty (env-based detection handles them, and the
	// raw-mode TTY query could interfere). For other terminals (iTerm2,
	// Konsole without env, SSH, etc.), the query is the only way to detect
	// Kitty graphics support.
	tp := os.Getenv("TERM_PROGRAM")
	alreadyKitty := tp == "kitty" || tp == "WezTerm" || tp == "ghostty" ||
		os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("WEZTERM_PANE") != "" ||
		os.Getenv("KONSOLE_DBUS_SESSION") != "" || os.Getenv("KONSOLE_VERSION") != ""
	if os.Getenv("TERMDESK_GRAPHICS") == "" && !alreadyKitty {
		if terminal.DetectKittyGraphicsTTY() {
			os.Setenv("TERMDESK_GRAPHICS", "kitty")
		}
	}

	// Detect sixel INDEPENDENTLY of Kitty via DA1 query.
	// Many terminals support both (iTerm2, WezTerm). Set TERMDESK_SIXEL=1
	// alongside TERMDESK_GRAPHICS=kitty — they use different sequence types
	// (DCS vs APC) and don't conflict.
	if os.Getenv("TERMDESK_SIXEL") != "1" {
		if terminal.DetectSixelTTY() {
			os.Setenv("TERMDESK_SIXEL", "1")
		}
	}
	// Fallback: if no Kitty detected, use sixel as primary graphics.
	if os.Getenv("TERMDESK_GRAPHICS") == "" && os.Getenv("TERMDESK_SIXEL") == "1" {
		os.Setenv("TERMDESK_GRAPHICS", "sixel")
	}

	// Start server as a detached background process
	selfExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find executable: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(selfExe, "--server", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // detach from our terminal
	}
	devNull, _ := os.Open(os.DevNull)
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	// Log server errors for debugging
	logPath := sess.SocketDir() + "/" + name + ".log"
	sess.EnsureSocketDir()
	logFile, logErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if logErr != nil {
		cmd.Stderr = devNull
	} else {
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot start server: %v\n", err)
		os.Exit(1)
	}
	cmd.Process.Release() // don't wait for it

	// Wait for server to be ready
	for i := 0; i < 50; i++ {
		if sess.SessionExists(name) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !sess.SessionExists(name) {
		fmt.Fprintf(os.Stderr, "server failed to start (check %s)\n", logPath)
		os.Exit(1)
	}

	attachSession(sess, name)
}

// attachSession connects to a running session.
func attachSession(sess session.Adapter, name string) {
	if err := sess.Attach(name); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

// listSessions prints all active sessions.
func listSessions(sess session.Adapter) {
	sessions, err := sess.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		return
	}
	for _, s := range sessions {
		fmt.Printf("  %-20s (pid %d)\n", s.Name, s.Pid)
	}
}

// killSession sends SIGTERM to the server process for a named session.
func killSession(sess session.Adapter, name string) {
	pidPath := sess.PidPath(name)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session %q not found\n", name)
		os.Exit(1)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "invalid pid for session %q\n", name)
		os.Exit(1)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "cannot kill session %q (pid %d): %v\n", name, pid, err)
		os.Exit(1)
	}
	fmt.Printf("Killed session %q (pid %d)\n", name, pid)
}

// checkNesting exits with an error if we're already inside a termdesk session.
// Internal subcommands (--app, --server) and read-only commands (ls, kill, help)
// bypass this check — only user-facing session commands are blocked.
func checkNesting() {
	if os.Getenv("TERMDESK") == "1" {
		fmt.Fprintln(os.Stderr, "termdesk: already running inside a termdesk session (nesting is not supported)")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`termdesk - A retro TUI desktop environment

Usage:
  termdesk              Start or attach to default session
  termdesk <path>       Start in a project directory (loads .termdesk.toml)
  termdesk new [NAME]   Create a new named session
  termdesk attach NAME  Attach to an existing session
  termdesk ls           List active sessions
  termdesk keys         List keybindings
  termdesk keys --edit  Open keybindings config in $EDITOR
  termdesk kill NAME    Kill a session
  termdesk help         Show this help

Options:
  --log-level LEVEL     Set log level: debug, info, warn, error, off
                        (also configurable via log_level in config.toml)
                        Logs saved to ~/.local/share/termdesk/termdesk.log

Environment variables:
  TERMDESK_PERF=1       Enable render performance overlay and logging
                        Shows FPS, frame timing, cache stats on screen
                        Writes periodic stats to ~/.local/share/termdesk/perf.log

Session control:
  Ctrl+G, d             Detach from session (session keeps running)
  Ctrl+Q                Quit termdesk (ends session)`)
}

func listKeybindings() {
	cfg := config.LoadUserConfig()
	kb := cfg.Keys
	type entry struct {
		name string
		key  string
	}
	entries := []entry{
		{"Prefix", kb.Prefix},
		{"Quit", kb.Quit},
		{"New Terminal", kb.NewTerminal},
		{"Close Window", kb.CloseWindow},
		{"Enter Terminal", kb.EnterTerminal},
		{"Minimize", kb.Minimize},
		{"Rename Window", kb.Rename},
		{"Dock Focus", kb.DockFocus},
		{"Launcher", kb.Launcher},
		{"Snap Left", kb.SnapLeft},
		{"Snap Right", kb.SnapRight},
		{"Maximize", kb.Maximize},
		{"Restore", kb.Restore},
		{"Tile All", kb.TileAll},
		{"Expose", kb.Expose},
		{"Next Window", kb.NextWindow},
		{"Prev Window", kb.PrevWindow},
		{"Help", kb.Help},
		{"Toggle Expose", kb.ToggleExpose},
		{"Menu Bar", kb.MenuBar},
		{"Menu File", kb.MenuFile},
		{"Menu Edit", kb.MenuEdit},
		{"Menu Apps", kb.MenuApps},
		{"Menu View", kb.MenuView},
		{"Move Left", kb.MoveLeft},
		{"Move Right", kb.MoveRight},
		{"Move Up", kb.MoveUp},
		{"Move Down", kb.MoveDown},
		{"Grow Width", kb.GrowWidth},
		{"Shrink Width", kb.ShrinkWidth},
		{"Grow Height", kb.GrowHeight},
		{"Shrink Height", kb.ShrinkHeight},
		{"Snap Top", kb.SnapTop},
		{"Snap Bottom", kb.SnapBottom},
		{"Center", kb.Center},
		{"Tile Columns", kb.TileColumns},
		{"Tile Rows", kb.TileRows},
		{"Cascade", kb.Cascade},
		{"Clipboard History", kb.ClipboardHistory},
		{"Notification Center", kb.NotificationCenter},
		{"Settings", kb.Settings},
		{"Quick Next Window", kb.QuickNextWindow},
		{"Quick Prev Window", kb.QuickPrevWindow},
		{"Tile Maximized", kb.TileMaximized},
		{"Show Desktop", kb.ShowDesktop},
		{"Toggle Tiling", kb.ToggleTiling},
		{"Swap Left", kb.SwapLeft},
		{"Swap Right", kb.SwapRight},
		{"Swap Up", kb.SwapUp},
		{"Swap Down", kb.SwapDown},
		{"Project Picker", kb.ProjectPicker},
		{"Save Workspace", kb.SaveWorkspace},
		{"Load Workspace", kb.LoadWorkspace},
		{"New Workspace", kb.NewWorkspace},
		{"Show Keys", kb.ShowKeys},
	}

	fmt.Println("Keybindings:")
	for _, e := range entries {
		if e.key == "" {
			continue
		}
		fmt.Printf("  %-20s %s\n", e.name, e.key)
	}
}

func editKeybindings() {
	path := config.ConfigPath()
	if path == "" {
		fmt.Fprintln(os.Stderr, "termdesk: cannot resolve config path")
		os.Exit(1)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := config.SaveUserConfig(config.DefaultUserConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "termdesk: cannot create config: %v\n", err)
			os.Exit(1)
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		fmt.Fprintln(os.Stderr, "termdesk: $EDITOR is not set")
		os.Exit(1)
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: editor failed: %v\n", err)
		os.Exit(1)
	}
}

// initLogging sets up the logging subsystem.
// Priority: CLI flag > TERMDESK_LOG_LEVEL env > config file > TERMDESK_DEBUG compat.
func initLogging(cliLevel string) {
	level := logging.LevelOff

	// 1. Config file (lowest priority)
	cfg := config.LoadUserConfig()
	if cfg.LogLevel != "" {
		level = logging.ParseLevel(cfg.LogLevel)
	}

	// 2. Environment variable
	if envLevel := os.Getenv("TERMDESK_LOG_LEVEL"); envLevel != "" {
		level = logging.ParseLevel(envLevel)
	}

	// 3. Legacy TERMDESK_DEBUG=1 compat
	if level == logging.LevelOff && os.Getenv("TERMDESK_DEBUG") == "1" {
		level = logging.LevelDebug
	}

	// 4. CLI flag (highest priority)
	if cliLevel != "" {
		level = logging.ParseLevel(cliLevel)
	}

	// Propagate level to child processes (--app via session server)
	if level != logging.LevelOff {
		os.Setenv("TERMDESK_LOG_LEVEL", logging.LevelName(level))
	}

	logging.Init(level)
}
