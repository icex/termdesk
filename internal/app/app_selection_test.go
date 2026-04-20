package app

import (
	"bytes"
	"encoding/base64"
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

func TestWriteOSC52WritesSequence(t *testing.T) {
	text := "hello"
	got := captureStdout(t, func() {
		writeOSC52(text)
	})

	wantPayload := base64.StdEncoding.EncodeToString([]byte(text))
	want := "\x1b]52;c;" + wantPayload + "\x07"
	if got != want {
		t.Fatalf("OSC52 mismatch: got %q want %q", got, want)
	}
}
