# Git Sync

Your contacts data repo is just a Git repo. Clawdex doesn't run a sync
daemon, doesn't talk to a clawdex.sh server, and doesn't store anything
about you remotely. Backup and multi-device sync are 100% Git.

The default suggested remote is a *private* GitHub repo:

```text
https://github.com/<you>/backup-clawdex.git
```

You set this once in [Quickstart](quickstart.md) with:

```bash
clawdex config set git.remote https://github.com/<you>/backup-clawdex.git
```

## `clawdex git` commands

All four commands run *inside the data repo*, not the clawdex source repo.
They're thin wrappers around `git` so you don't have to `cd` constantly.

### Status

```bash
clawdex git status
```

A wrapper for `git -C <repo> status --short --branch`. Same output, same
exit codes. Use this before committing to see what an import or edit
changed.

### Commit

```bash
clawdex git commit
clawdex git commit -m "sync: import google contacts"
```

Stages everything modified under the data repo and commits with the
provided message. The default message is
`sync: update clawdex contacts`. Returns whether a commit was actually
created — if there were no changes, `committed: false`.

### Pull

```bash
clawdex git pull
```

Pulls from the configured remote on the configured branch. Resolve
conflicts the way you'd resolve them in any other repo.

### Push

```bash
clawdex git push
```

Pushes to the configured remote on the configured branch. The first push
on a fresh repo also sets the upstream.

## Choosing a remote

Anything Git-hostable works:

- **Private GitHub repo** — recommended; integrates with GitHub Mobile if
  you want to read your notes on a phone.
- **Self-hosted** — Forgejo, Gitea, sourcehut, a bare repo over SSH.
- **Local-only** — leave `git.remote` unset and clawdex won't push. The
  repo is still version-controlled locally.

Whatever you pick, **the remote should be private**. The data is plain
markdown — names, phone numbers, emails, conversation snippets. Treat it
like a journal.

## Multi-device

The flow on a second machine is the same as on the first:

```bash
brew install steipete/tap/clawdex
git clone https://github.com/<you>/backup-clawdex.git ~/.clawdex/contacts
clawdex config set repo_path ~/.clawdex/contacts
clawdex config set git.remote https://github.com/<you>/backup-clawdex.git
clawdex git pull
clawdex doctor
```

Day-to-day:

```bash
clawdex git pull
# ... edit, import, add notes ...
clawdex git commit -m "sync: ..."
clawdex git push
```

Conflicts are normal Git conflicts. Markdown frontmatter merges
predictably; if a frontmatter merge ends up malformed, run
[`clawdex doctor --repair`](doctor.md) to salvage it.

## Encryption

vanilla `clawdex git` does **not** encrypt the repo. The data lives as
plaintext markdown both locally and on the remote. If you need encryption
at rest:

- Use a private remote you trust.
- Or layer `git-crypt` / `age` over the data repo manually.
- Or back up encrypted snapshots out-of-band, alongside Git.

A built-in encrypted backup mode (à la `gog backup`) is on the roadmap
but not shipped.

## Related pages

- [Quickstart](quickstart.md), [Config](config.md)
- [Markdown Storage](markdown-storage.md)
