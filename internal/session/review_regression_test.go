package session

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func TestListAndExistsDoNotConnectToAttachedSession(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if err := EnsureSocketDir(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", SocketPath("review"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	os.WriteFile(PidPath("review"), []byte(fmt.Sprint(os.Getpid())), 0600)
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	if !SessionExists("review") {
		t.Fatal("active session missing")
	}
	sessions, err := ListSessions()
	if err != nil || len(sessions) != 1 {
		t.Fatal("listing failed")
	}
	select {
	case conn := <-accepted:
		conn.Close()
		t.Fatal("liveness check connected to session")
	case <-time.After(30 * time.Millisecond):
	}
}
func TestSocketDirectoryRejectsSymlinkAndPublicPermissions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", root)
	target := t.TempDir()
	os.Symlink(target, SocketDir())
	if EnsureSocketDir() == nil {
		t.Fatal("symlink directory accepted")
	}
	os.Remove(SocketDir())
	os.Mkdir(SocketDir(), 0755)
	if EnsureSocketDir() == nil {
		t.Fatal("public directory accepted")
	}
}
func TestProjectSessionNamesIncludeCanonicalPath(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "first", "app")
	b := filepath.Join(root, "second", "app")
	os.MkdirAll(a, 0700)
	os.MkdirAll(b, 0700)
	if ProjectSessionName(a) == ProjectSessionName(b) {
		t.Fatal("same basename collides")
	}
	alias := filepath.Join(root, "alias")
	os.Symlink(a, alias)
	if ProjectSessionName(a) != ProjectSessionName(alias) {
		t.Fatal("canonical alias creates another session")
	}
}
func TestResizeRejectsUnboundedDimensions(t *testing.T) {
	// A rejected resize must not reach the emulator allocation path.
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	s := &Server{client: server, emu: vt.NewSafeEmulator(80, 24)}
	done := make(chan struct{})
	go func() { s.handleClient(server); close(done) }()
	WriteMsg(client, MsgResize, EncodeResize(65535, 65535))
	WriteMsg(client, MsgDetach, nil)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rejected resize did not complete")
	}
	if s.emu.Width() != 80 || s.emu.Height() != 24 {
		t.Fatal("malicious resize changed screen")
	}
}
func TestDuplicateServerCannotReplaceRunningSocket(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	EnsureSocketDir()
	listener, err := net.Listen("unix", SocketPath("existing"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	os.WriteFile(PidPath("existing"), []byte(fmt.Sprint(os.Getpid())), 0600)
	_, err = NewServer("existing", 80, 24, ServerOptions{AppCommand: "/bin/true"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatal("existing server replaced")
	}
	if !socketPresent(SocketPath("existing")) {
		t.Fatal("existing socket removed")
	}
}

func TestDetachMarkerAcrossEveryReadBoundary(t *testing.T) {
	for split := 1; split < len(detachOSC); split++ {
		var pending []byte
		first, found := extractDetach(append([]byte("frame"), detachOSC[:split]...), &pending)
		if found || string(first) != "frame" {
			t.Fatal("early detach or lost frame")
		}
		rest, found := extractDetach(append(detachOSC[split:], []byte("after")...), &pending)
		if !found || string(rest) != "after" || len(pending) != 0 {
			t.Fatalf("split %d: detach marker lost", split)
		}
	}
}
