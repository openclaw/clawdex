# Changelog

## 0.1.1 - Unreleased

- Hardened avatar storage and vCard avatar reads against symlink escapes, and made avatar and vCard file replacement private and atomic.
- Enforced the global `--dry-run` no-write contract across initialization, config, people, notes, imports, vCard export, Git helpers, doctor repairs, and automatic repair.
- Updated to Go 1.26.5 and CrawlKit 0.13.4 to address reachable Go standard-library vulnerability GO-2026-5856.
- Fixed Unicode-safe search snippets, live Discrawl WAL reads, unexpected person-directory errors, and tagged `go install` version reporting.
- Replaced legacy CI publication with a credential-free, read-only mounted release driver, locally Foundation-signed and notarized Darwin artifacts, protected-default-branch signed-tag-object reproduction, exact embedded-requirement, provenance, signature, online notarization, sealed-snapshot publication, and downstream re-download verification.

## 0.1.0 - 2026-05-08

- Initial `clawdex` CLI with markdown-backed people, timestamped notes, search, timeline, Git helpers, vCard export, and repair for damaged frontmatter.
- Added Apple Contacts import on macOS, Google Contacts import through `gog`, Discord DM backfill through Discrawl, and X/Twitter DM backfill through Birdclaw.
- Added local avatar support with manual avatar commands, Apple and Google avatar backfill, avatar repair checks, and optional vCard `PHOTO` export.
- Added CI with lint, tests, 90% coverage enforcement, race tests, dependency checks, secret scanning, and GoReleaser snapshot validation.
- Added GoReleaser config and release workflow that publishes cross-platform binaries and dispatches the Homebrew tap formula updater.
