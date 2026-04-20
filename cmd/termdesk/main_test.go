package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintUsage(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	stdout := os.Stdout
	os.Stdout = w
	printUsage()
	w.Close()
	os.Stdout = stdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "termdesk - A retro TUI desktop environment") {
		t.Fatalf("missing header in output")
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("missing usage section")
	}
}
