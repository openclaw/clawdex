//go:build darwin

package apple

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReadSystemUsesSwiftRunner(t *testing.T) {
	orig := runSwiftContacts
	t.Cleanup(func() { runSwiftContacts = orig })
	runSwiftContacts = func(ctx context.Context, script string) ([]byte, error) {
		if !strings.HasSuffix(script, "contacts-export.swift") {
			t.Fatalf("script = %s", script)
		}
		return []byte(`{"identifier":"a1","full_name":"Ada","emails":["ada@example.com"]}` + "\n"), nil
	}
	contacts, err := ReadSystem(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Name() != "Ada" {
		t.Fatalf("contacts = %#v", contacts)
	}
}

func TestReadSystemPropagatesSwiftError(t *testing.T) {
	orig := runSwiftContacts
	t.Cleanup(func() { runSwiftContacts = orig })
	runSwiftContacts = func(context.Context, string) ([]byte, error) {
		return nil, errors.New("denied")
	}
	if _, err := ReadSystem(t.Context()); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("err = %v", err)
	}
}

func TestSwiftCommand(t *testing.T) {
	cmd := swiftCommand(t.Context(), "/tmp/test.swift")
	if cmd == nil || len(cmd.Args) == 0 {
		t.Fatalf("cmd = %#v", cmd)
	}
}
