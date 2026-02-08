package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSocketDir(t *testing.T) {
	dir := SocketDir()
	if dir == "" {
		t.Error("SocketDir returned empty string")
	}
}

func TestSocketPath(t *testing.T) {
	path := SocketPath("test")
	if filepath.Ext(path) != ".sock" {
		t.Errorf("SocketPath = %q, want .sock extension", path)
	}
}

func TestPidPath(t *testing.T) {
	path := PidPath("test")
	if filepath.Ext(path) != ".pid" {
		t.Errorf("PidPath = %q, want .pid extension", path)
	}
}

func TestEnsureSocketDir(t *testing.T) {
	// Use a temp dir to avoid polluting the real socket dir
	old := os.Getenv("XDG_RUNTIME_DIR")
	tmp := t.TempDir()
	os.Setenv("XDG_RUNTIME_DIR", tmp)
	defer os.Setenv("XDG_RUNTIME_DIR", old)

	if err := EnsureSocketDir(); err != nil {
		t.Fatalf("EnsureSocketDir: %v", err)
	}

	dir := SocketDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("socket dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("socket dir is not a directory")
	}
}

func TestSessionExistsNone(t *testing.T) {
	old := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	defer os.Setenv("XDG_RUNTIME_DIR", old)

	if SessionExists("nonexistent") {
		t.Error("SessionExists returned true for nonexistent session")
	}
}

func TestListSessionsEmpty(t *testing.T) {
	old := os.Getenv("XDG_RUNTIME_DIR")
	os.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	defer os.Setenv("XDG_RUNTIME_DIR", old)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
