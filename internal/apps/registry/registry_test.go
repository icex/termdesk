package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripTermdeskPrefix(t *testing.T) {
	if got := StripTermdeskPrefix("termdesk-calc"); got != "calc" {
		t.Fatalf("got %q", got)
	}
	if got := StripTermdeskPrefix("/usr/bin/termdesk-calc"); got != "calc" {
		t.Fatalf("got %q", got)
	}
	if got := StripTermdeskPrefix("plain"); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestStripTermdeskPrefixNoPrefix(t *testing.T) {
	if got := StripTermdeskPrefix("/usr/local/bin/myapp"); got != "myapp" {
		t.Fatalf("expected 'myapp', got %q", got)
	}
}

func TestDockEntries(t *testing.T) {
	entries := []RegistryEntry{
		{Name: "Terminal", Command: "$SHELL"},
		{Name: "Skip", Command: "spf"},
		{Name: "Keep", Command: "nvim"},
	}
	out := DockEntries(entries)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[1].Command != "nvim" {
		t.Fatalf("unexpected command %q", out[1].Command)
	}
}

func TestDockEntriesFiltersTermdeskPrefix(t *testing.T) {
	entries := []RegistryEntry{
		{Name: "Terminal", Command: "$SHELL"},
		{Name: "Calc", Command: "termdesk-calc"},
		{Name: "Snake", Command: "termdesk-snake"},
	}
	out := DockEntries(entries)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
	if out[0].Command != "$SHELL" {
		t.Fatalf("expected $SHELL, got %q", out[0].Command)
	}
}

func TestDockEntriesFiltersAllSkipped(t *testing.T) {
	entries := []RegistryEntry{
		{Name: "Terminal", Command: "$SHELL"},
		{Name: "claude", Command: "claude"},
		{Name: "tetrigo", Command: "tetrigo"},
		{Name: "btop", Command: "btop"},
		{Name: "ranger", Command: "ranger"},
		{Name: "lf", Command: "lf"},
		{Name: "htop", Command: "htop"},
	}
	out := DockEntries(entries)
	// Only $SHELL and htop should remain
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestDockEntriesEmpty(t *testing.T) {
	out := DockEntries(nil)
	if len(out) != 0 {
		t.Fatalf("expected empty, got %d", len(out))
	}
}

func TestFindBinaryAndManifestForCommand(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	bin := filepath.Join(dir, "termdesk-calc")
	_ = os.WriteFile(bin, []byte(""), 0o700)
	_ = os.WriteFile(bin+".toml", []byte("[app]\nname = \"Calc\"\n"), 0o600)

	if got := FindBinary(exe, "termdesk-calc"); got != bin {
		t.Fatalf("FindBinary=%q want %q", got, bin)
	}

	m := ManifestForCommand(exe, "termdesk-calc")
	if m == nil || m.App.Name != "Calc" {
		t.Fatalf("manifest not loaded")
	}

	unknown := "termdesk-nope-should-not-exist"
	if got := FindBinary(exe, unknown); got != unknown {
		t.Fatalf("FindBinary unknown=%q", got)
	}
}

func TestFindBinaryEmptyExePath(t *testing.T) {
	// With empty exePath, FindBinary should fall back to the name itself
	got := FindBinary("", "nonexistent-binary-xyz-12345")
	if got != "nonexistent-binary-xyz-12345" {
		t.Fatalf("expected raw name, got %q", got)
	}
}

func TestFindBinaryNextToExe(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	target := filepath.Join(dir, "myutil")
	_ = os.WriteFile(target, []byte(""), 0o700)

	got := FindBinary(exe, "myutil")
	if got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestManifestForCommandNotFound(t *testing.T) {
	m := ManifestForCommand("", "nonexistent-cmd-xyz-99999")
	if m != nil {
		t.Fatalf("expected nil for unfound command")
	}
}

func TestManifestForCommandEmptyExePath(t *testing.T) {
	m := ManifestForCommand("", "some-cmd-that-does-not-exist-xyz")
	if m != nil {
		t.Fatalf("expected nil for empty exePath with unknown command")
	}
}

func TestManifestForCommandByBasename(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	// Create a manifest for "myapp" next to exe
	bin := filepath.Join(dir, "myapp")
	_ = os.WriteFile(bin, []byte(""), 0o700)
	_ = os.WriteFile(bin+".toml", []byte("[app]\nname = \"MyApp\"\n"), 0o600)

	// Pass a full path like /some/other/dir/myapp — the basename fallback should find it
	m := ManifestForCommand(exe, "/some/other/dir/myapp")
	if m == nil {
		t.Fatalf("expected manifest via basename fallback")
	}
	if m.App.Name != "MyApp" {
		t.Fatalf("name=%q", m.App.Name)
	}
}

func TestManifestForCommandBasenameSameAsCommand(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	// command has no path component, so base == command — skip redundant lookup
	m := ManifestForCommand(exe, "nope-not-here-xyz")
	if m != nil {
		t.Fatalf("expected nil")
	}
}

func TestBuildRegistryWithManifests(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	// Create a manifest app binary and its TOML
	app := filepath.Join(dir, "termdesk-calc")
	_ = os.WriteFile(app, []byte(""), 0o700)
	_ = os.WriteFile(app+".toml", []byte("[app]\nname = \"Calc\"\nicon = \"C\"\nicon_color = \"#aabbcc\"\ncategory = \"tools\"\n\n[window]\nwidth = 60\nheight = 20\n"), 0o600)

	entries := BuildRegistry(exe)
	// Should always have Terminal first
	if len(entries) < 1 {
		t.Fatalf("expected at least 1 entry")
	}
	if entries[0].Name != "Terminal" {
		t.Fatalf("first entry should be Terminal, got %q", entries[0].Name)
	}

	// Find the manifest entry
	found := false
	for _, e := range entries {
		if e.Command == "termdesk-calc" {
			found = true
			if e.Name != "Calc" {
				t.Fatalf("name=%q", e.Name)
			}
			if e.IconColor != "#aabbcc" {
				t.Fatalf("icon_color=%q", e.IconColor)
			}
			if e.Category != "tools" {
				t.Fatalf("category=%q", e.Category)
			}
			if e.Window.Width != 60 || e.Window.Height != 20 {
				t.Fatalf("window=%+v", e.Window)
			}
		}
	}
	if !found {
		t.Fatalf("termdesk-calc entry not found in registry")
	}
}

func TestBuildRegistryEmptyExePath(t *testing.T) {
	entries := BuildRegistry("")
	// Should still have Terminal
	if len(entries) < 1 {
		t.Fatalf("expected at least 1 entry")
	}
	if entries[0].Name != "Terminal" {
		t.Fatalf("first entry should be Terminal, got %q", entries[0].Name)
	}
}

func TestBuildRegistryBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "termdesk")
	_ = os.WriteFile(exe, []byte(""), 0o700)

	// Create a manifest TOML but no binary — should be skipped
	_ = os.WriteFile(filepath.Join(dir, "termdesk-ghost.toml"), []byte("[app]\nname = \"Ghost\"\n"), 0o600)

	entries := BuildRegistry(exe)
	for _, e := range entries {
		if e.Command == "termdesk-ghost" {
			t.Fatalf("ghost entry should have been skipped (no binary)")
		}
	}
}
