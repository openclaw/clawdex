# People

A *person* is the unit of clawdex. On disk, each person is a folder under
`people/` containing a `person.md` file, an optional `avatars/` folder, and
an optional `notes/` folder. The folder slug is derived from the person's
name — for example `Sally O'Malley` becomes `sally-o-malley`.

## Add

```bash
clawdex person add "Sally O'Malley" \
  --email sally@example.com -e sally.alt@example.com \
  --phone "+1 555 0100" \
  --tag friend --tag dinner-club
```

Flags:

- `--email`, `-e` — repeatable
- `--phone`, `-p` — repeatable
- `--tag`, `-t` — repeatable

`--dry-run` previews the planned slug without writing anything.

## List

```bash
clawdex person list
clawdex person list --query sally
clawdex person list --json | jq '.[].name'
```

`--query` filters by substring match against name, ID, and tags. The default
output is a TSV of `id<TAB>name<TAB>first-email`.

## Show

```bash
clawdex person show sally
clawdex person show sally@example.com
clawdex person show "+15550100"
clawdex person show sally-o-malley   # exact ID
```

`show` accepts an ID, a substring of the name, an email, or a phone number.
The first unambiguous match wins. If multiple people share a key, the
command errors and asks you to be more specific.

## Edit

```bash
clawdex person edit sally
EDITOR=nvim clawdex person edit sally
```

Opens the person's `person.md` in `$EDITOR`, falling back to `code` (Visual
Studio Code) if `EDITOR` is unset. Clawdex re-reads the file on the next
command — your edits are the source of truth.

You can also edit `person.md` directly in your shell, in another editor, or
in a pull request review. The file is plain markdown:

```markdown
---
id: sally-o-malley
name: Sally O'Malley
emails:
  - value: sally@example.com
phones:
  - value: "+15550100"
tags: [friend, dinner-club]
created_at: 2026-05-08T09:15:00Z
updated_at: 2026-05-08T09:15:00Z
---

# Sally O'Malley

Met at the dinner club in 2024. Loves Negronis.
```

If frontmatter gets damaged — a stray quote, a truncated YAML block —
[`clawdex doctor --repair`](doctor.md) salvages what it can and preserves
the body.

## Avatars

Avatars are managed under `clawdex person avatar`. They're a feature on
their own: see [Avatars](avatars.md).

## Bulk import

To populate clawdex from Apple Contacts, Google Contacts, X DMs, or Discord
DMs in one shot, use [Imports](imports.md). Imports project into the same
markdown shape — they don't bypass any of the rules above.

## Related pages

- [Notes](notes.md), [Timeline](timeline.md)
- [Search](search.md)
- [Markdown Storage](markdown-storage.md)
- [Doctor](doctor.md)
