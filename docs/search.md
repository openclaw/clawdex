# Search

`clawdex search <query>` finds people and notes that match a substring. It's
a local, offline, full-corpus search — no external service.

```bash
clawdex search dinner
clawdex search sally@example.com
clawdex search +1555
clawdex search "negroni recipe"
clawdex search whatsapp --json
```

## What gets searched

- Person names, IDs, and tags
- Person emails and phone numbers (normalized)
- Note bodies, kinds, sources, and topics

Hits are printed one per line, with the kind, name, a snippet, and the
file path:

```text
person   Sally O'Malley   sally@example.com                 people/sally-o-malley/person.md
note     Sally O'Malley   ...follow up about dinner...      people/sally-o-malley/notes/2026-05-08T09-15-00Z-whatsapp.md
```

`--json` returns a list of `SearchHit` objects; `--plain` swaps the
snippet for the stable ID, which is friendlier to scripts.

## How matching works

The query is a case-insensitive substring match against indexed fields.
For phone numbers the search normalizes both the query and the stored
value (strips spaces, dashes, parentheses, and a leading `+`), so any of
these find Sally:

```bash
clawdex search "+1 555 0100"
clawdex search "(555) 0100"
clawdex search 15550100
```

For emails the match is plain substring against the lowercase value, so
`gmail.com` works as a "find everyone on Gmail" query.

## Combine with timeline and grep

`search` is for finding the right thread; once you've got it, use
[`timeline`](timeline.md) for the full history of that person, or `rg` for
free-form regex on the data repo:

```bash
clawdex search "ankara"
clawdex timeline mehmet
rg -n "ankara" ~/.clawdex/contacts/people
```

## Indexes

Derived indexes live under `index/`:

```text
index/
  emails.json
  phones.json
  handles.json
```

These are rebuilt automatically as the markdown changes. They are
*derivable*, not authoritative — delete the folder and clawdex regenerates
it on the next read. Markdown is canonical; see
[Markdown Storage](markdown-storage.md).

## Related pages

- [People](people.md), [Notes](notes.md), [Timeline](timeline.md)
- [Markdown Storage](markdown-storage.md)
