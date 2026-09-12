package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/openclaw/clawdex/internal/safefile"
)

func TestDoctorPreservesOversizedManualAvatar(t *testing.T) {
	cfg, data := testPaths(t)
	src := filepath.Join(t.TempDir(), "avatar.png")
	writeCLITestPNG(t, src)
	for _, args := range [][]string{
		{"init", data, "--remote", ""},
		{"person", "add", "Legacy Avatar", "--email", "legacy@example.com"},
		{"person", "avatar", "set", "legacy@example.com", src},
	} {
		if err := Execute(append([]string{"--config", cfg}, args...), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	personPath := filepath.Join(data, "people", "legacy-avatar", "person.md")
	avatarPath := filepath.Join(filepath.Dir(personPath), "avatars", "avatar.png")
	if err := os.Truncate(avatarPath, safefile.MaxReadBytes+1); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, data)
	for _, dryRun := range []bool{true, false} {
		args := []string{"--config", cfg, "doctor", "--repair"}
		if dryRun {
			args = append(args, "--dry-run")
		}
		err := Execute(args, &bytes.Buffer{}, &bytes.Buffer{})
		if dryRun && err != nil || !dryRun && !errors.Is(err, safefile.ErrTooLarge) {
			t.Fatalf("dry-run=%v repair error=%v", dryRun, err)
		}
		if after := snapshotTree(t, data); !reflect.DeepEqual(before, after) {
			t.Fatalf("doctor changed oversized avatar or metadata: before=%v after=%v", before, after)
		}
	}
}

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
