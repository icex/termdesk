package config

import (
	"os"
	"path/filepath"
	"strings"
)

// UserConfig holds persistent user settings.
type UserConfig struct {
	Theme string `toml:"theme"`
}

// DefaultUserConfig returns the default user configuration.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Theme: "modern",
	}
}

// configDir returns the path to ~/.config/termdesk/.
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "termdesk")
}

// configPath returns the path to the config file.
func configPath() string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// LoadUserConfig reads the user config from ~/.config/termdesk/config.toml.
// Returns default config if the file doesn't exist.
func LoadUserConfig() UserConfig {
	cfg := DefaultUserConfig()
	path := configPath()
	if path == "" {
		return cfg
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	// Simple TOML parser for key = "value" format
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"")
		switch key {
		case "theme":
			cfg.Theme = val
		}
	}

	return cfg
}

// SaveUserConfig writes the user config to ~/.config/termdesk/config.toml.
func SaveUserConfig(cfg UserConfig) error {
	dir := configDir()
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Termdesk configuration\n\n")
	sb.WriteString("theme = \"")
	sb.WriteString(cfg.Theme)
	sb.WriteString("\"\n")

	return os.WriteFile(configPath(), []byte(sb.String()), 0o644)
}
