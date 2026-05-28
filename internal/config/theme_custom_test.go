package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCustomThemeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// TestMain sets TERMDESK_CONFIG_PATH globally; override it here so
	// configDir() resolves to this test's tempdir.
	t.Setenv(envConfigPath, filepath.Join(dir, "config.toml"))
	t.Setenv(envConfigDir, dir)

	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `
name = "ignored"
content_bg = "#FAFAFA"
active_border_fg = "#FF00FF"
active_title_fg = "#111111"
menu_bar_bg = "#EEEEEE"
dock_bg = "#EEEEEE"
dock_accent_bg = "#DDDDDD"
desktop_bg = "#F0F0F0"
accent_color = "#FF00FF"
accent_fg = "#FFFFFF"
subtle_fg = "#888888"
button_yes_bg = "#00AA00"
button_no_bg = "#AA0000"
button_fg = "#FFFFFF"
snap_left_button = " < "
snap_right_button = " > "
close_button = " X "
min_button = " _ "
max_button = " # "
border_top_left = "+"
title_bar_height = 1
unfocused_fade = 0.3
ansi0 = "#123456"
ansi_15 = "#ABCDEF"
`
	if err := os.WriteFile(filepath.Join(themesDir, "mytheme.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	theme, ok := LoadCustomTheme("mytheme")
	if !ok {
		t.Fatal("LoadCustomTheme returned !ok")
	}
	if theme.Name != "mytheme" { // name field in file should be overridden by filename
		t.Errorf("Name = %q, want mytheme", theme.Name)
	}
	if theme.ContentBg != "#FAFAFA" {
		t.Errorf("ContentBg = %q", theme.ContentBg)
	}
	if theme.BorderTopLeft != '+' {
		t.Errorf("BorderTopLeft = %q, want '+'", theme.BorderTopLeft)
	}
	if theme.UnfocusedFade != 0.3 {
		t.Errorf("UnfocusedFade = %v", theme.UnfocusedFade)
	}
	if theme.ANSIPalette[0] != "#123456" {
		t.Errorf("ANSIPalette[0] = %q", theme.ANSIPalette[0])
	}
	if theme.ANSIPalette[15] != "#ABCDEF" {
		t.Errorf("ANSIPalette[15] = %q", theme.ANSIPalette[15])
	}
	if theme.C().ActiveBorderFg == nil {
		t.Error("ParseColors was not called")
	}
}

func TestLoadCustomThemeMissing(t *testing.T) {
	dir := t.TempDir()
	// TestMain sets TERMDESK_CONFIG_PATH globally; override it here so
	// configDir() resolves to this test's tempdir.
	t.Setenv(envConfigPath, filepath.Join(dir, "config.toml"))
	t.Setenv(envConfigDir, dir)
	if _, ok := LoadCustomTheme("nope"); ok {
		t.Error("expected ok=false for missing theme")
	}
}

func TestLoadCustomThemeRejectsPathSeparators(t *testing.T) {
	dir := t.TempDir()
	// TestMain sets TERMDESK_CONFIG_PATH globally; override it here so
	// configDir() resolves to this test's tempdir.
	t.Setenv(envConfigPath, filepath.Join(dir, "config.toml"))
	t.Setenv(envConfigDir, dir)
	if _, ok := LoadCustomTheme("../etc/passwd"); ok {
		t.Error("expected LoadCustomTheme to reject path separators")
	}
}

func TestGetThemeCustomOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	// TestMain sets TERMDESK_CONFIG_PATH globally; override it here so
	// configDir() resolves to this test's tempdir.
	t.Setenv(envConfigPath, filepath.Join(dir, "config.toml"))
	t.Setenv(envConfigDir, dir)
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Override the built-in "modern" theme with a custom file.
	body := `content_bg = "#010101"` + "\n"
	if err := os.WriteFile(filepath.Join(themesDir, "modern.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	th := GetTheme("modern")
	if th.ContentBg != "#010101" {
		t.Errorf("custom override not applied: ContentBg = %q", th.ContentBg)
	}
}

func TestThemeNamesIncludesCustom(t *testing.T) {
	dir := t.TempDir()
	// TestMain sets TERMDESK_CONFIG_PATH globally; override it here so
	// configDir() resolves to this test's tempdir.
	t.Setenv(envConfigPath, filepath.Join(dir, "config.toml"))
	t.Setenv(envConfigDir, dir)
	themesDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themesDir, "my-custom.toml"), []byte("content_bg = \"#000\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := ThemeNames()
	found := false
	for _, n := range names {
		if n == "my-custom" {
			found = true
		}
	}
	if !found {
		t.Errorf("ThemeNames() missing custom theme: %v", names)
	}
}
