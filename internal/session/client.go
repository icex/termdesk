package session

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// detachSeq is the F12 escape sequence (xterm: \x1b[24~).
var detachSeq = []byte("\x1b[24~")

// Attach connects to the named session and proxies I/O until detach or disconnect.
func Attach(name string) error {
	sockPath := SocketPath(name)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return fmt.Errorf("cannot connect to session %q: %w", name, err)
	}
	defer conn.Close()

	// Put terminal in raw mode
	fd := os.Stdin.Fd()
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Send initial terminal size
	sendCurrentSize(conn)

	// Handle SIGWINCH for resize forwarding
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	defer signal.Stop(sigWinch)

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

	// Restore terminal before printing any message
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

var errDetached = fmt.Errorf("detached")

// clientReadLoop reads TLV messages from the server and writes output to the terminal.
func clientReadLoop(conn net.Conn, out *os.File) error {
	for {
		typ, payload, err := ReadMsg(conn)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch typ {
		case MsgOutput, MsgRedraw:
			out.Write(payload)
		}
	}
}

// clientWriteLoop reads from stdin and sends to the server.
// Intercepts F12 (\x1b[24~) as the detach key. Handles SIGWINCH.
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
			data := buf[:n]

			// Check for F12 escape sequence
			if bytes.Contains(data, detachSeq) {
				WriteMsg(conn, MsgDetach, nil)
				return errDetached
			}

			WriteMsg(conn, MsgInput, data)
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
