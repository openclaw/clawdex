package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openclaw/clawdex/internal/apple"
	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/safefile"
)

func TestAppleAvatarLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		limit    string
		avatars  bool
		wantFile bool
		wantWarn bool
	}{
		{"unlimited default", int(safefile.MaxReadBytes) + 1, "", true, true, false},
		{"explicit unlimited", 5, "0", true, true, false},
		{"exact limit", 4, "4", true, true, false},
		{"over limit", 5, "4", true, false, true},
		{"avatars disabled", 5, "4", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, data := testPaths(t)
			mustAppleCommand(t, cfg, "init", data, "--remote", "")
			payload := bytes.Repeat([]byte("a"), tc.size)
			input := writeAppleFixture(t, []apple.Contact{{Identifier: "a1", FullName: "Ada", AvatarData: payload}})
			args := []string{"import", "apple", "--input", input}
			if tc.avatars {
				args = append(args, "--avatars")
			}
			if tc.limit != "" {
				args = append(args, "--max-avatar-bytes", tc.limit)
			}
			before := snapshotTree(t, data)
			_, warning := mustAppleCommand(t, cfg, append(args, "--dry-run")...)
			if strings.Contains(warning, "skipped incoming avatar") != tc.wantWarn {
				t.Fatalf("dry-run warning = %q", warning)
			}
			if !reflect.DeepEqual(before, snapshotTree(t, data)) {
				t.Fatal("dry-run changed the repository")
			}
			_, warning = mustAppleCommand(t, cfg, args...)
			if strings.Contains(warning, "skipped incoming avatar") != tc.wantWarn {
				t.Fatalf("warning = %q", warning)
			}
			personPath := filepath.Join(data, "people", "ada", "person.md")
			person, _, err := markdown.ReadPerson(personPath)
			if err != nil {
				t.Fatal(err)
			}
			if (person.Avatar.Path != "") != tc.wantFile {
				t.Fatalf("avatar = %#v", person.Avatar)
			}
			if tc.wantFile {
				got, err := os.ReadFile(filepath.Join(filepath.Dir(personPath), person.Avatar.Path))
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("stored avatar differs: %v", err)
				}
			}
		})
	}
}

func TestAppleAvatarLimitPreservesExistingAvatarAndContinues(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(map[bool]string{false: "apple", true: "manual"}[manual], func(t *testing.T) {
			cfg, data := testPaths(t)
			mustAppleCommand(t, cfg, "init", data, "--remote", "")
			input := writeAppleFixture(t, []apple.Contact{{Identifier: "a1", FullName: "Ada", AvatarData: []byte("old")}})
			mustAppleCommand(t, cfg, "import", "apple", "--input", input, "--avatars")
			if manual {
				png := filepath.Join(t.TempDir(), "manual.png")
				writeCLITestPNG(t, png)
				mustAppleCommand(t, cfg, "person", "avatar", "set", "ada", png)
			}
			path := filepath.Join(data, "people", "ada", "person.md")
			before, _, err := markdown.ReadPerson(path)
			if err != nil {
				t.Fatal(err)
			}
			avatarBefore := snapshotTree(t, filepath.Join(filepath.Dir(path), "avatars"))
			input = writeAppleFixture(t, []apple.Contact{
				{Identifier: "a1", FullName: "Ada", Emails: []string{"new@example.com"}, AvatarData: []byte("oversized")},
				{Identifier: "b1", FullName: "Grace", AvatarData: []byte("yes")},
			})
			args := []string{"import", "apple", "--input", input, "--avatars", "--max-avatar-bytes", "3"}
			tree := snapshotTree(t, data)
			mustAppleCommand(t, cfg, append(args, "--dry-run")...)
			if !reflect.DeepEqual(tree, snapshotTree(t, data)) {
				t.Fatal("dry-run changed existing data")
			}
			mustAppleCommand(t, cfg, args...)
			after, _, err := markdown.ReadPerson(path)
			if err != nil || before.Avatar != after.Avatar || len(after.Emails) != 1 || after.Emails[0].Value != "new@example.com" {
				t.Fatalf("contact update or avatar preservation failed: %#v, %v", after, err)
			}
			if !reflect.DeepEqual(avatarBefore, snapshotTree(t, filepath.Join(filepath.Dir(path), "avatars"))) {
				t.Fatal("existing avatar bytes changed")
			}
			grace, _, err := markdown.ReadPerson(filepath.Join(data, "people", "grace", "person.md"))
			if err != nil || grace.Avatar.Path == "" {
				t.Fatalf("subsequent contact missing avatar: %#v, %v", grace, err)
			}
		})
	}
}

func TestAppleAvatarLimitRejectsNegativeBeforeReading(t *testing.T) {
	cfg, _ := testPaths(t)
	err := Execute([]string{"--config", cfg, "import", "apple", "--input", "missing.json", "--max-avatar-bytes=-1"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "must be non-negative") {
		t.Fatalf("negative limit: %v", err)
	}
}

func mustAppleCommand(t *testing.T, cfg string, args ...string) (string, string) {
	t.Helper()
	var out, warnings bytes.Buffer
	if err := Execute(append([]string{"--config", cfg}, args...), &out, &warnings); err != nil {
		t.Fatalf("%v: %v; stderr=%s", args, err, warnings.String())
	}
	return out.String(), warnings.String()
}

func writeAppleFixture(t *testing.T, contacts []apple.Contact) string {
	t.Helper()
	data, err := json.Marshal(contacts)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "contacts.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
