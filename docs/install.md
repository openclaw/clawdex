# Install

`clawdex` ships as a single Go binary. The visible version is injected at
build time: release builds use the tag, local builds use `git describe`.

## Homebrew (macOS, Linux)

```bash
brew install steipete/tap/clawdex
clawdex --version
```

The Homebrew formula lives in `steipete/homebrew-tap` and is updated by the
clawdex release workflow after each tagged release.

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

Release assets are published by GoReleaser:

- `clawdex_<version>_darwin_amd64.tar.gz`
- `clawdex_<version>_darwin_arm64.tar.gz`
- `clawdex_<version>_linux_amd64.tar.gz`
- `clawdex_<version>_linux_arm64.tar.gz`
- `clawdex_<version>_windows_amd64.zip`
- `checksums.txt`

Browse the [releases page](https://github.com/openclaw/clawdex/releases) for
the latest tag.

## Platform notes

- **macOS** is the most exercised target. `clawdex import apple` reads
  Contacts via `Contacts.framework`, so the binary must be allowed in
  *Settings → Privacy & Security → Contacts* the first time you run it.
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
