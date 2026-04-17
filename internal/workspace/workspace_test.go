package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/pkg/geometry"
)

func TestWindowStateArgsRoundTrip(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID:      "win-1",
				Title:   "Dev Server",
				Command: "npm",
				Args:    []string{"run", "dev"},
				WorkDir: "/home/user/project",
				X:       10,
				Y:       5,
				Width:   80,
				Height:  24,
			},
		},
	}

	// Serialize
	toml := serializeWorkspace(state)

	// Verify args appear in TOML
	if !strings.Contains(toml, `args = ["run", "dev"]`) {
		t.Errorf("serialized TOML missing args, got:\n%s", toml)
	}

	// Parse back
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	w := parsed.Windows[0]
	if w.Command != "npm" {
		t.Errorf("command: got %q, want %q", w.Command, "npm")
	}
	if len(w.Args) != 2 || w.Args[0] != "run" || w.Args[1] != "dev" {
		t.Errorf("args: got %v, want [run dev]", w.Args)
	}
	if w.WorkDir != "/home/user/project" {
		t.Errorf("workdir: got %q, want %q", w.WorkDir, "/home/user/project")
	}
}

func TestWindowStateNoArgs(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID:      "win-1",
				Title:   "Shell",
				Command: "/bin/bash",
				// Args is nil — shell windows have no args
				WorkDir: "/home/user",
				X:       0,
				Y:       0,
				Width:   80,
				Height:  24,
			},
		},
	}

	toml := serializeWorkspace(state)

	// No args line should be present
	if strings.Contains(toml, "args") {
		t.Errorf("serialized TOML should not contain args for nil args, got:\n%s", toml)
	}

	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	if parsed.Windows[0].Args != nil {
		t.Errorf("expected nil args, got %v", parsed.Windows[0].Args)
	}
}

func TestWindowStateManyArgs(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID:      "win-2",
				Title:   "Go Test",
				Command: "go",
				Args:    []string{"test", "-v", "-run", "TestFoo", "./..."},
				WorkDir: "/home/user/project",
				X:       0,
				Y:       0,
				Width:   120,
				Height:  40,
			},
		},
	}

	toml := serializeWorkspace(state)
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	w := parsed.Windows[0]
	if len(w.Args) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(w.Args), w.Args)
	}
	expected := []string{"test", "-v", "-run", "TestFoo", "./..."}
	for i, arg := range expected {
		if w.Args[i] != arg {
			t.Errorf("arg[%d]: got %q, want %q", i, w.Args[i], arg)
		}
	}
}

func TestBufferFileRoundTrip(t *testing.T) {
	content := "hello world\nline 2\n$ some-command"

	// Save buffer to external gzip file.
	id := SaveBuffer("test-win-1", content)
	if id == "" {
		t.Fatal("SaveBuffer returned empty id")
	}
	defer func() {
		CleanupBuffers(map[string]bool{}) // remove test file
	}()

	// Load it back.
	got := LoadBuffer(id)
	if got != content {
		t.Errorf("buffer content mismatch:\n  got:  %q\n  want: %q", got, content)
	}

	// Test TOML round-trip with buffer_file reference.
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID:         "test-win-1",
				Title:      "Terminal 1",
				Command:    "/bin/bash",
				WorkDir:    "/tmp",
				Width:      80,
				Height:     24,
				BufferFile: id,
				BufferRows: 24,
				BufferCols: 80,
			},
		},
	}

	toml := serializeWorkspace(state)
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	if parsed.Windows[0].BufferFile != id {
		t.Errorf("buffer file: got %q, want %q", parsed.Windows[0].BufferFile, id)
	}
	if parsed.Windows[0].BufferRows != 24 {
		t.Errorf("buffer rows: got %d, want 24", parsed.Windows[0].BufferRows)
	}
	if parsed.Windows[0].BufferCols != 80 {
		t.Errorf("buffer cols: got %d, want 80", parsed.Windows[0].BufferCols)
	}
}

func TestLegacyBufferContentMigration(t *testing.T) {
	// Simulate a legacy workspace with inline base64 buffer_content.
	content := "legacy buffer\nline 2"
	encoded := "bGVnYWN5IGJ1ZmZlcgpsaW5lIDI=" // base64 of content
	legacyTOML := `version = 1
saved_at = "2025-01-01T00:00:00Z"
focused_id = ""

[[window]]
id = "test-win-legacy"
title = "Legacy"
command = "/bin/bash"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0
buffer_content = "` + encoded + `"
buffer_rows = 24
buffer_cols = 80
`
	defer func() {
		CleanupBuffers(map[string]bool{}) // remove test file
	}()

	var parsed WorkspaceState
	parseWorkspace(&parsed, legacyTOML)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	// Should have migrated to buffer_file reference.
	if parsed.Windows[0].BufferFile == "" {
		t.Fatal("expected buffer_file to be set after legacy migration")
	}
	// And the file should contain the original content.
	got := LoadBuffer(parsed.Windows[0].BufferFile)
	if got != content {
		t.Errorf("migrated buffer mismatch:\n  got:  %q\n  want: %q", got, content)
	}
}

func TestMultipleWindowsWithArgs(t *testing.T) {
	state := WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "win-2",
		Windows: []WindowState{
			{
				ID:      "win-1",
				Title:   "Shell",
				Command: "/bin/zsh",
				WorkDir: "/home/user",
				X:       0, Y: 0, Width: 80, Height: 24,
			},
			{
				ID:      "win-2",
				Title:   "Dev Server",
				Command: "npm",
				Args:    []string{"run", "dev"},
				WorkDir: "/home/user/project",
				X:       10, Y: 5, Width: 100, Height: 30,
			},
			{
				ID:      "win-3",
				Title:   "Editor",
				Command: "nvim",
				Args:    []string{"."},
				WorkDir: "/home/user/project",
				X:       20, Y: 10, Width: 120, Height: 40,
			},
		},
	}

	toml := serializeWorkspace(state)
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(parsed.Windows))
	}
	if parsed.FocusedID != "win-2" {
		t.Errorf("focused_id: got %q, want %q", parsed.FocusedID, "win-2")
	}

	// Window 1: no args
	if parsed.Windows[0].Args != nil {
		t.Errorf("win-1 should have nil args, got %v", parsed.Windows[0].Args)
	}

	// Window 2: npm run dev
	if len(parsed.Windows[1].Args) != 2 || parsed.Windows[1].Args[0] != "run" {
		t.Errorf("win-2 args: got %v, want [run dev]", parsed.Windows[1].Args)
	}

	// Window 3: nvim .
	if len(parsed.Windows[2].Args) != 1 || parsed.Windows[2].Args[0] != "." {
		t.Errorf("win-3 args: got %v, want [.]", parsed.Windows[2].Args)
	}
}

func TestPreMaxRectRoundTrip(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID: "win-1", Title: "Maximized", Command: "/bin/sh",
				X: 0, Y: 0, Width: 120, Height: 40,
				PreMaxRect: &geometry.Rect{X: 10, Y: 5, Width: 80, Height: 24},
			},
		},
	}

	toml := serializeWorkspace(state)
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	w := parsed.Windows[0]
	if w.PreMaxRect == nil {
		t.Fatal("expected PreMaxRect to be set")
	}
	if w.PreMaxRect.X != 10 || w.PreMaxRect.Width != 80 {
		t.Errorf("PreMaxRect: got %+v, want X=10 W=80", w.PreMaxRect)
	}
}

func TestSaveLoadWorkspace(t *testing.T) {
	dir := t.TempDir()

	state := WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "win-1",
		Windows: []WindowState{
			{
				ID: "win-1", Title: "Test", Command: "go",
				Args: []string{"test", "./..."},
				WorkDir: dir, X: 5, Y: 5, Width: 80, Height: 24,
			},
		},
		Clipboard: []string{"clip1", "clip2"},
	}

	// Save
	if err := SaveWorkspace(state, dir); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	// Verify file exists
	wsPath := filepath.Join(dir, ".termdesk-workspace.toml")
	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("workspace file not created: %v", err)
	}

	// Load
	loaded, err := LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadWorkspace returned nil")
	}

	if len(loaded.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(loaded.Windows))
	}
	w := loaded.Windows[0]
	if w.Command != "go" {
		t.Errorf("command: got %q, want %q", w.Command, "go")
	}
	if len(w.Args) != 2 || w.Args[0] != "test" {
		t.Errorf("args: got %v, want [test ./...]", w.Args)
	}
	if loaded.FocusedID != "win-1" {
		t.Errorf("focused_id: got %q, want %q", loaded.FocusedID, "win-1")
	}
	if len(loaded.Clipboard) != 2 {
		t.Errorf("clipboard: got %d items, want 2", len(loaded.Clipboard))
	}
}

func TestLoadWorkspaceMissing(t *testing.T) {
	dir := t.TempDir()

	loaded, err := LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace should not error for missing file: %v", err)
	}
	if loaded != nil {
		t.Error("LoadWorkspace should return nil for missing file")
	}
}

func TestAppStateDataRoundTrip(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID:           "win-1",
				Title:        "Calculator",
				Command:      "termdesk-calc",
				X:            10,
				Y:            5,
				Width:        40,
				Height:       20,
				AppStateData: "eyJkIjoiOCIsInIiOiI4IiwibyI6MCwibCI6MCwiaCI6ZmFsc2UsIm4iOnRydWV9",
			},
		},
	}

	dir := t.TempDir()
	err := SaveWorkspace(state, dir)
	if err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	loaded, err := LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadWorkspace returned nil")
	}
	if len(loaded.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(loaded.Windows))
	}

	ws := loaded.Windows[0]
	if ws.AppStateData != state.Windows[0].AppStateData {
		t.Errorf("AppStateData: got %q, want %q", ws.AppStateData, state.Windows[0].AppStateData)
	}
}

func TestSaveBufferEmptyContent(t *testing.T) {
	id := SaveBuffer("empty-buf", "")
	if id != "" {
		t.Errorf("SaveBuffer with empty content should return empty, got %q", id)
	}
}

func TestLoadBufferMissingFile(t *testing.T) {
	got := LoadBuffer("nonexistent-window-id-xyz")
	if got != "" {
		t.Errorf("LoadBuffer for missing file should return empty, got %q", got)
	}
}

func TestLoadBufferCorruptGzip(t *testing.T) {
	dir := buffersDir()
	if dir == "" {
		t.Skip("could not determine buffers dir")
	}
	os.MkdirAll(dir, 0755)

	// Write a file that is not valid gzip.
	path := filepath.Join(dir, "corrupt-gzip-test.buf.gz")
	os.WriteFile(path, []byte("this is not gzip data"), 0644)
	defer os.Remove(path)

	got := LoadBuffer("corrupt-gzip-test")
	if got != "" {
		t.Errorf("LoadBuffer for corrupt gzip should return empty, got %q", got)
	}
}

func TestCleanupBuffersWithMixedFiles(t *testing.T) {
	dir := buffersDir()
	if dir == "" {
		t.Skip("could not determine buffers dir")
	}
	os.MkdirAll(dir, 0755)

	// Create buffer files: one active, one stale.
	activeID := "cleanup-test-active"
	staleID := "cleanup-test-stale"

	SaveBuffer(activeID, "active content")
	SaveBuffer(staleID, "stale content")
	defer func() {
		os.Remove(filepath.Join(dir, activeID+".buf.gz"))
		os.Remove(filepath.Join(dir, staleID+".buf.gz"))
	}()

	// Create a non-.buf.gz file that should be ignored.
	nonBufPath := filepath.Join(dir, "readme.txt")
	os.WriteFile(nonBufPath, []byte("not a buffer"), 0644)
	defer os.Remove(nonBufPath)

	// Create a subdirectory that should be skipped.
	subdir := filepath.Join(dir, "subdir.buf.gz")
	os.MkdirAll(subdir, 0755)
	defer os.RemoveAll(subdir)

	// Cleanup keeping only the active buffer.
	CleanupBuffers(map[string]bool{activeID: true})

	// Active buffer should still exist.
	if LoadBuffer(activeID) == "" {
		t.Error("active buffer should survive cleanup")
	}
	// Stale buffer should be removed.
	if LoadBuffer(staleID) != "" {
		t.Error("stale buffer should be removed by cleanup")
	}
	// Non-buffer file should still exist.
	if _, err := os.Stat(nonBufPath); err != nil {
		t.Error("non-buffer file should not be removed by cleanup")
	}
	// Subdirectory should still exist.
	if _, err := os.Stat(subdir); err != nil {
		t.Error("subdirectory should not be removed by cleanup")
	}
}

func TestCleanupBuffersEmptyDir(t *testing.T) {
	// CleanupBuffers should not panic when called with nil or empty map.
	CleanupBuffers(nil)
	CleanupBuffers(map[string]bool{})
}

func TestGetWorkspacePathGlobal(t *testing.T) {
	path := GetWorkspacePath("")
	if path == "" {
		t.Fatal("GetWorkspacePath('') returned empty")
	}
	if !strings.HasSuffix(path, filepath.Join(".config", "termdesk", "workspace.toml")) {
		t.Errorf("global workspace path should end with .config/termdesk/workspace.toml, got %q", path)
	}
}

func TestGetWorkspacePathProject(t *testing.T) {
	dir := t.TempDir()
	path := GetWorkspacePath(dir)
	expected := filepath.Join(dir, ".termdesk-workspace.toml")
	if path != expected {
		t.Errorf("project workspace path: got %q, want %q", path, expected)
	}
}

func TestIsValidWorkspaceNil(t *testing.T) {
	if isValidWorkspace(nil) {
		t.Error("nil state should be invalid")
	}
}

func TestIsValidWorkspaceWrongVersion(t *testing.T) {
	state := &WorkspaceState{
		Version: 0,
		SavedAt: time.Now(),
	}
	if isValidWorkspace(state) {
		t.Error("version 0 should be invalid")
	}

	state.Version = 99
	if isValidWorkspace(state) {
		t.Error("version 99 should be invalid")
	}
}

func TestIsValidWorkspaceZeroTime(t *testing.T) {
	state := &WorkspaceState{
		Version: 1,
		SavedAt: time.Time{},
	}
	if isValidWorkspace(state) {
		t.Error("zero SavedAt should be invalid")
	}
}

func TestIsValidWorkspaceFutureTime(t *testing.T) {
	state := &WorkspaceState{
		Version: 1,
		SavedAt: time.Now().Add(48 * time.Hour),
	}
	if isValidWorkspace(state) {
		t.Error("SavedAt far in the future should be invalid")
	}
}

func TestIsValidWorkspaceValid(t *testing.T) {
	state := &WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
	}
	if !isValidWorkspace(state) {
		t.Error("valid state should pass validation")
	}
}

func TestLoadWorkspaceCorruptedInvalidStructure(t *testing.T) {
	dir := t.TempDir()

	// Write a workspace file with wrong version so isValidWorkspace fails.
	content := `version = 99
saved_at = "2025-01-01T00:00:00Z"
focused_id = ""
`
	wsPath := filepath.Join(dir, ".termdesk-workspace.toml")
	os.WriteFile(wsPath, []byte(content), 0644)

	loaded, err := LoadWorkspace(dir)
	if err == nil {
		t.Fatal("expected error for invalid workspace structure")
	}
	if loaded != nil {
		t.Error("expected nil state for corrupted workspace")
	}
	if !strings.Contains(err.Error(), "corrupted") {
		t.Errorf("error should mention corruption, got: %v", err)
	}

	// File should have been removed.
	if _, statErr := os.Stat(wsPath); statErr == nil {
		t.Error("corrupted workspace file should have been removed")
	}
}

func TestLoadWorkspaceCorruptedTooLarge(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, ".termdesk-workspace.toml")

	// Write a file larger than 10MB.
	bigData := make([]byte, 11*1024*1024)
	for i := range bigData {
		bigData[i] = 'x'
	}
	os.WriteFile(wsPath, bigData, 0644)

	loaded, err := LoadWorkspace(dir)
	if err == nil {
		t.Fatal("expected error for oversized workspace file")
	}
	if loaded != nil {
		t.Error("expected nil state for oversized workspace")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should mention 'too large', got: %v", err)
	}

	// File should have been removed.
	if _, statErr := os.Stat(wsPath); statErr == nil {
		t.Error("oversized workspace file should have been removed")
	}
}

func TestSerializeNotifications(t *testing.T) {
	now := time.Now()
	state := WorkspaceState{
		Version: 1,
		SavedAt: now,
		Notifications: []NotificationState{
			{
				Title:     "Build Complete",
				Body:      "All tests passed",
				Severity:  "info",
				CreatedAt: now,
				Read:      false,
			},
			{
				Title:     "Error",
				Body:      "Connection lost",
				Severity:  "error",
				CreatedAt: now,
				Read:      true,
			},
		},
	}

	toml := serializeWorkspace(state)

	if !strings.Contains(toml, "[[notification]]") {
		t.Error("serialized TOML should contain [[notification]] sections")
	}
	if !strings.Contains(toml, `title = "Build Complete"`) {
		t.Error("serialized TOML should contain notification title")
	}
	if !strings.Contains(toml, `severity = "error"`) {
		t.Error("serialized TOML should contain error severity")
	}
	if !strings.Contains(toml, "read = true") {
		t.Error("serialized TOML should contain read = true for read notification")
	}

	// Parse back and verify.
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(parsed.Notifications))
	}

	n0 := parsed.Notifications[0]
	if n0.Title != "Build Complete" {
		t.Errorf("notification 0 title: got %q, want %q", n0.Title, "Build Complete")
	}
	if n0.Body != "All tests passed" {
		t.Errorf("notification 0 body: got %q, want %q", n0.Body, "All tests passed")
	}
	if n0.Severity != "info" {
		t.Errorf("notification 0 severity: got %q, want %q", n0.Severity, "info")
	}
	if n0.Read {
		t.Error("notification 0 should not be read")
	}

	n1 := parsed.Notifications[1]
	if n1.Title != "Error" {
		t.Errorf("notification 1 title: got %q, want %q", n1.Title, "Error")
	}
	if n1.Severity != "error" {
		t.Errorf("notification 1 severity: got %q, want %q", n1.Severity, "error")
	}
	if !n1.Read {
		t.Error("notification 1 should be read")
	}
}

func TestSerializeMinimizedWindow(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID: "win-min", Title: "Minimized", Command: "/bin/sh",
				X: 0, Y: 0, Width: 80, Height: 24,
				Minimized: true,
			},
		},
	}

	toml := serializeWorkspace(state)
	if !strings.Contains(toml, "minimized = true") {
		t.Error("serialized TOML should contain minimized = true")
	}

	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	if !parsed.Windows[0].Minimized {
		t.Error("parsed window should be minimized")
	}
}

func TestParseWorkspaceMalformedLines(t *testing.T) {
	// Lines without "=" should be skipped. Comments and blanks too.
	content := `version = 1
saved_at = "2025-06-01T12:00:00Z"
focused_id = "win-1"

# This is a comment
this line has no equals sign

[[window]]
id = "win-1"
title = "Test"
command = "/bin/sh"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0
`
	var state WorkspaceState
	parseWorkspace(&state, content)

	if state.Version != 1 {
		t.Errorf("version: got %d, want 1", state.Version)
	}
	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	if state.Windows[0].ID != "win-1" {
		t.Errorf("window id: got %q, want %q", state.Windows[0].ID, "win-1")
	}
}

func TestParseWorkspaceEmptyContent(t *testing.T) {
	var state WorkspaceState
	parseWorkspace(&state, "")

	if state.Version != 0 {
		t.Errorf("empty content should leave version at 0, got %d", state.Version)
	}
	if len(state.Windows) != 0 {
		t.Errorf("empty content should have no windows, got %d", len(state.Windows))
	}
}

func TestParseWorkspaceWindowFollowedByNotification(t *testing.T) {
	// Tests the transition from a window section to a notification section
	// to exercise the "append last window before starting notification" logic.
	content := `version = 1
saved_at = "2025-06-01T12:00:00Z"
focused_id = ""

[[window]]
id = "win-1"
title = "Shell"
command = "/bin/sh"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0

[[notification]]
title = "Alert"
body = "Something happened"
severity = "warning"
created_at = "2025-06-01T12:00:00Z"
`
	var state WorkspaceState
	parseWorkspace(&state, content)

	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	if len(state.Notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(state.Notifications))
	}
	if state.Notifications[0].Title != "Alert" {
		t.Errorf("notification title: got %q, want %q", state.Notifications[0].Title, "Alert")
	}
	if state.Notifications[0].Severity != "warning" {
		t.Errorf("notification severity: got %q, want %q", state.Notifications[0].Severity, "warning")
	}
}

func TestParseStringArrayEmpty(t *testing.T) {
	result := parseStringArray("[]")
	if result != nil {
		t.Errorf("parseStringArray([]) should return nil, got %v", result)
	}
}

func TestParseStringArrayQuotedCommas(t *testing.T) {
	// Values containing commas inside quotes should not be split
	result := parseStringArray(`["hello, world", "foo", "a,b,c"]`)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(result), result)
	}
	if result[0] != "hello, world" {
		t.Errorf("item[0]: got %q, want %q", result[0], "hello, world")
	}
	if result[1] != "foo" {
		t.Errorf("item[1]: got %q, want %q", result[1], "foo")
	}
	if result[2] != "a,b,c" {
		t.Errorf("item[2]: got %q, want %q", result[2], "a,b,c")
	}
}

func TestParseStringArrayEscapedQuotes(t *testing.T) {
	// Escaped quotes inside values should be handled
	result := parseStringArray(`["say \"hi\"", "normal"]`)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(result), result)
	}
	if result[0] != `say "hi"` {
		t.Errorf("item[0]: got %q, want %q", result[0], `say "hi"`)
	}
	if result[1] != "normal" {
		t.Errorf("item[1]: got %q, want %q", result[1], "normal")
	}
}

func TestParseStringArraySimple(t *testing.T) {
	result := parseStringArray(`["run", "dev"]`)
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(result), result)
	}
	if result[0] != "run" {
		t.Errorf("item[0]: got %q, want %q", result[0], "run")
	}
	if result[1] != "dev" {
		t.Errorf("item[1]: got %q, want %q", result[1], "dev")
	}
}

func TestBuffersDir(t *testing.T) {
	dir := buffersDir()
	if dir == "" {
		t.Fatal("buffersDir returned empty")
	}
	if !strings.Contains(dir, "buffers") {
		t.Errorf("buffersDir should contain 'buffers', got %q", dir)
	}
}

func TestEscapeStringSpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `hello`},
		{`say "hi"`, `say \"hi\"`},
		{"line1\nline2", `line1\nline2`},
		{`back\slash`, `back\\slash`},
		{`all "three\n" here`, `all \"three\\n\" here`},
	}
	for _, tc := range tests {
		got := escapeString(tc.input)
		if got != tc.want {
			t.Errorf("escapeString(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSerializeEmptyClipboard(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
	}
	toml := serializeWorkspace(state)
	if strings.Contains(toml, "clipboard") {
		t.Error("serialized TOML should not contain clipboard when empty")
	}
}

func TestSaveWorkspaceGlobalPath(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID: "win-g", Title: "Global", Command: "/bin/sh",
				WorkDir: "/tmp", X: 0, Y: 0, Width: 80, Height: 24,
			},
		},
	}

	err := SaveWorkspace(state, "")
	if err != nil {
		t.Fatalf("SaveWorkspace to global path: %v", err)
	}

	globalPath := GetWorkspacePath("")
	defer os.Remove(globalPath)

	loaded, err := LoadWorkspace("")
	if err != nil {
		t.Fatalf("LoadWorkspace from global path: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadWorkspace from global path returned nil")
	}
	if len(loaded.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(loaded.Windows))
	}
}

func TestParseWorkspaceMultipleNotifications(t *testing.T) {
	content := `version = 1
saved_at = "2025-06-01T12:00:00Z"
focused_id = ""

[[notification]]
title = "First"
body = "First body"
severity = "info"
created_at = "2025-06-01T12:00:00Z"

[[notification]]
title = "Second"
body = "Second body"
severity = "error"
created_at = "2025-06-01T13:00:00Z"
read = true
`
	var state WorkspaceState
	parseWorkspace(&state, content)

	if len(state.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(state.Notifications))
	}
	if state.Notifications[0].Title != "First" {
		t.Errorf("notification 0 title: got %q, want %q", state.Notifications[0].Title, "First")
	}
	if state.Notifications[1].Title != "Second" {
		t.Errorf("notification 1 title: got %q, want %q", state.Notifications[1].Title, "Second")
	}
	if !state.Notifications[1].Read {
		t.Error("notification 1 should be read")
	}
}

func TestParseWorkspaceMultipleWindows(t *testing.T) {
	// Exercises the "append previous window when hitting next [[window]]" path.
	content := `version = 1
saved_at = "2025-06-01T12:00:00Z"
focused_id = ""

[[window]]
id = "a"
title = "Win A"
command = "/bin/sh"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0

[[window]]
id = "b"
title = "Win B"
command = "/bin/bash"
workdir = "/home"
x = 10
y = 10
width = 100
height = 30
zindex = 1
minimized = true
`
	var state WorkspaceState
	parseWorkspace(&state, content)

	if len(state.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(state.Windows))
	}
	if state.Windows[0].ID != "a" {
		t.Errorf("window 0 ID: got %q, want %q", state.Windows[0].ID, "a")
	}
	if state.Windows[1].ID != "b" {
		t.Errorf("window 1 ID: got %q, want %q", state.Windows[1].ID, "b")
	}
	if !state.Windows[1].Minimized {
		t.Error("window 1 should be minimized")
	}
}

func TestLoadWorkspaceZeroVersionMissingTime(t *testing.T) {
	dir := t.TempDir()

	// A file with missing version and saved_at should fail validation.
	content := `focused_id = "win-1"

[[window]]
id = "win-1"
title = "Test"
command = "/bin/sh"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0
`
	wsPath := filepath.Join(dir, ".termdesk-workspace.toml")
	os.WriteFile(wsPath, []byte(content), 0644)

	loaded, err := LoadWorkspace(dir)
	if err == nil {
		t.Fatal("expected error for workspace with missing version/time")
	}
	if loaded != nil {
		t.Error("expected nil state")
	}
}

func TestSerializeWorkspaceNoWindows(t *testing.T) {
	state := WorkspaceState{
		Version:   1,
		SavedAt:   time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		FocusedID: "",
	}
	toml := serializeWorkspace(state)

	if !strings.Contains(toml, "version = 1") {
		t.Error("serialized TOML should contain version")
	}
	if strings.Contains(toml, "[[window]]") {
		t.Error("serialized TOML should not contain window sections when there are none")
	}
}

func TestProjectDirFromPathGlobal(t *testing.T) {
	// Global workspace path ends with workspace.toml — should return "".
	globalPath := filepath.Join("/home", "user", ".config", "termdesk", "workspace.toml")
	got := ProjectDirFromPath(globalPath)
	if got != "" {
		t.Errorf("ProjectDirFromPath(%q) = %q, want empty for global workspace", globalPath, got)
	}
}

func TestProjectDirFromPathProject(t *testing.T) {
	// Project workspace path ends with .termdesk-workspace.toml — should return the directory.
	projectPath := filepath.Join("/home", "user", "myproject", ".termdesk-workspace.toml")
	got := ProjectDirFromPath(projectPath)
	want := filepath.Join("/home", "user", "myproject")
	if got != want {
		t.Errorf("ProjectDirFromPath(%q) = %q, want %q", projectPath, got, want)
	}
}

func TestParsePaneFieldAllFields(t *testing.T) {
	w := &WindowState{
		Panes: make([]PaneState, 2),
	}

	// Parse all pane fields for pane 0.
	parsePaneField(w, "pane_0_id", `"term-0"`)
	parsePaneField(w, "pane_0_command", `"/bin/bash"`)
	parsePaneField(w, "pane_0_args", `["--login", "-i"]`)
	parsePaneField(w, "pane_0_workdir", `"/home/user"`)
	parsePaneField(w, "pane_0_buffer_file", `"buf-0"`)
	parsePaneField(w, "pane_0_buffer_rows", "24")
	parsePaneField(w, "pane_0_buffer_cols", "80")

	// Parse pane 1 fields.
	parsePaneField(w, "pane_1_id", `"term-1"`)
	parsePaneField(w, "pane_1_command", `"vim"`)
	parsePaneField(w, "pane_1_workdir", `"/tmp"`)

	// Verify pane 0.
	p := w.Panes[0]
	if p.TermID != "term-0" {
		t.Errorf("pane[0].TermID = %q, want %q", p.TermID, "term-0")
	}
	if p.Command != "/bin/bash" {
		t.Errorf("pane[0].Command = %q, want %q", p.Command, "/bin/bash")
	}
	if len(p.Args) != 2 || p.Args[0] != "--login" || p.Args[1] != "-i" {
		t.Errorf("pane[0].Args = %v, want [--login -i]", p.Args)
	}
	if p.WorkDir != "/home/user" {
		t.Errorf("pane[0].WorkDir = %q, want %q", p.WorkDir, "/home/user")
	}
	if p.BufferFile != "buf-0" {
		t.Errorf("pane[0].BufferFile = %q, want %q", p.BufferFile, "buf-0")
	}
	if p.BufferRows != 24 {
		t.Errorf("pane[0].BufferRows = %d, want 24", p.BufferRows)
	}
	if p.BufferCols != 80 {
		t.Errorf("pane[0].BufferCols = %d, want 80", p.BufferCols)
	}

	// Verify pane 1.
	p1 := w.Panes[1]
	if p1.TermID != "term-1" {
		t.Errorf("pane[1].TermID = %q, want %q", p1.TermID, "term-1")
	}
	if p1.Command != "vim" {
		t.Errorf("pane[1].Command = %q, want %q", p1.Command, "vim")
	}
	if p1.WorkDir != "/tmp" {
		t.Errorf("pane[1].WorkDir = %q, want %q", p1.WorkDir, "/tmp")
	}
}

func TestParsePaneFieldIgnoresNonPaneKey(t *testing.T) {
	w := &WindowState{}
	// Non-pane key should be ignored silently.
	parsePaneField(w, "title", `"hello"`)
	if len(w.Panes) != 0 {
		t.Errorf("non-pane key should not add panes, got %d", len(w.Panes))
	}
}

func TestParsePaneFieldNoUnderscore(t *testing.T) {
	w := &WindowState{}
	// "pane_xyz" has no second underscore — should be ignored.
	parsePaneField(w, "pane_xyz", `"hello"`)
	if len(w.Panes) != 0 {
		t.Errorf("pane key without proper index+field should not add panes, got %d", len(w.Panes))
	}
}

func TestParsePaneFieldGrowsSlice(t *testing.T) {
	w := &WindowState{}
	// Parse pane_2 on an empty Panes slice — should auto-grow.
	parsePaneField(w, "pane_2_id", `"term-2"`)
	if len(w.Panes) != 3 {
		t.Fatalf("Panes should have been grown to 3, got %d", len(w.Panes))
	}
	if w.Panes[2].TermID != "term-2" {
		t.Errorf("pane[2].TermID = %q, want %q", w.Panes[2].TermID, "term-2")
	}
}

func TestUnescapeStringBackslashSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "hello", "hello"},
		{"escaped_newline", `line1\nline2`, "line1\nline2"},
		{"escaped_quote", `say \"hi\"`, `say "hi"`},
		{"escaped_backslash", `back\\slash`, `back\slash`},
		{"mixed", `a\\b\nc\"d`, "a\\b\nc\"d"},
		{"trailing_backslash", `end\`, `end\`},
		{"unknown_escape", `\t remains`, `\t remains`},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unescapeString(tc.input)
			if got != tc.want {
				t.Errorf("unescapeString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestExtractQuotedValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"quoted_simple", `"hello"`, "hello"},
		{"quoted_with_space", `"hello world"`, "hello world"},
		{"quoted_with_escaped_quote", `"say \"hi\""`, `say \"hi\"`},
		{"unquoted_number", "42", "42"},
		{"unquoted_bool", "true", "true"},
		{"no_closing_quote", `"unclosed`, "unclosed"},
		{"empty_quoted", `""`, ""},
		{"with_leading_space", `  "trimmed"`, "trimmed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractQuotedValue(tc.input)
			if got != tc.want {
				t.Errorf("extractQuotedValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestHomeDirOverride(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(envHomeDir, tmpDir)
	got := homeDir()
	if got != tmpDir {
		t.Errorf("homeDir() with override = %q, want %q", got, tmpDir)
	}
}

func TestHomeDirOverrideWhitespace(t *testing.T) {
	// Empty/whitespace override should fall back to os.UserHomeDir.
	t.Setenv(envHomeDir, "  ")
	got := homeDir()
	if got == "" {
		t.Error("homeDir() with whitespace override should fall back, not return empty")
	}
}

func TestHomeDirDefault(t *testing.T) {
	t.Setenv(envHomeDir, "")
	got := homeDir()
	if got == "" {
		t.Fatal("homeDir() should return a non-empty home directory")
	}
}

func TestSaveBufferEmptyBuffersDir(t *testing.T) {
	// When TERMDESK_HOME points to a nonexistent path that homeDir returns empty,
	// SaveBuffer should return "".
	// Actually, empty home means empty buffersDir. Let's unset HOME too.
	t.Setenv(envHomeDir, "")
	// Cannot easily force os.UserHomeDir to fail, but we can test the content="" case.
	id := SaveBuffer("test-id", "")
	if id != "" {
		t.Errorf("SaveBuffer with empty content should return empty, got %q", id)
	}
}

func TestSplitPaneRoundTrip(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID: "win-split", Title: "Split", Command: "/bin/bash",
				WorkDir: "/tmp", X: 0, Y: 0, Width: 120, Height: 40,
				SplitTree:   "H|50",
				FocusedPane: "term-0",
				Panes: []PaneState{
					{
						TermID:     "term-0",
						Command:    "/bin/bash",
						Args:       []string{"--login"},
						WorkDir:    "/home/user",
						BufferFile: "",
					},
					{
						TermID:  "term-1",
						Command: "vim",
						WorkDir: "/tmp",
					},
				},
			},
		},
	}

	toml := serializeWorkspace(state)

	// Verify serialization includes pane data.
	if !strings.Contains(toml, `split_tree = "H|50"`) {
		t.Error("serialized TOML missing split_tree")
	}
	if !strings.Contains(toml, `focused_pane = "term-0"`) {
		t.Error("serialized TOML missing focused_pane")
	}
	if !strings.Contains(toml, "pane_count = 2") {
		t.Error("serialized TOML missing pane_count")
	}
	if !strings.Contains(toml, `pane_0_id = "term-0"`) {
		t.Error("serialized TOML missing pane_0_id")
	}

	// Parse back and verify.
	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	w := parsed.Windows[0]
	if w.SplitTree != "H|50" {
		t.Errorf("SplitTree = %q, want %q", w.SplitTree, "H|50")
	}
	if w.FocusedPane != "term-0" {
		t.Errorf("FocusedPane = %q, want %q", w.FocusedPane, "term-0")
	}
	if len(w.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(w.Panes))
	}
	if w.Panes[0].TermID != "term-0" {
		t.Errorf("pane[0].TermID = %q, want %q", w.Panes[0].TermID, "term-0")
	}
	if len(w.Panes[0].Args) != 1 || w.Panes[0].Args[0] != "--login" {
		t.Errorf("pane[0].Args = %v, want [--login]", w.Panes[0].Args)
	}
	if w.Panes[1].Command != "vim" {
		t.Errorf("pane[1].Command = %q, want %q", w.Panes[1].Command, "vim")
	}
}

func TestExtractArrayValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple_array", `["a", "b"]`, `["a", "b"]`},
		{"nested", `[[1, 2], [3]]`, `[[1, 2], [3]]`},
		{"not_array", `hello`, `hello`},
		{"empty_array", `[]`, `[]`},
		{"unclosed", `[a, b`, `[a, b`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractArrayValue(tc.input)
			if got != tc.want {
				t.Errorf("extractArrayValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResizableDefaultTrue(t *testing.T) {
	// When parsing a window without an explicit resizable field, it should default to true.
	content := `version = 1
saved_at = "2025-06-01T12:00:00Z"
focused_id = ""

[[window]]
id = "win-1"
title = "Test"
command = "/bin/sh"
workdir = "/tmp"
x = 0
y = 0
width = 80
height = 24
zindex = 0
`
	var state WorkspaceState
	parseWorkspace(&state, content)

	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	if !state.Windows[0].Resizable {
		t.Error("window without explicit resizable should default to true")
	}
}

func TestResizableFalseRoundTrip(t *testing.T) {
	state := WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []WindowState{
			{
				ID: "win-1", Title: "Fixed", Command: "/bin/sh",
				X: 0, Y: 0, Width: 40, Height: 20,
				Resizable: false,
			},
		},
	}

	toml := serializeWorkspace(state)
	if !strings.Contains(toml, "resizable = false") {
		t.Error("serialized TOML should contain resizable = false")
	}

	var parsed WorkspaceState
	parseWorkspace(&parsed, toml)

	if len(parsed.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(parsed.Windows))
	}
	if parsed.Windows[0].Resizable {
		t.Error("parsed window should have Resizable = false")
	}
}
