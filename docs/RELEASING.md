---
summary: "Release clawdex through the shared OpenClaw Go CLI workflow"
---

# Releasing clawdex

Official releases use `.github/workflows/release-unified.yml`, which calls
`openclaw/release-workflows/.github/workflows/release-go-cli.yml@v1`.
This is the same shared contract used by Wacrawl, Notcrawl, and Graincrawl.
The shared workflow owns the annotated version tag, Foundation signing, Apple
notarization, artifact verification, GitHub publication, and Homebrew handoff.

## Prepare the release

Work from clean, current, protected `main`. Merge the intended fixes first and
require CI green on the exact release commit. Move the relevant Unreleased
entries into one dated `## X.Y.Z - YYYY-MM-DD` section, preserving Highlights
first and ordering changes by user interest. Do not publish while release
changes are still only listed under Unreleased.

The shared workflow validates SemVer and requires exactly one matching dated
level-two changelog section on the frozen source. It preserves that section as
the release notes, including Highlights and their original ordering.
Any present `.release-version`, `VERSION`, `version.txt`, or root `package.json`
version must agree. Clawdex currently injects `internal/cli.Version` through
GoReleaser linker flags; tagged `go install` builds use Go module metadata.
Keep the `dev` source default rather than hardcoding a released version.
These checks belong to the shared workflow, alongside the exact-commit CI gate.

Local checks and snapshots need no signing credentials:

```bash
go test -count=1 ./... -coverprofile=coverage.out
go test -count=1 -race ./...
go vet ./...
node --test scripts/*.test.mjs
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck= .github/workflows/*.yml
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Use the Go toolchain specified by `go.mod`. Snapshot Darwin binaries are
development artifacts; the official workflow signs and notarizes its own frozen
payload before publication. Never upload local snapshots as an official release.

## Repository configuration

The caller passes the standard shared-workflow secrets by name:

| Caller secret | Shared workflow secret | Purpose |
| --- | --- | --- |
| `MACOS_SIGNING_P12` | `MACOS_SIGNING_P12` | Foundation Developer ID certificate and private key |
| `MACOS_SIGNING_P12_PASSWORD` | `MACOS_SIGNING_P12_PASSWORD` | Certificate export password |
| `ASC_KEY_ID` | `ASC_KEY_ID` | Notarization API key ID |
| `ASC_ISSUER_ID` | `ASC_ISSUER_ID` | Notarization issuer |
| `ASC_PRIVATE_KEY_P8` | `ASC_PRIVATE_KEY_P8` | Notarization API private key |
| `HOMEBREW_TAP_TOKEN` | `TAP_TOKEN` | Contents read and Actions write on `openclaw/homebrew-tap` |

Use the existing organization-managed release credentials made available to
the caller through repository secrets or eligible organization secrets. These
are not stored in the reusable workflow repository: GitHub resolves the caller's
`secrets` context. Missing secrets are a provisioning blocker, not a reason to
skip signing or notarization. Do not put credentials in workflow inputs or files.

The shared pipeline uses an ephemeral signing keychain and temporary API-key
material on its signing runner. Local keychains and `NOTARYTOOL_PROFILE` are
not required. There is no release-specific GitHub environment, manual reviewer
gate, or publication HMAC secret.

Repository Actions settings must allow `default_workflow_permissions=write`
and `can_approve_pull_request_reviews=true` for the shared closeout PR. The
caller starts with `permissions: {}` and grants only its jobs' required scopes.

## Dispatch and verification

Immediately before starting a new release, refresh tags and check for collisions:

```bash
git fetch --tags origin
gh release list --repo openclaw/clawdex --limit 3 --json tagName
git tag
```

If the target tag or release already exists, stop and inspect that release;
do not move its tag or start another publication attempt. With a new version,
dispatch from the exact green `main` head:

```bash
gh workflow run release-unified.yml --repo openclaw/clawdex --ref main -f version=X.Y.Z
gh run list --repo openclaw/clawdex --workflow release-unified.yml --limit 3 --json databaseId,headSha,status,conclusion,url
```

The pipeline checks signing credentials before creating its tag, freezes the
protected source commit, and builds all six native Darwin/Linux/Windows targets.
It signs Darwin binaries as `org.openclaw.clawdex` with the OpenClaw Foundation
identity (`FWJYW4S8P8`) and notarizes them. Independent Intel and Apple Silicon
verifiers check signatures, online notarization, architecture, native execution,
inventory, checksums, and exact release-note bytes without signing credentials.
A separate non-Darwin rebuild must reproduce every Linux and Windows binary.
The publisher re-downloads the draft assets and binds them to both verifier
attestations before making the release public.

The caller enables strict checks and limits eligible Actions CI events to
`push` and `pull_request`, so unrelated scheduled dispatches do not replace
the exact-commit CI gate. Do not publish manually around a failed pipeline.

Each native archive retains its established name:
`clawdex_X.Y.Z_<darwin|linux|windows>_<amd64|arm64>`, using `.tar.gz` except
`.zip` on Windows. Archives include the binary, README, LICENSE, and changelog.
The shared controls include `checksums.txt`, `ASSET-INVENTORY.json`,
`RELEASE-NOTES.md`, and `SIGNING-MANIFEST.json`. No universal Darwin or Linux
package assets are requested.

After the run completes, verify the published GitHub notes match the finalized
section and the downloaded assets satisfy `checksums.txt`. Verify a downstream
source install and the Homebrew formula:

```bash
go list -m github.com/openclaw/clawdex@vX.Y.Z
go install github.com/openclaw/clawdex/cmd/clawdex@vX.Y.Z
clawdex --version
brew update
brew install openclaw/tap/clawdex  # or brew upgrade openclaw/tap/clawdex
clawdex --version
```

The workflow hands exact verified asset names and hashes to
`openclaw/homebrew-tap`, waits for its update run, and verifies the resulting
formula URLs and hashes. A successful dispatch alone is insufficient.
Clawdex does not publish an npm package.

For an interrupted run, inspect the exact failed run and its frozen tag before
retrying. Before promotion, failed jobs can be rerun normally. If the release
is already public and the publisher failed its final API check, rerunning only
that publisher returns `draft identity changed before content binding`.

For that post-promotion case, rerun **Create exact draft** and its dependent jobs
in the same workflow run, retaining the successful build, signing, and comparison
artifacts. Resolve the job's API ID rather than copying a browser URL:

```bash
gh run view RUN_ID --repo openclaw/clawdex --json jobs --jq '.jobs[] | select(.name | contains("Create exact draft")) | {name, databaseId}'
gh run rerun RUN_ID --repo openclaw/clawdex --job DRAFT_JOB_ID
```

The draft job reuses the existing signed payload and frozen notes, generates a
new verification payload, and reruns both native verifiers. The publisher then
requires the existing public release's notes and every asset byte to equal that
verified payload, deletes only the redundant retry draft, and resumes Homebrew
handoff. Do not rebuild or re-sign for this recovery: changed signing timestamps
would produce different bytes. If the retained artifacts have expired, stop and
resolve recovery separately; never move the tag or delete the public release.

## Closeout

The shared workflow opens a PR restoring `## Unreleased` when needed. Reconcile
and land that PR after downstream verification instead of adding a duplicate
section. If a section already exists, preserve it. Finish with clean `main` at
`origin/main` and remove task-owned build outputs and branches.
