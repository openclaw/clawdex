package vcard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/clawdex/internal/model"
)

func TestWriteVCard(t *testing.T) {
	var buf bytes.Buffer
	person := model.Person{
		ID:     "person_1",
		Name:   "Ada Lovelace",
		Tags:   []string{"math"},
		Emails: []model.ContactValue{{Value: "ada@example.com", Label: "home"}},
		Phones: []model.ContactValue{{Value: "+1 555 0100", Label: "mobile"}},
	}
	if err := Write(&buf, []model.Person{person}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"BEGIN:VCARD", "UID:person_1", "FN:Ada Lovelace", "EMAIL;TYPE=home:ada@example.com", "TEL;TYPE=mobile:+1 555 0100", "NOTE:clawdex:person_1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}

func TestWriteVCardWithAvatar(t *testing.T) {
	dir := t.TempDir()
	avatarPath := filepath.Join(dir, "avatars", "avatar.png")
	if err := os.MkdirAll(filepath.Dir(avatarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(avatarPath, []byte("pngish"), 0o600); err != nil {
		t.Fatal(err)
	}
	person := model.Person{
		ID:   "person_1",
		Name: "Ada Lovelace",
		Path: filepath.Join(dir, "person.md"),
		Avatar: model.AvatarRef{
			Path: "avatars/avatar.png",
			MIME: "image/png",
		},
	}
	var buf bytes.Buffer
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PHOTO:data:image/png;base64,") {
		t.Fatalf("missing photo: %s", buf.String())
	}
	person.Avatar.MIME = ""
	buf.Reset()
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PHOTO:data:application/octet-stream;base64,") {
		t.Fatalf("missing default photo mime: %s", buf.String())
	}
	person.Avatar.Path = filepath.Join(dir, "avatars", "avatar.png")
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true}); err == nil {
		t.Fatal("expected absolute avatar path error")
	}
	person.Avatar.Path = "../avatar.png"
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true}); err == nil {
		t.Fatal("expected escaped avatar path error")
	}
	person.Avatar.Path = "avatars/missing.png"
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true}); err == nil {
		t.Fatal("expected missing avatar error")
	}
}

func TestVCardHelpers(t *testing.T) {
	if got := structuredName(model.Person{Name: "Ada Lovelace"}); got != "Lovelace;Ada;;;" {
		t.Fatalf("structured = %q", got)
	}
	if got := structuredName(model.Person{}); got != ";;;;" {
		t.Fatalf("empty structured = %q", got)
	}
	if got := typeParam("Mobile Phone!"); got != ";TYPE=mobilephone" {
		t.Fatalf("type = %q", got)
	}
	if got := typeParam("!!!"); got != "" {
		t.Fatalf("invalid type = %q", got)
	}
	if got := escape("a,b;c\\d\ne"); got != `a\,b\;c\\d\ne` {
		t.Fatalf("escape = %q", got)
	}
	var buf bytes.Buffer
	if err := folded(&buf, strings.Repeat("a", 90)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\r\n ") {
		t.Fatalf("not folded: %q", buf.String())
	}
}

func TestWriteSkipsEmptyValuesAndEmptyList(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buf = %q", buf.String())
	}
	err := Write(errWriter{}, []model.Person{{ID: "p", Name: "A"}})
	if err == nil {
		t.Fatal("expected writer error")
	}
	buf.Reset()
	if err := Write(&buf, []model.Person{{ID: "p", Name: "Solo", Emails: []model.ContactValue{{}}, Phones: []model.ContactValue{{}}, Tags: []string{"one", "two"}}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "EMAIL") || strings.Contains(buf.String(), "TEL") || !strings.Contains(buf.String(), "CATEGORIES:one\\,two") {
		t.Fatalf("vcard = %s", buf.String())
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
