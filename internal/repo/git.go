package repo

import (
	"context"

	"github.com/openclaw/crawlkit/mirror"
)

func (r Repo) MirrorOptions() mirror.Options {
	return mirror.Options{RepoPath: r.Path, Remote: r.Config.Git.Remote, Branch: r.Config.Git.Branch}
}

func (r Repo) Pull(ctx context.Context) error {
	return mirror.PullCurrent(ctx, r.MirrorOptions())
}

func (r Repo) Push(ctx context.Context) error {
	return mirror.Push(ctx, r.MirrorOptions())
}

func (r Repo) Commit(ctx context.Context, message string) (bool, error) {
	return mirror.Commit(ctx, r.MirrorOptions(), message)
}

func (r Repo) Dirty(ctx context.Context) (bool, error) {
	return mirror.Dirty(ctx, r.MirrorOptions())
}
