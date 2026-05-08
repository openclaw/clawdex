# Notes

A *note* is a timestamped event attached to a person. Notes capture what
contacts can't: what you talked about, when, and where it happened.

Notes live under `people/<slug>/notes/`. Each note is a single markdown file
named `<occurred-at>-<source>.md`, e.g.
`2026-05-08T09-15-00Z-whatsapp.md`.

**Notes are local-only.** They are never written to Apple Contacts, Google
Contacts, or anywhere else. They are pushed only to your private backup
remote when you run [`clawdex git push`](git-sync.md).

## Add

```bash
clawdex note add sally \
  --kind dm \
  --source whatsapp \
  --text "Follow up about dinner next Thursday" \
  --topic dinner --topic logistics
```

Flags:

- `--kind` *(required)* — type of interaction: `dm`, `call`, `meeting`,
  `email`, `event`, etc. Free-form; clawdex doesn't enforce a vocabulary.
- `--source` *(required)* — where it happened: `whatsapp`, `imessage`,
  `discord`, `x`, `email`, `inperson`, etc.
- `--text` *(required)* — note body. Multiline is fine; quote it.
- `--occurred-at` — ISO 8601 (`2026-05-08T09:15:00Z`), `2026-05-08 09:15`,
  or `2026-05-08`. Defaults to *now*.
- `--topic` — repeatable, becomes the `topics:` frontmatter array.

`--dry-run` shows the resolved note as JSON without writing.

## List

```bash
clawdex note list sally
clawdex note list sally --json
```

The default output is a TSV of `occurred-at<TAB>kind<TAB>source<TAB>body`.
Note bodies are flattened to a single line for TSV; use `--json` to get the
full body.

## Note file format

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

You can edit notes directly with `$EDITOR` — clawdex reads them back. If
the frontmatter gets damaged, [`clawdex doctor --repair`](doctor.md)
salvages known fields and preserves the body verbatim under a *Recovered
metadata* heading.

## Conventions that pay off

- **Use stable `--source` values.** `whatsapp`, not `WhatsApp`. Search and
  imports both rely on lowercase source slugs.
- **Tag topics, not relationships.** Topics are about the *content*; tags
  on the person are about the *role*. A `dinner` topic is fine; a
  `friend` topic is probably a person tag instead.
- **Don't store secrets in notes.** They sync to your private backup repo,
  but they're still in plain markdown on disk. Treat them like a Git repo,
  not a vault.

## Related pages

- [People](people.md), [Timeline](timeline.md), [Search](search.md)
- [Imports](imports.md) — birdclaw and discrawl create one note per imported
  DM thread head.
- [Markdown Storage](markdown-storage.md)
