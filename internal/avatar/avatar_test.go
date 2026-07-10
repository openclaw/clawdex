package avatar

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

func TestSetManualValidateAndRepair(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	writePNG(t, src)
	person := model.Person{ID: "person_1", Name: "Ada", Path: filepath.Join(dir, "people", "ada", "person.md")}

	person, err := SetManual(dir, person, src, time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if person.Avatar.Path != "avatars/avatar.png" || person.Avatar.MIME != "image/png" || person.Avatar.Width != 2 || person.Avatar.Height != 1 {
		t.Fatalf("avatar = %#v", person.Avatar)
	}
	if _, err := os.Stat(filepath.Join(dir, "people", "ada", "avatars", "avatar.png")); err != nil {
		t.Fatal(err)
	}
	if problems := Validate(dir, person); len(problems) != 0 {
		t.Fatalf("problems = %#v", problems)
	}
	if problems := Validate(dir, model.Person{Path: person.Path}); len(problems) != 0 {
		t.Fatalf("empty avatar problems = %#v", problems)
	}
	if repaired, changed, err := RepairMetadata(dir, person, time.Now()); err != nil || changed || repaired.Avatar.SHA256 != person.Avatar.SHA256 {
		t.Fatalf("fresh repair changed: repaired=%#v changed=%v err=%v", repaired.Avatar, changed, err)
	}

	person.Avatar.SHA256 = "bad"
	person.Avatar.MIME = "image/jpeg"
	if problems := Validate(dir, person); len(problems) != 2 {
		t.Fatalf("expected stale metadata problems, got %#v", problems)
	}
	person.Avatar.UpdatedAt = time.Time{}
	repaired, changed, err := RepairMetadata(dir, person, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || repaired.Avatar.SHA256 == "bad" || repaired.Avatar.UpdatedAt.IsZero() {
		t.Fatalf("repaired=%v avatar=%#v", changed, repaired.Avatar)
	}
	if _, _, err := RepairMetadata(dir, model.Person{Path: person.Path, Avatar: model.AvatarRef{Path: "missing.png"}}, time.Now()); err == nil {
		t.Fatal("expected missing repair error")
	}
}

func TestImportedAvatarGuardClauses(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	writePNG(t, src)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	source, err := InspectBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	person := model.Person{ID: "person_1", Name: "Ada", Path: filepath.Join(dir, "person.md")}
	person.Avatar = model.AvatarRef{Path: "avatars/avatar.png", Source: "manual", SHA256: "other"}
	if got, changed, err := SetImported(dir, person, source, "apple", time.Now()); err != nil || changed || got.Avatar.Source != "manual" {
		t.Fatalf("manual overwrite guard failed: got=%#v changed=%v err=%v", got, changed, err)
	}
	person.Avatar = model.AvatarRef{}
	got, changed, err := SetImported(dir, person, source, "apple", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Avatar.Source != "apple" {
		t.Fatalf("imported avatar failed: got=%#v changed=%v", got, changed)
	}
	got, changed, err = SetImported(dir, got, source, "apple", time.Now())
	if err != nil || changed {
		t.Fatalf("same imported avatar changed: got=%#v changed=%v err=%v", got, changed, err)
	}
	if cleared := Clear(got); cleared.Avatar.Path != "" {
		t.Fatalf("clear failed: %#v", cleared.Avatar)
	}
	if path, err := AbsolutePath(dir, got); err != nil || filepath.Base(path) != "avatar.png" {
		t.Fatalf("absolute path = %q err=%v", path, err)
	}
	if got, changed, err := SetImported(dir, person, model.SourceAvatar{}, "apple", time.Now()); err != nil || changed || got.ID != person.ID {
		t.Fatalf("empty avatar changed: got=%#v changed=%v err=%v", got, changed, err)
	}
	if _, _, err := SetImported(dir, model.Person{}, source, "apple", time.Now()); err == nil {
		t.Fatal("expected missing person path error")
	}
	if _, err := InspectBytes(nil); err == nil {
		t.Fatal("expected empty data error")
	}
	if problems := Validate(dir, model.Person{Path: filepath.Join(dir, "person.md"), Avatar: model.AvatarRef{Path: "../x"}}); len(problems) != 1 {
		t.Fatalf("expected escaped path problem, got %#v", problems)
	}
	if _, err := InspectFile(filepath.Join(dir, "missing.png")); err == nil {
		t.Fatal("expected missing inspect error")
	}
	if _, err := SetManual(dir, person, filepath.Join(dir, "missing.png"), time.Now()); err == nil {
		t.Fatal("expected missing manual file error")
	}
	fake := model.SourceAvatar{Data: []byte("not really jpeg"), MIME: "image/jpeg", SHA256: "fake-sha"}
	jpegPerson, changed, err := SetImported(dir, model.Person{ID: "person_2", Path: filepath.Join(dir, "jpeg", "person.md")}, fake, "google", time.Now())
	if err != nil || !changed || jpegPerson.Avatar.Path != "avatars/avatar.jpg" {
		t.Fatalf("jpeg extension fallback failed: %#v changed=%v err=%v", jpegPerson.Avatar, changed, err)
	}
	for mime, want := range map[string]string{
		"image/gif":  "avatars/avatar.gif",
		"image/webp": "avatars/avatar.webp",
		"text/plain": "avatars/avatar.img",
	} {
		p, changed, err := SetImported(dir, model.Person{ID: "person_3", Path: filepath.Join(dir, strings.ReplaceAll(mime, "/", "-"), "person.md")}, model.SourceAvatar{Data: []byte("x"), MIME: mime, SHA256: mime}, "test", time.Now())
		if err != nil || !changed || p.Avatar.Path != want {
			t.Fatalf("%s path = %#v changed=%v err=%v", mime, p.Avatar, changed, err)
		}
	}
	blockedDir := filepath.Join(dir, "blocked", "avatars")
	if err := os.MkdirAll(filepath.Dir(blockedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedDir, []byte("not dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SetImported(dir, model.Person{ID: "person_4", Path: filepath.Join(dir, "blocked", "person.md")}, source, "apple", time.Now()); err == nil {
		t.Fatal("expected blocked avatar dir error")
	}
}

func TestImportedAvatarRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(root, "src.png")
	writePNG(t, src)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	source, err := InspectBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	personDir := filepath.Join(root, "people", "ada")
	if err := os.MkdirAll(personDir, 0o755); err != nil {
		t.Fatal(err)
	}
	person := model.Person{ID: "person_1", Name: "Ada", Path: filepath.Join(personDir, "person.md")}

	if err := os.Symlink(outside, filepath.Join(personDir, "avatars")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if changed, err := ValidateImported(root, person, source, "apple"); err == nil || changed || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("import preview parent error: changed=%v err=%v", changed, err)
	}
	if _, err := ValidateManual(root, person, src); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("manual preview parent error = %v", err)
	}
	if _, _, err := SetImported(root, person, source, "apple", time.Now()); err == nil {
		t.Fatal("expected avatar parent symlink rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "avatar.png")); !os.IsNotExist(err) {
		t.Fatalf("parent escape wrote outside: %v", err)
	}
	if err := os.Remove(filepath.Join(personDir, "avatars")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(personDir, "avatars"), 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "outside.png")
	if err := os.WriteFile(outsideFile, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(personDir, "avatars", "avatar.png")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SetImported(root, person, source, "apple", time.Now()); err == nil {
		t.Fatal("expected avatar leaf symlink rejection")
	}
	if got, err := os.ReadFile(outsideFile); err != nil || string(got) != "untouched" {
		t.Fatalf("outside data = %q err=%v", got, err)
	}
	symlinkPerson := person
	symlinkPerson.Avatar = model.AvatarRef{Path: "avatars/avatar.png", MIME: "image/png"}
	if problems := Validate(root, symlinkPerson); len(problems) != 1 {
		t.Fatalf("expected symlink validation problem, got %#v", problems)
	}
	if _, _, err := RepairMetadata(root, symlinkPerson, time.Now()); err == nil {
		t.Fatal("expected symlink repair rejection")
	}
}

func TestManualAvatarRejectsSymlinkSourceLeafAndParent(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	realPath := filepath.Join(sourceDir, "avatar.png")
	writePNG(t, realPath)
	person := model.Person{ID: "person_1", Name: "Ada", Path: filepath.Join(root, "people", "ada", "person.md")}

	leaf := filepath.Join(root, "leaf.png")
	if err := os.Symlink(realPath, leaf); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := SetManual(root, person, leaf, time.Now()); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("leaf source error = %v", err)
	}
	if _, err := ValidateManual(root, person, leaf); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("leaf preview error = %v", err)
	}

	parent := filepath.Join(root, "parent")
	if err := os.Symlink(sourceDir, parent); err != nil {
		t.Fatal(err)
	}
	parentSource := filepath.Join(parent, "avatar.png")
	if _, err := SetManual(root, person, parentSource, time.Now()); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("parent source error = %v", err)
	}
	if _, err := ValidateManual(root, person, parentSource); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("parent preview error = %v", err)
	}
	if _, err := InspectFile(leaf); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("inspect leaf error = %v", err)
	}
}

func writePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{B: 255, A: 255})
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
