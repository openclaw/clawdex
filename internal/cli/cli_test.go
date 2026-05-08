package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	out = run("export", "vcard", "--all", "-o", vcardPath)
	if !strings.Contains(out, "exported: 1") {
		t.Fatalf("export out = %s", out)
	}
	if data, err := os.ReadFile(vcardPath); err != nil || !strings.Contains(string(data), "BEGIN:VCARD") {
		t.Fatalf("vcard data=%q err=%v", data, err)
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

func runShell(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
