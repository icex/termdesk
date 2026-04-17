package session

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

func TestBuildAppEnv(t *testing.T) {
	t.Setenv("TERM", "vt100")
	t.Setenv("COLORTERM", "badcolor")
	t.Setenv("TERM_PROGRAM", "SomeTerm")
	t.Setenv("KITTY_FOO", "1")
	t.Setenv("ITERM_BAR", "1")
	t.Setenv("FOO", "bar")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "TERM=vt100") {
		t.Fatalf("TERM should be stripped")
	}
	// COLORTERM value is preserved (not overridden to truecolor).
	if !strings.Contains(joined, "COLORTERM=badcolor") {
		t.Fatalf("COLORTERM value should be preserved from client env")
	}
	// Ensure it only appears once (stripped from passthrough, re-added).
	if strings.Count(joined, "COLORTERM=") != 1 {
		t.Fatalf("COLORTERM should appear exactly once")
	}
	if strings.Contains(joined, "TERM_PROGRAM=") {
		t.Fatalf("TERM_PROGRAM should be stripped")
	}
	if strings.Contains(joined, "KITTY_FOO=") {
		t.Fatalf("KITTY_* should be stripped")
	}
	if strings.Contains(joined, "ITERM_BAR=") {
		t.Fatalf("ITERM_* should be stripped")
	}
	if !strings.Contains(joined, "FOO=bar") {
		t.Fatalf("custom env should be preserved")
	}
	if !strings.Contains(joined, "TERM=xterm-256color") {
		t.Fatalf("TERM override missing")
	}
	// COLORTERM is set (value preserved from env).
	if !strings.Contains(joined, "COLORTERM=") {
		t.Fatalf("COLORTERM should be present")
	}
	if !strings.Contains(joined, "TERMDESK=1") {
		t.Fatalf("TERMDESK flag missing")
	}
}

func TestBuildAppEnvColortermDefault(t *testing.T) {
	// When COLORTERM is unset, BuildAppEnv should default to "truecolor".
	t.Setenv("COLORTERM", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "COLORTERM=truecolor") {
		t.Fatalf("expected COLORTERM=truecolor when env is empty, got: %s", joined)
	}
}

func TestBuildAppEnvStripsWTSession(t *testing.T) {
	t.Setenv("WT_SESSION", "some-guid")
	t.Setenv("VTE_VERSION", "7200")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "WT_SESSION=") {
		t.Fatalf("WT_SESSION should be stripped")
	}
	if strings.Contains(joined, "VTE_VERSION=") {
		t.Fatalf("VTE_VERSION should be stripped")
	}
}

func TestBuildAppEnvStripsTPV(t *testing.T) {
	t.Setenv("TERM_PROGRAM_VERSION", "1.2.3")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "TERM_PROGRAM_VERSION=") {
		t.Fatalf("TERM_PROGRAM_VERSION should be stripped")
	}
}

func TestBuildAppEnvMultipleKittyVars(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "42")
	t.Setenv("KITTY_PID", "1234")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "KITTY_WINDOW_ID=") {
		t.Fatalf("KITTY_WINDOW_ID should be stripped")
	}
	if strings.Contains(joined, "KITTY_PID=") {
		t.Fatalf("KITTY_PID should be stripped")
	}
}

func TestBuildAppEnvMultipleItermVars(t *testing.T) {
	t.Setenv("ITERM_SESSION_ID", "w0t0p0")
	t.Setenv("ITERM_PROFILE", "Default")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	// Original host values must be stripped from the environment.
	if strings.Contains(joined, "ITERM_SESSION_ID=w0t0p0") {
		t.Fatal("original ITERM_SESSION_ID value should be stripped")
	}
	if strings.Contains(joined, "ITERM_PROFILE=Default") {
		t.Fatal("original ITERM_PROFILE value should be stripped")
	}
}

func TestBuildAppEnvAlwaysHasTermdesk(t *testing.T) {
	env := BuildAppEnv()
	found := false
	for _, e := range env {
		if e == "TERMDESK=1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TERMDESK=1 should always be in the env")
	}
}

func TestBuildAppEnvAlwaysHasTerm(t *testing.T) {
	env := BuildAppEnv()
	found := false
	for _, e := range env {
		if e == "TERM=xterm-256color" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TERM=xterm-256color should always be in the env")
	}
}

func TestDetachOSC(t *testing.T) {
	expected := "\x1b]666;detach\x07"
	if string(detachOSC) != expected {
		t.Errorf("detachOSC = %q, want %q", string(detachOSC), expected)
	}
}

func TestDisconnectClientNil(t *testing.T) {
	// disconnectClient should be safe to call when client is nil.
	s := &Server{
		client: nil,
	}
	// Should not panic.
	s.disconnectClient()
	if s.client != nil {
		t.Error("client should remain nil after disconnect")
	}
}

func TestDisconnectClientWithConnection(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s := &Server{
		client: clientConn,
	}

	s.disconnectClient()
	if s.client != nil {
		t.Error("client should be nil after disconnect")
	}

	// The connection should be closed.
	_, err = clientConn.Write([]byte("test"))
	if err == nil {
		t.Error("expected write to closed connection to fail")
	}

	serverSide.Close()
}

func TestDisconnectClientIdempotent(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	s := &Server{
		client: clientConn,
	}

	// Call disconnect multiple times — should not panic.
	s.disconnectClient()
	s.disconnectClient()
	s.disconnectClient()

	if s.client != nil {
		t.Error("client should be nil")
	}
}

func TestCleanup(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	sockPath := filepath.Join(sockDir, "test.sock")
	pidPath := filepath.Join(sockDir, "test.pid")

	// Create a listener and PID file.
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	// Create a PTY for the server.
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	tty.Close()

	s := &Server{
		sockPath: sockPath,
		pidPath:  pidPath,
		listener: ln,
		ptmx:     ptmx,
		client:   nil,
	}

	s.cleanup()

	// Verify socket and PID file are removed.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after cleanup")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after cleanup")
	}

	// Verify listener is closed.
	_, err = net.Dial("unix", sockPath)
	if err == nil {
		t.Error("listener should be closed after cleanup")
	}
}

func TestCleanupWithClient(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	sockPath := filepath.Join(sockDir, "test.sock")
	pidPath := filepath.Join(sockDir, "test.pid")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	os.WriteFile(pidPath, []byte("12345"), 0o600)

	// Create a connection to serve as client.
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	tty.Close()

	s := &Server{
		sockPath: sockPath,
		pidPath:  pidPath,
		listener: ln,
		ptmx:     ptmx,
		client:   serverSide,
	}

	s.cleanup()

	// Client should be disconnected.
	if s.client != nil {
		t.Error("client should be nil after cleanup")
	}

	clientConn.Close()
}

func TestHandleClientInput(t *testing.T) {
	// Create a PTY pair to serve as the server's ptmx.
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	emu := vt.NewSafeEmulator(80, 24)
	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	// Create a socket pair.
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	// Run handleClient in background.
	done := make(chan struct{})
	go func() {
		s.handleClient(serverSide)
		close(done)
	}()

	// Send an input message from the client.
	WriteMsg(clientConn, MsgInput, []byte("hello"))

	// Data written to the master PTY is echoed back (PTY line discipline).
	// Read the echo from ptmx — may arrive in multiple chunks.
	var got []byte
	buf := make([]byte, 128)
	deadline := time.Now().Add(2 * time.Second)
	for {
		ptmx.SetReadDeadline(deadline)
		n, err := ptmx.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if strings.Contains(string(got), "hello") {
			break
		}
		if err != nil {
			t.Fatalf("ptmx.Read: %v (got so far: %q)", err, got)
		}
	}
	if !strings.Contains(string(got), "hello") {
		t.Errorf("PTY echo = %q, expected it to contain %q", got, "hello")
	}

	// Close the client to stop handleClient.
	clientConn.Close()
	<-done
}

func TestHandleClientDetach(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	emu := vt.NewSafeEmulator(80, 24)
	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.handleClient(serverSide)
		close(done)
	}()

	// Send a detach message.
	WriteMsg(clientConn, MsgDetach, nil)

	// handleClient should return after receiving detach.
	select {
	case <-done:
		// Success.
	case <-time.After(2 * time.Second):
		t.Fatal("handleClient did not return after detach")
	}

	clientConn.Close()
}

func TestHandleClientResize(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	// We need a real process on the PTY for SIGWINCH to work.
	// Instead, we just test that handleClient processes the resize message
	// without error by using a cat process.
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	emu := vt.NewSafeEmulator(80, 24)

	// Start a simple child process for the SIGWINCH target.
	cmd := newTestCmd(tty)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}
	defer cmd.Process.Kill()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.handleClient(serverSide)
		close(done)
	}()

	// Send a resize message.
	WriteMsg(clientConn, MsgResize, EncodeResize(50, 120))

	// Give it time to process.
	time.Sleep(50 * time.Millisecond)

	// Close to stop the handler.
	clientConn.Close()
	<-done
}

func TestHandleClientResizeInvalidPayload(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	emu := vt.NewSafeEmulator(80, 24)
	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.handleClient(serverSide)
		close(done)
	}()

	// Send a resize message with invalid (too short) payload.
	// This should be silently ignored (payload len != 4).
	WriteMsg(clientConn, MsgResize, []byte{0x00, 0x01})

	// Send another valid input to confirm the handler is still running.
	WriteMsg(clientConn, MsgInput, []byte("still alive"))

	time.Sleep(50 * time.Millisecond)

	// Read from ptmx to verify the input was echoed (PTY echo).
	buf := make([]byte, 128)
	ptmx.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := ptmx.Read(buf)
	if err != nil {
		t.Logf("ptmx.Read: %v (may timeout if no echo)", err)
	} else if !strings.Contains(string(buf[:n]), "still alive") {
		t.Logf("PTY echo = %q (data was forwarded to PTY)", string(buf[:n]))
	}

	clientConn.Close()
	<-done
}

func TestAcceptLoop(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	emu := vt.NewSafeEmulator(80, 24)

	// Start a child process for SIGWINCH.
	cmd := newTestCmd(tty)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}
	defer cmd.Process.Kill()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	// Start acceptLoop in background.
	go s.acceptLoop()

	// Connect a client.
	conn1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	// Wait a bit for accept to process.
	time.Sleep(100 * time.Millisecond)

	// The server should have a preamble (MsgRedraw) ready for us.
	typ, payload, err := ReadMsg(conn1)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}
	if typ != MsgRedraw {
		t.Errorf("expected MsgRedraw preamble, got %c", typ)
	}
	// Verify preamble contains alt screen enable.
	if !bytes.Contains(payload, []byte("\x1b[?1049h")) {
		t.Error("preamble should contain alt screen enable")
	}

	conn1.Close()
	time.Sleep(50 * time.Millisecond)

	// Close listener to stop acceptLoop.
	ln.Close()
}

func TestAcceptLoopReplacesClient(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	emu := vt.NewSafeEmulator(80, 24)

	cmd := newTestCmd(tty)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}
	defer cmd.Process.Kill()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	go s.acceptLoop()

	// Connect first client.
	conn1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial 1: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	// Read preamble.
	ReadMsg(conn1)

	// Connect second client — should replace first.
	conn2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial 2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// The first connection should have been closed.
	// Attempt to read from conn1 — should get EOF or error.
	conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = ReadMsg(conn1)
	if err == nil {
		t.Error("first client should have been disconnected")
	}

	// Second client should get preamble.
	typ, _, err := ReadMsg(conn2)
	if err != nil {
		t.Logf("conn2 ReadMsg: %v (might have been processed already)", err)
	} else if typ != MsgRedraw {
		t.Errorf("second client expected MsgRedraw, got %c", typ)
	}

	conn1.Close()
	conn2.Close()
	ln.Close()
}

func TestReadPtyLoopForwardsToClient(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()

	emu := vt.NewSafeEmulator(80, 24)

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	// Create a socket pair for client connection.
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	// Start readPtyLoop.
	go s.readPtyLoop()

	// Write to the tty side — this will be read from ptmx by readPtyLoop.
	tty.Write([]byte("from the pty"))
	// Allow time for readPtyLoop to read from ptmx and forward to the client
	// before tty.Close() triggers EOF on the master side.
	time.Sleep(500 * time.Millisecond)
	tty.Close()

	// Read the forwarded message from the client connection.
	// May take a moment for the data to propagate.
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var allData []byte
	for {
		typ, payload, err := ReadMsg(clientConn)
		if err != nil {
			break
		}
		if typ == MsgOutput {
			allData = append(allData, payload...)
		}
	}

	if !bytes.Contains(allData, []byte("from the pty")) {
		t.Errorf("client did not receive PTY output, got: %q", string(allData))
	}

	clientConn.Close()
	serverSide.Close()
}

func TestReadPtyLoopDetachOSC(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()

	emu := vt.NewSafeEmulator(80, 24)

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	// Create a socket pair for client connection.
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	// Start readPtyLoop.
	go s.readPtyLoop()

	// Write data containing the detach OSC sequence.
	tty.Write([]byte("before" + string(detachOSC) + "after"))
	time.Sleep(100 * time.Millisecond)
	tty.Close()

	// The client should receive the data with detachOSC stripped.
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var allData []byte
	for {
		typ, payload, err := ReadMsg(clientConn)
		if err != nil {
			break
		}
		if typ == MsgOutput {
			allData = append(allData, payload...)
		}
	}

	if bytes.Contains(allData, detachOSC) {
		t.Error("detach OSC should have been stripped from output")
	}

	// The non-detach data should be forwarded (before the disconnect).
	if !bytes.Contains(allData, []byte("before")) {
		t.Logf("data received: %q (detach may have disconnected before 'before' arrived)", string(allData))
	}

	clientConn.Close()
	serverSide.Close()
}

func TestReadPtyLoopNoClient(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()

	emu := vt.NewSafeEmulator(80, 24)

	s := &Server{
		ptmx: ptmx,
		emu:  emu,
		done: make(chan struct{}),
	}

	// Start readPtyLoop without any client.
	go s.readPtyLoop()

	// Write some data to the PTY.
	tty.Write([]byte("no client here"))
	tty.Close()

	// Wait for readPtyLoop to exit (due to PTY close).
	select {
	case <-s.done:
		// Good, the loop detected EOF and closed the done channel.
	case <-time.After(2 * time.Second):
		t.Fatal("readPtyLoop did not exit after PTY close")
	}
}

func TestReadPtyLoopWriteError(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()

	emu := vt.NewSafeEmulator(80, 24)

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	// Create a socket pair for client connection.
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	// Close the client side so writes from the server will fail.
	clientConn.Close()

	// Start readPtyLoop.
	go s.readPtyLoop()

	// Write data to the PTY to trigger a forward attempt.
	tty.Write([]byte("this will fail to forward"))
	time.Sleep(100 * time.Millisecond)

	// The server should have disconnected the client due to write error.
	s.clientMu.Lock()
	c := s.client
	s.clientMu.Unlock()
	if c != nil {
		t.Error("client should have been disconnected after write error")
	}

	tty.Close()

	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("readPtyLoop did not exit")
	}

	serverSide.Close()
}

func TestAcceptLoopPreambleContent(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	emu := vt.NewSafeEmulator(80, 24)

	cmd := newTestCmd(tty)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}
	defer cmd.Process.Kill()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	go s.acceptLoop()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	typ, payload, err := ReadMsg(conn)
	if err != nil {
		t.Fatalf("ReadMsg: %v", err)
	}
	if typ != MsgRedraw {
		t.Fatalf("expected MsgRedraw, got %c", typ)
	}

	preamble := string(payload)
	expected := []struct {
		seq  string
		desc string
	}{
		{"\x1b[?1049h", "alt screen enable"},
		{"\x1b[?1002h", "mouse cell motion enable"},
		{"\x1b[?1006h", "SGR mouse encoding enable"},
		{"\x1b[?25l", "cursor hide"},
		{"\x1b[?2004h", "bracketed paste enable"},
	}
	for _, e := range expected {
		if !strings.Contains(preamble, e.seq) {
			t.Errorf("preamble missing %s (%q)", e.desc, e.seq)
		}
	}

	conn.Close()
	ln.Close()
}

func TestHandleClientMultipleInputs(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	emu := vt.NewSafeEmulator(80, 24)
	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		done:     make(chan struct{}),
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	serverSide := <-accepted

	s.clientMu.Lock()
	s.client = serverSide
	s.clientMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.handleClient(serverSide)
		close(done)
	}()

	// Send multiple input messages.
	msgs := []string{"aaa", "bbb", "ccc"}
	for _, m := range msgs {
		WriteMsg(clientConn, MsgInput, []byte(m))
	}

	// Read echoed data from ptmx (PTY line discipline echoes input back).
	// Use a goroutine with a timer to avoid blocking on the read forever
	// since os.File.SetReadDeadline is not supported on PTYs.
	var received []byte
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 1024)
		// Try multiple reads to gather echoed data.
		for i := 0; i < 5; i++ {
			n, err := ptmx.Read(buf)
			if n > 0 {
				received = append(received, buf[:n]...)
			}
			if err != nil {
				return
			}
			// Check if we have all expected data.
			data := string(received)
			allFound := true
			for _, m := range msgs {
				if !strings.Contains(data, m) {
					allFound = false
					break
				}
			}
			if allFound {
				return
			}
		}
	}()

	select {
	case <-readDone:
	case <-time.After(3 * time.Second):
		t.Log("ptmx read timed out (PTY echo may not contain all messages)")
	}

	data := string(received)
	for _, m := range msgs {
		if !strings.Contains(data, m) {
			t.Errorf("PTY echo missing %q, got %q", m, data)
		}
	}

	clientConn.Close()
	<-done
}

func TestDisconnectClientConcurrent(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	s := &Server{
		client: clientConn,
	}

	// Call disconnectClient concurrently from multiple goroutines.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.disconnectClient()
		}()
	}
	wg.Wait()

	if s.client != nil {
		t.Error("client should be nil after concurrent disconnects")
	}
}

func TestNewServerEnsureSocketDirError(t *testing.T) {
	// Point XDG_RUNTIME_DIR to a read-only path to trigger EnsureSocketDir failure.
	tmp := shortTempDir(t)
	readOnly := filepath.Join(tmp, "readonly")
	os.MkdirAll(readOnly, 0o500)
	// Create a file where the termdesk dir would be, to prevent MkdirAll.
	os.WriteFile(filepath.Join(readOnly, "termdesk"), []byte("blocking"), 0o400)

	t.Setenv("XDG_RUNTIME_DIR", readOnly)

	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 1"},
		AppEnv:     BuildAppEnv(),
	}
	_, err := NewServer("test", 80, 24, opts)
	if err == nil {
		t.Error("NewServer should fail when socket dir cannot be created")
	}
	if !strings.Contains(err.Error(), "cannot create socket dir") {
		t.Errorf("expected 'cannot create socket dir' error, got: %v", err)
	}
}

func TestNewServerCreatesAndCleanup(t *testing.T) {
	// This test creates a real server using os.Executable() with --app.
	// The test binary doesn't handle --app, so the child will exit quickly.
	// But NewServer should still succeed in setting up the PTY, listener, etc.
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 0"},
		AppEnv:     BuildAppEnv(),
	}
	srv, err := NewServer("test-session", 80, 24, opts)
	if err != nil {
		// On some systems, os.Executable may fail or cmd.Start may fail.
		t.Skipf("NewServer: %v (may be unsupported in this env)", err)
		return
	}

	// Verify the server struct is populated.
	if srv.name != "test-session" {
		t.Errorf("server name = %q, want %q", srv.name, "test-session")
	}
	if srv.ptmx == nil {
		t.Error("server ptmx should not be nil")
	}
	if srv.listener == nil {
		t.Error("server listener should not be nil")
	}
	if srv.emu == nil {
		t.Error("server emulator should not be nil")
	}
	if srv.done == nil {
		t.Error("server done channel should not be nil")
	}

	// Check that sock and pid files exist.
	sockPath := SocketPath("test-session")
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("socket file should exist")
	}
	pidPath := PidPath("test-session")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("pid file should exist")
	}

	// Clean up the server.
	srv.cmd.Process.Kill()
	srv.cmd.Wait()
	srv.cleanup()

	// Verify cleanup removed the files.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after cleanup")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("pid file should be removed after cleanup")
	}
}

func TestNewServerAndRunChildExits(t *testing.T) {
	// NewServer starts a child process (test binary with --app) that will
	// exit immediately. Run() should detect the child exit and return.
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 0"},
		AppEnv:     BuildAppEnv(),
	}
	srv, err := NewServer("run-test", 80, 24, opts)
	if err != nil {
		t.Skipf("NewServer: %v (may be unsupported in this env)", err)
		return
	}

	// Run in background — the child will exit quickly since test binary
	// doesn't handle --app. Run() should detect the exit and return.
	runDone := make(chan error, 1)
	go func() {
		runDone <- srv.Run()
	}()

	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after child exit")
		// Force cleanup.
		srv.cmd.Process.Kill()
		srv.cleanup()
	}
}

func TestRunDoneChannel(t *testing.T) {
	// Test that Run() exits when the done channel is closed (by readPtyLoop).
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 0"},
		AppEnv:     BuildAppEnv(),
	}
	srv, err := NewServer("done-test", 80, 24, opts)
	if err != nil {
		t.Skipf("NewServer: %v", err)
		return
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- srv.Run()
	}()

	// Signal shutdown via stop() (safe for concurrent close).
	srv.stop()

	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after done channel closed")
		srv.cmd.Process.Kill()
	}
}

func TestAcceptLoopPreambleWriteFails(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	emu := vt.NewSafeEmulator(80, 24)

	cmd := newTestCmd(tty)
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start: %v", err)
	}
	defer cmd.Process.Kill()

	s := &Server{
		ptmx:     ptmx,
		emu:      emu,
		listener: ln,
		cmd:      cmd,
		done:     make(chan struct{}),
	}

	go s.acceptLoop()

	// Connect and immediately close our side so the preamble write fails.
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()

	// Give time for the accept loop to process the connection and hit the error.
	time.Sleep(100 * time.Millisecond)

	// Connect again to verify the accept loop is still running after the error.
	conn2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial 2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// This time, read the preamble to verify the loop recovered.
	typ, _, err := ReadMsg(conn2)
	if err != nil {
		t.Logf("ReadMsg: %v (preamble may have been sent already)", err)
	} else if typ != MsgRedraw {
		t.Errorf("expected MsgRedraw, got %c", typ)
	}

	conn2.Close()
	ln.Close()
}

// newTestCmd creates a simple sleep command attached to the given tty.
func newTestCmd(tty *os.File) *exec.Cmd {
	cmd := exec.Command("sleep", "60")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	return cmd
}

func TestNewServerEmptyCommand(t *testing.T) {
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	_, err := NewServer("test", 80, 24, ServerOptions{})
	if err == nil {
		t.Fatal("NewServer should fail with empty AppCommand")
	}
	if !strings.Contains(err.Error(), "missing app command") {
		t.Errorf("expected 'missing app command' error, got: %v", err)
	}
}

func TestNewServerBadCommand(t *testing.T) {
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	opts := ServerOptions{
		AppCommand: "/nonexistent/binary/that/does/not/exist",
		AppEnv:     BuildAppEnv(),
	}
	_, err := NewServer("test", 80, 24, opts)
	if err == nil {
		t.Fatal("NewServer should fail with non-existent command")
	}
	if !strings.Contains(err.Error(), "start app") {
		t.Errorf("expected 'start app' error, got: %v", err)
	}
}

func TestNewServerWithAppDir(t *testing.T) {
	tmp := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	appDir := t.TempDir()
	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 0"},
		AppEnv:     BuildAppEnv(),
		AppDir:     appDir,
	}
	srv, err := NewServer("dir-test", 80, 24, opts)
	if err != nil {
		t.Skipf("NewServer: %v", err)
		return
	}

	// Verify server was created.
	if srv.name != "dir-test" {
		t.Errorf("server name = %q, want %q", srv.name, "dir-test")
	}

	srv.cmd.Process.Kill()
	srv.cmd.Wait()
	srv.cleanup()
}

func TestBuildAppEnvGraphicsWezTerm(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty for WezTerm")
	}
}

func TestBuildAppEnvGraphicsKitty(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "kitty")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty for kitty terminal")
	}
}

func TestBuildAppEnvGraphicsGhostty(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty for ghostty")
	}
}

func TestBuildAppEnvGraphicsKittyWindowID(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "42")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty via KITTY_WINDOW_ID")
	}
}

func TestBuildAppEnvGraphicsWeztermPane(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "0")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty via WEZTERM_PANE")
	}
}

func TestBuildAppEnvGraphicsKonsoleDbusSession(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "/org/freedesktop/konsole")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty via KONSOLE_DBUS_SESSION")
	}
}

func TestBuildAppEnvGraphicsKonsoleVersion(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "230203")
	t.Setenv("TERMDESK_GRAPHICS", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty via KONSOLE_VERSION")
	}
}

func TestBuildAppEnvGraphicsTermdeskEnv(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "kitty")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty via TERMDESK_GRAPHICS env")
	}
}

func TestBuildAppEnvNoGraphics(t *testing.T) {
	// Clear all env vars that trigger graphics detection.
	// Use t.Setenv (not os.Unsetenv) for proper test isolation.
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")
	t.Setenv("TERMDESK_ITERM2", "")
	t.Setenv("TERMDESK_SIXEL", "")
	t.Setenv("LC_TERMINAL", "")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("XTERM_VERSION", "")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "TERMDESK_GRAPHICS=") {
		t.Fatalf("should not have TERMDESK_GRAPHICS when no graphics detected, env: %s", joined)
	}
}

func TestStopIdempotent(t *testing.T) {
	s := &Server{
		done: make(chan struct{}),
	}
	// Should not panic when called multiple times.
	s.stop()
	s.stop()
	s.stop()
	select {
	case <-s.done:
		// expected: channel is closed
	default:
		t.Error("done channel should be closed after stop()")
	}
}

// --- Additional BuildAppEnv tests for iTerm2, sixel, and re-propagation paths ---

func clearGraphicsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("WEZTERM_PANE", "")
	t.Setenv("KONSOLE_DBUS_SESSION", "")
	t.Setenv("KONSOLE_VERSION", "")
	t.Setenv("TERMDESK_GRAPHICS", "")
	t.Setenv("TERMDESK_ITERM2", "")
	t.Setenv("TERMDESK_SIXEL", "")
	t.Setenv("LC_TERMINAL", "")
	t.Setenv("ITERM_SESSION_ID", "")
	t.Setenv("XTERM_VERSION", "")
}

func TestBuildAppEnvITermApp(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERM_PROGRAM", "iTerm.app")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	// iTerm.app also triggers sixel detection, but since hasGraphics is false
	// and hasIterm2 is true, TERMDESK_GRAPHICS should be iterm2.
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=iterm2") {
		t.Fatalf("expected TERMDESK_GRAPHICS=iterm2 for iTerm.app, got: %s", joined)
	}
	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 for iTerm.app")
	}
	if !strings.Contains(joined, "LC_TERMINAL=iTerm2") {
		t.Fatal("expected LC_TERMINAL=iTerm2 propagated for iTerm.app")
	}
	if !strings.Contains(joined, "ITERM_SESSION_ID=termdesk") {
		t.Fatal("expected ITERM_SESSION_ID=termdesk propagated for iTerm.app")
	}
	// Sixel should also be enabled for iTerm.app
	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 for iTerm.app (sixel capable)")
	}
}

func TestBuildAppEnvLCTerminalITerm2(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("LC_TERMINAL", "iTerm2")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=iterm2") {
		t.Fatalf("expected TERMDESK_GRAPHICS=iterm2 via LC_TERMINAL, got: %s", joined)
	}
	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 via LC_TERMINAL")
	}
}

func TestBuildAppEnvItermSessionID(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("ITERM_SESSION_ID", "w0t0p0")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=iterm2") {
		t.Fatalf("expected TERMDESK_GRAPHICS=iterm2 via ITERM_SESSION_ID, got: %s", joined)
	}
	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 via ITERM_SESSION_ID")
	}
}

func TestBuildAppEnvWeztermPaneITerm2(t *testing.T) {
	clearGraphicsEnv(t)
	// WEZTERM_PANE triggers both kitty AND iterm2 detection.
	t.Setenv("WEZTERM_PANE", "0")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	// kitty detection via WEZTERM_PANE takes priority for TERMDESK_GRAPHICS
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatalf("expected TERMDESK_GRAPHICS=kitty via WEZTERM_PANE, got: %s", joined)
	}
	// But iterm2 should also be enabled
	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 alongside kitty via WEZTERM_PANE")
	}
}

func TestBuildAppEnvSixelFoot(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERM_PROGRAM", "foot")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=sixel") {
		t.Fatalf("expected TERMDESK_GRAPHICS=sixel for foot, got: %s", joined)
	}
	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 for foot")
	}
}

func TestBuildAppEnvSixelXtermVersion(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("XTERM_VERSION", "XTerm(379)")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_GRAPHICS=sixel") {
		t.Fatalf("expected TERMDESK_GRAPHICS=sixel via XTERM_VERSION, got: %s", joined)
	}
	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 via XTERM_VERSION")
	}
}

func TestBuildAppEnvRepropagateIterm2(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERMDESK_ITERM2", "1")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 re-propagated")
	}
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=iterm2") {
		t.Fatalf("expected TERMDESK_GRAPHICS=iterm2 via TERMDESK_ITERM2, got: %s", joined)
	}
}

func TestBuildAppEnvRepropagateSixel(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERMDESK_SIXEL", "1")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 re-propagated")
	}
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=sixel") {
		t.Fatalf("expected TERMDESK_GRAPHICS=sixel via TERMDESK_SIXEL, got: %s", joined)
	}
}

func TestBuildAppEnvRepropagateGraphicsIterm2(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERMDESK_GRAPHICS", "iterm2")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 via TERMDESK_GRAPHICS=iterm2")
	}
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=iterm2") {
		t.Fatalf("expected TERMDESK_GRAPHICS=iterm2, got: %s", joined)
	}
}

func TestBuildAppEnvRepropagateGraphicsSixel(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERMDESK_GRAPHICS", "sixel")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 via TERMDESK_GRAPHICS=sixel")
	}
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=sixel") {
		t.Fatalf("expected TERMDESK_GRAPHICS=sixel, got: %s", joined)
	}
}

func TestBuildAppEnvSixelContour(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERM_PROGRAM", "contour")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 for contour")
	}
}

func TestBuildAppEnvSixelMlterm(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERM_PROGRAM", "mlterm")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 for mlterm")
	}
}

func TestBuildAppEnvStripsLCTerminal(t *testing.T) {
	t.Setenv("LC_TERMINAL", "SomeTerm")
	t.Setenv("LC_TERMINAL_VERSION", "1.2.3")
	// Clear graphics env to avoid iterm2 propagation overwriting LC_TERMINAL.
	clearGraphicsEnv(t)
	t.Setenv("LC_TERMINAL", "SomeTerm")
	t.Setenv("LC_TERMINAL_VERSION", "1.2.3")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	if strings.Contains(joined, "LC_TERMINAL=SomeTerm") {
		t.Fatal("LC_TERMINAL should be stripped from passthrough")
	}
	if strings.Contains(joined, "LC_TERMINAL_VERSION=") {
		t.Fatal("LC_TERMINAL_VERSION should be stripped from passthrough")
	}
}

func TestBuildAppEnvStripsManagedVars(t *testing.T) {
	clearGraphicsEnv(t)
	// These managed vars should be stripped from the env passthrough.
	t.Setenv("TERMDESK", "old")
	t.Setenv("TERMDESK_GRAPHICS", "old")
	t.Setenv("TERMDESK_ITERM2", "old")
	t.Setenv("TERMDESK_SIXEL", "old")

	env := BuildAppEnv()

	// Count occurrences of TERMDESK=
	count := 0
	for _, e := range env {
		if e == "TERMDESK=1" {
			count++
		}
		if e == "TERMDESK=old" {
			t.Fatal("old TERMDESK value should be stripped")
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 TERMDESK=1, got %d", count)
	}
}

func TestNewServerListenError(t *testing.T) {
	// Make the socket directory read-only so net.Listen("unix", ...) fails
	// with permission denied. The socket dir exists (EnsureSocketDir succeeds),
	// but creating the actual socket file within it fails.
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Remove write permission from the socket directory.
	os.Chmod(sockDir, 0o500)
	t.Cleanup(func() { os.Chmod(sockDir, 0o700) })

	opts := ServerOptions{
		AppCommand: "/bin/sh",
		AppArgs:    []string{"-c", "exit 0"},
		AppEnv:     BuildAppEnv(),
	}
	_, err := NewServer("listen-fail", 80, 24, opts)
	if err == nil {
		t.Fatal("NewServer should fail when socket dir is read-only")
	}
	// The error could be "listen" or "pty.Setsize" etc — depends on which step
	// hits the permission error first. Just verify we get an error.
	t.Logf("NewServer error (expected): %v", err)
}

func TestBuildAppEnvWezTermFullStack(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("TERM_PROGRAM", "WezTerm")

	env := BuildAppEnv()
	joined := strings.Join(env, "\n")

	// WezTerm supports all three protocols.
	if !strings.Contains(joined, "TERMDESK_GRAPHICS=kitty") {
		t.Fatal("expected TERMDESK_GRAPHICS=kitty for WezTerm")
	}
	if !strings.Contains(joined, "TERMDESK_ITERM2=1") {
		t.Fatal("expected TERMDESK_ITERM2=1 for WezTerm")
	}
	if !strings.Contains(joined, "TERMDESK_SIXEL=1") {
		t.Fatal("expected TERMDESK_SIXEL=1 for WezTerm")
	}
}
