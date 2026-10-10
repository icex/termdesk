package session

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// DefaultSession is the name used when no session name is specified.
const DefaultSession = "default"

// SocketDir returns the directory for session sockets.
func SocketDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "termdesk")
	}
	if dir := os.Getenv("TMPDIR"); dir != "" {
		return filepath.Join(dir, fmt.Sprintf("termdesk-%d", os.Getuid()))
	}
	if dir := os.Getenv("PREFIX"); dir != "" {
		return filepath.Join(dir, "var", "run", "termdesk")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "termdesk")
	}
	return fmt.Sprintf("/tmp/termdesk-%d", os.Getuid())
}

// sanitizeSessionName ensures session names are safe for use in file paths.
// Strips directory components and restricts to alphanumeric, dash, underscore,
// and dot characters to prevent path traversal attacks.
func sanitizeSessionName(name string) string {
	name = filepath.Base(name) // strip directory components
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			sb.WriteRune(r)
		}
	}
	result := sb.String()
	if result == "" || result == "." || result == ".." {
		return DefaultSession
	}
	return result
}

// SocketPath returns the full path for a named session socket.
func SocketPath(name string) string {
	return filepath.Join(SocketDir(), sanitizeSessionName(name)+".sock")
}

// PidPath returns the path to the PID file for a named session.
func PidPath(name string) string {
	return filepath.Join(SocketDir(), sanitizeSessionName(name)+".pid")
}

// EnsureSocketDir creates the socket directory with 0700 permissions.
func EnsureSocketDir() error {
	dir := SocketDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return validateSocketDir()
}

func validateSocketDir() error {
	dir := SocketDir()
	fd, err := unix.Open(dir, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Uid != uint32(os.Getuid()) || st.Mode&077 != 0 {
		return fmt.Errorf("session directory must be owned by you and private (0700): %s", dir)
	}
	return nil
}

// SessionInfo holds information about an active session.
type SessionInfo struct {
	Name string
	Pid  int
	Sock string
}

// ListSessions returns all active sessions. Stale sockets are cleaned up.
func ListSessions() ([]SessionInfo, error) {
	dir := SocketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if err := validateSocketDir(); err != nil {
		return nil, err
	}
	var sessions []SessionInfo
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sock") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".sock")
		sockPath := filepath.Join(dir, e.Name())
		pidPath := filepath.Join(dir, name+".pid")

		pid := readPid(pidPath)
		if pid > 0 && !processAlive(pid) {
			os.Remove(sockPath)
			os.Remove(pidPath)
			continue
		}

		if pid <= 0 || !socketPresent(sockPath) {
			os.Remove(sockPath)
			os.Remove(pidPath)
			continue
		}

		sessions = append(sessions, SessionInfo{
			Name: name,
			Pid:  pid,
			Sock: sockPath,
		})
	}
	return sessions, nil
}

// SessionExists checks if a named session is running.
func SessionExists(name string) bool {
	if validateSocketDir() != nil {
		return false
	}
	sockPath := SocketPath(name)
	pidPath := PidPath(name)
	pid := readPid(pidPath)
	if pid > 0 && processAlive(pid) && socketPresent(sockPath) {
		return true
	}
	// Clean up stale files
	os.Remove(sockPath)
	os.Remove(pidPath)
	return false
}

func readPid(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func socketAlive(path string) bool { return socketPresent(path) }

// ProjectSessionName includes the canonical path so equal basenames do not collide.
func ProjectSessionName(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	sum := sha256.Sum256([]byte(path))
	base := sanitizeSessionName(filepath.Base(path))
	if len(base) > 32 {
		base = base[:32]
	}
	return fmt.Sprintf("%s-%x", base, sum[:6])
}
func socketPresent(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode()&os.ModeSocket != 0
}
