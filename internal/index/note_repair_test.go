package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotesHonorAutomaticRepairSettings(t *testing.T) {
	for _, auto := range []bool{false, true} {
		for _, backup := range []bool{false, true} {
			t.Run(fmt.Sprintf("auto=%v/backup=%v", auto, backup), func(t *testing.T) {
				s := New(testRepo(t))
				p, err := s.AddPerson("Ada", nil, nil, nil, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				s.Repo.Config.Repair.AutoRepair = auto
				s.Repo.Config.Repair.BackupBeforeRepair = backup
				path := filepath.Join(filepath.Dir(p.Path), "notes", "incomplete.md")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				original := "---\ncustom: [retained]\n---\nOriginal note\n"
				if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
				notes, err := s.Notes(p.ID)
				if err != nil || len(notes) != 1 {
					t.Fatalf("notes=%v err=%v", notes, err)
				}
				if notes[0].PersonID != p.ID {
					t.Fatalf("owner=%q", notes[0].PersonID)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if (string(data) != original) != auto {
					t.Fatalf("auto=%v changed note=%v", auto, string(data) != original)
				}
				if !strings.Contains(string(data), "retained") {
					t.Fatalf("lost metadata: %s", data)
				}
				if auto {
					again, err := s.Notes(p.ID)
					if err != nil || again[0].ID != notes[0].ID {
						t.Fatalf("identity not persisted: %v %v", again, err)
					}
				}
				entries, err := os.ReadDir(s.Repo.RepairDir())
				if err != nil {
					t.Fatal(err)
				}
				if (len(entries) > 0) != (auto && backup) {
					t.Fatalf("unexpected backups: %v", entries)
				}
			})
		}
	}
}

func TestNotesRejectSymlinkSourcesBeforeRepair(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent=%v", parent), func(t *testing.T) {
			s := New(testRepo(t))
			p, err := s.AddPerson("Ada", nil, nil, nil, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			private := filepath.Join(outside, "private.md")
			original := "Synthetic outside note"
			if err := os.WriteFile(private, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			notes := filepath.Join(filepath.Dir(p.Path), "notes")
			if parent {
				if err := os.Symlink(outside, notes); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(notes, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(private, filepath.Join(notes, "private.md")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.Notes(p.ID); err == nil {
				t.Error("symlink source was accepted")
			}
			data, err := os.ReadFile(private)
			if err != nil || string(data) != original {
				t.Fatalf("outside file changed: %q %v", data, err)
			}
			entries, err := os.ReadDir(s.Repo.RepairDir())
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("outside contents copied to backups: %v", entries)
			}
		})
	}
}
