package session

import (
	"bufio"
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

	// Handle SIGWINCH for resize forwarding
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)

	done := make(chan error, 2)

	// Read from server, write to stdout
	go func() {
		done <- clientReadLoop(conn, os.Stdout)
	}()

	// Read from stdin, write to server (with detach key handling)
	go func() {
		done <- clientWriteLoop(os.Stdin, conn, sigWinch)
	}()

	err = <-done

	// Close connection first to stop both goroutines — prevents the read
	// goroutine from writing more ANSI data to stdout after our reset.
	signal.Stop(sigWinch)
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
	f.Write(seq)
}

var errDetached = fmt.Errorf("detached")

// clientReadLoop reads TLV messages from the server and writes output to the terminal.
// Uses buffered output with periodic flushing to minimize syscalls.
// A mutex protects the bufio.Writer since Flush runs from a timer goroutine.
func clientReadLoop(conn net.Conn, out *os.File) error {
	bw := bufio.NewWriterSize(out, 32768)
	var mu sync.Mutex

	// Flush periodically — coalesces many small writes into fewer large ones.
	// Uses a mutex because bufio.Writer is not thread-safe.
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			mu.Lock()
			bw.Flush()
			mu.Unlock()
		}
	}()

	for {
		typ, payload, err := ReadMsg(conn)
		if err != nil {
			mu.Lock()
			bw.Flush()
			mu.Unlock()
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch typ {
		case MsgOutput, MsgRedraw:
			mu.Lock()
			bw.Write(payload)
			mu.Unlock()
		}
	}
}

// clientWriteLoop reads from stdin and sends to the server. Handles SIGWINCH.
func clientWriteLoop(in *os.File, conn net.Conn, sigWinch <-chan os.Signal) error {
	buf := make([]byte, 4096)

	for {
		// Check for SIGWINCH between reads (non-blocking)
		select {
		case <-sigWinch:
			sendCurrentSize(conn)
		default:
		}

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
