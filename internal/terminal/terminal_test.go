package terminal

import (
	"testing"
	"time"
)

func TestNewTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"hello"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	if term.IsClosed() {
		t.Error("should not be closed initially")
	}
}

func TestTerminalReadAndRender(t *testing.T) {
	// Run echo which outputs "hello\n" and exits
	term, err := New("/bin/echo", []string{"hello"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Give PTY time to produce output
	errCh := make(chan error, 1)
	go func() {
		errCh <- term.ReadPtyLoop()
	}()

	// Wait for output or timeout
	select {
	case <-errCh:
		// loop finished (echo exited)
	case <-time.After(2 * time.Second):
		// timeout — check what we have
	}

	output := term.Render()
	if output == "" {
		t.Error("expected non-empty render after echo")
	}
}

func TestTerminalClose(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = term.Close()
	if err != nil {
		t.Errorf("Close: %v", err)
	}

	if !term.IsClosed() {
		t.Error("should be closed after Close()")
	}

	// Double close should not error
	err = term.Close()
	if err != nil {
		t.Errorf("double Close: %v", err)
	}
}

func TestTerminalResize(t *testing.T) {
	term, err := New("/bin/echo", []string{"hi"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Resize should not panic
	term.Resize(120, 40)
}

func TestTerminalWriteInput(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// WriteInput should not panic
	term.WriteInput([]byte("hello"))
}

func TestTerminalSendKey(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// SendKey should not panic
	term.SendKey('a', 0, "a")
}

func TestNewShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/echo")
	term, err := NewShell(80, 24)
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	defer term.Close()
}

func TestNewShellDefault(t *testing.T) {
	t.Setenv("SHELL", "")
	term, err := NewShell(80, 24)
	if err != nil {
		t.Fatalf("NewShell with empty SHELL: %v", err)
	}
	defer term.Close()
}
