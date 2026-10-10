package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "termdesk-workspace-test-")
	if err != nil {
		panic(err)
	}
	if os.Getenv("TERMDESK_CONFIG_PATH") == "" {
		os.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(dir, "config.toml"))
	}
	// Existing HOME-path tests deliberately vary HOME; isolate its initial value.
	os.Setenv("HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
