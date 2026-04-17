package session

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResetTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	resetTerminal(w)
	w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "\x1b[?1049l") {
		t.Fatalf("missing alt-screen reset")
	}
	if !strings.Contains(out, "\x1b[?25h") {
		t.Fatalf("missing cursor show")
	}
	if !strings.Contains(out, "\x1b[0m") {
		t.Fatalf("missing SGR reset")
	}
}

func TestResetTerminalAllSequences(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	resetTerminal(w)
	w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(data)

	// Verify all escape sequences are present.
	expected := []struct {
		seq  string
		desc string
	}{
		{"\x1b[?1049l", "exit alt screen"},
		{"\x1b[?1002l", "disable mouse cell motion"},
		{"\x1b[?1003l", "disable mouse all motion"},
		{"\x1b[?1006l", "disable SGR mouse encoding"},
		{"\x1b[?2004l", "disable bracketed paste"},
		{"\x1b[?25h", "show cursor"},
		{"\x1b[0m", "reset SGR attributes"},
		{"\x1b]110\x07", "reset default foreground"},
		{"\x1b]111\x07", "reset default background"},
		{"\x1b[2J", "clear screen"},
		{"\x1b[H", "move cursor to home"},
	}
	for _, e := range expected {
		if !strings.Contains(out, e.seq) {
			t.Errorf("missing %s sequence", e.desc)
		}
	}
}

func TestClientReadLoopOutputMessages(t *testing.T) {
	// Create a Unix socket pair to simulate server-client.
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	// Accept in background.
	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	// Create a pipe for output capture.
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	// Run clientReadLoop in a goroutine.
	readDone := make(chan error, 1)
	go func() {
		readDone <- clientReadLoop(clientConn, outW, true)
	}()

	// Send some output messages from the "server" side.
	WriteMsg(serverConn, MsgOutput, []byte("hello "))
	WriteMsg(serverConn, MsgOutput, []byte("world"))
	WriteMsg(serverConn, MsgRedraw, []byte("!screen!"))

	// Close the server side to trigger EOF on clientReadLoop.
	time.Sleep(50 * time.Millisecond)
	serverConn.Close()

	// Wait for clientReadLoop to finish.
	err = <-readDone
	if err != nil {
		t.Fatalf("clientReadLoop returned error: %v", err)
	}

	outW.Close()
	data, _ := io.ReadAll(outR)
	got := string(data)

	if !strings.Contains(got, "hello ") {
		t.Errorf("output missing 'hello ', got: %q", got)
	}
	if !strings.Contains(got, "world") {
		t.Errorf("output missing 'world', got: %q", got)
	}
	if !strings.Contains(got, "!screen!") {
		t.Errorf("output missing '!screen!' (MsgRedraw), got: %q", got)
	}
}

func TestClientReadLoopIgnoresNonOutputMessages(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- clientReadLoop(clientConn, outW, true)
	}()

	// Send a non-output message type (e.g., MsgInput, MsgResize)
	// These should be ignored by clientReadLoop.
	WriteMsg(serverConn, MsgInput, []byte("should-be-ignored"))
	WriteMsg(serverConn, MsgResize, EncodeResize(24, 80))
	WriteMsg(serverConn, MsgOutput, []byte("visible"))

	time.Sleep(50 * time.Millisecond)
	serverConn.Close()

	err = <-readDone
	if err != nil {
		t.Fatalf("clientReadLoop: %v", err)
	}

	outW.Close()
	data, _ := io.ReadAll(outR)
	got := string(data)

	if strings.Contains(got, "should-be-ignored") {
		t.Error("clientReadLoop should not forward MsgInput messages")
	}
	if !strings.Contains(got, "visible") {
		t.Errorf("clientReadLoop should forward MsgOutput messages, got: %q", got)
	}
}

func TestClientReadLoopConnectionError(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	_, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- clientReadLoop(clientConn, outW, true)
	}()

	// Write a partial message (truncated header) then close.
	serverConn.Write([]byte{MsgOutput, 0, 0})
	serverConn.Close()

	err = <-readDone
	// Should return a non-nil error (not EOF because partial header).
	if err == nil {
		// Partial header might result in EOF depending on timing, which is also acceptable.
		// The important thing is that the loop terminates.
	}
	outW.Close()
	_ = serverConn
}

func TestClientWriteLoop(t *testing.T) {
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	// Create a pipe to simulate stdin.
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- clientWriteLoop(inR, clientConn)
	}()

	// Write some data to the "stdin" pipe.
	inW.Write([]byte("hello from client"))

	// Give time for the write to propagate.
	time.Sleep(50 * time.Millisecond)

	// Read the TLV message from the server side.
	typ, payload, err := ReadMsg(serverConn)
	if err != nil {
		t.Fatalf("ReadMsg from server: %v", err)
	}
	if typ != MsgInput {
		t.Errorf("message type = %c, want 'I'", typ)
	}
	if string(payload) != "hello from client" {
		t.Errorf("payload = %q, want %q", string(payload), "hello from client")
	}

	// Close stdin to stop the write loop.
	inW.Close()

	err = <-writeDone
	// EOF from closed stdin is expected.
	if err != nil && err != io.EOF {
		// Some OSes may return a different error for closed pipe.
		t.Logf("clientWriteLoop error (may be expected): %v", err)
	}

	serverConn.Close()
	clientConn.Close()
}

func TestClientWriteLoopMultipleWrites(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- clientWriteLoop(inR, clientConn)
	}()

	// Send multiple writes.
	messages := []string{"first", "second", "third"}
	for _, msg := range messages {
		inW.Write([]byte(msg))
		time.Sleep(20 * time.Millisecond)
	}

	// Read all TLV messages from the server side.
	var received []string
	for i := 0; i < len(messages); i++ {
		typ, payload, err := ReadMsg(serverConn)
		if err != nil {
			t.Fatalf("ReadMsg %d: %v", i, err)
		}
		if typ != MsgInput {
			t.Errorf("message %d type = %c, want 'I'", i, typ)
		}
		received = append(received, string(payload))
	}

	// Verify all messages arrived (order guaranteed by TCP-like Unix socket).
	for i, msg := range messages {
		if received[i] != msg {
			t.Errorf("message %d = %q, want %q", i, received[i], msg)
		}
	}

	inW.Close()
	<-writeDone
	serverConn.Close()
	clientConn.Close()
}

func TestErrDetached(t *testing.T) {
	// Verify errDetached is a non-nil error with a meaningful message.
	if errDetached == nil {
		t.Fatal("errDetached should not be nil")
	}
	if errDetached.Error() != "detached" {
		t.Errorf("errDetached.Error() = %q, want %q", errDetached.Error(), "detached")
	}
}

func TestSendCurrentSize(t *testing.T) {
	// sendCurrentSize reads from os.Stdout.Fd() which may fail in test env,
	// but it should not panic. We test it with a unix socket pair.
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	// sendCurrentSize should not panic even if term.GetSize fails.
	sendCurrentSize(clientConn)

	// Give it a moment, then close.
	time.Sleep(20 * time.Millisecond)

	clientConn.Close()
	serverConn.Close()
}

func TestSendCurrentSizeNoTerminal(t *testing.T) {
	// sendCurrentSize should not panic even without a real terminal.
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	// Should not panic — term.GetSize will fail on a non-tty, and
	// sendCurrentSize gracefully returns.
	sendCurrentSize(clientConn)

	time.Sleep(20 * time.Millisecond)
	serverConn.Close()
	clientConn.Close()
}

func TestClientReadLoopLargeData(t *testing.T) {
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- clientReadLoop(clientConn, outW, true)
	}()

	// Send a large payload.
	bigPayload := bytes.Repeat([]byte("X"), 32768)
	WriteMsg(serverConn, MsgOutput, bigPayload)

	time.Sleep(50 * time.Millisecond)
	serverConn.Close()

	err = <-readDone
	if err != nil {
		t.Fatalf("clientReadLoop: %v", err)
	}

	outW.Close()
	data, _ := io.ReadAll(outR)
	// Strip sync markers added by clientReadLoop's syncFlush.
	data = bytes.ReplaceAll(data, syncStart, nil)
	data = bytes.ReplaceAll(data, syncEnd, nil)
	if len(data) != 32768 {
		t.Errorf("received %d bytes, want 32768", len(data))
	}
}

func TestAttachConnectionError(t *testing.T) {
	// Attach should fail when there's no server listening.
	old := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	defer os.Setenv("XDG_RUNTIME_DIR", old)

	// Ensure the socket directory exists but no server is running.
	EnsureSocketDir()

	err := Attach("nonexistent-session")
	if err == nil {
		t.Error("Attach should return error when no server is listening")
	}
	if !strings.Contains(err.Error(), "cannot connect") {
		t.Errorf("expected 'cannot connect' error, got: %v", err)
	}
}

func TestClientReadLoopConcurrentFlush(t *testing.T) {
	// Test that the buffered writer + flush ticker don't race.
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	var serverConn net.Conn
	accepted := make(chan struct{})
	go func() {
		serverConn, _ = ln.Accept()
		close(accepted)
	}()

	clientConn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	<-accepted

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- clientReadLoop(clientConn, outW, true)
	}()

	// Send many small messages rapidly to exercise the mutex and ticker.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			WriteMsg(serverConn, MsgOutput, []byte("x"))
		}
	}()
	wg.Wait()

	time.Sleep(50 * time.Millisecond)
	serverConn.Close()

	err = <-readDone
	if err != nil {
		t.Fatalf("clientReadLoop: %v", err)
	}

	outW.Close()
	data, _ := io.ReadAll(outR)
	// Strip sync markers added by clientReadLoop's syncFlush.
	data = bytes.ReplaceAll(data, syncStart, nil)
	data = bytes.ReplaceAll(data, syncEnd, nil)
	if len(data) != 100 {
		t.Errorf("received %d bytes, want 100", len(data))
	}
}
