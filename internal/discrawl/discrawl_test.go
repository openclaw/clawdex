package discrawl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDMContactsViaSQLiteAdapter(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "sqlite3")
	script := `#!/bin/sh
case "$*" in
  *'count(m.id) > 4'*) ;;
  *) echo "missing threshold" >&2; exit 2 ;;
esac
cat <<'JSON'
[{"channel_id":"c1","name":"Ada DM","messages":5,"first_message":"2026-01-01T00:00:00Z","last_message":"2026-01-02T00:00:00Z","counterpart_id":"u1"}]
JSON
`
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (Adapter{DBPath: "~/discrawl.db", Binary: bin}).ListDMContacts(t.Context(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Name != "Ada DM" || contacts[0].Accounts["discord"][1] != "user:u1" {
		t.Fatalf("contacts = %#v", contacts)
	}
}

func TestListDMContactsErrors(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho locked >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Adapter{Binary: bin}).ListDMContacts(t.Context(), 0); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err = %v", err)
	}
	bad := filepath.Join(t.TempDir(), "sqlite3-bad")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\necho not-json\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Adapter{Binary: bad}).ListDMContacts(t.Context(), 4); err == nil {
		t.Fatal("expected json error")
	}
}

func TestListDMContactsEmptyAndFilters(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '[{\"channel_id\":\"\",\"name\":\"Skip\",\"messages\":8},{\"channel_id\":\"c2\",\"name\":\"  \",\"messages\":8}]'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (Adapter{Binary: bin}).ListDMContacts(t.Context(), -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 0 {
		t.Fatalf("contacts = %#v", contacts)
	}
	empty := filepath.Join(t.TempDir(), "sqlite3-empty")
	if err := os.WriteFile(empty, []byte("#!/bin/sh\nprintf '  '\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err = (Adapter{Binary: empty}).ListDMContacts(t.Context(), 4)
	if err != nil || len(contacts) != 0 {
		t.Fatalf("contacts=%#v err=%v", contacts, err)
	}
}
