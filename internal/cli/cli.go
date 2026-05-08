package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/openclaw/clawdex/internal/apple"
	"github.com/openclaw/clawdex/internal/avatar"
	"github.com/openclaw/clawdex/internal/birdclaw"
	"github.com/openclaw/clawdex/internal/discrawl"
	"github.com/openclaw/clawdex/internal/google"
	"github.com/openclaw/clawdex/internal/index"
	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/model"
	"github.com/openclaw/clawdex/internal/repo"
	"github.com/openclaw/clawdex/internal/vcard"
)

var Version = "dev"

type CLI struct {
	Config  string `name:"config" help:"Config path" env:"CLAWDEX_CONFIG"`
	Repo    string `name:"repo" help:"Contacts data repo path" env:"CLAWDEX_REPO"`
	JSON    bool   `name:"json" help:"Write JSON to stdout"`
	Plain   bool   `name:"plain" help:"Write stable plain text to stdout"`
	DryRun  bool   `name:"dry-run" short:"n" help:"Preview changes without writing"`
	NoInput bool   `name:"no-input" help:"Never prompt"`
	Verbose bool   `name:"verbose" short:"v" help:"Verbose diagnostics"`

	Version kong.VersionFlag `name:"version" help:"Print version and exit"`

	Init     InitCmd     `cmd:"" help:"Initialize a contacts data repo"`
	ConfigC  ConfigCmd   `cmd:"" name:"config" help:"Show or edit clawdex config"`
	Person   PersonCmd   `cmd:"" help:"Manage people"`
	Note     NoteCmd     `cmd:"" help:"Manage notes"`
	Timeline TimelineCmd `cmd:"" help:"Show person timeline"`
	Search   SearchCmd   `cmd:"" help:"Search people and notes"`
	Import   ImportCmd   `cmd:"" help:"Import contacts into local markdown"`
	Sync     SyncCmd     `cmd:"" help:"Preview sync with address books"`
	Export   ExportCmd   `cmd:"" help:"Export contacts"`
	Git      GitCmd      `cmd:"" help:"Run data repo git helpers"`
	Doctor   DoctorCmd   `cmd:"" help:"Check repo health"`
}

type Runtime struct {
	ctx        context.Context
	stdout     io.Writer
	stderr     io.Writer
	root       *CLI
	configPath string
	cfg        repo.Config
	repo       repo.Repo
	store      index.Store
}

func Execute(args []string, stdout, stderr io.Writer) error {
	var root CLI
	parser, err := kong.New(&root,
		kong.Name("clawdex"),
		kong.Description("Personal contact index backed by markdown and private Git."),
		kong.UsageOnError(),
		kong.Writers(stdout, stderr),
		kong.Vars{"version": Version},
	)
	if err != nil {
		return err
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		return usageErr{err}
	}
	configPath := repo.ResolveConfigPath(root.Config)
	cfg, err := repo.LoadConfig(configPath)
	if err != nil {
		return err
	}
	repoPath, err := repo.ResolveRepoPath(root.Repo, cfg)
	if err != nil {
		repoPath = cfg.RepoPath
	}
	r := &Runtime{
		ctx:        context.Background(),
		stdout:     stdout,
		stderr:     stderr,
		root:       &root,
		configPath: configPath,
		cfg:        cfg,
		repo:       repo.Open(repoPath, cfg),
	}
	r.store = index.New(r.repo)
	kctx.Bind(r)
	if err := kctx.Run(r); err != nil {
		return err
	}
	return nil
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage usageErr
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

type usageErr struct{ error }

type InitCmd struct {
	Dir      string `arg:"" optional:"" help:"Contacts data repo directory"`
	Remote   string `name:"remote" help:"Git remote for contacts backup"`
	NoConfig bool   `name:"no-config" help:"Do not write app config"`
}

func (c *InitCmd) Run(r *Runtime) error {
	cfg := r.cfg
	if c.Dir != "" {
		cfg.RepoPath = c.Dir
	}
	if c.Remote != "" {
		cfg.Git.Remote = c.Remote
	}
	cfg.Normalize()
	dataRepo := repo.Open(cfg.RepoPath, cfg)
	if err := dataRepo.Init(r.ctx); err != nil {
		return err
	}
	if !c.NoConfig {
		if err := repo.WriteConfig(r.configPath, cfg); err != nil {
			return err
		}
	}
	return r.print(map[string]any{"repo_path": cfg.RepoPath, "remote": cfg.Git.Remote, "config_path": r.configPath})
}

type ConfigCmd struct {
	Show ConfigShowCmd `cmd:"" default:"1" help:"Show config"`
	Set  ConfigSetCmd  `cmd:"" help:"Set config value"`
}

type ConfigShowCmd struct{}

func (c *ConfigShowCmd) Run(r *Runtime) error {
	return r.print(r.cfg)
}

type ConfigSetCmd struct {
	Key   string `arg:"" help:"Config key"`
	Value string `arg:"" help:"Config value"`
}

func (c *ConfigSetCmd) Run(r *Runtime) error {
	cfg := r.cfg
	switch c.Key {
	case "repo_path":
		cfg.RepoPath = c.Value
	case "git.remote":
		cfg.Git.Remote = c.Value
	case "git.branch":
		cfg.Git.Branch = c.Value
	case "google.default_account":
		cfg.Google.DefaultAccount = c.Value
	default:
		return usageErr{fmt.Errorf("unsupported config key %q", c.Key)}
	}
	if r.root.DryRun {
		return r.print(cfg)
	}
	if err := repo.WriteConfig(r.configPath, cfg); err != nil {
		return err
	}
	return r.print(map[string]any{"config_path": r.configPath, "set": c.Key})
}

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
		ref, err := avatar.InspectFile(c.File)
		if err != nil {
			return err
		}
		ref.Path = "avatars/avatar"
		ref.Source = "manual"
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
		path, err := avatar.AbsolutePath(p)
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

type ImportCmd struct {
	Apple    ImportAppleCmd    `cmd:"" help:"Import Apple Contacts into local markdown"`
	Birdclaw ImportBirdclawCmd `cmd:"" help:"Import X/Twitter DM contacts from local birdclaw archive"`
	Google   ImportGoogleCmd   `cmd:"" help:"Import Google Contacts into local markdown"`
	Discrawl ImportDiscrawlCmd `cmd:"" help:"Import Discord DM contacts from local discrawl archive"`
}

type ImportAppleCmd struct {
	Input   string `name:"input" help:"JSON/NDJSON contact file instead of macOS Contacts"`
	Avatars bool   `name:"avatars" help:"Import local avatar thumbnails"`
}

func (c *ImportAppleCmd) Run(r *Runtime) error {
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
	if c.Out == "-" {
		return vcard.WriteWithOptions(r.stdout, people, vcard.Options{IncludeAvatars: c.IncludeAvatars})
	}
	f, err := os.Create(c.Out)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := vcard.WriteWithOptions(f, people, vcard.Options{IncludeAvatars: c.IncludeAvatars}); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
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
	// #nosec G204 -- git is fixed and repo path is passed as a plain argument.
	cmd := exec.CommandContext(r.ctx, "git", "-C", r.repo.Path, "status", "--short", "--branch")
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

type GitPullCmd struct{}

func (c *GitPullCmd) Run(r *Runtime) error {
	return r.repo.Pull(r.ctx)
}

type GitPushCmd struct{}

func (c *GitPushCmd) Run(r *Runtime) error {
	return r.repo.Push(r.ctx)
}

type GitCommitCmd struct {
	Message string `name:"message" short:"m" help:"Commit message" default:"sync: update clawdex contacts"`
}

func (c *GitCommitCmd) Run(r *Runtime) error {
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
	dirty, _ := r.repo.Dirty(r.ctx)
	result := map[string]any{
		"config_path": r.configPath,
		"repo_path":   r.repo.Path,
		"remote":      r.cfg.Git.Remote,
		"people":      len(people),
		"git_dirty":   dirty,
	}
	avatarProblems := 0
	for _, p := range people {
		avatarProblems += len(avatar.Validate(p))
	}
	if avatarProblems > 0 {
		result["avatar_problems"] = avatarProblems
	}
	if c.Repair {
		var repaired int
		var avatarRepaired int
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
			if len(avatar.Validate(loaded)) > 0 {
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
		result["dry_run"] = r.root.DryRun
	}
	return r.print(result)
}

func (r *Runtime) print(value any) error {
	if r.root.JSON {
		enc := json.NewEncoder(r.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
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
		enc := json.NewEncoder(r.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
