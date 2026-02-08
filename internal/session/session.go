package session

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DefaultSession is the name used when no session name is specified.
const DefaultSession = "default"

// SocketDir returns the directory for session sockets.
func SocketDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "termdesk")
	}
	return fmt.Sprintf("/tmp/termdesk-%d", os.Getuid())
}

// SocketPath returns the full path for a named session socket.
func SocketPath(name string) string {
	return filepath.Join(SocketDir(), name+".sock")
}

// PidPath returns the path to the PID file for a named session.
func PidPath(name string) string {
	return filepath.Join(SocketDir(), name+".pid")
}

// EnsureSocketDir creates the socket directory with 0700 permissions.
func EnsureSocketDir() error {
	return os.MkdirAll(SocketDir(), 0o700)
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

		if !socketAlive(sockPath) {
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
	sockPath := SocketPath(name)
	pidPath := PidPath(name)
	pid := readPid(pidPath)
	if pid > 0 && processAlive(pid) && socketAlive(sockPath) {
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

func socketAlive(path string) bool {
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
