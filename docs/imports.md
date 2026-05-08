# Imports

Imports project an external contact graph into the same markdown shape
clawdex uses everywhere else. They are **local-only**: every import only
writes to your data repo. No address book on Apple, Google, X, or Discord
is mutated by any `clawdex import` subcommand.

If you want to push changes back into Apple Contacts or Google Contacts, see
[Sync](#sync-preview-only) below — it's preview-only today.

## What gets written

Each import returns a list of `ImportChange` rows, printed as TSV:

```text
add        Sally O'Malley   sally-o-malley
update     Bo Burnham       bo-burnham
unchanged  Frank Booth      frank-booth
```

Combine with `--dry-run` to preview without writing:

```bash
clawdex import apple --dry-run
clawdex import google --account you@gmail.com --dry-run
```

## Apple Contacts

```bash
clawdex import apple --dry-run
clawdex import apple
clawdex import apple --avatars
clawdex import apple --input ~/Desktop/contacts.json
```

- Default source is the macOS Contacts database via `Contacts.framework`.
  The first run prompts for *Contacts* access in System Settings.
- `--input PATH` reads JSON or NDJSON instead — useful on Linux, in CI,
  or when round-tripping a snapshot.
- `--avatars` imports thumbnail bytes. Without it, only structured fields
  are imported.

Manual avatars set with [`clawdex person avatar set`](avatars.md) are never
overwritten. Tags, notes, and any custom frontmatter you've added by hand
are preserved.

## Google Contacts

```bash
clawdex import google --account you@gmail.com --dry-run
clawdex import google --account you@gmail.com
clawdex import google --account you@gmail.com --avatars
```

The Google adapter shells out to [`gog`](https://github.com/steipete/gogcli),
the local-first Google Workspace CLI. You need to be authenticated there
first:

```bash
gog auth credentials ~/Downloads/client_secret_*.json
gog auth add you@gmail.com --services contacts
```

If `--account` is omitted, clawdex falls back to `google.default_account`
from your config — set it once with:

```bash
clawdex config set google.default_account you@gmail.com
```

`--avatars` fetches photo bytes through `gog contacts raw --person-fields photos`
and stores them locally.

## Birdclaw — X / Twitter DMs

```bash
clawdex import birdclaw --dry-run
clawdex import birdclaw --min-messages 4
clawdex import birdclaw --db ~/.birdclaw/birdclaw.sqlite
```

Reads from your local [birdclaw](https://github.com/steipete/birdclaw)
SQLite archive. For each DM thread above the `--min-messages` threshold,
clawdex creates or updates a person, stores the X handle as a stable
pointer under `accounts.x`, and adds a source-specific tag.

The default DB path is `~/.birdclaw/birdclaw.sqlite`. Threads with fewer
than `--min-messages` messages are skipped — most of those are one-shot
spam or intros that died.

## Discrawl — Discord DMs

```bash
clawdex import discrawl --dry-run
clawdex import discrawl --min-messages 4
clawdex import discrawl --db ~/.discrawl/discrawl.db
```

Same shape as birdclaw, but reads from
[discrawl](https://github.com/steipete/discrawl)'s SQLite cache. Discord
handles land under `accounts.discord`.

## Sync (preview-only)

```bash
clawdex sync apple
clawdex sync google --account you@gmail.com
```

These commands exist as placeholders. They report:

```text
status: remote writes not implemented yet; use import apple for local
markdown projection
```

Two-way sync requires a conflict report you can read before anything is
written remotely; that report doesn't exist yet, so the writes don't
either. Until it lands, treat clawdex as a one-way mirror: imports come
in, [vCard export](vcard-export.md) goes out.

## Related pages

- [People](people.md), [Avatars](avatars.md)
- [Markdown Storage](markdown-storage.md) — the shape imports project into
- [Git Sync](git-sync.md) — committing the import diff
- [Config](config.md) — `google.default_account`, repo path
