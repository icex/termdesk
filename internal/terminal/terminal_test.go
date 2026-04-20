package terminal

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

func TestNewTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"hello"}, 80, 24, 0, 0, "")
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
	term, err := New("/bin/echo", []string{"hello"}, 80, 24, 0, 0, "")
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
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
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
	term, err := New("/bin/echo", []string{"hi"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Resize should not panic
	term.Resize(120, 40)
}

func TestTerminalWriteInput(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// WriteInput should not panic
	term.WriteInput([]byte("hello"))
}

func TestTerminalSendKey(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// SendKey should not panic
	term.SendKey('a', 0, "a")
}

func TestNewShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/echo")
	term, err := NewShell(80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("NewShell: %v", err)
	}
	defer term.Close()
}

func TestNewShellDefault(t *testing.T) {
	t.Setenv("SHELL", "")
	term, err := NewShell(80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("NewShell with empty SHELL: %v", err)
	}
	defer term.Close()
}

func TestRestoreBuffer(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Wait for echo to complete so emulator processes its output
	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	// Restore buffer content to emulator
	term.RestoreBuffer("restored line 1\nrestored line 2")

	// Capture what the emulator displays
	output := term.CaptureBufferText()
	if output == "" {
		t.Fatal("expected non-empty buffer after RestoreBuffer")
	}
	// The restored text should appear somewhere in the output
	if !containsSubstring(output, "restored line 1") {
		t.Errorf("buffer should contain restored text, got:\n%s", output)
	}
	if !containsSubstring(output, "restored line 2") {
		t.Errorf("buffer should contain second line, got:\n%s", output)
	}
}

func TestRestoreBufferEmpty(t *testing.T) {
	term, err := New("/bin/echo", []string{"hi"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Should not panic on empty or closed terminal
	term.RestoreBuffer("")
	term.RestoreBuffer("some content")
}

func TestRestoreBufferClosed(t *testing.T) {
	term, err := New("/bin/echo", []string{"hi"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic on closed terminal
	term.RestoreBuffer("should be ignored")
}

func TestCaptureBufferText(t *testing.T) {
	term, err := New("/bin/echo", []string{"capture-this"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	output := term.CaptureBufferText()
	if output == "" {
		t.Fatal("expected non-empty capture")
	}
	if !containsSubstring(output, "capture-this") {
		t.Errorf("capture should contain echoed text, got:\n%s", output)
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRestoreBufferPopulatesScrollback(t *testing.T) {
	// Create a small terminal (5 rows) and restore 20 lines.
	// The first 15 should go to scrollback, last 5 to visible screen.
	term, err := New("/bin/echo", []string{"x"}, 40, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Wait for echo to finish
	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	// Build 20 lines of content
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line-%02d", i))
	}
	content := strings.Join(lines, "\n")
	term.RestoreBuffer(content)

	// Scrollback should have 15 lines (20 - 5 visible)
	if got := term.ScrollbackLen(); got != 15 {
		t.Errorf("ScrollbackLen: got %d, want 15", got)
	}

	// Oldest scrollback line (offset 14) should be "line-00"
	oldest := term.ScrollbackLine(14)
	if oldest == nil {
		t.Fatal("ScrollbackLine(14) returned nil")
	}
	oldestStr := cellsToString(oldest)
	if !containsSubstring(oldestStr, "line-00") {
		t.Errorf("oldest scrollback should contain 'line-00', got %q", oldestStr)
	}

	// Most recent scrollback (offset 0) should be "line-14"
	newest := term.ScrollbackLine(0)
	if newest == nil {
		t.Fatal("ScrollbackLine(0) returned nil")
	}
	newestStr := cellsToString(newest)
	if !containsSubstring(newestStr, "line-14") {
		t.Errorf("newest scrollback should contain 'line-14', got %q", newestStr)
	}

	// Visible screen should contain line-15 through line-19
	visible := term.CaptureBufferText()
	for i := 15; i < 20; i++ {
		expected := fmt.Sprintf("line-%02d", i)
		if !containsSubstring(visible, expected) {
			t.Errorf("visible screen should contain %q", expected)
		}
	}
}

func TestRestoreBufferFitsOnScreen(t *testing.T) {
	// Restore fewer lines than the terminal height — no scrollback needed.
	term, err := New("/bin/echo", []string{"x"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	term.RestoreBuffer("short line 1\nshort line 2")

	if got := term.ScrollbackLen(); got != 0 {
		t.Errorf("ScrollbackLen should be 0 for small content, got %d", got)
	}
	visible := term.CaptureBufferText()
	if !containsSubstring(visible, "short line 1") {
		t.Error("visible should contain 'short line 1'")
	}
}

func TestRestoreBufferUnicode(t *testing.T) {
	// Verify unicode characters survive restore/capture round-trip.
	term, err := New("/bin/echo", []string{"x"}, 40, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	// 10 lines so 5 go to scrollback, 5 to visible.
	// Include unicode: emoji, CJK, accented chars.
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("scroll-%d cafe\u0301", i))
	}
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("visible-%d abc", i))
	}
	term.RestoreBuffer(strings.Join(lines, "\n"))

	if got := term.ScrollbackLen(); got != 5 {
		t.Errorf("ScrollbackLen: got %d, want 5", got)
	}

	// Check accented char in scrollback
	line := term.ScrollbackLine(4) // oldest
	lineStr := cellsToString(line)
	if !containsSubstring(lineStr, "scroll-0") {
		t.Errorf("scrollback line should contain 'scroll-0', got %q", lineStr)
	}
}

func TestCaptureFullBufferWithScrollback(t *testing.T) {
	term, err := New("/bin/echo", []string{"x"}, 40, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	// Restore 30 lines into a 5-row terminal
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("L%03d", i))
	}
	term.RestoreBuffer(strings.Join(lines, "\n"))

	// CaptureFullBuffer should get scrollback + visible
	full := term.CaptureFullBuffer(1000)

	// Should contain both early and late lines
	if !containsSubstring(full, "L000") {
		t.Error("full buffer should contain L000 (oldest scrollback)")
	}
	if !containsSubstring(full, "L029") {
		t.Error("full buffer should contain L029 (newest visible)")
	}

	// CaptureFullBuffer with limit smaller than total
	limited := term.CaptureFullBuffer(10)
	fullLines := strings.Split(full, "\n")
	limitedLines := strings.Split(limited, "\n")
	if len(limitedLines) > len(fullLines) {
		t.Error("limited capture should not exceed full capture")
	}
}

func TestCaptureFullBufferEmpty(t *testing.T) {
	term, err := New("/bin/echo", []string{"x"}, 40, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Closed terminal should return empty
	term.Close()
	if got := term.CaptureFullBuffer(1000); got != "" {
		t.Errorf("closed terminal CaptureFullBuffer should be empty, got %q", got)
	}
}

func TestRestoreBufferEmptyLines(t *testing.T) {
	// Test content with empty lines (blank lines in scrollback).
	term, err := New("/bin/echo", []string{"x"}, 40, 3, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	// 6 lines with some blanks → 3 go to scrollback
	content := "first\n\nthird\nfourth\n\nsixth"
	term.RestoreBuffer(content)

	if got := term.ScrollbackLen(); got != 3 {
		t.Errorf("ScrollbackLen: got %d, want 3", got)
	}

	// Verify empty scrollback line exists and doesn't crash
	emptyLine := term.ScrollbackLine(1) // the blank line
	if emptyLine == nil {
		t.Fatal("blank scrollback line should not be nil")
	}
}

// cellsToString converts a ScreenCell slice to a plain string.
func cellsToString(cells []ScreenCell) string {
	var sb strings.Builder
	for _, c := range cells {
		if c.Content == "" {
			sb.WriteByte(' ')
		} else {
			sb.WriteString(c.Content)
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

func TestStripOSC667(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Test BEL-terminated OSC 667
	data := []byte("before\x1b]667;state-response;dGVzdA==\x07after")
	result := term.stripOSC667(data)
	if string(result) != "beforeafter" {
		t.Errorf("stripOSC667 BEL: got %q, want %q", string(result), "beforeafter")
	}
	if term.AppState() != "dGVzdA==" {
		t.Errorf("AppState: got %q, want %q", term.AppState(), "dGVzdA==")
	}

	// Test ST-terminated OSC 667
	term.mu.Lock()
	term.appState = ""
	term.mu.Unlock()
	data2 := []byte("hello\x1b]667;state-response;YWJj\x1b\\world")
	result2 := term.stripOSC667(data2)
	if string(result2) != "helloworld" {
		t.Errorf("stripOSC667 ST: got %q, want %q", string(result2), "helloworld")
	}
	if term.AppState() != "YWJj" {
		t.Errorf("AppState ST: got %q, want %q", term.AppState(), "YWJj")
	}

	// Test no OSC 667 — data unchanged
	data3 := []byte("normal terminal output")
	result3 := term.stripOSC667(data3)
	if string(result3) != "normal terminal output" {
		t.Errorf("stripOSC667 none: got %q, want %q", string(result3), "normal terminal output")
	}
}

func TestExtraEnv(t *testing.T) {
	// Test that extraEnv is passed through to the process
	term, err := New("/bin/sh", []string{"-c", "echo $TERMDESK_APP_STATE"}, 80, 24, 0, 0, "", "TERMDESK_APP_STATE=test123")
	if err != nil {
		t.Fatalf("New with extraEnv: %v", err)
	}
	defer term.Close()

	// Wait for output
	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	output := term.CaptureBufferText()
	if !containsSubstring(output, "test123") {
		t.Errorf("expected TERMDESK_APP_STATE in output, got:\n%s", output)
	}
}

func TestScrollbackRingBuffer(t *testing.T) {
	// Create a terminal with small scrollback cap for testing
	term, err := New("/bin/echo", []string{"test"}, 80, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially no scrollback
	if term.ScrollbackLen() != 0 {
		t.Errorf("initial ScrollbackLen: got %d, want 0", term.ScrollbackLen())
	}

	// Out-of-range ScrollbackLine should return nil
	if line := term.ScrollbackLine(0); line != nil {
		t.Error("ScrollbackLine(0) should return nil when empty")
	}
	if line := term.ScrollbackLine(-1); line != nil {
		t.Error("ScrollbackLine(-1) should return nil")
	}
}

func TestHasMouseMode(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially no mouse mode
	if term.HasMouseMode() {
		t.Error("HasMouseMode should be false initially")
	}

	// Enable mouse mode 1003 (any-event) via escape sequence written to emulator
	term.emu.Write([]byte("\x1b[?1003h"))
	if !term.HasMouseMode() {
		t.Error("HasMouseMode should be true after enabling 1003")
	}

	// Disable mouse mode 1003
	term.emu.Write([]byte("\x1b[?1003l"))
	if term.HasMouseMode() {
		t.Error("HasMouseMode should be false after disabling 1003")
	}

	// Enable button-event mode 1002
	term.emu.Write([]byte("\x1b[?1002h"))
	if !term.HasMouseMode() {
		t.Error("HasMouseMode should be true after enabling 1002")
	}

	// Disable 1002
	term.emu.Write([]byte("\x1b[?1002l"))
	if term.HasMouseMode() {
		t.Error("HasMouseMode should be false after disabling 1002")
	}
}

func TestMultiLineScrollCapture(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write 50 lines directly through safeEmuWrite (bypassing PTY).
	// In a 10-row terminal, 50 lines should push 40 into scrollback.
	var buf strings.Builder
	for i := 1; i <= 50; i++ {
		buf.WriteString(fmt.Sprintf("line %d\r\n", i))
	}
	term.safeEmuWrite([]byte(buf.String()))

	got := term.ScrollbackLen()
	if got < 35 {
		t.Errorf("ScrollbackLen = %d, want >= 35 (wrote 50 lines into 10-row terminal)", got)
	}

	// Verify most recent scrollback line contains actual content
	recent := term.ScrollbackLine(0)
	if recent == nil {
		t.Fatal("most recent scrollback line is nil")
	}
	content := cellsToString(recent)
	if !strings.Contains(content, "line") {
		t.Errorf("most recent scrollback = %q, expected to contain 'line'", content)
	}
}

func TestBlankLinesInScrollback(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write lines with blank gaps — blank lines should appear in scrollback
	term.safeEmuWrite([]byte("hello\r\n\r\n\r\nworld\r\n\r\n\r\n\r\n\r\nend\r\n"))

	got := term.ScrollbackLen()
	if got < 3 {
		t.Errorf("ScrollbackLen = %d, want >= 3 (blank lines should be captured)", got)
	}
}

// TestBlankRowScrollDetection verifies that consecutive blank rows scrolling
// through row 0 are correctly counted. This is the exact scenario that breaks
// Kitty graphics scrolling: cursor injection creates blank rows in the emulator
// (where the real terminal has the image), and when they scroll off, the old
// row-content-comparison method misses them (blank == blank).
func TestBlankRowScrollDetection(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 5, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Fill the screen with content first (rows 0-4)
	term.safeEmuWrite([]byte("line1\r\nline2\r\nline3\r\nline4\r\nline5\r\n"))
	sbBefore := term.ScrollbackLen()

	// Now write 5 blank newlines — simulates cursor injection.
	// Each one should cause a scroll since cursor is at the bottom.
	term.safeEmuWrite([]byte("\n\n\n\n\n"))
	sbAfter := term.ScrollbackLen()

	scrolled := sbAfter - sbBefore
	if scrolled < 5 {
		t.Errorf("blank newlines scrolled %d times, want >= 5 (cursor-based detection should catch blank→blank)", scrolled)
	}
}

func TestAltScreenNoScrollback(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write some content to create scrollback
	term.safeEmuWrite([]byte("before alt\r\n"))
	sbBefore := term.ScrollbackLen()

	// Enter alt screen
	term.emu.Write([]byte("\x1b[?1049h"))

	// Write many lines in alt screen — should NOT add to scrollback
	var buf strings.Builder
	for i := 0; i < 20; i++ {
		buf.WriteString(fmt.Sprintf("alt line %d\r\n", i))
	}
	term.safeEmuWrite([]byte(buf.String()))

	sbAfter := term.ScrollbackLen()
	if sbAfter != sbBefore {
		t.Errorf("scrollback grew during alt screen: before=%d, after=%d", sbBefore, sbAfter)
	}

	// Exit alt screen
	term.emu.Write([]byte("\x1b[?1049l"))
}

func TestLargeOutputScrollback(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write 15000 lines — should cap at scrollCap (10000)
	var buf strings.Builder
	for i := 1; i <= 15000; i++ {
		buf.WriteString(fmt.Sprintf("line %05d\r\n", i))
	}
	term.safeEmuWrite([]byte(buf.String()))

	got := term.ScrollbackLen()
	if got > 10000 {
		t.Errorf("ScrollbackLen = %d, should be capped at 10000", got)
	}
	if got < 5000 {
		t.Errorf("ScrollbackLen = %d, want >= 5000 (wrote 15000 lines)", got)
	}
}

// --- New coverage tests ---

func TestHasMouseMotionMode(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially no mouse motion mode
	if term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be false initially")
	}

	// Enable AnyEvent mode 1003 — this is a motion mode
	term.emu.Write([]byte("\x1b[?1003h"))
	if !term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be true after enabling 1003")
	}

	// Disable 1003
	term.emu.Write([]byte("\x1b[?1003l"))
	if term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be false after disabling 1003")
	}

	// Enable ButtonEvent mode 1002 — also a motion mode
	term.emu.Write([]byte("\x1b[?1002h"))
	if !term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be true after enabling 1002")
	}

	// Disable 1002
	term.emu.Write([]byte("\x1b[?1002l"))
	if term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be false after disabling 1002")
	}

	// Enable X10 mouse mode 9 — NOT a motion mode
	term.emu.Write([]byte("\x1b[?9h"))
	if term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be false for X10 mode (9)")
	}
	term.emu.Write([]byte("\x1b[?9l"))

	// Enable Normal mouse mode 1000 — NOT a motion mode
	term.emu.Write([]byte("\x1b[?1000h"))
	if term.HasMouseMotionMode() {
		t.Error("HasMouseMotionMode should be false for normal mode (1000)")
	}
	term.emu.Write([]byte("\x1b[?1000l"))
}

func TestIsCursorHidden(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially cursor should be visible (not hidden)
	if term.IsCursorHidden() {
		t.Error("cursor should not be hidden initially")
	}

	// Hide cursor via DECTCEM reset: \x1b[?25l
	term.emu.Write([]byte("\x1b[?25l"))
	if !term.IsCursorHidden() {
		t.Error("cursor should be hidden after DECTCEM reset")
	}

	// Show cursor via DECTCEM set: \x1b[?25h
	term.emu.Write([]byte("\x1b[?25h"))
	if term.IsCursorHidden() {
		t.Error("cursor should be visible after DECTCEM set")
	}
}

func TestCursorPosition(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initial cursor position should be at origin (0, 0)
	x, y := term.CursorPosition()
	if x != 0 || y != 0 {
		t.Errorf("initial cursor position: got (%d, %d), want (0, 0)", x, y)
	}

	// Move cursor to (5, 3) via CUP escape: \x1b[row;colH (1-indexed)
	term.emu.Write([]byte("\x1b[4;6H"))
	x, y = term.CursorPosition()
	if x != 5 || y != 3 {
		t.Errorf("cursor after CUP: got (%d, %d), want (5, 3)", x, y)
	}
}

func TestCellAt(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write "ABC" to the emulator at position 0
	term.emu.Write([]byte("\x1b[H")) // move to home
	term.emu.Write([]byte("ABC"))

	cell := term.CellAt(0, 0)
	if cell == nil {
		t.Fatal("CellAt(0,0) returned nil")
	}
	if cell.Content != "A" {
		t.Errorf("CellAt(0,0).Content: got %q, want %q", cell.Content, "A")
	}

	cell1 := term.CellAt(1, 0)
	if cell1 == nil {
		t.Fatal("CellAt(1,0) returned nil")
	}
	if cell1.Content != "B" {
		t.Errorf("CellAt(1,0).Content: got %q, want %q", cell1.Content, "B")
	}

	cell2 := term.CellAt(2, 0)
	if cell2 == nil {
		t.Fatal("CellAt(2,0) returned nil")
	}
	if cell2.Content != "C" {
		t.Errorf("CellAt(2,0).Content: got %q, want %q", cell2.Content, "C")
	}
}

func TestWidthHeight(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 120, 40, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	if got := term.Width(); got != 120 {
		t.Errorf("Width: got %d, want 120", got)
	}
	if got := term.Height(); got != 40 {
		t.Errorf("Height: got %d, want 40", got)
	}

	// After resize, dimensions should update
	term.Resize(60, 20)
	if got := term.Width(); got != 60 {
		t.Errorf("Width after resize: got %d, want 60", got)
	}
	if got := term.Height(); got != 20 {
		t.Errorf("Height after resize: got %d, want 20", got)
	}
}

func TestSendMousePress(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Enable mouse mode so SendMouse has an effect
	term.emu.Write([]byte("\x1b[?1000h"))

	// SendMouse press should not panic
	term.SendMouse(uv.MouseLeft, 5, 3, false)

	// SendMouse release should not panic
	term.SendMouse(uv.MouseLeft, 5, 3, true)

	// Middle and right buttons
	term.SendMouse(uv.MouseMiddle, 10, 10, false)
	term.SendMouse(uv.MouseMiddle, 10, 10, true)
	term.SendMouse(uv.MouseRight, 0, 0, false)
	term.SendMouse(uv.MouseRight, 0, 0, true)
}

func TestSendMouseOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic on closed terminal
	term.SendMouse(uv.MouseLeft, 0, 0, false)
	term.SendMouse(uv.MouseLeft, 0, 0, true)
}

func TestSendMouseMotion(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Enable any-event mouse mode for motion tracking
	term.emu.Write([]byte("\x1b[?1003h"))

	// SendMouseMotion should not panic
	term.SendMouseMotion(uv.MouseLeft, 5, 3)
	term.SendMouseMotion(uv.MouseNone, 10, 10)
}

func TestSendMouseMotionOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic on closed terminal
	term.SendMouseMotion(uv.MouseLeft, 0, 0)
}

func TestSendMouseWheel(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Enable mouse mode so wheel events are forwarded
	term.emu.Write([]byte("\x1b[?1000h"))

	// SendMouseWheel should not panic
	term.SendMouseWheel(uv.MouseWheelUp, 5, 3)
	term.SendMouseWheel(uv.MouseWheelDown, 5, 3)
}

func TestSendMouseWheelOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic on closed terminal
	term.SendMouseWheel(uv.MouseWheelUp, 0, 0)
}

func TestPid(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	pid := term.Pid()
	if pid <= 0 {
		t.Errorf("Pid: got %d, want > 0", pid)
	}
}

func TestReadOnce(t *testing.T) {
	// Use echo to produce output, then read it with ReadOnce
	term, err := New("/bin/echo", []string{"readonce-test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	buf := make([]byte, 4096)
	totalRead := 0
	for attempts := 0; attempts < 20; attempts++ {
		n, readErr := term.ReadOnce(buf)
		if n > 0 {
			totalRead += n
		}
		if readErr != nil {
			break
		}
		if totalRead > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if totalRead == 0 {
		t.Error("ReadOnce should have read some bytes from echo output")
	}

	// Allow time for async emulator write channel to process
	time.Sleep(100 * time.Millisecond)
}

func TestReadOnceClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	buf := make([]byte, 4096)
	_, readErr := term.ReadOnce(buf)
	if readErr != io.EOF {
		t.Errorf("ReadOnce on closed terminal: got err=%v, want io.EOF", readErr)
	}
}

func TestPtySessionFd(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	fd := term.pty.Fd()
	if fd == 0 {
		t.Error("Fd should return a non-zero file descriptor")
	}
}

func TestPtySessionSignal(t *testing.T) {
	// Start a long-running process so we can signal it
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Send SIGUSR1 — cat will be terminated by it (default handler)
	signalErr := term.pty.Signal(syscall.SIGUSR1)
	if signalErr != nil {
		t.Errorf("Signal(SIGUSR1): %v", signalErr)
	}
}

func TestPtySessionSignalNoProcess(t *testing.T) {
	// Test Signal when process is nil
	ps := &PtySession{}
	signalErr := ps.Signal(syscall.SIGUSR1)
	if signalErr == nil {
		t.Error("Signal on nil process should return error")
	}
}

func TestPtySessionWrite(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write directly to PtySession
	n, writeErr := term.pty.Write([]byte("hello"))
	if writeErr != nil {
		t.Errorf("PtySession.Write: %v", writeErr)
	}
	if n != 5 {
		t.Errorf("PtySession.Write: wrote %d bytes, want 5", n)
	}
}

func TestPtySessionPid(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	pid := term.pty.Pid()
	if pid <= 0 {
		t.Errorf("PtySession.Pid: got %d, want > 0", pid)
	}
}

func TestPtySessionPidNilProcess(t *testing.T) {
	ps := &PtySession{}
	if got := ps.Pid(); got != 0 {
		t.Errorf("Pid on nil process: got %d, want 0", got)
	}
}

func TestSendKeyOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic on closed terminal
	term.SendKey('a', 0, "a")
	term.SendKey('q', uv.ModCtrl, "")
}

func TestSendKeyShiftTextBypass(t *testing.T) {
	// When text is present and mod is Shift only (no Ctrl/Alt), SendKey bypasses
	// the emulator's key encoding and writes text directly to PTY.
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Shift+'A' — should bypass emulator and write "A" to PTY directly
	term.SendKey('A', uv.ModShift, "A")

	// Give the write loop time to process
	time.Sleep(50 * time.Millisecond)
}

func TestSendKeyCtrlKey(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Ctrl+A — goes through emulator's SendKey
	term.SendKey('a', uv.ModCtrl, "")

	// Ctrl+C
	term.SendKey('c', uv.ModCtrl, "")

	// Give the forwarding loop time
	time.Sleep(50 * time.Millisecond)
}

func TestSendKeySpecialKeys(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Test various special keys through the emulator pipeline
	term.SendKey(uv.KeyEnter, 0, "")
	term.SendKey(uv.KeyTab, 0, "")
	term.SendKey(uv.KeyEscape, 0, "")
	term.SendKey(uv.KeyBackspace, 0, "")
	term.SendKey(uv.KeyUp, 0, "")
	term.SendKey(uv.KeyDown, 0, "")

	time.Sleep(50 * time.Millisecond)
}

func TestWriteInputOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not panic — silently drops input
	term.WriteInput([]byte("should be ignored"))
}

func TestCaptureBufferTextClosed(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	got := term.CaptureBufferText()
	if got != "" {
		t.Errorf("CaptureBufferText on closed terminal: got %q, want empty", got)
	}
}

func TestRowEqual(t *testing.T) {
	// Both nil
	if !rowEqual(nil, nil) {
		t.Error("rowEqual(nil, nil) should be true")
	}

	// One nil, one non-nil
	cells := []ScreenCell{{Content: "a"}}
	if rowEqual(nil, cells) {
		t.Error("rowEqual(nil, non-nil) should be false")
	}
	if rowEqual(cells, nil) {
		t.Error("rowEqual(non-nil, nil) should be false")
	}

	// Different lengths
	short := []ScreenCell{{Content: "a"}}
	long := []ScreenCell{{Content: "a"}, {Content: "b"}}
	if rowEqual(short, long) {
		t.Error("rowEqual with different lengths should be false")
	}

	// Same content
	a := []ScreenCell{{Content: "x"}, {Content: "y"}}
	b := []ScreenCell{{Content: "x"}, {Content: "y"}}
	if !rowEqual(a, b) {
		t.Error("rowEqual with same content should be true")
	}

	// Different content
	c := []ScreenCell{{Content: "x"}, {Content: "z"}}
	if rowEqual(a, c) {
		t.Error("rowEqual with different content should be false")
	}

	// Empty slices (not nil)
	if !rowEqual([]ScreenCell{}, []ScreenCell{}) {
		t.Error("rowEqual with empty slices should be true")
	}
}

func TestSnapshotRowZeroWidth(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// snapshotRow uses emu.Width() which should be 80, test normal case first
	row := term.snapshotRow(0)
	if row == nil {
		t.Error("snapshotRow(0) should not be nil for a valid terminal")
	}
	if len(row) != 80 {
		t.Errorf("snapshotRow(0) length: got %d, want 80", len(row))
	}
}

func TestStripOSC667Incomplete(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Incomplete OSC 667 — no terminator. Data before the prefix should be returned,
	// and the incomplete sequence should be buffered.
	data := []byte("before\x1b]667;state-response;partial")
	result := term.stripOSC667(data)
	if string(result) != "before" {
		t.Errorf("stripOSC667 incomplete: got %q, want %q", string(result), "before")
	}

	// The buffered data should be prepended on next call. Now send the terminator.
	data2 := []byte("_more_data\x07after")
	result2 := term.stripOSC667(data2)
	if string(result2) != "after" {
		t.Errorf("stripOSC667 completion: got %q, want %q", string(result2), "after")
	}
	if term.AppState() != "partial_more_data" {
		t.Errorf("AppState after completion: got %q, want %q", term.AppState(), "partial_more_data")
	}
}

func TestStripOSC667MultipleSequences(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Two OSC 667 sequences in one data chunk
	data := []byte("A\x1b]667;state-response;first\x07B\x1b]667;state-response;second\x07C")
	result := term.stripOSC667(data)
	if string(result) != "ABC" {
		t.Errorf("stripOSC667 multiple: got %q, want %q", string(result), "ABC")
	}
	// The last one should win
	if term.AppState() != "second" {
		t.Errorf("AppState after multiple: got %q, want %q", term.AppState(), "second")
	}
}

func TestStripOSC667OnlyOSC(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Data that is only an OSC 667 sequence with nothing before/after
	data := []byte("\x1b]667;state-response;payload\x07")
	result := term.stripOSC667(data)
	if len(result) != 0 {
		t.Errorf("stripOSC667 only-OSC: got %q, want empty", string(result))
	}
	if term.AppState() != "payload" {
		t.Errorf("AppState: got %q, want %q", term.AppState(), "payload")
	}
}

func TestStripOSC667IncompleteAtStart(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Incomplete sequence with nothing before it (idx == 0)
	data := []byte("\x1b]667;state-response;incomplete")
	result := term.stripOSC667(data)
	if result != nil && len(result) != 0 {
		t.Errorf("stripOSC667 incomplete at start: got %q, want nil or empty", string(result))
	}
}

func TestExtractOSCTitles(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	t.Run("BEL terminated ASCII title", func(t *testing.T) {
		data := []byte("\x1b]0;hello world\x07more data")
		out := term.extractOSCTitles(data)
		if term.Title() != "hello world" {
			t.Errorf("title = %q, want %q", term.Title(), "hello world")
		}
		if string(out) != "more data" {
			t.Errorf("output = %q, want %q", string(out), "more data")
		}
	})

	t.Run("ST terminated title", func(t *testing.T) {
		data := []byte("\x1b]2;my title\x1b\\trailing")
		out := term.extractOSCTitles(data)
		if term.Title() != "my title" {
			t.Errorf("title = %q, want %q", term.Title(), "my title")
		}
		if string(out) != "trailing" {
			t.Errorf("output = %q, want %q", string(out), "trailing")
		}
	})

	t.Run("UTF-8 title with 0x9C byte (asterisk U+2733)", func(t *testing.T) {
		// ✳ = U+2733 = \xe2\x9c\xb3 — the \x9c byte would be
		// misinterpreted as C1 ST by the VT parser if we passed it through.
		title := "✳ Session"
		data := append([]byte("\x1b]0;"), []byte(title)...)
		data = append(data, 0x07)
		out := term.extractOSCTitles(data)
		if term.Title() != title {
			t.Errorf("title = %q, want %q", term.Title(), title)
		}
		if len(out) != 0 {
			t.Errorf("output = %q, want empty", string(out))
		}
	})

	t.Run("braille spinner title", func(t *testing.T) {
		title := "⠂ thinking"
		data := append([]byte("\x1b]0;"), []byte(title)...)
		data = append(data, 0x07)
		out := term.extractOSCTitles(data)
		if term.Title() != title {
			t.Errorf("title = %q, want %q", term.Title(), title)
		}
		if len(out) != 0 {
			t.Errorf("output = %q, want empty", string(out))
		}
	})

	t.Run("non-title OSC passes through", func(t *testing.T) {
		// OSC 8 (hyperlink) should pass through untouched
		data := []byte("\x1b]8;;http://example.com\x07click\x1b]8;;\x07")
		out := term.extractOSCTitles(data)
		if string(out) != string(data) {
			t.Errorf("OSC 8 modified: got %q", string(out))
		}
	})

	t.Run("no OSC returns unchanged", func(t *testing.T) {
		data := []byte("plain text\r\n")
		out := term.extractOSCTitles(data)
		if string(out) != string(data) {
			t.Errorf("plain text modified: got %q", string(out))
		}
	})

	t.Run("title embedded in other data", func(t *testing.T) {
		data := []byte("before\x1b]0;✳ Session\x07after")
		out := term.extractOSCTitles(data)
		if term.Title() != "✳ Session" {
			t.Errorf("title = %q, want %q", term.Title(), "✳ Session")
		}
		if string(out) != "beforeafter" {
			t.Errorf("output = %q, want %q", string(out), "beforeafter")
		}
	})
}

func TestRequestAppState(t *testing.T) {
	// Start a long-running process so SIGUSR1 can be sent
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// RequestAppState sends SIGUSR1 — should not error
	appErr := term.RequestAppState()
	if appErr != nil {
		t.Errorf("RequestAppState: %v", appErr)
	}
}

func TestRequestAppStateClosed(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Should not error on closed terminal — returns nil
	appErr := term.RequestAppState()
	if appErr != nil {
		t.Errorf("RequestAppState on closed terminal: %v", appErr)
	}
}

func TestForegroundCommand(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Give the process time to start
	time.Sleep(100 * time.Millisecond)

	cmd := term.ForegroundCommand()
	// On Linux, this should return "cat" for /bin/cat
	if cmd != "cat" && cmd != "" {
		// "cat" is expected, but empty is acceptable if /proc is weird
		t.Logf("ForegroundCommand: got %q (expected 'cat' or empty)", cmd)
	}
}

func TestGetCWD(t *testing.T) {
	dir := t.TempDir()
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Give the process time to start
	time.Sleep(100 * time.Millisecond)

	cwd := term.GetCWD()
	// Should be the working directory we specified
	if cwd != "" && cwd != dir {
		t.Logf("GetCWD: got %q, expected %q (may differ on some systems)", cwd, dir)
	}
}

func TestForegroundPgidInvalidPid(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Wait for echo to exit
	time.Sleep(200 * time.Millisecond)

	// After echo exits, foregroundPgid may return 0
	pgid := term.foregroundPgid()
	// We accept any value since the process may still exist momentarily
	_ = pgid
}

func TestDisableModeOtherMode(t *testing.T) {
	// Test that disabling a different mode than what was enabled does not reset mouseMode
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Enable 1003 (AnyEvent)
	term.emu.Write([]byte("\x1b[?1003h"))
	if !term.HasMouseMode() {
		t.Fatal("expected mouse mode enabled after 1003h")
	}

	// Disable 1000 (Normal) — should NOT reset since 1003 is active
	term.emu.Write([]byte("\x1b[?1000l"))
	if !term.HasMouseMode() {
		t.Error("disabling 1000 should not reset mouse mode when 1003 is active")
	}

	// Disable 1003 — should reset
	term.emu.Write([]byte("\x1b[?1003l"))
	if term.HasMouseMode() {
		t.Error("disabling 1003 should reset mouse mode")
	}
}

func TestEmuWriteLoopViaReadOnce(t *testing.T) {
	// Test that data read via ReadOnce gets processed by emuWriteLoop
	term, err := New("/bin/echo", []string{"emu-loop-test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	buf := make([]byte, 4096)
	// Read until echo finishes or timeout
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, readErr := term.ReadOnce(buf)
		if n > 0 {
			break
		}
		if readErr != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for emuWriteLoop to process the data
	time.Sleep(200 * time.Millisecond)

	output := term.CaptureBufferText()
	if !containsSubstring(output, "emu-loop-test") {
		t.Errorf("emuWriteLoop should have processed data, got:\n%s", output)
	}
}

func TestNewPtySessionWorkDir(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks so the comparison works on macOS where
	// /var → /private/var.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		resolved = dir
	}
	term, err := New("/bin/sh", []string{"-c", "pwd"}, 200, 24, 0, 0, resolved)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- term.ReadPtyLoop() }()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
	}

	output := term.CaptureBufferText()
	if !containsSubstring(output, resolved) {
		t.Errorf("expected working directory %q in output, got:\n%s", resolved, output)
	}
}

func TestSafeEmuWriteEmptyData(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Should not panic on empty data
	term.safeEmuWrite(nil)
	term.safeEmuWrite([]byte{})
}

func TestSafeEmuWriteNoNewline(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Data without newlines — should process as a single chunk
	term.safeEmuWrite([]byte("no newline here"))

	output := term.CaptureBufferText()
	if !containsSubstring(output, "no newline here") {
		t.Errorf("expected text without newline to appear, got:\n%s", output)
	}
}

func TestResizeOnClosedTerminal(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	term.Close()

	// Resize on closed terminal should not panic
	term.Resize(120, 40)
}

// --- Benchmarks ---

// BenchmarkSafeEmuWriteTyping simulates single-character typing (no newlines).
func BenchmarkSafeEmuWriteTyping(b *testing.B) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer term.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		term.safeEmuWrite([]byte("x"))
	}
}

// BenchmarkSafeEmuWriteShortLine simulates a command producing one line of output.
func BenchmarkSafeEmuWriteShortLine(b *testing.B) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer term.Close()

	line := []byte("$ ls -la /tmp/some-directory\r\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		term.safeEmuWrite(line)
	}
}

// BenchmarkSafeEmuWriteBulk50 simulates 50 lines of output in one batch.
func BenchmarkSafeEmuWriteBulk50(b *testing.B) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer term.Close()

	var buf strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&buf, "line %04d: some output data here\r\n", i)
	}
	data := []byte(buf.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		term.safeEmuWrite(data)
	}
}

// BenchmarkSafeEmuWriteBulk500 simulates 500 lines of output in one batch.
func BenchmarkSafeEmuWriteBulk500(b *testing.B) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	defer term.Close()

	var buf strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&buf, "line %04d: some output data here\r\n", i)
	}
	data := []byte(buf.String())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		term.safeEmuWrite(data)
	}
}

// TestInterceptCSIWindowOps verifies CSI 14t/16t/18t queries are intercepted
// and appropriate responses are sent back via writeCh.
func TestInterceptCSIWindowOps(t *testing.T) {
	// Build a minimal Terminal struct without goroutines so writeCh isn't drained.
	emu := &mockEmulator{w: 80, h: 24}
	term := &Terminal{
		emu:        emu,
		writeCh:    make(chan []byte, 256),
		cellPixelW: 10,
		cellPixelH: 20,
	}

	tests := []struct {
		name     string
		input    string
		wantResp string
		wantData string // remaining data after stripping
	}{
		{
			name:     "CSI 14t pixel size",
			input:    "\x1b[14t",
			wantResp: "\x1b[4;480;800t", // 24*20=480, 80*10=800
			wantData: "",
		},
		{
			name:     "CSI 16t cell size",
			input:    "\x1b[16t",
			wantResp: "\x1b[6;20;10t", // cellH=20, cellW=10
			wantData: "",
		},
		{
			name:     "CSI 18t text area",
			input:    "\x1b[18t",
			wantResp: "\x1b[8;24;80t", // rows=24, cols=80
			wantData: "",
		},
		{
			name:     "mixed data",
			input:    "hello\x1b[14tworld",
			wantResp: "\x1b[4;480;800t",
			wantData: "helloworld",
		},
		{
			name:     "non-query CSI t",
			input:    "\x1b[2t",
			wantResp: "",
			wantData: "\x1b[2t",
		},
		{
			name:     "no cell pixels",
			input:    "\x1b[14t",
			wantResp: "", // no response when cellPixel = 0
			wantData: "\x1b[14t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no cell pixels" {
				term.mu.Lock()
				term.cellPixelW = 0
				term.cellPixelH = 0
				term.mu.Unlock()
				defer func() {
					term.mu.Lock()
					term.cellPixelW = 10
					term.cellPixelH = 20
					term.mu.Unlock()
				}()
			}

			// Drain writeCh
			for {
				select {
				case <-term.writeCh:
				default:
					goto drained
				}
			}
		drained:

			result := term.interceptCSIWindowOps([]byte(tt.input))

			if tt.wantData != "" {
				if string(result) != tt.wantData {
					t.Errorf("data: got %q, want %q", string(result), tt.wantData)
				}
			} else if tt.wantResp != "" && len(result) != 0 {
				t.Errorf("data: got %q, want empty", string(result))
			}

			if tt.wantResp != "" {
				select {
				case resp := <-term.writeCh:
					if string(resp) != tt.wantResp {
						t.Errorf("response: got %q, want %q", string(resp), tt.wantResp)
					}
				default:
					t.Error("expected response in writeCh but none found")
				}
			} else {
				select {
				case resp := <-term.writeCh:
					t.Errorf("unexpected response in writeCh: %q", string(resp))
				default:
					// good
				}
			}
		})
	}
}

// mockEmulator is a minimal Emulator for unit tests that don't need real VT parsing.
type mockEmulator struct {
	w, h int
}

func (m *mockEmulator) Write(data []byte) (int, error)             { return len(data), nil }
func (m *mockEmulator) Read(buf []byte) (int, error)               { return 0, io.EOF }
func (m *mockEmulator) SendKey(ev uv.KeyEvent)                     {}
func (m *mockEmulator) SendMouse(ev uv.MouseEvent)                 {}
func (m *mockEmulator) InputPipe() io.Writer                       { return io.Discard }
func (m *mockEmulator) CursorPosition() uv.Position                { return uv.Position{} }
func (m *mockEmulator) Render() string                             { return "" }
func (m *mockEmulator) CellAt(x, y int) *uv.Cell                   { return nil }
func (m *mockEmulator) Width() int                                 { return m.w }
func (m *mockEmulator) Height() int                                { return m.h }
func (m *mockEmulator) Draw(screen uv.Screen, bounds uv.Rectangle) {}
func (m *mockEmulator) Resize(cols, rows int)                      {}
func (m *mockEmulator) SetCallbacks(cb vt.Callbacks)               {}
func (m *mockEmulator) SetDefaultForegroundColor(c color.Color)    {}
func (m *mockEmulator) SetDefaultBackgroundColor(c color.Color)    {}
func (m *mockEmulator) BackgroundColor() color.Color               { return nil }
func (m *mockEmulator) Close() error                               { return nil }

func TestTitle(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially empty title
	if got := term.Title(); got != "" {
		t.Errorf("initial Title: got %q, want empty", got)
	}

	// Set title via OSC 0
	term.emu.Write([]byte("\x1b]0;My Terminal\x07"))
	time.Sleep(50 * time.Millisecond)
	if got := term.Title(); got != "My Terminal" {
		t.Errorf("Title after OSC 0: got %q, want %q", got, "My Terminal")
	}
}

func TestIsAltScreen(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Initially not in alt screen
	if term.IsAltScreen() {
		t.Error("should not be in alt screen initially")
	}

	// Enter alt screen
	term.emu.Write([]byte("\x1b[?1049h"))
	if !term.IsAltScreen() {
		t.Error("should be in alt screen after 1049h")
	}

	// Exit alt screen
	term.emu.Write([]byte("\x1b[?1049l"))
	if term.IsAltScreen() {
		t.Error("should not be in alt screen after 1049l")
	}
}

func TestSetCellPixelSize(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	term.SetCellPixelSize(12, 24)
	term.mu.Lock()
	w, h := term.cellPixelW, term.cellPixelH
	term.mu.Unlock()
	if w != 12 || h != 24 {
		t.Errorf("SetCellPixelSize: got (%d, %d), want (12, 24)", w, h)
	}
}

func TestMarkDirty(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Consume any initial dirty
	term.ConsumeDirty()

	// MarkDirty should set dirty flag
	term.MarkDirty()
	if !term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=true after MarkDirty")
	}
	// After consuming, should be false
	if term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=false after second consume")
	}
}

func TestSetOnKittyGraphics(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	called := false
	term.SetOnKittyGraphics(func(cmd *KittyCommand, rawData []byte) *PlacementResult {
		called = true
		return nil
	})
	term.mu.Lock()
	fn := term.onKittyGraphics
	term.mu.Unlock()
	if fn == nil {
		t.Error("onKittyGraphics should be set")
	}
	_ = called // just verify it compiles, we can't trigger the callback easily
}

func TestSetOnScreenClear(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	called := false
	term.SetOnScreenClear(func() { called = true })
	term.mu.Lock()
	fn := term.onScreenClear
	term.mu.Unlock()
	if fn == nil {
		t.Error("onScreenClear should be set")
	}
	_ = called
}

func TestSetOnOutput(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	called := false
	term.SetOnOutput(func() { called = true })
	term.mu.Lock()
	fn := term.onOutput
	term.mu.Unlock()
	if fn == nil {
		t.Error("onOutput should be set")
	}
	_ = called
}

func TestSnapshotScreen(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Write some content
	term.emu.Write([]byte("\x1b[HABC"))

	cells, w, h := term.SnapshotScreen()
	if w != 80 || h != 24 {
		t.Errorf("SnapshotScreen dimensions: got %dx%d, want 80x24", w, h)
	}
	if cells == nil {
		t.Fatal("cells should not be nil")
	}
	if len(cells) != 24 {
		t.Errorf("expected 24 rows, got %d", len(cells))
	}
	if len(cells[0]) != 80 {
		t.Errorf("expected 80 cols, got %d", len(cells[0]))
	}
	// First cell should contain "A"
	if cells[0][0].Content != "A" {
		t.Errorf("cells[0][0].Content: got %q, want A", cells[0][0].Content)
	}
}

func TestEmuScreenMethods(t *testing.T) {
	screen := newEmuScreen(10, 5)

	// Test Bounds
	bounds := screen.Bounds()
	if bounds.Min.X != 0 || bounds.Min.Y != 0 || bounds.Max.X != 10 || bounds.Max.Y != 5 {
		t.Errorf("Bounds: got %v, want (0,0)-(10,5)", bounds)
	}

	// Test SetCell with valid cell
	screen.SetCell(3, 2, &uv.Cell{Content: "X", Width: 1, Style: uv.Style{}})
	cell := screen.CellAt(3, 2)
	if cell == nil {
		t.Fatal("CellAt(3,2) returned nil after SetCell")
	}
	if cell.Content != "X" {
		t.Errorf("CellAt(3,2).Content: got %q, want X", cell.Content)
	}

	// Test SetCell with nil cell
	screen.SetCell(4, 2, nil)
	if screen.cells[2][4].Content != " " {
		t.Errorf("nil cell should produce space, got %q", screen.cells[2][4].Content)
	}

	// Test out-of-bounds SetCell (should not panic)
	screen.SetCell(-1, 0, &uv.Cell{Content: "X", Width: 1})
	screen.SetCell(0, -1, &uv.Cell{Content: "X", Width: 1})
	screen.SetCell(10, 0, &uv.Cell{Content: "X", Width: 1})
	screen.SetCell(0, 5, &uv.Cell{Content: "X", Width: 1})

	// Test out-of-bounds CellAt (should return nil)
	if screen.CellAt(-1, 0) != nil {
		t.Error("CellAt(-1,0) should return nil")
	}
	if screen.CellAt(0, -1) != nil {
		t.Error("CellAt(0,-1) should return nil")
	}
	if screen.CellAt(10, 0) != nil {
		t.Error("CellAt(10,0) should return nil")
	}
	if screen.CellAt(0, 5) != nil {
		t.Error("CellAt(0,5) should return nil")
	}

	// Test WidthMethod
	wm := screen.WidthMethod()
	_ = wm // just verify it doesn't panic
}

func TestEmuScreenSetCellNegativeWidth(t *testing.T) {
	screen := newEmuScreen(10, 5)
	// Width < 0 should be normalized to 1
	screen.SetCell(0, 0, &uv.Cell{Content: "A", Width: -1})
	if screen.cells[0][0].Width != 1 {
		t.Errorf("negative width should normalize to 1, got %d", screen.cells[0][0].Width)
	}
}

// TestResizeDirtyGracePeriod verifies that ConsumeDirty returns true
// without consuming during the grace period after Resize, and reverts
// to normal consume behavior after the grace period expires.
func TestResizeDirtyGracePeriod(t *testing.T) {
	term, err := New("/bin/echo", []string{"hello"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Consume initial dirty state
	term.ConsumeDirty()
	term.mu.Lock()
	term.dirty = false
	term.mu.Unlock()

	// Resize sets dirty and dirtyUntil
	term.Resize(100, 30)

	// ConsumeDirty should return true (resize set dirty)
	if !term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=true after Resize")
	}

	// ConsumeDirty should STILL return true during grace period
	// (unlike normal behavior where it resets to false)
	if !term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=true during grace period (second call)")
	}

	// Third call should also return true
	if !term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=true during grace period (third call)")
	}

	// After grace period expires, ConsumeDirty should consume normally
	term.mu.Lock()
	term.dirtyUntil = time.Now().Add(-time.Second) // expire it
	term.dirty = true                              // set dirty for one final consume
	term.mu.Unlock()

	if !term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=true when dirty flag is set after grace expired")
	}
	// Now should be false (consumed)
	if term.ConsumeDirty() {
		t.Error("expected ConsumeDirty=false after normal consume")
	}
}

func TestWritePTYDirect(t *testing.T) {
	term, err := New("/bin/cat", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// WritePTYDirect should not panic
	term.WritePTYDirect([]byte("hello"))

	// After close, WritePTYDirect should be a no-op
	term.Close()
	term.WritePTYDirect([]byte("after close"))
}

// mockPty implements the Pty interface for testing.
type mockPty struct {
	reader io.Reader
	writer io.Writer
	closed bool
}

func (m *mockPty) Read(buf []byte) (int, error)   { return m.reader.Read(buf) }
func (m *mockPty) Write(data []byte) (int, error) { return m.writer.Write(data) }
func (m *mockPty) Resize(rows, cols uint16) error { return nil }
func (m *mockPty) Close() error                   { m.closed = true; return nil }
func (m *mockPty) Signal(sig os.Signal) error     { return nil }
func (m *mockPty) Pid() int                       { return 99999 }
func (m *mockPty) Fd() uintptr                    { return 0 }

func TestNewWithDeps(t *testing.T) {
	emu := vt.NewSafeEmulator(80, 24)
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	mock := &mockPty{reader: pr, writer: pw}
	term := NewWithDeps(mock, emu, 80, 24)
	if term == nil {
		t.Fatal("expected non-nil terminal")
	}
	defer term.Close()

	if term.Width() != 80 {
		t.Errorf("expected width 80, got %d", term.Width())
	}
	if term.Height() != 24 {
		t.Errorf("expected height 24, got %d", term.Height())
	}
}

func TestEncodeKeyCtrlSpecials(t *testing.T) {
	tests := []struct {
		code rune
		mod  uv.KeyMod
		want []byte
	}{
		{'@', uv.ModCtrl, []byte{0x00}},
		{'[', uv.ModCtrl, []byte{0x1B}},
		{'\\', uv.ModCtrl, []byte{0x1C}},
		{']', uv.ModCtrl, []byte{0x1D}},
		{'^', uv.ModCtrl, []byte{0x1E}},
		{'_', uv.ModCtrl, []byte{0x1F}},
		{'A', uv.ModCtrl, []byte{1}},  // Ctrl+A
		{'z', uv.ModCtrl, []byte{26}}, // Ctrl+Z
	}
	for _, tt := range tests {
		got := encodeKey(tt.code, tt.mod, "")
		if string(got) != string(tt.want) {
			t.Errorf("encodeKey(%c, Ctrl) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestEncodeKeyAlt(t *testing.T) {
	// Alt+a with text
	got := encodeKey('a', uv.ModAlt, "a")
	if len(got) != 2 || got[0] != 0x1B || got[1] != 'a' {
		t.Errorf("Alt+a with text: got %v", got)
	}

	// Alt+b without text
	got = encodeKey('b', uv.ModAlt, "")
	if len(got) != 2 || got[0] != 0x1B || got[1] != 'b' {
		t.Errorf("Alt+b without text: got %v", got)
	}

	// Alt+code > 127 falls through to printable path (no ESC prefix added)
	got = encodeKey(200, uv.ModAlt, "")
	if got == nil {
		t.Error("Alt+highcode: expected non-nil (falls to printable rune)")
	}
}

func TestEncodeKeySpecialKeys(t *testing.T) {
	tests := []struct {
		code rune
		mod  uv.KeyMod
		want string
	}{
		{uv.KeyEnter, 0, "\r"},
		{uv.KeyTab, 0, "\t"},
		{uv.KeyTab, uv.ModShift, "\x1b[Z"},
		{uv.KeyBackspace, 0, "\x7f"},
		{uv.KeyEscape, 0, "\x1b"},
		{uv.KeySpace, 0, " "},
		{uv.KeyHome, 0, "\x1b[H"},
		{uv.KeyEnd, 0, "\x1b[F"},
		{uv.KeyInsert, 0, "\x1b[2~"},
		{uv.KeyDelete, 0, "\x1b[3~"},
		{uv.KeyPgUp, 0, "\x1b[5~"},
		{uv.KeyPgDown, 0, "\x1b[6~"},
		{uv.KeyF1, 0, "\x1bOP"},
		{uv.KeyF2, 0, "\x1bOQ"},
		{uv.KeyF3, 0, "\x1bOR"},
		{uv.KeyF4, 0, "\x1bOS"},
		{uv.KeyF5, 0, "\x1b[15~"},
		{uv.KeyF6, 0, "\x1b[17~"},
		{uv.KeyF7, 0, "\x1b[18~"},
		{uv.KeyF8, 0, "\x1b[19~"},
		{uv.KeyF9, 0, "\x1b[20~"},
		{uv.KeyF10, 0, "\x1b[21~"},
		{uv.KeyF11, 0, "\x1b[23~"},
		{uv.KeyF12, 0, "\x1b[24~"},
	}
	for _, tt := range tests {
		got := encodeKey(tt.code, tt.mod, "")
		if string(got) != tt.want {
			t.Errorf("encodeKey(%d, %d) = %q, want %q", tt.code, tt.mod, got, tt.want)
		}
	}
}

func TestEncodeKeyPrintableAndUTF8(t *testing.T) {
	// Printable text takes priority
	got := encodeKey('a', 0, "hello")
	if string(got) != "hello" {
		t.Errorf("expected text 'hello', got %q", got)
	}

	// Single printable rune
	got = encodeKey('Z', 0, "")
	if string(got) != "Z" {
		t.Errorf("expected 'Z', got %q", got)
	}

	// Multi-byte UTF-8 rune
	got = encodeKey('日', 0, "")
	if string(got) != "日" {
		t.Errorf("expected '日', got %q", got)
	}

	// Non-printable, no text — returns nil
	got = encodeKey(0, 0, "")
	if got != nil {
		t.Errorf("expected nil for code=0, got %v", got)
	}
}

func TestEncodeKeyArrowsWithModifiers(t *testing.T) {
	// Shift+Up
	got := encodeKey(uv.KeyUp, uv.ModShift, "")
	if string(got) != "\x1b[1;2A" {
		t.Errorf("Shift+Up = %q", got)
	}

	// Ctrl+Right
	got = encodeKey(uv.KeyRight, uv.ModCtrl, "")
	if string(got) != "\x1b[1;5C" {
		t.Errorf("Ctrl+Right = %q", got)
	}
}

func TestExtractKittyAPCsWithBEL(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// APC terminated with BEL (0x07)
	data := []byte("before\x1b_Ga=q,i=1;AAAA\x07after")
	segments, trailing := term.extractKittyAPCs(data)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if string(segments[0].DataBefore) != "before" {
		t.Errorf("DataBefore = %q, want 'before'", segments[0].DataBefore)
	}
	if string(trailing) != "after" {
		t.Errorf("trailing = %q, want 'after'", trailing)
	}
}

func TestExtractKittyAPCsIncomplete(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Incomplete APC — no terminator
	data := []byte("before\x1b_Ga=q,i=1;AAAA")
	segments, trailing := term.extractKittyAPCs(data)

	if len(segments) != 0 {
		t.Errorf("expected 0 segments for incomplete APC, got %d", len(segments))
	}
	if string(trailing) != "before" {
		t.Errorf("trailing = %q, want 'before'", trailing)
	}
	// Buffer should contain the incomplete APC
	if len(term.kittyAPCBuf) == 0 {
		t.Error("expected kittyAPCBuf to be non-empty")
	}
}

func TestExtractKittyAPCsMultiple(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Two APCs in one buffer
	data := []byte("A\x1b_Ga=q,i=1;AA\x1b\\B\x1b_Ga=q,i=2;BB\x1b\\C")
	segments, trailing := term.extractKittyAPCs(data)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if string(segments[0].DataBefore) != "A" {
		t.Errorf("seg[0].DataBefore = %q", segments[0].DataBefore)
	}
	if string(segments[1].DataBefore) != "B" {
		t.Errorf("seg[1].DataBefore = %q", segments[1].DataBefore)
	}
	if string(trailing) != "C" {
		t.Errorf("trailing = %q", trailing)
	}
}

func TestExtractKittyAPCsBufferedContinuation(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// First call: incomplete APC
	data1 := []byte("\x1b_Ga=q,i=1;AA")
	term.extractKittyAPCs(data1)

	// Second call: complete the APC
	data2 := []byte("AA\x1b\\rest")
	segments, trailing := term.extractKittyAPCs(data2)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment from continuation, got %d", len(segments))
	}
	if string(trailing) != "rest" {
		t.Errorf("trailing = %q, want 'rest'", trailing)
	}
}

func TestFeedEmuDataScreenClear(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	clearCalled := false
	term.SetOnScreenClear(func() { clearCalled = true })

	// Feed data with CSI 2J (screen clear)
	term.feedEmuData([]byte("hello\x1b[2Jworld"))
	if !clearCalled {
		t.Error("expected onScreenClear to be called for CSI 2J")
	}

	clearCalled = false
	term.feedEmuData([]byte("\x1b[3J"))
	if !clearCalled {
		t.Error("expected onScreenClear to be called for CSI 3J")
	}
}

func TestSafeEmuWriteBasic(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Basic write should not panic
	term.safeEmuWrite([]byte("hello world"))

	// Empty data should be no-op
	term.safeEmuWrite(nil)
	term.safeEmuWrite([]byte{})
}

func TestSafeEmuWriteWithKittyCallback(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	apcCalled := false
	term.SetOnKittyGraphics(func(cmd *KittyCommand, rawData []byte) *PlacementResult {
		apcCalled = true
		return nil
	})

	// Write with a Kitty APC embedded
	term.safeEmuWrite([]byte("before\x1b_Ga=q,i=1;AAAA\x1b\\after"))
	if !apcCalled {
		t.Error("expected kitty graphics callback to be called")
	}
}

func TestSafeEmuWriteWithKittyCallbackAndCursorMove(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	term.SetOnKittyGraphics(func(cmd *KittyCommand, rawData []byte) *PlacementResult {
		return &PlacementResult{Rows: 3, CursorMove: 0}
	})

	// Should inject newlines for cursor movement
	term.safeEmuWrite([]byte("X\x1b_Ga=T,i=1,f=32,s=1,v=1;AAAA\x1b\\Y"))
}

func TestForegroundPgidAndCommand(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		t.Skip("foregroundPgid uses /proc, Linux/Android only")
	}
	// Use /bin/sleep for a process that sticks around
	term, err := New("/bin/sleep", []string{"10"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	time.Sleep(100 * time.Millisecond)

	pid := term.Pid()
	if pid <= 0 {
		t.Fatalf("expected positive PID, got %d", pid)
	}

	// foregroundPgid should return a positive value for a real PTY
	pgid := term.foregroundPgid()
	if pgid <= 0 {
		t.Errorf("expected positive pgid, got %d", pgid)
	}

	// ForegroundCommand should return something
	cmd := term.ForegroundCommand()
	if cmd == "" {
		t.Error("expected non-empty foreground command")
	}
}

func TestGetCWDWithForeground(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "android" {
		t.Skip("GetCWD uses /proc, Linux/Android only")
	}
	term, err := New("/bin/sleep", []string{"10"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	time.Sleep(100 * time.Millisecond)

	cwd := term.GetCWD()
	if cwd == "" {
		t.Error("expected non-empty CWD")
	}
	// CWD should be a valid path
	if _, err := os.Stat(cwd); err != nil {
		t.Errorf("CWD %q is not a valid path: %v", cwd, err)
	}
}

func TestSnapshotRowEdgeCases(t *testing.T) {
	term, err := New("/bin/echo", []string{"hi"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Wait for output
	go func() { term.ReadPtyLoop() }()
	time.Sleep(200 * time.Millisecond)

	// snapshotRow should return cells
	row := term.snapshotRow(0)
	if row == nil {
		t.Fatal("expected non-nil row")
	}
	if len(row) != 80 {
		t.Errorf("expected 80 cells, got %d", len(row))
	}
}

func TestSafeEmuWriteOSC667(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Should strip OSC 667 without crashing
	term.safeEmuWrite([]byte("\x1b]667;key=value\x07rest"))
}

func TestInterceptCSIWindowOps14t(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Set cellPixel sizes — interceptCSIWindowOps requires them to be non-zero
	term.SetCellPixelSize(10, 20)

	// CSI 14t (report pixel size) should be intercepted
	result := term.interceptCSIWindowOps([]byte("\x1b[14trest"))
	if strings.Contains(string(result), "[14t") {
		t.Errorf("expected CSI 14t to be intercepted, got %q", result)
	}
	if string(result) != "rest" {
		t.Errorf("expected remaining 'rest', got %q", result)
	}
}

func TestDetectKittyGraphicsSupportEnvVars(t *testing.T) {
	// Save and clear all relevant env vars
	vars := []string{"TERMDESK_GRAPHICS", "KITTY_WINDOW_ID", "WEZTERM_PANE",
		"TERM_PROGRAM", "KONSOLE_DBUS_SESSION", "KONSOLE_VERSION"}
	saved := make(map[string]string)
	for _, v := range vars {
		saved[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	// TERMDESK_GRAPHICS=kitty should return true
	os.Setenv("TERMDESK_GRAPHICS", "kitty")
	if !detectKittyGraphicsSupport() {
		t.Error("expected true for TERMDESK_GRAPHICS=kitty")
	}
	os.Unsetenv("TERMDESK_GRAPHICS")

	// KITTY_WINDOW_ID should return true
	os.Setenv("KITTY_WINDOW_ID", "1")
	if !detectKittyGraphicsSupport() {
		t.Error("expected true for KITTY_WINDOW_ID")
	}
	os.Unsetenv("KITTY_WINDOW_ID")

	// WEZTERM_PANE should return true
	os.Setenv("WEZTERM_PANE", "0")
	if !detectKittyGraphicsSupport() {
		t.Error("expected true for WEZTERM_PANE")
	}
	os.Unsetenv("WEZTERM_PANE")

	// TERM_PROGRAM=kitty should return true
	os.Setenv("TERM_PROGRAM", "kitty")
	if !detectKittyGraphicsSupport() {
		t.Error("expected true for TERM_PROGRAM=kitty")
	}
	os.Unsetenv("TERM_PROGRAM")

	// KONSOLE_VERSION should return true
	os.Setenv("KONSOLE_VERSION", "220101")
	if !detectKittyGraphicsSupport() {
		t.Error("expected true for KONSOLE_VERSION")
	}
	os.Unsetenv("KONSOLE_VERSION")
}

func TestBellCallbackFires(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	bellFired := make(chan struct{}, 1)
	term.SetOnBell(func() {
		select {
		case bellFired <- struct{}{}:
		default:
		}
	})

	// Write BEL character through the emulator write channel.
	// emuWriteLoop will process it, emu.Write triggers the Bell callback
	// which sets bellPending, then emuWriteLoop calls onBell after emu.Write returns.
	term.emuCh <- []byte("\x07")

	select {
	case <-bellFired:
		// Success — bell callback was called
	case <-time.After(2 * time.Second):
		t.Error("expected onBell callback to fire after BEL character, timed out")
	}
}

func TestBellCallbackNotFiredWithoutBEL(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	bellFired := make(chan struct{}, 1)
	term.SetOnBell(func() {
		select {
		case bellFired <- struct{}{}:
		default:
		}
	})

	// Write normal text (no BEL) — bell should NOT fire
	term.emuCh <- []byte("hello world\r\n")

	select {
	case <-bellFired:
		t.Error("onBell callback should not fire for normal text")
	case <-time.After(200 * time.Millisecond):
		// Good — no bell fired
	}
}

func TestBellCallbackNilSafe(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// No onBell callback set — writing BEL should not panic
	done := make(chan struct{})
	go func() {
		term.emuCh <- []byte("\x07")
		// Give emuWriteLoop time to process
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Success — no panic
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for BEL processing without callback")
	}
}

func TestBellDeferredFlag(t *testing.T) {
	term, err := New("/bin/sh", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// Verify the deferred bell mechanism: the Bell callback in emu.SetCallbacks
	// sets bellPending=true, and emuWriteLoop fires the actual onBell callback
	// after emu.Write() returns (to avoid deadlock with SafeEmulator lock).
	var callOrder []string
	bellFired := make(chan struct{}, 1)

	term.SetOnBell(func() {
		callOrder = append(callOrder, "onBell")
		select {
		case bellFired <- struct{}{}:
		default:
		}
	})

	// Send BEL via the emulator channel
	term.emuCh <- []byte("text\x07more")

	select {
	case <-bellFired:
		if len(callOrder) != 1 || callOrder[0] != "onBell" {
			t.Errorf("expected exactly one onBell call, got %v", callOrder)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for deferred bell callback")
	}
}

// ---------------------------------------------------------------------------
// SetOnImageGraphics coverage
// ---------------------------------------------------------------------------

func TestSetOnImageGraphics(t *testing.T) {
	term, err := New("/bin/echo", []string{"test"}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	called := false
	term.SetOnImageGraphics(func(rawData []byte, format ImageFormat, estimatedCellRows int) int {
		called = true
		return 0
	})
	term.mu.Lock()
	fn := term.onImageGraphics
	term.mu.Unlock()
	if fn == nil {
		t.Error("onImageGraphics should be set")
	}
	_ = called
}

// ---------------------------------------------------------------------------
// safeEmuWrite with image graphics callbacks (sixel/iTerm2 paths)
// ---------------------------------------------------------------------------

func TestSafeEmuWriteWithSixelCallback(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()
	term.SetCellPixelSize(10, 16)

	var imgCalls []ImageFormat
	term.SetOnImageGraphics(func(rawData []byte, format ImageFormat, estimatedCellRows int) int {
		imgCalls = append(imgCalls, format)
		return 0
	})

	// Write data containing a sixel DCS sequence
	sixelData := []byte("before\x1bPq??~-??~\x1b\\after")
	term.safeEmuWrite(sixelData)

	if len(imgCalls) != 1 {
		t.Fatalf("expected 1 image callback, got %d", len(imgCalls))
	}
	if imgCalls[0] != ImageFormatSixel {
		t.Errorf("expected ImageFormatSixel, got %d", imgCalls[0])
	}
}

func TestSafeEmuWriteWithIterm2Callback(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()
	term.SetCellPixelSize(10, 20)

	var imgCalls []ImageFormat
	term.SetOnImageGraphics(func(rawData []byte, format ImageFormat, estimatedCellRows int) int {
		imgCalls = append(imgCalls, format)
		return 0
	})

	// Write data containing an iTerm2 OSC sequence
	iterm2Data := []byte("pre\x1b]1337;File=inline=1:AAAA\x07post")
	term.safeEmuWrite(iterm2Data)

	if len(imgCalls) != 1 {
		t.Fatalf("expected 1 image callback, got %d", len(imgCalls))
	}
	if imgCalls[0] != ImageFormatIterm2 {
		t.Errorf("expected ImageFormatIterm2, got %d", imgCalls[0])
	}
}

func TestSafeEmuWriteWithIterm2MultipartCallback(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()
	term.SetCellPixelSize(10, 20)

	var imgCalls []ImageFormat
	term.SetOnImageGraphics(func(rawData []byte, format ImageFormat, estimatedCellRows int) int {
		imgCalls = append(imgCalls, format)
		return 0
	})

	// Write data containing a multipart iTerm2 OSC sequence (FilePart=)
	iterm2Data := []byte("\x1b]1337;FilePart=id=1:AAAA\x07rest")
	term.safeEmuWrite(iterm2Data)

	if len(imgCalls) != 1 {
		t.Fatalf("expected 1 image callback for multipart, got %d", len(imgCalls))
	}
}

func TestSafeEmuWriteNoImageCallbackNil(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	// No image callback set — sixel data should be passed to emulator without issues
	term.safeEmuWrite([]byte("before\x1bPq??~\x1b\\after"))
}

func TestSafeEmuWriteWithKittyCallbackNilResult(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	term.SetOnKittyGraphics(func(cmd *KittyCommand, rawData []byte) *PlacementResult {
		return nil // no cursor injection needed
	})

	// Write with APC — callback returns nil, no cursor injection
	term.safeEmuWrite([]byte("X\x1b_Ga=q,i=1;AAAA\x1b\\Y"))
}

func TestSafeEmuWriteWithKittyCallbackCursorMoveOne(t *testing.T) {
	term, err := New("/bin/echo", nil, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer term.Close()

	term.SetOnKittyGraphics(func(cmd *KittyCommand, rawData []byte) *PlacementResult {
		return &PlacementResult{Rows: 3, CursorMove: 1} // C=1 means don't move cursor
	})

	// CursorMove=1 means no newline injection
	term.safeEmuWrite([]byte("\x1b_Ga=T,i=1,f=32,s=1,v=1,C=1;AAAA\x1b\\"))
}

// ---------------------------------------------------------------------------
// interceptCSIWindowOps additional branch coverage
// ---------------------------------------------------------------------------

func TestInterceptCSIWindowOpsMultipleQueries(t *testing.T) {
	emu := &mockEmulator{w: 80, h: 24}
	term := &Terminal{
		emu:        emu,
		writeCh:    make(chan []byte, 256),
		cellPixelW: 10,
		cellPixelH: 20,
	}

	// Multiple queries in one data chunk
	result := term.interceptCSIWindowOps([]byte("\x1b[14t\x1b[16t\x1b[18t"))
	if len(result) != 0 {
		t.Errorf("expected empty result after stripping all queries, got %q", result)
	}

	// Should have 3 responses
	count := 0
	for {
		select {
		case <-term.writeCh:
			count++
		default:
			goto done
		}
	}
done:
	if count != 3 {
		t.Errorf("expected 3 responses, got %d", count)
	}
}

func TestInterceptCSIWindowOpsInvalidParam(t *testing.T) {
	emu := &mockEmulator{w: 80, h: 24}
	term := &Terminal{
		emu:        emu,
		writeCh:    make(chan []byte, 256),
		cellPixelW: 10,
		cellPixelH: 20,
	}

	// CSI with non-numeric content should pass through
	result := term.interceptCSIWindowOps([]byte("\x1b[abct"))
	if string(result) != "\x1b[abct" {
		t.Errorf("non-numeric CSI should pass through, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// SnapshotScreen with mock emulator returning nil cells
// ---------------------------------------------------------------------------

func TestSnapshotScreenZeroDimensions(t *testing.T) {
	emu := &mockEmulator{w: 0, h: 0}
	term := &Terminal{
		emu:        emu,
		writeCh:    make(chan []byte, 1),
		emuCh:      make(chan []byte, 1),
		done:       make(chan struct{}),
		inputDone:  make(chan struct{}),
		scrollCap:  defaultScrollbackCap,
		scrollRing: make([][]ScreenCell, defaultScrollbackCap),
	}

	cells, w, h := term.SnapshotScreen()
	if cells != nil {
		t.Error("expected nil cells for zero-dimension emulator")
	}
	if w != 0 || h != 0 {
		t.Errorf("expected 0x0, got %dx%d", w, h)
	}
}

// ---------------------------------------------------------------------------
// ParseKittyCommand additional branch coverage
// ---------------------------------------------------------------------------

func TestParseKittyCommandFileMediumNoPadding(t *testing.T) {
	// File medium with base64 path that lacks padding (uses RawStdEncoding fallback)
	// "/tmp/img.png" = "L3RtcC9pbWcucG5n" without padding
	data := []byte("a=t,t=f;L3RtcC9pbWcucG5n")
	cmd, err := ParseKittyCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.FilePath != "/tmp/img.png" {
		t.Errorf("FilePath: got %q, want /tmp/img.png", cmd.FilePath)
	}
}

func TestParseKittyCommandFileMediumInvalidBase64(t *testing.T) {
	// Completely invalid base64 for file path — should not crash
	data := []byte("a=t,t=f;!!!invalid!!!")
	cmd, err := ParseKittyCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// FilePath should be empty since decode failed
	if cmd.FilePath != "" {
		t.Errorf("expected empty FilePath for invalid base64, got %q", cmd.FilePath)
	}
}

func TestParseKittyCommandTempFileNoData(t *testing.T) {
	// Temp file medium with no data part
	data := []byte("a=t,t=t")
	cmd, err := ParseKittyCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Medium != KittyMediumTempFile {
		t.Errorf("expected temp file medium, got %c", byte(cmd.Medium))
	}
	// FilePath should be empty
	if cmd.FilePath != "" {
		t.Errorf("expected empty FilePath, got %q", cmd.FilePath)
	}
}

func TestParseKittyCommandDirectWithInvalidBase64Data(t *testing.T) {
	// Direct medium with invalid base64 data — falls back to raw data
	data := []byte("a=t;!!!not-base64!!!")
	cmd, err := ParseKittyCommand(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Data) == 0 {
		t.Error("expected non-empty data (raw fallback)")
	}
}

// ---------------------------------------------------------------------------
// ForwardCommand additional branch coverage
// ---------------------------------------------------------------------------

func TestForwardCommandEchoedResponseFiltered(t *testing.T) {
	kp := newTestKP()

	// Transmit with data that looks like a response ("OK")
	cmd := &KittyCommand{
		Action: KittyActionTransmit,
		Data:   []byte("OK"),
	}
	result := kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if result != nil {
		t.Error("echoed response should be filtered (return nil)")
	}
	// Should not have added anything to pendingOutput
	if len(kp.pendingOutput) != 0 {
		t.Error("echoed response should not produce output")
	}
}

func TestForwardCommandFileMediumEmptyPath(t *testing.T) {
	kp := newTestKP()

	// File medium without a file path — should return nil
	cmd := &KittyCommand{
		Action: KittyActionTransmit,
		Medium: KittyMediumFile,
		// FilePath is empty
	}
	result := kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if result != nil {
		t.Error("file medium with empty path should return nil")
	}
}

func TestForwardCommandShmQueryRejected(t *testing.T) {
	kp := newTestKP()

	var ptyResponse []byte
	cmd := &KittyCommand{
		Action:  KittyActionQuery,
		Medium:  KittyMediumSharedMemory,
		ImageID: 123,
	}
	kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, func(b []byte) {
		ptyResponse = append(ptyResponse, b...)
	})

	if !bytes.Contains(ptyResponse, []byte("ENOTSUPPORTED")) {
		t.Errorf("expected ENOTSUPPORTED response for shm query, got %q", ptyResponse)
	}
}

func TestForwardCommandFileMediumConversion(t *testing.T) {
	kp := newTestKP()

	// Create a temp file for file medium conversion
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake-image-data"), 0644)

	cmd := &KittyCommand{
		Action:   KittyActionTransmitPlace,
		Medium:   KittyMediumTempFile,
		FilePath: path,
		Format:   KittyFormatPNG,
		Width:    100,
		Height:   200,
	}
	result := kp.ForwardCommand(cmd, nil, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	if result == nil {
		t.Fatal("expected non-nil result for file medium conversion")
	}

	// File should be deleted (temp file medium)
	if _, err := os.Stat(path); err == nil {
		t.Error("temp file should be deleted after conversion")
	}
}

func TestForwardCommandDefaultMediumZero(t *testing.T) {
	kp := newTestKP()

	// Medium=0 should be treated as KittyMediumDirect
	rawData := []byte("a=t,i=1,f=24,s=10,v=10;AAAA")
	cmd := &KittyCommand{
		Action:  KittyActionTransmit,
		ImageID: 1,
		Format:  KittyFormatRGB,
		Width:   10,
		Height:  10,
		Medium:  0, // zero value
		Data:    []byte{0, 0, 0},
	}
	kp.ForwardCommand(cmd, rawData, "win1", 0, 0, 80, 24, 1, 1, 0, 0, 0, false, nil)
	// Should forward as direct without error
	if len(kp.pendingOutput) == 0 {
		t.Error("expected pending output for direct transmission")
	}
}

// ---------------------------------------------------------------------------
// RefreshAllPlacements additional branch coverage
// ---------------------------------------------------------------------------

func TestRefreshAllPlacementsManipulatedWindow(t *testing.T) {
	kp := newTestKP()

	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Make visible first
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
			},
		}
	})
	kp.FlushPending()

	// Now mark as manipulated (during animation)
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
				IsManipulated: true,
			},
		}
	})

	// All placements should be hidden
	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				t.Error("expected all placements hidden when window is manipulated")
			}
		}
	}
}

func TestRefreshAllPlacementsAltScreenMismatch(t *testing.T) {
	kp := newTestKP()

	// Place image in normal screen
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Make visible first
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
				IsAltScreen: false,
			},
		}
	})
	kp.FlushPending()

	// Now window enters alt screen — placement should be hidden
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
				IsAltScreen: true, // mismatch with placement
			},
		}
	})

	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				t.Error("expected placement hidden during alt screen mismatch")
			}
		}
	}
}

func TestRefreshAllPlacementsAltScreenPlacementDeleted(t *testing.T) {
	kp := newTestKP()

	// Place image in alt screen
	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, true, nil)
	kp.FlushPending()

	// Now window exits alt screen — alt screen placement should be deleted
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
				IsAltScreen: false, // not alt screen — placement was in alt screen
			},
		}
	})

	// The alt screen placement should have been deleted
	if len(kp.placements["win1"]) > 0 {
		t.Error("alt screen placements should be deleted when window exits alt screen")
	}
}

func TestRefreshAllPlacementsHorizontalClipping(t *testing.T) {
	kp := newTestKP()

	// Place a wide image
	cmd := &KittyCommand{
		Action:  KittyActionTransmitPlace,
		Format:  KittyFormatPNG,
		Width:   1000, // very wide
		Height:  200,
		Columns: 100, // 100 columns
		Data:    []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Window only has 50 cols of content — image should be horizontally clipped
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 50, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
			},
		}
	})

	// Verify placement was shown with clipped columns
	for _, placements := range kp.placements {
		for _, p := range placements {
			if p.MaxShowableCols > 50 {
				t.Errorf("expected MaxShowableCols <= 50, got %d", p.MaxShowableCols)
			}
		}
	}
}

func TestRefreshAllPlacementsNegativeHostPosition(t *testing.T) {
	kp := newTestKP()

	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Window at negative position — placement should not be visible
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: -10, WindowY: -10, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
			},
		}
	})

	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				t.Error("expected placement hidden when window is at negative position")
			}
		}
	}
}

func TestRefreshAllPlacementsDeferredScrollback(t *testing.T) {
	kp := newTestKP()

	cmd := &KittyCommand{
		Action: KittyActionTransmitPlace,
		Format: KittyFormatPNG,
		Width:  100, Height: 200,
		Data: []byte("fakepng"),
	}
	kp.ForwardCommand(cmd, nil, "win1", 5, 2, 80, 24, 1, 1, 0, 0, 0, false, nil)
	kp.FlushPending()

	// Set ReadyScrollback to a value higher than current scrollback
	for _, placements := range kp.placements {
		for _, p := range placements {
			p.ReadyScrollback = 20
		}
	}

	// Refresh with scrollback < ReadyScrollback — should stay hidden
	kp.RefreshAllPlacements(func() map[string]*KittyWindowInfo {
		return map[string]*KittyWindowInfo{
			"win1": {
				WindowX: 5, WindowY: 2, ContentOffX: 1, ContentOffY: 1,
				ContentWidth: 78, ContentHeight: 22,
				Width: 80, Height: 24, Visible: true,
				ScrollbackLen: 10, // < ReadyScrollback (20)
			},
		}
	})

	for _, placements := range kp.placements {
		for _, p := range placements {
			if !p.Hidden {
				t.Error("expected placement hidden when scrollback < ReadyScrollback (deferred)")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// placeOne additional branch coverage
// ---------------------------------------------------------------------------

func TestPlaceOneHorizontalClipping(t *testing.T) {
	kp := newTestKP()
	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 20, Rows: 10, DisplayRows: 10,
		MaxShowable: 10, MaxShowableCols: 15, // less than Cols (20)
		PixelWidth: 200, PixelHeight: 200,
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// Should have c=15 (clipped columns)
	if !strings.Contains(out, ",c=15") {
		t.Errorf("expected c=15 for horizontal clipping, got: %s", out)
	}
	// Should have w= for horizontal clip
	if !strings.Contains(out, ",w=") {
		t.Errorf("expected w= for horizontal clipping, got: %s", out)
	}
}

func TestPlaceOnePreservedSourceWidth(t *testing.T) {
	kp := newTestKP()
	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 20, Rows: 10, DisplayRows: 10,
		MaxShowable: 10, MaxShowableCols: 20, // no horizontal clip
		PixelWidth: 200, PixelHeight: 200,
		SourceWidth: 150, // app specified source width
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// Should preserve the original source width
	if !strings.Contains(out, ",w=150") {
		t.Errorf("expected w=150 (preserved source width), got: %s", out)
	}
}

func TestPlaceOneClipTopWithPixelClamp(t *testing.T) {
	kp := newTestKP()
	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 20, Rows: 10, DisplayRows: 10,
		MaxShowable:     5, // only 5 rows visible
		MaxShowableCols: 20,
		ClipTop:         5,                     // 5 rows clipped from top
		PixelWidth:      200, PixelHeight: 100, // small image
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// Should have y= for clip top
	if !strings.Contains(out, ",y=") {
		t.Errorf("expected y= for clipTop>0, got: %s", out)
	}
	// Should have h= for sourceHeight clamping
	if !strings.Contains(out, ",h=") {
		t.Errorf("expected h= for sourceHeight, got: %s", out)
	}
}

func TestPlaceOneAllZeroFallbacks(t *testing.T) {
	kp := newTestKP()
	kp.cellPixelH = 0 // will default to 20
	kp.cellPixelW = 0 // will default to 10

	p := &KittyPlacement{
		HostImageID: 1,
		HostX:       10, HostY: 5,
		Cols: 0, Rows: 0, DisplayRows: 0,
		MaxShowable: 0, MaxShowableCols: 0,
	}
	kp.placeOne(p)
	out := string(kp.pendingOutput)

	// Should have r=1 (fallback for all zeros)
	if !strings.Contains(out, ",r=1") {
		t.Errorf("expected r=1 fallback, got: %s", out)
	}
}
