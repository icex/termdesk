package app

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	return buf.String()
}

func TestOSC52SetCmdEmitsClipboardMsg(t *testing.T) {
	cmd := osc52SetCmd("hello")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for non-empty text")
	}
	msg := cmd()
	if got := fmt.Sprint(msg); got != "hello" {
		t.Errorf("expected clipboard msg payload %q, got %q", "hello", got)
	}
}

func TestOSC52SetCmdEmptyTextReturnsNil(t *testing.T) {
	if cmd := osc52SetCmd(""); cmd != nil {
		t.Error("expected nil cmd for empty text")
	}
}

func TestTerminalClipboardMsgMirrorsAndForwards(t *testing.T) {
	m := New()
	updated, cmd := m.Update(TerminalClipboardMsg{WindowID: "term-1", Text: "from ssh"})
	um, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got := um.clipboard.Paste(); got != "from ssh" {
		t.Errorf("clipboard ring = %q, want %q", got, "from ssh")
	}
	if cmd == nil {
		t.Fatal("expected an OSC 52 cmd forwarding the copy to the host terminal")
	}
	if got := fmt.Sprint(cmd()); got != "from ssh" {
		t.Errorf("forwarded payload = %q, want %q", got, "from ssh")
	}
}

func TestTerminalClipboardMsgIgnoresEmpty(t *testing.T) {
	m := New()
	if _, cmd := m.Update(TerminalClipboardMsg{WindowID: "term-1"}); cmd != nil {
		t.Error("expected nil cmd for empty text")
	}
}
