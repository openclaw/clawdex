package cli

import (
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/openclaw/clawdex/internal/avatar"
	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/vcard"
)

type SyncCmd struct {
	Apple  SyncAppleCmd  `cmd:"" help:"Preview Apple Contacts sync"`
	Google SyncGoogleCmd `cmd:"" help:"Preview Google Contacts sync"`
}

type SyncAppleCmd struct{}

func (c *SyncAppleCmd) Run(r *Runtime) error {
	return r.print(map[string]any{"dry_run": true, "status": "remote writes not implemented yet; use import apple for local markdown projection"})
}

type SyncGoogleCmd struct {
	Account string `name:"account" help:"Google account email"`
}

func (c *SyncGoogleCmd) Run(r *Runtime) error {
	return r.print(map[string]any{"dry_run": true, "account": firstNonEmpty(c.Account, r.cfg.Google.DefaultAccount), "status": "remote writes not implemented yet; use import google for local markdown projection"})
}

type ExportCmd struct {
	VCard ExportVCardCmd `cmd:"" name:"vcard" help:"Export vCard"`
}

type ExportVCardCmd struct {
	Person         string `name:"person" help:"Person query"`
	All            bool   `name:"all" help:"Export all people"`
	IncludeAvatars bool   `name:"include-avatars" help:"Include avatar PHOTO fields"`
	Out            string `name:"out" short:"o" required:"" help:"Output .vcf path, or - for stdout"`
}

func (c *ExportVCardCmd) Run(r *Runtime) error {
	var people []model.Person
	switch {
	case c.All:
		var err error
		people, err = r.store.People()
		if err != nil {
			return err
		}
	case c.Person != "":
		p, err := r.store.FindPerson(c.Person)
		if err != nil {
			return err
		}
		people = []model.Person{p}
	default:
		return usageErr{errors.New("provide --person or --all")}
	}
	opts := vcard.Options{IncludeAvatars: c.IncludeAvatars, RepoRoot: r.repo.Path}
	if c.Out == "-" {
		return vcard.WriteWithOptions(r.stdout, people, opts)
	}
	if r.root.DryRun {
		if err := vcard.ValidateOutputPath(c.Out); err != nil {
			return err
		}
		if err := vcard.WriteWithOptions(io.Discard, people, opts); err != nil {
			return err
		}
		return r.print(map[string]any{"dry_run": true, "out": c.Out, "would_export": len(people)})
	}
	if err := vcard.WriteFile(c.Out, people, opts); err != nil {
		return err
	}
	return r.print(map[string]any{"exported": len(people), "out": c.Out})
}

type GitCmd struct {
	Status GitStatusCmd `cmd:"" default:"1" help:"Show git status"`
	Pull   GitPullCmd   `cmd:"" help:"Pull data repo"`
	Push   GitPushCmd   `cmd:"" help:"Push data repo"`
	Commit GitCommitCmd `cmd:"" help:"Commit data repo changes"`
}

type GitStatusCmd struct{}

func (c *GitStatusCmd) Run(r *Runtime) error {
	args := []string{"-C", r.repo.Path, "status", "--short", "--branch"}
	if r.root.DryRun {
		args = append([]string{"-c", "core.fsmonitor=false", "--no-optional-locks"}, args...)
	}
	cmd := exec.CommandContext(r.ctx, "git", args...) // #nosec G204 -- git is fixed and repo path is passed as a plain argument.
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

type GitPullCmd struct{}

func (c *GitPullCmd) Run(r *Runtime) error {
	if r.root.DryRun {
		if err := r.repo.ValidateRemote(r.ctx); err != nil {
			return err
		}
		return r.print(map[string]any{"branch": r.repo.Config.Git.Branch, "dry_run": true, "would_pull": true})
	}
	return r.repo.Pull(r.ctx)
}

type GitPushCmd struct{}

func (c *GitPushCmd) Run(r *Runtime) error {
	if r.root.DryRun {
		if err := r.repo.ValidateRemote(r.ctx); err != nil {
			return err
		}
		return r.print(map[string]any{"branch": r.repo.Config.Git.Branch, "dry_run": true, "would_push": true})
	}
	return r.repo.Push(r.ctx)
}

type GitCommitCmd struct {
	Message string `name:"message" short:"m" help:"Commit message" default:"sync: update clawdex contacts"`
}

func (c *GitCommitCmd) Run(r *Runtime) error {
	if r.root.DryRun {
		dirty, err := r.repo.DirtyReadOnly(r.ctx)
		if err != nil {
			return err
		}
		return r.print(map[string]any{"dry_run": true, "message": c.Message, "would_commit": dirty})
	}
	committed, err := r.repo.Commit(r.ctx, c.Message)
	if err != nil {
		return err
	}
	return r.print(map[string]any{"committed": committed})
}

type DoctorCmd struct {
	Repair bool `name:"repair" help:"Repair damaged markdown frontmatter"`
}

func (c *DoctorCmd) Run(r *Runtime) error {
	store := r.store
	if c.Repair {
		store.Repo.Config.Repair.AutoRepair = false
	}
	people, err := store.People()
	if err != nil {
		return err
	}
	dirty, _ := r.repo.DirtyReadOnly(r.ctx)
	result := map[string]any{
		"config_path": r.configPath,
		"repo_path":   r.repo.Path,
		"remote":      r.cfg.Git.Remote,
		"people":      len(people),
		"git_dirty":   dirty,
	}
	avatarProblems := 0
	for _, p := range people {
		avatarProblems += len(avatar.Validate(r.repo.Path, p))
	}
	if avatarProblems > 0 {
		result["avatar_problems"] = avatarProblems
	}
	if c.Repair {
		var repaired int
		var avatarRepaired int
		var notesRepaired int
		for _, p := range people {
			loaded, report, err := markdown.ReadPerson(p.Path)
			if err != nil {
				return err
			}
			if report.Needed {
				repaired++
				if !r.root.DryRun {
					if err := markdown.RepairPerson(p.Path, r.repo.RepairDir(), loaded, report, r.cfg.Repair.BackupBeforeRepair); err != nil {
						return err
					}
				}
			}
			count, err := store.RepairNotes(loaded, r.root.DryRun)
			if err != nil {
				return err
			}
			notesRepaired += count
			if len(avatar.Validate(r.repo.Path, loaded)) > 0 {
				avatarRepaired++
				if !r.root.DryRun {
					_, _, err := store.RepairAvatarMetadata(loaded, time.Now())
					if err != nil {
						return err
					}
				}
			}
		}
		result["repaired"] = repaired
		result["avatar_repaired"] = avatarRepaired
		result["notes_repaired"] = notesRepaired
		result["dry_run"] = r.root.DryRun
	}
	return r.print(result)
}
