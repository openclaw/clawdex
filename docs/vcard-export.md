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
person's avatar is embedded as a base64 `PHOTO:data:<mime>;base64,...` URI. A few
hundred avatars adds up — expect a few megabytes.

Avatar MIME metadata must be a valid media type without line breaks. MIME
parameters are encoded for the data URI. Invalid metadata stops the export
and leaves an existing output file intact; use `clawdex doctor --repair` to
recompute avatar metadata from the stored file, then export again.

Clawdex resolves avatar files within the contacts repo, rejects symlink
components, and opens only regular files; FIFOs and devices are rejected
without blocking. File output uses a private `0600` temporary file in the
destination directory and an atomic rename; an existing symlink at the output
path is rejected. An explicitly selected symlinked directory such as macOS
`/tmp` is opened as the destination root; the open directory remains anchored
if that symlink is later retargeted. With `--dry-run`, Clawdex validates the
export but does not create or replace the output file.

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
- `EMAIL` per email entry, with the original `label` as a `TYPE` parameter
  when present
- `TEL` per phone entry, with the original `label` as a `TYPE` parameter
- `NOTE` — the `clawdex:<person ID>` marker; private Markdown prose and notes
  are not exported
- `PHOTO` — a data URI, only when `--include-avatars` is set
- `UID` — the person's stable `person_<UUID>` ID
- `CATEGORIES` — individual tags, with punctuation escaped per tag

Lines fold at 75 octets, including continuation whitespace, without splitting
valid UTF-8 characters.

## Round-tripping

The UID remains stable across exports, but duplicate handling belongs to the
receiving address book. Preview its import behavior before importing a large
file; Clawdex does not guarantee that an importer updates existing cards.
Programmatic [Sync](imports.md#sync-preview-only) remains preview-only.

## Related pages

- [People](people.md), [Avatars](avatars.md)
- [Imports](imports.md) — the inbound counterpart
- [Markdown Storage](markdown-storage.md) — the source of truth
