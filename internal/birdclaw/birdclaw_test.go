package birdclaw

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
[{"conversation_id":"1-2","profile_id":"2","handle":"ada","display_name":"Ada Lovelace","title":"ada","messages":5}]
JSON
`
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (Adapter{DBPath: "~/birdclaw.sqlite", Binary: bin}).ListDMContacts(t.Context(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Name != "Ada Lovelace" {
		t.Fatalf("contacts = %#v", contacts)
	}
	accounts := contacts[0].Accounts["x"]
	if len(accounts) != 3 || accounts[1] != "@ada" || accounts[2] != "user:2" {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestListDMContactsFallbacksAndFilters(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
cat <<'JSON'
[{"conversation_id":"c1","profile_id":"","handle":"handle","display_name":"","title":"","messages":8},{"conversation_id":"","handle":"skip","messages":8},{"conversation_id":"c2","handle":"","display_name":"","title":"  ","messages":8}]
JSON
`), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (Adapter{Binary: bin}).ListDMContacts(t.Context(), -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Name != "handle" {
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

func TestListDMContactsErrors(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "sqlite3")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho locked >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Adapter{Binary: bin}).ListDMContacts(t.Context(), 4); err == nil || !strings.Contains(err.Error(), "locked") {
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

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Fatalf("got = %q", got)
	}
}
