package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearchNormalizesPhoneQueries(t *testing.T) {
	s := New(testRepo(t))
	if _, err := s.AddPerson("Ada", nil, []string{"+1 (555) 0100", "+1 222 3333"}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"15550100", "(555) 0100", "+1-555-0100", "0015550100"} {
		hits, err := s.Search(query)
		if err != nil || len(hits) != 1 || hits[0].Name != "Ada" {
			t.Errorf("%q: hits=%v err=%v", query, hits, err)
		}
	}
	for _, query := range []string{"ticket555", "01001222", "+---"} {
		hits, err := s.Search(query)
		if err != nil || len(hits) != 0 {
			t.Errorf("%q: unrelated hits=%v err=%v", query, hits, err)
		}
	}
}

func TestPeopleReportsUnreadablePerson(t *testing.T) {
	r := testRepo(t)
	dir := filepath.Join(r.PeopleDir(), "loop")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "person.md")
	if err := os.Symlink("person.md", path); err != nil {
		t.Fatal(err)
	}
	if _, err := New(r).People(); err == nil || !strings.Contains(err.Error(), "person.md") {
		t.Fatalf("unreadable person was hidden: %v", err)
	}
}
