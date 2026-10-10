package ttyprobe

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

func TestInheritedTTYProbeTimeoutAndFragmentedResponse(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	old, err := term.MakeRaw(slave.Fd())
	if err != nil {
		t.Fatal(err)
	}
	defer term.Restore(slave.Fd(), old)
	start := time.Now()
	data := ReadUntil(slave, 30*time.Millisecond, func(b []byte) bool { return bytes.Contains(b, []byte("$y")) })
	if len(data) != 0 || time.Since(start) > 300*time.Millisecond {
		t.Fatal("TTY read did not time out")
	}
	go func() {
		master.Write([]byte("\x1b[?2026;"))
		time.Sleep(5 * time.Millisecond)
		master.Write([]byte("2$y"))
	}()
	data = ReadUntil(slave, time.Second, func(b []byte) bool { return bytes.Contains(b, []byte("$y")) })
	if string(data) != "\x1b[?2026;2$y" {
		t.Fatalf("fragmented response lost: %q", data)
	}
}
