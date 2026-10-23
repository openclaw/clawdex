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

Directory slugs are derived from the person's name. IDs are separate
`person_<UUID>` values stored in frontmatter. Renaming a person
in `person.md` updates the display name but keeps the folder path. To
rename the slug itself, move the folder by hand and re-run
[`clawdex doctor`](doctor.md).

## `person.md`

```markdown
---
id: person_01234567-89ab-4cde-8f01-23456789abcd
name: Sally O'Malley
emails:
  - value: sally@example.com
    label: work
phones:
  - value: "+15550100"
    label: mobile
tags: [friend, dinner-club]
accounts:
  x: ["@sally"]
  discord: ["user:234234234234234234"]
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

Unknown frontmatter fields, including nested mappings and lists, survive
updates to people and notes. They remain in Markdown and are not added to
CLI JSON output.

Markdown prose is retained; serialization normalizes CRLF line endings,
removes leading blank lines, and adds a final newline.

Person and note files, generated indexes, and repair backups must be ordinary
files beneath the contacts repository. Clawdex rejects symbolic links in their
paths and uses atomic replacement for writes. The repository root itself may
use a platform alias, such as macOS `/var`.

New person and note files are created exclusively, so concurrent `person add`,
`note add`, and imports never replace each other's files. Clawdex publishes a
new file with a hard link where the filesystem supports one. Otherwise it
takes an exclusive hidden lock file beside the destination, such as
`.person.md.create-lock`, and atomically renames the complete file into place
only while the destination is still absent, so hard-link support is not
required and no placeholder ever appears at the destination. Clawdex never
loads lock files. One left behind by an interrupted command only makes later
creations use the next free name, and you can delete it once no clawdex
command is running.

If an existing repository uses linked person files, replace each link with a
regular file containing only the contact data you intend to archive. Inspect
the link target before copying it. Remove linked index files; the next person
creation or import regenerates them, and reads work without indexes. Replace
linked storage or repair directories with real directories inside the repository.
Remove the links themselves, leaving their targets intact. Then run
`clawdex doctor --repair --dry-run` to check the contact files.

## Note files

Notes are timestamped markdown files under `notes/`:

```markdown
---
id: note_01234567-89ab-4cde-8f01-23456789abcd
person_id: person_01234567-89ab-4cde-8f01-23456789abcd
kind: dm
source: whatsapp
occurred_at: 2026-05-08T09:15:00Z
captured_at: 2026-05-08T09:15:00Z
topics: [dinner, logistics]
---

Follow up about dinner next Thursday.
```

The filename encodes `occurred_at` as `2006-01-02T15-04-05Z` plus the
note kind. Sorting by filename and sorting by `occurred_at` produce the same
order, which is intentional — `ls notes/` is a serviceable timeline.

See [Notes](notes.md) and [Timeline](timeline.md).

## Index files

`index/*.json` are derived caches:

- `emails.json` — email → person ID
- `phones.json` — normalized phone → person ID
- `handles.json` — service handle (X, Discord, …) → person ID

Person creation and completed imports rebuild these files. Reads and search
load Markdown directly, so deleting an index does not prevent lookup. The
next person creation or import regenerates it.

## clawdex.toml

A small repo-local config file written by `clawdex init`:

```toml
version = 1

[git]
remote = "https://github.com/you/backup-clawdex.git"
branch = "main"
```

The Git section is omitted without a configured remote. This file is not
loaded as configuration: active settings live in `~/.clawdex/config.toml`.
See [Config](config.md).

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
