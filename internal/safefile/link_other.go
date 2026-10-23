//go:build !windows

package safefile

import (
	"errors"
	"syscall"
)

// platformLinkUnsupported reports link errors that mean the filesystem cannot
// hard-link the temporary file. Linux returns EPERM for filesystems such as
// FAT and exFAT that lack hard links, and some FUSE and network mounts return
// EXDEV even within one directory.
func platformLinkUnsupported(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EXDEV)
}
