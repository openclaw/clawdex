package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	var out, errOut bytes.Buffer
	cfg := filepath.Join(t.TempDir(), "config.toml")
	code := run([]string{"--config", cfg, "config"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "backup-clawdex") {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errOut.String())
	}
	code = run([]string{"--bogus"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}
