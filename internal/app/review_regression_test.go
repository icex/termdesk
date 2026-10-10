package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/config"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/internal/workspace"
	"github.com/icex/termdesk/pkg/geometry"
)

func isolatedReviewModel(t *testing.T) Model {
	t.Helper()
	home := t.TempDir()
	t.Setenv("TERMDESK_HOME", home)
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(home, "config.toml"))
	m := setupReadyModel()
	m.projectConfig = nil
	t.Cleanup(func() { m.closeAllTerminals() })
	return m
}
func TestCapturePreservesOtherWorkspaceBuffers(t *testing.T) {
	m := isolatedReviewModel(t)
	id := workspace.SaveBuffer("other-workspace", "saved history")
	m.captureWorkspaceState()
	if workspace.LoadBuffer(id) != "saved history" {
		t.Fatal("other workspace's buffer removed")
	}
}
func TestGlobalWorkspaceSwitchLoadsSavedWindows(t *testing.T) {
	m := isolatedReviewModel(t)
	state := workspace.WorkspaceState{Windows: []workspace.WindowState{{ID: "saved", Title: "saved global", Command: "/bin/cat", Width: 40, Height: 10, Resizable: true}}}
	if err := workspace.SaveWorkspace(state, ""); err != nil {
		t.Fatal(err)
	}
	m.switchToWorkspace(workspace.GetWorkspacePath(""))
	if m.wm.Count() != 1 {
		t.Fatal("global workspace not loaded")
	}
	if m.workspaceProjectDir() != "" {
		t.Fatal("global path treated as project directory")
	}
}
func TestSaveDestinationUsesSelectedWorkspaceWithoutConfig(t *testing.T) {
	m := isolatedReviewModel(t)
	dir := t.TempDir()
	m.activeWorkspacePath = workspace.GetWorkspacePath(dir)
	if m.workspaceProjectDir() != dir {
		t.Fatal("selected project saves to global workspace")
	}
}
func TestFailedNestedSplitPreservesExistingPanes(t *testing.T) {
	m := isolatedReviewModel(t)
	w := window.NewWindow("a", "shell", geometry.Rect{X: 1, Y: 1, Width: 80, Height: 24}, nil)
	w.SplitRoot = &window.SplitNode{Dir: window.SplitHorizontal, Ratio: .333333, Children: [2]*window.SplitNode{{TermID: "a"}, {TermID: "b"}}}
	w.FocusedPane = "a"
	m.wm.AddWindow(w)
	before := window.EncodeSplitTree(w.SplitRoot)
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "missing"))
	m.splitPane(window.SplitHorizontal)
	if w.SplitRoot == nil || window.EncodeSplitTree(w.SplitRoot) != before || w.SplitRoot.Ratio != .333333 || w.FocusedPane != "a" {
		t.Fatal("failed split discarded existing tree")
	}
}
func TestFailedLaunchLeavesNoGhostWindow(t *testing.T) {
	m := isolatedReviewModel(t)
	m.createTerminalWindow(TerminalWindowOpts{Command: "/nonexistent/termdesk-review"})
	if m.wm.Count() != 0 || len(m.terminals) != 0 || m.hasActiveAnimations() {
		t.Fatal("failed launch retained window or animation")
	}
	if len(m.notifications.HistoryItems()) == 0 {
		t.Fatal("launch error not visible")
	}
}
func TestNarrowBellTitleNeverPanics(t *testing.T) {
	th := config.GetTheme("retro")
	for width := 3; width < 50; width++ {
		for _, icon := range []string{"", "\uf120"} {
			w := window.NewWindow("narrow", "title", geometry.Rect{Width: width, Height: 10}, nil)
			w.HasBell = true
			w.Icon = icon
			w.IconColor = "#ffffff"
			renderWindowChrome(NewBuffer(80, 24, ""), w, th, window.HitNone)
		}
	}
}
func TestWorkspaceCaptureDoesNotTypeIntoEditors(t *testing.T) {
	m := isolatedReviewModel(t)
	marker := filepath.Join(t.TempDir(), "typed")
	tr, err := terminal.New("/bin/sh", []string{"-c", "stty raw -echo; cat > \"$1\"", "sh", marker}, 80, 24, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	w := window.NewWindow("editor", "Editor", geometry.Rect{Width: 80, Height: 24}, nil)
	w.Command = "nvim"
	m.wm.AddWindow(w)
	m.terminals[w.ID] = tr
	m.signalAppsForCapture()
	time.Sleep(30 * time.Millisecond)
	data, _ := os.ReadFile(marker)
	if len(data) != 0 {
		t.Fatalf("capture injected editor commands: %q", data)
	}
	m.workspaceAutoSave = false
	m.lastWorkspaceSave = time.Now().Add(-2 * time.Minute)
	ret, _ := m.Update(WorkspaceAutoSaveMsg{})
	m = ret.(Model)
	if m.lastWorkspaceSave.After(time.Now().Add(-time.Minute)) {
		t.Fatal("disabled autosave ran")
	}
}
func TestUntrustedRestoreCannotOverwriteOriginal(t *testing.T) {
	m := isolatedReviewModel(t)
	dir := t.TempDir()
	path := workspace.GetWorkspacePath(dir)
	content := "version = 1\nsaved_at = \"" + time.Now().Format(time.RFC3339) + "\"\n[[window]]\nid = \"unsafe\"\ncommand = \"/bin/sh\"\nwidth = 80\nheight = 24\n"
	os.WriteFile(path, []byte(content), 0600)
	state, err := workspace.LoadWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.restoreWorkspace(state, dir)
	if m.wm.Count() != 0 || !m.workspaceSaveBlocked {
		t.Fatal("unapproved restore executed")
	}
	if workspace.SaveWorkspace(m.captureWorkspaceState(), dir) == nil {
		t.Fatal("blocked restore allowed autosave")
	}
	after, _ := os.ReadFile(path)
	if string(after) != content {
		t.Fatal("untrusted file overwritten")
	}
}
func TestWallpaperRestartWaitsAndCanBeCancelled(t *testing.T) {
	m := isolatedReviewModel(t)
	m.wallpaperMode = "program"
	m.wallpaperProgram = "/bin/true"
	ret, cmd := m.Update(PtyClosedMsg{WindowID: wallpaperTermID})
	m = ret.(Model)
	if cmd == nil || m.wallpaperTerminal != nil {
		t.Fatal("wallpaper immediately relaunched")
	}
	m.wallpaperMode = "theme"
	ret, _ = m.Update(WallpaperRestartMsg{})
	m = ret.(Model)
	if m.wallpaperTerminal != nil {
		t.Fatal("cancelled restart launched")
	}
}
func BenchmarkCaptureWorkspacePopulated(b *testing.B) {
	home := b.TempDir()
	b.Setenv("TERMDESK_HOME", home)
	b.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(home, "config.toml"))
	m := setupReadyModel()
	m.projectConfig = nil
	defer m.closeAllTerminals()
	for i := 0; i < 9; i++ {
		m.createTerminalWindow(TerminalWindowOpts{Command: "/bin/cat", FixedSize: true, Width: 200, Height: 60})
	}
	data := strings.Repeat("scrollback text to measure workspace capture\n", 1000)
	for _, tr := range m.terminals {
		tr.RestoreBuffer(data)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.captureWorkspaceState()
	}
}

func TestUntrustedProjectDockEntriesAreNotAdded(t *testing.T) {
	base := []registry.RegistryEntry{{Name: "Terminal", Command: "$SHELL"}}
	cfg := &config.ProjectConfig{
		ConfigPath: "/repo/.termdesk.toml",
		DockItems:  []config.DockEntry{{Name: "Terminal", Command: "/bin/sh", Args: []string{"-c", "touch pwned"}, Position: 1}},
	}
	if got := withProjectDock(base, cfg); len(got) != 1 || got[0].Command != "$SHELL" {
		t.Fatalf("untrusted dock entry added: %+v", got)
	}
	cfg.Trusted = true
	if got := withProjectDock(base, cfg); len(got) != 2 || got[0].Command != "/bin/sh" {
		t.Fatalf("trusted dock entry missing: %+v", got)
	}
}
