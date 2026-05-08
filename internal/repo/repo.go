package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openclaw/crawlkit/mirror"
)

type Repo struct {
	Path   string
	Config Config
}

func Open(path string, cfg Config) Repo {
	return Repo{Path: path, Config: cfg}
}

func (r Repo) Init(ctx context.Context) error {
	if strings.TrimSpace(r.Path) == "" {
		return errors.New("repo path is required")
	}
	for _, dir := range []string{
		filepath.Join(r.Path, "people"),
		filepath.Join(r.Path, "index"),
		filepath.Join(r.Path, ".clawdex", "repairs"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := mirror.EnsureRepo(ctx, mirror.Options{RepoPath: r.Path, Branch: r.Config.Git.Branch}); err != nil {
		return err
	}
	if err := writeDataConfig(filepath.Join(r.Path, "clawdex.toml"), r.Config); err != nil {
		return err
	}
	if strings.TrimSpace(r.Config.Git.Remote) != "" {
		if err := mirror.EnsureRemote(ctx, mirror.Options{RepoPath: r.Path, Remote: r.Config.Git.Remote, Branch: r.Config.Git.Branch}); err != nil {
			return err
		}
	}
	return nil
}

func (r Repo) Require() error {
	if strings.TrimSpace(r.Path) == "" {
		return errors.New("repo path is required")
	}
	if _, err := os.Stat(filepath.Join(r.Path, "people")); err != nil {
		return fmt.Errorf("contacts repo not initialized at %s; run clawdex init %s", r.Path, r.Path)
	}
	return nil
}

func (r Repo) PeopleDir() string {
	return filepath.Join(r.Path, "people")
}

func (r Repo) IndexDir() string {
	return filepath.Join(r.Path, "index")
}

func (r Repo) RepairDir() string {
	return filepath.Join(r.Path, ".clawdex", "repairs")
}

func writeDataConfig(path string, cfg Config) error {
	data := []byte("version = 1\n")
	if strings.TrimSpace(cfg.Git.Remote) != "" {
		data = append(data, []byte("\n[git]\nremote = \""+escapeTOML(cfg.Git.Remote)+"\"\nbranch = \""+escapeTOML(cfg.Git.Branch)+"\"\n")...)
	}
	return os.WriteFile(path, data, 0o600)
}

func escapeTOML(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "\"", "\\\"")
}
