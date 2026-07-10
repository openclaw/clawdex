package safefile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Relative returns target as a path beneath root. It rejects lexical escapes;
// filesystem accessors below additionally reject symbolic-link components.
func Relative(root, target string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return "", fmt.Errorf("path escaped root %s: %w", root, err)
	}
	return clean, nil
}

// ExistingPath returns the absolute path after verifying that every component
// beneath root exists and is not a symbolic link.
func ExistingPath(root, relative string) (string, error) {
	clean, err := cleanRelative(relative)
	if err != nil {
		return "", err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	if err := checkNoSymlinks(r, clean); err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootAbs, clean), nil
}

// ReadFile reads a regular rooted path without following symbolic links in any
// component. The platform opener anchors the root and rejects links while the
// path is resolved, so concurrent parent swaps cannot redirect the read.
func ReadFile(root, relative string) ([]byte, error) {
	clean, err := cleanRelative(relative)
	if err != nil {
		return nil, err
	}
	f, err := openReadFile(root, clean)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file: %s", clean)
	}
	return io.ReadAll(f)
}

// ReadPath reads an ordinary user-selected file without following symbolic
// links beneath a trusted process directory. The current directory, home, and
// temporary directory are treated as anchors so stable platform aliases such
// as macOS /var can be resolved once without permitting symlinks below them.
func ReadPath(path string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, relative, err := readPathRoot(abs)
	if err != nil {
		return nil, err
	}
	return ReadFile(root, relative)
}

func readPathRoot(abs string) (string, string, error) {
	volume := filepath.VolumeName(abs)
	root := volume + string(filepath.Separator)
	if volume == "" {
		root = string(filepath.Separator)
	}
	anchor := root
	for _, candidate := range readPathAnchors() {
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(candidateAbs, abs)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if len(candidateAbs) > len(anchor) {
			anchor = candidateAbs
		}
	}
	relative, err := filepath.Rel(anchor, abs)
	if err != nil {
		return "", "", err
	}
	resolved, err := filepath.EvalSymlinks(anchor)
	if err != nil {
		return "", "", err
	}
	return resolved, relative, nil
}

func readPathAnchors() []string {
	anchors := make([]string, 0, 3)
	if cwd, err := os.Getwd(); err == nil {
		anchors = append(anchors, cwd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		anchors = append(anchors, home)
	}
	if temporary := os.TempDir(); temporary != "" {
		anchors = append(anchors, temporary)
	}
	return anchors
}

// AtomicWriteFile writes through a private temporary file in the destination
// directory, then replaces the destination without following a symlink leaf.
func AtomicWriteFile(root, relative string, data []byte, perm os.FileMode) error {
	return AtomicWrite(root, relative, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// ValidateWrite verifies an existing destination path without creating or
// replacing anything. A missing leaf is allowed; every parent must already be
// a real directory beneath root, and an existing leaf must be regular.
func ValidateWrite(root, relative string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	return ValidateWriteRoot(r, relative)
}

// ValidateWriteRoot applies ValidateWrite to an already-open root. The caller
// retains ownership of root, which keeps a selected directory anchored across
// symbolic-link renames.
func ValidateWriteRoot(root *os.Root, relative string) error {
	if root == nil {
		return errors.New("open root is required")
	}
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	if err := checkDirectoryComponents(root, filepath.Dir(clean)); err != nil {
		return err
	}
	return rejectSymlinkLeaf(root, clean)
}

// ValidateAtomicWrite applies AtomicWrite's destination safety checks without
// creating directories or files. Missing directory suffixes are allowed;
// existing components and the leaf must have safe types.
func ValidateAtomicWrite(root, relative string) error {
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	parentsExist, err := validateDirectoryPlan(r, filepath.Dir(clean))
	if err != nil || !parentsExist {
		return err
	}
	return rejectSymlinkLeaf(r, clean)
}

// AtomicWrite streams into a private temporary file in the destination
// directory, then replaces the destination only after the callback succeeds.
func AtomicWrite(root, relative string, perm os.FileMode, write func(io.Writer) error) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	return AtomicWriteRoot(r, relative, perm, write)
}

// AtomicWriteRoot applies AtomicWrite to an already-open root. The caller
// retains ownership of root.
func AtomicWriteRoot(root *os.Root, relative string, perm os.FileMode, write func(io.Writer) error) error {
	if root == nil {
		return errors.New("open root is required")
	}
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	if perm != perm.Perm() {
		return fmt.Errorf("invalid file mode: %v", perm)
	}
	if write == nil {
		return errors.New("write callback is required")
	}

	parent := filepath.Dir(clean)
	if err := ensureDirectories(root, parent, 0o755); err != nil {
		return err
	}
	if err := rejectSymlinkLeaf(root, clean); err != nil {
		return err
	}

	tmp, f, err := createTemp(root, parent, filepath.Base(clean))
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = root.Remove(tmp)
		}
	}()
	if err := write(f); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Recheck immediately before rename. A concurrent swap still cannot leave
	// the os.Root, and Rename replaces rather than follows a leaf symlink.
	if err := checkDirectoryComponents(root, parent); err != nil {
		return err
	}
	if err := rejectSymlinkLeaf(root, clean); err != nil {
		return err
	}
	// Root.Rename uses an in-root renameat operation. On Windows in Go 1.26,
	// this is NtSetInformationFileEx with REPLACE_IF_EXISTS/POSIX semantics,
	// not the non-replacing os.Rename implementation. The temporary and target
	// are in the same directory, so supported release filesystems get one
	// atomic replacement operation on every shipped platform.
	if err := root.Rename(tmp, clean); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func cleanRelative(path string) (string, error) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("path must be relative: %s", path)
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay beneath root: %s", path)
	}
	return clean, nil
}

func checkNoSymlinks(root *os.Root, path string) error {
	for _, prefix := range prefixes(path) {
		info, err := root.Lstat(prefix)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains symbolic link: %s", prefix)
		}
	}
	return nil
}

func checkDirectoryComponents(root *os.Root, path string) error {
	if path == "." {
		return nil
	}
	for _, prefix := range prefixes(path) {
		info, err := root.Lstat(prefix)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path contains symbolic link: %s", prefix)
		}
		if !info.IsDir() {
			return fmt.Errorf("path component is not a directory: %s", prefix)
		}
	}
	return nil
}

func ensureDirectories(root *os.Root, path string, perm os.FileMode) error {
	if path == "." {
		return nil
	}
	for _, prefix := range prefixes(path) {
		info, err := root.Lstat(prefix)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("path contains symbolic link: %s", prefix)
			}
			if !info.IsDir() {
				return fmt.Errorf("path component is not a directory: %s", prefix)
			}
		case errors.Is(err, os.ErrNotExist):
			if err := root.Mkdir(prefix, perm); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			info, err := root.Lstat(prefix)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("created path component is not a directory: %s", prefix)
			}
		default:
			return err
		}
	}
	return nil
}

func validateDirectoryPlan(root *os.Root, path string) (bool, error) {
	if path == "." {
		return true, nil
	}
	for _, prefix := range prefixes(path) {
		info, err := root.Lstat(prefix)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				return false, fmt.Errorf("path contains symbolic link: %s", prefix)
			}
			if !info.IsDir() {
				return false, fmt.Errorf("path component is not a directory: %s", prefix)
			}
		case errors.Is(err, os.ErrNotExist):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func rejectSymlinkLeaf(root *os.Root, path string) error {
	info, err := root.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination is a symbolic link: %s", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("destination is not a regular file: %s", path)
		}
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return err
	}
}

func createTemp(root *os.Root, parent, base string) (string, *os.File, error) {
	for range 100 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := "." + base + ".tmp-" + hex.EncodeToString(random[:])
		path := name
		if parent != "." {
			path = filepath.Join(parent, name)
		}
		f, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return path, f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
	}
	return "", nil, errors.New("could not allocate temporary file")
}

func prefixes(path string) []string {
	parts := strings.Split(path, string(filepath.Separator))
	out := make([]string, 0, len(parts))
	current := ""
	for _, part := range parts {
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		out = append(out, current)
	}
	return out
}
