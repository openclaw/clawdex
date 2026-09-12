package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

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
