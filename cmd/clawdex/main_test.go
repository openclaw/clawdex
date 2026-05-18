package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	var out, errOut bytes.Buffer
	cfg := filepath.Join(t.TempDir(), "config.toml")
	code := run([]string{"--config", cfg, "config"}, &out, &errOut)
	if code != 0 || strings.Contains(out.String(), "steipete") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	code = run([]string{"--bogus"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestMainEntrypoint(t *testing.T) {
	oldExit := exit
	oldArgs := os.Args
	defer func() {
		exit = oldExit
		os.Args = oldArgs
	}()
	code := -1
	exit = func(value int) {
		code = value
	}
	cfg := filepath.Join(t.TempDir(), "config.toml")
	os.Args = []string{"clawdex", "--config", cfg, "config"}
	main()
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}
