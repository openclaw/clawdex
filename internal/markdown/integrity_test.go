package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"go.yaml.in/yaml/v3"
)

func TestPreserveUnknownFrontmatter(t *testing.T) {
	for _, kind := range []string{"person", "note"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), kind+".md")
			unknown := "nickname: Countess\naliases: [Ada, Augusta]\ncustom: {enabled: true, nested: [1, two]}\n"
			original := "---\nid: stable\nname: Ada\n" + unknown + "---\nPrivate prose\n"
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			var encoded []byte
			if kind == "person" {
				p, _, err := ReadPerson(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePerson(path, p); err != nil {
					t.Fatal(err)
				}
				encoded, err = json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				n, _, err := ReadNote(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := WriteNote(path, n); err != nil {
					t.Fatal(err)
				}
				encoded, err = json.Marshal(n)
				if err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			front, body, ok := splitFrontmatter(data)
			if !ok || !strings.Contains(body, "Private prose") {
				t.Fatalf("lost body: %s", data)
			}
			var got, want map[string]any
			if err := yaml.Unmarshal([]byte(front), &got); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(unknown), &want); err != nil {
				t.Fatal(err)
			}
			for key, value := range want {
				if !reflect.DeepEqual(got[key], value) {
					t.Errorf("%s: got %#v want %#v", key, got[key], value)
				}
			}
			if strings.Contains(string(encoded), "Countess") {
				t.Fatalf("unknown metadata leaked into JSON: %s", encoded)
			}
		})
	}
}

func TestValidYAMLWithMissingIdentityNeedsRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "person.md")
	if err := os.WriteFile(path, []byte("---\nname: Ada\n---\n# Ada\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, report, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Needed {
		t.Fatal("missing identity and timestamps were not reported")
	}
	if err := RepairPerson(path, filepath.Join(t.TempDir(), "repairs"), p, report, true); err != nil {
		t.Fatal(err)
	}
	got, next, err := ReadPerson(path)
	if err != nil {
		t.Fatal(err)
	}
	if next.Needed || got.ID != p.ID || !got.CreatedAt.Equal(p.CreatedAt) {
		t.Fatalf("repair did not persist inferred fields: %#v %#v", got, next)
	}
}

func TestRepairBackupsRetainEveryOriginalAtSameTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "person.md")
		repairs := filepath.Join(t.TempDir(), "repairs")
		for _, original := range []string{"first original", "second original"} {
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := backupOriginal(path, repairs); err != nil {
				t.Fatal(err)
			}
		}
		contents := map[string]bool{}
		err := filepath.WalkDir(repairs, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				contents[string(data)] = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(contents) != 2 || !contents["first original"] || !contents["second original"] {
			t.Fatalf("backups overwritten: %v", contents)
		}
	})
}
