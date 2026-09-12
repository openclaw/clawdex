package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/model"
)

func (r *Runtime) print(value any) error {
	if r.root.JSON {
		return r.printJSON(value)
	}
	switch v := value.(type) {
	case map[string]any:
		keys := slices.Sorted(maps.Keys(v))
		for _, key := range keys {
			if _, err := fmt.Fprintf(r.stdout, "%s: %v\n", key, v[key]); err != nil {
				return err
			}
		}
		return nil
	case model.Note:
		_, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", v.ID, v.Kind, v.Source, v.Path)
		return err
	case []model.Note:
		return r.printTimeline(v)
	case []model.SearchHit:
		return r.printHits(v)
	case []model.ImportChange:
		for _, change := range v {
			if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", change.Action, change.Name, change.PersonID); err != nil {
				return err
			}
		}
		return nil
	default:
		return r.printJSON(value)
	}
}

func (r *Runtime) printPerson(p model.Person) error {
	if r.root.JSON {
		return r.print(p)
	}
	if r.root.Plain {
		_, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", p.ID, p.Name, p.Path)
		return err
	}
	if _, err := fmt.Fprintf(r.stdout, "id: %s\nname: %s\npath: %s\n", p.ID, p.Name, p.Path); err != nil {
		return err
	}
	for _, email := range p.Emails {
		if _, err := fmt.Fprintf(r.stdout, "email: %s\n", email.Value); err != nil {
			return err
		}
	}
	for _, phone := range p.Phones {
		if _, err := fmt.Fprintf(r.stdout, "phone: %s\n", phone.Value); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) printPeople(people []model.Person) error {
	if r.root.JSON {
		return r.print(people)
	}
	for _, p := range people {
		if r.root.Plain {
			if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", p.ID, p.Name, p.Path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\n", p.ID, p.Name, firstEmail(p)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Runtime) printTimeline(notes []model.Note) error {
	if r.root.JSON {
		return r.print(notes)
	}
	for _, n := range notes {
		if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", n.OccurredAt.Format(time.RFC3339), n.Kind, n.Source, strings.ReplaceAll(n.Body, "\n", " ")); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) printHits(hits []model.SearchHit) error {
	for _, hit := range hits {
		if r.root.Plain {
			if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", hit.Kind, hit.ID, hit.Name, hit.Path); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", hit.Kind, hit.Name, hit.Snippet, hit.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func firstEmail(p model.Person) string {
	if len(p.Emails) == 0 {
		return ""
	}
	return p.Emails[0].Value
}

func (r *Runtime) printJSON(value any) error {
	enc := json.NewEncoder(r.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
