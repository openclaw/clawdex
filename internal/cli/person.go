package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openclaw/clawdex/internal/avatar"
)

type PersonCmd struct {
	Add    PersonAddCmd    `cmd:"" help:"Add a person"`
	List   PersonListCmd   `cmd:"" help:"List people"`
	Show   PersonShowCmd   `cmd:"" help:"Show a person"`
	Edit   PersonEditCmd   `cmd:"" help:"Edit a person markdown file"`
	Avatar PersonAvatarCmd `cmd:"" help:"Manage person avatars"`
}

type PersonAddCmd struct {
	Name  string   `arg:"" help:"Person name"`
	Email []string `name:"email" short:"e" help:"Email address"`
	Phone []string `name:"phone" short:"p" help:"Phone number"`
	Tag   []string `name:"tag" short:"t" help:"Tag"`
}

func (c *PersonAddCmd) Run(r *Runtime) error {
	if err := r.repo.Require(); err != nil {
		return err
	}
	if r.root.DryRun {
		return r.print(map[string]any{"would_create": c.Name})
	}
	p, err := r.store.AddPerson(c.Name, c.Email, c.Phone, c.Tag, time.Now())
	if err != nil {
		return err
	}
	return r.printPerson(p)
}

type PersonListCmd struct {
	Query string `name:"query" short:"q" help:"Filter query"`
}

func (c *PersonListCmd) Run(r *Runtime) error {
	people, err := r.store.People()
	if err != nil {
		return err
	}
	if c.Query != "" {
		filtered := people[:0]
		q := strings.ToLower(c.Query)
		for _, p := range people {
			if strings.Contains(strings.ToLower(p.Name+" "+p.ID+" "+strings.Join(p.Tags, " ")), q) {
				filtered = append(filtered, p)
			}
		}
		people = filtered
	}
	return r.printPeople(people)
}

type PersonShowCmd struct {
	Query string `arg:"" help:"ID, name, email, or phone"`
}

func (c *PersonShowCmd) Run(r *Runtime) error {
	p, err := r.store.FindPerson(c.Query)
	if err != nil {
		return err
	}
	return r.printPerson(p)
}

type PersonEditCmd struct {
	Query string `arg:"" help:"ID, name, email, or phone"`
}

func (c *PersonEditCmd) Run(r *Runtime) error {
	p, err := r.store.FindPerson(c.Query)
	if err != nil {
		return err
	}
	if r.root.DryRun {
		return r.print(map[string]any{"dry_run": true, "would_edit": p.Path})
	}
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "code"
	}
	// #nosec G204,G702 -- EDITOR is a deliberate user-controlled executable; no shell is involved.
	cmd := exec.CommandContext(r.ctx, editor, p.Path)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

type PersonAvatarCmd struct {
	Set   PersonAvatarSetCmd   `cmd:"" help:"Set a local avatar image"`
	Show  PersonAvatarShowCmd  `cmd:"" help:"Show avatar metadata"`
	Clear PersonAvatarClearCmd `cmd:"" help:"Clear avatar metadata"`
}

type PersonAvatarSetCmd struct {
	Person string `arg:"" help:"Person query"`
	File   string `arg:"" help:"Image file"`
}

func (c *PersonAvatarSetCmd) Run(r *Runtime) error {
	p, err := r.store.FindPerson(c.Person)
	if err != nil {
		return err
	}
	if r.root.DryRun {
		ref, err := avatar.ValidateManual(r.repo.Path, p, c.File)
		if err != nil {
			return err
		}
		return r.print(map[string]any{"would_set_avatar": p.ID, "mime": ref.MIME, "sha256": ref.SHA256})
	}
	p, err = r.store.SetAvatar(c.Person, c.File, time.Now())
	if err != nil {
		return err
	}
	return r.print(p.Avatar)
}

type PersonAvatarShowCmd struct {
	Person string `arg:"" help:"Person query"`
	Path   bool   `name:"path" help:"Print absolute avatar path only"`
}

func (c *PersonAvatarShowCmd) Run(r *Runtime) error {
	p, err := r.store.FindPerson(c.Person)
	if err != nil {
		return err
	}
	if p.Avatar.Path == "" {
		return fmt.Errorf("%s has no avatar", p.Name)
	}
	if c.Path {
		path, err := avatar.AbsolutePath(r.repo.Path, p)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(r.stdout, path)
		return err
	}
	return r.print(p.Avatar)
}

type PersonAvatarClearCmd struct {
	Person string `arg:"" help:"Person query"`
}

func (c *PersonAvatarClearCmd) Run(r *Runtime) error {
	p, err := r.store.FindPerson(c.Person)
	if err != nil {
		return err
	}
	if r.root.DryRun {
		return r.print(map[string]any{"would_clear_avatar": p.ID})
	}
	p, err = r.store.ClearAvatar(c.Person, time.Now())
	if err != nil {
		return err
	}
	return r.printPerson(p)
}
