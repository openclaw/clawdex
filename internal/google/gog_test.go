package google

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseGogContactsEnvelopeAndArray(t *testing.T) {
	inputs := [][]byte{
		[]byte(`{"contacts":[{"resourceName":"people/c1","etag":"e1","name":"Ada","email":"ada@example.com","phone":"+1 555 0100"}]}`),
		[]byte(`[{"resource":"people/c1","names":[{"displayName":"Ada"}],"emailAddresses":[{"value":"ada@example.com","type":"home"}],"phoneNumbers":[{"value":"+1","type":"mobile"}]}]`),
	}
	for _, input := range inputs {
		contacts, err := parseGogContacts(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(contacts) != 1 || contacts[0].Source != "google" || contacts[0].Name != "Ada" {
			t.Fatalf("contacts = %#v", contacts)
		}
		if contacts[0].ExternalID != "people/c1" || len(contacts[0].Emails) == 0 {
			t.Fatalf("bad contact = %#v", contacts[0])
		}
	}
}

func TestGogAdapterListContactsUsesNoInput(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if runtime.GOOS == "windows" {
		bin += ".bat"
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"" + filepath.Join(dir, "args") + "\"\ncase \"$*\" in *next*) printf '%s\\n' '{\"contacts\":[{\"resourceName\":\"people/c2\",\"name\":\"Grace\"}]}' ;; *) printf '%s\\n' '{\"contacts\":[{\"resourceName\":\"people/c1\",\"name\":\"Ada\"}],\"nextPageToken\":\"next\"}' ;; esac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	contacts, err := (GogAdapter{Binary: bin}).ListContacts(t.Context(), "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 2 {
		t.Fatalf("contacts = %#v", contacts)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--no-input") || !strings.Contains(string(args), "--account") {
		t.Fatalf("args = %s", args)
	}
	if !strings.Contains(string(args), "--page") {
		t.Fatalf("missing page args = %s", args)
	}
}

func TestGogAdapterListContactsCommandFailure(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gog")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho nope >&2\nexit 7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := (GogAdapter{Binary: bin}).ListContacts(t.Context(), "")
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	if _, err := (GogAdapter{Binary: filepath.Join(dir, "missing")}).ListContacts(t.Context(), ""); err == nil {
		t.Fatal("expected missing binary error")
	}
}

func TestParseGogContactsRejectsInvalidJSON(t *testing.T) {
	if _, err := parseGogContacts([]byte(`{`)); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("err = %v", err)
	}
	var p gogPerson
	if err := json.Unmarshal([]byte(`{"names":[{"givenName":"Ada","familyName":"Lovelace"}]}`), &p); err != nil {
		t.Fatal(err)
	}
	got := convertPeople([]gogPerson{p})
	if len(got) != 1 || got[0].Name != "Ada Lovelace" {
		t.Fatalf("got = %#v", got)
	}
}
