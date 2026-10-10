package fileutil

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAtomicConcurrentWritersAndSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	var wg sync.WaitGroup
	for _, c := range []byte{'a', 'b'} {
		wg.Add(1)
		go func(c byte) {
			defer wg.Done()
			if err := WriteAtomic(path, bytes.Repeat([]byte{c}, 1<<16)); err != nil {
				t.Error(err)
			}
		}(c)
	}
	wg.Wait()
	got, err := os.ReadFile(path)
	if err != nil || len(got) != 1<<16 || !bytes.Equal(got, bytes.Repeat(got[:1], len(got))) {
		t.Fatal("torn write")
	}
	victim := filepath.Join(dir, "victim")
	os.WriteFile(victim, []byte("keep"), 0600)
	os.Remove(path)
	os.Symlink(victim, path)
	WriteAtomic(path, []byte("replacement"))
	got, _ = os.ReadFile(victim)
	if string(got) != "keep" {
		t.Fatal("symlink target modified")
	}
	os.Symlink(victim, path+".lock")
	if WithLock(path, func() error { return nil }) == nil {
		t.Fatal("symlink lock accepted")
	}
}

func TestPrivateDirectoryRejectsSymlinkAndTightensPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	os.Mkdir(dir, 0755)
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0700 {
		t.Fatal("old store remains public")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	os.Symlink(dir, alias)
	if EnsurePrivateDir(alias) == nil {
		t.Fatal("private directory follows symlink")
	}
}
