package index

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/repo"
)

func TestAddNoteAndSearch(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	p, err := s.AddPerson("Ada Lovelace", []string{"ada@example.com"}, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	n := markdown.NewNote(p.ID, "dm", "manual", "Analytical engine follow-up", time.Time{}, time.Now(), []string{"math"})
	if _, err := s.AddNote("ada@example.com", n); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search("engine")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Kind != "note" {
		t.Fatalf("hits = %#v", hits)
	}
}

func TestAvatarSetAndImportBackfill(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	img := filepath.Join(t.TempDir(), "avatar.png")
	writeTestPNG(t, img)
	data, err := os.ReadFile(img)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddPerson("Ada Avatar", []string{"avatar@example.com"}, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.SetAvatar("avatar@example.com", img, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Avatar.Source != "manual" {
		t.Fatalf("avatar = %#v", p.Avatar)
	}
	p, err = s.ClearAvatar("avatar@example.com", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if p.Avatar.Path != "" {
		t.Fatalf("clear avatar = %#v", p.Avatar)
	}
	_, err = s.SetAvatar("avatar@example.com", img, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	changes, err := s.ImportContacts("apple", []model.SourceContact{{
		ExternalID: "a1",
		Name:       "Ada Avatar",
		Emails:     []model.ContactValue{{Value: "avatar@example.com"}},
		Avatar:     &model.SourceAvatar{Data: data},
	}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected external update without manual avatar overwrite, changes = %#v", changes)
	}
	p, err = s.FindPerson("avatar@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.Avatar.Source != "manual" {
		t.Fatalf("manual avatar overwritten: %#v", p.Avatar)
	}

	changes, err = s.ImportContacts("apple", []model.SourceContact{{
		ExternalID: "a2",
		Name:       "Apple Avatar",
		Emails:     []model.ContactValue{{Value: "apple-avatar@example.com"}},
		Avatar:     &model.SourceAvatar{Data: data},
	}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "create" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err = s.FindPerson("apple-avatar@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.Avatar.Source != "apple" || p.Avatar.Path != "avatars/avatar.png" {
		t.Fatalf("imported avatar = %#v", p.Avatar)
	}
	p.Avatar.SHA256 = "stale"
	if err := markdown.WritePerson(p.Path, p); err != nil {
		t.Fatal(err)
	}
	p, changed, err := s.RepairAvatarMetadata(p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || p.Avatar.SHA256 == "stale" {
		t.Fatalf("metadata repair failed: changed=%v avatar=%#v", changed, p.Avatar)
	}
	p.Avatar.Path = "avatars/missing.png"
	if err := markdown.WritePerson(p.Path, p); err != nil {
		t.Fatal(err)
	}
	p, changed, err = s.RepairAvatarMetadata(p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || p.Avatar.Path != "" {
		t.Fatalf("missing avatar was not cleared: changed=%v avatar=%#v", changed, p.Avatar)
	}
}

func TestFindPersonVariantsAndErrors(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	p, err := s.AddPerson("Ada Lovelace", []string{"ada@example.com"}, []string{"+1 555 0100"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPerson("Ada Lovelace", []string{"ada2@example.com"}, nil, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{p.ID, "ada@example.com", "15550100"} {
		got, err := s.FindPerson(query)
		if err != nil || got.ID != p.ID {
			t.Fatalf("query %q got=%#v err=%v", query, got, err)
		}
	}
	if _, err := s.FindPerson("ada"); err == nil {
		t.Fatal("expected ambiguous name")
	}
	if _, err := s.FindPerson("missing"); err == nil {
		t.Fatal("expected missing")
	}
	if _, err := s.FindPerson(""); err == nil {
		t.Fatal("expected empty query error")
	}
}

func TestNotesMissingDirAndDuplicateNames(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if _, err := s.AddPerson("Ada Lovelace", nil, nil, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	notes, err := s.Notes("Ada Lovelace")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v", notes)
	}
	now := time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC)
	n := markdown.NewNote("", "dm", "manual", "first", now, now, nil)
	if _, err := s.AddNote("Ada Lovelace", n); err != nil {
		t.Fatal(err)
	}
	n.Body = "second"
	if _, err := s.AddNote("Ada Lovelace", n); err != nil {
		t.Fatal(err)
	}
	notes, err = s.Notes("Ada Lovelace")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || notes[0].Body == notes[1].Body {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestImportMatchesEmail(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if _, err := s.AddPerson("Ada Lovelace", []string{"ada@example.com"}, nil, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	changes, err := s.ImportContacts("google", []model.SourceContact{{
		Source: "google",
		Name:   "Ada Lovelace",
		Emails: []model.ContactValue{{Value: "ADA@example.com"}},
		Phones: []model.ContactValue{{Value: "+1 555 0100"}},
	}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "update" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err := s.FindPerson("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Phones) != 1 {
		t.Fatalf("phones = %#v", p.Phones)
	}
}

func TestImportWritesExternalOnlyChange(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if _, err := s.AddPerson("Ada Lovelace", []string{"ada@example.com"}, nil, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	changes, err := s.ImportContacts("google", []model.SourceContact{{
		ExternalID: "people/c1",
		Name:       "Ada Lovelace",
		Emails:     []model.ContactValue{{Value: "ada@example.com"}},
	}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "update" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err := s.FindPerson("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.Google.Resource != "people/c1" {
		t.Fatalf("google = %#v", p.Google)
	}
}

func TestImportCreateDryRunAndExternalID(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	now := time.Now()
	changes, err := s.ImportContacts("apple", []model.SourceContact{{
		ExternalID: "a1",
		Name:       "Ada Apple",
		Emails:     []model.ContactValue{{Value: "apple@example.com"}},
	}}, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "create" {
		t.Fatalf("changes = %#v", changes)
	}
	if _, err := s.FindPerson("apple@example.com"); err == nil {
		t.Fatal("dry-run created person")
	}
	if _, err := s.ImportContacts("apple", []model.SourceContact{{
		ExternalID: "a1",
		Name:       "Ada Apple",
		Emails:     []model.ContactValue{{Value: "apple@example.com"}},
	}}, false, now); err != nil {
		t.Fatal(err)
	}
	p, err := s.FindPerson("apple@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.Apple.ID != "a1" {
		t.Fatalf("apple ref = %#v", p.Apple)
	}
	changes, err = s.ImportContacts("apple", []model.SourceContact{{
		ExternalID: "a1",
		Name:       "Ada Renamed",
		Phones:     []model.ContactValue{{Value: "+1 555 0101"}},
	}}, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "update" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestImportTagsAccountsAndExactNameMatch(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	now := time.Now()
	changes, err := s.ImportContacts("discord", []model.SourceContact{{
		ExternalID: "channel1",
		Name:       "Discord Person",
		Tags:       []string{"discord", "dm"},
		Accounts:   map[string][]string{"discord": {"channel:channel1", "user:user1"}},
	}}, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "create" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err := s.FindPerson("Discord Person")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tags) != 2 || len(p.Accounts["discord"]) != 2 {
		t.Fatalf("person = %#v", p)
	}
	changes, err = s.ImportContacts("discord", []model.SourceContact{{
		Name:     "Discord Person",
		Tags:     []string{"discord", "dm", "friend"},
		Accounts: map[string][]string{"discord": {"channel:channel1"}},
	}}, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "update" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err = s.FindPerson("Discord Person")
	if err != nil {
		t.Fatal(err)
	}
	if !stringIn(p.Tags, "friend") {
		t.Fatalf("tags = %#v", p.Tags)
	}
}

func TestSearchPersonAndEmptyQuery(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if _, err := s.AddPerson("Ada Lovelace", nil, nil, []string{"math"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search("math")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Kind != "person" {
		t.Fatalf("hits = %#v", hits)
	}
	if _, err := s.Search(""); err == nil {
		t.Fatal("expected empty search error")
	}
}

func TestSearchAccountsAndBadNoteError(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	p, err := s.AddPerson("Handle Person", nil, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Accounts = map[string][]string{"github": {"handle-person"}}
	if err := markdown.WritePerson(p.Path, p); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search("handle-person")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Kind != "person" {
		t.Fatalf("hits = %#v", hits)
	}
	notesDir := filepath.Join(filepath.Dir(p.Path), "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(notesDir, "missing"), filepath.Join(notesDir, "bad.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search("handle-person"); err == nil {
		t.Fatal("expected bad note read error")
	}
}

func TestSmallHelpers(t *testing.T) {
	if got := cleanList([]string{"a", "a", "", " b "}); len(got) != 2 || got[1] != "b" {
		t.Fatalf("clean = %#v", got)
	}
	if scoreText("abc", "abc") != 100 || scoreText("abc xyz", "abc") != 1 || scoreText("abc", "z") != 0 {
		t.Fatal("bad scores")
	}
	if got := snippet("short body", "missing"); got != "" {
		t.Fatalf("snippet = %q", got)
	}
}

func TestPeopleAutoRepairRebuildAccountsAndImportNoop(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	p, err := s.AddPerson("Ada Accounts", []string{"acct@example.com"}, nil, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Accounts = map[string][]string{"github": {"ada"}}
	if err := markdown.WritePerson(p.Path, p); err != nil {
		t.Fatal(err)
	}
	if err := s.Rebuild(); err != nil {
		t.Fatal(err)
	}
	people, err := s.People()
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 {
		t.Fatalf("people = %#v", people)
	}
	changes, err := s.ImportContacts("google", []model.SourceContact{{Name: ""}, {Name: "Ada Accounts", Emails: []model.ContactValue{{Value: "acct@example.com"}}}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestPeopleAndNotesForgivingBranches(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if err := os.WriteFile(filepath.Join(r.PeopleDir(), "not-a-dir"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.PeopleDir(), "empty-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(r.PeopleDir(), "broken-person", "person.md")
	if err := os.MkdirAll(filepath.Dir(brokenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brokenPath, []byte("---\nid: person_broken\nname: Broken Person\ntags: [oops\n---\n# Broken Person\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	people, err := s.People()
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].Name != "Broken Person" {
		t.Fatalf("people = %#v", people)
	}
	notesDir := filepath.Join(filepath.Dir(people[0].Path), "notes")
	if err := os.MkdirAll(filepath.Join(notesDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "skip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	note := markdown.NewNote("", "note", "manual", "missing person id", time.Now(), time.Now(), nil)
	note.PersonID = ""
	if err := markdown.WriteNote(filepath.Join(notesDir, "2026-note.md"), note); err != nil {
		t.Fatal(err)
	}
	notes, err := s.Notes("Broken Person")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].PersonID != "person_broken" {
		t.Fatalf("notes = %#v", notes)
	}
	if err := os.RemoveAll(r.IndexDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.IndexDir(), []byte("not dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Rebuild(); err == nil {
		t.Fatal("expected index dir mkdir error")
	}
}

func TestImportMatchesPhoneAndGoogleExternal(t *testing.T) {
	r := testRepo(t)
	s := New(r)
	if _, err := s.ImportContacts("google", []model.SourceContact{{ExternalID: "people/c1", Name: "Ada Google", Phones: []model.ContactValue{{Value: "+1 555 0100"}}}}, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	changes, err := s.ImportContacts("google", []model.SourceContact{{ExternalID: "people/c1", Name: "Ada Google", Emails: []model.ContactValue{{Value: "g@example.com"}}}}, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != "update" {
		t.Fatalf("changes = %#v", changes)
	}
	p, err := s.FindPerson("+1 555 0100")
	if err != nil {
		t.Fatal(err)
	}
	if p.Google.Resource != "people/c1" {
		t.Fatalf("google = %#v", p.Google)
	}
}

func testRepo(t *testing.T) repo.Repo {
	t.Helper()
	dir := t.TempDir()
	cfg := repo.DefaultConfig()
	cfg.RepoPath = dir
	cfg.Git.Remote = ""
	r := repo.Open(dir, cfg)
	if err := r.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(r.PeopleDir()); got != "people" {
		t.Fatalf("bad people dir: %s", r.PeopleDir())
	}
	return r
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{G: 255, A: 255})
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

func stringIn(values []string, want string) bool {
	return slices.Contains(values, want)
}
