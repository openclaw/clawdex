package repo

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/openclaw/crawlkit/mirror"
)

var ErrRemoteNotConfigured = errors.New("git remote is not configured; run clawdex config set git.remote URL or clawdex init DIR --remote URL")

func (r Repo) MirrorOptions() mirror.Options {
	return mirror.Options{RepoPath: r.Path, Remote: r.Config.Git.Remote, Branch: r.Config.Git.Branch}
}

func (r Repo) Pull(ctx context.Context) error {
	if err := r.requireRemote(ctx); err != nil {
		return err
	}
	return mirror.PullCurrent(ctx, r.MirrorOptions())
}

func (r Repo) Push(ctx context.Context) error {
	if err := r.requireRemote(ctx); err != nil {
		return err
	}
	return mirror.Push(ctx, r.MirrorOptions())
}

func (r Repo) requireRemote(ctx context.Context) error {
	if strings.TrimSpace(r.Config.Git.Remote) != "" {
		return nil
	}
	// #nosec G204 -- git is fixed and repo path is passed as a plain argument.
	cmd := exec.CommandContext(ctx, "git", "-C", r.Path, "remote", "get-url", "origin")
	if err := cmd.Run(); err == nil {
		return nil
	}
	return ErrRemoteNotConfigured
}

func (r Repo) Commit(ctx context.Context, message string) (bool, error) {
	return mirror.Commit(ctx, r.MirrorOptions(), message)
}

func (r Repo) Dirty(ctx context.Context) (bool, error) {
	return mirror.Dirty(ctx, r.MirrorOptions())
}
