package session

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
)

// Attach connects to the named session and proxies I/O until detach or disconnect.
func Attach(name string) error {
	sockPath := SocketPath(name)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return fmt.Errorf("cannot connect to session %q: %w", name, err)
	}

	// Put terminal in raw mode
	fd := os.Stdin.Fd()
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		conn.Close()
		return fmt.Errorf("raw mode: %w", err)
	}

	// Send initial terminal size
	sendCurrentSize(conn)

	// Probe the terminal for mode 2026 (synchronized output) support.
	// Must be done AFTER raw mode (so we can read the response) and
	// BEFORE starting the I/O goroutines (so the response isn't consumed
	// by clientWriteLoop).
	termHasSync := probeSync2026(os.Stdin, os.Stdout)

	// Handle SIGWINCH for resize forwarding
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)

	done := make(chan error, 2)

	// Read from server, write to stdout
	go func() {
		done <- clientReadLoop(conn, os.Stdout, termHasSync)
	}()

	// Handle SIGWINCH in its own goroutine so resize is forwarded
	// immediately, even when stdin.Read() is blocking (which it always
	// is when the user isn't typing). Without this, resize only takes
	// effect after the next keystroke/click.
	go func() {
		for range sigWinch {
			sendCurrentSize(conn)
		}
	}()

	// Read from stdin, write to server.
	// Note: clientWriteLoop may leak until process exit since os.Stdin.Read()
	// cannot be interrupted. This is acceptable as Attach() is the last call
	// before process exit in normal usage.
	go func() {
		done <- clientWriteLoop(os.Stdin, conn)
	}()

	err = <-done

	// Close connection first to stop both goroutines — prevents the read
	// goroutine from writing more ANSI data to stdout after our reset.
	signal.Stop(sigWinch)
	close(sigWinch) // unblocks the for-range goroutine
	conn.Close()

	// Give the read goroutine a moment to drain and exit
	time.Sleep(20 * time.Millisecond)

	// Reset terminal state: the BT app enables alt screen, mouse mode,
	// bracketed paste, and custom colors — we must undo all of that.
	resetTerminal(os.Stdout)

	// Restore terminal (raw → cooked mode)
	term.Restore(fd, oldState)

	if err == errDetached {
		fmt.Fprintf(os.Stderr, "\r\n[detached from session %q]\r\n", name)
		return nil
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\r\n[session %q ended]\r\n", name)
	return nil
}

// resetTerminal writes ANSI escape sequences to undo the modes enabled by
// the Bubble Tea app: alt screen, mouse tracking, bracketed paste, custom
// colors, and cursor visibility.
func resetTerminal(f *os.File) {
	var seq []byte
	seq = append(seq, "\x1b[?1049l"...)  // exit alt screen
	seq = append(seq, "\x1b[?1002l"...)  // disable mouse cell motion
	seq = append(seq, "\x1b[?1003l"...)  // disable mouse all motion
	seq = append(seq, "\x1b[?1006l"...)  // disable SGR mouse encoding
	seq = append(seq, "\x1b[?2004l"...)  // disable bracketed paste
	seq = append(seq, "\x1b[?25h"...)    // show cursor
	seq = append(seq, "\x1b[0m"...)      // reset all SGR attributes
	seq = append(seq, "\x1b]110\x07"...) // reset default foreground (OSC 110)
	seq = append(seq, "\x1b]111\x07"...) // reset default background (OSC 111)
	seq = append(seq, "\x1b[2J"...)      // clear screen
	seq = append(seq, "\x1b[H"...)       // move cursor to home
	f.Write(seq)
}

var errDetached = fmt.Errorf("detached")

// Synchronized output mode 2026 markers.
var (
	syncStart = []byte("\x1b[?2026h")
	syncEnd   = []byte("\x1b[?2026l")
)

// probeSync2026 queries the terminal for mode 2026 support via DECRQM.
// Returns true if the terminal responds positively within 100ms.
// Must be called in raw mode before I/O goroutines start.
// Accumulates reads until the full DECRQM response ($y) is found or the
// deadline expires, handling partial responses on high-latency connections.
func probeSync2026(in *os.File, out *os.File) bool {
	// Send DECRQM query for mode 2026
	out.Write([]byte("\x1b[?2026$p"))

	// Read response with timeout. Expected: \x1b[?2026;N$y
	// where N=1(set), 2(reset), 3(permanently set) all mean supported.
	// Use SetReadDeadline on the TTY fd to avoid orphaning a goroutine.
	// TTYs support deadline-based I/O via kqueue/epoll on macOS/Linux.
	var resp []byte
	buf := make([]byte, 64)
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		in.SetReadDeadline(deadline)
		n, err := in.Read(buf)
		if n > 0 {
			resp = append(resp, buf[:n]...)
			if bytes.Contains(resp, []byte("$y")) {
				break
			}
		}
		if err != nil {
			break
		}
	}
	in.SetReadDeadline(time.Time{}) // clear deadline

	if len(resp) > 0 {
		// Any valid DECRQM response means the terminal knows mode 2026.
		// Value 0 = not recognized, anything else = supported.
		if bytes.Contains(resp, []byte("2026")) && bytes.Contains(resp, []byte("$y")) {
			if !bytes.Contains(resp, []byte(";0$y")) {
				return true
			}
		}
	}
	return false
}

// clientReadLoop dispatches to sync or basic read loop based on terminal capability.
func clientReadLoop(conn net.Conn, out *os.File, termHasSync bool) error {
	if termHasSync {
		return clientReadLoopSync(conn, out)
	}
	return clientReadLoopBasic(conn, out)
}

// clientReadLoopSync is the fast path for terminals that support mode 2026.
// Passes BT's output through as-is (including native sync markers) with
// frame-aligned flushing. The terminal handles sync markers natively.
// An 8ms ticker acts as safety valve for non-frame data.
func clientReadLoopSync(conn net.Conn, out *os.File) error {
	var buf bytes.Buffer
	var mu sync.Mutex

	ticker := time.NewTicker(8 * time.Millisecond)
	defer ticker.Stop()
	tickerDone := make(chan struct{})
	defer close(tickerDone)
	go func() {
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				if buf.Len() > 0 {
					buf.WriteTo(out)
				}
				mu.Unlock()
			case <-tickerDone:
				return
			}
		}
	}()

	for {
		typ, payload, err := ReadMsg(conn)
		if err != nil {
			mu.Lock()
			buf.WriteTo(out)
			mu.Unlock()
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch typ {
		case MsgOutput, MsgRedraw:
			mu.Lock()
			buf.Write(payload)
			// Flush on frame boundary — sync end marker means BT
			// completed a frame. Pass through as-is; the terminal
			// handles mode 2026 markers natively.
			if bytes.Contains(payload, syncEnd) {
				buf.WriteTo(out)
			}
			mu.Unlock()
		case MsgDetach:
			mu.Lock()
			buf.WriteTo(out)
			mu.Unlock()
			return errDetached
		}
	}
}

// clientReadLoopBasic is the path for terminals without mode 2026 (e.g. Termux).
// Rate-limited vsync: uses BT's sync markers as frame boundary hints and
// enforces a minimum interval between flushes. Rapid frames get coalesced
// into one write, reducing flicker over slow transports like SSH.
func clientReadLoopBasic(conn net.Conn, out *os.File) error {
	var buf bytes.Buffer
	var mu sync.Mutex
	var lastFlush time.Time

	const minInterval = 30 * time.Millisecond // ~33fps cap

	// Safety-valve ticker: flush held data that wasn't flushed by frame
	// boundaries (startup sequences, query responses, rate-limited frames).
	ticker := time.NewTicker(minInterval)
	defer ticker.Stop()
	tickerDone := make(chan struct{})
	defer close(tickerDone)
	go func() {
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				basicFlush(&buf, out, &lastFlush)
				mu.Unlock()
			case <-tickerDone:
				return
			}
		}
	}()

	for {
		typ, payload, err := ReadMsg(conn)
		if err != nil {
			mu.Lock()
			basicFlush(&buf, out, &lastFlush)
			mu.Unlock()
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch typ {
		case MsgOutput, MsgRedraw:
			mu.Lock()
			buf.Write(payload)
			// Frame-aligned + rate-limited flush: only flush when we have
			// a complete frame AND enough time has passed since last flush.
			// This coalesces rapid frame bursts into single writes.
			if bytes.Contains(buf.Bytes(), syncEnd) {
				if time.Since(lastFlush) >= minInterval {
					basicFlush(&buf, out, &lastFlush)
				}
				// else: hold it — ticker will pick it up
			}
			mu.Unlock()
		case MsgDetach:
			mu.Lock()
			basicFlush(&buf, out, &lastFlush)
			mu.Unlock()
			return errDetached
		}
	}
}

// basicFlush strips sync markers and writes accumulated data in one call.
func basicFlush(buf *bytes.Buffer, out *os.File, lastFlush *time.Time) {
	if buf.Len() == 0 {
		return
	}
	data := buf.Bytes()
	clean := bytes.ReplaceAll(data, syncStart, nil)
	clean = bytes.ReplaceAll(clean, syncEnd, nil)
	if len(clean) > 0 {
		out.Write(clean)
	}
	buf.Reset()
	*lastFlush = time.Now()
}

// clientWriteLoop reads from stdin and sends to the server.
func clientWriteLoop(in *os.File, conn net.Conn) error {
	buf := make([]byte, 4096)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			WriteMsg(conn, MsgInput, buf[:n])
		}
		if err != nil {
			return err
		}
	}
}

func sendCurrentSize(conn net.Conn) {
	w, h, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		return
	}
	WriteMsg(conn, MsgResize, EncodeResize(uint16(h), uint16(w)))
}
