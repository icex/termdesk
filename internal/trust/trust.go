// Package trust records explicit consent to execute commands from a file.
// Consent is bound to its canonical path and contents, not just a directory.
package trust

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/icex/termdesk/internal/fileutil"
)

func recordPath(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	if h := os.Getenv("TERMDESK_HOME"); h != "" {
		home = h
	}
	dir := filepath.Join(home, ".config", "termdesk")
	if d := os.Getenv("TERMDESK_CONFIG_DIR"); d != "" {
		dir = d
	}
	if p := os.Getenv("TERMDESK_CONFIG_PATH"); p != "" {
		dir = filepath.Dir(p)
	}
	if home == "" && dir == ".config/termdesk" {
		return "", fmt.Errorf("cannot locate trust store")
	}
	key := sha256.Sum256([]byte(real))
	return filepath.Join(dir, "trust", fmt.Sprintf("%x", key)), nil
}
func Record(path string) error {
	data, err := fileutil.ReadBounded(path, 10<<20)
	if err != nil {
		return err
	}
	return RecordData(path, data)
}

// RecordData approves exactly the bytes the caller generated or inspected.
func RecordData(path string, data []byte) error {
	record, err := recordPath(path)
	if err != nil {
		return err
	}
	if err := fileutil.EnsurePrivateDir(filepath.Dir(record)); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	return fileutil.WriteAtomic(record, []byte(fmt.Sprintf("%x", sum)))
}
func IsTrusted(path string) bool {
	data, err := fileutil.ReadBounded(path, 10<<20)
	return err == nil && IsTrustedData(path, data)
}
func IsTrustedData(path string, data []byte) bool {
	record, err := recordPath(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	approved, err := os.ReadFile(record)
	return err == nil && strings.TrimSpace(string(approved)) == fmt.Sprintf("%x", sum)
}
