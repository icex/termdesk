package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "termdesk-config-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	if os.Getenv(envConfigPath) == "" {
		_ = os.Setenv(envConfigPath, filepath.Join(tmpDir, "config.toml"))
	}
	if os.Getenv(envConfigDir) == "" {
		_ = os.Setenv(envConfigDir, tmpDir)
	}

	os.Exit(m.Run())
}
