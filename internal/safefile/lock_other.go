//go:build !windows

package safefile

import "os"

func createLock(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}
