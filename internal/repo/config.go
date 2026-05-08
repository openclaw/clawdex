package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	crawlconfig "github.com/openclaw/crawlkit/config"
)

const (
	DefaultConfigEnv = "CLAWDEX_CONFIG"
	RepoEnv          = "CLAWDEX_REPO"
	DefaultRemote    = "https://github.com/steipete/backup-clawdex.git"
)

type Config struct {
	Version  int       `toml:"version" json:"version"`
	RepoPath string    `toml:"repo_path" json:"repo_path"`
	Git      GitConfig `toml:"git" json:"git"`
	Repair   Repair    `toml:"repair" json:"repair"`
	Google   Google    `toml:"google" json:"google"`
	Apple    Apple     `toml:"apple" json:"apple"`
}

type GitConfig struct {
	Remote   string `toml:"remote" json:"remote"`
	Branch   string `toml:"branch" json:"branch"`
	AutoPull bool   `toml:"auto_pull" json:"auto_pull"`
	AutoPush bool   `toml:"auto_push" json:"auto_push"`
}

type Repair struct {
	AutoRepair         bool `toml:"auto_repair" json:"auto_repair"`
	BackupBeforeRepair bool `toml:"backup_before_repair" json:"backup_before_repair"`
}

type Google struct {
	DefaultAccount string `toml:"default_account" json:"default_account"`
	Adapter        string `toml:"adapter" json:"adapter"`
}

type Apple struct {
	Enabled bool `toml:"enabled" json:"enabled"`
}

var appConfig = crawlconfig.App{Name: "clawdex", ConfigEnv: DefaultConfigEnv, BaseDir: "~/.clawdex"}

func DefaultConfig() Config {
	paths, err := appConfig.DefaultPaths()
	if err != nil {
		home, _ := os.UserHomeDir()
		paths.ShareDir = filepath.Join(home, ".clawdex", "contacts")
	}
	return Config{
		Version:  1,
		RepoPath: paths.ShareDir,
		Git: GitConfig{
			Remote: DefaultRemote,
			Branch: "main",
		},
		Repair: Repair{
			AutoRepair:         true,
			BackupBeforeRepair: true,
		},
		Google: Google{Adapter: "gog"},
		Apple:  Apple{Enabled: true},
	}
}

func ResolveConfigPath(flagPath string) string {
	path, err := appConfig.ResolveConfigPath(flagPath)
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".clawdex", "config.toml")
	}
	return path
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if _, err := os.Stat(crawlconfig.ExpandHome(path)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.Normalize()
			return cfg, nil
		}
		return Config{}, err
	}
	if err := crawlconfig.LoadTOML(path, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Normalize()
	return cfg, nil
}

func WriteConfig(path string, cfg Config) error {
	cfg.Normalize()
	return crawlconfig.WriteTOML(path, cfg, 0o600)
}

func (c *Config) Normalize() {
	def := DefaultConfig()
	if c.Version == 0 {
		c.Version = 1
	}
	if strings.TrimSpace(c.RepoPath) == "" {
		c.RepoPath = def.RepoPath
	}
	c.RepoPath = crawlconfig.ExpandHome(c.RepoPath)
	if strings.TrimSpace(c.Git.Remote) == "" {
		c.Git.Remote = def.Git.Remote
	}
	if strings.TrimSpace(c.Git.Branch) == "" {
		c.Git.Branch = "main"
	}
	if strings.TrimSpace(c.Google.Adapter) == "" {
		c.Google.Adapter = "gog"
	}
}

func ResolveRepoPath(flagRepo string, cfg Config) (string, error) {
	switch {
	case strings.TrimSpace(flagRepo) != "":
		return crawlconfig.ExpandHome(flagRepo), nil
	case strings.TrimSpace(os.Getenv(RepoEnv)) != "":
		return crawlconfig.ExpandHome(os.Getenv(RepoEnv)), nil
	case strings.TrimSpace(cfg.RepoPath) != "":
		return crawlconfig.ExpandHome(cfg.RepoPath), nil
	default:
		return "", errors.New("contacts repo not configured; run clawdex init DIR or pass --repo DIR")
	}
}
