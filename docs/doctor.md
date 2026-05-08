# Doctor

`clawdex doctor` is a one-shot health check for your data repo. It reports
counts and surfaces problems; with `--repair`, it fixes the ones it knows
how to fix.

```bash
clawdex doctor
clawdex doctor --json
clawdex doctor --repair --dry-run
clawdex doctor --repair
```

## What it reports

```text
config_path: /Users/you/.clawdex/config.toml
repo_path: /Users/you/.clawdex/contacts
remote: https://github.com/you/backup-clawdex.git
people: 412
git_dirty: true
avatar_problems: 3
```

Fields:

- `config_path` — the user-level config clawdex loaded.
- `repo_path` — which contacts data repo is in use (after `--repo`,
  `CLAWDEX_REPO`, and config resolution).
- `remote`, `people`, `git_dirty` — sanity numbers.
- `avatar_problems` — count of person records whose avatar metadata
  doesn't match what's on disk (file gone, sha256 mismatch). Only printed
  when non-zero.

## What `--repair` repairs

Two classes of problem:

### 1. Damaged frontmatter

If `person.md` or a note's YAML frontmatter is malformed — a stray quote,
a truncated block, a dangling key — clawdex's strict parse fails. The
repair pass:

1. Salvages known scalar keys: `id`, `name`, `created_at`, and the note
   fields (`kind`, `source`, `occurred_at`, `topics`).
2. Infers a missing `id` from the file path and a missing
   `created_at`/`updated_at` from the file mtime.
3. Preserves the Markdown body verbatim.
4. Copies the original damaged file under `.clawdex/repairs/` so nothing
   is lost.
5. Appends the unsalvageable scrap to the body under a `## Recovered
   metadata` heading, so you can finish the cleanup by hand.

### 2. Stale avatar metadata

If `person.md` says there's an avatar but the file is missing, the SHA256
no longer matches, or the MIME type is wrong, repair will:

- Drop the metadata if the file is gone.
- Recompute MIME and SHA256 if the file is present.
- Leave the bytes themselves untouched.

Avatar bytes are never *recovered* by repair — only the metadata is
synced to whatever's actually on disk. Use
[`clawdex person avatar set`](avatars.md) to put bytes back.

## Dry-run

```bash
clawdex doctor --repair --dry-run
```

Walks the repo and reports counts without writing anything. The output
includes:

```text
repaired: 4
avatar_repaired: 2
dry_run: true
```

Treat the counts as the worst case — once you re-run without `--dry-run`,
the numbers should drop to zero on the next clean `clawdex doctor`.

## When to run it

- After a big import (`apple`, `google`, `birdclaw`, `discrawl`).
- After resolving Git merge conflicts in the data repo.
- After a hand-edit binge.
- As the last step before `clawdex git commit && clawdex git push`.

`clawdex doctor` is read-only. `clawdex doctor --repair` is the only
variant that writes; it always backs up the original file first when
`repair.backup_before_repair = true` (the default).

## Exit codes

- `0` — everything looked fine.
- `1` — runtime error (bad path, IO error, malformed config).
- `2` — usage error (bad flag combination).

`avatar_problems` and `git_dirty` are diagnostics, not errors. They do not
change the exit code on their own.

## Related pages

- [Markdown Storage](markdown-storage.md)
- [People](people.md), [Notes](notes.md), [Avatars](avatars.md)
- [Config](config.md)
