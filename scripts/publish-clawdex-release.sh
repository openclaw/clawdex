#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${1:-}
ASSET_DIR=${2:-}
REPOSITORY=${GITHUB_REPOSITORY:-}
ENVIRONMENT=clawdex-release

usage() {
  echo "usage: $0 vX.Y.Z asset-directory" >&2
  exit 2
}

[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || usage
[[ -d "$ASSET_DIR" && ! -L "$ASSET_DIR" ]] || usage
[[ "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || usage
[[ -n "${GH_TOKEN:-}" ]] || {
  echo "GH_TOKEN is required to publish the verified draft" >&2
  exit 1
}
[[ "${EXPECTED_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "EXPECTED_COMMIT must bind publication to the signed release tag" >&2
  exit 1
}
[[ "${EXPECTED_TAG_OBJECT:-}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "EXPECTED_TAG_OBJECT must bind publication to the signed annotated tag" >&2
  exit 1
}
[[ "${EXPECTED_DRIVER_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || {
  echo "EXPECTED_DRIVER_COMMIT must bind publication to the protected verifier" >&2
  exit 1
}
[[ "${EXPECTED_RELEASE_ID:-}" =~ ^[0-9]+$ ]] || {
  echo "EXPECTED_RELEASE_ID must bind publication to the verified draft" >&2
  exit 1
}
[[ "${PUBLICATION_RUN_ID:-}" =~ ^[0-9]+$ ]] || {
  echo "PUBLICATION_RUN_ID must identify the protected workflow run" >&2
  exit 1
}
[[ "${PUBLICATION_SNAPSHOT_ID:-}" =~ ^[0-9]+$ ]] || {
  echo "PUBLICATION_SNAPSHOT_ID must identify the approved asset snapshot" >&2
  exit 1
}
[[ "${PUBLICATION_SNAPSHOT_DIGEST:-}" =~ ^[0-9a-f]{64}$ ]] || {
  echo "PUBLICATION_SNAPSHOT_DIGEST must bind the approved asset snapshot" >&2
  exit 1
}
[[ "${CLAWDEX_PUBLICATION_HMAC_KEY:-}" =~ ^[0-9a-f]{64}$ ]] || {
  echo "CLAWDEX_PUBLICATION_HMAC_KEY must be the protected 32-byte hex key" >&2
  exit 1
}
for tool in cmp gh jq python3 shasum; do
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

WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-publish.XXXXXX")
VERIFIER_HOME="$WORK_DIR/verifier-home"
mkdir -m 0700 "$VERIFIER_HOME"
trap 'rm -rf "$WORK_DIR"' EXIT

# Authenticate the exact publication intent before any API or candidate
# operation, then remove the environment secret. A same-run retry can
# recompute this marker; a contents-write token without the protected
# environment key cannot forge it for an out-of-band publication.
jq -cnS \
  --arg repository "$REPOSITORY" \
  --arg tag "$VERSION" \
  --arg commit "$EXPECTED_COMMIT" \
  --arg tag_object "$EXPECTED_TAG_OBJECT" \
  --arg driver_commit "$EXPECTED_DRIVER_COMMIT" \
  --argjson release_id "$EXPECTED_RELEASE_ID" \
  --argjson run_id "$PUBLICATION_RUN_ID" \
  --argjson snapshot_id "$PUBLICATION_SNAPSHOT_ID" \
  --arg snapshot_digest "$PUBLICATION_SNAPSHOT_DIGEST" \
  '{
    schema_version: 1,
    repository: $repository,
    tag: $tag,
    commit: $commit,
    tag_object: $tag_object,
    driver_commit: $driver_commit,
    release_id: $release_id,
    run_id: $run_id,
    snapshot_id: $snapshot_id,
    snapshot_digest: $snapshot_digest
  }' > "$WORK_DIR/publication-intent.json"
publication_mac=$(python3 - "$WORK_DIR/publication-intent.json" <<'PY'
import hashlib
import hmac
import os
from pathlib import Path
import sys

key = bytes.fromhex(os.environ["CLAWDEX_PUBLICATION_HMAC_KEY"])
intent = Path(sys.argv[1]).read_bytes()
print(hmac.new(key, intent, hashlib.sha256).hexdigest())
PY
)
unset CLAWDEX_PUBLICATION_HMAC_KEY
[[ "$publication_mac" =~ ^[0-9a-f]{64}$ ]] || {
  echo "could not authenticate the protected publication intent" >&2
  exit 1
}
publication_marker="<!-- clawdex-publication:v1:${PUBLICATION_RUN_ID}:${publication_mac} -->"

# Static verification receives only the explicit provenance pins. In particular,
# the publisher token is not inherited by the verifier.
env -i PATH="$PATH" HOME="$VERIFIER_HOME" TMPDIR="$WORK_DIR" \
  EXPECTED_COMMIT="$EXPECTED_COMMIT" \
  EXPECTED_TAG_OBJECT="$EXPECTED_TAG_OBJECT" \
  EXPECTED_DRIVER_COMMIT="$EXPECTED_DRIVER_COMMIT" \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    "$VERSION" "$ASSET_DIR" --inventory-only >/dev/null

api() {
  gh api \
    -H 'Accept: application/vnd.github+json' \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "$@"
}

# GitHub documents Get an environment as requiring repository Actions(read),
# which the protected publish job grants; it does not require Administration.
# This policy read is a precondition, never a fallback mutation. A token that
# cannot prove the approval policy stops before publication.
api "repos/$REPOSITORY/environments/$ENVIRONMENT" > "$WORK_DIR/environment.json"
jq -e --arg name "$ENVIRONMENT" '
  .name == $name and
  .deployment_branch_policy.protected_branches == true and
  .deployment_branch_policy.custom_branch_policies == false and
  any(.protection_rules[];
    .type == "required_reviewers" and
    .prevent_self_review == true and
    (.reviewers | type == "array" and length > 0))
' "$WORK_DIR/environment.json" >/dev/null || {
  echo "$ENVIRONMENT must require a non-self reviewer and protected branches" >&2
  exit 1
}

validate_live_tag() {
  local destination=$1
  api "repos/$REPOSITORY/git/ref/tags/$VERSION" > "$destination"
  jq -e --arg ref "refs/tags/$VERSION" --arg tag_object "$EXPECTED_TAG_OBJECT" '
    .ref == $ref and
    .object.type == "tag" and
    .object.sha == $tag_object
  ' "$destination" >/dev/null || {
    echo "live release ref does not match the verified signed tag object: $VERSION" >&2
    return 1
  }
}

validate_live_tag "$WORK_DIR/tag-initial.json"

api --paginate "repos/$REPOSITORY/releases?per_page=100" > "$WORK_DIR/release-pages.json"
imported_release="$WORK_DIR/release-match.json"
jq -cs --arg tag "$VERSION" --argjson id "$EXPECTED_RELEASE_ID" \
  '[.[][] | select(
    .id == $id and
    .tag_name == $tag and
    (.draft | type == "boolean") and
    .prerelease == false)]' \
  "$WORK_DIR/release-pages.json" > "$imported_release"
[[ "$(jq 'length' "$imported_release")" == 1 ]] || {
  echo "expected exact stable release id $EXPECTED_RELEASE_ID for $VERSION" >&2
  exit 1
}
release_id=$(jq -r '.[0].id' "$imported_release")
[[ "$release_id" == "$EXPECTED_RELEASE_ID" ]] || {
  echo "release API id does not match the verified draft" >&2
  exit 1
}
initial_draft=$(jq -r '.[0].draft' "$imported_release")
jq -e '.[0].body | type == "string" and length > 0' "$imported_release" >/dev/null || {
  echo "draft release body must contain the changelog notes" >&2
  exit 1
}
release_body_json=$(jq -c '.[0].body' "$imported_release")
canonical_body_json=$(jq -c '.release_notes | join("\n")' "$ASSET_DIR/provenance.json")
canonical_body=$(jq -r '.release_notes | join("\n")' "$ASSET_DIR/provenance.json")
published_body="${canonical_body}"$'\n\n'"${publication_marker}"
published_body_json=$(jq -n --arg body "$published_body" '$body')
expected_body_json=$canonical_body_json
if [[ "$initial_draft" == false ]]; then
  expected_body_json=$published_body_json
fi
jq -e --argjson expected "$expected_body_json" '
  .[0].body |
  gsub("\r\n"; "\n") |
  sub("\n+$"; "") == $expected
' "$imported_release" >/dev/null || {
  echo "release body does not match the sealed notes and protected publication marker" >&2
  exit 1
}

write_local_manifest() {
  local name size digest
  for name in "${expected_names[@]}"; do
    [[ -f "$ASSET_DIR/$name" && ! -L "$ASSET_DIR/$name" ]] || return 1
    size=$(wc -c < "$ASSET_DIR/$name" | tr -d ' ')
    digest="sha256:$(shasum -a 256 "$ASSET_DIR/$name" | awk '{print $1}')"
    printf '%s\t%s\t%s\n' "$name" "$size" "$digest"
  done | sort
}

validate_remote_assets() {
  local json=$1 content_out=$2 identity_out=$3 name
  [[ "$(jq 'length' "$json")" == "${#expected_names[@]}" ]] || {
    echo "draft release must contain exactly ${#expected_names[@]} assets" >&2
    return 1
  }
  jq -e '
    all(.[];
      (.id | type == "number" and floor == . and . > 0) and
      (.name | type == "string") and
      .state == "uploaded" and
      (.size | type == "number" and floor == . and . > 0) and
      (.digest | type == "string" and test("^sha256:[0-9a-f]{64}$")))
  ' "$json" >/dev/null || {
    echo "draft release asset metadata is incomplete" >&2
    return 1
  }
  for name in "${expected_names[@]}"; do
    [[ "$(jq --arg name "$name" '[.[] | select(.name == $name)] | length' "$json")" == 1 ]] || {
      echo "draft release asset missing or duplicated: $name" >&2
      return 1
    }
  done
  jq -r 'sort_by(.name)[] | [.name, (.size | tostring), .digest] | @tsv' \
    "$json" > "$content_out"
  jq -r 'sort_by(.name)[] | [(.id | tostring), .name, (.size | tostring), .digest, .state] | @tsv' \
    "$json" > "$identity_out"
}

write_local_manifest > "$WORK_DIR/local-manifest.tsv" || {
  echo "approved snapshot is missing a regular release asset" >&2
  exit 1
}

fetch_assets() {
  local destination=$1
  api --paginate "repos/$REPOSITORY/releases/$release_id/assets?per_page=100" |
    jq -cs '[.[][]]' > "$destination"
}

redownload_assets() {
  local json=$1 destination=$2 name api_url asset_id
  local api_prefix="https://api.github.com/repos/$REPOSITORY/releases/assets/"
  mkdir -m 0700 "$destination"
  for name in "${expected_names[@]}"; do
    api_url=$(jq -r --arg name "$name" '.[] | select(.name == $name) | .url' "$json")
    asset_id=${api_url#"$api_prefix"}
    [[ "$api_url" == "$api_prefix"* && "$asset_id" =~ ^[0-9]+$ ]] || {
      echo "release asset has an invalid API URL: $name" >&2
      return 1
    }
    api "$api_url" -H 'Accept: application/octet-stream' > "$destination/$name"
    cmp -s "$ASSET_DIR/$name" "$destination/$name" || {
      echo "re-downloaded release asset does not match the approved snapshot: $name" >&2
      return 1
    }
  done
}

fetch_assets "$WORK_DIR/assets-before.json"
validate_remote_assets "$WORK_DIR/assets-before.json" \
  "$WORK_DIR/content-before.tsv" "$WORK_DIR/identity-before.tsv"
cmp -s "$WORK_DIR/local-manifest.tsv" "$WORK_DIR/content-before.tsv" || {
  echo "draft release assets do not match the approved workflow snapshot" >&2
  exit 1
}

# Re-read the draft and asset identities immediately before publication. The
# workflow concurrency group prevents another run of this workflow from racing.
# It is not an authorization boundary against another principal with Releases
# API access; docs/RELEASING.md requires the owner to accept that trusted set.
api "repos/$REPOSITORY/releases/$release_id" > "$WORK_DIR/release-before.json"
jq -e --argjson id "$release_id" --arg tag "$VERSION" \
  --argjson expected_draft "$initial_draft" --argjson body "$release_body_json" \
  '.id == $id and .tag_name == $tag and .draft == $expected_draft and .prerelease == false and
   (.body // "") == $body' \
  "$WORK_DIR/release-before.json" >/dev/null || {
  echo "draft release state changed before publication" >&2
  exit 1
}
fetch_assets "$WORK_DIR/assets-ready.json"
validate_remote_assets "$WORK_DIR/assets-ready.json" \
  "$WORK_DIR/content-ready.tsv" "$WORK_DIR/identity-ready.tsv"
if ! cmp -s "$WORK_DIR/local-manifest.tsv" "$WORK_DIR/content-ready.tsv" ||
  ! cmp -s "$WORK_DIR/identity-before.tsv" "$WORK_DIR/identity-ready.tsv"; then
  echo "draft release assets changed before publication" >&2
  exit 1
fi
redownload_assets "$WORK_DIR/assets-ready.json" "$WORK_DIR/draft-redownload"

# Downloads take time, so refresh both the release and exact asset identities
# once more immediately before changing draft state.
api "repos/$REPOSITORY/releases/$release_id" > "$WORK_DIR/release-publish-ready.json"
jq -e --argjson id "$release_id" --arg tag "$VERSION" \
  --argjson expected_draft "$initial_draft" --argjson body "$release_body_json" \
  '.id == $id and .tag_name == $tag and .draft == $expected_draft and .prerelease == false and
   (.body // "") == $body' \
  "$WORK_DIR/release-publish-ready.json" >/dev/null || {
  echo "draft release state changed during re-download verification" >&2
  exit 1
}
fetch_assets "$WORK_DIR/assets-publish-ready.json"
validate_remote_assets "$WORK_DIR/assets-publish-ready.json" \
  "$WORK_DIR/content-publish-ready.tsv" "$WORK_DIR/identity-publish-ready.tsv"
if ! cmp -s "$WORK_DIR/local-manifest.tsv" "$WORK_DIR/content-publish-ready.tsv" ||
  ! cmp -s "$WORK_DIR/identity-ready.tsv" "$WORK_DIR/identity-publish-ready.tsv"; then
  echo "draft release assets changed during re-download verification" >&2
  exit 1
fi
validate_live_tag "$WORK_DIR/tag-publish-ready.json"

if [[ "$initial_draft" == true ]]; then
  api --method PATCH "repos/$REPOSITORY/releases/$release_id" \
    -F draft=false -f body="$published_body" > "$WORK_DIR/published.json"
  jq -e --argjson id "$release_id" --arg tag "$VERSION" --argjson body "$published_body_json" '
    .id == $id and .tag_name == $tag and .draft == false and .prerelease == false and
    (.body // "") == $body
  ' "$WORK_DIR/published.json" >/dev/null || {
    echo "GitHub did not publish the verified draft" >&2
    exit 1
  }
  release_body_json=$published_body_json
else
  echo "resuming verification of already-published release id $release_id" >&2
fi

fetch_assets "$WORK_DIR/assets-final.json"
validate_remote_assets "$WORK_DIR/assets-final.json" \
  "$WORK_DIR/content-final.tsv" "$WORK_DIR/identity-final.tsv"
if ! cmp -s "$WORK_DIR/local-manifest.tsv" "$WORK_DIR/content-final.tsv" ||
  ! cmp -s "$WORK_DIR/identity-publish-ready.tsv" "$WORK_DIR/identity-final.tsv"; then
  echo "published release assets do not match the approved snapshot" >&2
  exit 1
fi
redownload_assets "$WORK_DIR/assets-final.json" "$WORK_DIR/published-redownload"

api "repos/$REPOSITORY/releases/$release_id" > "$WORK_DIR/release-final.json"
jq -e --argjson id "$release_id" --arg tag "$VERSION" --argjson body "$release_body_json" '
  .id == $id and .tag_name == $tag and .draft == false and .prerelease == false and
  (.body // "") == $body
' "$WORK_DIR/release-final.json" >/dev/null || {
  echo "published release state changed during re-download verification" >&2
  exit 1
}
fetch_assets "$WORK_DIR/assets-after-download.json"
validate_remote_assets "$WORK_DIR/assets-after-download.json" \
  "$WORK_DIR/content-after-download.tsv" "$WORK_DIR/identity-after-download.tsv"
if ! cmp -s "$WORK_DIR/local-manifest.tsv" "$WORK_DIR/content-after-download.tsv" ||
  ! cmp -s "$WORK_DIR/identity-final.tsv" "$WORK_DIR/identity-after-download.tsv"; then
  echo "published release assets changed during re-download verification" >&2
  exit 1
fi
validate_live_tag "$WORK_DIR/tag-final.json"

manifest_sha=$(shasum -a 256 "$WORK_DIR/identity-after-download.tsv" | awk '{print $1}')
echo "published verified release: $VERSION id=$release_id asset_manifest_sha256=$manifest_sha"
