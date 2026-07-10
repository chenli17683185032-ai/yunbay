#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
builder="$repo_root/deploy/build-source-release.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fixture_repo="$tmp/repo with spaces"
archive="$tmp/release output/release artifact.tar.gz"
unpacked="$tmp/unpacked"

git init -q "$fixture_repo"
git -C "$fixture_repo" config user.name 'Release Builder Test'
git -C "$fixture_repo" config user.email 'release-builder-test@example.invalid'
printf 'tracked commit content\n' >"$fixture_repo/tracked.txt"
git -C "$fixture_repo" add tracked.txt
git -C "$fixture_repo" commit -qm 'add tracked fixture'
fixture_sha=$(git -C "$fixture_repo" rev-parse HEAD)

printf 'uncommitted working tree content\n' >"$fixture_repo/tracked.txt"
printf 'ordinary untracked fixture\n' >"$fixture_repo/untracked.txt"
printf 'untracked AppleDouble fixture\n' >"$fixture_repo/._junk"
printf 'untracked Finder fixture\n' >"$fixture_repo/.DS_Store"

"$builder" "$fixture_repo" "$fixture_sha" "$archive"

mkdir -p "$unpacked"
tar -xzf "$archive" -C "$unpacked"

test -f "$unpacked/source/tracked.txt"
cmp -s "$unpacked/source/tracked.txt" <(printf 'tracked commit content\n')
test -f "$unpacked/source/.yunbay-source-manifest"
test ! -e "$unpacked/source/.git"
test ! -e "$unpacked/source/untracked.txt"
test ! -e "$unpacked/source/._junk"
test ! -e "$unpacked/source/.DS_Store"

actual_entries=$(cd "$unpacked/source" && find . -mindepth 1 -print | LC_ALL=C sort)
expected_entries=$(printf '%s\n' './.yunbay-source-manifest' './tracked.txt' | LC_ALL=C sort)
if [[ "$actual_entries" != "$expected_entries" ]]; then
  LC_ALL=C comm -3 \
    <(printf '%s\n' "$expected_entries") \
    <(printf '%s\n' "$actual_entries") \
    >&2
  exit 1
fi

manifest="$unpacked/source/.yunbay-source-manifest"
manifest_line_count=$(wc -l <"$manifest" | tr -d '[:space:]')
test "$manifest_line_count" = '2'
test "$(sed -n '1p' "$manifest")" = "git_sha=$fixture_sha"
[[ "$(sed -n '2p' "$manifest")" =~ ^built_at_utc=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
grep -Fqx "git_sha=$fixture_sha" "$manifest"

checksum="$archive.sha256"
test -f "$checksum"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c "$checksum"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -c "$checksum"
else
  printf 'error: neither sha256sum nor shasum is available\n' >&2
  exit 1
fi

git -C "$fixture_repo" add .DS_Store
git -C "$fixture_repo" commit -qm 'track Finder metadata'
finder_metadata_sha=$(git -C "$fixture_repo" rev-parse HEAD)
set +e
finder_error=$("$builder" "$fixture_repo" "$finder_metadata_sha" "$archive" 2>&1)
finder_status=$?
set -e
test "$finder_status" -ne 0
test "$finder_error" = 'error: tracked source contains forbidden macOS metadata'
if [[ -e "$archive" || -e "$checksum" ]]; then
  printf 'stale release artifacts remain after tracked .DS_Store rejection\n' >&2
  exit 1
fi

appledouble_repo="$tmp/appledouble repo"
appledouble_archive="$tmp/appledouble output/release artifact.tar.gz"
git init -q "$appledouble_repo"
git -C "$appledouble_repo" config user.name 'Release Builder Test'
git -C "$appledouble_repo" config user.email 'release-builder-test@example.invalid'
printf 'clean tracked content\n' >"$appledouble_repo/tracked.txt"
git -C "$appledouble_repo" add tracked.txt
git -C "$appledouble_repo" commit -qm 'add clean tracked fixture'
"$builder" "$appledouble_repo" HEAD "$appledouble_archive"
test -f "$appledouble_archive"
test -f "$appledouble_archive.sha256"

printf 'tracked AppleDouble fixture\n' >"$appledouble_repo/._junk"
git -C "$appledouble_repo" add ._junk
git -C "$appledouble_repo" commit -qm 'track AppleDouble metadata'
appledouble_sha=$(git -C "$appledouble_repo" rev-parse HEAD)
set +e
appledouble_error=$("$builder" "$appledouble_repo" "$appledouble_sha" "$appledouble_archive" 2>&1)
appledouble_status=$?
set -e
test "$appledouble_status" -ne 0
test "$appledouble_error" = 'error: tracked source contains forbidden macOS metadata'
if [[ -e "$appledouble_archive" || -e "$appledouble_archive.sha256" ]]; then
  printf 'stale release artifacts remain after tracked ._junk rejection\n' >&2
  exit 1
fi

printf 'build-source-release test passed\n'
