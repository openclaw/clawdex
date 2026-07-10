//go:build !unix && !windows

package safefile

import (
	"errors"
	"os"
)

func openReadFile(_, _ string) (*os.File, error) {
	return nil, errors.New("secure rooted reads are unsupported on this platform")
}
