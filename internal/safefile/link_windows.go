//go:build windows

package safefile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// platformLinkUnsupported reports link errors that mean the filesystem cannot
// hard-link the temporary file. FAT and exFAT volumes reject hard links with
// ERROR_INVALID_FUNCTION, and some network shares report ERROR_ACCESS_DENIED
// or ERROR_NOT_SAME_DEVICE.
func platformLinkUnsupported(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_FUNCTION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
