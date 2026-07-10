#!/usr/bin/env bash
set -euo pipefail

SOURCE_REPOSITORY=https://github.com/openclaw/clawdex.git
PROTECTED_BRANCH=main
DRIVER_COMMIT=${1:-}
DESTINATION=${2:-}
IMAGE=${DESTINATION:+$DESTINATION.dmg}

usage() {
  echo "usage: $0 40-character-protected-main-commit destination" >&2
  exit 2
}

[[ "$DRIVER_COMMIT" =~ ^[0-9a-f]{40}$ && -n "$DESTINATION" ]] || usage
[[ "$DESTINATION" == /* ]] || {
  echo "release driver destination must be absolute" >&2
  exit 1
}
[[ ! -e "$DESTINATION" && ! -L "$DESTINATION" && ! -e "$IMAGE" && ! -L "$IMAGE" ]] || {
  echo "refusing to overwrite release driver destination: $DESTINATION" >&2
  exit 1
}
destination_parent=$(dirname "$DESTINATION")
[[ -d "$destination_parent" && ! -L "$destination_parent" ]] || {
  echo "release driver destination parent must be a real directory" >&2
  exit 1
}

# This stage must finish before release-mac-app opens the signing keychain or
# exposes the notary profile. Refuse common release credentials defensively.
for credential_name in CODESIGN_IDENTITY NOTARYTOOL_PROFILE GH_TOKEN GITHUB_TOKEN \
  MAC_RELEASE_CODESIGN_KEYCHAIN MAC_RELEASE_CODESIGN_KEYCHAIN_PASSWORD \
  CLAWDEX_PUBLICATION_HMAC_KEY; do
  [[ -z "${!credential_name:-}" ]] || {
    echo "unset $credential_name before materializing the release driver" >&2
    exit 1
  }
done

for tool in diskutil git hdiutil jq plutil shasum tar; do
  command -v "$tool" >/dev/null || {
    echo "missing required tool: $tool" >&2
    exit 1
  }
done

umask 077
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-driver-source.XXXXXX")
DRIVER_HOME="$WORK_DIR/home"
REPOSITORY="$WORK_DIR/repository"
ARCHIVE="$WORK_DIR/driver.tar"
STAGE=$(mktemp -d "$destination_parent/.clawdex-driver.XXXXXX")
MOUNTED=0
KEEP_MOUNT=0
mkdir -m 0700 "$DRIVER_HOME"
cleanup() {
  if [[ "$MOUNTED" == 1 && "$KEEP_MOUNT" == 0 ]]; then
    hdiutil detach -quiet -force "$DESTINATION" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_MOUNT" == 0 ]]; then
    chmod u+w "$DESTINATION" 2>/dev/null || true
    rm -rf "$DESTINATION"
    rm -f "$IMAGE"
  fi
  rm -rf "$WORK_DIR"
  if [[ -n "$STAGE" ]]; then
    chmod -R u+w "$STAGE" 2>/dev/null || true
    rm -rf "$STAGE"
  fi
}
trap cleanup EXIT

clean_git() {
  env -i PATH="$PATH" HOME="$DRIVER_HOME" TMPDIR="$WORK_DIR" \
    GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0 \
    git -c core.fsmonitor=false -c core.hooksPath=/dev/null "$@"
}

clean_git init -q "$REPOSITORY"
clean_git -C "$REPOSITORY" remote add origin "$SOURCE_REPOSITORY"
clean_git -C "$REPOSITORY" fetch --quiet --no-tags origin \
  "refs/heads/$PROTECTED_BRANCH:refs/remotes/origin/$PROTECTED_BRANCH"
fetched_commit=$(clean_git -C "$REPOSITORY" \
  rev-parse "refs/remotes/origin/$PROTECTED_BRANCH^{commit}")
[[ "$fetched_commit" == "$DRIVER_COMMIT" ]] || {
  echo "protected $PROTECTED_BRANCH is $fetched_commit, expected $DRIVER_COMMIT" >&2
  exit 1
}
[[ "$(clean_git -C "$REPOSITORY" cat-file -t "$DRIVER_COMMIT")" == commit ]] || {
  echo "approved release driver is not a commit" >&2
  exit 1
}
driver_tree=$(clean_git -C "$REPOSITORY" rev-parse "$DRIVER_COMMIT^{tree}")
[[ "$driver_tree" =~ ^[0-9a-f]{40}$ ]] || {
  echo "could not resolve approved release driver tree" >&2
  exit 1
}

clean_git -C "$REPOSITORY" ls-tree -r "$DRIVER_COMMIT" > "$WORK_DIR/tree.txt"
if awk '$1 != "100644" && $1 != "100755" { bad = 1 } END { exit bad ? 0 : 1 }' \
  "$WORK_DIR/tree.txt"; then
  echo "release driver tree contains symlinks, submodules, or unsupported modes" >&2
  exit 1
fi

clean_git -C "$REPOSITORY" archive --format=tar --output="$ARCHIVE" "$DRIVER_COMMIT"
archive_sha=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
[[ "$archive_sha" =~ ^[0-9a-f]{64}$ ]] || {
  echo "could not hash approved release driver archive" >&2
  exit 1
}
tar -xf "$ARCHIVE" -C "$STAGE"
[[ -f "$STAGE/scripts/package-clawdex-release.sh" && \
  ! -L "$STAGE/scripts/package-clawdex-release.sh" && \
  -f "$STAGE/.github/release-allowed-signers" && \
  ! -L "$STAGE/.github/release-allowed-signers" ]] || {
  echo "approved release driver is missing required regular files" >&2
  exit 1
}
[[ -z "$(find "$STAGE" -type l -print -quit)" ]] || {
  echo "materialized release driver contains a symlink" >&2
  exit 1
}

jq -nS \
  --arg repository github.com/openclaw/clawdex \
  --arg branch "$PROTECTED_BRANCH" \
  --arg commit "$DRIVER_COMMIT" \
  --arg tree "$driver_tree" \
  --arg archive_sha256 "$archive_sha" \
  '{
    schema_version: 1,
    repository: $repository,
    protected_branch: $branch,
    commit: $commit,
    tree: $tree,
    archive_sha256: $archive_sha256
  }' > "$STAGE/.clawdex-release-driver.json"
chmod -R a-w "$STAGE"
hdiutil create -quiet -fs HFS+ -format UDRO \
  -volname "ClawdexReleaseDriver-${DRIVER_COMMIT:0:12}" \
  -srcfolder "$STAGE" "$IMAGE"
chmod a-w "$IMAGE"
mkdir -m 0700 "$DESTINATION"
hdiutil attach -quiet -readonly -nobrowse -mountpoint "$DESTINATION" "$IMAGE"
MOUNTED=1

diskutil info -plist "$DESTINATION" > "$WORK_DIR/mount.plist"
plutil -convert json -o "$WORK_DIR/mount.json" "$WORK_DIR/mount.plist"
jq -e --arg mount "$DESTINATION" '
  .MountPoint == $mount and .Writable == false and .WritableVolume == false
' "$WORK_DIR/mount.json" >/dev/null || {
  echo "release driver was not attached as a read-only volume" >&2
  exit 1
}
[[ -f "$DESTINATION/scripts/package-clawdex-release.sh" && \
  ! -L "$DESTINATION/scripts/package-clawdex-release.sh" && \
  -f "$DESTINATION/.clawdex-release-driver.json" && \
  ! -L "$DESTINATION/.clawdex-release-driver.json" && \
  -z "$(find "$DESTINATION" -type l -print -quit)" ]] || {
  echo "read-only release driver mount does not match the authenticated archive" >&2
  exit 1
}
KEEP_MOUNT=1

echo "materialized verified read-only release driver: mount=$DESTINATION image=$IMAGE commit=$DRIVER_COMMIT tree=$driver_tree archive_sha256=$archive_sha"
