package apple

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeJSONArrayAndNDJSON(t *testing.T) {
	for _, input := range []string{
		`[{"identifier":"a1","full_name":"Ada Lovelace","emails":["ada@example.com"],"phones":["+1 555 0100"]}]`,
		"{\"identifier\":\"a1\",\"first_name\":\"Ada\",\"last_name\":\"Lovelace\",\"emails\":[\"ada@example.com\"]}\n",
	} {
		contacts, err := Decode(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(contacts) != 1 || contacts[0].Name() != "Ada Lovelace" {
			t.Fatalf("contacts = %#v", contacts)
		}
		src := contacts[0].SourceContact(false)
		if src.Source != "apple" || src.ExternalID != "a1" || src.Name != "Ada Lovelace" {
			t.Fatalf("source = %#v", src)
		}
	}
}

func TestReadFileAndToSourceContacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contacts.ndjson")
	encoded := base64.StdEncoding.EncodeToString([]byte("avatar"))
	if err := os.WriteFile(path, []byte("{\"full_name\":\"Ada\",\"emails\":[\"ada@example.com\"],\"avatar_data\":\""+encoded+"\"}\n{\"phones\":[\"+1\"]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	contacts, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sources := ToSourceContacts(contacts, true)
	if len(sources) != 1 || sources[0].Name != "Ada" {
		t.Fatalf("sources = %#v", sources)
	}
	if sources[0].Avatar == nil || string(sources[0].Avatar.Data) != "avatar" {
		t.Fatalf("avatar source = %#v", sources[0].Avatar)
	}
}

func TestDecodeEmptyAndInvalid(t *testing.T) {
	contacts, err := Decode(strings.NewReader(" \n"))
	if err != nil || len(contacts) != 0 {
		t.Fatalf("contacts=%#v err=%v", contacts, err)
	}
	if _, err := Decode(strings.NewReader("{bad")); err == nil {
		t.Fatal("expected invalid json error")
	}
	if _, err := ReadFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected read file error")
	}
}

func TestDecodeLargeAvatarLine(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 128*1024))
	input := "{\"identifier\":\"a1\",\"full_name\":\"Ada\",\"emails\":[\"ada@example.com\"],\"avatar_data\":\"" + encoded + "\"}\n"
	contacts, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || len(contacts[0].AvatarData) != 128*1024 {
		t.Fatalf("contacts = %#v", contacts)
	}
}
