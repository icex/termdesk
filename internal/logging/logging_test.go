package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"err", LevelError},
		{"off", LevelOff},
		{"none", LevelOff},
		{"", LevelOff},
		{"garbage", LevelOff},
	}
	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLevelName(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{LevelOff, "off"},
	}
	for _, tt := range tests {
		if got := LevelName(tt.level); got != tt.want {
			t.Errorf("LevelName(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLogWritesFile(t *testing.T) {
	// Reset global state for test isolation
	mu.Lock()
	logFile = nil
	logLevel = LevelOff
	initDone = false
	mu.Unlock()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Manually set up for test (bypass Init's home directory logic)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	mu.Lock()
	logFile = f
	logLevel = LevelDebug
	initDone = true
	mu.Unlock()

	Debug("hello %s", "world")
	Info("count=%d", 42)
	Warn("caution")
	Error("oops")

	Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)

	for _, want := range []string{"DBG  hello world", "INF  count=42", "WRN  caution", "ERR  oops"} {
		if !strings.Contains(content, want) {
			t.Errorf("log missing %q, got:\n%s", want, content)
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	mu.Lock()
	logFile = nil
	logLevel = LevelOff
	initDone = false
	mu.Unlock()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "filter.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	mu.Lock()
	logFile = f
	logLevel = LevelWarn // only warn and error
	initDone = true
	mu.Unlock()

	Debug("should not appear")
	Info("should not appear")
	Warn("visible warning")
	Error("visible error")

	Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "should not appear") {
		t.Errorf("filtered messages leaked:\n%s", content)
	}
	if !strings.Contains(content, "visible warning") {
		t.Errorf("warn message missing:\n%s", content)
	}
	if !strings.Contains(content, "visible error") {
		t.Errorf("error message missing:\n%s", content)
	}
}

func TestSetLevel(t *testing.T) {
	mu.Lock()
	logLevel = LevelOff
	initDone = false
	logFile = nil
	mu.Unlock()

	SetLevel(LevelInfo)
	if got := GetLevel(); got != LevelInfo {
		t.Errorf("GetLevel() = %d, want %d", got, LevelInfo)
	}

	SetLevel(LevelDebug)
	if got := GetLevel(); got != LevelDebug {
		t.Errorf("GetLevel() = %d, want %d", got, LevelDebug)
	}
}

// resetGlobals resets all package-level state for test isolation.
func resetGlobals() {
	mu.Lock()
	logFile = nil
	logLevel = LevelOff
	initDone = false
	mu.Unlock()
}

func TestInitLevelOff(t *testing.T) {
	resetGlobals()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	Init(LevelOff)
	t.Cleanup(Close)

	// With LevelOff, no log file should be created and initDone stays false.
	mu.Lock()
	done := initDone
	f := logFile
	mu.Unlock()

	if done {
		t.Error("initDone should be false after Init(LevelOff)")
	}
	if f != nil {
		t.Error("logFile should be nil after Init(LevelOff)")
	}

	// The log directory should not exist either.
	logPath := filepath.Join(tmpDir, ".local", "share", "termdesk", "termdesk.log")
	if _, err := os.Stat(logPath); err == nil {
		t.Error("log file should not exist after Init(LevelOff)")
	}
}

func TestInitCreatesLogFile(t *testing.T) {
	resetGlobals()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	Init(LevelDebug)
	t.Cleanup(Close)

	logPath := filepath.Join(tmpDir, ".local", "share", "termdesk", "termdesk.log")
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file should exist after Init(LevelDebug): %v", err)
	}
	if info.Size() == 0 {
		t.Error("log file should not be empty — Init writes a header")
	}

	// Verify global state is set correctly.
	mu.Lock()
	done := initDone
	f := logFile
	lvl := logLevel
	mu.Unlock()

	if !done {
		t.Error("initDone should be true after Init(LevelDebug)")
	}
	if f == nil {
		t.Error("logFile should not be nil after Init(LevelDebug)")
	}
	if lvl != LevelDebug {
		t.Errorf("logLevel = %d, want %d (LevelDebug)", lvl, LevelDebug)
	}
}

func TestInitIdempotent(t *testing.T) {
	resetGlobals()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	Init(LevelInfo)
	t.Cleanup(Close)

	// Capture the file pointer from the first Init call.
	mu.Lock()
	firstFile := logFile
	mu.Unlock()

	if firstFile == nil {
		t.Fatal("logFile should not be nil after first Init")
	}

	// Call Init again with a different level — it should update the level
	// but NOT open a second file.
	Init(LevelDebug)

	mu.Lock()
	secondFile := logFile
	lvl := logLevel
	mu.Unlock()

	if secondFile != firstFile {
		t.Error("second Init should reuse the same file, not open a new one")
	}
	if lvl != LevelDebug {
		t.Errorf("logLevel should be updated to LevelDebug, got %d", lvl)
	}
}

func TestInitWithDebugWritesHeader(t *testing.T) {
	resetGlobals()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	Init(LevelDebug)
	t.Cleanup(Close)

	logPath := filepath.Join(tmpDir, ".local", "share", "termdesk", "termdesk.log")

	// Sync the file before reading so the header is flushed.
	mu.Lock()
	if logFile != nil {
		logFile.Sync()
	}
	mu.Unlock()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "termdesk log") {
		t.Errorf("header missing 'termdesk log', got:\n%s", content)
	}
	if !strings.Contains(content, "level=debug") {
		t.Errorf("header missing 'level=debug', got:\n%s", content)
	}
	if !strings.Contains(content, "exe:") {
		t.Errorf("header missing 'exe:', got:\n%s", content)
	}
	if !strings.Contains(content, "go:") {
		t.Errorf("header missing 'go:', got:\n%s", content)
	}
}

func TestCloseNoFile(t *testing.T) {
	resetGlobals()

	// Close with nil logFile should not panic.
	Close()

	mu.Lock()
	f := logFile
	mu.Unlock()

	if f != nil {
		t.Error("logFile should remain nil after Close with no file")
	}
}

func TestLogWithNilFile(t *testing.T) {
	resetGlobals()

	// Set level to Debug but leave logFile nil.
	mu.Lock()
	logLevel = LevelDebug
	mu.Unlock()

	// These should not panic even though logFile is nil.
	Debug("test %s", "debug")
	Info("test %s", "info")
	Warn("test %s", "warn")
	Error("test %s", "error")
}
