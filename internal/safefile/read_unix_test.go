//go:build unix

package safefile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestReadFileRejectsFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "avatar"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ReadFile(root, "avatar")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("FIFO error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("rooted FIFO read blocked")
	}
}

func TestReadFileParentSwapCannotRedirect(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "selected")
	parked := filepath.Join(root, "selected-real")
	attacker := filepath.Join(root, "attacker")
	for _, dir := range []string{selected, attacker} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(selected, "avatar"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attacker, "avatar"), []byte("redirected"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, "symlink-probe")
	if err := os.Symlink("attacker", probe); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		for range 500 {
			if err := os.Rename(selected, parked); err != nil {
				done <- fmt.Errorf("park selected directory: %w", err)
				return
			}
			if err := os.Symlink("attacker", selected); err != nil {
				done <- fmt.Errorf("install redirect: %w", err)
				return
			}
			runtime.Gosched()
			if err := os.Remove(selected); err != nil {
				done <- fmt.Errorf("remove redirect: %w", err)
				return
			}
			if err := os.Rename(parked, selected); err != nil {
				done <- fmt.Errorf("restore selected directory: %w", err)
				return
			}
			runtime.Gosched()
		}
		done <- nil
	}()

	var redirected string
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			if redirected != "" {
				t.Fatalf("rooted read followed swapped parent and returned %q", redirected)
			}
			return
		default:
			data, err := ReadFile(root, filepath.Join("selected", "avatar"))
			if err == nil && string(data) != "safe" {
				redirected = string(data)
			}
		}
	}
}
