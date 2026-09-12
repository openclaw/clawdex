package vcard

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/safefile"
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
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{}); err != nil {
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
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PHOTO:data:image/png;base64,") {
		t.Fatalf("missing photo: %s", buf.String())
	}
	person.Avatar.MIME = ""
	buf.Reset()
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PHOTO:data:application/octet-stream;base64,") {
		t.Fatalf("missing default photo mime: %s", buf.String())
	}
	person.Avatar.Path = filepath.Join(dir, "avatars", "avatar.png")
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: dir}); err == nil {
		t.Fatal("expected absolute avatar path error")
	}
	person.Avatar.Path = "../avatar.png"
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: dir}); err == nil {
		t.Fatal("expected escaped avatar path error")
	}
	person.Avatar.Path = "avatars/missing.png"
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: dir}); err == nil {
		t.Fatal("expected missing avatar error")
	}
}

func TestWriteWithAvatarRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	personDir := filepath.Join(root, "people", "ada")
	if err := os.MkdirAll(filepath.Join(personDir, "avatars"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(personDir, "avatars", "avatar.png")
	if err := os.Symlink(secret, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	person := model.Person{
		ID:   "person_1",
		Name: "Ada",
		Path: filepath.Join(personDir, "person.md"),
		Avatar: model.AvatarRef{
			Path: "avatars/avatar.png",
			MIME: "image/png",
		},
	}
	var buf bytes.Buffer
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: root}); err == nil {
		t.Fatal("expected avatar leaf symlink rejection")
	}
	if err := os.RemoveAll(filepath.Join(personDir, "avatars")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(personDir, "avatars")); err != nil {
		t.Fatal(err)
	}
	if err := WriteWithOptions(&buf, []model.Person{person}, Options{IncludeAvatars: true, RepoRoot: root}); err == nil {
		t.Fatal("expected avatar parent symlink rejection")
	}
}

func TestWriteFileIsPrivateAtomicAndAnchorsSymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.vcf")
	people := []model.Person{{ID: "p", Name: "Ada"}}
	if err := WriteFile(path, people, Options{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := WriteFile(path, []model.Person{{ID: "p2", Name: "Grace"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "FN:Grace") {
		t.Fatalf("data = %q err=%v", data, err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WriteFile(path, people, Options{}); err == nil {
		t.Fatal("expected output symlink rejection")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "untouched" {
		t.Fatalf("outside = %q err=%v", got, err)
	}

	linkedParent := filepath.Join(dir, "linked-parent")
	outsideDir := t.TempDir()
	if err := os.Symlink(outsideDir, linkedParent); err != nil {
		t.Fatal(err)
	}
	linkedOutput := filepath.Join(linkedParent, "contacts.vcf")
	if err := ValidateOutputPath(linkedOutput); err != nil {
		t.Fatalf("parent alias validation = %v", err)
	}
	if err := WriteFile(linkedOutput, people, Options{}); err != nil {
		t.Fatalf("parent alias write = %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(outsideDir, "contacts.vcf")); err != nil || !strings.Contains(string(got), "FN:Ada") {
		t.Fatalf("resolved output = %q err=%v", got, err)
	}

	anchoredRoot, relative, err := validatedOutput(filepath.Join(linkedParent, "anchored.vcf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = anchoredRoot.Close() }()
	retargetedDir := t.TempDir()
	if err := os.Remove(linkedParent); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(retargetedDir, linkedParent); err != nil {
		t.Fatal(err)
	}
	if err := safefile.AtomicWriteRoot(anchoredRoot, relative, 0o600, func(w io.Writer) error {
		return WriteWithOptions(w, people, Options{})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outsideDir, "anchored.vcf")); err != nil {
		t.Fatalf("anchored output missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(retargetedDir, "anchored.vcf")); !os.IsNotExist(err) {
		t.Fatalf("retargeted parent received output: %v", err)
	}
}

func TestWriteFileStreamsAtomicallyAndRejectsDirectoryForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contacts.vcf")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	people := []model.Person{
		{ID: "ok", Name: "Written first"},
		{
			ID:   "bad",
			Name: "Missing avatar",
			Path: filepath.Join(dir, "bad", "person.md"),
			Avatar: model.AvatarRef{
				Path: "avatars/missing.png",
			},
		},
	}
	if err := WriteFile(path, people, Options{IncludeAvatars: true, RepoRoot: dir}); err == nil {
		t.Fatal("expected streaming render error")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatalf("data = %q err=%v", data, err)
	}

	unrelated := filepath.Join(dir, filepath.Base(dir))
	if err := os.WriteFile(unrelated, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(dir+string(filepath.Separator), nil, Options{}); err == nil {
		t.Fatal("expected trailing-separator path rejection")
	}
	if err := ValidateOutputPath(dir + string(filepath.Separator)); err == nil {
		t.Fatal("expected dry output validation to reject directory form")
	}
	data, err = os.ReadFile(unrelated)
	if err != nil || string(data) != "unrelated" {
		t.Fatalf("unrelated data = %q err=%v", data, err)
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

func TestFoldedInvalidUTF8DoesNotHang(t *testing.T) {
	line := "\xff" + strings.Repeat("a", 80)
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- folded(&buf, line)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("folded hung on a line that starts with invalid UTF-8")
	}
	out := buf.String()
	if !strings.Contains(out, "\r\n ") {
		t.Fatalf("not folded: %q", out)
	}
	unfolded := strings.ReplaceAll(strings.TrimSuffix(out, "\r\n"), "\r\n ", "")
	if unfolded != line {
		t.Fatalf("unfolded = %q want %q", unfolded, line)
	}
}

func TestFoldedUTF8KeepsRuneBoundaries(t *testing.T) {
	for _, line := range []string{strings.Repeat("é", 50), strings.Repeat("世", 40)} {
		var buf bytes.Buffer
		if err := folded(&buf, line); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !utf8.ValidString(out) {
			t.Fatalf("folded split a rune: %q", out)
		}
		if !strings.Contains(out, "\r\n ") {
			t.Fatalf("not folded: %q", out)
		}
		unfolded := strings.ReplaceAll(strings.TrimSuffix(out, "\r\n"), "\r\n ", "")
		if unfolded != line {
			t.Fatalf("unfolded = %q want %q", unfolded, line)
		}
	}
}

func TestWriteLongInvalidUTF8Name(t *testing.T) {
	var buf bytes.Buffer
	person := model.Person{
		ID:   "person_1",
		Name: "\xff" + strings.Repeat("a", 80),
	}
	done := make(chan error, 1)
	go func() {
		done <- WriteWithOptions(&buf, []model.Person{person}, Options{})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Write hung on a long name that starts with invalid UTF-8")
	}
	unfolded := strings.ReplaceAll(buf.String(), "\r\n ", "")
	if !strings.Contains(unfolded, "FN:\xff"+strings.Repeat("a", 80)) {
		t.Fatalf("missing FN remainder: %q", buf.String())
	}
}

func TestWriteSkipsEmptyValuesAndEmptyList(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWithOptions(&buf, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buf = %q", buf.String())
	}
	err := WriteWithOptions(errWriter{}, []model.Person{{ID: "p", Name: "A"}}, Options{})
	if err == nil {
		t.Fatal("expected writer error")
	}
	buf.Reset()
	if err := WriteWithOptions(&buf, []model.Person{{ID: "p", Name: "Solo", Emails: []model.ContactValue{{}}, Phones: []model.ContactValue{{}}, Tags: []string{"one", "two"}}}, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "EMAIL") || strings.Contains(buf.String(), "TEL") || !strings.Contains(buf.String(), "CATEGORIES:one,two") {
		t.Fatalf("vcard = %s", buf.String())
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestCategoriesEscapeEachValue(t *testing.T) {
	var out bytes.Buffer
	p := model.Person{ID: "p", Name: "Ada", Tags: []string{"friend", "work, east", "semi;colon", `slash\`}}
	if err := WriteWithOptions(&out, []model.Person{p}, Options{}); err != nil {
		t.Fatal(err)
	}
	want := `CATEGORIES:friend,work\, east,semi\;colon,slash\\` + "\r\n"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("categories = %q, want %q", out.String(), want)
	}
}

func TestFoldedPhysicalLinesFitOctetLimit(t *testing.T) {
	for _, line := range []string{strings.Repeat("a", 75), strings.Repeat("a", 76), strings.Repeat("a", 150), strings.Repeat("a", 300), strings.Repeat("世", 100)} {
		var out bytes.Buffer
		if err := folded(&out, line); err != nil {
			t.Fatal(err)
		}
		for physical := range strings.SplitSeq(strings.TrimSuffix(out.String(), "\r\n"), "\r\n") {
			if len(physical) > 75 {
				t.Errorf("physical line has %d octets: %q", len(physical), physical)
			}
		}
		if got := strings.ReplaceAll(strings.TrimSuffix(out.String(), "\r\n"), "\r\n ", ""); got != line {
			t.Fatalf("unfolded changed: %q", got)
		}
	}
}
