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
