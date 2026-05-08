package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

func TestPersonRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people", "ada", "person.md")
	now := time.Date(2026, 5, 8, 9, 15, 0, 0, time.UTC)
	p := NewPerson("Ada Lovelace", now)
	p.Tags = []string{"math"}
	p.Body = "# Ada Lovelace\n\nNotes."
	if err := WritePerson(path, p); err != nil {
		t.Fatal(err)
	}
	got, report, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Needed {
		t.Fatalf("unexpected repair report: %#v", report)
	}
	if got.ID != p.ID || got.Name != p.Name || strings.TrimSpace(got.Body) != strings.TrimSpace(p.Body) {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}

func TestPersonRepairSalvagesBrokenFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people", "ada", "person.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: person_1\nname: Ada Lovelace\ntags: [math\n---\n# Ada\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, report, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed || p.ID != "person_1" || p.Name != "Ada Lovelace" {
		t.Fatalf("repair = %#v person = %#v", report, p)
	}
	if err := RepairPerson(path, filepath.Join(dir, ".clawdex", "repairs"), p, report, true); err != nil {
		t.Fatal(err)
	}
	repaired, _, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(repaired.Body, "Recovered metadata") {
		t.Fatalf("missing recovered metadata in body: %q", repaired.Body)
	}
}

func TestReadPersonMissingFrontmatterInfersNameFromHeading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people", "ada", "person.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Ada Heading\n\nBody"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, report, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed || p.Name != "Ada Heading" || p.ID == "" {
		t.Fatalf("person=%#v report=%#v", p, report)
	}
}

func TestReadPersonMalformedDelimiterUsesSlug(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people", "slug-name", "person.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: Ada\n# Missing close"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, report, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed || p.Name != "Missing close" {
		t.Fatalf("person=%#v report=%#v", p, report)
	}
}

func TestNoteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	n := NewNote("person_1", "dm", "whatsapp", "hello", now, now, []string{"intro"})
	if err := WriteNote(path, n); err != nil {
		t.Fatal(err)
	}
	got, report, err := ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Needed || got.ID != n.ID || got.PersonID != "person_1" || strings.TrimSpace(got.Body) != "hello" {
		t.Fatalf("got %#v report %#v", got, report)
	}
}

func TestReadNoteRepairAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	body := "---\nid: note_1\nperson_id: person_1\noccurred_at: 2026-05-08\ncaptured_at: 2026-05-08T09:00:00Z\ntopics: [one, two\n---\nBody\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	n, report, err := ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed || n.ID != "note_1" || n.Kind != "note" || n.Source != "manual" || n.Privacy != "normal" {
		t.Fatalf("note=%#v report=%#v", n, report)
	}
}

func TestFrontmatterHelpers(t *testing.T) {
	front, body, ok := splitFrontmatter([]byte("---\nid: x\n---"))
	if !ok || strings.TrimSpace(front) != "id: x" || body != "" {
		t.Fatalf("front=%q body=%q ok=%v", front, body, ok)
	}
	for _, value := range []string{"2026-05-08", "2026-05-08T09:00:00Z", "bad"} {
		_ = parseTime(value)
	}
	n := NewNote("p", "", "", "", time.Time{}, time.Time{}, nil)
	if n.Kind != "" || !n.OccurredAt.IsZero() {
		t.Fatalf("new note zero = %#v", n)
	}
	if got := NoteFileName(n); !strings.HasSuffix(got, "-note.md") {
		t.Fatalf("note file = %q", got)
	}
}

func TestWritePersonOmitsEmptyStructsButKeepsNonEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "person.md")
	p := NewPerson("Ada", time.Now())
	p.Accounts = map[string][]string{"github": {"ada"}}
	p.Google.Resource = "people/c1"
	if err := WritePerson(path, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "accounts:") || !strings.Contains(text, "google:") || strings.Contains(text, "apple:") {
		t.Fatalf("frontmatter = %s", text)
	}
}

func TestMoreRepairAndNoteBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("body only"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, report, err := ReadNote(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed || n.Kind != "note" || strings.TrimSpace(n.Body) != "body only" {
		t.Fatalf("note=%#v report=%#v", n, report)
	}
	now := time.Now().UTC()
	n.FollowUpAt = now
	if err := WriteNote(path, n); err != nil {
		t.Fatal(err)
	}
	if err := RepairPerson(path, filepath.Join(dir, "repairs"), modelPersonForTest(), RepairReport{}, true); err != nil {
		t.Fatal(err)
	}
	if nameFromBody("no heading") != "" {
		t.Fatal("expected no heading")
	}
	parentFile := filepath.Join(dir, "file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(parentFile, "child"), []byte("x"), 0o600); err == nil {
		t.Fatal("expected atomic write mkdir error")
	}
}

func modelPersonForTest() model.Person {
	return NewPerson("Ada", time.Now())
}
