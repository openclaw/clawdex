//go:build !windows

package safefile

import (
	"os"
	"syscall"
	"testing"
)

func TestLinkUnsupportedPlatformErrors(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EPERM, syscall.EXDEV, syscall.EOPNOTSUPP, syscall.ENOSYS} {
		if !linkUnsupported(&os.LinkError{Op: "linkat", Old: "a", New: "b", Err: errno}) {
			t.Errorf("linkUnsupported(%v) = false, want true", errno)
		}
	}
}
