# Install

`clawdex` ships as a single Go binary. Release builds inject the tag. Tagged
`go install` builds fall back to Go build metadata; untagged local builds report
`dev` unless a version is injected.

## Homebrew (macOS, Linux)

```bash
brew install openclaw/tap/clawdex
clawdex --version
```

The Homebrew formula lives in `openclaw/homebrew-tap`. It is updated only after
the release inventory and Darwin trust checks have passed.

## Go install

```bash
go install github.com/openclaw/clawdex/cmd/clawdex@latest
clawdex --version
```

Source builds require the Go version declared in
[`go.mod`](https://github.com/openclaw/clawdex/blob/main/go.mod).

## Build from source

```bash
git clone https://github.com/openclaw/clawdex.git
cd clawdex
go build -o ./bin/clawdex ./cmd/clawdex
./bin/clawdex --version
```

## GitHub release archives

Release assets are built by the shared OpenClaw Go CLI workflow from a frozen
protected-branch commit and workflow-owned annotated tag. Darwin binaries are
Foundation-signed and Apple-notarized with the permanent identifier
`org.openclaw.clawdex`. Independent native macOS verification and reproducible
non-Darwin rebuilds bind the inventory, checksums, release notes, and exact
artifact bytes before publication and the Homebrew handoff.

- `clawdex_<version>_darwin_amd64.tar.gz`
- `clawdex_<version>_darwin_arm64.tar.gz`
- `clawdex_<version>_linux_amd64.tar.gz`
- `clawdex_<version>_linux_arm64.tar.gz`
- `clawdex_<version>_windows_amd64.zip`
- `clawdex_<version>_windows_arm64.zip`
- `checksums.txt`
- `ASSET-INVENTORY.json`
- `RELEASE-NOTES.md`
- `SIGNING-MANIFEST.json`

Browse the [releases page](https://github.com/openclaw/clawdex/releases) for
the latest tag.

## Platform notes

- **macOS** is the most exercised target. Official archives contain hardened,
  timestamped, notarized binaries for Apple Silicon and Intel. `clawdex import
  apple` currently runs `Contacts.framework` access through a temporary Swift
  helper; permission belongs to that helper/toolchain identity and is not
  guaranteed to persist as `clawdex` permission. Use `--input` when permission
  behavior must be deterministic.
- **Linux** builds support markdown editing, notes, search, Git, Google
  imports through `gog`, and vCard export. Apple direct import is macOS-only.
- **Windows** binaries are produced but lightly tested; the Git layer assumes
  a working `git` on `PATH`.

## Verify the install

```bash
clawdex --version
clawdex --help
clawdex doctor
```

For an official Darwin archive, verify the strict Developer ID signature and
Apple's online notarization constraint:

```bash
codesign --verify --strict --check-notarization -R=notarized --verbose=2 clawdex
codesign -dvvv clawdex 2>&1
codesign -d -r- clawdex 2>&1
```

The metadata must report identifier `org.openclaw.clawdex`, Team ID
`FWJYW4S8P8`, OpenClaw Foundation Developer ID authority, hardened runtime, a
secure timestamp, and the release designated requirement. On macOS 26.5,
`spctl --assess --type execute` rejects valid Apple-notarized standalone tools
as code that is not an app; raw-CLI `spctl` success is therefore not a release
or installation requirement. The shared workflow verifies online notarization
and native execution on independent Intel and Apple Silicon runners.

After `clawdex init` (see [Quickstart](quickstart.md)), `clawdex doctor`
prints a one-shot health summary: config path, repo path, remote, person
count, and any avatar problems.

## Updating

- **Homebrew:** `brew upgrade clawdex`.
- **Go install:** rerun `go install github.com/openclaw/clawdex/cmd/clawdex@latest`.
- **Release archives:** download the new tarball and replace the binary.
- **Source:** `git pull && go build -o ./bin/clawdex ./cmd/clawdex`.

The on-disk markdown layout is forward-compatible across point releases. A
breaking layout change would ship a `clawdex doctor --repair` migration —
see [Doctor](doctor.md).
