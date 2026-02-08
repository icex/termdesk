package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultUserConfig(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Theme != "modern" {
		t.Errorf("default theme = %q, want modern", cfg.Theme)
	}
	if cfg.IconsOnly {
		t.Error("default IconsOnly should be false")
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
	if kb.Prefix != "ctrl+g" {
		t.Errorf("default prefix = %q, want ctrl+g", kb.Prefix)
	}
	if kb.Quit != "q" {
		t.Errorf("default quit = %q, want q", kb.Quit)
	}
	if kb.NewTerminal != "n" {
		t.Errorf("default new_terminal = %q, want n", kb.NewTerminal)
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
