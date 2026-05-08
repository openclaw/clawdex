# vCard Export

`clawdex export vcard` writes one or more people as RFC 6350 vCards. The
result imports cleanly into Apple Contacts, Google Contacts, iOS Contacts,
Outlook, and most other address books.

This is the *outbound* half of clawdex. [Imports](imports.md) bring the
world in; vCard export sends a curated slice back out.

## Export everything

```bash
clawdex export vcard --all -o contacts.vcf
clawdex export vcard --all --include-avatars -o contacts.vcf
```

Without `--include-avatars`, the file is text-only and small. With it, each
person's avatar is embedded as a base64 `PHOTO;ENCODING=b` payload. A few
hundred avatars adds up — expect a few megabytes.

## Export one person

```bash
clawdex export vcard --person sally -o sally.vcf
clawdex export vcard --person sally@example.com -o sally.vcf
```

The `--person` argument accepts the same query string that
[`clawdex person show`](people.md) accepts: an ID, a name substring, an
email, or a phone number.

## Stream to stdout

`-o -` writes to stdout, so you can pipe directly:

```bash
clawdex export vcard --person sally -o - | pbcopy            # macOS
clawdex export vcard --all       -o - | wl-copy             # wayland
clawdex export vcard --person sally -o - | mail -a contacts.vcf you@example.com
```

## What's in the vCard

Each vCard includes:

- `FN` — display name from `person.md`
- `N` — best-effort surname/given split
- `EMAIL` per email entry, with the original `kind` as a `TYPE` parameter
  when present
- `TEL` per phone entry, with the original `kind` as a `TYPE` parameter
- `NOTE` — short summary from the person body, when present
- `PHOTO;ENCODING=b` — only when `--include-avatars` is set
- `UID` — the person's stable ID slug, so re-imports update existing
  cards instead of duplicating

## Round-tripping

Apple Contacts and Google Contacts both treat repeated `UID` as an update
trigger. That means you can:

1. Import from Apple → markdown.
2. Edit names / emails / tags in markdown.
3. `clawdex export vcard --all -o contacts.vcf`.
4. Drag the `.vcf` into Apple Contacts → it updates existing cards in
   place.

This is the closest thing clawdex has to two-way sync today, and it works
because vCard files are dumb, well-understood text. Programmatic
[Sync](imports.md#sync-preview-only) is still preview-only.

## Related pages

- [People](people.md), [Avatars](avatars.md)
- [Imports](imports.md) — the inbound counterpart
- [Markdown Storage](markdown-storage.md) — the source of truth
