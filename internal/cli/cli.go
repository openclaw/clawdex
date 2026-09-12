package cli

import (
	"context"
	"errors"
	"io"
	"runtime/debug"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/openclaw/clawdex/internal/index"
	"github.com/openclaw/clawdex/internal/repo"
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
		kong.Vars{"version": effectiveVersion()},
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
	dataCfg := cfg
	if root.DryRun {
		dataCfg.Repair.AutoRepair = false
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
		repo:       repo.Open(repoPath, dataCfg),
	}
	r.store = index.New(r.repo)
	kctx.Bind(r)
	return kctx.Run(r)
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if _, ok := errors.AsType[usageErr](err); ok {
		return 2
	}
	return 1
}

type usageErr struct{ error }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func effectiveVersion() string {
	info, ok := debug.ReadBuildInfo()
	return resolveVersion(Version, info, ok)
}

func resolveVersion(linked string, info *debug.BuildInfo, ok bool) string {
	linked = strings.TrimSpace(linked)
	if linked != "" && linked != "dev" && linked != "(devel)" {
		return strings.TrimPrefix(linked, "v")
	}
	if ok && info != nil {
		version := strings.TrimSpace(info.Main.Version)
		if version != "" && version != "(devel)" {
			return strings.TrimPrefix(version, "v")
		}
	}
	if linked == "" || linked == "(devel)" {
		return "dev"
	}
	return linked
}
