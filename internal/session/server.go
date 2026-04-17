package session

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// detachOSC is the OSC sequence the app writes to trigger detach.
var detachOSC = []byte("\x1b]666;detach\x07")

// decrqmSync is the DECRQM query for mode 2026 (synchronized output).
// BT sends this at startup to detect terminal support.
var decrqmSync = []byte("\x1b[?2026$p")

// decrqmSyncReply tells BT that mode 2026 is supported (reset/known).
// This enables BT's synchronized output wrapping around frames.
var decrqmSyncReply = []byte("\x1b[?2026;2$y")

// decrqmGraphemeReply tells BT that mode 2027 (grapheme cluster width) is
// supported. Without this, BT's renderer uses codepoint-by-codepoint width
// (☀=1 + FE0F=0 = 1 cell) instead of grapheme width (☀️=2 cells), causing
// emoji to misalign all subsequent text on the same line.
var decrqmGraphemeReply = []byte("\x1b[?2027;2$y")

// Server manages a termdesk session: PTY, child app process, Unix socket, screen tracker.
type Server struct {
	name     string
	sockPath string
	pidPath  string

	ptmx  *os.File         // master side of the PTY
	ptyMu sync.Mutex       // protects ptmx.Write and pty.Setsize calls
	cmd   *exec.Cmd        // the child termdesk --app process
	emu   *vt.SafeEmulator // mirrors PTY output for screen capture on reattach

	listener net.Listener

	clientMu sync.Mutex
	writeMu  sync.Mutex
	client   net.Conn // current connected client (nil if none)

	done     chan struct{}
	doneOnce sync.Once
}

// ServerOptions controls how the session server launches the child app.
type ServerOptions struct {
	AppCommand string
	AppArgs    []string
	AppEnv     []string
	AppDir     string
}

// NewServer creates and starts a new server for the named session.
func NewServer(name string, cols, rows int, opts ServerOptions) (*Server, error) {
	if err := EnsureSocketDir(); err != nil {
		return nil, fmt.Errorf("cannot create socket dir: %w", err)
	}

	if opts.AppCommand == "" {
		return nil, fmt.Errorf("missing app command")
	}

	sockPath := SocketPath(name)
	pidPath := PidPath(name)

	// Clean up stale socket if present
	os.Remove(sockPath)

	// Create PTY pair
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("pty.Open: %w", err)
	}

	// Set initial size
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}); err != nil {
		ptmx.Close()
		tty.Close()
		return nil, fmt.Errorf("pty.Setsize: %w", err)
	}

	// Launch child app process.
	cmd := exec.Command(opts.AppCommand, opts.AppArgs...)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Env = opts.AppEnv
	if opts.AppDir != "" {
		cmd.Dir = opts.AppDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0, // fd 0 in the child = tty (stdin was dup2'd to tty)
	}

	if err := cmd.Start(); err != nil {
		ptmx.Close()
		tty.Close()
		return nil, fmt.Errorf("start app: %w", err)
	}
	tty.Close() // server only needs the master side

	// VT emulator to track screen state for reattach
	emu := vt.NewSafeEmulator(cols, rows)

	// Build the Server struct early so we can use ptyMu for the DECRQM write.
	s := &Server{
		name:     name,
		sockPath: sockPath,
		pidPath:  pidPath,
		ptmx:     ptmx,
		cmd:      cmd,
		emu:      emu,
		done:     make(chan struct{}),
	}

	// Inject fake DECRQM response for mode 2026 (synchronized output)
	// into the PTY master. BT's input handler will see this on stdin and
	// enable sync output wrapping around frames. We do this BEFORE BT
	// starts its Run() loop so the response is queued in the PTY buffer.
	// SSH_TTY is set in BT's env to prevent Kitty keyboard and other
	// features that don't work through the PTY proxy, but SSH_TTY also
	// prevents BT from querying mode 2026 — so we inject the response
	// directly instead.
	s.ptyMu.Lock()
	ptmx.Write(decrqmSyncReply)
	ptmx.Write(decrqmGraphemeReply)
	s.ptyMu.Unlock()

	// Listen on Unix socket
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		cmd.Process.Kill()
		ptmx.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		// Log but don't fail — the session can still function without a PID file
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}

	s.listener = listener
	return s, nil
}

// Run is the main server loop. Blocks until the child exits or SIGTERM.
func (s *Server) Run() error {
	defer s.cleanup()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	// Read PTY output → emulator + client
	go s.readPtyLoop()

	// Accept client connections
	go s.acceptLoop()

	// Wait for child to exit
	childDone := make(chan error, 1)
	go func() {
		childDone <- s.cmd.Wait()
	}()

	select {
	case <-sigCh:
		s.cmd.Process.Signal(syscall.SIGTERM)
	case <-childDone:
		// App exited normally (user quit)
	case <-s.done:
		// Server told to stop
	}

	return nil
}

// readPtyLoop reads from the master PTY, feeds to emulator, and forwards to client.
func (s *Server) readPtyLoop() {
	buf := make([]byte, 32768)
	// Emulator runs in a separate goroutine to keep it off the critical output path.
	emuCh := make(chan []byte, 64)
	go func() {
		for data := range emuCh {
			s.emu.Write(data)
		}
	}()
	defer close(emuCh)

	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			data := buf[:n]

			// Check for OSC detach sequence from the app
			detach := false
			if bytes.Contains(data, detachOSC) {
				detach = true
				data = bytes.ReplaceAll(data, detachOSC, nil)
			}

			// Feed copy to emulator goroutine (async, non-blocking best-effort)
			if len(data) > 0 {
				emuData := make([]byte, len(data))
				copy(emuData, data)
				select {
				case emuCh <- emuData:
				default:
				}
			}

			// Forward to connected client with a write deadline to prevent
			// blocking the PTY read loop if the client is slow to consume.
			s.clientMu.Lock()
			c := s.client
			s.clientMu.Unlock()
			if c != nil && len(data) > 0 {
				s.writeMu.Lock()
				c.SetWriteDeadline(time.Now().Add(2 * time.Second))
				writeErr := WriteMsg(c, MsgOutput, data)
				c.SetWriteDeadline(time.Time{})
				s.writeMu.Unlock()
				if writeErr != nil {
					s.disconnectClient()
				}
			}

			if detach {
				s.disconnectClient()
			}
		}
		if err != nil {
			s.stop()
			return
		}
	}
}

// acceptLoop accepts incoming client connections. One client at a time.
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}

		// Disconnect existing client (new one replaces it)
		s.disconnectClient()

		s.clientMu.Lock()
		s.client = conn
		s.clientMu.Unlock()

		// Send terminal mode preamble for reattach.
		// The client's terminal starts fresh — re-enable alt screen,
		// mouse mode, and colors. A forced PTY resize follows to trigger
		// SIGWINCH so BT redraws the full frame with correct dimensions.
		var preamble []byte
		preamble = append(preamble, "\x1b[?1049h"...) // alt screen
		preamble = append(preamble, "\x1b[?1002h"...) // mouse cell motion
		preamble = append(preamble, "\x1b[?1006h"...) // SGR mouse encoding
		preamble = append(preamble, "\x1b[?25l"...)   // hide cursor (BT manages it)
		preamble = append(preamble, "\x1b[?2004h"...) // bracketed paste
		s.writeMu.Lock()
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		err = WriteMsg(conn, MsgRedraw, preamble)
		conn.SetWriteDeadline(time.Time{})
		s.writeMu.Unlock()
		if err != nil {
			s.disconnectClient()
			continue
		}

		// Force SIGWINCH to the child app so BT does a full redraw.
		// pty.Setsize to the same dimensions doesn't generate SIGWINCH,
		// so we send the signal explicitly.
		s.cmd.Process.Signal(syscall.SIGWINCH)

		go s.handleClient(conn)
	}
}

// handleClient reads messages from a connected client.
func (s *Server) handleClient(conn net.Conn) {
	for {
		typ, payload, err := ReadMsg(conn)
		if err != nil {
			s.disconnectClient()
			return
		}

		switch typ {
		case MsgInput:
			s.ptyMu.Lock()
			s.ptmx.Write(payload)
			s.ptyMu.Unlock()

		case MsgResize:
			if len(payload) == 4 {
				rows, cols := DecodeResize(payload)
				s.ptyMu.Lock()
				pty.Setsize(s.ptmx, &pty.Winsize{
					Rows: rows,
					Cols: cols,
				})
				s.ptyMu.Unlock()
				s.emu.Resize(int(cols), int(rows))
				// Explicitly signal the child — pty.Setsize doesn't always
				// generate SIGWINCH reliably through the PTY proxy.
				s.cmd.Process.Signal(syscall.SIGWINCH)
			}

		case MsgDetach:
			s.disconnectClient()
			return
		}
	}
}

func (s *Server) disconnectClient() {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
}

// stop signals the server to shut down by closing the done channel.
// Safe to call multiple times from any goroutine.
func (s *Server) stop() {
	s.doneOnce.Do(func() { close(s.done) })
}

func (s *Server) cleanup() {
	s.listener.Close()
	s.disconnectClient()
	s.ptmx.Close()
	os.Remove(s.sockPath)
	os.Remove(s.pidPath)
}

// BuildAppEnv creates a sanitized environment for the child app process.
// Terminal-specific env vars (TERM_PROGRAM, KITTY_*, ITERM_*, etc.) are
// removed so that Bubble Tea v2 doesn't enable features like synchronized
// output or Kitty keyboard protocol that don't work through the PTY proxy.
// Graphics capability is detected from the host terminal's env BEFORE
// stripping and propagated via TERMDESK_GRAPHICS={kitty,sixel,iterm2}.
func BuildAppEnv() []string {
	// Detect Kitty graphics capability from the host terminal env BEFORE
	// stripping. The server process inherits the client's env vars.
	hasGraphics := false
	switch os.Getenv("TERM_PROGRAM") {
	case "WezTerm", "kitty", "ghostty":
		hasGraphics = true
	}
	if !hasGraphics && os.Getenv("KITTY_WINDOW_ID") != "" {
		hasGraphics = true
	}
	if !hasGraphics && os.Getenv("WEZTERM_PANE") != "" {
		hasGraphics = true
	}
	if !hasGraphics && (os.Getenv("KONSOLE_DBUS_SESSION") != "" || os.Getenv("KONSOLE_VERSION") != "") {
		hasGraphics = true
	}
	// Detect iTerm2 inline image support INDEPENDENTLY of Kitty.
	// Some terminals (WezTerm) support both Kitty and iTerm2 protocols.
	// Since they use different sequence types (APC vs OSC), both can be
	// active simultaneously without conflict.
	hasIterm2 := false
	tp := os.Getenv("TERM_PROGRAM")
	if tp == "iTerm.app" || tp == "WezTerm" {
		hasIterm2 = true
	}
	if !hasIterm2 && os.Getenv("LC_TERMINAL") == "iTerm2" {
		hasIterm2 = true
	}
	if !hasIterm2 && os.Getenv("ITERM_SESSION_ID") != "" {
		hasIterm2 = true
	}
	if !hasIterm2 && os.Getenv("WEZTERM_PANE") != "" {
		hasIterm2 = true
	}

	// Detect sixel support INDEPENDENTLY of Kitty/iTerm2.
	// Many terminals support multiple protocols (e.g. WezTerm supports all 3).
	// Sixel uses DCS sequences which don't conflict with Kitty (APC) or iTerm2 (OSC).
	hasSixel := false
	switch tp {
	case "WezTerm", "ghostty", "foot", "contour", "mlterm", "iTerm.app":
		hasSixel = true
	}
	if !hasSixel && os.Getenv("XTERM_VERSION") != "" {
		hasSixel = true
	}

	// Session re-propagation: pick up existing detection flags
	gfx := os.Getenv("TERMDESK_GRAPHICS")
	if !hasGraphics && gfx == "kitty" {
		hasGraphics = true
	}
	if !hasIterm2 && (gfx == "iterm2" || os.Getenv("TERMDESK_ITERM2") == "1") {
		hasIterm2 = true
	}
	if !hasSixel && (gfx == "sixel" || os.Getenv("TERMDESK_SIXEL") == "1") {
		hasSixel = true
	}

	skip := map[string]bool{
		"TERM": true, "COLORTERM": true,
		"TERM_PROGRAM": true, "TERM_PROGRAM_VERSION": true,
		"WT_SESSION": true, "VTE_VERSION": true,
		"LC_TERMINAL": true, "LC_TERMINAL_VERSION": true,
		// Strip SSH_TTY so BT v2 queries for synchronized output (mode 2026).
		// Without sync output, frame diffs over SSH cause visible flicker.
		// The DECRQM query/response round-trips through the session proxy correctly.
		"SSH_TTY": true,
		// Managed by BuildAppEnv — strip to avoid duplicates.
		"TERMDESK": true, "TERMDESK_GRAPHICS": true,
		"TERMDESK_ITERM2": true, "TERMDESK_SIXEL": true,
	}
	var env []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if skip[k] || strings.HasPrefix(k, "KITTY_") || strings.HasPrefix(k, "ITERM_") {
			continue
		}
		env = append(env, e)
	}
	// Preserve the client terminal's COLORTERM for proper color profile
	// detection. Defaults to truecolor for modern terminals.
	colorterm := os.Getenv("COLORTERM")
	if colorterm == "" {
		colorterm = "truecolor"
	}
	env = append(env,
		"TERM=xterm-256color",
		"COLORTERM="+colorterm,
		"TERMDESK=1",
	)
	if hasGraphics {
		env = append(env, "TERMDESK_GRAPHICS=kitty")
	} else if hasIterm2 {
		env = append(env, "TERMDESK_GRAPHICS=iterm2")
	} else if hasSixel {
		env = append(env, "TERMDESK_GRAPHICS=sixel")
	}
	// iTerm2 support flag — can be alongside Kitty since they use different
	// sequence types (APC vs OSC). Enables imgcat etc. in WezTerm/iTerm2.
	if hasIterm2 {
		env = append(env, "TERMDESK_ITERM2=1")
		// Propagate iTerm2 detection env vars so that child tools (imgcat,
		// etc.) detect inline image protocol support. TERM_PROGRAM is
		// stripped to prevent BT v2 feature conflicts, but LC_TERMINAL
		// and ITERM_SESSION_ID are safe and widely checked.
		env = append(env, "LC_TERMINAL=iTerm2")
		env = append(env, "ITERM_SESSION_ID=termdesk")
	}
	// Sixel support flag — can be alongside Kitty since they use different
	// sequence types (DCS vs APC). Enables img2sixel etc. in WezTerm/foot.
	if hasSixel {
		env = append(env, "TERMDESK_SIXEL=1")
	}
	return env
}
