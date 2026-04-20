package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setTestConfigPath(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	t.Setenv(envConfigPath, cfgPath)
	return cfgPath
}

func TestDefaultUserConfig(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Theme != "modern" {
		t.Errorf("default theme = %q, want modern", cfg.Theme)
	}
	if cfg.IconsOnly {
		t.Error("default IconsOnly should be false")
	}
	if cfg.TilingLayout != "columns" {
		t.Errorf("default TilingLayout = %q, want columns", cfg.TilingLayout)
	}
}

func TestSaveAndLoadUserConfig(t *testing.T) {
	// Use a temp directory instead of real config path
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.toml")

	cfg := UserConfig{Theme: "tokyonight", IconsOnly: true}

	// Manually write config to temp file (bypassing configPath)
	var sb strings.Builder
	sb.WriteString("# Termdesk configuration\n\n")
	sb.WriteString("theme = \"" + cfg.Theme + "\"\n")
	sb.WriteString("icons_only = true\n")
	if err := os.WriteFile(tmpFile, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read and parse
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	loaded := DefaultUserConfig()
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		switch key {
		case "theme":
			loaded.Theme = val
		case "icons_only":
			loaded.IconsOnly = val == "true"
		}
	}

	if loaded.Theme != "tokyonight" {
		t.Errorf("loaded theme = %q, want tokyonight", loaded.Theme)
	}
	if !loaded.IconsOnly {
		t.Error("loaded IconsOnly should be true")
	}
}

func TestParseConfigComments(t *testing.T) {
	input := "# This is a comment\ntheme = \"retro\"\n# Another comment\nicons_only = false\n"
	cfg := DefaultUserConfig()
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		switch key {
		case "theme":
			cfg.Theme = val
		case "icons_only":
			cfg.IconsOnly = val == "true"
		}
	}
	if cfg.Theme != "retro" {
		t.Errorf("theme = %q, want retro", cfg.Theme)
	}
	if cfg.IconsOnly {
		t.Error("icons_only should be false")
	}
}

func TestParseConfigEmptyLines(t *testing.T) {
	input := "\n\ntheme = \"catppuccin\"\n\n\nicons_only = true\n\n"
	cfg := DefaultUserConfig()
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		switch key {
		case "theme":
			cfg.Theme = val
		case "icons_only":
			cfg.IconsOnly = val == "true"
		}
	}
	if cfg.Theme != "catppuccin" {
		t.Errorf("theme = %q, want catppuccin", cfg.Theme)
	}
	if !cfg.IconsOnly {
		t.Error("icons_only should be true")
	}
}

func TestParseConfigMalformed(t *testing.T) {
	// Lines without = should be skipped
	input := "no_equals_here\ntheme = \"modern\"\njust_a_key\n"
	cfg := DefaultUserConfig()
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		switch key {
		case "theme":
			cfg.Theme = val
		case "icons_only":
			cfg.IconsOnly = val == "true"
		}
	}
	if cfg.Theme != "modern" {
		t.Errorf("theme = %q, want modern", cfg.Theme)
	}
}

func TestLoadUserConfigMissing(t *testing.T) {
	// LoadUserConfig returns defaults when file doesn't exist
	cfg := LoadUserConfig()
	// Should get defaults (may or may not have a config file on this machine)
	if cfg.Theme == "" {
		t.Error("theme should not be empty")
	}
}

func TestDefaultKeyBindings(t *testing.T) {
	kb := DefaultKeyBindings()
	if kb.Prefix != "ctrl+a" {
		t.Errorf("default prefix = %q, want ctrl+a", kb.Prefix)
	}
	if kb.Quit != "q" {
		t.Errorf("default quit = %q, want q", kb.Quit)
	}
	if kb.NewTerminal != "n" {
		t.Errorf("default new_terminal = %q, want n", kb.NewTerminal)
	}
	if kb.ShowKeys != "f2" {
		t.Errorf("default show_keys = %q, want f2", kb.ShowKeys)
	}
	if kb.ToggleTiling != "f6" {
		t.Errorf("default toggle_tiling = %q, want f6", kb.ToggleTiling)
	}
	if kb.SwapLeft != "alt+h" || kb.SwapRight != "alt+l" || kb.SwapUp != "alt+k" || kb.SwapDown != "alt+j" {
		t.Errorf("default swap keys = %q/%q/%q/%q", kb.SwapLeft, kb.SwapRight, kb.SwapUp, kb.SwapDown)
	}
}

func TestParseKeybindingsSection(t *testing.T) {
	input := `theme = "retro"
icons_only = true

[keybindings]
prefix = "ctrl+]"
quit = "x"
snap_left = "a"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Theme != "retro" {
		t.Errorf("theme = %q, want retro", cfg.Theme)
	}
	if !cfg.IconsOnly {
		t.Error("icons_only should be true")
	}
	if cfg.Keys.Prefix != "ctrl+]" {
		t.Errorf("prefix = %q, want ctrl+]", cfg.Keys.Prefix)
	}
	if cfg.Keys.Quit != "x" {
		t.Errorf("quit = %q, want x", cfg.Keys.Quit)
	}
	if cfg.Keys.SnapLeft != "a" {
		t.Errorf("snap_left = %q, want a", cfg.Keys.SnapLeft)
	}
	// Non-overridden keys should keep defaults
	if cfg.Keys.NewTerminal != "n" {
		t.Errorf("new_terminal = %q, want n (default)", cfg.Keys.NewTerminal)
	}
}

func TestParseKeybindingsPartial(t *testing.T) {
	// Only override prefix, rest should stay default
	input := `[keybindings]
prefix = "ctrl+b"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Keys.Prefix != "ctrl+b" {
		t.Errorf("prefix = %q, want ctrl+b", cfg.Keys.Prefix)
	}
	// All other keys should be default
	defaults := DefaultKeyBindings()
	if cfg.Keys.Quit != defaults.Quit {
		t.Errorf("quit should remain default %q, got %q", defaults.Quit, cfg.Keys.Quit)
	}
	if cfg.Keys.Help != defaults.Help {
		t.Errorf("help should remain default %q, got %q", defaults.Help, cfg.Keys.Help)
	}
}

func TestParseUnknownSectionIgnored(t *testing.T) {
	input := `theme = "modern"

[unknown_section]
foo = "bar"

[keybindings]
prefix = "ctrl+x"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Theme != "modern" {
		t.Errorf("theme = %q, want modern", cfg.Theme)
	}
	if cfg.Keys.Prefix != "ctrl+x" {
		t.Errorf("prefix = %q, want ctrl+x", cfg.Keys.Prefix)
	}
}

func TestParseKeybindingAllKeys(t *testing.T) {
	tests := []struct {
		key string
		get func(KeyBindings) string
	}{
		{"prefix", func(kb KeyBindings) string { return kb.Prefix }},
		{"quit", func(kb KeyBindings) string { return kb.Quit }},
		{"new_terminal", func(kb KeyBindings) string { return kb.NewTerminal }},
		{"close_window", func(kb KeyBindings) string { return kb.CloseWindow }},
		{"enter_terminal", func(kb KeyBindings) string { return kb.EnterTerminal }},
		{"minimize", func(kb KeyBindings) string { return kb.Minimize }},
		{"rename", func(kb KeyBindings) string { return kb.Rename }},
		{"dock_focus", func(kb KeyBindings) string { return kb.DockFocus }},
		{"launcher", func(kb KeyBindings) string { return kb.Launcher }},
		{"snap_left", func(kb KeyBindings) string { return kb.SnapLeft }},
		{"snap_right", func(kb KeyBindings) string { return kb.SnapRight }},
		{"maximize", func(kb KeyBindings) string { return kb.Maximize }},
		{"restore", func(kb KeyBindings) string { return kb.Restore }},
		{"tile_all", func(kb KeyBindings) string { return kb.TileAll }},
		{"expose", func(kb KeyBindings) string { return kb.Expose }},
		{"next_window", func(kb KeyBindings) string { return kb.NextWindow }},
		{"prev_window", func(kb KeyBindings) string { return kb.PrevWindow }},
		{"help", func(kb KeyBindings) string { return kb.Help }},
		{"toggle_expose", func(kb KeyBindings) string { return kb.ToggleExpose }},
		{"menu_bar", func(kb KeyBindings) string { return kb.MenuBar }},
		{"menu_file", func(kb KeyBindings) string { return kb.MenuFile }},
		{"menu_apps", func(kb KeyBindings) string { return kb.MenuApps }},
		{"menu_view", func(kb KeyBindings) string { return kb.MenuView }},
		{"move_left", func(kb KeyBindings) string { return kb.MoveLeft }},
		{"move_right", func(kb KeyBindings) string { return kb.MoveRight }},
		{"move_up", func(kb KeyBindings) string { return kb.MoveUp }},
		{"move_down", func(kb KeyBindings) string { return kb.MoveDown }},
		{"grow_width", func(kb KeyBindings) string { return kb.GrowWidth }},
		{"shrink_width", func(kb KeyBindings) string { return kb.ShrinkWidth }},
		{"grow_height", func(kb KeyBindings) string { return kb.GrowHeight }},
		{"shrink_height", func(kb KeyBindings) string { return kb.ShrinkHeight }},
		{"snap_top", func(kb KeyBindings) string { return kb.SnapTop }},
		{"snap_bottom", func(kb KeyBindings) string { return kb.SnapBottom }},
		{"center", func(kb KeyBindings) string { return kb.Center }},
		{"tile_columns", func(kb KeyBindings) string { return kb.TileColumns }},
		{"tile_rows", func(kb KeyBindings) string { return kb.TileRows }},
		{"cascade", func(kb KeyBindings) string { return kb.Cascade }},
		{"clipboard_history", func(kb KeyBindings) string { return kb.ClipboardHistory }},
		{"menu_edit", func(kb KeyBindings) string { return kb.MenuEdit }},
		{"notification_center", func(kb KeyBindings) string { return kb.NotificationCenter }},
		{"settings", func(kb KeyBindings) string { return kb.Settings }},
		{"quick_next_window", func(kb KeyBindings) string { return kb.QuickNextWindow }},
		{"quick_prev_window", func(kb KeyBindings) string { return kb.QuickPrevWindow }},
		{"tile_maximized", func(kb KeyBindings) string { return kb.TileMaximized }},
		{"show_desktop", func(kb KeyBindings) string { return kb.ShowDesktop }},
		{"project_picker", func(kb KeyBindings) string { return kb.ProjectPicker }},
		{"save_workspace", func(kb KeyBindings) string { return kb.SaveWorkspace }},
		{"load_workspace", func(kb KeyBindings) string { return kb.LoadWorkspace }},
		{"new_workspace", func(kb KeyBindings) string { return kb.NewWorkspace }},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			kb := KeyBindings{}
			val := "test_" + tt.key
			parseKeybinding(&kb, tt.key, val)
			got := tt.get(kb)
			if got != val {
				t.Errorf("parseKeybinding(%q, %q): got %q, want %q", tt.key, val, got, val)
			}
		})
	}
}

func TestParseKeybindingEmptyValue(t *testing.T) {
	kb := DefaultKeyBindings()
	parseKeybinding(&kb, "prefix", "")
	if kb.Prefix != "" {
		t.Errorf("empty value should clear binding (unbind): got %q, want %q", kb.Prefix, "")
	}
}

func TestParseKeybindingUnknownKey(t *testing.T) {
	kb := DefaultKeyBindings()
	parseKeybinding(&kb, "nonexistent_key", "some_value")
	if kb.Prefix != "ctrl+a" {
		t.Errorf("unknown key should not affect existing bindings: prefix = %q", kb.Prefix)
	}
}

func TestSaveUserConfig(t *testing.T) {
	cfgPath := setTestConfigPath(t)

	cfg := UserConfig{
		Theme:                 "tokyonight",
		IconsOnly:             true,
		Animations:            false,
		AnimationSpeed:        "slow",
		AnimationStyle:        "bouncy",
		ShowDeskClock:         true,
		ShowKeys:              false,
		TilingMode:            true,
		TilingLayout:          "rows",
		MinimizeToDock:    false,
		HideDockWhenMaximized: true,
		TourCompleted:         true,
		WorkspaceAutoSave:     false,
		WorkspaceAutoSaveMin:  5,
		RecentApps:            []string{"htop", "vim"},
		Favorites:             []string{"calc"},
		Keys:                  DefaultKeyBindings(),
	}
	cfg.Keys.Prefix = "ctrl+b"
	cfg.Keys.Quit = "Q"
	cfg.Keys.CloseWindow = "ctrl+w"
	cfg.Keys.NextWindow = "ctrl+n"
	cfg.Keys.Help = "f2"

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	checks := []string{
		`theme = "tokyonight"`,
		"icons_only = true",
		"animations = false",
		`animation_speed = "slow"`,
		`animation_style = "bouncy"`,
		"show_desk_clock = true",
		"show_keys = false",
		"tiling_mode = true",
		`tiling_layout = "rows"`,
		"minimize_to_dock = false",
		"hide_dock_when_maximized = true",
		"tour_completed = true",
		"workspace_auto_save = false",
		"workspace_auto_save_min = 5",
	}
	for _, want := range checks {
		if !strings.Contains(content, want) {
			t.Errorf("saved config missing %q", want)
		}
	}

	if !strings.Contains(content, "recent_apps") {
		t.Error("saved config missing recent_apps")
	}
	if !strings.Contains(content, "favorites") {
		t.Error("saved config missing favorites")
	}

	if !strings.Contains(content, "[keybindings]") {
		t.Error("saved config missing [keybindings] section")
	}
	kbChecks := []string{
		`prefix = "ctrl+b"`,
		`quit = "Q"`,
		`close_window = "ctrl+w"`,
		`next_window = "ctrl+n"`,
		`help = "f2"`,
	}
	for _, want := range kbChecks {
		if !strings.Contains(content, want) {
			t.Errorf("saved config missing keybinding %q", want)
		}
	}

	// Default keybindings should NOT be written.
	if strings.Contains(content, `new_terminal = "n"`) {
		t.Error("default keybinding new_terminal should not be written")
	}
}

func TestSaveUserConfigDefaultKeys(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "[keybindings]") {
		t.Error("default config should not contain [keybindings] section")
	}
}

func TestSaveUserConfigWithWorkspaceHistory(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.RecentWorkspaces = []WorkspaceHistoryEntry{
		{Path: "/home/user/project1/.termdesk-workspace.toml"},
		{Path: "/home/user/project2/.termdesk-workspace.toml"},
	}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if count := strings.Count(content, "[[recent_workspaces]]"); count != 2 {
		t.Errorf("expected 2 [[recent_workspaces]] entries, got %d", count)
	}
	if !strings.Contains(content, "project1") {
		t.Error("missing project1 workspace entry")
	}
	if !strings.Contains(content, "project2") {
		t.Error("missing project2 workspace entry")
	}
}

func TestSaveAndReloadRoundTrip(t *testing.T) {
	setTestConfigPath(t)

	cfg := UserConfig{
		Theme:                 "nord",
		IconsOnly:             true,
		Animations:            true,
		AnimationSpeed:        "fast",
		AnimationStyle:        "snappy",
		ShowDeskClock:         true,
		ShowKeys:              true,
		TilingMode:            true,
		TilingLayout:          "all",
		MinimizeToDock:    true,
		HideDockWhenMaximized: false,
		TourCompleted:         false,
		WorkspaceAutoSave:     true,
		WorkspaceAutoSaveMin:  3,
		Keys:                  DefaultKeyBindings(),
	}
	cfg.Keys.Prefix = "ctrl+z"

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	loaded := LoadUserConfig()

	if loaded.Theme != "nord" {
		t.Errorf("round-trip theme: got %q, want %q", loaded.Theme, "nord")
	}
	if !loaded.IconsOnly {
		t.Error("round-trip IconsOnly should be true")
	}
	if !loaded.Animations {
		t.Error("round-trip Animations should be true")
	}
	if loaded.AnimationSpeed != "fast" {
		t.Errorf("round-trip AnimationSpeed: got %q, want %q", loaded.AnimationSpeed, "fast")
	}
	if loaded.AnimationStyle != "snappy" {
		t.Errorf("round-trip AnimationStyle: got %q, want %q", loaded.AnimationStyle, "snappy")
	}
	if !loaded.ShowDeskClock {
		t.Error("round-trip ShowDeskClock should be true")
	}
	if !loaded.ShowKeys {
		t.Error("round-trip ShowKeys should be true")
	}
	if !loaded.TilingMode {
		t.Error("round-trip TilingMode should be true")
	}
	if loaded.TilingLayout != "all" {
		t.Errorf("round-trip TilingLayout: got %q, want %q", loaded.TilingLayout, "all")
	}
	if !loaded.MinimizeToDock {
		t.Error("round-trip MinimizeToDock should be true")
	}
	if loaded.HideDockWhenMaximized {
		t.Error("round-trip HideDockWhenMaximized should be false")
	}
	if loaded.WorkspaceAutoSaveMin != 3 {
		t.Errorf("round-trip WorkspaceAutoSaveMin: got %d, want 3", loaded.WorkspaceAutoSaveMin)
	}
	if loaded.Keys.Prefix != "ctrl+z" {
		t.Errorf("round-trip prefix: got %q, want %q", loaded.Keys.Prefix, "ctrl+z")
	}
}

func TestConfigDirAndPath(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	dir := configDir()
	if dir == "" {
		t.Fatal("configDir returned empty")
	}
	if dir != filepath.Dir(cfgFile) {
		t.Errorf("configDir = %q, want %q", dir, filepath.Dir(cfgFile))
	}

	path := configPath()
	if path == "" {
		t.Fatal("configPath returned empty")
	}
	if path != cfgFile {
		t.Errorf("configPath = %q, want %q", path, cfgFile)
	}
}

func TestParseConfigAllFields(t *testing.T) {
	input := `theme = "dracula"
icons_only = true
animations = false
animation_speed = "slow"
animation_style = "bouncy"
show_desk_clock = true
show_keys = true
tiling_mode = true
tiling_layout = "rows"
minimize_to_dock = false
hide_dock_when_maximized = true
tour_completed = true
workspace_auto_save = false
workspace_auto_save_min = 10
recent_apps = "htop,vim,calc"
favorites = "calc,htop"
log_level = "debug"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Theme != "dracula" {
		t.Errorf("theme = %q, want dracula", cfg.Theme)
	}
	if !cfg.IconsOnly {
		t.Error("IconsOnly should be true")
	}
	if cfg.Animations {
		t.Error("Animations should be false")
	}
	if cfg.AnimationSpeed != "slow" {
		t.Errorf("AnimationSpeed = %q, want slow", cfg.AnimationSpeed)
	}
	if cfg.AnimationStyle != "bouncy" {
		t.Errorf("AnimationStyle = %q, want bouncy", cfg.AnimationStyle)
	}
	if !cfg.ShowDeskClock {
		t.Error("ShowDeskClock should be true")
	}
	if !cfg.ShowKeys {
		t.Error("ShowKeys should be true")
	}
	if !cfg.TilingMode {
		t.Error("TilingMode should be true")
	}
	if cfg.TilingLayout != "rows" {
		t.Errorf("TilingLayout = %q, want rows", cfg.TilingLayout)
	}
	if cfg.MinimizeToDock {
		t.Error("MinimizeToDock should be false")
	}
	if !cfg.HideDockWhenMaximized {
		t.Error("HideDockWhenMaximized should be true")
	}
	if !cfg.TourCompleted {
		t.Error("TourCompleted should be true")
	}
	if cfg.WorkspaceAutoSave {
		t.Error("WorkspaceAutoSave should be false")
	}
	if cfg.WorkspaceAutoSaveMin != 10 {
		t.Errorf("WorkspaceAutoSaveMin = %d, want 10", cfg.WorkspaceAutoSaveMin)
	}
	if len(cfg.RecentApps) != 3 || cfg.RecentApps[0] != "htop" {
		t.Errorf("RecentApps = %v, want [htop vim calc]", cfg.RecentApps)
	}
	if len(cfg.Favorites) != 2 || cfg.Favorites[0] != "calc" {
		t.Errorf("Favorites = %v, want [calc htop]", cfg.Favorites)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestParseConfigWorkspaceHistory(t *testing.T) {
	input := `theme = "modern"

[[recent_workspaces]]
path = "/home/user/project1/.termdesk-workspace.toml"
last_access = "2025-01-15T10:30:00Z"

[[recent_workspaces]]
path = "/home/user/project2/.termdesk-workspace.toml"
last_access = "2025-02-20T14:00:00Z"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Theme != "modern" {
		t.Errorf("theme = %q, want modern", cfg.Theme)
	}
	if len(cfg.RecentWorkspaces) != 2 {
		t.Fatalf("RecentWorkspaces: got %d entries, want 2", len(cfg.RecentWorkspaces))
	}
	if cfg.RecentWorkspaces[0].Path != "/home/user/project1/.termdesk-workspace.toml" {
		t.Errorf("workspace[0].Path = %q", cfg.RecentWorkspaces[0].Path)
	}
	if cfg.RecentWorkspaces[1].Path != "/home/user/project2/.termdesk-workspace.toml" {
		t.Errorf("workspace[1].Path = %q", cfg.RecentWorkspaces[1].Path)
	}
	if cfg.RecentWorkspaces[0].LastAccess.Year() != 2025 {
		t.Errorf("workspace[0].LastAccess year = %d, want 2025", cfg.RecentWorkspaces[0].LastAccess.Year())
	}
}

func TestParseConfigWorkspaceHistoryBadDate(t *testing.T) {
	input := `
[[recent_workspaces]]
path = "/home/user/project/.termdesk-workspace.toml"
last_access = "not-a-date"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if len(cfg.RecentWorkspaces) != 1 {
		t.Fatalf("RecentWorkspaces: got %d entries, want 1", len(cfg.RecentWorkspaces))
	}
	if cfg.RecentWorkspaces[0].Path != "/home/user/project/.termdesk-workspace.toml" {
		t.Errorf("workspace path = %q", cfg.RecentWorkspaces[0].Path)
	}
	if !cfg.RecentWorkspaces[0].LastAccess.IsZero() {
		t.Error("bad date should result in zero LastAccess")
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := splitList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseIntSafe(t *testing.T) {
	if got := parseIntSafe("42"); got != 42 {
		t.Errorf("parseIntSafe(42) = %d", got)
	}
	if got := parseIntSafe("0"); got != 0 {
		t.Errorf("parseIntSafe(0) = %d", got)
	}
	if got := parseIntSafe("not_a_number"); got != 0 {
		t.Errorf("parseIntSafe(not_a_number) = %d", got)
	}
	if got := parseIntSafe(""); got != 0 {
		t.Errorf("parseIntSafe('') = %d", got)
	}
}

func TestSaveUserConfigAllKeybindingsNonDefault(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.Keys = KeyBindings{
		Prefix:             "ctrl+b",
		Quit:               "Q",
		NewTerminal:        "ctrl+n",
		CloseWindow:        "ctrl+w",
		EnterTerminal:      "ctrl+i",
		Minimize:           "ctrl+m",
		Rename:             "ctrl+r",
		DockFocus:          "ctrl+d",
		Launcher:           "ctrl+space",
		SnapLeft:           "ctrl+h",
		SnapRight:          "ctrl+l",
		Maximize:           "ctrl+k",
		Restore:            "ctrl+j",
		TileAll:            "ctrl+t",
		Expose:             "ctrl+x",
		NextWindow:         "ctrl+tab",
		PrevWindow:         "ctrl+shift+tab",
		Help:               "f2",
		ToggleExpose:       "f8",
		MenuBar:            "f11",
		MenuFile:           "ctrl+f",
		MenuApps:           "ctrl+a",
		MenuView:           "ctrl+v",
		MoveLeft:           "super+left",
		MoveRight:          "super+right",
		MoveUp:             "super+up",
		MoveDown:           "super+down",
		GrowWidth:          "super+alt+right",
		ShrinkWidth:        "super+alt+left",
		GrowHeight:         "super+alt+down",
		ShrinkHeight:       "super+alt+up",
		SnapTop:            "ctrl+u",
		SnapBottom:         "ctrl+p",
		Center:             "ctrl+o",
		TileColumns:        "ctrl+|",
		TileRows:           "ctrl+_",
		Cascade:            "ctrl+\\",
		ClipboardHistory:   "ctrl+y",
		MenuEdit:           "ctrl+e",
		NotificationCenter: "ctrl+b",
		Settings:           "ctrl+,",
		QuickNextWindow:    "ctrl+]",
		QuickPrevWindow:    "ctrl+[",
		TileMaximized:      "ctrl+g",
		ShowDesktop:        "ctrl+s",
		ProjectPicker:      "ctrl+alt+p",
		SaveWorkspace:      "ctrl+alt+s",
		LoadWorkspace:      "ctrl+alt+o",
		NewWorkspace:       "ctrl+alt+n",
		ShowKeys:           "ctrl+alt+k",
		ToggleTiling:       "ctrl+alt+t",
		SwapLeft:           "ctrl+alt+h",
		SwapRight:          "ctrl+alt+l",
		SwapUp:             "ctrl+alt+k",
		SwapDown:           "ctrl+alt+j",
	}

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	kbNames := []string{
		"prefix", "quit", "new_terminal", "close_window", "enter_terminal",
		"minimize", "rename", "dock_focus", "launcher", "snap_left", "snap_right",
		"maximize", "restore", "tile_all", "expose", "next_window", "prev_window",
		"help", "toggle_expose", "menu_bar", "menu_file", "menu_apps", "menu_view",
		"move_left", "move_right", "move_up", "move_down",
		"grow_width", "shrink_width", "grow_height", "shrink_height",
		"snap_top", "snap_bottom", "center", "tile_columns", "tile_rows", "cascade",
		"clipboard_history", "menu_edit", "notification_center", "settings",
		"quick_next_window", "quick_prev_window", "tile_maximized", "show_desktop",
		"project_picker", "save_workspace", "load_workspace", "new_workspace", "show_keys",
		"toggle_tiling", "swap_left", "swap_right", "swap_up", "swap_down",
	}
	for _, name := range kbNames {
		if !strings.Contains(content, name+" = ") {
			t.Errorf("saved config missing keybinding %q", name)
		}
	}
}

func TestLoadUserConfigEmptyHome(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv(envConfigPath, cfgFile)

	cfg := LoadUserConfig()
	if cfg.Theme != "modern" {
		t.Errorf("expected default theme, got %q", cfg.Theme)
	}
}

func TestSaveUserConfigNoAnimationSpeedOrStyle(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.AnimationSpeed = ""
	cfg.AnimationStyle = ""

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "animation_speed") {
		t.Error("empty AnimationSpeed should not be written")
	}
	if strings.Contains(content, "animation_style") {
		t.Error("empty AnimationStyle should not be written")
	}
}

func TestSaveUserConfigNoRecentAppsOrFavorites(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.RecentApps = nil
	cfg.Favorites = nil

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "recent_apps") {
		t.Error("nil RecentApps should not be written")
	}
	if strings.Contains(content, "favorites") {
		t.Error("nil Favorites should not be written")
	}
}

func TestSaveUserConfigZeroAutoSaveMin(t *testing.T) {
	cfgFile := setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.WorkspaceAutoSaveMin = 0

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "workspace_auto_save_min") {
		t.Error("zero WorkspaceAutoSaveMin should not be written")
	}
}

func TestParseConfigInvalidAutoSaveMin(t *testing.T) {
	input := `workspace_auto_save_min = 0
`
	cfg := DefaultUserConfig()
	originalMin := cfg.WorkspaceAutoSaveMin
	parseConfig(&cfg, input)

	// 0 is not > 0, so the original default should be preserved.
	if cfg.WorkspaceAutoSaveMin != originalMin {
		t.Errorf("WorkspaceAutoSaveMin = %d, want %d (unchanged)", cfg.WorkspaceAutoSaveMin, originalMin)
	}
}

func TestConfigDirPublicWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(envConfigDir, tmpDir)
	t.Setenv(envConfigPath, "")
	got := ConfigDir()
	if got != tmpDir {
		t.Errorf("ConfigDir() = %q, want %q", got, tmpDir)
	}
}

func TestConfigPathPublicWrapper(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv(envConfigPath, cfgFile)
	got := ConfigPath()
	if got != cfgFile {
		t.Errorf("ConfigPath() = %q, want %q", got, cfgFile)
	}
}

func TestConfigDirFromConfigPathEnv(t *testing.T) {
	// When TERMDESK_CONFIG_PATH is set, configDir should return its directory.
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "sub", "config.toml")
	t.Setenv(envConfigPath, cfgFile)
	t.Setenv(envConfigDir, "")
	got := ConfigDir()
	want := filepath.Join(tmpDir, "sub")
	if got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestParseConfigCustomWidgets(t *testing.T) {
	input := `theme = "modern"

[[custom_widgets]]
name = "git_branch"
label = "Git Branch"
icon = "\ue725"
command = "git branch --show-current"
interval = 5
onClick = "lazygit"

[[custom_widgets]]
name = "uptime"
label = "Uptime"
command = "uptime -p"
interval = 60
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.Theme != "modern" {
		t.Errorf("theme = %q, want modern", cfg.Theme)
	}
	if len(cfg.CustomWidgets) != 2 {
		t.Fatalf("CustomWidgets: got %d entries, want 2", len(cfg.CustomWidgets))
	}

	w0 := cfg.CustomWidgets[0]
	if w0.Name != "git_branch" {
		t.Errorf("widget[0].Name = %q, want %q", w0.Name, "git_branch")
	}
	if w0.Label != "Git Branch" {
		t.Errorf("widget[0].Label = %q, want %q", w0.Label, "Git Branch")
	}
	if w0.Command != "git branch --show-current" {
		t.Errorf("widget[0].Command = %q, want %q", w0.Command, "git branch --show-current")
	}
	if w0.Interval != 5 {
		t.Errorf("widget[0].Interval = %d, want 5", w0.Interval)
	}
	if w0.OnClick != "lazygit" {
		t.Errorf("widget[0].OnClick = %q, want %q", w0.OnClick, "lazygit")
	}

	w1 := cfg.CustomWidgets[1]
	if w1.Name != "uptime" {
		t.Errorf("widget[1].Name = %q, want %q", w1.Name, "uptime")
	}
	if w1.Interval != 60 {
		t.Errorf("widget[1].Interval = %d, want 60", w1.Interval)
	}
	if w1.OnClick != "" {
		t.Errorf("widget[1].OnClick = %q, want empty", w1.OnClick)
	}
}

func TestParseConfigCustomWidgetOnClick(t *testing.T) {
	// Test the on_click (snake_case) alias as well.
	input := `
[[custom_widgets]]
name = "test"
command = "echo test"
on_click = "htop"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if len(cfg.CustomWidgets) != 1 {
		t.Fatalf("CustomWidgets: got %d entries, want 1", len(cfg.CustomWidgets))
	}
	if cfg.CustomWidgets[0].OnClick != "htop" {
		t.Errorf("on_click = %q, want %q", cfg.CustomWidgets[0].OnClick, "htop")
	}
}

func TestParseConfigEnabledWidgets(t *testing.T) {
	input := `enabled_widgets = "clock,battery,cpu"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if len(cfg.EnabledWidgets) != 3 {
		t.Fatalf("EnabledWidgets: got %d entries, want 3", len(cfg.EnabledWidgets))
	}
	expected := []string{"clock", "battery", "cpu"}
	for i, want := range expected {
		if cfg.EnabledWidgets[i] != want {
			t.Errorf("EnabledWidgets[%d] = %q, want %q", i, cfg.EnabledWidgets[i], want)
		}
	}
}

func TestSaveAndReloadCustomWidgets(t *testing.T) {
	setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.CustomWidgets = []CustomWidgetDef{
		{
			Name:     "weather",
			Label:    "Weather",
			Icon:     "W",
			Command:  "curl wttr.in?format=3",
			Interval: 300,
			OnClick:  "open https://weather.com",
		},
	}

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	loaded := LoadUserConfig()
	if len(loaded.CustomWidgets) != 1 {
		t.Fatalf("round-trip CustomWidgets: got %d, want 1", len(loaded.CustomWidgets))
	}
	w := loaded.CustomWidgets[0]
	if w.Name != "weather" {
		t.Errorf("round-trip widget Name = %q, want %q", w.Name, "weather")
	}
	if w.Command != "curl wttr.in?format=3" {
		t.Errorf("round-trip widget Command = %q, want %q", w.Command, "curl wttr.in?format=3")
	}
	if w.Interval != 300 {
		t.Errorf("round-trip widget Interval = %d, want 300", w.Interval)
	}
	if w.OnClick != "open https://weather.com" {
		t.Errorf("round-trip widget OnClick = %q, want %q", w.OnClick, "open https://weather.com")
	}
}

func TestSaveAndReloadEnabledWidgets(t *testing.T) {
	setTestConfigPath(t)

	cfg := DefaultUserConfig()
	cfg.EnabledWidgets = []string{"clock", "cpu", "memory"}

	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}

	loaded := LoadUserConfig()
	if len(loaded.EnabledWidgets) != 3 {
		t.Fatalf("round-trip EnabledWidgets: got %d, want 3", len(loaded.EnabledWidgets))
	}
	expected := []string{"clock", "cpu", "memory"}
	for i, want := range expected {
		if loaded.EnabledWidgets[i] != want {
			t.Errorf("round-trip EnabledWidgets[%d] = %q, want %q", i, loaded.EnabledWidgets[i], want)
		}
	}
}

func TestParseConfigMixedCustomWidgetsAndWorkspaces(t *testing.T) {
	// Test that custom widgets and workspace history can coexist.
	input := `theme = "nord"

[[custom_widgets]]
name = "test_widget"
command = "echo hello"
interval = 10

[[recent_workspaces]]
path = "/tmp/project/.termdesk-workspace.toml"
last_access = "2025-06-01T12:00:00Z"
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if len(cfg.CustomWidgets) != 1 {
		t.Fatalf("CustomWidgets: got %d entries, want 1", len(cfg.CustomWidgets))
	}
	if cfg.CustomWidgets[0].Name != "test_widget" {
		t.Errorf("widget name = %q, want %q", cfg.CustomWidgets[0].Name, "test_widget")
	}

	if len(cfg.RecentWorkspaces) != 1 {
		t.Fatalf("RecentWorkspaces: got %d entries, want 1", len(cfg.RecentWorkspaces))
	}
	if cfg.RecentWorkspaces[0].Path != "/tmp/project/.termdesk-workspace.toml" {
		t.Errorf("workspace path = %q", cfg.RecentWorkspaces[0].Path)
	}
}

func TestParseConfigQuakeHeightPercent(t *testing.T) {
	input := `quake_height_percent = 60
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if cfg.QuakeHeightPercent != 60 {
		t.Errorf("QuakeHeightPercent = %d, want 60", cfg.QuakeHeightPercent)
	}
}

func TestParseConfigQuakeHeightPercentOutOfRange(t *testing.T) {
	// Below 10 should not change the default.
	input := `quake_height_percent = 5
`
	cfg := DefaultUserConfig()
	original := cfg.QuakeHeightPercent
	parseConfig(&cfg, input)
	if cfg.QuakeHeightPercent != original {
		t.Errorf("QuakeHeightPercent = %d, want %d (unchanged for < 10)", cfg.QuakeHeightPercent, original)
	}

	// Above 90 should not change the default.
	input = `quake_height_percent = 95
`
	cfg = DefaultUserConfig()
	original = cfg.QuakeHeightPercent
	parseConfig(&cfg, input)
	if cfg.QuakeHeightPercent != original {
		t.Errorf("QuakeHeightPercent = %d, want %d (unchanged for > 90)", cfg.QuakeHeightPercent, original)
	}
}

func TestParseConfigBooleanFields(t *testing.T) {
	input := `default_terminal_mode = true
hide_dock_apps = true
focus_follows_mouse = true
show_resize_indicator = true
`
	cfg := DefaultUserConfig()
	parseConfig(&cfg, input)

	if !cfg.DefaultTerminalMode {
		t.Error("DefaultTerminalMode should be true")
	}
	if !cfg.HideDockApps {
		t.Error("HideDockApps should be true")
	}
	if !cfg.FocusFollowsMouse {
		t.Error("FocusFollowsMouse should be true")
	}
	if !cfg.ShowResizeIndicator {
		t.Error("ShowResizeIndicator should be true")
	}
}

func TestNormalizeTilingLayout(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"columns", "columns"},
		{"Columns", "columns"},
		{"rows", "rows"},
		{"Rows", "rows"},
		{"all", "all"},
		{"tile_all", "all"},
		{"grid", "all"},
		{"unknown", "columns"},
		{"", "columns"},
	}
	for _, tc := range tests {
		got := normalizeTilingLayout(tc.input)
		if got != tc.want {
			t.Errorf("normalizeTilingLayout(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseConfigLegacyShowLaunchedInDock(t *testing.T) {
	// The legacy key "show_launched_in_dock" should be an alias for minimize_to_dock.
	input := `show_launched_in_dock = true
`
	cfg := DefaultUserConfig()
	cfg.MinimizeToDock = false
	parseConfig(&cfg, input)

	if !cfg.MinimizeToDock {
		t.Error("show_launched_in_dock should set MinimizeToDock to true")
	}
}
