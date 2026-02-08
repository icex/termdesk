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

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// detachOSC is the OSC sequence the app writes to trigger detach.
var detachOSC = []byte("\x1b]666;detach\x07")

// Server manages a termdesk session: PTY, child app process, Unix socket, screen tracker.
type Server struct {
	name     string
	sockPath string
	pidPath  string

	ptmx *os.File   // master side of the PTY
	cmd  *exec.Cmd  // the child termdesk --app process
	emu  *vt.SafeEmulator // mirrors PTY output for screen capture on reattach

	listener net.Listener

	clientMu sync.Mutex
	writeMu  sync.Mutex
	client   net.Conn // current connected client (nil if none)

	done chan struct{}
}

// NewServer creates and starts a new server for the named session.
func NewServer(name string, cols, rows int) (*Server, error) {
	if err := EnsureSocketDir(); err != nil {
		return nil, fmt.Errorf("cannot create socket dir: %w", err)
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

	// Launch child: termdesk --app
	selfExe, err := os.Executable()
	if err != nil {
		ptmx.Close()
		tty.Close()
		return nil, fmt.Errorf("os.Executable: %w", err)
	}

	cmd := exec.Command(selfExe, "--app")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Env = buildAppEnv()
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

	// Listen on Unix socket
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		cmd.Process.Kill()
		ptmx.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}

	// Write PID file
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	return &Server{
		name:     name,
		sockPath: sockPath,
		pidPath:  pidPath,
		ptmx:     ptmx,
		cmd:      cmd,
		emu:      emu,
		listener: listener,
		done:     make(chan struct{}),
	}, nil
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

			// Forward to connected client
			s.clientMu.Lock()
			c := s.client
			s.clientMu.Unlock()
			if c != nil && len(data) > 0 {
				s.writeMu.Lock()
				writeErr := WriteMsg(c, MsgOutput, data)
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
			select {
			case <-s.done:
			default:
				close(s.done)
			}
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

		// Send terminal mode preamble + screen content for instant reattach.
		// The client's terminal starts fresh — we must re-enable alt screen,
		// mouse mode, and colors before sending the cell content.
		var preamble []byte
		preamble = append(preamble, "\x1b[?1049h"...)   // alt screen
		preamble = append(preamble, "\x1b[?1002h"...)   // mouse cell motion
		preamble = append(preamble, "\x1b[?1006h"...)   // SGR mouse encoding
		preamble = append(preamble, "\x1b[?25l"...)     // hide cursor (BT manages it)
		preamble = append(preamble, "\x1b[?2004h"...)   // bracketed paste
		preamble = append(preamble, s.emu.Render()...)
		s.writeMu.Lock()
		err = WriteMsg(conn, MsgRedraw, preamble)
		s.writeMu.Unlock()
		if err != nil {
			s.disconnectClient()
			continue
		}

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
			s.ptmx.Write(payload)

		case MsgResize:
			if len(payload) == 4 {
				rows, cols := DecodeResize(payload)
				pty.Setsize(s.ptmx, &pty.Winsize{
					Rows: rows,
					Cols: cols,
				})
				s.emu.Resize(int(cols), int(rows))
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

func (s *Server) cleanup() {
	s.listener.Close()
	s.disconnectClient()
	s.ptmx.Close()
	os.Remove(s.sockPath)
	os.Remove(s.pidPath)
}

// buildAppEnv creates a sanitized environment for the child --app process.
// Terminal-specific env vars (TERM_PROGRAM, KITTY_*, ITERM_*, etc.) are
// removed so that Bubble Tea v2 doesn't enable features like synchronized
// output or Kitty keyboard protocol that don't work through the PTY proxy.
func buildAppEnv() []string {
	skip := map[string]bool{
		"TERM": true, "COLORTERM": true,
		"TERM_PROGRAM": true, "TERM_PROGRAM_VERSION": true,
		"WT_SESSION": true, "VTE_VERSION": true,
	}
	var env []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if skip[k] || strings.HasPrefix(k, "KITTY_") || strings.HasPrefix(k, "ITERM_") {
			continue
		}
		env = append(env, e)
	}
	env = append(env,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"TERMDESK=1",
	)
	return env
}
