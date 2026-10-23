//go:build windows

package safefile

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func createLock(root *os.Root, name string) (*os.File, error) {
	parent, err := root.Open(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	objectName, err := windows.NewNTUnicodeString(filepath.Base(name))
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(parent.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_DONT_REPARSE,
	}
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_WRITE,
		attributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_CREATE,
		windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_NON_DIRECTORY_FILE,
		0,
		0,
	)
	if err != nil {
		// Go maps DELETE_PENDING to access denied. Preserve the NT status so
		// a retiring lock is a collision without masking real ACL failures.
		if errors.Is(err, windows.STATUS_DELETE_PENDING) {
			err = os.ErrExist
		} else if nt, ok := err.(windows.NTStatus); ok {
			err = nt.Errno()
		}
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), name), nil
}
