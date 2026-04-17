package session

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewAdapterReturnsNonNil(t *testing.T) {
	a := NewAdapter()
	if a == nil {
		t.Fatal("NewAdapter returned nil")
	}
}

func TestAdapterSocketDir(t *testing.T) {
	a := NewAdapter()
	dir := a.SocketDir()
	want := SocketDir()
	if dir != want {
		t.Errorf("Adapter.SocketDir = %q, want %q", dir, want)
	}
}

func TestAdapterEnsureSocketDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	a := NewAdapter()
	if err := a.EnsureSocketDir(); err != nil {
		t.Fatalf("Adapter.EnsureSocketDir: %v", err)
	}

	dir := a.SocketDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("socket dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("socket dir is not a directory")
	}
}

func TestAdapterPidPath(t *testing.T) {
	a := NewAdapter()
	got := a.PidPath("mysession")
	want := PidPath("mysession")
	if got != want {
		t.Errorf("Adapter.PidPath = %q, want %q", got, want)
	}
}

func TestAdapterSessionExistsFalse(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	a := NewAdapter()
	if a.SessionExists("nonexistent") {
		t.Error("Adapter.SessionExists should return false for nonexistent session")
	}
}

func TestAdapterListSessionsEmpty(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	a := NewAdapter()
	sessions, err := a.ListSessions()
	if err != nil {
		t.Fatalf("Adapter.ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestAdapterListSessionsWithActive(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	sockPath := filepath.Join(sockDir, "adtest.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	pidPath := filepath.Join(sockDir, "adtest.pid")
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	a := NewAdapter()
	sessions, err := a.ListSessions()
	if err != nil {
		t.Fatalf("Adapter.ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "adtest" {
		t.Errorf("session name = %q, want %q", sessions[0].Name, "adtest")
	}
}

func TestAdapterSessionExistsActive(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	sockPath := filepath.Join(sockDir, "exists-test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	pidPath := filepath.Join(sockDir, "exists-test.pid")
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	a := NewAdapter()
	if !a.SessionExists("exists-test") {
		t.Error("Adapter.SessionExists should return true for active session")
	}
}

func TestAdapterAttachNoServer(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	EnsureSocketDir()

	a := NewAdapter()
	err := a.Attach("nonexistent")
	if err == nil {
		t.Error("Adapter.Attach should fail when no server is running")
	}
}

func TestAdapterNewServerEmptyCommand(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	a := NewAdapter()
	_, err := a.NewServer("test", 80, 24, ServerOptions{})
	if err == nil {
		t.Error("Adapter.NewServer should fail with empty AppCommand")
	}
}
