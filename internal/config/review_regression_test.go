package config

import (
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
