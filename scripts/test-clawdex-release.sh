#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
EXPECTED_AUTHORITY='Developer ID Application: OpenClaw Foundation (FWJYW4S8P8)'
# Literal policy text is passed to the isolated codesign mock.
# shellcheck disable=SC2089
EXPECTED_DESIGNATED='designated => identifier "org.openclaw.clawdex" and anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists and certificate leaf[subject.OU] = "FWJYW4S8P8"'

fail() {
  echo "release script test failed: $*" >&2
  exit 1
}

for script in download-clawdex-release-assets.sh materialize-clawdex-release-driver.sh \
  package-clawdex-release.sh \
  publish-clawdex-release.sh verify-clawdex-release.sh; do
  bash -n "$ROOT/scripts/$script"
done
[[ "$(grep -Fc "ref: \${{ github.workflow_sha }}" "$ROOT/.github/workflows/release-assets.yml")" == 3 ]] || \
  fail "release jobs do not pin the protected verifier to the workflow commit"
if grep -F 'types: [published]' "$ROOT/.github/workflows/release-assets.yml" >/dev/null; then
  fail "tag-controlled release events must not run the privileged verifier"
fi
grep -F 'unset GH_TOKEN GITHUB_TOKEN' "$ROOT/.github/workflows/release-assets.yml" >/dev/null
[[ "$(grep -Fc "env -i PATH=\"\$PATH\"" "$ROOT/.github/workflows/release-assets.yml")" == 2 ]] || \
  fail "release verifier does not receive a minimal environment"
grep -F '  validate-context:' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "release workflow has no unconditional context gate"
grep -F '    needs: validate-context' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "release inventory can bypass the context gate"
grep -F '    environment: clawdex-release' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publication has no protected environment gate"
grep -F "          PUBLICATION_RUN_ID: \${{ github.run_id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher does not bind its protected workflow run"
grep -F "          PUBLICATION_SNAPSHOT_ID: \${{ needs.verify-inventory.outputs.snapshot-id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher does not bind the approved snapshot ID"
grep -F "          PUBLICATION_SNAPSHOT_DIGEST: \${{ needs.verify-inventory.outputs.snapshot-digest }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher does not bind the approved snapshot digest"
grep -F "          CLAWDEX_PUBLICATION_HMAC_KEY: \${{ secrets.CLAWDEX_PUBLICATION_HMAC_KEY }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher does not receive its protected recovery authenticator"
if grep -Eq 'PUBLICATION_RECOVERY_NONCE|authorize-publication' \
  "$ROOT/.github/workflows/release-assets.yml" "$ROOT/scripts/publish-clawdex-release.sh" >/dev/null; then
  fail "release path still uses a readable artifact as a publication authenticator"
fi
grep -F '      - verify-inventory' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publication does not depend on inventory verification"
grep -F '      - verify-darwin' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publication does not depend on Darwin verification"
grep -F '      actions: read' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher cannot read its protected environment policy"
grep -F "      tag-object: \${{ steps.verify.outputs.tag-object }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "workflow does not propagate the verified signed tag object"
grep -F "      release-id: \${{ steps.download.outputs.release-id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "workflow does not bind publication to the verified draft ID"
grep -F "          EXPECTED_RELEASE_ID: \${{ needs.verify-inventory.outputs.release-id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "publisher does not receive the verified draft ID"
[[ "$(grep -Fc '          go-version: 1.26.5' "$ROOT/.github/workflows/release-assets.yml")" == 2 ]] || \
  fail "native verification jobs do not use the exact Go metadata reader"
[[ "$(grep -Fc 'download-clawdex-release-assets.sh' "$ROOT/.github/workflows/release-assets.yml")" == 1 ]] || \
  fail "workflow must download the mutable release exactly once"
grep -F "snapshot-id: \${{ steps.snapshot.outputs.artifact-id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || fail "approved snapshot ID is not exported"
grep -F "artifact-ids: \${{ needs.verify-inventory.outputs.snapshot-id }}" \
  "$ROOT/.github/workflows/release-assets.yml" >/dev/null || fail "Darwin jobs do not consume the approved snapshot"
grep -F '          retention-days: 14' "$ROOT/.github/workflows/release-assets.yml" >/dev/null || \
  fail "sealed snapshot does not cover the approval window"
grep -F 'identity-before.tsv' "$ROOT/scripts/publish-clawdex-release.sh" >/dev/null || \
  fail "publisher does not bind exact release asset identities"
grep -F 'published-redownload' "$ROOT/scripts/publish-clawdex-release.sh" >/dev/null || \
  fail "publisher does not re-download the published assets"
if grep -F 'immutable-releases' "$ROOT/scripts/publish-clawdex-release.sh" >/dev/null; then
  fail "publisher must not require or mutate repository immutable-release policy"
fi
if grep -Eiq '(^|[^[:alnum:]_])spctl([^[:alnum:]_]|$)' \
  "$ROOT/scripts/package-clawdex-release.sh" \
  "$ROOT/scripts/verify-clawdex-release.sh" \
  "$ROOT/.github/workflows/release-assets.yml"; then
  fail "standalone CLI verification must not use spctl"
fi
if grep -Eiq 'goreleaser|homebrew|gh release (create|upload)' "$ROOT/.github/workflows/release-assets.yml"; then
  fail "release workflow still contains a legacy publication or downstream mutation path"
fi
if grep -Eq 'candidate_version|--darwin-(amd64|arm64)' "$ROOT/scripts/package-clawdex-release.sh"; then
  fail "credentialed packager must not execute Darwin candidates"
fi
if grep -F "git -C \"\$ROOT\"" "$ROOT/scripts/package-clawdex-release.sh" >/dev/null; then
  fail "credentialed packager still trusts mutable worktree Git state"
fi
if grep -Eq '^[[:space:]]+- darwin_(amd64|arm64)$' "$ROOT/.goreleaser.yaml"; then
  fail "GoReleaser still emits unsigned Darwin assets"
fi

WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-release-test.XXXXXX")
trap 'chmod -R u+w "$WORK_DIR" 2>/dev/null || true; rm -rf "$WORK_DIR"' EXIT
FAKE_BIN="$WORK_DIR/bin"
mkdir -p "$FAKE_BIN"

cat > "$FAKE_BIN/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  -s) echo Darwin ;;
  -m) echo arm64 ;;
  *) echo Darwin ;;
esac
EOF

cat > "$FAKE_BIN/hdiutil" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
  create)
    source_dir=
    for ((i = 1; i <= $#; i++)); do
      if [[ "${!i}" == -srcfolder ]]; then
        next=$((i + 1))
        source_dir=${!next}
      fi
    done
    image=${!#}
    [[ -d "$source_dir" && -n "$image" ]]
    mkdir -p "$image.contents"
    cp -R "$source_dir/." "$image.contents"
    : > "$image"
    ;;
  attach)
    mount_point=
    for ((i = 1; i <= $#; i++)); do
      if [[ "${!i}" == -mountpoint ]]; then
        next=$((i + 1))
        mount_point=${!next}
      fi
    done
    image=${!#}
    [[ -d "$mount_point" && -d "$image.contents" ]]
    cp -R "$image.contents/." "$mount_point"
    chmod -R a-w "$mount_point"
    ;;
  detach) ;;
  *) exit 2 ;;
esac
EOF

cat > "$FAKE_BIN/diskutil" <<'EOF'
#!/usr/bin/env bash
mount_point=${!#}
mock_root=$(cd "$(dirname "$0")/.." && pwd)
writable=false
[[ ! -f "$mock_root/mock-writable-volume" ]] || writable=true
jq -n --arg mount "$mount_point" --argjson writable "$writable" \
  '{MountPoint: $mount, Writable: $writable, WritableVolume: $writable}'
EOF

cat > "$FAKE_BIN/plutil" <<'EOF'
#!/usr/bin/env bash
[[ "$#" == 5 && "$1" == -convert && "$2" == json && "$3" == -o ]]
output=$4
input=$5
if [[ "$input" == - ]]; then
  if [[ "$output" == - ]]; then
    cat
  else
    cat > "$output"
  fi
elif [[ "$output" == - ]]; then
  cat "$input"
else
  cp "$input" "$output"
fi
EOF

cat > "$FAKE_BIN/git" <<'EOF'
#!/usr/bin/env bash
while true; do
  case "${1:-}" in
    -C|-c) shift 2 ;;
    *) break ;;
  esac
done
case "${1:-}" in
  init)
    destination=${!#}
    mock_root=$(cd "$(dirname "$0")/.." && pwd)
    mkdir -p "$destination"
    printf 'module github.com/openclaw/clawdex\n\ngo 1.26.5\n' > "$destination/go.mod"
    cp "$mock_root/release-changelog.md" "$destination/CHANGELOG.md"
    ;;
  rev-parse)
    if [[ "${2:-}" == --is-inside-work-tree ]]; then
      echo true
    elif [[ "${2:-}" == refs/remotes/origin/main* ]]; then
      mock_root=$(cd "$(dirname "$0")/.." && pwd)
      if [[ -f "$mock_root/mock-protected-commit" ]]; then
        cat "$mock_root/mock-protected-commit"
      else
        echo 0123456789abcdef0123456789abcdef01234567
      fi
    else
      echo 0123456789abcdef0123456789abcdef01234567
    fi
    ;;
  show) echo 1700000000 ;;
  verify-tag) [[ "${MOCK_GIT_VERIFY_TAG:-ok}" == ok ]] ;;
  cat-file)
    case "${2:-}" in
      -t)
        if [[ "${3:-}" == 0123456789abcdef0123456789abcdef01234567 ]]; then
          echo commit
        else
          echo "${MOCK_TAG_TYPE:-tag}"
        fi
        ;;
      -p)
        printf 'object 0123456789abcdef0123456789abcdef01234567\n'
        printf 'type commit\n'
        printf 'tag %s\n' "${MOCK_EMBEDDED_TAG:-v0.1.1}"
        ;;
      *) exit 2 ;;
    esac
    ;;
  ls-tree)
    printf '100644 blob 0123456789abcdef0123456789abcdef01234567\tREADME.md\n'
    ;;
  archive)
    output=
    for argument in "$@"; do
      [[ "$argument" == --output=* ]] && output=${argument#--output=}
    done
    [[ -n "$output" ]]
    mock_root=$(cd "$(dirname "$0")/.." && pwd)
    driver_source=$(cat "$mock_root/mock-driver-source-path")
    archive_root=$(mktemp -d "${TMPDIR:-/tmp}/clawdex-mock-driver.XXXXXX")
    cp -R "$driver_source/." "$archive_root"
    rm -rf "$archive_root/.git" "$archive_root/dist"
    rm -f "$archive_root/coverage.out"
    /usr/bin/tar -cf "$output" -C "$archive_root" .
    rm -rf "$archive_root"
    ;;
  status) [[ "${MOCK_GIT_STATUS:-ok}" == ok ]] ;;
  remote|fetch|checkout|config|tag|diff) exit 0 ;;
  *) exit 2 ;;
esac
EOF

cat > "$FAKE_BIN/go" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == env && "${2:-}" == GOVERSION ]]; then
  echo go1.26.5
  exit 0
fi
if [[ "${1:-}" == version && "${2:-}" == -m && "${3:-}" == -json ]]; then
  case "${4:-}" in
    */amd64/*) goarch=amd64 ;;
    *) goarch=arm64 ;;
  esac
  jq -n \
    --arg go_version "${MOCK_BUILD_GO_VERSION:-go1.26.5}" \
    --arg path "${MOCK_BUILD_PATH:-github.com/openclaw/clawdex/cmd/clawdex}" \
    --arg module "${MOCK_BUILD_MODULE:-github.com/openclaw/clawdex}" \
    --arg commit "${MOCK_BUILD_COMMIT:-0123456789abcdef0123456789abcdef01234567}" \
    --arg modified "${MOCK_BUILD_MODIFIED:-false}" \
    --arg goarch "$goarch" \
    '{
      GoVersion: $go_version,
      Path: $path,
      Main: {Path: $module},
      Settings: [
        {Key: "vcs", Value: "git"},
        {Key: "vcs.revision", Value: $commit},
        {Key: "vcs.modified", Value: $modified},
        {Key: "-trimpath", Value: "true"},
        {Key: "CGO_ENABLED", Value: "0"},
        {Key: "GOOS", Value: "darwin"},
        {Key: "GOARCH", Value: $goarch}
      ]
    }'
  exit 0
fi
[[ "${1:-}" == build ]] || exit 2
mock_root=$(cd "$(dirname "$0")/.." && pwd)
printf '%s\n' "$PWD" >> "$mock_root/go-build.log"
output=
version=
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -o)
      output=$2
      shift 2
      ;;
    -ldflags)
      version=${2##*=}
      shift 2
      ;;
    *) shift ;;
  esac
done
[[ -n "$output" && -n "$version" ]]
{
  echo '#!/usr/bin/env bash'
  echo '[[ "${1:-}" == --version ]] || exit 2'
  echo '[[ -z "${GH_TOKEN:-}${GITHUB_TOKEN:-}${CODESIGN_IDENTITY:-}${NOTARYTOOL_PROFILE:-}" ]] || exit 3'
  echo '[[ "$HOME" == *candidate-home ]] || exit 4'
  printf 'echo %q\n' "$version"
} > "$output"
chmod 0755 "$output"
EOF

cat > "$FAKE_BIN/codesign" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${MOCK_CODESIGN_LOG:?}"
case " $* " in
  *' -dvvv '*)
    {
      echo 'Identifier=org.openclaw.clawdex'
      echo 'CodeDirectory v=20500 size=1 flags=0x10000(runtime) hashes=1+0 location=embedded'
      echo 'Timestamp=Jul 9, 2026 at 12:00:00'
      echo "Authority=${MOCK_CODESIGN_AUTHORITY:-Developer ID Application: OpenClaw Foundation (FWJYW4S8P8)}"
      echo 'TeamIdentifier=FWJYW4S8P8'
    } >&2
    ;;
  *' -r- '*)
    {
      echo 'Executable=/mock/clawdex'
      echo "${MOCK_DESIGNATED_REQUIREMENT:-${EXPECTED_DESIGNATED:?}}"
    } >&2
    ;;
esac
EOF

cat > "$FAKE_BIN/csreq" <<'EOF'
#!/usr/bin/env bash
[[ "$#" == 3 && "$1" == -r=* && "$2" == -b ]]
printf '%s' "${1#-r=}" > "$3"
EOF

cat > "$FAKE_BIN/lipo" <<'EOF'
#!/usr/bin/env bash
if [[ -n "${MOCK_LIPO_ARCHS:-}" ]]; then
  echo "$MOCK_LIPO_ARCHS"
  exit 0
fi
case "${2:-}" in
  */amd64/*|*/darwin-amd64/*) echo x86_64 ;;
  *) echo arm64 ;;
esac
EOF

cat > "$FAKE_BIN/arch" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${MOCK_EXECUTION_LOG:?}"
shift
exec "$@"
EOF

cat > "$FAKE_BIN/ditto" <<'EOF'
#!/usr/bin/env bash
[[ "$#" == 5 && "$1" == --norsrc && "$2" == -c && "$3" == -k ]]
zip -X -q -j "$5" "$4"
EOF

cat > "$FAKE_BIN/xcrun" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${MOCK_NOTARY_LOG:?}"
[[ "${1:-}" == notarytool && "${2:-}" == submit ]]
case "${3:-}" in
  *amd64*) id=11111111-1111-1111-1111-111111111111 ;;
  *) id=22222222-2222-2222-2222-222222222222 ;;
esac
printf '{"id":"%s","status":"%s"}\n' "$id" "${MOCK_NOTARY_STATUS:-Accepted}"
EOF

cat > "$FAKE_BIN/gh" <<'EOF'
#!/usr/bin/env bash
[[ -z "${CLAWDEX_PUBLICATION_HMAC_KEY:-}" ]] || exit 3
[[ "${1:-}" == api ]] || exit 2
shift
method=GET
endpoint=
body=
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --paginate) shift ;;
    --method)
      method=$2
      shift 2
      ;;
    -f)
      [[ "$2" == body=* ]] && body=${2#body=}
      shift 2
      ;;
    -H|-F)
      shift 2
      ;;
    -*) shift ;;
    *)
      [[ -z "$endpoint" ]] && endpoint=$1
      shift
      ;;
  esac
done
case "$endpoint" in
  repos/*/releases\?per_page=100) cat "${MOCK_GH_RELEASES_JSON:?}" ;;
  repos/*/releases/*/assets\?per_page=100) cat "${MOCK_GH_ASSETS_JSON:?}" ;;
  https://api.github.com/repos/*/releases/assets/*) cat "${MOCK_GH_ASSET_DIR:?}/${endpoint##*/}" ;;
  repos/*/git/ref/tags/*) cat "${MOCK_GH_TAG_JSON:?}" ;;
  repos/*/environments/clawdex-release) cat "${MOCK_GH_ENVIRONMENT_JSON:?}" ;;
  repos/*/releases/*)
    if [[ "$method" == PATCH ]]; then
      printf 'PATCH\n' >> "${MOCK_PUBLISH_LOG:?}"
      jq --arg body "$body" '.draft = false | .body = $body' \
        "${MOCK_GH_RELEASE_JSON:?}" > "${MOCK_GH_RELEASE_JSON:?}.next"
      mv "${MOCK_GH_RELEASE_JSON:?}.next" "${MOCK_GH_RELEASE_JSON:?}"
    fi
    cat "${MOCK_GH_RELEASE_JSON:?}"
    ;;
  *) exit 2 ;;
esac
EOF

chmod 0755 "$FAKE_BIN"/*
export PATH="$FAKE_BIN:$PATH"
export MOCK_CODESIGN_LOG="$WORK_DIR/codesign.log"
export MOCK_NOTARY_LOG="$WORK_DIR/notary.log"
export MOCK_EXECUTION_LOG="$WORK_DIR/execution.log"
export MOCK_GO_BUILD_LOG="$WORK_DIR/go-build.log"
printf '%s\n' "$ROOT" > "$WORK_DIR/mock-driver-source-path"
export CLAWDEX_RELEASE_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567
# shellcheck disable=SC2090
export EXPECTED_DESIGNATED
unset GH_TOKEN GITHUB_TOKEN

write_finalized_changelog() {
  awk '
    /^## 0\.1\.1 - Unreleased$/ { print "## 0.1.1 - 2026-07-09"; next }
    { print }
  ' "$ROOT/CHANGELOG.md" > "$WORK_DIR/release-changelog.md"
}
write_finalized_changelog

if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" \
  bash "$ROOT/scripts/materialize-clawdex-release-driver.sh" \
    "$CLAWDEX_RELEASE_DRIVER_COMMIT" "$WORK_DIR/credentialed-driver" >/dev/null 2>&1; then
  fail "release driver materialization accepted signing credentials"
fi
printf '%s\n' ffffffffffffffffffffffffffffffffffffffff > "$WORK_DIR/mock-protected-commit"
if bash "$ROOT/scripts/materialize-clawdex-release-driver.sh" \
  "$CLAWDEX_RELEASE_DRIVER_COMMIT" "$WORK_DIR/wrong-protected-head" >/dev/null 2>&1; then
  fail "release driver materialization accepted a different protected head"
fi
rm "$WORK_DIR/mock-protected-commit"
DRIVER_ROOT="$WORK_DIR/release-driver"
bash "$ROOT/scripts/materialize-clawdex-release-driver.sh" \
  "$CLAWDEX_RELEASE_DRIVER_COMMIT" "$DRIVER_ROOT" >/dev/null
PACKAGER="$DRIVER_ROOT/scripts/package-clawdex-release.sh"
[[ ! -e "$DRIVER_ROOT/.git" && -f "$DRIVER_ROOT/.clawdex-release-driver.json" ]] || \
  fail "release driver was not materialized as an archive"
[[ -z "$(find "$DRIVER_ROOT" \( -perm -200 -o -perm -020 -o -perm -002 \) -print -quit)" ]] || \
  fail "materialized release driver is writable"
[[ -f "$DRIVER_ROOT.dmg" ]] || fail "release driver disk image is missing"

if bash "$ROOT/scripts/verify-clawdex-release.sh" \
  v0.1.1-rc.1 "$WORK_DIR" --inventory-only >/dev/null 2>&1; then
  fail "stable release tooling accepted a prerelease tag"
fi

if CODESIGN_IDENTITY='Developer ID Application: Peter Steinberger (Y5PE65HELJ)' \
  NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/wrong-identity" >/dev/null 2>&1; then
  fail "personal signing identity was accepted"
fi
if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 >/dev/null 2>&1; then
  fail "release packaging accepted a missing external output directory"
fi
if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/no-notary" >/dev/null 2>&1; then
  fail "missing notary profile was accepted"
fi
if GH_TOKEN=test CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/token-present" >/dev/null 2>&1; then
  fail "release candidate executed with a GitHub token"
fi
if env -u CLAWDEX_RELEASE_DRIVER_COMMIT \
  CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/no-driver-pin" >/dev/null 2>&1; then
  fail "release packaging accepted an unpinned driver"
fi
if CLAWDEX_RELEASE_DRIVER_COMMIT=ffffffffffffffffffffffffffffffffffffffff \
  CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/wrong-driver-pin" >/dev/null 2>&1; then
  fail "release packaging accepted the wrong driver commit"
fi
chmod u+w "$PACKAGER"
if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" \
  NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/writable-driver" >/dev/null 2>&1; then
  fail "release packaging accepted a modified writable driver"
fi
chmod a-w "$PACKAGER"
printf 'writable\n' > "$WORK_DIR/mock-writable-volume"
if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" \
  NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/writable-volume" >/dev/null 2>&1; then
  fail "release packaging accepted a writable driver volume"
fi
rm "$WORK_DIR/mock-writable-volume"
cp "$ROOT/CHANGELOG.md" "$WORK_DIR/release-changelog.md"
if CODESIGN_IDENTITY="$EXPECTED_AUTHORITY" NOTARYTOOL_PROFILE=test-profile \
  bash "$PACKAGER" v0.1.1 "$WORK_DIR/unreleased-changelog" >/dev/null 2>&1; then
  fail "release packaging accepted an unfinalized signed changelog"
fi
write_finalized_changelog

export CODESIGN_IDENTITY="$EXPECTED_AUTHORITY"
export NOTARYTOOL_PROFILE=test-profile
RELEASE_DIR="$WORK_DIR/release"
bash "$PACKAGER" v0.1.1 "$RELEASE_DIR" >/dev/null
[[ ! -s "$MOCK_EXECUTION_LOG" ]] || fail "packager executed a candidate while signing state was active"
[[ "$(wc -l < "$MOCK_GO_BUILD_LOG" | tr -d ' ')" == 10 ]] || fail "unexpected release build count"
if grep -Fx "$ROOT" "$MOCK_GO_BUILD_LOG" >/dev/null; then
  fail "packager built from its mutable driver worktree"
fi

expected_names=(
  clawdex_0.1.1_darwin_amd64.tar.gz
  clawdex_0.1.1_darwin_arm64.tar.gz
  clawdex_0.1.1_linux_amd64.tar.gz
  clawdex_0.1.1_linux_arm64.tar.gz
  clawdex_0.1.1_windows_amd64.zip
  clawdex_0.1.1_windows_arm64.zip
  checksums.txt
  provenance.json
)
for name in "${expected_names[@]}"; do
  [[ -f "$RELEASE_DIR/$name" ]] || fail "missing release asset: $name"
done
[[ "$(find "$RELEASE_DIR" -type f | wc -l | tr -d ' ')" == "${#expected_names[@]}" ]] || fail "unexpected release inventory"
jq -e '.schema_version == 3 and .tag == "v0.1.1" and .tag_object == "0123456789abcdef0123456789abcdef01234567" and .go_version == "go1.26.5" and .driver_commit == "0123456789abcdef0123456789abcdef01234567" and (.release_notes | length > 0) and (.artifacts | length == 6) and (.darwin | length == 2)' \
  "$RELEASE_DIR/provenance.json" >/dev/null

TRUSTED_SOURCE="$WORK_DIR/trusted-source"
mkdir -p "$TRUSTED_SOURCE"
printf 'module github.com/openclaw/clawdex\n\ngo 1.26.5\n' > "$TRUSTED_SOURCE/go.mod"
cp "$WORK_DIR/release-changelog.md" "$TRUSTED_SOURCE/CHANGELOG.md"

if MOCK_GIT_VERIFY_TAG=fail EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$RELEASE_DIR" --non-darwin "$TRUSTED_SOURCE" >/dev/null 2>&1; then
  fail "non-Darwin verifier accepted an untrusted release tag"
fi
if MOCK_EMBEDDED_TAG=v0.1.0 EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$RELEASE_DIR" --non-darwin "$TRUSTED_SOURCE" >/dev/null 2>&1; then
  fail "non-Darwin verifier accepted a replayed signed tag object"
fi
if MOCK_GIT_STATUS=fail EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$RELEASE_DIR" --non-darwin "$TRUSTED_SOURCE" >/dev/null 2>&1; then
  fail "non-Darwin verifier accepted an unprovable source status"
fi

BAD_NOTES_DIR="$WORK_DIR/bad-release-notes"
cp -R "$RELEASE_DIR" "$BAD_NOTES_DIR"
jq '.release_notes[0] = "- Attacker-controlled release note"' \
  "$BAD_NOTES_DIR/provenance.json" > "$WORK_DIR/provenance-notes.json"
mv "$WORK_DIR/provenance-notes.json" "$BAD_NOTES_DIR/provenance.json"
EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$BAD_NOTES_DIR" --inventory-only >/dev/null
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$BAD_NOTES_DIR" --non-darwin "$TRUSTED_SOURCE" >/dev/null 2>&1; then
  fail "non-Darwin verifier accepted release notes outside the signed tag"
fi

grep -F -- '--options runtime --timestamp --identifier org.openclaw.clawdex' "$MOCK_CODESIGN_LOG" >/dev/null || fail "missing hardened runtime signature"
grep -F -- '--requirements =designated => identifier "org.openclaw.clawdex"' "$MOCK_CODESIGN_LOG" >/dev/null || fail "missing explicit designated requirement"
grep -F -- '--check-notarization -R=notarized' "$MOCK_CODESIGN_LOG" >/dev/null || fail "missing online notarization constraint"
grep -F -- '--keychain-profile test-profile --wait' "$MOCK_NOTARY_LOG" >/dev/null || fail "missing runtime notary profile"

if GH_TOKEN=test EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "candidate verifier accepted a GitHub token"
fi
if env -u EXPECTED_COMMIT \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "Darwin verifier accepted an unauthenticated source commit"
fi
if MOCK_BUILD_COMMIT=ffffffffffffffffffffffffffffffffffffffff \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "Darwin verifier accepted build metadata from another commit"
fi
if MOCK_BUILD_GO_VERSION=go1.26.4 \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "Darwin verifier accepted build metadata from the wrong Go toolchain"
fi
if MOCK_BUILD_MODIFIED=true \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "Darwin verifier accepted modified build metadata"
fi
if MOCK_CODESIGN_AUTHORITY='Developer ID Application: Peter Steinberger (Y5PE65HELJ)' \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "candidate verifier accepted the wrong signing authority"
fi
if MOCK_DESIGNATED_REQUIREMENT='designated => identifier "org.openclaw.wrong"' \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "candidate verifier accepted the wrong embedded designated requirement"
fi
if MOCK_LIPO_ARCHS='arm64 x86_64' \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --darwin-arm64 >/dev/null 2>&1; then
  fail "candidate verifier accepted a multi-architecture Darwin binary"
fi
if EXPECTED_DRIVER_COMMIT=ffffffffffffffffffffffffffffffffffffffff \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted the wrong release driver commit"
fi
if EXPECTED_TAG_OBJECT=ffffffffffffffffffffffffffffffffffffffff \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted the wrong signed tag object"
fi

BAD_DIR="$WORK_DIR/bad-inventory"
cp -R "$RELEASE_DIR" "$BAD_DIR"
printf 'unexpected\n' > "$BAD_DIR/extra"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$BAD_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted an extra asset"
fi

BAD_PROVENANCE_DIR="$WORK_DIR/bad-provenance"
cp -R "$RELEASE_DIR" "$BAD_PROVENANCE_DIR"
jq '.unexpected = true' "$BAD_PROVENANCE_DIR/provenance.json" > "$WORK_DIR/provenance-extra.json"
mv "$WORK_DIR/provenance-extra.json" "$BAD_PROVENANCE_DIR/provenance.json"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$BAD_PROVENANCE_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted extra provenance fields"
fi

MULTI_PROVENANCE_DIR="$WORK_DIR/multiple-provenance"
cp -R "$RELEASE_DIR" "$MULTI_PROVENANCE_DIR"
cp "$MULTI_PROVENANCE_DIR/provenance.json" "$WORK_DIR/valid-provenance.json"
printf '{"tag":"attacker-controlled"}\n' > "$MULTI_PROVENANCE_DIR/provenance.json"
cat "$WORK_DIR/valid-provenance.json" >> "$MULTI_PROVENANCE_DIR/provenance.json"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$MULTI_PROVENANCE_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted multiple top-level provenance documents"
fi

BAD_ARCH_DIR="$WORK_DIR/bad-provenance-arch"
cp -R "$RELEASE_DIR" "$BAD_ARCH_DIR"
jq '.darwin[1].arch = "amd64"' "$BAD_ARCH_DIR/provenance.json" > "$WORK_DIR/provenance-arch.json"
mv "$WORK_DIR/provenance-arch.json" "$BAD_ARCH_DIR/provenance.json"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" v0.1.1 "$BAD_ARCH_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted duplicate Darwin provenance"
fi

refresh_metadata() {
  local dir=$1 name hash artifacts='[]'
  (
    cd "$dir"
    for name in "${expected_names[@]:0:6}"; do
      shasum -a 256 "$name"
    done > checksums.txt
  )
  for name in "${expected_names[@]:0:6}"; do
    hash=$(shasum -a 256 "$dir/$name" | awk '{print $1}')
    artifacts=$(jq -c --arg name "$name" --arg sha256 "$hash" \
      '. + [{name: $name, sha256: $sha256}]' <<<"$artifacts")
  done
  jq --argjson artifacts "$artifacts" '.artifacts = $artifacts' \
    "$dir/provenance.json" > "$WORK_DIR/refreshed-provenance.json"
  mv "$WORK_DIR/refreshed-provenance.json" "$dir/provenance.json"
}

BAD_MODE_DIR="$WORK_DIR/bad-tar-mode"
cp -R "$RELEASE_DIR" "$BAD_MODE_DIR"
BAD_MODE_PAYLOAD="$WORK_DIR/bad-mode-payload"
mkdir -p "$BAD_MODE_PAYLOAD"
tar -xOf "$BAD_MODE_DIR/clawdex_0.1.1_linux_amd64.tar.gz" clawdex > "$BAD_MODE_PAYLOAD/clawdex"
chmod 0644 "$BAD_MODE_PAYLOAD/clawdex"
tar -czf "$BAD_MODE_DIR/clawdex_0.1.1_linux_amd64.tar.gz" -C "$BAD_MODE_PAYLOAD" clawdex
refresh_metadata "$BAD_MODE_DIR"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  bash "$ROOT/scripts/verify-clawdex-release.sh" \
    v0.1.1 "$BAD_MODE_DIR" --inventory-only >/dev/null 2>&1; then
  fail "candidate verifier accepted a non-executable tar member"
fi

non_darwin_assets=(
  clawdex_0.1.1_linux_amd64.tar.gz
  clawdex_0.1.1_linux_arm64.tar.gz
  clawdex_0.1.1_windows_amd64.zip
  clawdex_0.1.1_windows_arm64.zip
)
for asset in "${non_darwin_assets[@]}"; do
  ATTACK_DIR="$WORK_DIR/attacker-${asset//./-}"
  cp -R "$RELEASE_DIR" "$ATTACK_DIR"
  PAYLOAD_DIR="$WORK_DIR/payload-${asset//./-}"
  mkdir -p "$PAYLOAD_DIR"
  case "$asset" in
    *.tar.gz)
      printf 'attacker-controlled payload\n' > "$PAYLOAD_DIR/clawdex"
      chmod 0755 "$PAYLOAD_DIR/clawdex"
      tar -czf "$ATTACK_DIR/$asset" -C "$PAYLOAD_DIR" clawdex
      ;;
    *.zip)
      printf 'attacker-controlled payload\n' > "$PAYLOAD_DIR/clawdex.exe"
      (cd "$PAYLOAD_DIR" && zip -X -q "$ATTACK_DIR/$asset" clawdex.exe)
      ;;
  esac
  refresh_metadata "$ATTACK_DIR"
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
    bash "$ROOT/scripts/verify-clawdex-release.sh" \
      v0.1.1 "$ATTACK_DIR" --inventory-only >/dev/null
  if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
    bash "$ROOT/scripts/verify-clawdex-release.sh" \
      v0.1.1 "$ATTACK_DIR" --non-darwin "$TRUSTED_SOURCE" >/dev/null 2>&1; then
    fail "reproduction accepted attacker-controlled payload: $asset"
  fi
done

MOCK_GH_ASSET_DIR="$WORK_DIR/gh-assets"
MOCK_GH_RELEASES_JSON="$WORK_DIR/releases.json"
MOCK_GH_ASSETS_JSON="$WORK_DIR/assets.json"
MOCK_GH_RELEASE_JSON="$WORK_DIR/release.json"
MOCK_GH_TAG_JSON="$WORK_DIR/tag.json"
MOCK_GH_ENVIRONMENT_JSON="$WORK_DIR/environment.json"
MOCK_PUBLISH_LOG="$WORK_DIR/publish.log"
mkdir -p "$MOCK_GH_ASSET_DIR"
release_body=$(awk '
  /^## 0\.1\.1([[:space:]]|$)/ { in_release = 1; next }
  in_release && /^## / { exit }
  in_release && /^- / { print }
' "$ROOT/CHANGELOG.md")
release_body+=$'\n\n'
jq -n --arg body "$release_body" \
  '[{id: 42, tag_name: "v0.1.1", draft: true, prerelease: false, immutable: false, body: $body}]' \
  > "$MOCK_GH_RELEASES_JSON"
jq -n --arg body "$release_body" \
  '{id: 42, tag_name: "v0.1.1", draft: true, prerelease: false, immutable: false, body: $body}' \
  > "$MOCK_GH_RELEASE_JSON"
printf '%s\n' '{
  "ref":"refs/tags/v0.1.1",
  "object":{"type":"tag","sha":"0123456789abcdef0123456789abcdef01234567"}
}' > "$MOCK_GH_TAG_JSON"
printf '%s\n' '{
  "name":"clawdex-release",
  "protection_rules":[{
    "type":"required_reviewers",
    "prevent_self_review":true,
    "reviewers":[{"type":"User","reviewer":{"login":"release-reviewer"}}]
  }],
  "deployment_branch_policy":{"protected_branches":true,"custom_branch_policies":false}
}' > "$MOCK_GH_ENVIRONMENT_JSON"
: > "$MOCK_PUBLISH_LOG"
assets='[]'
id=1
for name in "${expected_names[@]}"; do
  cp "$RELEASE_DIR/$name" "$MOCK_GH_ASSET_DIR/$id"
  url="https://api.github.com/repos/openclaw/clawdex/releases/assets/$id"
  size=$(wc -c < "$RELEASE_DIR/$name" | tr -d ' ')
  digest="sha256:$(shasum -a 256 "$RELEASE_DIR/$name" | awk '{print $1}')"
  assets=$(jq -c --argjson id "$id" --arg name "$name" --arg url "$url" \
    --argjson size "$size" --arg digest "$digest" \
    '. + [{id: $id, name: $name, url: $url, state: "uploaded", size: $size, digest: $digest}]' \
    <<<"$assets")
  id=$((id + 1))
done
printf '%s\n' "$assets" > "$MOCK_GH_ASSETS_JSON"
export MOCK_GH_ASSET_DIR MOCK_GH_RELEASES_JSON MOCK_GH_ASSETS_JSON \
  MOCK_GH_RELEASE_JSON MOCK_GH_TAG_JSON MOCK_GH_ENVIRONMENT_JSON MOCK_PUBLISH_LOG
DOWNLOAD_DIR="$WORK_DIR/download"
DOWNLOAD_OUTPUT="$WORK_DIR/download-output"
GITHUB_OUTPUT="$DOWNLOAD_OUTPUT" GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/download-clawdex-release-assets.sh" v0.1.1 true "$DOWNLOAD_DIR" >/dev/null
[[ "$(cat "$DOWNLOAD_OUTPUT")" == release-id=42 ]] || fail "release downloader did not export the exact draft ID"
export EXPECTED_RELEASE_ID=42
export PUBLICATION_RUN_ID=12345
export PUBLICATION_SNAPSHOT_ID=67890
export PUBLICATION_SNAPSHOT_DIGEST=cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
export CLAWDEX_PUBLICATION_HMAC_KEY=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
for name in "${expected_names[@]}"; do
  cmp "$RELEASE_DIR/$name" "$DOWNLOAD_DIR/$name"
done
if env -u CLAWDEX_PUBLICATION_HMAC_KEY \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher accepted a missing protected recovery authenticator"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "missing-authenticator failure mutated the release"
if GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/download-clawdex-release-assets.sh" v0.1.1 false "$WORK_DIR/wrong-draft" >/dev/null 2>&1; then
  fail "draft release matched published lookup"
fi

jq '.[0].prerelease = true' "$MOCK_GH_RELEASES_JSON" > "$WORK_DIR/releases-prerelease.json"
mv "$WORK_DIR/releases-prerelease.json" "$MOCK_GH_RELEASES_JSON"
jq '.prerelease = true' "$MOCK_GH_RELEASE_JSON" > "$WORK_DIR/release-prerelease.json"
mv "$WORK_DIR/release-prerelease.json" "$MOCK_GH_RELEASE_JSON"
if GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/download-clawdex-release-assets.sh" v0.1.1 true "$WORK_DIR/prerelease-download" >/dev/null 2>&1; then
  fail "stable release downloader accepted prerelease metadata"
fi
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "stable release publisher accepted prerelease metadata"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "prerelease-metadata failure mutated the release"
jq '.[0].prerelease = false' "$MOCK_GH_RELEASES_JSON" > "$WORK_DIR/releases-stable.json"
mv "$WORK_DIR/releases-stable.json" "$MOCK_GH_RELEASES_JSON"
jq '.prerelease = false' "$MOCK_GH_RELEASE_JSON" > "$WORK_DIR/release-stable.json"
mv "$WORK_DIR/release-stable.json" "$MOCK_GH_RELEASE_JSON"

# Every failed precondition must stop before the publication mutation.
jq '.[0].body += "\n\nDownload from attacker.example"' "$MOCK_GH_RELEASES_JSON" \
  > "$WORK_DIR/releases-extra-body.json"
mv "$WORK_DIR/releases-extra-body.json" "$MOCK_GH_RELEASES_JSON"
jq '.body += "\n\nDownload from attacker.example"' "$MOCK_GH_RELEASE_JSON" \
  > "$WORK_DIR/release-extra-body.json"
mv "$WORK_DIR/release-extra-body.json" "$MOCK_GH_RELEASE_JSON"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher accepted release-body content outside sealed notes"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "release-body failure mutated the release"
jq --arg body "$release_body" '.[0].body = $body' "$MOCK_GH_RELEASES_JSON" \
  > "$WORK_DIR/releases-body-restored.json"
mv "$WORK_DIR/releases-body-restored.json" "$MOCK_GH_RELEASES_JSON"
jq --arg body "$release_body" '.body = $body' "$MOCK_GH_RELEASE_JSON" \
  > "$WORK_DIR/release-body-restored.json"
mv "$WORK_DIR/release-body-restored.json" "$MOCK_GH_RELEASE_JSON"

jq '.protection_rules[0].prevent_self_review = false' "$MOCK_GH_ENVIRONMENT_JSON" \
  > "$WORK_DIR/environment-unprotected.json"
mv "$WORK_DIR/environment-unprotected.json" "$MOCK_GH_ENVIRONMENT_JSON"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher accepted an unprotected release environment"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "environment-policy failure mutated the release"
jq '.protection_rules[0].prevent_self_review = true' "$MOCK_GH_ENVIRONMENT_JSON" \
  > "$WORK_DIR/environment-protected.json"
mv "$WORK_DIR/environment-protected.json" "$MOCK_GH_ENVIRONMENT_JSON"

jq '.object.sha = "ffffffffffffffffffffffffffffffffffffffff"' "$MOCK_GH_TAG_JSON" \
  > "$WORK_DIR/tag-moved.json"
mv "$WORK_DIR/tag-moved.json" "$MOCK_GH_TAG_JSON"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher accepted a moved live release tag"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "live-tag failure mutated the release"
jq '.object.sha = "0123456789abcdef0123456789abcdef01234567"' "$MOCK_GH_TAG_JSON" \
  > "$WORK_DIR/tag-restored.json"
mv "$WORK_DIR/tag-restored.json" "$MOCK_GH_TAG_JSON"

cp "$MOCK_GH_ASSET_DIR/1" "$WORK_DIR/asset-1.good"
printf 'different downloaded bytes\n' > "$MOCK_GH_ASSET_DIR/1"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher accepted a re-downloaded asset with different bytes"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "re-download failure mutated the release"
mv "$WORK_DIR/asset-1.good" "$MOCK_GH_ASSET_DIR/1"

# A release published outside this protected protocol has no unguessable
# same-run marker and must not be blessed as a retry.
jq '.[0].draft = false' "$MOCK_GH_RELEASES_JSON" > "$WORK_DIR/releases-unmarked.json"
mv "$WORK_DIR/releases-unmarked.json" "$MOCK_GH_RELEASES_JSON"
jq '.draft = false' "$MOCK_GH_RELEASE_JSON" > "$WORK_DIR/release-unmarked.json"
mv "$WORK_DIR/release-unmarked.json" "$MOCK_GH_RELEASE_JSON"
if EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publisher blessed an already-public release without its protected same-run marker"
fi
[[ ! -s "$MOCK_PUBLISH_LOG" ]] || fail "unmarked-public-release failure mutated the release"
jq '.[0].draft = true' "$MOCK_GH_RELEASES_JSON" > "$WORK_DIR/releases-draft-restored.json"
mv "$WORK_DIR/releases-draft-restored.json" "$MOCK_GH_RELEASES_JSON"
jq '.draft = true' "$MOCK_GH_RELEASE_JSON" > "$WORK_DIR/release-draft-restored.json"
mv "$WORK_DIR/release-draft-restored.json" "$MOCK_GH_RELEASE_JSON"

EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null
[[ "$(cat "$MOCK_PUBLISH_LOG")" == PATCH ]] || fail "publisher did not promote the verified draft"
jq -e '.draft == false and .prerelease == false and .immutable == false' "$MOCK_GH_RELEASE_JSON" >/dev/null || \
  fail "publisher did not support the approved mutable-release policy"
jq -e '.body | test("\\n\\n<!-- clawdex-publication:v1:12345:[0-9a-f]{64} -->$")' \
  "$MOCK_GH_RELEASE_JSON" >/dev/null || \
  fail "publication did not persist its protected same-run marker"

# A failed post-PATCH job can rerun against the exact draft ID captured by the
# successful inventory job. It must not issue a second publication mutation.
jq --slurpfile release "$MOCK_GH_RELEASE_JSON" \
  '.[0].draft = false | .[0].body = $release[0].body' \
  "$MOCK_GH_RELEASES_JSON" > "$WORK_DIR/releases-published.json"
mv "$WORK_DIR/releases-published.json" "$MOCK_GH_RELEASES_JSON"
EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1
[[ "$(cat "$MOCK_PUBLISH_LOG")" == PATCH ]] || fail "publication recovery repeated the PATCH"

if CLAWDEX_PUBLICATION_HMAC_KEY=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publication recovery accepted another run's marker"
fi
[[ "$(cat "$MOCK_PUBLISH_LOG")" == PATCH ]] || fail "wrong-marker failure mutated the release"

if EXPECTED_RELEASE_ID=99 \
  EXPECTED_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_TAG_OBJECT=0123456789abcdef0123456789abcdef01234567 \
  EXPECTED_DRIVER_COMMIT=0123456789abcdef0123456789abcdef01234567 \
  GITHUB_REPOSITORY=openclaw/clawdex GH_TOKEN=test \
  bash "$ROOT/scripts/publish-clawdex-release.sh" v0.1.1 "$RELEASE_DIR" >/dev/null 2>&1; then
  fail "publication recovery accepted a different release ID"
fi
[[ "$(cat "$MOCK_PUBLISH_LOG")" == PATCH ]] || fail "release-ID failure mutated the release"

echo "clawdex release script tests passed"
