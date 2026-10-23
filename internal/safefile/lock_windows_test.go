//go:build windows

package safefile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCreateLockPendingDeletionIsCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".leaf.create-lock")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if err := windows.DeleteFile(name); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	file, err := createLock(root, filepath.Base(path))
	if file != nil {
		_ = file.Close()
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("create pending-delete lock: %v, want os.ErrExist", err)
	}
}
