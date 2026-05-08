# Clawdex

`clawdex` is a local-first, markdown-first personal contact index. The CLI
lives in this repo. Your contacts live in a separate private Git-backed
markdown repository that you own.

The whole thing is a Rolodex you can `grep` — every person is a folder, every
note is a timestamped markdown file, every change is a commit.

## Try it

After [installing](install.md) and [pointing clawdex at a data repo](quickstart.md):

```bash
clawdex person add "Sally O'Malley" --email sally@example.com --tag friend
clawdex note add sally --kind dm --source whatsapp --text "Follow up about dinner"
clawdex timeline sally
clawdex search dinner
clawdex export vcard --all --include-avatars -o contacts.vcf
```

`--json` produces a stable JSON envelope on stdout. `--plain` produces TSV.
Human messages go to stderr, so pipes stay parseable.

## What clawdex does

- **Markdown is canonical.** People are folders under `people/`. Notes are
  timestamped files. Indexes under `index/` are derived and rebuildable.
- **Git is the sync layer.** No proprietary daemon, no opaque database.
  `clawdex git push` pushes to your private remote.
- **Imports are local-only.** Apple, Google, Birdclaw (X DMs), and Discrawl
  (Discord DMs) all project into the same markdown shape. Address-book writes
  are still preview-only — see [Imports](imports.md).
- **Avatars are real files.** Stored next to each person; `vcard` export can
  embed them; manual avatars are never overwritten by imports.
- **Notes are local-only.** They are never written to Apple or Google.

## Pick your path

- **Trying it.** [Install](install.md) → [Quickstart](quickstart.md). Five
  minutes from `brew install` to your first commit.
- **Importing the network you already have.** [Imports](imports.md) covers
  Apple Contacts, Google Contacts, X DMs (birdclaw), and Discord DMs
  (discrawl).
- **Daily use.** [People](people.md), [Notes](notes.md),
  [Timeline](timeline.md), [Search](search.md).
- **Sharing back.** [vCard Export](vcard-export.md) for round-tripping into
  Apple/Google/anything-else, [Git Sync](git-sync.md) for the private backup
  remote.
- **Repair and storage.** [Markdown Storage](markdown-storage.md),
  [Doctor](doctor.md), [Config](config.md).

## Project

Source: [openclaw/clawdex](https://github.com/openclaw/clawdex).
[Changelog](https://github.com/openclaw/clawdex/blob/main/CHANGELOG.md) tracks
what shipped recently. Released under the
[MIT license](https://github.com/openclaw/clawdex/blob/main/LICENSE). Not
affiliated with Apple, Google, X, or Discord.
