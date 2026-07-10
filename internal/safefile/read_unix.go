//go:build unix

package safefile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Each component is opened relative to the preceding directory handle. This
// keeps the walk anchored even when an attacker renames a parent concurrently.
func openReadFile(root, path string) (*os.File, error) {
	dirFD, err := unix.Open(root, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: root, Err: err}
	}

	parts := strings.Split(path, string(filepath.Separator))
	for _, component := range parts[:len(parts)-1] {
		nextFD, openErr := unix.Openat(
			dirFD,
			component,
			unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK,
			0,
		)
		if openErr != nil {
			err = noFollowOpenError(dirFD, component, path, openErr)
			_ = unix.Close(dirFD)
			return nil, err
		}
		_ = unix.Close(dirFD)
		dirFD = nextFD
	}

	leaf := parts[len(parts)-1]
	fd, openErr := unix.Openat(
		dirFD,
		leaf,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK,
		0,
	)
	if openErr != nil {
		err = noFollowOpenError(dirFD, leaf, path, openErr)
		_ = unix.Close(dirFD)
		return nil, err
	}
	_ = unix.Close(dirFD)
	return os.NewFile(uintptr(fd), filepath.Join(root, path)), nil
}

func noFollowOpenError(dirFD int, component, path string, openErr error) error {
	var stat unix.Stat_t
	if err := unix.Fstatat(dirFD, component, &stat, unix.AT_SYMLINK_NOFOLLOW); err == nil &&
		stat.Mode&unix.S_IFMT == unix.S_IFLNK {
		return fmt.Errorf("path contains symbolic link: %s: %w", path, openErr)
	}
	return &os.PathError{Op: "openat", Path: path, Err: openErr}
}
