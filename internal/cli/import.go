package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/openclaw/clawdex/internal/apple"
	"github.com/openclaw/clawdex/internal/birdclaw"
	"github.com/openclaw/clawdex/internal/discrawl"
	"github.com/openclaw/clawdex/internal/google"
)

type ImportCmd struct {
	Apple    ImportAppleCmd    `cmd:"" help:"Import Apple Contacts into local markdown"`
	Birdclaw ImportBirdclawCmd `cmd:"" help:"Import X/Twitter DM contacts from local birdclaw archive"`
	Contacts ImportContactsCmd `cmd:"" help:"Import contacts from a source crawler"`
	Google   ImportGoogleCmd   `cmd:"" help:"Import Google Contacts into local markdown"`
	Discrawl ImportDiscrawlCmd `cmd:"" help:"Import Discord DM contacts from local discrawl archive"`
}

type ImportContactsCmd struct {
	From string `name:"from" help:"Crawler binary to import contacts from" required:""`
}

func (c *ImportContactsCmd) Run(r *Runtime) error {
	source, contacts, err := readCrawlerContacts(r.ctx, c.From)
	if err != nil {
		return err
	}
	changes, err := r.store.ImportCrawlerContacts(source, contacts, r.root.DryRun, time.Now())
	if err != nil {
		return err
	}
	return r.print(changes)
}

type ImportAppleCmd struct {
	Input          string `name:"input" help:"JSON/NDJSON contact file instead of macOS Contacts"`
	Avatars        bool   `name:"avatars" help:"Import local avatar thumbnails"`
	MaxAvatarBytes int64  `name:"max-avatar-bytes" help:"Skip incoming Apple avatars larger than this many decoded bytes (0: unlimited)" default:"0"`
}

func (c *ImportAppleCmd) Run(r *Runtime) error {
	if c.MaxAvatarBytes < 0 {
		return errors.New("--max-avatar-bytes must be non-negative")
	}
	var contacts []apple.Contact
	var err error
	if c.Input != "" {
		contacts, err = apple.ReadFile(c.Input)
	} else {
		contacts, err = apple.ReadSystem(r.ctx)
	}
	if err != nil {
		return err
	}
	if c.Avatars && c.MaxAvatarBytes > 0 {
		for i := range contacts {
			if size := int64(len(contacts[i].AvatarData)); size > c.MaxAvatarBytes {
				if _, err := fmt.Fprintf(r.stderr, "warning: skipped incoming avatar for %q: %d bytes exceeds --max-avatar-bytes %d\n", contacts[i].Name(), size, c.MaxAvatarBytes); err != nil {
					return err
				}
				contacts[i].AvatarData = nil
			}
		}
	}
	changes, err := r.store.ImportContacts("apple", apple.ToSourceContacts(contacts, c.Avatars), r.root.DryRun, time.Now())
	if err != nil {
		return err
	}
	return r.print(changes)
}

type ImportGoogleCmd struct {
	Account string `name:"account" help:"Google account email"`
	Avatars bool   `name:"avatars" help:"Fetch Google contact avatar bytes through gog raw photo URLs"`
}

func (c *ImportGoogleCmd) Run(r *Runtime) error {
	account := c.Account
	if account == "" {
		account = r.cfg.Google.DefaultAccount
	}
	contacts, err := (google.GogAdapter{}).ListContactsWithOptions(r.ctx, account, google.Options{IncludeAvatars: c.Avatars})
	if err != nil {
		return err
	}
	changes, err := r.store.ImportContacts("google", contacts, r.root.DryRun, time.Now())
	if err != nil {
		return err
	}
	return r.print(changes)
}

type ImportDiscrawlCmd struct {
	DBPath      string `name:"db" help:"discrawl SQLite database path" default:"~/.discrawl/discrawl.db"`
	MinMessages int    `name:"min-messages" help:"Import DMs with more than this many messages" default:"4"`
}

type ImportBirdclawCmd struct {
	DBPath      string `name:"db" help:"birdclaw SQLite database path" default:"~/.birdclaw/birdclaw.sqlite"`
	MinMessages int    `name:"min-messages" help:"Import DMs with more than this many messages" default:"4"`
}

func (c *ImportBirdclawCmd) Run(r *Runtime) error {
	contacts, err := (birdclaw.Adapter{DBPath: c.DBPath}).ListDMContacts(r.ctx, c.MinMessages)
	if err != nil {
		return err
	}
	changes, err := r.store.ImportContacts("x", contacts, r.root.DryRun, time.Now())
	if err != nil {
		return err
	}
	return r.print(changes)
}

func (c *ImportDiscrawlCmd) Run(r *Runtime) error {
	contacts, err := (discrawl.Adapter{DBPath: c.DBPath}).ListDMContacts(r.ctx, c.MinMessages)
	if err != nil {
		return err
	}
	changes, err := r.store.ImportContacts("discord", contacts, r.root.DryRun, time.Now())
	if err != nil {
		return err
	}
	return r.print(changes)
}
