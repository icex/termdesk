package app

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/icex/termdesk/internal/apps/registry"
	"github.com/icex/termdesk/internal/contextmenu"
	"github.com/icex/termdesk/internal/dock"
	"github.com/icex/termdesk/internal/launcher"
	"github.com/icex/termdesk/internal/menubar"
	"github.com/icex/termdesk/internal/notification"
	"github.com/icex/termdesk/internal/terminal"
	"github.com/icex/termdesk/internal/window"
	"github.com/icex/termdesk/pkg/geometry"
)

func normalizeViewText(s string) string {
	// Strip ANSI and normalize newlines
	runes := stripANSI(s)
	text := string(runes)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	// Trim trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderViewString(m Model) string {
	v := m.View()
	return v.Content
}

func goldenPath(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", "golden", name+".golden")
	}
	base := filepath.Dir(file)
	return filepath.Join(base, "testdata", "golden", name+".golden")
}

func withTestConfig(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("TERMDESK_HOME", tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.toml")
	t.Setenv("TERMDESK_CONFIG_PATH", cfgPath)

	cfg := strings.Join([]string{
		`theme = "Retro"`,
		`animation_speed = "normal"`,
		`animation_style = "smooth"`,
		``,
		`[keybindings]`,
		`prefix = "ctrl+a"`,
	}, "\n")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

var updateGoldens = flag.Bool("update", false, "regenerate golden render fixtures")

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *updateGoldens {
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != got {
		t.Errorf("render differs from %s; inspect changes and regenerate with -update\n got:\n%s", path, got)
	}
}

func newGoldenModel(t *testing.T) Model {
	t.Helper()
	_ = withTestConfig(t)

	m := New()
	// Golden output must not depend on installed programs or PATH order.
	m.registry = []registry.RegistryEntry{{Name: "Terminal", Icon: "\uf120", Command: "$SHELL"}, {Name: "Files", Icon: "\uf07b", Command: "yazi"}, {Name: "Browser", Icon: "\uf0ac", Command: "carbonyl"}, {Name: "nvim", Icon: "\ue62b", Command: "nvim"}, {Name: "System Monitor", Icon: "\uf200", Command: "htop"}}
	m.dock = dock.New(registry.DockEntries(m.registry), 80)
	m.launcher = launcher.New(m.registry)
	m.launcher.SetExecIndex([]string{"termdesk-calc", "termdesk-snake"})
	m.menuBar = menubar.New(80, m.keybindings, m.registry, nil)
	m.tour.Skip()
	m.ready = true
	m.width = 80
	m.height = 24
	m.wm.SetBounds(80, 24)
	m.wm.SetReserved(1, 1)
	m.menuBar.SetWidth(80)
	m.showDeskClock = false
	m.menuBar.WidgetBar = nil
	return m
}

func addTerminalWindow(t *testing.T, m *Model, content string) *terminal.Terminal {
	t.Helper()
	win := window.NewWindow("term1", "Terminal", geometry.Rect{X: 2, Y: 2, Width: 30, Height: 8}, nil)
	m.wm.AddWindow(win)
	m.wm.FocusWindow(win.ID)

	cr := win.ContentRect()
	term, err := terminal.NewShell(cr.Width, cr.Height, 0, 0, "")
	if err != nil {
		t.Fatalf("terminal.NewShell: %v", err)
	}
	term.RestoreBuffer(content)
	m.terminals[win.ID] = term
	return term
}

func TestGoldenBasicView(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "basic_view", normalized)
}

func TestGoldenMenuOpen(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.menuBar.OpenMenu(0)

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "menu_open", normalized)
}

func TestGoldenLauncherOpen(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.launcher.Show()
	m.launcher.SetQuery("term")

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "launcher_open", normalized)
}

func TestGoldenClipboardHistory(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.clipboard.Copy("first line")
	m.clipboard.Copy("second line")
	m.clipboard.ShowHistory()

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "clipboard_history", normalized)
}

func TestGoldenNotificationCenter(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.notifications.Push("Build", "success", notification.Info)
	m.notifications.Push("Warnings", "2 issues", notification.Warning)
	m.notifications.ShowCenter()

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "notification_center", normalized)
}

func TestGoldenToasts(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.notifications.Push("Saved", "workspace stored", notification.Info)

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "toasts", normalized)
}

func TestGoldenCopyModeSelection(t *testing.T) {
	m := newGoldenModel(t)
	term := addTerminalWindow(t, &m, "alpha\nbeta\ngamma\ndelta")
	defer term.Close()

	m.inputMode = ModeCopy
	m.scrollOffset = 1
	m.selActive = true
	m.selStart = geometry.Point{X: 0, Y: 1}
	m.selEnd = geometry.Point{X: 3, Y: 2}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "copy_mode_selection", normalized)
}

func TestGoldenExposeMode(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m.openDemoWindow()
	m.openDemoWindow()
	m = completeAnimations(m)
	m.enterExpose()

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "expose_mode", normalized)
}

func TestGoldenSettingsPanel(t *testing.T) {
	m := newGoldenModel(t)
	m.openDemoWindow()
	m = completeAnimations(m)
	m.settings.Show()

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "settings_panel", normalized)
}

func TestGoldenWorkspacePicker(t *testing.T) {
	m := newGoldenModel(t)
	m.workspacePickerVisible = true
	m.workspacePickerSelected = 1
	m.workspaceList = []string{
		"/home/user/.config/termdesk/workspace.toml",
		"/home/user/Projects/myapp/.termdesk-workspace.toml",
	}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "workspace_picker", normalized)
}

func TestGoldenConfirmDialog(t *testing.T) {
	m := newGoldenModel(t)
	m.confirmClose = &ConfirmDialog{Title: "Quit termdesk?", IsQuit: true}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "confirm_dialog", normalized)
}

func TestGoldenRenameDialog(t *testing.T) {
	m := newGoldenModel(t)
	m.renameDialog = &RenameDialog{WindowID: "w1", Text: []rune("My Window"), Cursor: 3}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "rename_dialog", normalized)
}

func TestGoldenNewWorkspaceDialog(t *testing.T) {
	m := newGoldenModel(t)
	m.newWorkspaceDialog = &NewWorkspaceDialog{
		Name:       []rune("Demo"),
		DirPath:    "/tmp/demo",
		DirEntries: []string{"docs", "src", "tests"},
		TextCursor: 2,
		Cursor:     0,
	}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "new_workspace_dialog", normalized)
}

func TestGoldenContextMenu(t *testing.T) {
	m := newGoldenModel(t)
	m.contextMenu = &contextmenu.Menu{
		X: 5, Y: 5,
		Items: []contextmenu.Item{
			{Label: "Open", Action: "open"},
			{Label: "Rename", Action: "rename"},
			{Label: "Delete", Action: "delete"},
		},
		Visible: true,
	}

	view := renderViewString(m)
	normalized := normalizeViewText(view)
	assertGolden(t, "context_menu", normalized)
}
