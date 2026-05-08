//go:build darwin

package apple

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed contacts_export.swift
var contactsExportSwift string

func ReadSystem(ctx context.Context) ([]Contact, error) {
	dir, err := os.MkdirTemp("", "clawdex-contacts-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	script := filepath.Join(dir, "contacts-export.swift")
	if err := os.WriteFile(script, []byte(contactsExportSwift), 0o600); err != nil {
		return nil, err
	}
	out, err := runSwiftContacts(ctx, script)
	if err != nil {
		return nil, err
	}
	return Decode(bytes.NewReader(out))
}

var runSwiftContacts = func(ctx context.Context, script string) ([]byte, error) {
	cmd := swiftCommand(ctx, script)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("read macOS Contacts: %s", string(ee.Stderr))
		}
		return nil, fmt.Errorf("run swift Contacts helper: %w", err)
	}
	return out, nil
}

func swiftCommand(ctx context.Context, script string) *exec.Cmd {
	if path, err := exec.LookPath("swift"); err == nil {
		// #nosec G204 -- swift is resolved from PATH and the generated script is passed as an argument.
		return exec.CommandContext(ctx, path, script)
	}
	// #nosec G204 -- xcrun is fixed and the generated script is passed as an argument.
	return exec.CommandContext(ctx, "xcrun", "swift", script)
}
