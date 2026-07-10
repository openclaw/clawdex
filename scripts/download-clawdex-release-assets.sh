#!/usr/bin/env bash
set -euo pipefail

VERSION=${1:-}
EXPECTED_DRAFT=${2:-}
OUT_DIR=${3:-}
REPOSITORY=${GITHUB_REPOSITORY:-}

usage() {
  echo "usage: $0 vX.Y.Z true|false output-directory" >&2
  exit 2
}

[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || usage
[[ "$EXPECTED_DRAFT" == true || "$EXPECTED_DRAFT" == false ]] || usage
[[ -n "$OUT_DIR" && "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || usage
[[ ! -e "$OUT_DIR" ]] || {
  echo "refusing to merge release downloads into an existing path: $OUT_DIR" >&2
  exit 1
}
[[ -n "${GH_TOKEN:-}" ]] || {
  echo "GH_TOKEN is required" >&2
  exit 1
}
for tool in gh jq; do
  command -v "$tool" >/dev/null || {
    echo "missing required tool: $tool" >&2
    exit 1
  }
done

version_number=${VERSION#v}
expected_names=(
  "clawdex_${version_number}_darwin_amd64.tar.gz"
  "clawdex_${version_number}_darwin_arm64.tar.gz"
  "clawdex_${version_number}_linux_amd64.tar.gz"
  "clawdex_${version_number}_linux_arm64.tar.gz"
  "clawdex_${version_number}_windows_amd64.zip"
  "clawdex_${version_number}_windows_arm64.zip"
  checksums.txt
  provenance.json
)
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-release-download.XXXXXX")
DOWNLOAD_DIR="$WORK_DIR/assets"
mkdir -p "$DOWNLOAD_DIR"
trap 'rm -rf "$WORK_DIR"' EXIT

gh api --paginate "repos/$REPOSITORY/releases?per_page=100" > "$WORK_DIR/release-pages.json"
release=$(jq -cs --arg tag "$VERSION" --argjson draft "$EXPECTED_DRAFT" \
  '[.[][] | select(.tag_name == $tag and .draft == $draft and .prerelease == false)]' \
  "$WORK_DIR/release-pages.json")
[[ "$(jq 'length' <<<"$release")" == 1 ]] || {
  echo "expected exactly one stable release for $VERSION with draft=$EXPECTED_DRAFT" >&2
  exit 1
}
release_id=$(jq -r '.[0].id' <<<"$release")
[[ "$release_id" =~ ^[0-9]+$ ]] || {
  echo "release has an invalid API id" >&2
  exit 1
}
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'release-id=%s\n' "$release_id" >> "$GITHUB_OUTPUT"
fi

gh api --paginate "repos/$REPOSITORY/releases/$release_id/assets?per_page=100" > "$WORK_DIR/asset-pages.json"
assets=$(jq -cs '[.[][]]' "$WORK_DIR/asset-pages.json")
[[ "$(jq 'length' <<<"$assets")" == "${#expected_names[@]}" ]] || {
  echo "release must contain exactly ${#expected_names[@]} assets" >&2
  exit 1
}
api_prefix="https://api.github.com/repos/$REPOSITORY/releases/assets/"
for name in "${expected_names[@]}"; do
  matches=$(jq -c --arg name "$name" '[.[] | select(.name == $name)]' <<<"$assets")
  [[ "$(jq 'length' <<<"$matches")" == 1 ]] || {
    echo "release asset missing or duplicated: $name" >&2
    exit 1
  }
  api_url=$(jq -r '.[0].url' <<<"$matches")
  asset_id=${api_url#"$api_prefix"}
  [[ "$api_url" == "$api_prefix"* && "$asset_id" =~ ^[0-9]+$ ]] || {
    echo "release asset has an invalid API URL: $name" >&2
    exit 1
  }
  gh api "$api_url" -H 'Accept: application/octet-stream' > "$DOWNLOAD_DIR/$name"
  [[ -s "$DOWNLOAD_DIR/$name" ]] || {
    echo "downloaded release asset is empty: $name" >&2
    exit 1
  }
done

mkdir -p "$(dirname "$OUT_DIR")"
mv "$DOWNLOAD_DIR" "$OUT_DIR"
echo "downloaded exact release inventory: $VERSION"
