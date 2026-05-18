# Config

Clawdex has two config files, on purpose:

| File                          | Scope         | What lives here                              |
|-------------------------------|---------------|----------------------------------------------|
| `~/.clawdex/config.toml`      | User-level    | Default repo path, default Google account.   |
| `<repo>/clawdex.toml`         | Repo-local    | Git remote, branch, repair behavior.         |

The user-level file follows you across data repos. The repo-local file
follows the data — it ships with the repo when you clone it on a second
machine.

## User-level config

Default location: `~/.clawdex/config.toml`. Override with `--config PATH`
or `CLAWDEX_CONFIG=PATH`.

```toml
repo_path = "/Users/you/.clawdex/contacts"

[google]
default_account = "you@gmail.com"
```

## Repo-local config

Default location: `<repo>/clawdex.toml`. Created by `clawdex init`.

```toml
[git]
remote = "https://github.com/you/backup-clawdex.git"
branch = "main"

[repair]
backup_before_repair = true
auto_repair = false
```

## `clawdex config`

```bash
clawdex config              # default subcommand: show
clawdex config show
clawdex config show --json
clawdex config set repo_path ~/.clawdex/contacts
clawdex config set git.remote https://github.com/you/backup-clawdex.git
clawdex config set git.branch main
clawdex config set google.default_account you@gmail.com
```

`clawdex init DIR --remote URL` also writes `git.remote` during initial
setup. Without `--remote`, contacts stay local-only until a remote is
configured.

`clawdex config set` writes the user-level config file. `--dry-run`
echoes the resolved config without writing.

Supported keys:

- `repo_path`
- `git.remote`
- `git.branch`
- `google.default_account`

Other keys are edited by hand in the appropriate TOML file.

## Per-run overrides

Every config value can be overridden for a single command:

```bash
clawdex --config /tmp/alt.toml person list
clawdex --repo /tmp/scratch person add "Test Person"
CLAWDEX_REPO=/tmp/scratch clawdex import apple --dry-run
```

These don't persist anything — they're for one-off runs, CI jobs, and
testing.

## Global flags

These apply to every subcommand:

| Flag             | Env                | Effect                                                  |
|------------------|--------------------|---------------------------------------------------------|
| `--config PATH`  | `CLAWDEX_CONFIG`   | Override the user-level config path.                    |
| `--repo DIR`     | `CLAWDEX_REPO`     | Override the contacts data repo for this run.           |
| `--json`         |                    | Stable JSON envelope on stdout.                         |
| `--plain`        |                    | TSV on stdout (script-friendly).                        |
| `--dry-run`, `-n`|                    | Preview without writing.                                |
| `--no-input`     |                    | Never prompt; useful in CI.                             |
| `--verbose`, `-v`|                    | Verbose diagnostics on stderr.                          |
| `--version`      |                    | Print version and exit.                                 |

## Environment

- `CLAWDEX_CONFIG` — user-level config path.
- `CLAWDEX_REPO` — contacts data repo path.
- `EDITOR` — used by `clawdex person edit`. Falls back to `code` (Visual
  Studio Code) when unset.

## Related pages

- [Quickstart](quickstart.md)
- [Markdown Storage](markdown-storage.md)
- [Git Sync](git-sync.md)
- [Doctor](doctor.md)
