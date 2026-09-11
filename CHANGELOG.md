# Changelog

## 0.2.3 - 2026-09-11

- **Highlights:** Keep unpublished contacts safe when pulling from a configured backup remote.
- Fixed `git pull` discarding unpublished local commits when `git.remote` is configured; pulls now preserve local history and reject divergence instead of resetting the branch.
- Updated x/sys to 0.48.0, deadcode to 0.50.0, and govulncheck to 1.8.0 while retaining Go 1.26.8 and the existing macOS support baseline.

## 0.2.2 - 2026-09-07

- **Highlights:** Repair damaged contacts reliably on Windows and optionally skip large Apple thumbnails without interrupting contact imports.
- Fixed Windows contact repair backups failing on drive-letter paths; added native Windows CLI coverage for original-file preservation and repeat repairs. Thanks @SebTardif.
- Added `import apple --avatars --max-avatar-bytes N` to skip oversized incoming thumbnails with a warning while preserving existing avatars and continuing contact imports; the default remains unlimited, and dry runs apply the same filter. Thanks @SebTardif.
- Updated the Go 1.26 toolchain to 1.26.8 and gofumpt to 0.12.0 while preserving the existing macOS support baseline.

## 0.2.1 - 2026-09-06

0.2.0 was tagged by a failed automated run but never published as a GitHub Release; its assets and release notes ship as 0.2.1.

- **Highlights:** Import contacts from compatible local crawlers, with safer matching, local-only setup, and more reliable archive maintenance.
- Added `import contacts --from` for compatible crawler exports, including Telecrawl and Wacrawl, with per-source evidence, normalized phone deduplication, idempotent imports, and safeguards against name-only and cross-person joins. Thanks @joshp123.
- Made initialization local-only by default; backup remotes are configured explicitly. Thanks @TeodoroRodrigo.
- Streamed Apple contact export decoding without a total size ceiling, preserving large file and macOS imports and the existing 16 MiB NDJSON record limit. Thanks @SebTardif.
- Enforced the global `--dry-run` no-write contract across initialization, config, people, notes, imports, vCard export, Git helpers, doctor repairs, and automatic repair.
- Hardened avatar storage and vCard avatar reads against symlink escapes, and made avatar and vCard file replacement private and atomic.
- Capped manual avatar reads at 10 MiB while preserving oversized legacy avatar metadata and import protection during Doctor repair. Thanks @SebTardif.
- Fixed vCard export hanging on long salvaged names containing invalid UTF-8 bytes. Thanks @SebTardif.
- Fixed `note add` hanging on inaccessible notes paths and corrected duplicate filename suffixes without limiting note counts. Thanks @SebTardif.
- Stopped Google imports from looping on repeated pagination tokens, with a 500-page safety ceiling that preserves larger terminating imports. Thanks @SebTardif.
- Added a dedicated 20-second HTTP timeout for Google avatar downloads while preserving earlier caller cancellation. Thanks @SebTardif.
- Fixed Unicode-safe search snippets, live Discrawl WAL reads, unexpected person-directory errors, and tagged `go install` version reporting.
- Hardened documentation rendering and third-party asset loading, and preserved metadata in the generated LLM documentation index. Thanks @vincentkoc.
- Updated to Go 1.26.6 to resolve Go standard-library vulnerabilities GO-2026-5856, GO-2026-5026, GO-2026-5972, GO-2026-6090, and GO-2026-6218; refreshed Kong to 1.16.1, CrawlKit to 0.14.7, and x/sys to 0.47.0.
- Updated CI checks and tooling, including Checkout 7.0.1, Setup Go 7.0.0, and Setup Node 7.0.0, while preserving the Go 1.26.6 minimum.
- Simplified signed releases through the shared OpenClaw Go CLI workflow, with automated notarization, artifact verification, and Homebrew updates.
- Made release binaries reproducible across checkout and dependency-cache locations so the independent rebuild gate can verify them before publication.

## 0.1.0 - 2026-05-08

- Initial `clawdex` CLI with markdown-backed people, timestamped notes, search, timeline, Git helpers, vCard export, and repair for damaged frontmatter.
- Added Apple Contacts import on macOS, Google Contacts import through `gog`, Discord DM backfill through Discrawl, and X/Twitter DM backfill through Birdclaw.
- Added local avatar support with manual avatar commands, Apple and Google avatar backfill, avatar repair checks, and optional vCard `PHOTO` export.
- Added CI with lint, tests, 90% coverage enforcement, race tests, dependency checks, secret scanning, and GoReleaser snapshot validation.
- Added GoReleaser config and release workflow that publishes cross-platform binaries and dispatches the Homebrew tap formula updater.
