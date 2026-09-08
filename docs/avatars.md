# Avatars

Every person can carry a single avatar image, stored as a real file next to
their `person.md`. Avatars are deliberately first-class — they are how
exported vCards stay recognizable in Apple Contacts, Google Contacts, and
phone dial screens.

On disk:

```text
people/sally-o-malley/
  person.md
  avatars/
    avatar.jpg          # or .png
```

The `person.md` frontmatter records the avatar's MIME type, sha256, source
(`manual`, `apple`, `google`), and updated time. The bytes are kept on disk;
the markdown only points at them.

Avatar paths are confined to the contacts repo. Clawdex rejects symlinks in
the avatar leaf or any parent component, requires reads to resolve to an open
regular file rather than a FIFO or device, then replaces avatar files
atomically with private `0600` permissions.

## Set a manual avatar

```bash
clawdex person avatar set sally ~/Pictures/sally.jpg
clawdex person avatar set sally ~/Pictures/sally.png --dry-run
```

`--dry-run` inspects the source image (MIME, SHA256), validates the same rooted
destination and overwrite policy as the real write, and previews the avatar
metadata without touching the data repo. Avatar-enabled imports perform the
same non-writing destination validation.

Manual source files must be regular files reached without a symlink leaf or a
symlink beneath the current directory, home directory, or temporary directory.
This prevents an apparently local avatar from copying unrelated file contents
into the contacts repo. Files larger than 10 MiB are rejected, matching the
Google import avatar cap, so `clawdex person avatar set` cannot load a huge
path into memory.

For existing stored avatars above this limit, `doctor --repair` reports the
size error and leaves the avatar file and metadata untouched. The manual
avatar remains protected from imports; resize it and set it again explicitly
to bring it within the inspection limit.

Manual avatars are sticky: subsequent `clawdex import apple` and
`clawdex import google` runs will **never overwrite a manual avatar**. To
let imports manage the avatar again, clear it first.

## Show

```bash
clawdex person avatar show sally
clawdex person avatar show sally --path
clawdex person avatar show sally --json
```

`--path` prints the absolute path to the avatar file and nothing else, so
you can pipe it into a viewer:

```bash
open "$(clawdex person avatar show sally --path)"
```

## Clear

```bash
clawdex person avatar clear sally
```

Clears the avatar metadata from `person.md`. The file under `avatars/` is
left alone in case you want to recover it; remove it manually if you're
sure.

## Avatars from imports

Avatars are opt-in on imports — the `--avatars` flag is required:

```bash
clawdex import apple  --avatars
clawdex import google --account you@gmail.com --avatars
```

Apple imports accept `--max-avatar-bytes N` with `--avatars` to skip incoming
thumbnails above N decoded bytes. The default is `0` (unlimited), preserving
existing imports. For example, `--max-avatar-bytes 10485760` skips thumbnails
above 10 MiB with a warning on stderr, while contact fields and subsequent
contacts continue importing. Existing avatar files and metadata stay intact.
Dry runs apply the same filtering and warnings without writing. The limit is
checked after JSON decoding, so it does not bound input memory. Manual and
Google avatar limits are unchanged.

- **Apple.** Reads the thumbnail bytes that
  [`Contacts.framework`](https://developer.apple.com/documentation/contacts)
  hands out. macOS-only.
- **Google.** Calls `gog contacts raw --person-fields photos`, picks the
  selected photo URL, fetches the bytes through
  [`gog`](https://github.com/steipete/gogcli), and stores them locally.
  Only metadata (URL, MIME, SHA256) is written into `person.md`.

In both cases the bytes live on your machine. Clawdex never silently
re-fetches them on later runs.

## vCard export

Avatars are embedded into vCards as base64 `PHOTO;ENCODING=b` payloads when
you opt in:

```bash
clawdex export vcard --all --include-avatars -o contacts.vcf
```

Without `--include-avatars`, the export is text-only — handy when you want a
small file or are emailing the vCard to someone.

## Related pages

- [People](people.md), [Imports](imports.md), [vCard Export](vcard-export.md)
- [Doctor](doctor.md) — `clawdex doctor` reports stale avatar metadata
  (file gone, hash mismatch) and `--repair` rewrites it from disk.
