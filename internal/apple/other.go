//go:build !darwin

package apple

import (
	"context"
	"errors"
)

func ReadSystem(context.Context) ([]Contact, error) {
	return nil, errors.New("apple contacts are only supported on macOS; use vCard import/export fallback on this platform")
}
