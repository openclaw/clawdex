package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/markdown"
)

type NoteCmd struct {
	Add  NoteAddCmd  `cmd:"" help:"Add a note"`
	List NoteListCmd `cmd:"" help:"List notes"`
}

type NoteAddCmd struct {
	Person     string   `arg:"" help:"Person query"`
	Kind       string   `name:"kind" required:"" help:"Note kind"`
	Source     string   `name:"source" required:"" help:"Note source"`
	Text       string   `name:"text" help:"Note body"`
	OccurredAt string   `name:"occurred-at" help:"Occurrence time"`
	Topic      []string `name:"topic" help:"Topic"`
}

func (c *NoteAddCmd) Run(r *Runtime) error {
	if c.Text == "" {
		return usageErr{errors.New("--text is required")}
	}
	occurredAt, err := parseOptionalTime(c.OccurredAt)
	if err != nil {
		return err
	}
	n := markdown.NewNote("", c.Kind, c.Source, c.Text, occurredAt, time.Now(), c.Topic)
	if r.root.DryRun {
		p, err := r.store.FindPerson(c.Person)
		if err != nil {
			return err
		}
		n.PersonID = p.ID
		return r.print(n)
	}
	n, err = r.store.AddNote(c.Person, n)
	if err != nil {
		return err
	}
	return r.print(n)
}

type NoteListCmd struct {
	Person string `arg:"" help:"Person query"`
}

func (c *NoteListCmd) Run(r *Runtime) error {
	notes, err := r.store.Notes(c.Person)
	if err != nil {
		return err
	}
	return r.print(notes)
}

type TimelineCmd struct {
	Person string `arg:"" help:"Person query"`
}

func (c *TimelineCmd) Run(r *Runtime) error {
	notes, err := r.store.Notes(c.Person)
	if err != nil {
		return err
	}
	return r.printTimeline(notes)
}

type SearchCmd struct {
	Query string `arg:"" help:"Search query"`
}

func (c *SearchCmd) Run(r *Runtime) error {
	hits, err := r.store.Search(c.Query)
	if err != nil {
		return err
	}
	return r.print(hits)
}

func parseOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, usageErr{fmt.Errorf("invalid time %q", value)}
}
