package safefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileRejectsSymlinkLeafAndParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "safe"), 0o755); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "safe", "leaf")
	if err := os.Symlink(secret, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ReadFile(root, filepath.Join("safe", "leaf")); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("leaf error = %v", err)
	}
	if _, err := ExistingPath(root, filepath.Join("safe", "leaf")); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("existing leaf error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "parent")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, filepath.Join("parent", "secret")); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("parent error = %v", err)
	}
}

func TestReadPathRejectsSymlinkLeafAndParent(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "avatar.png")
	if err := os.WriteFile(realPath, []byte("avatar"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadPath(realPath); err != nil || string(data) != "avatar" {
		t.Fatalf("real data = %q err=%v", data, err)
	}

	leaf := filepath.Join(root, "leaf.png")
	if err := os.Symlink(realPath, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ReadPath(leaf); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("leaf error = %v", err)
	}
	parent := filepath.Join(root, "parent")
	if err := os.Symlink(realDir, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPath(filepath.Join(parent, "avatar.png")); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("parent error = %v", err)
	}
}

func TestAtomicWriteFileIsPrivateAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := AtomicWriteFile(root, filepath.Join("nested", "value"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "nested", "value")
	if data, err := os.ReadFile(path); err != nil || string(data) != "first" {
		t.Fatalf("data = %q err=%v", data, err)
	}
	if data, err := ReadFile(root, filepath.Join("nested", "value")); err != nil || string(data) != "first" {
		t.Fatalf("rooted data = %q err=%v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := AtomicWriteFile(root, filepath.Join("nested", "value"), []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "second" {
		t.Fatalf("replacement data = %q err=%v", data, err)
	}

	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside")
	if err := os.WriteFile(outsideFile, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "leaf")
	if err := os.Symlink(outsideFile, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := AtomicWriteFile(root, "leaf", []byte("overwrite"), 0o600); err == nil {
		t.Fatal("expected leaf symlink rejection")
	}
	parent := filepath.Join(root, "parent")
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(root, filepath.Join("parent", "outside"), []byte("overwrite"), 0o600); err == nil {
		t.Fatal("expected parent symlink rejection")
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "untouched" {
		t.Fatalf("outside data = %q err=%v", data, err)
	}
}

func TestAtomicWriteStreamsAndPreservesDestinationOnError(t *testing.T) {
	root := t.TempDir()
	if err := AtomicWriteFile(root, "value", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("render failed")
	err := AtomicWrite(root, "value", 0o600, func(w io.Writer) error {
		if _, err := io.WriteString(w, "partial replacement"); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "value"))
	if err != nil || string(data) != "original" {
		t.Fatalf("data = %q err=%v", data, err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".value.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v err=%v", matches, err)
	}
	if err := AtomicWrite(root, "value", 0o600, nil); err == nil {
		t.Fatal("expected nil callback error")
	}
}

func TestRootedMissingAndDestinationTypeErrors(t *testing.T) {
	root := t.TempDir()
	missingRoot := filepath.Join(root, "missing-root")
	if _, err := ReadFile(missingRoot, "file"); err == nil {
		t.Fatal("expected missing read root error")
	}
	if _, err := ExistingPath(missingRoot, "file"); err == nil {
		t.Fatal("expected missing existing-path root error")
	}
	if err := AtomicWriteFile(missingRoot, "file", nil, 0o600); err == nil {
		t.Fatal("expected missing write root error")
	}
	if _, err := ReadFile(root, "missing"); err == nil {
		t.Fatal("expected missing read error")
	}
	if _, err := ExistingPath(root, "missing"); err == nil {
		t.Fatal("expected missing existing-path error")
	}

	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "directory"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory read error = %v", err)
	}
	if err := AtomicWriteFile(root, "directory", nil, 0o600); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory destination error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "parent-file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(root, filepath.Join("parent-file", "child"), nil, 0o600); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file parent error = %v", err)
	}
	if err := ValidateWrite(root, filepath.Join("missing-parent", "child")); err == nil {
		t.Fatal("expected missing parent validation error")
	}
	if err := ValidateWrite(root, "missing-leaf"); err != nil {
		t.Fatalf("missing leaf validation = %v", err)
	}
	if err := ValidateWrite(root, "directory"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory validation error = %v", err)
	}
}

func TestRootAPIsRequireOpenRoot(t *testing.T) {
	if err := ValidateWriteRoot(nil, "value"); err == nil || !strings.Contains(err.Error(), "open root") {
		t.Fatalf("validate nil root error = %v", err)
	}
	if err := AtomicWriteRoot(nil, "value", 0o600, func(io.Writer) error { return nil }); err == nil || !strings.Contains(err.Error(), "open root") {
		t.Fatalf("write nil root error = %v", err)
	}
}

func TestValidateAtomicWriteAllowsMissingParentsAndRejectsUnsafeExistingPaths(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join("missing", "avatars", "avatar.png")
	if err := ValidateAtomicWrite(root, missing); err != nil {
		t.Fatalf("missing safe parents: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
		t.Fatalf("validation created a directory: %v", err)
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := ValidateAtomicWrite(root, filepath.Join("linked", "avatar.png")); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("linked parent error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAtomicWrite(root, filepath.Join("file", "avatar.png")); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file parent error = %v", err)
	}
}

func TestDirectoryComponentChecksRejectSymlinkAndFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("directory", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if err := checkDirectoryComponents(r, "."); err != nil {
		t.Fatal(err)
	}
	if err := checkDirectoryComponents(r, "directory"); err != nil {
		t.Fatal(err)
	}
	if err := checkDirectoryComponents(r, "link"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("symlink component error = %v", err)
	}
	if err := checkDirectoryComponents(r, "file"); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file component error = %v", err)
	}
}

func TestPathValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := Relative(root, path)
	if err != nil || rel != "file" {
		t.Fatalf("relative = %q err=%v", rel, err)
	}
	got, err := ExistingPath(root, rel)
	if err != nil || got != path {
		t.Fatalf("existing path = %q err=%v", got, err)
	}
	for _, bad := range []string{"", ".", "..", filepath.Join("..", "outside"), path} {
		if _, err := ReadFile(root, bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	if _, err := Relative(root, filepath.Join(filepath.Dir(root), "outside")); err == nil {
		t.Fatal("expected escaped relative error")
	}
	if err := AtomicWriteFile(root, "bad-mode", nil, os.ModeDir|0o600); err == nil {
		t.Fatal("expected bad mode error")
	}
}
