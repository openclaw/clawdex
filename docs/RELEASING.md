---
summary: "Release checklist for clawdex (GitHub release binaries via GoReleaser + Homebrew tap update)"
---

# Releasing `clawdex`

Always do all steps below. No partial releases.

Assumptions:
- Repo: `openclaw/clawdex`
- Binary: `clawdex`
- GoReleaser config: `.goreleaser.yaml`
- Homebrew tap repo: `~/Projects/homebrew-tap`
- Tap workflow: `steipete/homebrew-tap/.github/workflows/update-formula.yml`

## 0) Prereqs

- Clean working tree on `main`
- Go toolchain from `go.mod`
- GitHub CLI authenticated
- CI green on `main`
- `HOMEBREW_TAP_TOKEN` set in `openclaw/clawdex` Actions secrets

## 1) Verify build + tests

```sh
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.1 run
go test -count=1 ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -n 1
go test -count=1 -race ./...
go build -o /tmp/clawdex ./cmd/clawdex
goreleaser release --snapshot --clean --skip=publish
gh run list -L 5 --branch main
```

Coverage floor: `90%+`

## 2) Update changelog

Add a new section in `CHANGELOG.md`.

Example:

- `## 0.2.0 - 2026-05-08`

## 3) Commit, tag, push

```sh
git checkout main
git pull --ff-only origin main
git commit -am "release: vX.Y.Z"
git tag -a vX.Y.Z -m "Release X.Y.Z"
git push origin main --tags
```

## 4) Verify GitHub release assets

The tag push triggers `.github/workflows/release.yml`.

```sh
gh run list -L 5 --workflow release.yml
gh release view vX.Y.Z
```

Confirm assets exist for:

- `darwin_amd64`
- `darwin_arm64`
- `linux_amd64`
- `linux_arm64`
- `windows_amd64`
- `windows_arm64`

## 5) Verify Homebrew tap update

The release workflow dispatches the tap updater after GoReleaser succeeds. The
tap updater rewrites `Formula/clawdex.rb` with the release archive checksums.

```sh
gh run list --repo steipete/homebrew-tap --workflow update-formula.yml -L 5
brew update
brew reinstall steipete/tap/clawdex
clawdex --version
brew test steipete/tap/clawdex
```

## Notes

- Build-time version stamping comes from `-X github.com/openclaw/clawdex/internal/cli.Version={{ .Version }}`
- If release workflow needs a rerun:

```sh
gh workflow run release.yml -f tag=vX.Y.Z
```
