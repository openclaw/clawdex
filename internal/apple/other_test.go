//go:build !darwin

package apple

import "testing"

func TestReadSystemUnsupported(t *testing.T) {
	_, err := ReadSystem(t.Context())
	if err == nil {
		t.Fatal("expected unsupported error")
	}
}
