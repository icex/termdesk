package appmanifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowInfo(t *testing.T) {
	w := WindowInfo{}
	if !w.IsResizable() {
		t.Fatalf("default resizable should be true")
	}
	if w.IsFixedSize() {
		t.Fatalf("empty window should not be fixed")
	}
	f := false
	w.Resizable = &f
	if w.IsResizable() {
		t.Fatalf("resizable override failed")
	}
	w.Width = 80
	w.Height = 25
	if !w.IsFixedSize() {
		t.Fatalf("fixed size expected")
	}
}

func TestWindowInfoPartialFixed(t *testing.T) {
	// Only width set — not fixed size
	w := WindowInfo{Width: 80}
	if w.IsFixedSize() {
		t.Fatalf("width-only should not be fixed")
	}
	// Only height set — not fixed size
	w2 := WindowInfo{Height: 25}
	if w2.IsFixedSize() {
		t.Fatalf("height-only should not be fixed")
	}
}

func TestWindowInfoResizableTrue(t *testing.T) {
	tr := true
	w := WindowInfo{Resizable: &tr}
	if !w.IsResizable() {
		t.Fatalf("explicit true should be resizable")
	}
}

func TestLoadManifestAndUnescape(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "termdesk-calc")
	_ = os.WriteFile(bin, []byte(""), 0o600)

	data := `# sample
[app]
name = "Calc"
icon = "\uF1EC"
icon_color = "#ff00ff"

[window]
width = 80
height = 24
resizable = false
maximized = true
`
	if err := os.WriteFile(bin+".toml", []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	m := LoadManifest(bin)
	if m == nil {
		t.Fatalf("manifest not loaded")
	}
	if m.App.Name != "Calc" {
		t.Fatalf("name=%q", m.App.Name)
	}
	if m.App.Icon != "\uf1ec" {
		t.Fatalf("icon=%q", m.App.Icon)
	}
	if m.App.IconColor != "#ff00ff" {
		t.Fatalf("icon_color=%q", m.App.IconColor)
	}
	if m.Window.Width != 80 || m.Window.Height != 24 || m.Window.IsResizable() {
		t.Fatalf("window prefs=%+v", m.Window)
	}
	if !m.Window.Maximized {
		t.Fatalf("maximized expected")
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	m := LoadManifest("/nonexistent/path/no-such-binary")
	if m != nil {
		t.Fatalf("expected nil for missing manifest")
	}
}

func TestLoadManifestsInDir(t *testing.T) {
	dir := t.TempDir()
	data := `[app]
icon = "\uF1EC"
`
	if err := os.WriteFile(filepath.Join(dir, "termdesk-calc.toml"), []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	m := LoadManifestsInDir(dir)
	if m == nil || m["termdesk-calc"] == nil {
		t.Fatalf("manifest map missing entry")
	}
	if m["termdesk-calc"].App.Name != "termdesk-calc" {
		t.Fatalf("default name=%q", m["termdesk-calc"].App.Name)
	}
}

func TestLoadManifestsInDirNonexistent(t *testing.T) {
	m := LoadManifestsInDir("/nonexistent/dir/that/does/not/exist")
	if m != nil {
		t.Fatalf("expected nil for nonexistent dir, got %v", m)
	}
}

func TestLoadManifestsInDirSkipsNonTOML(t *testing.T) {
	dir := t.TempDir()
	// Write a non-TOML file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Write a valid TOML file
	if err := os.WriteFile(filepath.Join(dir, "myapp.toml"), []byte("[app]\nname = \"MyApp\"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	m := LoadManifestsInDir(dir)
	if m == nil {
		t.Fatalf("expected non-nil map")
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m))
	}
	if m["myapp"] == nil {
		t.Fatalf("missing myapp entry")
	}
	if m["myapp"].App.Name != "MyApp" {
		t.Fatalf("name=%q", m["myapp"].App.Name)
	}
}

func TestLoadManifestsInDirDefaultName(t *testing.T) {
	dir := t.TempDir()
	// Manifest with no name field — should default to filename stem
	data := `[window]
width = 60
`
	if err := os.WriteFile(filepath.Join(dir, "foo.toml"), []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	m := LoadManifestsInDir(dir)
	if m["foo"] == nil {
		t.Fatalf("missing foo entry")
	}
	if m["foo"].App.Name != "foo" {
		t.Fatalf("expected default name 'foo', got %q", m["foo"].App.Name)
	}
}

func TestUnescapeIconNoEscape(t *testing.T) {
	// No \u sequence — should return as-is
	got := unescapeIcon("hello")
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestUnescapeIconInvalidHex(t *testing.T) {
	// \u followed by invalid hex — should pass through literally
	got := unescapeIcon(`\uZZZZ`)
	if got != `\uZZZZ` {
		t.Fatalf("expected literal pass-through, got %q", got)
	}
}

func TestUnescapeIconTruncated(t *testing.T) {
	// \u at end of string — not enough chars for 4-digit hex
	got := unescapeIcon(`\u00`)
	if got != `\u00` {
		t.Fatalf("expected literal pass-through for truncated, got %q", got)
	}
}

func TestUnescapeIconMixed(t *testing.T) {
	// Valid escape followed by plain text
	got := unescapeIcon(`\u0041BC`)
	if got != "ABC" {
		t.Fatalf("expected 'ABC', got %q", got)
	}
}

func TestParseManifestResizableTrue(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[window]
resizable = true
`)
	if m.Window.Resizable == nil {
		t.Fatalf("resizable should be set")
	}
	if !*m.Window.Resizable {
		t.Fatalf("resizable should be true")
	}
}

func TestParseManifestMaximizedFalse(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[window]
maximized = false
`)
	if m.Window.Maximized {
		t.Fatalf("maximized should be false")
	}
}

func TestParseManifestInvalidNumbers(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[window]
width = abc
height = xyz
`)
	if m.Window.Width != 0 {
		t.Fatalf("width should be 0 for invalid number, got %d", m.Window.Width)
	}
	if m.Window.Height != 0 {
		t.Fatalf("height should be 0 for invalid number, got %d", m.Window.Height)
	}
}

func TestParseManifestMalformedLines(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[app]
this line has no equals sign
name = "Valid"
`)
	if m.App.Name != "Valid" {
		t.Fatalf("expected 'Valid', got %q", m.App.Name)
	}
}

func TestParseManifestEmptyLinesAndComments(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `
# This is a comment
[app]
# Another comment
name = "Test"

`)
	if m.App.Name != "Test" {
		t.Fatalf("expected 'Test', got %q", m.App.Name)
	}
}

func TestParseManifestCategory(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[app]
name = "Snake"
category = "games"
`)
	if m.App.Category != "games" {
		t.Fatalf("expected 'games', got %q", m.App.Category)
	}
}

func TestParseManifestUnknownSection(t *testing.T) {
	m := &AppManifest{}
	parseManifest(m, `[unknown]
foo = "bar"
[app]
name = "Valid"
`)
	if m.App.Name != "Valid" {
		t.Fatalf("expected 'Valid', got %q", m.App.Name)
	}
}
