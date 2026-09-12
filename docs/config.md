# Config

Clawdex loads settings from `~/.clawdex/config.toml`. Override that path with
`--config PATH` or `CLAWDEX_CONFIG=PATH`.

```toml
version = 1
repo_path = "/Users/you/.clawdex/contacts"

[git]
remote = "https://github.com/you/backup-clawdex.git"
branch = "main"

[repair]
auto_repair = true
backup_before_repair = true

[google]
default_account = "you@gmail.com"
adapter = "gog"

[apple]
enabled = true
```

The remote defaults to empty: local use needs no backup service. Automatic
person and note frontmatter repair and original-file backups are enabled by default.

`init` also writes `<repo>/clawdex.toml`: a version marker, plus Git settings
when a remote is configured. This file is descriptive; commands currently
read settings only from the user-level config. After cloning a data repo,
configure its path and remote in your user-level config.

## Show and edit

```bash
clawdex config
clawdex config show --json
clawdex config set repo_path ~/.clawdex/contacts
clawdex config set git.remote https://github.com/you/backup-clawdex.git
clawdex config set git.branch main
clawdex config set google.default_account you@gmail.com
```

Those four keys are supported by `config set`. Edit repair settings directly
in the user-level TOML file. `--dry-run` previews the resolved config without
writing. `init DIR --remote URL` saves the selected repo and remote there
unless `--no-config` is supplied; an omitted remote retains existing config.

The serialized `git.auto_pull`, `git.auto_push`, `google.adapter`, and
`apple.enabled` settings are reserved. Commands do not use them to enable,
disable, or schedule operations; Google imports use `gog`.

## Per-run overrides

```bash
clawdex --config /tmp/alt.toml person list
clawdex --repo /tmp/scratch person list
CLAWDEX_REPO=/tmp/scratch clawdex import apple --dry-run
```

`--repo` takes precedence over `CLAWDEX_REPO` and the configured repo path.
These selectors do not change the saved configuration.

## Global flags

| Flag | Effect |
| --- | --- |
| `--config PATH` | Select config; environment: `CLAWDEX_CONFIG`. |
| `--repo DIR` | Select data repo; environment: `CLAWDEX_REPO`. |
| `--json` | JSON objects or arrays for structured commands. |
| `--plain` | Stable text output, including TSV for people, notes, and search. |
| `--dry-run`, `-n` | Preview without changing local Clawdex or Git state. |
| `--no-input` | Reserved noninteractive flag. |
| `--verbose`, `-v` | Reserved diagnostic flag. |
| `--version` | Print version and exit. |

`--dry-run` applies to initialization, config, people, notes, imports, file
exports, Git helpers, doctor, and automatic frontmatter repair. It does not
launch an editor. `export vcard -o -` still emits vCard bytes; import adapters
still read their sources.

`git status`, `export vcard -o -`, and `person avatar show --path` stream
their native text even with `--json`. Help and version output are also text.

`person edit` launches the executable named by `EDITOR`, falling back to
`code`. It does not interpret shell syntax or split an editor command line.

## Related pages

- [Quickstart](quickstart.md), [Markdown Storage](markdown-storage.md)
- [Git Sync](git-sync.md), [Doctor](doctor.md)
