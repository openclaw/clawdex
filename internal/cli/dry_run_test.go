package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestGlobalDryRunLeavesFilesystemAndGitStateUntouched(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	data := filepath.Join(dir, "contacts")
	beforeInit := snapshotTree(t, dir)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "--dry-run", "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if after := snapshotTree(t, dir); !reflect.DeepEqual(beforeInit, after) {
		t.Fatalf("dry-run init changed filesystem\nbefore=%v\nafter=%v", beforeInit, after)
	}

	remote := filepath.Join(dir, "remote.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runShell(t, remote, "git", "init", "--bare")
	for _, args := range [][]string{
		{"--config", cfg, "init", data, "--remote", remote},
		{"--config", cfg, "person", "add", "Ada Dry", "--email", "ada@example.com"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	avatarPath := filepath.Join(dir, "avatar.png")
	writeCLITestPNG(t, avatarPath)
	appleInput := filepath.Join(dir, "apple.ndjson")
	if err := os.WriteFile(appleInput, []byte("{\"identifier\":\"a1\",\"full_name\":\"Apple Dry\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "editor-ran")
	editor := filepath.Join(dir, "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf touched > \""+marker+"\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)
	fsmonitorMarker := filepath.Join(dir, "fsmonitor-ran")
	if runtime.GOOS != "windows" {
		fsmonitorHook := filepath.Join(dir, "fsmonitor-hook")
		if err := os.WriteFile(fsmonitorHook, []byte("#!/bin/sh\nprintf touched > \"$CLAWDEX_FSMONITOR_MARKER\"\nprintf '\\n'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CLAWDEX_FSMONITOR_MARKER", fsmonitorMarker)
		runShell(t, data, "git", "config", "core.fsmonitor", fsmonitorHook)
	}
	personPath := filepath.Join(data, "people", "ada-dry", "person.md")
	if err := os.WriteFile(personPath, []byte("---\nid: person_dry\nname: Ada Dry\ntags: [broken\n---\n# Ada Dry\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(dir, "contacts.vcf")
	before := snapshotTree(t, dir)
	runDry := func(args []string) {
		t.Helper()
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v stderr=%s stdout=%s", args, err, errOut.String(), out.String())
		}
		if after := snapshotTree(t, dir); !reflect.DeepEqual(before, after) {
			t.Fatalf("%v changed filesystem or git state\nbefore=%v\nafter=%v", args, before, after)
		}
	}
	commands := [][]string{
		{"--config", cfg, "--dry-run", "config", "show"},
		{"--config", cfg, "--dry-run", "config", "set", "google.default_account", "dry@example.com"},
		{"--config", cfg, "--dry-run", "person", "add", "No Write"},
		{"--config", cfg, "--dry-run", "person", "list"},
		{"--config", cfg, "--dry-run", "person", "show", "Ada Dry"},
		{"--config", cfg, "--dry-run", "person", "edit", "Ada Dry"},
		{"--config", cfg, "--dry-run", "person", "avatar", "set", "Ada Dry", avatarPath},
		{"--config", cfg, "--dry-run", "person", "avatar", "clear", "Ada Dry"},
		{"--config", cfg, "--dry-run", "note", "add", "Ada Dry", "--kind", "note", "--source", "manual", "--text", "no write"},
		{"--config", cfg, "--dry-run", "note", "list", "Ada Dry"},
		{"--config", cfg, "--dry-run", "timeline", "Ada Dry"},
		{"--config", cfg, "--dry-run", "search", "Ada"},
		{"--config", cfg, "--dry-run", "import", "apple", "--input", appleInput},
		{"--config", cfg, "--dry-run", "sync", "apple"},
		{"--config", cfg, "--dry-run", "sync", "google", "--account", "dry@example.com"},
		{"--config", cfg, "--dry-run", "export", "vcard", "--all", "-o", exportPath},
		{"--config", cfg, "--dry-run", "export", "vcard", "--all", "-o", "-"},
		{"--config", cfg, "--dry-run", "git", "commit", "-m", "test: dry"},
		{"--config", cfg, "--dry-run", "git", "pull"},
		{"--config", cfg, "--dry-run", "git", "push"},
		{"--config", cfg, "--dry-run", "git", "status"},
		{"--config", cfg, "--dry-run", "doctor"},
		{"--config", cfg, "--dry-run", "doctor", "--repair"},
	}
	for _, args := range commands {
		runDry(args)
	}

	fakeGog := writeFakeGog(t, `[{"resourceName":"people/g1","name":"Google Dry","email":"google@example.com"}]`)
	t.Setenv("PATH", filepath.Dir(fakeGog)+string(os.PathListSeparator)+os.Getenv("PATH"))
	runDry([]string{"--config", cfg, "--dry-run", "import", "google", "--account", "dry@example.com"})

	fakeDiscrawl := writeFakeSQLite(t, `[{"channel_id":"dm1","name":"Discord Dry","messages":5,"counterpart_id":"user1"}]`)
	t.Setenv("PATH", filepath.Dir(fakeDiscrawl)+string(os.PathListSeparator)+os.Getenv("PATH"))
	runDry([]string{"--config", cfg, "--dry-run", "import", "discrawl", "--db", filepath.Join(dir, "discrawl.db")})

	fakeBirdclaw := writeFakeSQLite(t, `[{"conversation_id":"1-2","profile_id":"2","handle":"dry","display_name":"Bird Dry","messages":5}]`)
	t.Setenv("PATH", filepath.Dir(fakeBirdclaw)+string(os.PathListSeparator)+os.Getenv("PATH"))
	runDry([]string{"--config", cfg, "--dry-run", "import", "birdclaw", "--db", filepath.Join(dir, "birdclaw.sqlite")})

	fakeCrawler := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Crawler Dry","phone_numbers":["123"]}]}`)
	runDry([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fakeCrawler})

	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "person", "avatar", "show", "Ada Dry"}, &out, &errOut); err == nil {
		t.Fatal("expected missing-avatar error")
	}
	if after := snapshotTree(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("failed dry-run avatar show changed state\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("dry-run editor executed: %v", err)
	}
	if _, err := os.Stat(fsmonitorMarker); !os.IsNotExist(err) {
		t.Fatalf("dry-run FSMonitor executed: %v", err)
	}
	if _, err := os.Stat(exportPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run export wrote output: %v", err)
	}
}

func TestDryRunVCardValidatesDestinationWithoutWriting(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	for _, args := range [][]string{
		{"--config", cfg, "init", data, "--remote", ""},
		{"--config", cfg, "person", "add", "Ada Dry Export"},
	} {
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}

	destinationDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.vcf")
	if err := os.WriteFile(outside, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(destinationDir, "contacts.vcf")
	if err := os.Symlink(outside, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	run := func(path string) error {
		out.Reset()
		errOut.Reset()
		return Execute([]string{"--config", cfg, "--dry-run", "export", "vcard", "--all", "-o", path}, &out, &errOut)
	}
	if err := run(leaf); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("leaf symlink error = %v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "untouched" {
		t.Fatalf("outside = %q err=%v", got, err)
	}

	linkedParent := filepath.Join(destinationDir, "linked-parent")
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, linkedParent); err != nil {
		t.Fatal(err)
	}
	linkedOutput := filepath.Join(linkedParent, "contacts.vcf")
	if err := run(linkedOutput); err != nil {
		t.Fatalf("parent alias dry-run = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "contacts.vcf")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote through parent alias: %v", err)
	}
	if err := run(destinationDir + string(filepath.Separator)); err == nil || !strings.Contains(err.Error(), "must name a file") {
		t.Fatalf("directory-form error = %v", err)
	}

	valid := filepath.Join(destinationDir, "valid.vcf")
	if err := run(valid); err != nil {
		t.Fatalf("valid dry-run destination: %v", err)
	}
	if _, err := os.Stat(valid); !os.IsNotExist(err) {
		t.Fatalf("dry-run output exists: %v", err)
	}
}
