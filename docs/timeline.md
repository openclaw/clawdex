# Timeline

`clawdex timeline <person>` prints every note for one person, sorted by
`occurred_at`. It's the fastest way to remember what's been going on with
someone.

```bash
clawdex timeline sally
clawdex timeline sally --json
clawdex timeline sally@example.com --plain
```

Default output is a TSV:

```text
2026-04-12T19:30:00Z   meeting   inperson   Drinks at Bar Centrale
2026-04-22T08:01:00Z   dm        whatsapp   Sent recipe link
2026-05-08T09:15:00Z   dm        whatsapp   Follow up about dinner
```

`--json` returns the full note objects, including bodies, topics, and IDs.
`--plain` is a simpler TSV intended for `awk`/`cut` pipelines.

## Resolving the person

The argument is the same query string that
[`clawdex person show`](people.md) accepts: an ID slug, a name substring,
an email, or a phone number. The first unambiguous match wins.

## Reading flow

Pair `timeline` with `search` for a quick "what was that about" loop:

```bash
clawdex search "negroni"               # find the conversation
clawdex timeline sally | head -20      # surrounding context
```

Or pipe into `less` for long histories:

```bash
clawdex timeline sally | column -t -s $'\t' | less -S
```

## Limits

- Sort key is `occurred_at`, not file mtime. Edit `occurred_at` in the note
  frontmatter to fix ordering after the fact.
- Only one person at a time. To get a multi-person timeline, use
  [Search](search.md) with a date-ish term, or grep notes directly:

  ```bash
  rg -n "occurred_at: 2026-05" ~/.clawdex/contacts/people
  ```

## Related pages

- [Notes](notes.md), [People](people.md), [Search](search.md)
- [Markdown Storage](markdown-storage.md)
