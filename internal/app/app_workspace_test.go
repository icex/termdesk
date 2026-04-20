package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"

	tea "charm.land/bubbletea/v2"
)

func setTestConfigPath(t *testing.T, dir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("TERMDESK_CONFIG_PATH", cfgPath)
	return cfgPath
}

func TestCaptureWorkspaceStateIncludesArgs(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("go", []string{"test", "./..."}, "Tests", "/home/user")
	m = completeAnimations(m)

	state := m.captureWorkspaceState()
	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window in state, got %d", len(state.Windows))
	}
	ws := state.Windows[0]
	if ws.Command == "" {
		t.Error("expected non-empty command in workspace state")
	}
	if len(ws.Args) != 2 || ws.Args[0] != "test" || ws.Args[1] != "./..." {
		t.Errorf("args in workspace state: got %v, want [test ./...]", ws.Args)
	}
}

func TestAutoStartSkipsWhenPending(t *testing.T) {
	m := setupReadyModel()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: "/tmp",
		AutoStart: []config.AutoStartItem{
			{Command: "echo", Args: []string{"hello"}, Directory: ".", Title: "Echo"},
		},
	}
	m.workspaceRestorePending = true
	m.autoStartTriggered = false

	// WindowSizeMsg should NOT trigger auto-start when workspace restore is pending
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.autoStartTriggered {
		t.Error("auto-start should not be triggered when workspace restore is pending")
	}
}

func TestAutoStartFiresWhenNoPending(t *testing.T) {
	m := setupReadyModel()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: "/tmp",
		AutoStart: []config.AutoStartItem{
			{Command: "/bin/echo", Args: []string{"hello"}, Directory: ".", Title: "Echo"},
		},
	}
	m.workspaceRestorePending = false
	m.autoStartTriggered = false

	// WindowSizeMsg should trigger auto-start when no workspace restore pending
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if !m.autoStartTriggered {
		t.Error("auto-start should be triggered when no workspace restore is pending")
	}
}

func TestAutoStartSkipsAlreadyRunning(t *testing.T) {
	m := setupReadyModel()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: "/tmp",
		AutoStart: []config.AutoStartItem{
			{Command: "already-running", Title: "Running App"},
		},
	}

	// Create a window with the same command (simulating workspace restore)
	m.openDemoWindow()
	m.wm.Windows()[0].Command = "already-running"

	before := m.wm.Count()
	m.runProjectAutoStart()

	if m.wm.Count() != before {
		t.Errorf("auto-start should skip already-running commands: count went from %d to %d", before, m.wm.Count())
	}
}

func TestAutoStartLaunchesNewCommands(t *testing.T) {
	m := setupReadyModel()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: "/tmp",
		AutoStart: []config.AutoStartItem{
			{Command: "/bin/echo", Args: []string{"new"}, Title: "New App"},
		},
	}

	// Create a window with a DIFFERENT command
	m.openDemoWindow()
	m.wm.Windows()[0].Command = "other-command"

	before := m.wm.Count()
	m.runProjectAutoStart()

	if m.wm.Count() != before+1 {
		t.Errorf("auto-start should launch new command: count was %d, now %d", before, m.wm.Count())
	}
}

func TestAutoStartMsgSentAfterWorkspaceRestore(t *testing.T) {
	m := setupReadyModel()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: "/tmp",
		AutoStart: []config.AutoStartItem{
			{Command: "/bin/echo", Title: "Echo"},
		},
	}
	m.workspaceRestorePending = true

	// Process WorkspaceRestoreMsg (no workspace file exists, so restore is empty)
	updated, cmd := m.Update(WorkspaceRestoreMsg{})
	m = updated.(Model)

	if m.workspaceRestorePending {
		t.Error("workspaceRestorePending should be false after processing")
	}
	if !m.autoStartTriggered {
		t.Error("autoStartTriggered should be true after workspace restore")
	}

	// The cmd should produce AutoStartMsg
	if cmd == nil {
		t.Fatal("expected a cmd to be returned for auto-start")
	}
	msg := cmd()
	if _, ok := msg.(AutoStartMsg); !ok {
		t.Errorf("expected AutoStartMsg, got %T", msg)
	}
}

func TestWorkspaceRestoreDeferredUntilReady(t *testing.T) {
	m := New()
	m.workspaceRestorePending = true
	m.ready = false

	updated, cmd := m.Update(WorkspaceRestoreMsg{})
	m = updated.(Model)

	if !m.workspaceRestorePending {
		t.Fatal("workspace restore should remain pending until model is ready")
	}
	if cmd == nil {
		t.Fatal("expected retry cmd while waiting for initial window size")
	}
	msg := cmd()
	if _, ok := msg.(WorkspaceRestoreMsg); !ok {
		t.Fatalf("expected WorkspaceRestoreMsg retry, got %T", msg)
	}
}

func TestWorkspaceRestoreDeferredUntilWindowSizeSettles(t *testing.T) {
	m := New()
	m.workspaceRestorePending = true
	m.ready = true
	m.lastWindowSizeAt = time.Now()

	updated, cmd := m.Update(WorkspaceRestoreMsg{})
	m = updated.(Model)

	if !m.workspaceRestorePending {
		t.Fatal("workspace restore should remain pending while size is settling")
	}
	if cmd == nil {
		t.Fatal("expected retry cmd while waiting for size settle")
	}
	if _, ok := cmd().(WorkspaceRestoreMsg); !ok {
		t.Fatalf("expected WorkspaceRestoreMsg retry, got %T", cmd())
	}
}

func TestWorkspaceRestorePreservesSavedGeometryWhenTilingEnabled(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	state := workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "w1", Title: "W1", Command: shell, X: 1, Y: 1, Width: 30, Height: 8},
			{ID: "w2", Title: "W2", Command: shell, X: 40, Y: 5, Width: 20, Height: 6},
		},
	}
	if err := workspace.SaveWorkspace(state, ""); err != nil {
		t.Fatalf("failed to save workspace fixture: %v", err)
	}

	m := setupReadyModel()
	m.tilingMode = true
	m.tilingLayout = "columns"
	m.workspaceRestorePending = true
	m.projectConfig = nil

	updated, _ := m.Update(WorkspaceRestoreMsg{})
	m = updated.(Model)

	if m.workspaceRestorePending {
		t.Fatal("workspaceRestorePending should be false after restore")
	}
	if m.wm.Count() != 2 {
		t.Fatalf("expected 2 restored windows, got %d", m.wm.Count())
	}
	for _, w := range m.wm.Windows() {
		switch w.ID {
		case "w1":
			if w.Rect.X != 1 || w.Rect.Y != 1 || w.Rect.Width != 30 || w.Rect.Height != 8 {
				t.Fatalf("w1 rect = %+v, want {X:1 Y:1 Width:30 Height:8}", w.Rect)
			}
		case "w2":
			if w.Rect.X != 40 || w.Rect.Y != 5 || w.Rect.Width != 20 || w.Rect.Height != 6 {
				t.Fatalf("w2 rect = %+v, want {X:40 Y:5 Width:20 Height:6}", w.Rect)
			}
		default:
			t.Fatalf("unexpected restored window ID: %s", w.ID)
		}
	}

	for _, term := range m.terminals {
		term.Close()
	}
}

// --- saveWorkspaceNow tests ---

func TestSaveWorkspaceNowGlobalWorkspace(t *testing.T) {
	m := setupReadyModel()

	// Use a temp home override so workspace saves to a predictable location
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	m.projectConfig = nil // global workspace

	m.saveWorkspaceNow()
	time.Sleep(100 * time.Millisecond) // wait for async goroutine

	wsPath := filepath.Join(tmpHome, ".config", "termdesk", "workspace.toml")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("expected workspace file at %s to exist after saveWorkspaceNow", wsPath)
	}
}

func TestSaveWorkspaceNowProjectWorkspace(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: tmpDir,
	}

	m.saveWorkspaceNow()
	time.Sleep(100 * time.Millisecond) // wait for async goroutine

	wsPath := filepath.Join(tmpDir, ".termdesk-workspace.toml")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("expected project workspace file at %s to exist", wsPath)
	}

	// Verify the timestamp was updated
	if m.lastWorkspaceSave.IsZero() {
		t.Error("expected lastWorkspaceSave to be updated")
	}
}

func TestSaveWorkspaceNowUpdatesTimestamp(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: tmpDir,
	}

	before := time.Now()
	m.saveWorkspaceNow()
	time.Sleep(100 * time.Millisecond) // wait for async goroutine to finish writing

	if m.lastWorkspaceSave.Before(before) {
		t.Error("lastWorkspaceSave should be >= time before save")
	}
}

// --- toggleWorkspacePicker tests ---

func TestToggleWorkspacePickerOpensAndCloses(t *testing.T) {
	m := setupReadyModel()

	// Create a fake workspace file so picker won't immediately close
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create a global workspace file for discovery
	configDir := filepath.Join(tmpHome, ".config", "termdesk")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "workspace.toml"), []byte("version = 1\n"), 0644)

	// Toggle on — async discovery returns a tea.Cmd
	cmd := m.toggleWorkspacePicker()
	if !m.workspacePickerVisible {
		t.Error("expected workspacePickerVisible to be true after first toggle")
	}
	if m.workspacePickerSelected != 0 {
		t.Error("expected workspacePickerSelected to be 0")
	}

	// Simulate discovery result
	if cmd != nil {
		msg := cmd()
		if dm, ok := msg.(WorkspaceDiscoveryMsg); ok {
			m.workspaceList = dm.Workspaces
			m.workspaceWindowCounts = dm.WindowCounts
		}
	}
	if len(m.workspaceList) == 0 {
		t.Error("expected at least one workspace after discovery")
	}

	// Toggle off
	m.toggleWorkspacePicker()
	if m.workspacePickerVisible {
		t.Error("expected workspacePickerVisible to be false after second toggle")
	}
}

func TestToggleWorkspacePickerNoWorkspacesClosesOnDiscovery(t *testing.T) {
	m := setupReadyModel()

	// Use a temp home override with no workspace files
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	cmd := m.toggleWorkspacePicker()
	// Picker is initially visible while discovery runs
	if !m.workspacePickerVisible {
		t.Error("picker should be visible while waiting for discovery")
	}

	// Simulate empty discovery result via Update (value semantics)
	if cmd != nil {
		msg := cmd()
		ret, _ := m.handleUpdate(msg)
		m = ret.(Model)
	}

	// Should close after receiving empty discovery results
	if m.workspacePickerVisible {
		t.Error("picker should not stay visible when no workspaces exist")
	}
}

// --- discoverWorkspaces tests ---

func TestDiscoverWorkspacesFindsGlobalWorkspace(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create global workspace file
	cfgDir := filepath.Join(tmpHome, ".config", "termdesk")
	os.MkdirAll(cfgDir, 0755)
	globalWs := filepath.Join(cfgDir, "workspace.toml")
	os.WriteFile(globalWs, []byte("version = 1\n"), 0644)

	workspaces := discoverWorkspaces()
	found := false
	for _, ws := range workspaces {
		if ws == globalWs {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected global workspace at %s in results %v", globalWs, workspaces)
	}
}

func TestDiscoverWorkspacesFindsProjectWorkspaces(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create a project workspace in a "Projects" subdirectory
	projectDir := filepath.Join(tmpHome, "Projects", "myapp")
	os.MkdirAll(projectDir, 0755)
	wsFile := filepath.Join(projectDir, ".termdesk-workspace.toml")
	os.WriteFile(wsFile, []byte("version = 1\n"), 0644)

	workspaces := discoverWorkspaces()
	found := false
	for _, ws := range workspaces {
		if ws == wsFile {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected project workspace at %s in results %v", wsFile, workspaces)
	}
}

func TestDiscoverWorkspacesReturnsEmptyWithNoFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	workspaces := discoverWorkspaces()
	if len(workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d: %v", len(workspaces), workspaces)
	}
}

func TestDiscoverWorkspacesSkipsDeepDirectories(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create a workspace file too deep (depth > 3)
	deepDir := filepath.Join(tmpHome, "Projects", "a", "b", "c", "d")
	os.MkdirAll(deepDir, 0755)
	deepWs := filepath.Join(deepDir, ".termdesk-workspace.toml")
	os.WriteFile(deepWs, []byte("version = 1\n"), 0644)

	workspaces := discoverWorkspaces()
	for _, ws := range workspaces {
		if ws == deepWs {
			t.Errorf("should not find workspace at depth > 3: %s", deepWs)
		}
	}
}

func TestDiscoverWorkspacesSkipsHiddenDirs(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	// Create a workspace file inside a hidden directory
	hiddenDir := filepath.Join(tmpHome, "Projects", ".hidden-project")
	os.MkdirAll(hiddenDir, 0755)
	hiddenWs := filepath.Join(hiddenDir, ".termdesk-workspace.toml")
	os.WriteFile(hiddenWs, []byte("version = 1\n"), 0644)

	workspaces := discoverWorkspaces()
	for _, ws := range workspaces {
		if ws == hiddenWs {
			t.Errorf("should not find workspace inside hidden dir: %s", hiddenWs)
		}
	}
}

// --- renderWorkspacePicker tests ---

func TestRenderWorkspacePickerWithWorkspaces(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspacePickerSelected = 0
	m.workspaceList = []string{
		"/home/user/.config/termdesk/workspace.toml",
		"/home/user/Projects/myapp/.termdesk-workspace.toml",
	}

	buf := NewBuffer(m.width, m.height, m.theme.DesktopBg)
	m.renderWorkspacePicker(buf)

	// Check that "Load Workspace" title was rendered somewhere in the buffer
	found := false
	for y := 0; y < buf.Height; y++ {
		var line strings.Builder
		for x := 0; x < buf.Width; x++ {
			line.WriteRune(buf.Cells[y][x].Char)
		}
		if strings.Contains(line.String(), "Load Workspace") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Load Workspace' title in rendered picker")
	}
}

func TestRenderWorkspacePickerEmpty(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspaceList = nil

	buf := NewBuffer(m.width, m.height, m.theme.DesktopBg)
	// Should be a no-op when workspace list is empty (no panic)
	m.renderWorkspacePicker(buf)
}

func TestRenderWorkspacePickerHighlightsSelected(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspacePickerSelected = 1
	m.workspaceList = []string{
		"/home/user/.config/termdesk/workspace.toml",
		"/home/user/Projects/myapp/.termdesk-workspace.toml",
	}

	buf := NewBuffer(m.width, m.height, m.theme.DesktopBg)
	m.renderWorkspacePicker(buf)

	// The selected item (index 1) should have '>' prefix
	found := false
	for y := 0; y < buf.Height; y++ {
		var line strings.Builder
		for x := 0; x < buf.Width; x++ {
			line.WriteRune(buf.Cells[y][x].Char)
		}
		lineStr := line.String()
		if strings.Contains(lineStr, ">") && strings.Contains(lineStr, "myapp") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '>' indicator on selected workspace item (myapp)")
	}
}

func TestRenderWorkspacePickerGlobalLabel(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspacePickerSelected = 0
	m.workspaceList = []string{
		"/home/user/.config/termdesk/workspace.toml",
	}

	buf := NewBuffer(m.width, m.height, m.theme.DesktopBg)
	m.renderWorkspacePicker(buf)

	// Global workspace should be labeled "Global Workspace"
	found := false
	for y := 0; y < buf.Height; y++ {
		var line strings.Builder
		for x := 0; x < buf.Width; x++ {
			line.WriteRune(buf.Cells[y][x].Char)
		}
		if strings.Contains(line.String(), "Global Workspace") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Global Workspace' label for global workspace path")
	}
}

func TestRenderWorkspacePickerFooterHint(t *testing.T) {
	m := setupReadyModel()
	m.workspacePickerVisible = true
	m.workspacePickerSelected = 0
	m.workspaceList = []string{
		"/home/user/.config/termdesk/workspace.toml",
	}

	buf := NewBuffer(m.width, m.height, m.theme.DesktopBg)
	m.renderWorkspacePicker(buf)

	// Footer should contain navigation hints
	found := false
	for y := 0; y < buf.Height; y++ {
		var line strings.Builder
		for x := 0; x < buf.Width; x++ {
			line.WriteRune(buf.Cells[y][x].Char)
		}
		if strings.Contains(line.String(), "Navigate") && strings.Contains(line.String(), "Enter") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected footer hint with 'Navigate' and 'Enter' in rendered picker")
	}
}

// --- switch/load workspace tests ---

func TestSwitchToProjectLoadsWorkspaceAndAutoStart(t *testing.T) {
	m := setupReadyModel()

	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	currentProject := filepath.Join(tmpHome, "current")
	newProject := filepath.Join(tmpHome, "newproj")
	os.MkdirAll(currentProject, 0755)
	os.MkdirAll(newProject, 0755)

	// Set current project config
	m.projectConfig = &config.ProjectConfig{ProjectDir: currentProject}

	// Create a workspace for the new project
	ws := workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "",
		Windows: []workspace.WindowState{
			{
				ID:      "win1",
				Title:   "Test",
				Command: "/bin/sh",
				Args:    nil,
				WorkDir: newProject,
				X:       1,
				Y:       1,
				Width:   40,
				Height:  10,
				ZIndex:  0,
			},
		},
	}
	if err := workspace.SaveWorkspace(ws, newProject); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	// Create project config with autostart in new project
	cfgPath := filepath.Join(newProject, ".termdesk.toml")
	os.WriteFile(cfgPath, []byte(`[[autostart]]
command = "/bin/echo"
args = ["auto"]
directory = "."
title = "Auto"
`), 0644)

	cmd := m.switchToProject(newProject)
	if m.projectConfig == nil || m.projectConfig.ProjectDir != newProject {
		t.Fatalf("expected projectConfig to be %s", newProject)
	}
	if m.wm.Count() == 0 {
		t.Fatal("expected restored window(s) after switchToProject")
	}
	if cmd == nil {
		t.Fatal("expected autostart cmd when project has autostart entries")
	}
}

func TestLoadSelectedWorkspaceLoadsAndRecordsHistory(t *testing.T) {
	m := setupReadyModel()

	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	projectDir := filepath.Join(tmpHome, "Projects", "wsproj")
	os.MkdirAll(projectDir, 0755)

	ws := workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "",
		Windows: []workspace.WindowState{
			{
				ID:      "win1",
				Title:   "Test",
				Command: "/bin/echo",
				Args:    []string{"hello"},
				WorkDir: projectDir,
				X:       1,
				Y:       1,
				Width:   40,
				Height:  10,
				ZIndex:  0,
			},
		},
	}
	if err := workspace.SaveWorkspace(ws, projectDir); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	wsPath := filepath.Join(projectDir, ".termdesk-workspace.toml")
	m.workspaceList = []string{wsPath}
	m.workspacePickerSelected = 0

	cmd := m.loadSelectedWorkspace()
	if cmd != nil {
		t.Error("expected nil cmd from loadSelectedWorkspace")
	}
	if m.wm.Count() == 0 {
		t.Fatal("expected window(s) after loadSelectedWorkspace")
	}
	if len(m.notifications.HistoryItems()) == 0 {
		t.Fatal("expected notification after workspace load")
	}
}

func TestLoadWorkspaceFromHistoryLoadsWorkspace(t *testing.T) {
	m := setupReadyModel()

	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	projectDir := filepath.Join(tmpHome, "Projects", "histproj")
	os.MkdirAll(projectDir, 0755)

	ws := workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "",
		Windows: []workspace.WindowState{
			{
				ID:      "win1",
				Title:   "Test",
				Command: "/bin/echo",
				Args:    []string{"hello"},
				WorkDir: projectDir,
				X:       1,
				Y:       1,
				Width:   40,
				Height:  10,
				ZIndex:  0,
			},
		},
	}
	if err := workspace.SaveWorkspace(ws, projectDir); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	wsPath := filepath.Join(projectDir, ".termdesk-workspace.toml")
	cmd := m.loadWorkspaceFromHistory(wsPath)
	if cmd != nil {
		t.Error("expected nil cmd from loadWorkspaceFromHistory")
	}
	if m.wm.Count() == 0 {
		t.Fatal("expected window(s) after loadWorkspaceFromHistory")
	}
	if len(m.notifications.HistoryItems()) == 0 {
		t.Fatal("expected notification after workspace load")
	}
}

func TestLoadRecentWorkspace(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	setTestConfigPath(t, tmpDir)

	projectDir := t.TempDir()
	ws := workspace.WorkspaceState{
		Version:   1,
		FocusedID: "",
		Windows: []workspace.WindowState{
			{
				ID:      "win1",
				Title:   "Test",
				Command: "/bin/echo",
				Args:    []string{"hello"},
				WorkDir: projectDir,
				X:       1,
				Y:       1,
				Width:   40,
				Height:  10,
				ZIndex:  0,
			},
		},
	}
	if err := workspace.SaveWorkspace(ws, projectDir); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	wsPath := filepath.Join(projectDir, ".termdesk-workspace.toml")
	cfg := config.LoadUserConfig()
	cfg.RecentWorkspaces = []config.WorkspaceHistoryEntry{
		{Path: wsPath, LastAccess: time.Now()},
	}
	if err := config.SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	cmd := m.loadRecentWorkspace(0)
	if cmd != nil {
		t.Error("expected nil cmd from loadRecentWorkspace")
	}
	if m.wm.Count() == 0 {
		t.Fatal("expected window(s) after loadRecentWorkspace")
	}
}

func TestLoadRecentWorkspaceMissing(t *testing.T) {
	m := setupReadyModel()
	tmpDir := t.TempDir()
	setTestConfigPath(t, tmpDir)

	cmd := m.loadRecentWorkspace(1)
	if cmd != nil {
		t.Error("expected nil cmd from loadRecentWorkspace with missing entry")
	}
	if len(m.notifications.HistoryItems()) == 0 {
		t.Fatal("expected notification for missing workspace slot")
	}
}

// --- createNewWorkspace tests ---

func TestCreateNewWorkspaceCreatesFiles(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "newproject")

	m.createNewWorkspace("TestProject", wsDir)

	// Check workspace file was created
	wsPath := filepath.Join(wsDir, ".termdesk-workspace.toml")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("expected workspace file at %s", wsPath)
	}

	// Check project config was created
	cfgPath := filepath.Join(wsDir, ".termdesk.toml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatalf("expected project config at %s", cfgPath)
	}
}

func TestCreateNewWorkspaceProjectConfigContent(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "myproject")

	m.createNewWorkspace("MyProject", wsDir)

	cfgPath := filepath.Join(wsDir, ".termdesk.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read project config: %v", err)
	}
	if !strings.Contains(string(data), "MyProject") {
		t.Error("expected project name in config file content")
	}
}

func TestCreateNewWorkspaceExistingDirectory(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	// Directory already exists -- should still work
	m.createNewWorkspace("Existing", tmpDir)

	wsPath := filepath.Join(tmpDir, ".termdesk-workspace.toml")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("expected workspace file even when directory already exists")
	}
}

func TestCreateNewWorkspaceDoesNotOverwriteExistingConfig(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, ".termdesk.toml")
	existingContent := "# existing config\n"
	os.WriteFile(cfgPath, []byte(existingContent), 0644)

	m.createNewWorkspace("Test", tmpDir)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(data) != existingContent {
		t.Error("expected existing .termdesk.toml to not be overwritten")
	}
}

func TestCreateNewWorkspaceBlankState(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	wsDir := filepath.Join(tmpDir, "blank")

	m.createNewWorkspace("Blank", wsDir)

	// Load the workspace and verify it has no windows
	loaded, err := workspace.LoadWorkspace(wsDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded workspace")
	}
	if len(loaded.Windows) != 0 {
		t.Errorf("expected 0 windows in blank workspace, got %d", len(loaded.Windows))
	}
}

// --- recordWorkspaceAccess tests ---

func TestRecordWorkspaceAccessAddsEntry(t *testing.T) {
	// Use temp home override so we don't modify the real config
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)
	setTestConfigPath(t, tmpHome)

	// Create config directory
	cfgDir := filepath.Join(tmpHome, ".config", "termdesk")
	os.MkdirAll(cfgDir, 0755)

	recordWorkspaceAccess("/tmp/project/.termdesk-workspace.toml")

	cfg := config.LoadUserConfig()
	if len(cfg.RecentWorkspaces) == 0 {
		t.Fatal("expected at least 1 recent workspace entry")
	}
	if cfg.RecentWorkspaces[0].Path != "/tmp/project/.termdesk-workspace.toml" {
		t.Errorf("expected path '/tmp/project/.termdesk-workspace.toml', got %q", cfg.RecentWorkspaces[0].Path)
	}
}

func TestRecordWorkspaceAccessDeduplicates(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)
	setTestConfigPath(t, tmpHome)

	cfgDir := filepath.Join(tmpHome, ".config", "termdesk")
	os.MkdirAll(cfgDir, 0755)

	// Record same path twice
	recordWorkspaceAccess("/tmp/project/.termdesk-workspace.toml")
	recordWorkspaceAccess("/tmp/project/.termdesk-workspace.toml")

	cfg := config.LoadUserConfig()
	count := 0
	for _, ws := range cfg.RecentWorkspaces {
		if ws.Path == "/tmp/project/.termdesk-workspace.toml" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 entry for the path, got %d", count)
	}
}

func TestRecordWorkspaceAccessMostRecentFirst(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)
	setTestConfigPath(t, tmpHome)

	cfgDir := filepath.Join(tmpHome, ".config", "termdesk")
	os.MkdirAll(cfgDir, 0755)

	recordWorkspaceAccess("/tmp/first/.termdesk-workspace.toml")
	recordWorkspaceAccess("/tmp/second/.termdesk-workspace.toml")

	cfg := config.LoadUserConfig()
	if len(cfg.RecentWorkspaces) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(cfg.RecentWorkspaces))
	}
	if cfg.RecentWorkspaces[0].Path != "/tmp/second/.termdesk-workspace.toml" {
		t.Errorf("expected most recent entry first, got %q", cfg.RecentWorkspaces[0].Path)
	}
}

// --- captureWorkspaceState additional branches ---

func TestCaptureWorkspaceStateEmptyWindows(t *testing.T) {
	m := setupReadyModel()

	state := m.captureWorkspaceState()
	if len(state.Windows) != 0 {
		t.Errorf("expected 0 windows, got %d", len(state.Windows))
	}
	if state.FocusedID != "" {
		t.Errorf("expected empty focused ID, got %q", state.FocusedID)
	}
}

func TestCaptureWorkspaceStateMinimizedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"minimized"}, "Minimized", "")
	m = completeAnimations(m)

	// Mark window as minimized
	wins := m.wm.Windows()
	if len(wins) == 0 {
		t.Fatal("expected at least 1 window")
	}
	wins[0].Minimized = true

	state := m.captureWorkspaceState()
	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	if !state.Windows[0].Minimized {
		t.Error("expected captured window to be minimized")
	}
}

func TestCaptureWorkspaceStateMaximizedWindow(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"max"}, "Maximized", "")
	m = completeAnimations(m)

	wins := m.wm.Windows()
	if len(wins) == 0 {
		t.Fatal("expected at least 1 window")
	}
	// Set PreMaxRect to simulate a maximized window
	wins[0].PreMaxRect = &geometry.Rect{X: 5, Y: 5, Width: 60, Height: 20}

	state := m.captureWorkspaceState()
	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	ws := state.Windows[0]
	if ws.PreMaxRect == nil {
		t.Fatal("expected PreMaxRect to be captured")
	}
	if ws.PreMaxRect.X != 5 || ws.PreMaxRect.Y != 5 {
		t.Errorf("PreMaxRect position: got (%d,%d), want (5,5)", ws.PreMaxRect.X, ws.PreMaxRect.Y)
	}
	if ws.PreMaxRect.Width != 60 || ws.PreMaxRect.Height != 20 {
		t.Errorf("PreMaxRect size: got (%d,%d), want (60,20)", ws.PreMaxRect.Width, ws.PreMaxRect.Height)
	}
}

func TestCaptureWorkspaceStatePreservesCurrentGeometryInTilingMode(t *testing.T) {
	m := setupReadyModel()
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m.tilingMode = true
	m.tilingLayout = "rows"

	// Scramble rects to simulate stale/incorrect geometry before save.
	for _, w := range m.wm.Windows() {
		w.Rect.Width = 40
		w.Rect.Height = 10
	}

	state := m.captureWorkspaceState()
	if len(state.Windows) != 3 {
		t.Fatalf("expected 3 windows in state, got %d", len(state.Windows))
	}
	for _, ws := range state.Windows {
		if ws.Minimized {
			continue
		}
		if ws.Width != 40 || ws.Height != 10 {
			t.Fatalf("window %s saved rect=%dx%d want=40x10", ws.ID, ws.Width, ws.Height)
		}
	}
}

func TestCaptureWorkspaceStateMultipleWindows(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"one"}, "Win1", "")
	m = completeAnimations(m)
	m.openTerminalWindowWith("/bin/echo", []string{"two"}, "Win2", "")
	m = completeAnimations(m)

	state := m.captureWorkspaceState()
	if len(state.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(state.Windows))
	}

	// Check both have commands
	for i, ws := range state.Windows {
		if ws.Command == "" {
			t.Errorf("window %d: expected non-empty command", i)
		}
	}
}

func TestCaptureWorkspaceStateFocusedID(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"first"}, "First", "")
	m = completeAnimations(m)
	m.openTerminalWindowWith("/bin/echo", []string{"second"}, "Second", "")
	m = completeAnimations(m)

	// The most recently opened window should be focused
	focused := m.wm.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window")
	}

	state := m.captureWorkspaceState()
	if state.FocusedID != focused.ID {
		t.Errorf("expected focused ID %q, got %q", focused.ID, state.FocusedID)
	}
	if state.FocusedID == "" {
		t.Error("expected non-empty focused ID")
	}
}

func TestCaptureWorkspaceStateClipboard(t *testing.T) {
	m := setupReadyModel()
	m.clipboard.Copy("hello clipboard")
	m.clipboard.Copy("second item")

	state := m.captureWorkspaceState()
	if len(state.Clipboard) != 2 {
		t.Fatalf("expected 2 clipboard items, got %d", len(state.Clipboard))
	}
	// Most recent should be first
	if state.Clipboard[0] != "second item" {
		t.Errorf("expected first clipboard item 'second item', got %q", state.Clipboard[0])
	}
}

func TestCaptureWorkspaceStateClipboardTruncation(t *testing.T) {
	m := setupReadyModel()
	// Create a string over 1000 characters
	longStr := strings.Repeat("x", 1500)
	m.clipboard.Copy(longStr)

	state := m.captureWorkspaceState()
	if len(state.Clipboard) != 1 {
		t.Fatalf("expected 1 clipboard item, got %d", len(state.Clipboard))
	}
	// Should be truncated to 1000 + "... [truncated]"
	if len(state.Clipboard[0]) > 1020 {
		t.Errorf("expected truncated clipboard item, got length %d", len(state.Clipboard[0]))
	}
	if !strings.HasSuffix(state.Clipboard[0], "... [truncated]") {
		t.Error("expected truncated suffix on long clipboard item")
	}
}

func TestCaptureWorkspaceStateFallbackCommand(t *testing.T) {
	m := setupReadyModel()

	// Add a demo window (no terminal, no command)
	m.openDemoWindow()
	wins := m.wm.Windows()
	if len(wins) == 0 {
		t.Fatal("expected at least 1 window")
	}
	wins[0].Command = "" // empty command should fall back to SHELL or /bin/sh

	state := m.captureWorkspaceState()
	if len(state.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state.Windows))
	}
	if state.Windows[0].Command == "" {
		t.Error("expected non-empty command after fallback")
	}
}

// --- vimSessionPath tests ---

func TestVimSessionPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	path := vimSessionPath("abc123")
	if path == "" {
		t.Fatal("expected non-empty vim session path")
	}
	if !strings.Contains(path, "sessions") {
		t.Error("expected 'sessions' in vim session path")
	}
	if !strings.HasSuffix(path, "vim-abc123.vim") {
		t.Errorf("expected suffix 'vim-abc123.vim', got %q", filepath.Base(path))
	}
}

func TestVimSessionsDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	dir := vimSessionsDir()
	if dir == "" {
		t.Fatal("expected non-empty vim sessions directory")
	}
	expected := filepath.Join(tmpHome, ".config", "termdesk", "sessions")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestVimSessionPathFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	path := vimSessionPath("deadbeef01234567")
	expected := filepath.Join(tmpHome, ".config", "termdesk", "sessions", "vim-deadbeef01234567.vim")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// --- isAppStateCapable and isVimEditor helper tests ---

func TestIsAppStateCapable(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"termdesk-calc", true},
		{"termdesk-clock", true},
		{"/usr/bin/termdesk-calc", true},
		{"bash", false},
		{"nvim", false},
		{"/bin/sh", false},
	}
	for _, tt := range tests {
		got := isAppStateCapable(tt.cmd)
		if got != tt.want {
			t.Errorf("isAppStateCapable(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestIsVimEditor(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"nvim", true},
		{"vim", true},
		{"/usr/bin/nvim", true},
		{"/usr/bin/vim", true},
		{"bash", false},
		{"nano", false},
		{"/bin/sh", false},
		{"termdesk-calc", false},
	}
	for _, tt := range tests {
		got := isVimEditor(tt.cmd)
		if got != tt.want {
			t.Errorf("isVimEditor(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

// --- restoreWorkspace tests ---

func TestRestoreWorkspaceBasic(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-test-1",
				Title:   "Test Window",
				Command: "/bin/echo",
				Args:    []string{"hello"},
				X:       5,
				Y:       3,
				Width:   60,
				Height:  20,
				ZIndex:  1,
			},
		},
		FocusedID: "win-test-1",
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window after restore, got %d", m.wm.Count())
	}

	win := m.wm.Windows()[0]
	if win.ID != "win-test-1" {
		t.Errorf("expected window ID 'win-test-1', got %q", win.ID)
	}
	if win.Title != "Test Window" {
		t.Errorf("expected title 'Test Window', got %q", win.Title)
	}

	// Clean up terminal
	if term, ok := m.terminals["win-test-1"]; ok {
		term.Close()
	}
}

func TestRestoreWorkspaceMinimized(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:        "win-min-1",
				Title:     "Minimized",
				Command:   "/bin/echo",
				X:         2,
				Y:         2,
				Width:     50,
				Height:    15,
				Minimized: true,
			},
		},
	}

	m.restoreWorkspace(state, "")

	wins := m.wm.Windows()
	if len(wins) != 1 {
		t.Fatalf("expected 1 window, got %d", len(wins))
	}
	if !wins[0].Minimized {
		t.Error("expected restored window to be minimized")
	}

	// Clean up
	if term, ok := m.terminals["win-min-1"]; ok {
		term.Close()
	}
}

func TestRestoreWorkspaceMaximized(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:         "win-max-1",
				Title:      "Maximized",
				Command:    "/bin/echo",
				X:          0,
				Y:          1,
				Width:      120,
				Height:     38,
				PreMaxRect: &geometry.Rect{X: 10, Y: 5, Width: 60, Height: 20},
			},
		},
	}

	m.restoreWorkspace(state, "")

	wins := m.wm.Windows()
	if len(wins) != 1 {
		t.Fatalf("expected 1 window, got %d", len(wins))
	}
	if wins[0].PreMaxRect == nil {
		t.Fatal("expected PreMaxRect to be restored")
	}
	if wins[0].PreMaxRect.X != 10 || wins[0].PreMaxRect.Y != 5 {
		t.Errorf("PreMaxRect position: got (%d,%d), want (10,5)", wins[0].PreMaxRect.X, wins[0].PreMaxRect.Y)
	}

	// Clean up
	if term, ok := m.terminals["win-max-1"]; ok {
		term.Close()
	}
}

func TestRestoreWorkspaceClipboard(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		Clipboard: []string{"item1", "item2", "item3"},
	}

	m.restoreWorkspace(state, "")

	history := m.clipboard.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 clipboard items, got %d", len(history))
	}
}

func TestRestoreWorkspaceNextNumber(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "w1", Title: "Terminal 1", Command: "/bin/echo", X: 2, Y: 2, Width: 50, Height: 15},
			{ID: "w2", Title: "Terminal 2", Command: "/bin/echo", X: 5, Y: 5, Width: 50, Height: 15},
			{ID: "w3", Title: "Terminal 3", Command: "/bin/echo", X: 8, Y: 8, Width: 50, Height: 15},
		},
	}

	m.restoreWorkspace(state, "")

	// Next terminal should be number 4
	num := m.nextTerminalNumber()
	if num != 4 {
		t.Errorf("expected nextTerminalNumber() = 4, got %d", num)
	}

	// Clean up terminals
	for id, term := range m.terminals {
		term.Close()
		delete(m.terminals, id)
	}
}

func TestRestoreWorkspaceFocusedWindow(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "wA", Title: "A", Command: "/bin/echo", X: 2, Y: 2, Width: 50, Height: 15},
			{ID: "wB", Title: "B", Command: "/bin/echo", X: 5, Y: 5, Width: 50, Height: 15},
		},
		FocusedID: "wA",
	}

	m.restoreWorkspace(state, "")

	focused := m.wm.FocusedWindow()
	if focused == nil {
		t.Fatal("expected a focused window after restore")
	}
	if focused.ID != "wA" {
		t.Errorf("expected focused window 'wA', got %q", focused.ID)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

// --- absLineToContentRow tests ---

func TestAbsLineToContentRowEmulatorRegion(t *testing.T) {
	// No scrollback, line 0 = emulator row 0
	row := absLineToContentRow(0, 0, 0, 24)
	if row != 0 {
		t.Errorf("expected row 0 for absLine 0 with no scrollback, got %d", row)
	}

	// absLine 5 => emulator row 5
	row = absLineToContentRow(5, 0, 0, 24)
	if row != 5 {
		t.Errorf("expected row 5, got %d", row)
	}
}

func TestAbsLineToContentRowScrollbackVisible(t *testing.T) {
	// 100 lines of scrollback, scrollOffset=5, contentH=24
	// Visible scrollback range: absLine 95..99 => rows 0..4
	row := absLineToContentRow(95, 5, 100, 24)
	if row != 0 {
		t.Errorf("expected row 0 for absLine 95 (oldest visible scrollback), got %d", row)
	}

	row = absLineToContentRow(99, 5, 100, 24)
	if row != 4 {
		t.Errorf("expected row 4 for absLine 99 (newest visible scrollback), got %d", row)
	}
}

func TestAbsLineToContentRowNotVisible(t *testing.T) {
	// 100 lines of scrollback, scrollOffset=5 => visible scrollback: absLine 95..99
	// absLine 50 is not visible
	row := absLineToContentRow(50, 5, 100, 24)
	if row != -1 {
		t.Errorf("expected -1 for non-visible scrollback line, got %d", row)
	}
}

func TestAbsLineToContentRowBeyondContent(t *testing.T) {
	// absLine beyond total content
	row := absLineToContentRow(30, 0, 0, 24)
	if row != -1 {
		t.Errorf("expected -1 for absLine beyond content, got %d", row)
	}
}

func TestAbsLineToContentRowScrollOffsetLargerThanContentH(t *testing.T) {
	// scrollOffset > contentH is clamped
	row := absLineToContentRow(95, 30, 100, 24)
	// scrollLines = min(30, 24) = 24
	// absLine 95 => row = 95 - (100 - 30) = 95 - 70 = 25 => >= 24, not visible
	if row != -1 {
		t.Errorf("expected -1 when scrollOffset > contentH pushes line out, got %d", row)
	}
}

func TestAbsLineToContentRowEmulatorWithScrollback(t *testing.T) {
	// 50 scrollback lines, scrollOffset=3, contentH=24
	// Emulator row 0 = absLine 50, at contentRow = 3 (after 3 scrollback lines)
	row := absLineToContentRow(50, 3, 50, 24)
	if row != 3 {
		t.Errorf("expected row 3 for emulator row 0, got %d", row)
	}
}

func TestAbsLineToContentRowZeroScrollOffset(t *testing.T) {
	// No scroll offset: scrollback lines are not visible
	// 50 scrollback, scrollOffset=0, contentH=24
	// absLine 40 is in scrollback but not visible
	row := absLineToContentRow(40, 0, 50, 24)
	if row != -1 {
		t.Errorf("expected -1 for scrollback line with no scroll offset, got %d", row)
	}

	// absLine 50 = emulator row 0
	row = absLineToContentRow(50, 0, 50, 24)
	if row != 0 {
		t.Errorf("expected row 0 for emulator row 0 with no scroll offset, got %d", row)
	}
}

// --- mouseToAbsLine tests ---

func TestMouseToAbsLineNoScrollback(t *testing.T) {
	// No scrollback, no scroll offset: row N = absLine N
	abs := mouseToAbsLine(5, 0, 0, 24)
	if abs != 5 {
		t.Errorf("expected absLine 5, got %d", abs)
	}
}

func TestMouseToAbsLineWithScrollOffset(t *testing.T) {
	// 100 scrollback lines, scrollOffset=10, contentH=24
	// Row 0 (scrollback region) => absLine = 100 - 10 + 0 = 90
	abs := mouseToAbsLine(0, 10, 100, 24)
	if abs != 90 {
		t.Errorf("expected absLine 90, got %d", abs)
	}

	// Row 10 (first emulator row) => absLine = 100 + (10-10) = 100
	abs = mouseToAbsLine(10, 10, 100, 24)
	if abs != 100 {
		t.Errorf("expected absLine 100, got %d", abs)
	}
}

func TestMouseToAbsLineRoundTrip(t *testing.T) {
	// Verify mouseToAbsLine and absLineToContentRow are inverses
	scrollbackLen := 100
	scrollOffset := 10
	contentH := 24

	for contentRow := 0; contentRow < contentH; contentRow++ {
		absLine := mouseToAbsLine(contentRow, scrollOffset, scrollbackLen, contentH)
		backToRow := absLineToContentRow(absLine, scrollOffset, scrollbackLen, contentH)
		if backToRow != contentRow {
			t.Errorf("round-trip failed: contentRow %d -> absLine %d -> contentRow %d",
				contentRow, absLine, backToRow)
		}
	}
}

// --- extractSelText tests ---

func TestExtractSelTextEmulatorOnly(t *testing.T) {
	// Create a real terminal with echo output
	term, err := terminal.New("/bin/echo", []string{"HELLO WORLD"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	// Wait for output to appear
	time.Sleep(200 * time.Millisecond)
	_ = term.ReadPtyLoop()
	time.Sleep(50 * time.Millisecond)

	sbLen := term.ScrollbackLen()

	// Select first line from the emulator (absLine = sbLen, which is emu row 0)
	start := geometry.Point{X: 0, Y: sbLen}
	end := geometry.Point{X: 10, Y: sbLen}
	text := extractSelText(term, start, end)

	if text == "" {
		t.Error("expected non-empty text from selection")
	}
	// The output should contain "HELLO WORLD" or at least part of it
	if !strings.Contains(text, "HELLO") {
		t.Errorf("expected text to contain 'HELLO', got %q", text)
	}
}

func TestExtractSelTextReversedSelection(t *testing.T) {
	// extractSelText should normalize start/end (swap if start > end)
	term, err := terminal.New("/bin/echo", []string{"REVERSED"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	time.Sleep(200 * time.Millisecond)
	_ = term.ReadPtyLoop()
	time.Sleep(50 * time.Millisecond)

	sbLen := term.ScrollbackLen()

	// Reversed selection (end before start)
	start := geometry.Point{X: 7, Y: sbLen}
	end := geometry.Point{X: 0, Y: sbLen}
	text := extractSelText(term, start, end)

	if !strings.Contains(text, "REVERSED") {
		t.Errorf("expected text to contain 'REVERSED' even with reversed selection, got %q", text)
	}
}

func TestExtractSelTextNegativeLines(t *testing.T) {
	// extractSelText should handle negative line numbers gracefully
	term, err := terminal.New("/bin/echo", []string{"test"}, 40, 10, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	time.Sleep(200 * time.Millisecond)
	_ = term.ReadPtyLoop()
	time.Sleep(50 * time.Millisecond)

	start := geometry.Point{X: 0, Y: -5}
	end := geometry.Point{X: 10, Y: 0}
	// Should not panic; may return empty or partial text
	_ = extractSelText(term, start, end)
}

func TestExtractSelTextMultipleLines(t *testing.T) {
	// Use printf to output multiple lines.
	// Pass an explicit workDir so /bin/sh can always getcwd (avoids
	// "shell-init: error retrieving current directory" on macOS).
	term, err := terminal.New("/bin/sh", []string{"-c", "printf 'LINE1\\nLINE2\\nLINE3\\n'"}, 40, 10, 0, 0, t.TempDir())
	if err != nil {
		t.Fatalf("terminal.New: %v", err)
	}
	defer term.Close()

	time.Sleep(300 * time.Millisecond)
	_ = term.ReadPtyLoop()
	time.Sleep(50 * time.Millisecond)

	sbLen := term.ScrollbackLen()

	// Select across all three lines
	start := geometry.Point{X: 0, Y: sbLen}
	end := geometry.Point{X: 4, Y: sbLen + 2}
	text := extractSelText(term, start, end)

	if !strings.Contains(text, "LINE1") {
		t.Errorf("expected text to contain 'LINE1', got %q", text)
	}
	// Check we got multiple lines
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines in selection, got %d: %q", len(lines), text)
	}
}

// --- saveWorkspaceNow with windows ---

func TestSaveWorkspaceNowWithWindows(t *testing.T) {
	m := setupReadyModel()
	m.openTerminalWindowWith("/bin/echo", []string{"save-test"}, "SaveTest", "")
	m = completeAnimations(m)

	tmpDir := t.TempDir()
	m.projectConfig = &config.ProjectConfig{
		ProjectDir: tmpDir,
	}

	m.saveWorkspaceNow()
	time.Sleep(100 * time.Millisecond) // wait for async goroutine

	wsPath := filepath.Join(tmpDir, ".termdesk-workspace.toml")
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatalf("failed to read workspace file: %v", err)
	}
	content := string(data)

	// Verify it contains a window section
	if !strings.Contains(content, "[[window]]") {
		t.Error("expected [[window]] section in saved workspace")
	}
	if !strings.Contains(content, "/bin/echo") {
		t.Error("expected '/bin/echo' command in saved workspace")
	}
}

// --- Workspace round-trip (save + load) ---

func TestWorkspaceRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	state := workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		FocusedID: "win-abc",
		Windows: []workspace.WindowState{
			{
				ID:      "win-abc",
				Title:   "My Terminal",
				Command: "/bin/bash",
				X:       10,
				Y:       5,
				Width:   80,
				Height:  24,
				ZIndex:  1,
			},
		},
		Clipboard: []string{"copied text"},
	}

	err := workspace.SaveWorkspace(state, tmpDir)
	if err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}

	loaded, err := workspace.LoadWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil loaded workspace")
	}
	if len(loaded.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(loaded.Windows))
	}
	if loaded.Windows[0].ID != "win-abc" {
		t.Errorf("expected window ID 'win-abc', got %q", loaded.Windows[0].ID)
	}
	if loaded.FocusedID != "win-abc" {
		t.Errorf("expected focused ID 'win-abc', got %q", loaded.FocusedID)
	}
}

// --- window.Manager dependency check (used by workspace functions) ---

func TestWorkspaceManagerSetBounds(t *testing.T) {
	// Verify window manager setup used in workspace switch
	wm := window.NewManager(120, 40)
	wm.SetBounds(120, 40)
	wm.SetReserved(1, 1)

	wa := wm.WorkArea()
	if wa.Width != 120 {
		t.Errorf("expected work area width 120, got %d", wa.Width)
	}
	if wa.Height != 38 { // 40 - 1 (menu) - 1 (dock)
		t.Errorf("expected work area height 38, got %d", wa.Height)
	}
}

// --- restoreWorkspace extended branch tests ---

func TestRestoreWorkspaceWithProjectDir(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	defer os.Chdir(origCwd)

	// Resolve symlinks so the comparison works on macOS where
	// /var → /private/var.
	resolvedTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		resolvedTmpDir = tmpDir
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-proj-1",
				Title:   "Project Window",
				Command: "/bin/echo",
				WorkDir: resolvedTmpDir,
				X:       5,
				Y:       3,
				Width:   60,
				Height:  20,
			},
		},
	}

	m.restoreWorkspace(state, resolvedTmpDir)

	cwd, _ := os.Getwd()
	// Resolve cwd too — os.Getwd may return the canonical path.
	resolvedCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		resolvedCwd = cwd
	}
	if resolvedCwd != resolvedTmpDir {
		t.Errorf("expected cwd to be %q after restore with projectDir, got %q", resolvedTmpDir, resolvedCwd)
	}

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceEmptyProjectDir(t *testing.T) {
	m := setupReadyModel()
	origCwd, _ := os.Getwd()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
	}

	m.restoreWorkspace(state, "")

	newCwd, _ := os.Getwd()
	if newCwd != origCwd {
		t.Error("empty projectDir should not change working directory")
	}
}

func TestRestoreWorkspaceWithArgs(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-args-1",
				Title:   "Echo Args",
				Command: "/bin/echo",
				Args:    []string{"arg1", "arg2"},
				X:       5,
				Y:       3,
				Width:   60,
				Height:  20,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	win := m.wm.Windows()[0]
	if win.Command != "/bin/echo" {
		t.Errorf("expected command '/bin/echo', got %q", win.Command)
	}
	if len(win.Args) != 2 || win.Args[0] != "arg1" || win.Args[1] != "arg2" {
		t.Errorf("expected args [arg1 arg2], got %v", win.Args)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceWithAppState(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:           "win-state-1",
				Title:        "State App",
				Command:      "/bin/echo",
				AppStateData: "some-state-data",
				X:            5,
				Y:            3,
				Width:        60,
				Height:       20,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Terminal should have been created (check it exists)
	if _, ok := m.terminals["win-state-1"]; !ok {
		t.Error("expected terminal to be created for window with app state")
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceShellWindow(t *testing.T) {
	m := setupReadyModel()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-shell-1",
				Title:   "Shell",
				Command: shell,
				X:       5,
				Y:       3,
				Width:   60,
				Height:  20,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	if _, ok := m.terminals["win-shell-1"]; !ok {
		t.Error("expected terminal for shell window")
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceEmptyCommandUsesShell(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:      "win-empty-cmd",
				Title:   "Empty",
				Command: "", // empty command => shell
				X:       5,
				Y:       3,
				Width:   60,
				Height:  20,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}
	if _, ok := m.terminals["win-empty-cmd"]; !ok {
		t.Error("expected terminal for empty command window (should use shell)")
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceMultipleWindowsFocuses(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "w1", Title: "First", Command: "/bin/echo", X: 2, Y: 2, Width: 50, Height: 15},
			{ID: "w2", Title: "Second", Command: "/bin/echo", X: 10, Y: 10, Width: 50, Height: 15},
		},
		FocusedID: "w2",
	}

	m.restoreWorkspace(state, "")

	focused := m.wm.FocusedWindow()
	if focused == nil {
		t.Fatal("expected focused window")
	}
	if focused.ID != "w2" {
		t.Errorf("expected focused window 'w2', got %q", focused.ID)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceEmptyClipboard(t *testing.T) {
	m := setupReadyModel()

	state := &workspace.WorkspaceState{
		Version:   1,
		SavedAt:   time.Now(),
		Clipboard: nil, // no clipboard
	}

	// Should not panic
	m.restoreWorkspace(state, "")

	history := m.clipboard.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty clipboard, got %d items", len(history))
	}
}

func TestNextTerminalNumberReusesGaps(t *testing.T) {
	m := setupReadyModel()

	// Simulate windows "Terminal 1" and "Terminal 3" (gap at 2)
	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{ID: "w1", Title: "Terminal 1", Command: "/bin/echo", X: 2, Y: 2, Width: 50, Height: 15},
			{ID: "w3", Title: "Terminal 3", Command: "/bin/echo", X: 8, Y: 8, Width: 50, Height: 15},
		},
	}

	m.restoreWorkspace(state, "")

	// Should reuse gap: next number = 2
	num := m.nextTerminalNumber()
	if num != 2 {
		t.Errorf("expected nextTerminalNumber() = 2, got %d", num)
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

func TestRestoreWorkspaceWithBufferFile(t *testing.T) {
	m := setupReadyModel()

	// Create a temp buffer file
	tmpHome := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpHome)

	bufDir := filepath.Join(tmpHome, ".config", "termdesk", "buffers")
	os.MkdirAll(bufDir, 0755)

	// Save a buffer via the workspace package
	bufContent := "line1\nline2\nline3\n"
	bufID := workspace.SaveBuffer("buf-test-win", bufContent)

	state := &workspace.WorkspaceState{
		Version: 1,
		SavedAt: time.Now(),
		Windows: []workspace.WindowState{
			{
				ID:         "buf-test-win",
				Title:      "Buffer Test",
				Command:    "/bin/echo",
				X:          5,
				Y:          3,
				Width:      60,
				Height:     20,
				BufferFile: bufID,
				BufferRows: 20,
				BufferCols: 60,
			},
		},
	}

	m.restoreWorkspace(state, "")

	if m.wm.Count() != 1 {
		t.Fatalf("expected 1 window, got %d", m.wm.Count())
	}

	// Check that buffer was loaded into windowBuffers map
	if bufID != "" {
		if _, ok := m.windowBuffers["buf-test-win"]; !ok {
			t.Error("expected buffer content in windowBuffers after restore")
		}
	}

	// Clean up
	for _, term := range m.terminals {
		term.Close()
	}
}

// --- createNewWorkspace additional branch tests ---

func TestCreateNewWorkspaceInvalidPath(t *testing.T) {
	m := setupReadyModel()

	// Use a path inside /proc which cannot be created
	m.createNewWorkspace("Bad", "/proc/0/nonexistent/deep/path")

	// Should push an error notification (no panic)
}

func TestCreateNewWorkspaceRelativePath(t *testing.T) {
	m := setupReadyModel()

	tmpDir := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origCwd)

	// Use relative path - should be resolved to absolute
	m.createNewWorkspace("Relative", "relative-ws-dir")

	absPath := filepath.Join(tmpDir, "relative-ws-dir")
	wsPath := filepath.Join(absPath, ".termdesk-workspace.toml")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("expected workspace file at %s after relative path", wsPath)
	}
}
