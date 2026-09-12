package cli

import (
	"fmt"

	"github.com/openclaw/clawdex/internal/repo"
)

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
	if r.root.DryRun {
		return r.print(map[string]any{"config_path": r.configPath, "dry_run": true, "remote": cfg.Git.Remote, "would_initialize": cfg.RepoPath})
	}
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
	cfg.Normalize()
	if r.root.DryRun {
		return r.print(cfg)
	}
	if err := repo.WriteConfig(r.configPath, cfg); err != nil {
		return err
	}
	return r.print(map[string]any{"config_path": r.configPath, "set": c.Key})
}
