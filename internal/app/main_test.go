package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "termdesk-app-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	if os.Getenv("TERMDESK_HOME") == "" {
		_ = os.Setenv("TERMDESK_HOME", tmpDir)
	}
	if os.Getenv("TERMDESK_CONFIG_PATH") == "" {
		_ = os.Setenv("TERMDESK_CONFIG_PATH", filepath.Join(tmpDir, "config.toml"))
	}

	os.Exit(m.Run())
}
