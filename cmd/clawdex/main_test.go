package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	var out, errOut bytes.Buffer
	cfg := filepath.Join(t.TempDir(), "config.toml")
	code := run([]string{"--config", cfg, "config"}, &out, &errOut)
	var payload struct {
		Git struct {
			Remote string `json:"remote"`
		} `json:"git"`
	}
	if code != 0 || json.Unmarshal(out.Bytes(), &payload) != nil || payload.Git.Remote != "" {
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
