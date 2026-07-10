#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${1:-}
OUT_DIR=${2:-}
IDENTIFIER=org.openclaw.clawdex
EXPECTED_AUTHORITY='Developer ID Application: OpenClaw Foundation (FWJYW4S8P8)'
EXPECTED_TEAM_ID=FWJYW4S8P8
SOURCE_REPOSITORY=https://github.com/openclaw/clawdex.git
REQUIREMENT="identifier \"$IDENTIFIER\" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = \"$EXPECTED_TEAM_ID\""

usage() {
  echo "usage: $0 vX.Y.Z absolute-output-directory" >&2
  exit 2
}

[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ && "$OUT_DIR" == /* ]] || usage
case "$OUT_DIR/" in
  "$ROOT/"*)
    echo "release output must be outside the read-only driver mount" >&2
    exit 1
    ;;
esac
[[ "$(uname -s)" == Darwin ]] || {
  echo "clawdex release packaging must run on macOS" >&2
  exit 1
}
[[ "$(uname -m)" == arm64 ]] || {
  echo "clawdex release packaging requires Apple Silicon" >&2
  exit 1
}
for tool in cmp codesign csreq diskutil ditto git go jq lipo plutil shasum ssh-keygen tar unzip xcrun zip; do
  command -v "$tool" >/dev/null || {
    echo "missing required tool: $tool" >&2
    exit 1
  }
done
[[ "${CLAWDEX_RELEASE_DRIVER_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "CLAWDEX_RELEASE_DRIVER_COMMIT must pin the approved protected-branch driver" >&2
  exit 1
}
DRIVER_METADATA="$ROOT/.clawdex-release-driver.json"
[[ ! -e "$ROOT/.git" && ! -L "$ROOT/.git" && \
  -f "$DRIVER_METADATA" && ! -L "$DRIVER_METADATA" ]] || {
  echo "run only from a credential-free materialized release driver" >&2
  exit 1
}
[[ -z "$(find "$ROOT" -type l -print -quit)" ]] || {
  echo "materialized release driver must not contain symlinks" >&2
  exit 1
}
[[ -z "$(find "$ROOT" \( -perm -200 -o -perm -020 -o -perm -002 \) -print -quit)" ]] || {
  echo "materialized release driver must remain read-only" >&2
  exit 1
}
driver_mount_json=$(diskutil info -plist "$ROOT" | plutil -convert json -o - -)
jq -e --arg mount "$ROOT" '
  .MountPoint == $mount and .Writable == false and .WritableVolume == false
' <<<"$driver_mount_json" >/dev/null || {
  echo "release driver must execute from its authenticated read-only mount" >&2
  exit 1
}
driver_commit=$(jq -er \
  --arg commit "$CLAWDEX_RELEASE_DRIVER_COMMIT" '
    (keys | sort) == ["archive_sha256", "commit", "protected_branch", "repository", "schema_version", "tree"] and
    .schema_version == 1 and
    .repository == "github.com/openclaw/clawdex" and
    .protected_branch == "main" and
    .commit == $commit and
    (.tree | type == "string" and test("^[0-9a-f]{40}$")) and
    (.archive_sha256 | type == "string" and test("^[0-9a-f]{64}$")) |
    if . then $commit else error("invalid release driver metadata") end
  ' "$DRIVER_METADATA") || {
  echo "materialized release driver metadata does not match the approved commit" >&2
  exit 1
}
[[ -n "${CODESIGN_IDENTITY:-}" && "$CODESIGN_IDENTITY" == "$EXPECTED_AUTHORITY" ]] || {
  echo "clawdex releases require $EXPECTED_AUTHORITY via release-mac-app codesign-run" >&2
  exit 1
}
[[ -n "${NOTARYTOOL_PROFILE:-}" ]] || {
  echo "NOTARYTOOL_PROFILE must name a runtime Keychain profile" >&2
  exit 1
}
[[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]] || {
  echo "unset GH_TOKEN and GITHUB_TOKEN before release packaging" >&2
  exit 1
}
[[ ! -e "$OUT_DIR" ]] || {
  echo "refusing to overwrite output path: $OUT_DIR" >&2
  exit 1
}

[[ -f "$ROOT/.github/release-allowed-signers" && ! -L "$ROOT/.github/release-allowed-signers" ]] || {
  echo "release driver has no regular allowed-signers trust anchor" >&2
  exit 1
}

WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-release.XXXXXX")
SOURCE_DIR="$WORK_DIR/source"
ASSET_DIR="$WORK_DIR/assets"
DRIVER_HOME="$WORK_DIR/driver-home"
mkdir -m 0700 "$DRIVER_HOME"
mkdir -p "$ASSET_DIR"
trap 'rm -rf "$WORK_DIR"' EXIT

clean_git() {
  env -i PATH="$PATH" HOME="$DRIVER_HOME" TMPDIR="$WORK_DIR" \
    GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0 git "$@"
}

clean_git init -q "$SOURCE_DIR"
clean_git -C "$SOURCE_DIR" remote add origin "$SOURCE_REPOSITORY"
clean_git -C "$SOURCE_DIR" fetch --quiet --no-tags origin \
  "refs/tags/$VERSION:refs/tags/$VERSION"
[[ "$(clean_git -C "$SOURCE_DIR" cat-file -t "refs/tags/$VERSION")" == tag ]] || {
  echo "release ref is not an annotated tag: $VERSION" >&2
  exit 1
}
embedded_tag=$(clean_git -C "$SOURCE_DIR" cat-file -p "refs/tags/$VERSION" |
  awk '/^tag / && embedded == "" { embedded = substr($0, 5) } END { print embedded }')
[[ "$embedded_tag" == "$VERSION" ]] || {
  echo "signed tag object names $embedded_tag, expected $VERSION" >&2
  exit 1
}
tag_object=$(clean_git -C "$SOURCE_DIR" rev-parse "refs/tags/$VERSION^{tag}")
[[ "$tag_object" =~ ^[0-9a-f]{40}$ ]] || {
  echo "release tag does not resolve to an annotated tag object: $VERSION" >&2
  exit 1
}
clean_git -C "$SOURCE_DIR" \
  -c gpg.format=ssh \
  -c gpg.ssh.allowedSignersFile="$ROOT/.github/release-allowed-signers" \
  verify-tag "$VERSION" >/dev/null 2>&1 || {
  echo "release tag is not signed by a trusted Git signing key: $VERSION" >&2
  exit 1
}
tag_commit=$(clean_git -C "$SOURCE_DIR" rev-parse "refs/tags/$VERSION^{commit}")
[[ "$tag_commit" =~ ^[0-9a-f]{40}$ ]] || {
  echo "release tag does not resolve to a commit: $VERSION" >&2
  exit 1
}
clean_git -C "$SOURCE_DIR" \
  -c core.hooksPath=/dev/null checkout --detach --quiet "$tag_commit"
source_head=$(clean_git -C "$SOURCE_DIR" rev-parse HEAD) || {
  echo "could not resolve materialized release source commit" >&2
  exit 1
}
source_status=$(clean_git -C "$SOURCE_DIR" status --porcelain --untracked-files=normal) || {
  echo "could not prove materialized release source is clean" >&2
  exit 1
}
[[ "$source_head" == "$tag_commit" && -z "$source_status" ]] || {
  echo "materialized release source does not exactly match $VERSION" >&2
  exit 1
}

required_go="go$(awk '/^go / { print $2; exit }' "$SOURCE_DIR/go.mod")"
actual_go=$(env -i PATH="$PATH" HOME="$DRIVER_HOME" TMPDIR="$WORK_DIR" \
  GOTOOLCHAIN=local GOENV=off go env GOVERSION)
[[ "$actual_go" == "$required_go" ]] || {
  echo "release requires $required_go, found $actual_go" >&2
  exit 1
}

version_number=${VERSION#v}
release_heading_prefix="## $version_number - "
release_heading_count=$(awk -v prefix="$release_heading_prefix" \
  'index($0, prefix) == 1 { count++ } END { print count + 0 }' "$SOURCE_DIR/CHANGELOG.md")
release_heading=$(awk -v prefix="$release_heading_prefix" \
  'index($0, prefix) == 1 { print; exit }' "$SOURCE_DIR/CHANGELOG.md")
[[ "$release_heading_count" == 1 && -n "$release_heading" && "$release_heading" != *Unreleased* ]] || {
  echo "signed release changelog must contain one finalized $version_number section" >&2
  exit 1
}
release_notes_json=$(awk -v prefix="$release_heading_prefix" '
  index($0, prefix) == 1 { in_release = 1; next }
  in_release && /^## / { exit }
  in_release && /^- / { print }
' "$SOURCE_DIR/CHANGELOG.md" | jq -Rsc 'split("\n") | map(select(length > 0))')
[[ "$(jq 'length' <<<"$release_notes_json")" -gt 0 ]] || {
  echo "signed release changelog has no entries for $version_number" >&2
  exit 1
}
source_date_epoch=$(clean_git -C "$SOURCE_DIR" show -s --format=%ct "$tag_commit")
[[ "$source_date_epoch" =~ ^[0-9]+$ ]] || {
  echo "could not resolve source timestamp" >&2
  exit 1
}
export SOURCE_DATE_EPOCH="$source_date_epoch" TZ=UTC

archive_names=(
  "clawdex_${version_number}_darwin_amd64.tar.gz"
  "clawdex_${version_number}_darwin_arm64.tar.gz"
  "clawdex_${version_number}_linux_amd64.tar.gz"
  "clawdex_${version_number}_linux_arm64.tar.gz"
  "clawdex_${version_number}_windows_amd64.zip"
  "clawdex_${version_number}_windows_arm64.zip"
)

build_binary() {
  local goos=$1 goarch=$2 output=$3
  (
    cd "$SOURCE_DIR"
    env -i PATH="$PATH" HOME="$DRIVER_HOME" TMPDIR="$WORK_DIR" \
      CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOWORK=off \
      GOTOOLCHAIN=local GOENV=off GOFLAGS=-mod=readonly \
      go build -trimpath -buildvcs=true \
      -ldflags "-s -w -X github.com/openclaw/clawdex/internal/cli.Version=$version_number" \
      -o "$output" ./cmd/clawdex
  )
  chmod 0755 "$output"
}

verify_signed_binary() {
  local binary=$1 expected_arch=$2 signature requirement_dir actual_requirement
  codesign --verify --strict --check-notarization -R=notarized --verbose=2 "$binary"
  codesign --verify --strict -R="$REQUIREMENT" --verbose=2 "$binary"
  signature=$(codesign -dvvv "$binary" 2>&1)
  grep -Fx "Identifier=$IDENTIFIER" <<<"$signature" >/dev/null
  grep -Fx "TeamIdentifier=$EXPECTED_TEAM_ID" <<<"$signature" >/dev/null
  grep -Fx "Authority=$EXPECTED_AUTHORITY" <<<"$signature" >/dev/null
  grep -F '(runtime)' <<<"$signature" >/dev/null
  grep -E '^Timestamp=' <<<"$signature" >/dev/null
  [[ "$(lipo -archs "$binary")" == "$expected_arch" ]] || {
    echo "signed binary is not exactly one $expected_arch slice: $binary" >&2
    return 1
  }

  actual_requirement=$(codesign -d -r- "$binary" 2>&1 | awk '/^designated =>/ { print; exit }')
  [[ -n "$actual_requirement" ]] || {
    echo "signed binary has no embedded designated requirement: $binary" >&2
    return 1
  }
  requirement_dir=$(mktemp -d "$WORK_DIR/requirements.XXXXXX")
  csreq -r="$actual_requirement" -b "$requirement_dir/actual.csreq"
  csreq -r="designated => $REQUIREMENT" -b "$requirement_dir/expected.csreq"
  cmp -s "$requirement_dir/expected.csreq" "$requirement_dir/actual.csreq" || {
    echo "embedded designated requirement does not match release policy: $binary" >&2
    return 1
  }
}

notary_arm64=
notary_amd64=
build_darwin() {
  local goarch=$1 signature_arch=$2 stage binary notary_zip notary_json status submission_id asset
  stage="$WORK_DIR/darwin-$goarch"
  binary="$stage/clawdex"
  mkdir -p "$stage"
  build_binary darwin "$goarch" "$binary"
  codesign --force --options runtime --timestamp --identifier "$IDENTIFIER" \
    --requirements "=designated => $REQUIREMENT" \
    --sign "$CODESIGN_IDENTITY" "$binary"
  codesign --verify --strict -R="$REQUIREMENT" --verbose=2 "$binary"

  notary_zip="$WORK_DIR/clawdex-${goarch}-notary.zip"
  (cd "$stage" && ditto --norsrc -c -k clawdex "$notary_zip")
  notary_json="$WORK_DIR/notary-${goarch}.json"
  xcrun notarytool submit "$notary_zip" \
    --keychain-profile "$NOTARYTOOL_PROFILE" \
    --wait --timeout 30m --output-format json --no-s3-acceleration > "$notary_json"
  status=$(jq -er '.status' "$notary_json")
  submission_id=$(jq -er '.id' "$notary_json")
  [[ "$status" == Accepted && "$submission_id" =~ ^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$ ]] || {
    echo "notarization was not accepted for darwin/$goarch" >&2
    exit 1
  }
  verify_signed_binary "$binary" "$signature_arch"

  asset="clawdex_${version_number}_darwin_${goarch}.tar.gz"
  (cd "$stage" && COPYFILE_DISABLE=1 tar -czf "$ASSET_DIR/$asset" clawdex)
  if [[ "$goarch" == arm64 ]]; then
    notary_arm64=$submission_id
  else
    notary_amd64=$submission_id
  fi
}

build_archive() {
  local goos=$1 goarch=$2 stage binary asset
  stage="$WORK_DIR/$goos-$goarch"
  mkdir -p "$stage"
  if [[ "$goos" == windows ]]; then
    binary="$stage/clawdex.exe"
    asset="clawdex_${version_number}_${goos}_${goarch}.zip"
    build_binary "$goos" "$goarch" "$binary"
    (cd "$stage" && zip -X -q "$ASSET_DIR/$asset" clawdex.exe)
  else
    binary="$stage/clawdex"
    asset="clawdex_${version_number}_${goos}_${goarch}.tar.gz"
    build_binary "$goos" "$goarch" "$binary"
    (cd "$stage" && COPYFILE_DISABLE=1 tar -czf "$ASSET_DIR/$asset" clawdex)
  fi
}

build_darwin amd64 x86_64
build_darwin arm64 arm64
build_archive linux amd64
build_archive linux arm64
build_archive windows amd64
build_archive windows arm64

(
  cd "$ASSET_DIR"
  for asset in "${archive_names[@]}"; do
    shasum -a 256 "$asset"
  done > checksums.txt
)

artifacts_json='[]'
for asset in "${archive_names[@]}"; do
  hash=$(shasum -a 256 "$ASSET_DIR/$asset" | awk '{print $1}')
  artifacts_json=$(jq -c --arg name "$asset" --arg sha256 "$hash" \
    '. + [{name: $name, sha256: $sha256}]' <<<"$artifacts_json")
done
jq -n \
  --arg repository github.com/openclaw/clawdex \
  --arg tag "$VERSION" \
  --arg tag_object "$tag_object" \
  --arg commit "$tag_commit" \
  --arg driver_commit "$driver_commit" \
  --arg go_version "$actual_go" \
  --arg identifier "$IDENTIFIER" \
  --arg team_id "$EXPECTED_TEAM_ID" \
  --arg authority "$EXPECTED_AUTHORITY" \
  --arg notary_amd64 "$notary_amd64" \
  --arg notary_arm64 "$notary_arm64" \
  --argjson source_date_epoch "$source_date_epoch" \
  --argjson release_notes "$release_notes_json" \
  --argjson artifacts "$artifacts_json" \
  '{
    schema_version: 3,
    repository: $repository,
    tag: $tag,
    tag_object: $tag_object,
    commit: $commit,
    driver_commit: $driver_commit,
    go_version: $go_version,
    source_date_epoch: $source_date_epoch,
    release_notes: $release_notes,
    artifacts: $artifacts,
    darwin: [
      {arch: "amd64", identifier: $identifier, team_id: $team_id, authority: $authority, notary_submission_id: $notary_amd64, notary_status: "Accepted"},
      {arch: "arm64", identifier: $identifier, team_id: $team_id, authority: $authority, notary_submission_id: $notary_arm64, notary_status: "Accepted"}
    ]
  }' > "$ASSET_DIR/provenance.json"

env -i PATH="$PATH" HOME="$DRIVER_HOME" TMPDIR="$WORK_DIR" \
  EXPECTED_TAG_OBJECT="$tag_object" EXPECTED_COMMIT="$tag_commit" \
  EXPECTED_DRIVER_COMMIT="$driver_commit" \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
  "$VERSION" "$ASSET_DIR" --non-darwin "$SOURCE_DIR"

mkdir -p "$(dirname "$OUT_DIR")"
mv "$ASSET_DIR" "$OUT_DIR"
echo "verified release assets: $OUT_DIR"
