package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/icex/termdesk/internal/app"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/session"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--app":
			runApp()
			return
		case "--server":
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			runServer(name)
			return
		case "ls", "list":
			listSessions()
			return
		case "new":
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			createAndAttach(name)
			return
		case "attach", "a":
			name := session.DefaultSession
			if len(os.Args) > 2 {
				name = os.Args[2]
			}
			attachSession(name)
			return
		case "kill":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: termdesk kill NAME")
				os.Exit(1)
			}
			killSession(os.Args[2])
			return
		case "--help", "-h", "help":
			printUsage()
			return
		}
	}

	// Default: attach to existing "default" session, or create one
	createOrAttach(session.DefaultSession)
}

// runApp runs the Bubble Tea app directly (used by the server on a slave PTY).
func runApp() {
	userCfg := config.LoadUserConfig()
	theme := config.GetTheme(userCfg.Theme)
	if bg := theme.DesktopBg; len(bg) == 7 && bg[0] == '#' {
		fmt.Fprintf(os.Stdout, "\x1b]11;rgb:%s/%s/%s\x07", bg[1:3], bg[3:5], bg[5:7])
		defer fmt.Fprintf(os.Stdout, "\x1b]111\x07")
	}

	m := app.New()
	p := tea.NewProgram(m)
	m.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

// runServer starts the server process for a named session.
func runServer(name string) {
	cols, rows := 80, 24
	srv, err := session.NewServer(name, cols, rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "termdesk server: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk server: %v\n", err)
		os.Exit(1)
	}
}

// createOrAttach attaches to an existing session, or creates a new one.
func createOrAttach(name string) {
	if session.SessionExists(name) {
		attachSession(name)
	} else {
		createAndAttach(name)
	}
}

// createAndAttach starts a server in the background, then attaches.
func createAndAttach(name string) {
	if session.SessionExists(name) {
		fmt.Fprintf(os.Stderr, "session %q already exists, attaching...\n", name)
		attachSession(name)
		return
	}

	// Start server as a detached background process
	selfExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find executable: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(selfExe, "--server", name)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // detach from our terminal
	}
	devNull, _ := os.Open(os.DevNull)
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	// Log server errors for debugging
	logPath := session.SocketDir() + "/" + name + ".log"
	session.EnsureSocketDir()
	logFile, logErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if logErr != nil {
		cmd.Stderr = devNull
	} else {
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot start server: %v\n", err)
		os.Exit(1)
	}
	cmd.Process.Release() // don't wait for it

	// Wait for server to be ready
	for i := 0; i < 50; i++ {
		if session.SessionExists(name) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !session.SessionExists(name) {
		fmt.Fprintf(os.Stderr, "server failed to start (check %s)\n", logPath)
		os.Exit(1)
	}

	attachSession(name)
}

// attachSession connects to a running session.
func attachSession(name string) {
	if err := session.Attach(name); err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
}

// listSessions prints all active sessions.
func listSessions() {
	sessions, err := session.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "termdesk: %v\n", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		return
	}
	for _, s := range sessions {
		fmt.Printf("  %-20s (pid %d)\n", s.Name, s.Pid)
	}
}

// killSession sends SIGTERM to the server process for a named session.
func killSession(name string) {
	pidPath := session.PidPath(name)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session %q not found\n", name)
		os.Exit(1)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "invalid pid for session %q\n", name)
		os.Exit(1)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "cannot kill session %q (pid %d): %v\n", name, pid, err)
		os.Exit(1)
	}
	fmt.Printf("Killed session %q (pid %d)\n", name, pid)
}

func printUsage() {
	fmt.Println(`termdesk - A retro TUI desktop environment

Usage:
  termdesk              Start or attach to default session
  termdesk new [NAME]   Create a new named session
  termdesk attach NAME  Attach to an existing session
  termdesk ls           List active sessions
  termdesk kill NAME    Kill a session
  termdesk help         Show this help

Session control:
  F12                   Detach from session (session keeps running)
  Ctrl+Q                Quit termdesk (ends session)`)
}
