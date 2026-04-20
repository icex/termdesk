package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProjectConfigAutoStart(t *testing.T) {
	cfg := &ProjectConfig{ProjectDir: "/tmp"}
	parseProjectConfig(cfg, `
[[autostart]]
command = "npm"
args = ["run", "dev"]
directory = "frontend"
title = "Dev Server"

[[autostart]]
command = "go"
args = ["run", "."]
directory = "backend"
title = "API Server"
`)

	if len(cfg.AutoStart) != 2 {
		t.Fatalf("expected 2 autostart items, got %d", len(cfg.AutoStart))
	}

	// First item
	item := cfg.AutoStart[0]
	if item.Command != "npm" {
		t.Errorf("item[0] command: got %q, want %q", item.Command, "npm")
	}
	if len(item.Args) != 2 || item.Args[0] != "run" || item.Args[1] != "dev" {
		t.Errorf("item[0] args: got %v, want [run dev]", item.Args)
	}
	if item.Directory != "frontend" {
		t.Errorf("item[0] directory: got %q, want %q", item.Directory, "frontend")
	}
	if item.Title != "Dev Server" {
		t.Errorf("item[0] title: got %q, want %q", item.Title, "Dev Server")
	}

	// Second item
	item = cfg.AutoStart[1]
	if item.Command != "go" {
		t.Errorf("item[1] command: got %q, want %q", item.Command, "go")
	}
	if len(item.Args) != 2 || item.Args[0] != "run" || item.Args[1] != "." {
		t.Errorf("item[1] args: got %v, want [run .]", item.Args)
	}
	if item.Directory != "backend" {
		t.Errorf("item[1] directory: got %q, want %q", item.Directory, "backend")
	}
}

func TestParseProjectConfigNoArgs(t *testing.T) {
	cfg := &ProjectConfig{ProjectDir: "/tmp"}
	parseProjectConfig(cfg, `
[[autostart]]
command = "htop"
title = "System Monitor"
`)

	if len(cfg.AutoStart) != 1 {
		t.Fatalf("expected 1 autostart item, got %d", len(cfg.AutoStart))
	}
	item := cfg.AutoStart[0]
	if item.Command != "htop" {
		t.Errorf("command: got %q, want %q", item.Command, "htop")
	}
	if item.Args != nil {
		t.Errorf("args should be nil, got %v", item.Args)
	}
	// Default directory should be "."
	if item.Directory != "." {
		t.Errorf("directory: got %q, want %q", item.Directory, ".")
	}
}

func TestParseProjectConfigComments(t *testing.T) {
	cfg := &ProjectConfig{ProjectDir: "/tmp"}
	parseProjectConfig(cfg, `
# Project config
[[autostart]]
# Dev server
command = "node"
args = ["server.js"]
`)

	if len(cfg.AutoStart) != 1 {
		t.Fatalf("expected 1 autostart item, got %d", len(cfg.AutoStart))
	}
	if cfg.AutoStart[0].Command != "node" {
		t.Errorf("command: got %q, want %q", cfg.AutoStart[0].Command, "node")
	}
	if len(cfg.AutoStart[0].Args) != 1 || cfg.AutoStart[0].Args[0] != "server.js" {
		t.Errorf("args: got %v, want [server.js]", cfg.AutoStart[0].Args)
	}
}

func TestFindProjectConfig(t *testing.T) {
	dir := t.TempDir()

	// Write .termdesk.toml
	content := `
[[autostart]]
command = "make"
args = ["run"]
title = "Build"
`
	if err := os.WriteFile(filepath.Join(dir, ".termdesk.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := FindProjectConfig(dir)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.ProjectDir != dir {
		t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, dir)
	}
	if len(cfg.AutoStart) != 1 {
		t.Fatalf("expected 1 autostart item, got %d", len(cfg.AutoStart))
	}
	if cfg.AutoStart[0].Command != "make" {
		t.Errorf("command: got %q, want %q", cfg.AutoStart[0].Command, "make")
	}
	if len(cfg.AutoStart[0].Args) != 1 || cfg.AutoStart[0].Args[0] != "run" {
		t.Errorf("args: got %v, want [run]", cfg.AutoStart[0].Args)
	}
}

func TestFindProjectConfigWalksUp(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write config at root
	content := `
[[autostart]]
command = "echo"
args = ["found"]
`
	if err := os.WriteFile(filepath.Join(dir, ".termdesk.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Search from sub/deep
	cfg, err := FindProjectConfig(subdir)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.ProjectDir != dir {
		t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, dir)
	}
}

func TestParseProjectConfigDockItems(t *testing.T) {
	cfg := &ProjectConfig{ProjectDir: "/tmp"}
	parseProjectConfig(cfg, `
[[autostart]]
command = "echo"
args = ["hello"]
title = "Hello"

[[dock]]
name = "Notepad"
icon = "\uf15c"
icon_color = "#E5C07B"
command = "nano"

[[dock]]
name = "Files"
command = "mc"
args = ["-b"]
position = 1
`)

	if len(cfg.AutoStart) != 1 {
		t.Fatalf("expected 1 autostart item, got %d", len(cfg.AutoStart))
	}
	if len(cfg.DockItems) != 2 {
		t.Fatalf("expected 2 dock items, got %d", len(cfg.DockItems))
	}

	d := cfg.DockItems[0]
	if d.Name != "Notepad" {
		t.Errorf("dock[0] name: got %q, want %q", d.Name, "Notepad")
	}
	if d.Command != "nano" {
		t.Errorf("dock[0] command: got %q, want %q", d.Command, "nano")
	}
	if d.IconColor != "#E5C07B" {
		t.Errorf("dock[0] icon_color: got %q, want %q", d.IconColor, "#E5C07B")
	}
	if d.Position != 0 {
		t.Errorf("dock[0] position: got %d, want 0 (default)", d.Position)
	}

	d = cfg.DockItems[1]
	if d.Name != "Files" {
		t.Errorf("dock[1] name: got %q, want %q", d.Name, "Files")
	}
	if d.Command != "mc" {
		t.Errorf("dock[1] command: got %q, want %q", d.Command, "mc")
	}
	if len(d.Args) != 1 || d.Args[0] != "-b" {
		t.Errorf("dock[1] args: got %v, want [-b]", d.Args)
	}
	if d.Position != 1 {
		t.Errorf("dock[1] position: got %d, want 1", d.Position)
	}
}

func TestFindProjectConfigNotFound(t *testing.T) {
	dir := t.TempDir()

	cfg, err := FindProjectConfig(dir)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when no .termdesk.toml exists")
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

func TestParseStringArrayEmpty(t *testing.T) {
	result := parseStringArray("[]")
	if result != nil {
		t.Errorf("parseStringArray([]) should return nil, got %v", result)
	}
}
