package cli

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorRepairsNotesAndPreservesOriginal(t *testing.T) {
	cfg, dataRepo := testPaths(t)
	runJSON := func(args ...string) map[string]any {
		t.Helper()
		var out bytes.Buffer
		if err := Execute(append([]string{"--config", cfg, "--json"}, args...), &out, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	runJSON("init", dataRepo)
	person := runJSON("person", "add", "Ada")
	notesDir := filepath.Join(filepath.Dir(person["path"].(string)), "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(notesDir, "damaged.md")
	original := "---\nid: note_stable\nkind: meeting\nsource: manual\ntopics: [broken\n---\nOriginal note body\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, dataRepo)
	preview := runJSON("doctor", "--repair", "--dry-run")
	if preview["notes_repaired"] != float64(1) {
		t.Fatalf("preview = %v", preview)
	}
	if after := snapshotTree(t, dataRepo); !maps.Equal(before, after) {
		t.Fatal("dry run changed note or backup")
	}
	result := runJSON("doctor", "--repair")
	if result["notes_repaired"] != float64(1) {
		t.Fatalf("repair = %v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Original note body") || !strings.Contains(string(data), "Recovered metadata") || !strings.Contains(string(data), "person_id: "+person["id"].(string)) {
		t.Fatalf("repaired note = %s", data)
	}
	found := false
	err = filepath.WalkDir(filepath.Join(dataRepo, ".clawdex", "repairs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			found = found || string(data) == original
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("original note not backed up")
	}
	if got := runJSON("doctor", "--repair")["notes_repaired"]; got != float64(0) {
		t.Fatalf("repeat repair = %v", got)
	}
}
