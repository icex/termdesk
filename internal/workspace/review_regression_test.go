package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/icex/termdesk/internal/trust"
)

func TestConcurrentSavesNewestSnapshotWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMDESK_HOME", t.TempDir())
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(t.TempDir(), "config.toml"))
	states := []WorkspaceState{{CapturedAt: time.Now().Add(-time.Second), FocusedID: "old"}, {CapturedAt: time.Now(), FocusedID: "new"}}
	var wg sync.WaitGroup
	for _, state := range states {
		wg.Add(1)
		go func(s WorkspaceState) {
			defer wg.Done()
			if err := SaveWorkspace(s, dir); err != nil {
				t.Error(err)
			}
		}(state)
	}
	wg.Wait()
	got, err := LoadWorkspace(dir)
	if err != nil || got.FocusedID != "new" {
		t.Fatalf("stale save won: %+v %v", got, err)
	}
	if !got.Trusted {
		t.Fatal("generated workspace not trusted")
	}
}
func TestSavedSecretsArePrivateAndBuffersStayIndependent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TERMDESK_HOME", home)
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(home, "config.toml"))
	other := SaveBuffer("other", "other workspace secret")
	state := WorkspaceState{Windows: []WindowState{{ID: "current", Width: 80, Height: 24, BufferContent: "current secret"}}, Clipboard: []string{"clipboard secret"}}
	if err := SaveWorkspace(state, ""); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{GetWorkspacePath(""), filepath.Join(buffersDir(), bufferPrefix(GetWorkspacePath(""))+"current.buf.gz")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&077 != 0 {
			t.Fatalf("not private: %s %v", path, err)
		}
	}
	if LoadBuffer(other) != "other workspace secret" {
		t.Fatal("another workspace buffer lost")
	}
	got, err := LoadWorkspace("")
	if err != nil || LoadBuffer(got.Windows[0].BufferFile) != "current secret" {
		t.Fatal("saved buffer missing")
	}
}
func TestUntrustedAndMalformedWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TERMDESK_HOME", home)
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(home, "config.toml"))
	dir := t.TempDir()
	state := WorkspaceState{Version: 1, SavedAt: time.Now(), Windows: []WindowState{{ID: "w", Width: 80, Height: 24, SplitTree: "S:H:0.5 L:a", Panes: []PaneState{{TermID: "a"}}}}}
	path := GetWorkspacePath(dir)
	os.WriteFile(path, []byte(serializeWorkspace(state)), 0600)
	if _, err := LoadWorkspace(dir); err == nil {
		t.Fatal("incomplete split accepted")
	}
	state.Windows = nil
	os.WriteFile(path, []byte(serializeWorkspace(state)), 0600)
	got, err := LoadWorkspace(dir)
	if err != nil || got.Trusted {
		t.Fatal("unapproved file trusted")
	}
	if err := trust.Record(path); err != nil {
		t.Fatal(err)
	}
	got, err = LoadWorkspace(dir)
	if err != nil || !got.Trusted {
		t.Fatal("approved file rejected")
	}
	data, _ := os.ReadFile(path)
	os.WriteFile(path, append(data, []byte("# changed\n")...), 0600)
	got, _ = LoadWorkspace(dir)
	if got.Trusted {
		t.Fatal("changed contents inherited trust")
	}
}
func TestBufferPathTraversalRejected(t *testing.T) {
	t.Setenv("TERMDESK_HOME", t.TempDir())
	if SaveBuffer("../escape", "secret") != "" || LoadBuffer("../escape") != "" {
		t.Fatal("unsafe buffer ID accepted")
	}
}
func TestHugePaneCountIsBounded(t *testing.T) {
	var state WorkspaceState
	parseWorkspace(&state, "[[window]]\npane_count = 1000000000\n")
	if len(state.Windows) != 1 || len(state.Windows[0].Panes) > 64 {
		t.Fatal("unbounded allocation")
	}
}
func TestBlockedSavePreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := GetWorkspacePath(dir)
	os.WriteFile(path, []byte("original"), 0600)
	if SaveWorkspace(WorkspaceState{SaveBlocked: true}, dir) == nil {
		t.Fatal("save was allowed")
	}
	data, _ := os.ReadFile(path)
	if strings.TrimSpace(string(data)) != "original" {
		t.Fatal("original overwritten")
	}
}

func TestScopedCleanupKeepsOtherSavedWorkspaces(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TERMDESK_HOME", home)
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(home, "config.toml"))
	a, b := t.TempDir(), t.TempDir()
	state := WorkspaceState{Windows: []WindowState{{ID: "same-id", Width: 80, Height: 24, BufferContent: "workspace A"}}}
	if err := SaveWorkspace(state, a); err != nil {
		t.Fatal(err)
	}
	first, _ := LoadWorkspace(a)
	state.Windows[0].BufferContent = "workspace B"
	if err := SaveWorkspace(state, b); err != nil {
		t.Fatal(err)
	}
	second, _ := LoadWorkspace(b)
	if first.Windows[0].BufferFile == second.Windows[0].BufferFile {
		t.Fatal("workspace buffer names collide")
	}
	if err := SaveWorkspace(WorkspaceState{}, b); err != nil {
		t.Fatal(err)
	}
	if LoadBuffer(first.Windows[0].BufferFile) != "workspace A" {
		t.Fatal("another saved workspace lost its buffer")
	}
	if LoadBuffer(second.Windows[0].BufferFile) != "" {
		t.Fatal("own stale buffer was not cleaned up")
	}
}
