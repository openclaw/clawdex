---
summary: "Release checklist for locally signed and notarized clawdex archives"
---

# Releasing `clawdex`

Always complete every gate. No partial release and no unsigned or unnotarized
official Darwin asset.

Release identity:

- Repository: `openclaw/clawdex`
- Binary: `clawdex`
- macOS identifier: `org.openclaw.clawdex`
- Team: OpenClaw Foundation (`FWJYW4S8P8`)
- Architectures: Darwin, Linux, and Windows on `amd64` and `arm64`

The release credential boundary is local. Ordinary builds, tests, and snapshot
archives require no signing or notarization credentials. The signing identity
lives only in the managed runtime release keychain, and the notarytool profile
lives only in the runtime keychain. Never commit either configuration.

## 0) Prerequisites

- Clean, current `main`; exact release commit agreed and CI green.
- Go toolchain exactly matches `go.mod`.
- Changelog section contains the complete release notes.
- Managed passwordless, non-locking Foundation release keychain is in the user
  search list.
- Runtime `NOTARYTOOL_PROFILE` names a valid Keychain profile.
- The `clawdex-release` environment allows only protected branches, requires
  at least one reviewer, and prevents the workflow initiator from self-review.
- That environment alone stores `CLAWDEX_PUBLICATION_HMAC_KEY`, a stable random
  32-byte lowercase-hex key used only to authenticate post-promotion recovery.
  Provisioning or rotating it is a separately serialized secret-management
  gate; never expose it to builds, candidates, logs, or repository files.
- The protected publisher token can read that environment policy. An unreadable
  policy is a hard pre-publication failure, not a reason to bypass the check.
- A repository owner has audited every principal able to mutate GitHub Releases
  through `Contents: write` and explicitly accepted that set as trusted release
  writers for this release. Environment approval gates this workflow job; it
  does not revoke an independently authorized principal's Releases API access.
  If workflow-only writer isolation is required, stop and establish a separate,
  owner-approved authorization control before publishing. Candidate code cannot
  prove or create that control.
- GitHub and Homebrew mutations have their separate serialized approvals.

### One-time legacy-workflow migration gate

Before the commit that removes `.github/workflows/release.yml` is merged, a
repository owner must disable that existing workflow ID under a separately
authorized GitHub-settings gate. Deleting the file is insufficient: a `v*` tag
pointing at an older commit can still load the historical tag-triggered workflow
from that commit and publish unsigned assets. Perform and verify the disable
while the legacy file is still on the default branch:

```bash
gh workflow disable release.yml --repo openclaw/clawdex
gh api --paginate repos/openclaw/clawdex/actions/workflows |
  jq -s -e '[.[].workflows[] | select(
    .path == ".github/workflows/release.yml" and
    .state == "disabled_manually")] | length == 1'
```

Do not merge the deletion, create any release tag, or enable the new publication
lane until this state is independently recorded. This is a one-time repository
policy migration, not an action performed by candidate code or ordinary builds.

## 1) Local proof

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.1 run
go test -count=1 ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -n 1
go test -count=1 -race ./...
go vet ./...
govulncheck ./...
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  GOOS="${target%/*}" GOARCH="${target#*/}" CGO_ENABLED=0 go build -o /tmp/clawdex-cross ./cmd/clawdex
done
bash scripts/test-clawdex-release.sh
```

Coverage floor: `90%+`. Run the repository autoreview workflow to convergence
after the diff and proof scope are frozen.

GoReleaser is retained only for credential-free Linux/Windows snapshot sanity.
It is not a publication path and must never produce an official Darwin asset.

## 2) Release commit and signed tag

Before the commit, replace `## X.Y.Z - Unreleased` with
`## X.Y.Z - YYYY-MM-DD`, review the exact notes and full release diff, and run
the proof suite again. The packager rejects a signed tag whose release section
is absent, empty, duplicated, or still marked `Unreleased`.

Only after the commit/tag gate:

```bash
git checkout main
git pull --ff-only origin main
git commit -am "release: vX.Y.Z"
git tag -s vX.Y.Z -m "Release X.Y.Z"
git push origin main vX.Y.Z
```

The remote tag must be an annotated tag whose embedded `tag` header exactly
matches `vX.Y.Z`, resolve to the reviewed release commit, and carry the approved
SSH signature. This prevents a correctly signed tag object from being replayed
under a different ref name. This v0.1 release lane accepts stable `vX.Y.Z` tags
only; prereleases require a separate metadata-aware process. The credentialed
packager never executes code or policy from that tag before authenticating it.

## 3) Build, sign, notarize, and verify locally

Materialize and mount a read-only driver disk image from the exact current protected
default-branch commit approved by the serialized release gate. The driver
supplies the packager and allowed-signers trust anchor; it is separate from the
release-tag source. Bootstrap the materializer itself from that authenticated
commit under sanitized Git configuration, and finish this stage before making
the signing keychain or notary profile available:

```bash
: "${APPROVED_DRIVER_COMMIT:?set the exact commit granted by the serialized gate}"
DRIVER_BOOTSTRAP=$(mktemp -d /private/tmp/clawdex-driver-bootstrap.XXXXXX)
mkdir -m 0700 "$DRIVER_BOOTSTRAP/home"
clean_driver_git() {
  env -i PATH="$PATH" HOME="$DRIVER_BOOTSTRAP/home" TMPDIR="$DRIVER_BOOTSTRAP" \
    GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0 \
    git -c core.fsmonitor=false -c core.hooksPath=/dev/null "$@"
}
clean_driver_git init -q "$DRIVER_BOOTSTRAP/repository"
clean_driver_git -C "$DRIVER_BOOTSTRAP/repository" remote add origin \
  https://github.com/openclaw/clawdex.git
clean_driver_git -C "$DRIVER_BOOTSTRAP/repository" fetch --quiet --no-tags origin \
  refs/heads/main:refs/remotes/origin/main
test "$(clean_driver_git -C "$DRIVER_BOOTSTRAP/repository" \
  rev-parse refs/remotes/origin/main^{commit})" = "$APPROVED_DRIVER_COMMIT"
test "$(clean_driver_git -C "$DRIVER_BOOTSTRAP/repository" \
  cat-file -t "$APPROVED_DRIVER_COMMIT")" = commit
clean_driver_git -C "$DRIVER_BOOTSTRAP/repository" \
  show "$APPROVED_DRIVER_COMMIT:scripts/materialize-clawdex-release-driver.sh" \
  > "$DRIVER_BOOTSTRAP/materialize.sh"
chmod 0500 "$DRIVER_BOOTSTRAP/materialize.sh"
env -i PATH="$PATH" TMPDIR="$DRIVER_BOOTSTRAP" \
  bash "$DRIVER_BOOTSTRAP/materialize.sh" "$APPROVED_DRIVER_COMMIT" \
  /private/tmp/clawdex-release-driver-vX.Y.Z

export CLAWDEX_RELEASE_DRIVER_COMMIT="$APPROVED_DRIVER_COMMIT"
export MAC_RELEASE_CODESIGN_IDENTITY='Developer ID Application: OpenClaw Foundation (FWJYW4S8P8)'
export MAC_RELEASE_CODESIGN_KEYCHAIN=/runtime/path/to/managed-release.keychain-db
export MAC_RELEASE_CODESIGN_KEYCHAIN_MANAGED=1
export MAC_RELEASE_CODESIGN_PASSWORDLESS=1
export NOTARYTOOL_PROFILE=runtime-profile-name
unset GH_TOKEN GITHUB_TOKEN
~/Projects/agent-scripts/skills/release-mac-app/scripts/mac-release codesign-run -- \
  /private/tmp/clawdex-release-driver-vX.Y.Z/scripts/package-clawdex-release.sh \
  vX.Y.Z /private/tmp/clawdex-vX.Y.Z
```

The credential-free materializer fetches the current protected `main`, requires
it to equal the approved commit, rejects symlinks and submodules, archives only
that Git tree, records its tree and archive hashes, creates a read-only disk
image, and attaches it with `hdiutil -readonly`. It verifies through `diskutil`
that both the mounted filesystem and volume are non-writable before returning.
The packager refuses a Git worktree, ordinary directory, writable volume,
missing metadata, output path inside the mount, or different commit. Therefore
ignored files, `assume-unchanged` and `skip-worktree` state, external fsmonitor
configuration, and hidden changes in the operator checkout cannot influence
credentialed execution.

With user and system Git config disabled, that authenticated packager fetches
only the requested tag into a new private temporary repository, verifies the
tag against the archived driver's trust anchor before checkout, and builds only
that materialized tag commit.

Keep `/private/tmp/clawdex-release-driver-vX.Y.Z.dmg` and its read-only mount
attached until local packaging and verification finish. Then detach the mount
and remove both temporary paths. Never reuse that image for another release
attempt.

The packager produces exactly:

- `clawdex_X.Y.Z_darwin_amd64.tar.gz`
- `clawdex_X.Y.Z_darwin_arm64.tar.gz`
- `clawdex_X.Y.Z_linux_amd64.tar.gz`
- `clawdex_X.Y.Z_linux_arm64.tar.gz`
- `clawdex_X.Y.Z_windows_amd64.zip`
- `clawdex_X.Y.Z_windows_arm64.zip`
- `checksums.txt`
- `provenance.json`

Darwin binaries must have hardened runtime, secure timestamp, identifier
`org.openclaw.clawdex`, Foundation authority/team, the expected thin
architecture, exact embedded designated requirement, accepted notary
submission, embedded clean VCS revision matching the signed tag commit, exact
Go 1.26.5 build metadata, correct runtime version, and an online
`codesign --verify --strict --check-notarization -R=notarized` result. A
standalone command-line binary has no stapling container.

Do not use or mock `spctl --assess --type execute` as a standalone-binary gate.
On macOS 26.5 it rejects known Apple-notarized command-line tools as valid code
that is not an app. Keep `spctl`, `syspolicy_check`, and `stapler` gates for
actual `.app`, `.dmg`, or `.pkg` targets where those tools apply.

`provenance.json` schema 3 binds the exact signed annotated tag object, peeled
tag commit, protected driver commit, Go version, source timestamp, finalized
release notes, artifact hashes, signing identity, and both notary submission
IDs. The local verifier rechecks it, verifies the tag against the pinned
release signer, then rebuilds and byte-compares all four Linux and Windows
payloads before the output directory becomes visible.

The credentialed packager never executes a Darwin candidate. While the
Foundation keychain is active it performs only builds, signing, submission,
static metadata/requirement checks, and archive verification. Candidate
`--version` execution occurs later on protected runners after signing and
notarization credentials are absent.

## 4) Create an unpublished draft

Use a separate GitHub-authenticated shell only after the draft-upload gate.
Create release notes from the exact finalized changelog section in the signed
tag, create a non-prerelease draft for that tag, and upload the eight verified
files. Inspect the draft inventory; do not publish it yet. The same notes are
sealed in provenance; the protected publisher validates the draft body against
that sealed copy rather than its verifier checkout.

Candidate execution and the protected verifier must never inherit credentials.
Upload tooling may hold a token, but candidate execution occurs only in a
minimal environment with an empty temporary home after the GitHub token names
are unset. The local packager has narrowly scoped signing/notary access and
does not execute candidate bytes.

## 5) Protected verifier gate

For a verifier-only rehearsal, dispatch `Release Assets` on the protected
default branch with the exact tag, `draft=true`, and `publish=false`. For the
final candidate, use `publish=true`; the publication job remains blocked at
the protected environment until the later VM and serialized approval gates. An
unconditional first job rejects every non-default-branch dispatch instead of
leaving a misleading all-skipped green run. The workflow:

1. Checks out verifier code and the allowed SSH release signer at the exact
   protected-default-branch workflow commit, never the tag.
2. Downloads exactly one matching stable draft and exactly eight named assets,
   then carries that draft's numeric release ID through the protected job
   outputs.
3. Checks out the release tag separately, requires its signed tag object's
   embedded name to equal the requested ref, verifies its signature against
   that protected trust anchor, and binds provenance to both the annotated tag
   object and resulting commit.
4. Uses exact Go 1.26.5 to rebuild all four Linux and Windows binaries from the
   trusted tag and byte-compares them with the uploaded payloads.
5. Seals that exact eight-file snapshot as a workflow artifact;
   both Darwin jobs consume its artifact ID instead of re-downloading mutable
   draft/release assets.
6. Runs reproduction, verification, and candidate execution in a minimal
   environment with no GitHub token and an empty temporary home.
7. Verifies inventory, archive shape and executable metadata, hashes,
   provenance, Foundation signature, identifier, exact embedded designated
   requirement, hardened runtime, secure timestamp, exactly one architecture,
   clean embedded VCS revision matching the authenticated commit, exact Go
   1.26.5 build metadata, online notarization constraint, and `--version` on
   native Intel and Apple Silicon runners.
8. After protected-environment approval, authenticates the exact repository,
   tag object, commit, verifier commit, release ID, workflow run, and sealed
   snapshot ID/digest with the environment-only HMAC key. The key is removed
   from the process environment before any API or candidate operation.

Both non-Darwin reproduction and Darwin jobs must pass for the exact uploaded
draft. A per-tag workflow concurrency group prevents two trusted verification
or publication runs from overlapping. The sealed artifact is retained for 14
days to cover VM testing and approval; an expired run must be repeated from
verification through VM proof rather than reused.

## 6) Clean-VM Gatekeeper gate

For the final `publish=true` run, wait until every verifier passes and the
publication job is pending `clawdex-release` approval. Download
each Darwin archive from the still-unpublished draft through a normal browser
on clean Intel and Apple Silicon macOS VMs. Record each archive's SHA-256 and
compare it with `checksums.txt` from that run's sealed artifact. Do not
synthesize or copy a quarantine attribute. Confirm the downloaded archive and
extracted binary naturally retain `com.apple.quarantine`, open/extract through
the normal macOS path, then run `clawdex --version`. Both architectures must
execute without a Gatekeeper prompt or security alert.

This user-visible, naturally quarantined execution is the standalone CLI's
Gatekeeper proof. It complements, but does not replace, the strict signature,
exact designated requirement, timestamp, hardened runtime, and online
notarization checks.

## 7) Publish and verify

Only after the clean-VM hashes match the same run's sealed artifact and the
serialized publish gate opens, approve that run's `clawdex-release`
environment. Do not start a replacement run, publish the draft manually, or
run privileged verifier policy from a release-tag event.

Approval makes the environment-only HMAC key available to the protected
publisher. Before any API operation, the publisher authenticates the exact
same-run publication intent, removes the key from its environment, and then
downloads the exact asset snapshot by artifact ID. It verifies the static
inventory without a token, requires the stable release's numeric ID to match
the draft captured by the inventory job, validates the draft body against
release notes recomputed from the authenticated signed-tag checkout, then uses
GitHub's asset IDs, sizes, states, and SHA-256 digests to bind the draft to the
sealed bytes. It re-downloads and byte-compares every draft asset, refreshes
release and asset identities, then requires the live tag ref to still point at
the verified signed tag object before atomically promoting the draft and adding
the authenticated intent as an exact hidden body marker. After publication it
re-downloads all eight assets by their exact asset IDs, byte-compares them
again, and performs one final release, tag-ref, and identity refresh. Confirm:

- Git tag and GitHub Release resolve to the same commit.
- Release body contains exactly the changelog notes and protected hidden marker.
- Exactly eight assets remain and checksums/provenance still match.
- Both Darwin online-ticket jobs pass.
- The publication job reports the final release ID and asset-identity manifest
  hash.
- Fresh downstream installs report `X.Y.Z`.

GitHub release immutability is a separate repository-owner policy. This
workflow neither requires nor changes it; later mutation risk must be handled
by that separately authorized policy decision and ongoing downstream checks.
Likewise, the workflow's revalidation is an integrity gate inside the accepted
release-writer trust boundary, not an authorization firewall against another
principal that already has `Contents: write`. Never claim that the protected
job is the repository's exclusive release writer without separate owner-level
proof.

If the publication job fails or is cancelled after GitHub accepts the draft
promotion, rerun that failed job in the same workflow run. The protected job
reuses the sealed asset snapshot, captured release ID, and protected
environment key. It recomputes the same intent authenticator, recognizes only
the exact public body carrying that marker, skips the irreversible PATCH, and
completes all post-publication re-download and identity checks. A release
published outside this protocol has no marker and fails closed; a new workflow
run binds a different run and snapshot and still starts at the stable-draft
download gate.

## 8) Homebrew and closeout

Homebrew is a separate serialized gate after the GitHub release is verified.
Update and test the formula from the published archives; never let candidate
code or the release verifier mutate the tap.

After downstream verification, add the next patch `Unreleased` changelog
section, commit it, push it, pull `main` with `--ff-only`, and leave the checkout
clean.
