package session

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// shortTempDir creates a short temp directory under /tmp suitable for Unix
// sockets. macOS limits socket paths to ~104 bytes, and t.TempDir() paths
// (e.g. /var/folders/.../TestName.../001/) often exceed that.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "td-")
	if err != nil {
		t.Fatalf("shortTempDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestSocketDir(t *testing.T) {
	dir := SocketDir()
	if dir == "" {
		t.Error("SocketDir returned empty string")
	}
}

func TestSocketDirTMPDIR(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", tmp)
	t.Setenv("PREFIX", "")

	dir := SocketDir()
	want := filepath.Join(tmp, "termdesk")
	if dir != want {
		t.Fatalf("SocketDir=%q want %q", dir, want)
	}
}

func TestSocketDirPREFIX(t *testing.T) {
	prefix := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "")
	t.Setenv("PREFIX", prefix)

	dir := SocketDir()
	want := filepath.Join(prefix, "var", "run", "termdesk")
	if dir != want {
		t.Fatalf("SocketDir=%q want %q", dir, want)
	}
}

func TestSocketDirHomeFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "")
	t.Setenv("PREFIX", "")

	dir := SocketDir()
	home, err := os.UserHomeDir()
	if err != nil {
		// If UserHomeDir fails, we should get the /tmp fallback.
		want := fmt.Sprintf("/tmp/termdesk-%d", os.Getuid())
		if dir != want {
			t.Fatalf("SocketDir=%q want %q", dir, want)
		}
		return
	}
	want := filepath.Join(home, ".cache", "termdesk")
	if dir != want {
		t.Fatalf("SocketDir=%q want %q", dir, want)
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
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

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
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	if SessionExists("nonexistent") {
		t.Error("SessionExists returned true for nonexistent session")
	}
}

func TestListSessionsEmpty(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsNonExistentDir(t *testing.T) {
	// Point to a directory that does not exist at all.
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "nonexistent"))

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions should return nil error for missing dir: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsWithActiveSessions(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create a live Unix socket listener to simulate an active session.
	sockPath := filepath.Join(sockDir, "mysession.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	// Write a PID file with the current process PID (known alive).
	pidPath := filepath.Join(sockDir, "mysession.pid")
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "mysession" {
		t.Errorf("session name = %q, want %q", sessions[0].Name, "mysession")
	}
	if sessions[0].Pid != os.Getpid() {
		t.Errorf("session pid = %d, want %d", sessions[0].Pid, os.Getpid())
	}
	if sessions[0].Sock != sockPath {
		t.Errorf("session sock = %q, want %q", sessions[0].Sock, sockPath)
	}
}

func TestListSessionsSkipsNonSockFiles(t *testing.T) {
	tmp := t.TempDir()
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create a non-.sock file; it should be ignored.
	os.WriteFile(filepath.Join(sockDir, "readme.txt"), []byte("hi"), 0o600)
	os.WriteFile(filepath.Join(sockDir, "test.pid"), []byte("123"), 0o600)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsCleansStaleDeadProcess(t *testing.T) {
	tmp := t.TempDir()
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create a .sock file and .pid file with a dead PID.
	sockPath := filepath.Join(sockDir, "stale.sock")
	pidPath := filepath.Join(sockDir, "stale.pid")
	os.WriteFile(sockPath, []byte{}, 0o600)
	os.WriteFile(pidPath, []byte("999999999"), 0o600)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after stale cleanup, got %d", len(sessions))
	}

	// Verify the stale files were cleaned up.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("stale .sock file should have been removed")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale .pid file should have been removed")
	}
}

func TestListSessionsCleansStaleDeadSocket(t *testing.T) {
	tmp := t.TempDir()
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create a .sock file (regular file, not a real socket) with no PID file.
	// The process check is skipped (pid=0), but socketAlive will fail.
	sockPath := filepath.Join(sockDir, "dead.sock")
	os.WriteFile(sockPath, []byte{}, 0o600)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	// The fake .sock should have been cleaned up.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("dead .sock file should have been removed")
	}
}

func TestSessionExistsActive(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Set up a live socket and PID file.
	sockPath := filepath.Join(sockDir, "active.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	pidPath := filepath.Join(sockDir, "active.pid")
	os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)

	if !SessionExists("active") {
		t.Error("SessionExists should return true for an active session")
	}
}

func TestReadPidValid(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")
	os.WriteFile(path, []byte("12345\n"), 0o600)

	pid := readPid(path)
	if pid != 12345 {
		t.Errorf("readPid = %d, want 12345", pid)
	}
}

func TestReadPidMissingFile(t *testing.T) {
	pid := readPid("/nonexistent/path/test.pid")
	if pid != 0 {
		t.Errorf("readPid for missing file = %d, want 0", pid)
	}
}

func TestReadPidInvalidContent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.pid")
	os.WriteFile(path, []byte("not-a-number"), 0o600)

	pid := readPid(path)
	if pid != 0 {
		t.Errorf("readPid for invalid content = %d, want 0", pid)
	}
}

func TestReadPidWithWhitespace(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "ws.pid")
	os.WriteFile(path, []byte("  42  \n"), 0o600)

	pid := readPid(path)
	if pid != 42 {
		t.Errorf("readPid with whitespace = %d, want 42", pid)
	}
}

func TestProcessAliveCurrentProcess(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("processAlive should return true for the current process")
	}
}

func TestProcessAliveDeadPid(t *testing.T) {
	// PID 999999999 is almost certainly not a running process.
	if processAlive(999999999) {
		t.Error("processAlive should return false for a non-existent PID")
	}
}

func TestSocketAliveWithListener(t *testing.T) {
	tmp := shortTempDir(t)
	sockPath := filepath.Join(tmp, "test.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	if !socketAlive(sockPath) {
		t.Error("socketAlive should return true for an active listener")
	}
}

func TestSocketAliveNoListener(t *testing.T) {
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "dead.sock")
	// Create a regular file, not a real socket.
	os.WriteFile(sockPath, []byte{}, 0o600)

	if socketAlive(sockPath) {
		t.Error("socketAlive should return false for a non-listening path")
	}
}

func TestSocketAliveNonExistentPath(t *testing.T) {
	if socketAlive("/nonexistent/path/test.sock") {
		t.Error("socketAlive should return false for a non-existent path")
	}
}

func TestDefaultSessionConstant(t *testing.T) {
	if DefaultSession != "default" {
		t.Errorf("DefaultSession = %q, want %q", DefaultSession, "default")
	}
}

func TestSocketDirXDGRuntime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	dir := SocketDir()
	want := filepath.Join(tmp, "termdesk")
	if dir != want {
		t.Fatalf("SocketDir=%q want %q", dir, want)
	}
}

func TestSocketPathContainsName(t *testing.T) {
	path := SocketPath("mysession")
	base := filepath.Base(path)
	if base != "mysession.sock" {
		t.Errorf("SocketPath base = %q, want %q", base, "mysession.sock")
	}
}

func TestPidPathContainsName(t *testing.T) {
	path := PidPath("mysession")
	base := filepath.Base(path)
	if base != "mysession.pid" {
		t.Errorf("PidPath base = %q, want %q", base, "mysession.pid")
	}
}

func TestSessionExistsCleansStaleSocket(t *testing.T) {
	tmp := t.TempDir()
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create stale sock and pid files.
	sockPath := filepath.Join(sockDir, "stale.sock")
	pidPath := filepath.Join(sockDir, "stale.pid")
	os.WriteFile(sockPath, []byte{}, 0o600)
	os.WriteFile(pidPath, []byte("999999999"), 0o600)

	if SessionExists("stale") {
		t.Error("SessionExists should return false for stale session")
	}

	// Verify files were cleaned up.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("stale .sock should have been removed by SessionExists")
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale .pid should have been removed by SessionExists")
	}
}

func TestSessionExistsNoPidFile(t *testing.T) {
	tmp := t.TempDir()
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create socket file without pid file.
	sockPath := filepath.Join(sockDir, "nopid.sock")
	os.WriteFile(sockPath, []byte{}, 0o600)

	// pid=0, processAlive(0) returns false (signal to pid 0 sends to all processes in group,
	// but on most systems Kill(0,0) returns an error for non-existent process).
	// SessionExists should return false and clean up.
	result := SessionExists("nopid")
	if result {
		t.Error("SessionExists should return false without valid PID")
	}
}

func TestListSessionsMultipleSessions(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Create two active sessions.
	names := []string{"session-a", "session-b"}
	var listeners []net.Listener
	for _, name := range names {
		sockPath := filepath.Join(sockDir, name+".sock")
		ln, err := net.Listen("unix", sockPath)
		if err != nil {
			t.Fatalf("net.Listen %s: %v", name, err)
		}
		listeners = append(listeners, ln)

		pidPath := filepath.Join(sockDir, name+".pid")
		os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o600)
	}
	defer func() {
		for _, ln := range listeners {
			ln.Close()
		}
	}()

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Check both sessions are present (order not guaranteed by ReadDir).
	nameSet := make(map[string]bool)
	for _, s := range sessions {
		nameSet[s.Name] = true
	}
	for _, name := range names {
		if !nameSet[name] {
			t.Errorf("session %q not found in list", name)
		}
	}
}

func TestListSessionsMixedStaleAndActive(t *testing.T) {
	tmp := shortTempDir(t)
	sockDir := filepath.Join(tmp, "termdesk")
	os.MkdirAll(sockDir, 0o700)

	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// One active session.
	activeSock := filepath.Join(sockDir, "active.sock")
	ln, err := net.Listen("unix", activeSock)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	os.WriteFile(filepath.Join(sockDir, "active.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600)

	// One stale session (dead process).
	staleSock := filepath.Join(sockDir, "stale.sock")
	os.WriteFile(staleSock, []byte{}, 0o600)
	os.WriteFile(filepath.Join(sockDir, "stale.pid"), []byte("999999999"), 0o600)

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (only active), got %d", len(sessions))
	}
	if sessions[0].Name != "active" {
		t.Errorf("session name = %q, want %q", sessions[0].Name, "active")
	}

	// Verify stale files were cleaned up.
	if _, err := os.Stat(staleSock); !os.IsNotExist(err) {
		t.Error("stale sock should be cleaned up")
	}
}

func TestEnsureSocketDirIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Call twice; should not fail the second time.
	if err := EnsureSocketDir(); err != nil {
		t.Fatalf("first EnsureSocketDir: %v", err)
	}
	if err := EnsureSocketDir(); err != nil {
		t.Fatalf("second EnsureSocketDir: %v", err)
	}
}

func TestReadPidEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.pid")
	os.WriteFile(path, []byte(""), 0o600)

	pid := readPid(path)
	if pid != 0 {
		t.Errorf("readPid for empty file = %d, want 0", pid)
	}
}

func TestReadPidNegativeNumber(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "neg.pid")
	os.WriteFile(path, []byte("-1"), 0o600)

	pid := readPid(path)
	if pid != -1 {
		t.Logf("readPid for -1 = %d (negative PIDs are valid in Atoi)", pid)
	}
}

func TestProcessAlivePid1(t *testing.T) {
	// PID 1 (init/systemd) should always be alive on Linux.
	if !processAlive(1) {
		t.Log("processAlive(1) returned false (may happen in some containers)")
	}
}

func TestSessionInfoStruct(t *testing.T) {
	info := SessionInfo{
		Name: "test",
		Pid:  12345,
		Sock: "/tmp/test.sock",
	}
	if info.Name != "test" {
		t.Errorf("Name = %q, want %q", info.Name, "test")
	}
	if info.Pid != 12345 {
		t.Errorf("Pid = %d, want 12345", info.Pid)
	}
	if info.Sock != "/tmp/test.sock" {
		t.Errorf("Sock = %q, want %q", info.Sock, "/tmp/test.sock")
	}
}

func TestSanitizeSessionName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"mysession", "mysession"},
		{"my-session", "my-session"},
		{"my_session", "my_session"},
		{"my.session", "my.session"},
		{"Session123", "Session123"},
		// Path traversal attempts — filepath.Base strips directory components
		{"../../etc/passwd", "passwd"},
		{"../../../tmp/evil", "evil"},
		{"/absolute/path", "path"},
		{"relative/path", "path"},
		// Special characters stripped
		{"session;rm -rf /", "sessionrm-rf"},
		{"session$(cmd)", "sessioncmd"},
		{"session`cmd`", "sessioncmd"},
		{"session name", "sessionname"},
		// Edge cases
		{"", DefaultSession},
		{".", DefaultSession},
		{"..", DefaultSession},
		{"...", "..."},
	}
	for _, tt := range tests {
		got := sanitizeSessionName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeSessionName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSocketPathSanitizesTraversal(t *testing.T) {
	path := SocketPath("../../evil")
	base := filepath.Base(path)
	if base != "evil.sock" {
		t.Errorf("SocketPath base = %q, want %q", base, "evil.sock")
	}
	// Verify the path stays within the socket directory
	dir := filepath.Dir(path)
	if dir != SocketDir() {
		t.Errorf("SocketPath directory = %q, want %q", dir, SocketDir())
	}
}

func TestPidPathSanitizesTraversal(t *testing.T) {
	path := PidPath("../../evil")
	base := filepath.Base(path)
	if base != "evil.pid" {
		t.Errorf("PidPath base = %q, want %q", base, "evil.pid")
	}
	dir := filepath.Dir(path)
	if dir != SocketDir() {
		t.Errorf("PidPath directory = %q, want %q", dir, SocketDir())
	}
}

func TestSocketDirFallbackTmp(t *testing.T) {
	// Clear all env vars that SocketDir checks, including HOME to force
	// UserHomeDir to fail, so we hit the /tmp/termdesk-UID fallback.
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "")
	t.Setenv("PREFIX", "")
	t.Setenv("HOME", "")

	dir := SocketDir()
	want := fmt.Sprintf("/tmp/termdesk-%d", os.Getuid())
	if dir != want {
		t.Fatalf("SocketDir fallback = %q, want %q", dir, want)
	}
}
