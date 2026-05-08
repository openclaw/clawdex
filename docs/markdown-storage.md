# Markdown Storage

Clawdex stores everything as files on disk. There is no database, no opaque
binary format, no migration step that locks you out. If clawdex disappeared
tomorrow, you would still have your data, in plaintext, in a Git repo.

## Layout

```text
<repo>/
  clawdex.toml                          # repo-local settings
  people/
    sally-o-malley/
      person.md
      avatars/
        avatar.jpg
      notes/
        2026-05-08T09-15-00Z-whatsapp.md
        2026-05-09T18-02-00Z-imessage.md
      attachments/
        ...                              # opt-in; not yet wired into the CLI
  index/
    emails.json
    phones.json
    handles.json
  .clawdex/
    repairs/                             # backups written by `doctor --repair`
```

Slugs are derived from the person's name and are stable: renaming a person
in `person.md` updates the display name but keeps the folder path. To
rename the slug itself, move the folder by hand and re-run
[`clawdex doctor`](doctor.md).

## `person.md`

```markdown
---
id: sally-o-malley
name: Sally O'Malley
emails:
  - value: sally@example.com
    kind: work
phones:
  - value: "+15550100"
    kind: mobile
tags: [friend, dinner-club]
accounts:
  x: { handle: sally }
  discord: { id: "234234234234234234", username: "sally" }
created_at: 2026-05-08T09:15:00Z
updated_at: 2026-05-08T09:15:00Z
avatar:
  path: avatars/avatar.jpg
  mime: image/jpeg
  sha256: "..."
  source: manual
---

# Sally O'Malley

Met at the dinner club in 2024. Loves Negronis.
```

YAML frontmatter is parsed strictly first; if that fails, clawdex falls
back to a best-effort scalar salvage and copies the original file under
`.clawdex/repairs/` before writing anything new. See
[Doctor](doctor.md).

The Markdown body is preserved verbatim across reads, edits, and repair.
Your hand-written prose is safe.

## Note files

Notes are timestamped markdown files under `notes/`:

```markdown
---
id: 2026-05-08T09-15-00Z-whatsapp
kind: dm
source: whatsapp
occurred_at: 2026-05-08T09:15:00Z
created_at: 2026-05-08T09:15:00Z
topics: [dinner, logistics]
---

Follow up about dinner next Thursday.
```

The filename encodes `occurred_at` as `2006-01-02T15-04-05Z` plus the
source. Sorting by filename and sorting by `occurred_at` produce the same
order, which is intentional — `ls notes/` is a serviceable timeline.

See [Notes](notes.md) and [Timeline](timeline.md).

## Index files

`index/*.json` are derived caches:

- `emails.json` — email → person ID
- `phones.json` — normalized phone → person ID
- `handles.json` — service handle (X, Discord, …) → person ID

Clawdex rebuilds these on read whenever they're stale. They are safe to
delete — the next command will regenerate them. They are *not* the source
of truth: markdown is.

## clawdex.toml

A small repo-local config file written by `clawdex init`:

```toml
[git]
remote = "https://github.com/you/backup-clawdex.git"
branch = "main"

[repair]
backup_before_repair = true
auto_repair = false
```

This is *separate* from the user-level config at `~/.clawdex/config.toml`,
which holds your default repo path, default Google account, and editor
preferences. See [Config](config.md).

## Why markdown

- **Diffable.** A person rename, a tag change, a note edit — they show up
  as readable diffs in `git log`.
- **Editable anywhere.** Your editor, GitHub's web UI, mobile markdown
  editors, plain `vim` over SSH.
- **Greppable.** `rg`, `awk`, and `sed` work on the data repo without
  needing clawdex on the host.
- **Future-proof.** Plain text outlives every CLI built on top of it.

## Related pages

- [People](people.md), [Notes](notes.md), [Avatars](avatars.md)
- [Doctor](doctor.md), [Config](config.md)
- [Git Sync](git-sync.md)
