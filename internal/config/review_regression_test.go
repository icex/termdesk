package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestKeybindingsSurviveWidgetsAndHistory(t *testing.T) {
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(t.TempDir(), "config.toml"))
	c := DefaultUserConfig()
	c.Keys.Prefix = "ctrl+g"
	c.Keys.QuakeTerminal = "ctrl+t"
	c.CustomWidgets = []CustomWidgetDef{{Name: "test", Command: "printf ok"}}
	c.RecentWorkspaces = []WorkspaceHistoryEntry{{Path: "/tmp/example", LastAccess: time.Now()}}
	if err := SaveUserConfig(c); err != nil {
		t.Fatal(err)
	}
	got := LoadUserConfig()
	if got.Keys != c.Keys || len(got.CustomWidgets) != 1 || len(got.RecentWorkspaces) != 1 {
		t.Fatalf("roundtrip corrupted config: %+v", got)
	}
}
func TestConcurrentUpdatesPreserveIndependentFields(t *testing.T) {
	t.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(t.TempDir(), "config.toml"))
	var wg sync.WaitGroup
	updates := []func(*UserConfig){func(c *UserConfig) { c.Keys.Prefix = "ctrl+g" }, func(c *UserConfig) { c.Favorites = []string{"nvim"} }, func(c *UserConfig) { c.TourCompleted = true }, func(c *UserConfig) { c.RecentApps = []string{"yazi"} }}
	for _, update := range updates {
		wg.Add(1)
		go func(fn func(*UserConfig)) {
			defer wg.Done()
			if err := UpdateUserConfig(fn); err != nil {
				t.Error(err)
			}
		}(update)
	}
	wg.Wait()
	got := LoadUserConfig()
	if got.Keys.Prefix != "ctrl+g" || !got.TourCompleted || len(got.Favorites) != 1 || len(got.RecentApps) != 1 {
		t.Fatalf("concurrent settings lost: %+v", got)
	}
}

func TestUpdateRefusesToOverwriteUnreadableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("TERMDESK_CONFIG_PATH", path)
	c := DefaultUserConfig()
	c.Keys.Prefix = "ctrl+g"
	if err := SaveUserConfig(c); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() == 0 {
		t.Skip("root can read mode-0 files")
	}
	if err := UpdateUserConfig(func(c *UserConfig) { c.IconsOnly = true }); err == nil {
		t.Fatal("update succeeded on unreadable config")
	}
	os.Chmod(path, 0o600)
	if got := LoadUserConfig(); got.Keys.Prefix != "ctrl+g" {
		t.Fatalf("settings were overwritten: prefix=%q", got.Keys.Prefix)
	}
}
