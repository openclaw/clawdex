# Quickstart

Five minutes from a fresh install to a populated, committed contact index.

## 1. Install

```bash
brew install steipete/tap/clawdex
clawdex --version
```

Other paths (Go install, source build, release archives) are documented on
[Install](install.md).

## 2. Pick a place for your contacts

The CLI lives in this repo. Your contacts live in **a separate, private
markdown repo** that you own. The default suggested remote is:

```text
https://github.com/<you>/backup-clawdex.git
```

Create that empty private repo on GitHub first — it's where your data will
back up to.

## 3. Initialize a contacts data repo

```bash
clawdex init ~/.clawdex/contacts
clawdex config set repo_path ~/.clawdex/contacts
clawdex config set git.remote https://github.com/<you>/backup-clawdex.git
```

`init` writes:

```text
clawdex.toml
people/
index/
.clawdex/repairs/
```

The app config lives at `~/.clawdex/config.toml` by default. `--repo DIR` or
`CLAWDEX_REPO=DIR` overrides the configured repo for one run. See
[Config](config.md) for the full key list.

## 4. Add your first person

```bash
clawdex person add "Sally O'Malley" \
  --email sally@example.com \
  --phone "+1 555 0100" \
  --tag friend
```

Look at what just appeared in the data repo:

```text
people/sally-o-malley/
  person.md
```

Open `person.md` — it's plain markdown with YAML frontmatter. Edit it by
hand, in your editor, on another machine, in a Pull Request review. Clawdex
will read your edits back. See [People](people.md).

## 5. Add a note

```bash
clawdex note add sally \
  --kind dm \
  --source whatsapp \
  --text "Follow up about dinner next Thursday"
clawdex timeline sally
```

Notes land in `people/sally-o-malley/notes/` as timestamped files. They are
local-only — never written to Apple, Google, or anywhere else. See
[Notes](notes.md) and [Timeline](timeline.md).

## 6. Search across everything

```bash
clawdex search dinner
clawdex search +1555
clawdex search sally@example.com
```

Search hits emails, phones, names, tags, and note bodies. See
[Search](search.md).

## 7. Import the network you already have

Optional, but most people start here on day one. All imports are local-only:
they only write to your markdown repo. Address-book writes (Apple Contacts,
Google Contacts) are not implemented yet — see [Imports](imports.md).

```bash
clawdex import apple --dry-run
clawdex import apple --avatars

clawdex import google --account you@gmail.com --dry-run
clawdex import google --account you@gmail.com --avatars

clawdex import birdclaw --min-messages 4 --dry-run
clawdex import discrawl --min-messages 4 --dry-run
```

## 8. Commit and push

```bash
clawdex git status
clawdex git commit -m "sync: import apple + google"
clawdex git push
```

`git status` is a thin wrapper over `git -C <repo> status --short --branch`.
Commit and push run inside the data repo, not this repo. See
[Git Sync](git-sync.md).

## 9. Export back into the world

```bash
clawdex export vcard --all --include-avatars -o ~/Desktop/contacts.vcf
clawdex export vcard --person sally -o -          # stdout
```

The `.vcf` file imports cleanly into macOS Contacts, Google Contacts, iOS
Contacts, and most other address books. See [vCard Export](vcard-export.md).

## Where next

- **The four pillars.** [People](people.md), [Notes](notes.md),
  [Avatars](avatars.md), [Imports](imports.md).
- **Flow.** [Search](search.md), [Timeline](timeline.md),
  [Git Sync](git-sync.md), [vCard Export](vcard-export.md).
- **Maintenance.** [Doctor](doctor.md), [Markdown Storage](markdown-storage.md),
  [Config](config.md).
