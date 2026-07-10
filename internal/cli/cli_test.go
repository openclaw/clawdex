package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/contactexport"
	"github.com/openclaw/clawdex/internal/model"
)

func TestExecuteEndToEndLocalCommands(t *testing.T) {
	cfg, data := testPaths(t)
	run := func(args ...string) string {
		t.Helper()
		var out, errOut bytes.Buffer
		full := append([]string{"--config", cfg}, args...)
		if err := Execute(full, &out, &errOut); err != nil {
			t.Fatalf("Execute(%v): %v stderr=%s stdout=%s", full, err, errOut.String(), out.String())
		}
		return out.String()
	}
	out := run("init", data, "--remote", "")
	if !strings.Contains(out, "repo_path:") {
		t.Fatalf("init out = %s", out)
	}
	out = run("person", "add", "Ada Lovelace", "--email", "ada@example.com", "--phone", "+1 555 0100", "--tag", "math")
	if !strings.Contains(out, "Ada Lovelace") {
		t.Fatalf("add out = %s", out)
	}
	out = run("person", "list", "--plain")
	if !strings.Contains(out, "Ada Lovelace") {
		t.Fatalf("list out = %s", out)
	}
	out = run("person", "show", "ada@example.com")
	if !strings.Contains(out, "email: ada@example.com") {
		t.Fatalf("show out = %s", out)
	}
	avatarPath := filepath.Join(t.TempDir(), "avatar.png")
	writeCLITestPNG(t, avatarPath)
	out = run("person", "avatar", "set", "ada@example.com", avatarPath)
	if !strings.Contains(out, `"path": "avatars/avatar.png"`) {
		t.Fatalf("avatar set out = %s", out)
	}
	out = run("person", "avatar", "show", "ada@example.com", "--path")
	if !strings.Contains(out, "avatars/avatar.png") {
		t.Fatalf("avatar show path = %s", out)
	}
	out = run("person", "avatar", "show", "ada@example.com")
	if !strings.Contains(out, `"mime": "image/png"`) {
		t.Fatalf("avatar show = %s", out)
	}
	out = run("--dry-run", "person", "avatar", "set", "ada@example.com", avatarPath)
	if !strings.Contains(out, "would_set_avatar") {
		t.Fatalf("avatar dry set out = %s", out)
	}
	out = run("note", "add", "ada", "--kind", "dm", "--source", "manual", "--text", "Analytical engine")
	if !strings.Contains(out, "dm\tmanual") {
		t.Fatalf("note out = %s", out)
	}
	out = run("note", "list", "ada")
	if !strings.Contains(out, "Analytical engine") {
		t.Fatalf("notes out = %s", out)
	}
	out = run("timeline", "ada")
	if !strings.Contains(out, "Analytical engine") {
		t.Fatalf("timeline out = %s", out)
	}
	out = run("search", "engine", "--plain")
	if !strings.Contains(out, "note") {
		t.Fatalf("search out = %s", out)
	}
	vcardPath := filepath.Join(t.TempDir(), "contacts.vcf")
	out = run("export", "vcard", "--all", "--include-avatars", "-o", vcardPath)
	if !strings.Contains(out, "exported: 1") {
		t.Fatalf("export out = %s", out)
	}
	if data, err := os.ReadFile(vcardPath); err != nil || !strings.Contains(string(data), "BEGIN:VCARD") {
		t.Fatalf("vcard data=%q err=%v", data, err)
	}
	out = run("person", "avatar", "clear", "ada@example.com")
	if !strings.Contains(out, "Ada Lovelace") {
		t.Fatalf("avatar clear out = %s", out)
	}
	out = run("--dry-run", "person", "avatar", "clear", "ada@example.com")
	if !strings.Contains(out, "would_clear_avatar") {
		t.Fatalf("avatar dry clear out = %s", out)
	}
	var noAvatarOut, noAvatarErr bytes.Buffer
	if err := Execute([]string{"--config", cfg, "person", "avatar", "show", "ada@example.com"}, &noAvatarOut, &noAvatarErr); err == nil {
		t.Fatal("expected no avatar error")
	}
	out = run("sync", "apple")
	if !strings.Contains(out, "remote writes not implemented") {
		t.Fatalf("sync out = %s", out)
	}
	out = run("sync", "google", "--account", "me@example.com")
	if !strings.Contains(out, "me@example.com") {
		t.Fatalf("sync google out = %s", out)
	}
	out = run("doctor")
	if !strings.Contains(out, "people: 1") {
		t.Fatalf("doctor out = %s", out)
	}
	out = run("git", "commit", "-m", "test: contacts")
	if !strings.Contains(out, "committed: true") {
		t.Fatalf("git commit out = %s", out)
	}
	out = run("git", "commit", "-m", "test: no changes")
	if !strings.Contains(out, "committed: false") {
		t.Fatalf("git commit clean out = %s", out)
	}
}

func writeCLITestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunManualAvatarRejectsUnsafeDestinationWithoutWriting(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	for _, args := range [][]string{
		{"--config", cfg, "init", data, "--remote", ""},
		{"--config", cfg, "person", "add", "Dry Avatar", "--email", "dry-avatar@example.com"},
	} {
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	src := filepath.Join(t.TempDir(), "avatar.png")
	writeCLITestPNG(t, src)
	outside := t.TempDir()
	avatarDir := filepath.Join(data, "people", "dry-avatar", "avatars")
	if err := os.Symlink(outside, avatarDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := Execute([]string{
		"--config", cfg, "--dry-run", "person", "avatar", "set",
		"dry-avatar@example.com", src,
	}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("dry-run manual avatar error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "avatar.png")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote outside: %v", err)
	}
}

func TestExecuteConfigJSONAndUsage(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "--json", "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatalf("init: %v %s", err, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json = %s err=%v", out.String(), err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "config", "set", "git.branch", "main"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "config", "set", "google.default_account", "me@example.com"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "me@example.com") {
		t.Fatalf("dry config = %s", out.String())
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "--json", "config", "show"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"branch": "main"`) {
		t.Fatalf("config = %s", out.String())
	}
	if err := Execute([]string{"--config", cfg, "config", "set", "nope", "x"}, &out, &errOut); err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage err, got %v", err)
	}
	if err := Execute([]string{"--bogus"}, &out, &errOut); err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected parse usage err, got %v", err)
	}
	badCfg := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(badCfg, []byte("["), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"--config", badCfg, "config"}, &out, &errOut); err == nil {
		t.Fatal("expected config parse error")
	}
	if ExitCode(nil) != 0 {
		t.Fatal("nil exit code")
	}
}

func TestExecuteImportAppleFromFileAndGoogleViaFakeGog(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "apple.ndjson")
	if err := os.WriteFile(input, []byte("{\"identifier\":\"a1\",\"full_name\":\"Ada Apple\",\"emails\":[\"apple@example.com\"]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "import", "apple", "--input", input}, &out, &errOut); err != nil {
		t.Fatalf("apple import: %v %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "create\tAda Apple") {
		t.Fatalf("apple import out = %s", out.String())
	}
	fakeGog := writeFakeGog(t, `[{"resourceName":"people/g1","name":"Grace Google","email":"grace@example.com"}]`)
	t.Setenv("PATH", filepath.Dir(fakeGog)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	if err := Execute([]string{"--config", cfg, "import", "google", "--account", "me@example.com"}, &out, &errOut); err != nil {
		t.Fatalf("google import: %v %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "create\tGrace Google") {
		t.Fatalf("google import out = %s", out.String())
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "person", "show", "grace@example.com"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Grace Google") {
		t.Fatalf("show = %s", out.String())
	}
	fakeSQLite := writeFakeSQLite(t, `[{"channel_id":"dm1","name":"Discord Friend","messages":5,"counterpart_id":"user1"}]`)
	t.Setenv("PATH", filepath.Dir(fakeSQLite)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	if err := Execute([]string{"--config", cfg, "import", "discrawl", "--db", filepath.Join(t.TempDir(), "discrawl.db"), "--min-messages", "4"}, &out, &errOut); err != nil {
		t.Fatalf("discrawl import: %v %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "create\tDiscord Friend") {
		t.Fatalf("discrawl import out = %s", out.String())
	}
	fakeBirdclaw := writeFakeSQLite(t, `[{"conversation_id":"1-2","profile_id":"2","handle":"bird","display_name":"Bird Person","messages":5}]`)
	t.Setenv("PATH", filepath.Dir(fakeBirdclaw)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	if err := Execute([]string{"--config", cfg, "import", "birdclaw", "--db", filepath.Join(t.TempDir(), "birdclaw.sqlite"), "--min-messages", "4"}, &out, &errOut); err != nil {
		t.Fatalf("birdclaw import: %v %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "create\tBird Person") {
		t.Fatalf("birdclaw import out = %s", out.String())
	}
}

func TestExecuteGitStatusAndDryRun(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "person", "add", "Dry Run"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would_create: Dry Run") {
		t.Fatalf("dry run = %s", out.String())
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "git"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No commits yet") {
		t.Fatalf("git status = %s", out.String())
	}
	for _, args := range [][]string{
		{"--config", cfg, "git", "push"},
		{"--config", cfg, "git", "pull"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err == nil || !strings.Contains(err.Error(), "git remote is not configured") {
			t.Fatalf("%v: err=%v stdout=%s stderr=%s", args, err, out.String(), errOut.String())
		}
	}
}

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

func TestExecuteImportDiscrawlErrors(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fakeSQLite := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(fakeSQLite, []byte("#!/bin/sh\necho locked >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(fakeSQLite)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Execute([]string{"--config", cfg, "import", "discrawl", "--db", filepath.Join(t.TempDir(), "discrawl.db")}, &out, &errOut); err == nil {
		t.Fatal("expected discrawl import error")
	}
}

func TestExecuteImportContactsFromCrawler(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Source","phone_numbers":[" +1 555 0100 "]}]}`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Source") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsFromCrawlerPath(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Path","phone_numbers":["123"]}]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Path") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsNoopJSONIsEmptyArray(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[{"display_name":"Ada Source","phone_numbers":["+1 555 0100"]}]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("first import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--json", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("second import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	var changes []model.ImportChange
	if err := json.Unmarshal(out.Bytes(), &changes); err != nil {
		t.Fatalf("noop import output is not an array: %s", out.String())
	}
	if len(changes) != 0 {
		t.Fatalf("noop import changes = %#v", changes)
	}
}

func TestExecuteImportContactsDoesNotShellExpandManifestArgv(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "telecrawl")
	manifest := `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Fake Crawler","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export;echo shell-expanded"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"contacts\" ] && [ \"$3\" = \"export;echo shell-expanded\" ]; then\n" +
		"cat <<'JSON'\n{\"contacts\":[{\"display_name\":\"Ada Argv\",\"phone_numbers\":[\"123\"]}]}\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo unexpected args: \"$@\" >&2\nexit 2\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err != nil {
		t.Fatalf("import contacts: %v stderr=%s stdout=%s", err, errOut.String(), out.String())
	}
	if !strings.Contains(out.String(), "create\tAda Argv") {
		t.Fatalf("import contacts out = %s", out.String())
	}
}

func TestExecuteImportContactsRejectsMutatingCommand(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawlerManifest(t, "telecrawl", `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true,"mutates":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`, `{"contacts":[]}`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
		t.Fatal("expected mutating command error")
	}
}

func TestExecuteImportContactsRejectsBadManifests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
	}{
		{
			name:     "wrong schema",
			manifest: `{"schema_version":"not-crawlkit","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "missing command",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "not json",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","contacts","export"]}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "json command missing json flag",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
		{
			name:     "empty argv",
			manifest: `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":[],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, data := testPaths(t)
			var out, errOut bytes.Buffer
			if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			fake := writeFakeContactCrawlerManifest(t, "telecrawl", tc.manifest, `{"contacts":[]}`)
			t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
			out.Reset()
			errOut.Reset()
			if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
				t.Fatal("expected bad manifest error")
			}
		})
	}
}

func TestExecuteImportContactsRejectsBadPayload(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawler(t, "telecrawl", `{"contacts":[]} private junk`)
	t.Setenv("PATH", filepath.Dir(fake)+string(os.PathListSeparator)+os.Getenv("PATH"))
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", "telecrawl"}, &out, &errOut); err == nil {
		t.Fatal("expected bad payload error")
	}
}

func TestExecuteImportContactsRejectsDifferentManifestBinary(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fake := writeFakeContactCrawlerManifest(t, "telecrawl", `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Telegram Crawl","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["othercrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`, `{"contacts":[]}`)
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "import", "contacts", "--from", fake}, &out, &errOut); err == nil {
		t.Fatal("expected mismatched manifest binary error")
	}
}

func TestReadCrawlerManifestErrors(t *testing.T) {
	dir := t.TempDir()
	failing := filepath.Join(dir, "failing")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\necho metadata failed >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readCrawlerManifest(t.Context(), failing); err == nil {
		t.Fatal("expected metadata command error")
	}

	badJSON := filepath.Join(dir, "badjson")
	if err := os.WriteFile(badJSON, []byte("#!/bin/sh\necho not-json\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := readCrawlerManifest(t.Context(), badJSON); err == nil {
		t.Fatal("expected metadata decode error")
	}
}

func TestReadCrawlerContactsReportsExportFailure(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "telecrawl")
	manifest := `{"schema_version":"crawlkit.control.v1","id":"telecrawl","display_name":"Fake Crawler","binary":{"name":"telecrawl"},"commands":{"contact-export":{"argv":["telecrawl","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo export failed >&2\nexit 9\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readCrawlerContacts(t.Context(), fake); err == nil {
		t.Fatal("expected export command error")
	}
}

func TestContactExportArgv(t *testing.T) {
	got, err := contactExportArgv("/tmp/telecrawl", []string{"telecrawl", "--json", "contacts", "export"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "/tmp/telecrawl" || got[1] != "--json" {
		t.Fatalf("argv = %#v", got)
	}
	if _, err := contactExportArgv("telecrawl", nil); err == nil {
		t.Fatal("expected empty argv error")
	}
	if _, err := contactExportArgv("telecrawl", []string{"othercrawl"}); err == nil {
		t.Fatal("expected mismatched argv error")
	}
	if _, err := contactExportArgv("telecrawl", []string{"telecrawl", "contacts", "export"}); err == nil {
		t.Fatal("expected missing json flag error")
	}
}

func TestSourceContactsFromExportMapsPhones(t *testing.T) {
	contacts := sourceContactsFromExport("telecrawl", contactexport.ContactExport{Contacts: []contactexport.Contact{{
		DisplayName:  "Ada",
		PhoneNumbers: []string{"123", "456"},
	}}})
	if len(contacts) != 1 {
		t.Fatalf("contacts = %#v", contacts)
	}
	got := contacts[0]
	if got.Source != "telecrawl" || got.Name != "Ada" || len(got.Phones) != 2 {
		t.Fatalf("mapped contact = %#v", got)
	}
	if !got.Phones[0].Primary || got.Phones[1].Primary {
		t.Fatalf("primary phones = %#v", got.Phones)
	}
	if got.Phones[1].Value != "456" || got.Phones[1].Source != "telecrawl" {
		t.Fatalf("second phone = %#v", got.Phones[1])
	}
}

func TestExecuteImportBirdclawErrors(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	fakeSQLite := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(fakeSQLite, []byte("#!/bin/sh\necho locked >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(fakeSQLite)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Execute([]string{"--config", cfg, "import", "birdclaw", "--db", filepath.Join(t.TempDir(), "birdclaw.sqlite")}, &out, &errOut); err == nil {
		t.Fatal("expected birdclaw import error")
	}
}

func TestExecuteJSONPlainAndStdoutBranches(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	must := func(args ...string) string {
		t.Helper()
		out.Reset()
		errOut.Reset()
		if err := Execute(append([]string{"--config", cfg}, args...), &out, &errOut); err != nil {
			t.Fatalf("%v: %v stderr=%s", args, err, errOut.String())
		}
		return out.String()
	}
	must("init", data, "--remote", "")
	must("person", "add", "Ada JSON", "--email", "json@example.com")
	must("person", "add", "Empty Email")
	if got := must("--json", "person", "show", "json@example.com"); !strings.Contains(got, `"name": "Ada JSON"`) {
		t.Fatalf("json show = %s", got)
	}
	if got := must("--json", "person", "list", "--query", "Ada"); !strings.Contains(got, `"Ada JSON"`) {
		t.Fatalf("json list = %s", got)
	}
	if got := must("--plain", "person", "show", "json@example.com"); !strings.Contains(got, "Ada JSON") {
		t.Fatalf("plain show = %s", got)
	}
	if got := must("--plain", "person", "list", "--query", "NoMatch"); got != "" {
		t.Fatalf("empty list = %s", got)
	}
	if got := must("person", "list", "--query", "Empty"); !strings.Contains(got, "Empty Email") {
		t.Fatalf("no-email list = %s", got)
	}
	must("note", "add", "json@example.com", "--kind", "call", "--source", "manual", "--text", "Call body", "--occurred-at", "2026-05-08 10:00")
	if got := must("--json", "note", "list", "json@example.com"); !strings.Contains(got, `"kind": "call"`) {
		t.Fatalf("json notes = %s", got)
	}
	if got := must("export", "vcard", "--person", "json@example.com", "-o", "-"); !strings.Contains(got, "BEGIN:VCARD") {
		t.Fatalf("stdout vcard = %s", got)
	}
	input := filepath.Join(t.TempDir(), "apple.ndjson")
	if err := os.WriteFile(input, []byte("{\"identifier\":\"a1\",\"full_name\":\"Dry Apple\",\"emails\":[\"dry@example.com\"]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := must("--dry-run", "import", "apple", "--input", input); !strings.Contains(got, "create\tDry Apple") {
		t.Fatalf("dry import = %s", got)
	}
}

func TestPrintHelpersCoverPlainJSONAndWriteErrors(t *testing.T) {
	var out bytes.Buffer
	person := model.Person{
		ID:     "person_1",
		Name:   "Print Person",
		Path:   "/tmp/person.md",
		Emails: []model.ContactValue{{Value: "print@example.com"}},
	}
	note := model.Note{
		ID:         "note_1",
		Kind:       "note",
		Source:     "manual",
		OccurredAt: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		Body:       "line one\nline two",
	}
	hit := model.SearchHit{Kind: "note", ID: "note_1", Name: "Print Person", Snippet: "line", Path: "/tmp/note.md"}

	r := &Runtime{stdout: &out, root: &CLI{}}
	if err := r.printPeople([]model.Person{person}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "print@example.com") {
		t.Fatalf("people out = %s", out.String())
	}
	out.Reset()
	if err := r.printTimeline([]model.Note{note}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\nline two") {
		t.Fatalf("timeline did not flatten body = %s", out.String())
	}
	out.Reset()
	if err := r.printHits([]model.SearchHit{hit}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "line") {
		t.Fatalf("hits out = %s", out.String())
	}
	out.Reset()
	r.root.Plain = true
	if err := r.printPeople([]model.Person{{ID: "person_2", Name: "No Email", Path: "/tmp/no.md"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.printHits([]model.SearchHit{hit}); err != nil {
		t.Fatal(err)
	}
	r.root.JSON = true
	if err := r.printTimeline([]model.Note{note}); err != nil {
		t.Fatal(err)
	}

	r.stdout = errWriter{}
	r.root.JSON = false
	if err := r.printPeople([]model.Person{person}); err == nil {
		t.Fatal("expected printPeople write error")
	}
	if err := r.printTimeline([]model.Note{note}); err == nil {
		t.Fatal("expected printTimeline write error")
	}
	if err := r.printHits([]model.SearchHit{hit}); err == nil {
		t.Fatal("expected printHits write error")
	}
	if err := r.printPerson(person); err == nil {
		t.Fatal("expected printPerson write error")
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestExecuteGitPushPullWithLocalRemote(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	data := filepath.Join(dir, "contacts")
	remote := filepath.Join(dir, "remote.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runShell(t, remote, "git", "init", "--bare")
	var out, errOut bytes.Buffer
	for _, args := range [][]string{
		{"--config", cfg, "init", data, "--remote", remote},
		{"--config", cfg, "person", "add", "Ada Remote"},
		{"--config", cfg, "git", "commit", "-m", "test: remote"},
		{"--config", cfg, "git", "push"},
		{"--config", cfg, "git", "pull"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v stderr=%s stdout=%s", args, err, errOut.String(), out.String())
		}
	}
}

func TestExecuteGitPushPullWithExistingOrigin(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	data := filepath.Join(dir, "contacts")
	remote := filepath.Join(dir, "remote.git")
	if err := os.Mkdir(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	runShell(t, remote, "git", "init", "--bare")
	var out, errOut bytes.Buffer
	for _, args := range [][]string{
		{"--config", cfg, "init", data, "--remote", ""},
		{"--config", cfg, "person", "add", "Ada Origin"},
		{"--config", cfg, "git", "commit", "-m", "test: origin"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v stderr=%s stdout=%s", args, err, errOut.String(), out.String())
		}
	}
	runShell(t, data, "git", "remote", "add", "origin", remote)
	for _, args := range [][]string{
		{"--config", cfg, "git", "push"},
		{"--config", cfg, "git", "pull"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err != nil {
			t.Fatalf("%v: %v stderr=%s stdout=%s", args, err, errOut.String(), out.String())
		}
	}
}

func TestExecuteEditorExportPersonAndRepair(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"--config", cfg, "person", "add", "Ada Edit", "--email", "edit@example.com"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	editor := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf '%s' \"$1\" > \""+filepath.Join(t.TempDir(), "edited")+"\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)
	if err := Execute([]string{"--config", cfg, "person", "edit", "edit@example.com"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	vcardPath := filepath.Join(t.TempDir(), "one.vcf")
	if err := Execute([]string{"--config", cfg, "export", "vcard", "--person", "edit@example.com", "-o", vcardPath}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	personPath := filepath.Join(data, "people", "ada-edit", "person.md")
	if err := os.WriteFile(personPath, []byte("---\nid: person_x\nname: Ada Edit\ntags: [broken\n---\n# Ada Edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "doctor", "--repair"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "repaired: 1") {
		t.Fatalf("repair dry-run = %s", out.String())
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "doctor", "--repair"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "repaired: 1") {
		t.Fatalf("repair = %s", out.String())
	}
	if err := os.WriteFile(personPath, []byte("---\nid: person_x\nname: Ada Edit\navatar:\n  path: avatars/missing.png\n---\n# Ada Edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "--dry-run", "doctor", "--repair"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "avatar_problems: 1") || !strings.Contains(out.String(), "avatar_repaired: 1") {
		t.Fatalf("avatar repair dry-run = %s", out.String())
	}
	out.Reset()
	if err := Execute([]string{"--config", cfg, "doctor", "--repair"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "avatar_repaired: 1") {
		t.Fatalf("avatar repair = %s", out.String())
	}
}

func TestExecuteUsageGuards(t *testing.T) {
	cfg, data := testPaths(t)
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--config", cfg, "note", "add", "nobody", "--kind", "note", "--source", "manual"},
		{"--config", cfg, "note", "add", "nobody", "--kind", "note", "--source", "manual", "--text", "x", "--occurred-at", "bad"},
		{"--config", cfg, "export", "vcard", "-o", filepath.Join(t.TempDir(), "x.vcf")},
		{"--config", cfg, "person", "show", "missing"},
		{"--config", cfg, "person", "avatar", "clear", "missing"},
		{"--config", cfg, "--dry-run", "person", "avatar", "set", "nobody", filepath.Join(t.TempDir(), "missing.png")},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestSmallCLIHelpers(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := firstNonEmpty("", " "); got != "" {
		t.Fatalf("firstNonEmpty empty = %q", got)
	}
}

func TestResolveVersionUsesTaggedBuildInfoFallback(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1"}}
	if got := resolveVersion("dev", info, true); got != "0.1.1" {
		t.Fatalf("build info version = %q", got)
	}
	if got := resolveVersion("v0.2.0", info, true); got != "0.2.0" {
		t.Fatalf("linked version = %q", got)
	}
	if got := resolveVersion("", nil, false); got != "dev" {
		t.Fatalf("development version = %q", got)
	}
}

func TestExecuteErrorBranchesAndNoConfigInit(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	data := filepath.Join(dir, "contacts")
	var out, errOut bytes.Buffer
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", "", "--no-config"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Fatalf("config unexpectedly exists: %v", err)
	}
	for _, args := range [][]string{
		{"--config", cfg, "--repo", filepath.Join(dir, "missing"), "person", "add", "No Repo"},
		{"--config", cfg, "--repo", filepath.Join(dir, "missing"), "person", "list"},
		{"--config", cfg, "--repo", filepath.Join(dir, "missing"), "export", "vcard", "--all", "-o", "-"},
		{"--config", cfg, "--repo", filepath.Join(dir, "missing"), "doctor"},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	out.Reset()
	errOut.Reset()
	if err := Execute([]string{"--config", cfg, "init", data, "--remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"--config", cfg, "config", "set", "repo_path", data}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := Execute([]string{"--config", cfg, "config", "set", "git.remote", ""}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--config", cfg, "note", "list", "missing"},
		{"--config", cfg, "timeline", "missing"},
		{"--config", cfg, "search", ""},
		{"--config", cfg, "export", "vcard", "--person", "missing", "-o", "-"},
		{"--config", cfg, "export", "vcard", "--all", "-o", filepath.Join(dir, "nope", "x.vcf")},
		{"--config", cfg, "import", "apple", "--input", filepath.Join(dir, "missing.ndjson")},
	} {
		out.Reset()
		errOut.Reset()
		if err := Execute(args, &out, &errOut); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
	fakeGog := writeFakeGogExit(t)
	t.Setenv("PATH", filepath.Dir(fakeGog)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Execute([]string{"--config", cfg, "import", "google"}, &out, &errOut); err == nil {
		t.Fatal("expected fake gog failure")
	}
}

func testPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "config.toml"), filepath.Join(dir, "contacts")
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%s:%o", info.Mode().Type(), info.Mode().Perm())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(data))
		}
		out[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFakeGog(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	name := "gog"
	if runtime.GOOS == "windows" {
		name = "gog.bat"
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s\\n' '" + output + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeGogExit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gog")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho bad >&2\nexit 4\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeSQLite(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sqlite3")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat <<'JSON'\n"+output+"\nJSON\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeContactCrawler(t *testing.T, name, contacts string) string {
	t.Helper()
	manifest := `{"schema_version":"crawlkit.control.v1","id":"` + name + `","display_name":"Fake Crawler","binary":{"name":"` + name + `"},"commands":{"contact-export":{"argv":["` + name + `","--json","contacts","export"],"json":true}},"privacy":{"contains_private_messages":true,"exports_secrets":false}}`
	return writeFakeContactCrawlerManifest(t, name, manifest, contacts)
}

func writeFakeContactCrawlerManifest(t *testing.T, name, manifest, contacts string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"metadata\" ]; then\n" +
		"cat <<'JSON'\n" + manifest + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"--json\" ] && [ \"$2\" = \"contacts\" ] && [ \"$3\" = \"export\" ]; then\n" +
		"cat <<'JSON'\n" + contacts + "\nJSON\n" +
		"exit 0\n" +
		"fi\n" +
		"echo unexpected args: \"$@\" >&2\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func runShell(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
