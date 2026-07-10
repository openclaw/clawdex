#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${1:-}
ASSET_DIR=${2:-}
MODE=${3:---inventory-only}
SOURCE_DIR=${4:-}
IDENTIFIER=org.openclaw.clawdex
EXPECTED_AUTHORITY='Developer ID Application: OpenClaw Foundation (FWJYW4S8P8)'
EXPECTED_TEAM_ID=FWJYW4S8P8
REQUIREMENT="identifier \"$IDENTIFIER\" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = \"$EXPECTED_TEAM_ID\""

usage() {
  echo "usage: $0 vX.Y.Z asset-directory [--inventory-only|--non-darwin source-directory|--darwin-amd64|--darwin-arm64]" >&2
  exit 2
}

[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ && -d "$ASSET_DIR" ]] || usage
case "$MODE" in
  --inventory-only|--non-darwin|--darwin-amd64|--darwin-arm64) ;;
  *) usage ;;
esac
if [[ "$MODE" == --non-darwin ]]; then
  [[ -n "$SOURCE_DIR" ]] || usage
else
  [[ -z "$SOURCE_DIR" ]] || usage
fi
if [[ "$MODE" != --inventory-only && ! "${EXPECTED_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "candidate verification requires EXPECTED_COMMIT from the authenticated signed tag" >&2
  exit 1
fi
[[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]] || {
  echo "release verification requires GH_TOKEN and GITHUB_TOKEN to be absent" >&2
  exit 1
}
for tool in jq shasum tar unzip; do
  command -v "$tool" >/dev/null || {
    echo "missing required tool: $tool" >&2
    exit 1
  }
done

version_number=${VERSION#v}
archive_names=(
  "clawdex_${version_number}_darwin_amd64.tar.gz"
  "clawdex_${version_number}_darwin_arm64.tar.gz"
  "clawdex_${version_number}_linux_amd64.tar.gz"
  "clawdex_${version_number}_linux_arm64.tar.gz"
  "clawdex_${version_number}_windows_amd64.zip"
  "clawdex_${version_number}_windows_arm64.zip"
)
expected_names=("${archive_names[@]}" checksums.txt provenance.json)

contains() {
  local want=$1 value
  shift
  for value in "$@"; do
    [[ "$value" == "$want" ]] && return 0
  done
  return 1
}

shopt -s nullglob dotglob
actual_paths=("$ASSET_DIR"/*)
[[ "${#actual_paths[@]}" == "${#expected_names[@]}" ]] || {
  echo "release must contain exactly ${#expected_names[@]} assets" >&2
  exit 1
}
for path in "${actual_paths[@]}"; do
  [[ -f "$path" && ! -L "$path" ]] || {
    echo "release asset is not a regular file: $path" >&2
    exit 1
  }
  contains "$(basename "$path")" "${expected_names[@]}" || {
    echo "unexpected release asset: $(basename "$path")" >&2
    exit 1
  }
done
for name in "${expected_names[@]}"; do
  [[ -f "$ASSET_DIR/$name" && ! -L "$ASSET_DIR/$name" ]] || {
    echo "release asset missing: $name" >&2
    exit 1
  }
done

expected_checksums=$(
  cd "$ASSET_DIR"
  for asset in "${archive_names[@]}"; do
    shasum -a 256 "$asset"
  done
)
[[ "$(cat "$ASSET_DIR/checksums.txt")" == "$expected_checksums" ]] || {
  echo "checksums.txt is stale, malformed, or out of order" >&2
  exit 1
}

provenance="$ASSET_DIR/provenance.json"
jq -se \
  --arg repository github.com/openclaw/clawdex \
  --arg tag "$VERSION" \
  --arg identifier "$IDENTIFIER" \
  --arg team_id "$EXPECTED_TEAM_ID" \
  --arg authority "$EXPECTED_AUTHORITY" \
  'length == 1 and
   (.[0] |
     (keys == ["artifacts", "commit", "darwin", "driver_commit", "go_version", "release_notes", "repository", "schema_version", "source_date_epoch", "tag", "tag_object"]) and
     .schema_version == 3 and
     .repository == $repository and
     .tag == $tag and
     (.tag_object | test("^[0-9a-f]{40}$")) and
     (.commit | test("^[0-9a-f]{40}$")) and
     (.driver_commit | test("^[0-9a-f]{40}$")) and
     .go_version == "go1.26.5" and
     (.source_date_epoch | type == "number" and floor == . and . > 0) and
     (.release_notes | type == "array" and length > 0) and
     all(.release_notes[];
       type == "string" and
       startswith("- ") and
       (contains("\n") | not) and
       (contains("\r") | not)) and
     (.artifacts | type == "array" and length == 6) and
     all(.artifacts[];
       keys == ["name", "sha256"] and
       (.name | type == "string") and
       (.sha256 | test("^[0-9a-f]{64}$"))) and
     (.darwin | type == "array" and length == 2) and
     ([.darwin[].arch] | sort) == ["amd64", "arm64"] and
     all(.darwin[];
       keys == ["arch", "authority", "identifier", "notary_status", "notary_submission_id", "team_id"] and
       .identifier == $identifier and
       .team_id == $team_id and
       .authority == $authority and
       .notary_status == "Accepted" and
       (.notary_submission_id | test("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$"))))' \
  "$provenance" >/dev/null
if [[ -n "${EXPECTED_COMMIT:-}" ]]; then
  [[ "$EXPECTED_COMMIT" =~ ^[0-9a-f]{40}$ ]] || {
    echo "EXPECTED_COMMIT is invalid" >&2
    exit 1
  }
  jq -e --arg commit "$EXPECTED_COMMIT" '.commit == $commit' "$provenance" >/dev/null || {
    echo "provenance commit does not match release tag" >&2
    exit 1
  }
fi
if [[ -n "${EXPECTED_TAG_OBJECT:-}" ]]; then
  [[ "$EXPECTED_TAG_OBJECT" =~ ^[0-9a-f]{40}$ ]] || {
    echo "EXPECTED_TAG_OBJECT is invalid" >&2
    exit 1
  }
  jq -e --arg tag_object "$EXPECTED_TAG_OBJECT" '.tag_object == $tag_object' "$provenance" >/dev/null || {
    echo "provenance tag object does not match the trusted signed tag" >&2
    exit 1
  }
fi
if [[ -n "${EXPECTED_DRIVER_COMMIT:-}" ]]; then
  [[ "$EXPECTED_DRIVER_COMMIT" =~ ^[0-9a-f]{40}$ ]] || {
    echo "EXPECTED_DRIVER_COMMIT is invalid" >&2
    exit 1
  }
  jq -e --arg commit "$EXPECTED_DRIVER_COMMIT" '.driver_commit == $commit' "$provenance" >/dev/null || {
    echo "provenance driver does not match protected verifier" >&2
    exit 1
  }
fi
for asset in "${archive_names[@]}"; do
  hash=$(shasum -a 256 "$ASSET_DIR/$asset" | awk '{print $1}')
  jq -e --arg name "$asset" --arg hash "$hash" \
    '[.artifacts[] | select(.name == $name and .sha256 == $hash)] | length == 1' \
    "$provenance" >/dev/null || {
    echo "provenance does not match artifact: $asset" >&2
    exit 1
  }
  case "$asset" in
    *.tar.gz)
      [[ "$(tar -tzf "$ASSET_DIR/$asset")" == clawdex ]] || {
        echo "archive must contain only clawdex: $asset" >&2
        exit 1
      }
      metadata=$(tar -tvzf "$ASSET_DIR/$asset")
      mode=${metadata%%[[:space:]]*}
      member=${metadata##*[[:space:]]}
      [[ "$mode" == -rwxr-xr-x && "$member" == clawdex ]] || {
        echo "archive clawdex must be a regular executable with mode 0755: $asset" >&2
        exit 1
      }
      ;;
    *.zip)
      [[ "$(unzip -Z1 "$ASSET_DIR/$asset")" == clawdex.exe ]] || {
        echo "archive must contain only clawdex.exe: $asset" >&2
        exit 1
      }
      ;;
  esac
done

[[ "$MODE" != --inventory-only ]] || {
  echo "release inventory verified: $VERSION"
  exit 0
}

if [[ "$MODE" == --non-darwin ]]; then
  for tool in cmp git go; do
    command -v "$tool" >/dev/null || {
      echo "missing required tool: $tool" >&2
      exit 1
    }
  done
  [[ -d "$SOURCE_DIR" && ! -L "$SOURCE_DIR" ]] || {
    echo "trusted source checkout is not a real directory: $SOURCE_DIR" >&2
    exit 1
  }
  [[ "$(git -C "$SOURCE_DIR" rev-parse --is-inside-work-tree 2>/dev/null)" == true ]] || {
    echo "trusted source is not a Git checkout: $SOURCE_DIR" >&2
    exit 1
  }
  [[ "$(git -C "$SOURCE_DIR" cat-file -t "refs/tags/$VERSION")" == tag ]] || {
    echo "release ref is not an annotated tag: $VERSION" >&2
    exit 1
  }
  embedded_tag=$(git -C "$SOURCE_DIR" cat-file -p "refs/tags/$VERSION" |
    awk '/^tag / && embedded == "" { embedded = substr($0, 5) } END { print embedded }')
  [[ "$embedded_tag" == "$VERSION" ]] || {
    echo "signed tag object names $embedded_tag, expected $VERSION" >&2
    exit 1
  }
  source_tag_object=$(git -C "$SOURCE_DIR" rev-parse "refs/tags/$VERSION^{tag}")
  [[ "$source_tag_object" =~ ^[0-9a-f]{40}$ ]] || {
    echo "trusted source has no annotated release tag object: $VERSION" >&2
    exit 1
  }
  git -C "$SOURCE_DIR" \
    -c gpg.format=ssh \
    -c gpg.ssh.allowedSignersFile="$ROOT/.github/release-allowed-signers" \
    verify-tag "$VERSION" >/dev/null 2>&1 || {
    echo "release tag is not signed by a trusted Git signing key: $VERSION" >&2
    exit 1
  }
  source_commit=$(git -C "$SOURCE_DIR" rev-parse "refs/tags/$VERSION^{commit}") || {
    echo "could not resolve trusted release commit: $VERSION" >&2
    exit 1
  }
  source_head=$(git -C "$SOURCE_DIR" rev-parse HEAD) || {
    echo "could not resolve trusted source checkout head" >&2
    exit 1
  }
  [[ "$source_commit" =~ ^[0-9a-f]{40}$ && "$source_head" == "$source_commit" ]] || {
    echo "trusted source checkout does not match release tag: $VERSION" >&2
    exit 1
  }
  source_status=$(git -C "$SOURCE_DIR" status --porcelain --untracked-files=normal) || {
    echo "could not prove trusted source checkout is clean" >&2
    exit 1
  }
  [[ -z "$source_status" ]] || {
    echo "trusted source checkout is not clean" >&2
    exit 1
  }
  [[ -f "$SOURCE_DIR/CHANGELOG.md" && ! -L "$SOURCE_DIR/CHANGELOG.md" ]] || {
    echo "trusted source changelog is not a regular file" >&2
    exit 1
  }
  release_heading_prefix="## $version_number - "
  release_heading_count=$(awk -v prefix="$release_heading_prefix" \
    'index($0, prefix) == 1 { count++ } END { print count + 0 }' "$SOURCE_DIR/CHANGELOG.md")
  release_heading=$(awk -v prefix="$release_heading_prefix" \
    'index($0, prefix) == 1 { print; exit }' "$SOURCE_DIR/CHANGELOG.md")
  [[ "$release_heading_count" == 1 && -n "$release_heading" && "$release_heading" != *Unreleased* ]] || {
    echo "trusted signed-tag changelog must contain one finalized $version_number section" >&2
    exit 1
  }
  source_release_notes=$(awk -v prefix="$release_heading_prefix" '
    index($0, prefix) == 1 { in_release = 1; next }
    in_release && /^## / { exit }
    in_release && /^- / { print }
  ' "$SOURCE_DIR/CHANGELOG.md" | jq -Rsc 'split("\n") | map(select(length > 0))')
  [[ "$(jq 'length' <<<"$source_release_notes")" -gt 0 ]] || {
    echo "trusted signed-tag changelog has no entries for $version_number" >&2
    exit 1
  }
  jq -e --argjson notes "$source_release_notes" '.release_notes == $notes' \
    "$provenance" >/dev/null || {
    echo "provenance release notes do not match the trusted signed tag" >&2
    exit 1
  }
  jq -e --arg commit "$source_commit" '.commit == $commit' "$provenance" >/dev/null || {
    echo "provenance commit does not match trusted signed tag" >&2
    exit 1
  }
  jq -e --arg tag_object "$source_tag_object" '.tag_object == $tag_object' "$provenance" >/dev/null || {
    echo "provenance tag object does not match trusted signed tag" >&2
    exit 1
  }
  source_date_epoch=$(git -C "$SOURCE_DIR" show -s --format=%ct "$source_commit")
  jq -e --argjson epoch "$source_date_epoch" '.source_date_epoch == $epoch' "$provenance" >/dev/null || {
    echo "provenance timestamp does not match trusted signed tag" >&2
    exit 1
  }
  required_go="go$(awk '/^go / { print $2; exit }' "$SOURCE_DIR/go.mod")"
  actual_go=$(GOTOOLCHAIN=local GOENV=off go env GOVERSION)
  [[ "$required_go" == go1.26.5 && "$actual_go" == "$required_go" ]] || {
    echo "non-Darwin reproduction requires go1.26.5, found $actual_go for $required_go source" >&2
    exit 1
  }

  WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-reproduce.XXXXXX")
  trap 'rm -rf "$WORK_DIR"' EXIT
  targets=(linux/amd64 linux/arm64 windows/amd64 windows/arm64)
  for target in "${targets[@]}"; do
    goos=${target%/*}
    goarch=${target#*/}
    suffix=
    [[ "$goos" == windows ]] && suffix=.exe
    rebuilt="$WORK_DIR/rebuilt-${goos}-${goarch}${suffix}"
    (
      cd "$SOURCE_DIR"
      CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOWORK=off \
        GOTOOLCHAIN=local GOENV=off GOFLAGS=-mod=readonly \
        go build -trimpath -buildvcs=true \
        -ldflags "-s -w -X github.com/openclaw/clawdex/internal/cli.Version=$version_number" \
        -o "$rebuilt" ./cmd/clawdex
    )
    uploaded="$WORK_DIR/uploaded-${goos}-${goarch}${suffix}"
    if [[ "$goos" == windows ]]; then
      archive="$ASSET_DIR/clawdex_${version_number}_${goos}_${goarch}.zip"
      unzip -p "$archive" clawdex.exe > "$uploaded"
    else
      archive="$ASSET_DIR/clawdex_${version_number}_${goos}_${goarch}.tar.gz"
      tar -xOf "$archive" clawdex > "$uploaded"
    fi
    cmp -s "$rebuilt" "$uploaded" || {
      echo "release payload does not reproduce from trusted signed tag: $(basename "$archive")" >&2
      exit 1
    }
  done
  echo "non-Darwin release payloads reproduced: $VERSION $source_commit"
  exit 0
fi

[[ "$(uname -s)" == Darwin ]] || {
  echo "Darwin candidate verification must run on macOS" >&2
  exit 1
}
for tool in arch cmp codesign csreq go lipo; do
  command -v "$tool" >/dev/null || {
    echo "missing required tool: $tool" >&2
    exit 1
  }
done

case "$MODE" in
  --darwin-arm64)
    goarch=arm64
    signature_arch=arm64
    ;;
  --darwin-amd64)
    goarch=amd64
    signature_arch=x86_64
    ;;
esac
archive="$ASSET_DIR/clawdex_${version_number}_darwin_${goarch}.tar.gz"
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-verify.XXXXXX")
trap 'rm -rf "$WORK_DIR"' EXIT
candidate_home="$WORK_DIR/candidate-home"
mkdir -m 0700 "$candidate_home"
binary="$WORK_DIR/$goarch/clawdex"
mkdir -p "$(dirname "$binary")"
tar -xOf "$archive" clawdex > "$binary"
chmod 0755 "$binary"

go version -m -json "$binary" > "$WORK_DIR/build-info.json"
jq -e \
  --arg commit "$EXPECTED_COMMIT" \
  --arg goarch "$goarch" \
  'def setting($key): [.Settings[] | select(.Key == $key) | .Value];
   .GoVersion == "go1.26.5" and
   .Path == "github.com/openclaw/clawdex/cmd/clawdex" and
   .Main.Path == "github.com/openclaw/clawdex" and
   setting("vcs") == ["git"] and
   setting("vcs.revision") == [$commit] and
   setting("vcs.modified") == ["false"] and
   setting("-trimpath") == ["true"] and
   setting("CGO_ENABLED") == ["0"] and
   setting("GOOS") == ["darwin"] and
   setting("GOARCH") == [$goarch]' \
  "$WORK_DIR/build-info.json" >/dev/null || {
  echo "Darwin candidate build metadata does not match the authenticated source: $archive" >&2
  exit 1
}

codesign --verify --strict --check-notarization -R=notarized --verbose=2 "$binary"
codesign --verify --strict -R="$REQUIREMENT" --verbose=2 "$binary"
signature=$(codesign -dvvv "$binary" 2>&1)
grep -Fx "Identifier=$IDENTIFIER" <<<"$signature" >/dev/null
grep -Fx "TeamIdentifier=$EXPECTED_TEAM_ID" <<<"$signature" >/dev/null
grep -Fx "Authority=$EXPECTED_AUTHORITY" <<<"$signature" >/dev/null
grep -F '(runtime)' <<<"$signature" >/dev/null
grep -E '^Timestamp=' <<<"$signature" >/dev/null
[[ "$(lipo -archs "$binary")" == "$signature_arch" ]] || {
  echo "Darwin candidate is not exactly one $signature_arch slice: $archive" >&2
  exit 1
}
actual_requirement=$(codesign -d -r- "$binary" 2>&1 | awk '/^designated =>/ { print; exit }')
[[ -n "$actual_requirement" ]] || {
  echo "signed binary has no embedded designated requirement: $archive" >&2
  exit 1
}
csreq -r="$actual_requirement" -b "$WORK_DIR/actual.csreq"
csreq -r="designated => $REQUIREMENT" -b "$WORK_DIR/expected.csreq"
cmp -s "$WORK_DIR/expected.csreq" "$WORK_DIR/actual.csreq" || {
  echo "embedded designated requirement does not match release policy: $archive" >&2
  exit 1
}
[[ "$(env -i PATH="$PATH" HOME="$candidate_home" TMPDIR="$candidate_home" \
  arch -"$signature_arch" "$binary" --version)" == "$version_number" ]] || {
  echo "candidate version mismatch: $archive" >&2
  exit 1
}
echo "Darwin candidate verified: $VERSION $goarch"
