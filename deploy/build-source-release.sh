#!/usr/bin/env bash
set -euo pipefail

repo=${1:?usage: build-source-release.sh REPO REV OUTPUT}
rev=${2:?usage: build-source-release.sh REPO REV OUTPUT}
output=${3:?usage: build-source-release.sh REPO REV OUTPUT}

output_dir=$(dirname -- "$output")
tmp=''
archive_tmp=''
checksum_tmp=''
published=0
cleanup() {
  status=$?
  trap - EXIT
  set +e
  [[ -z "$tmp" ]] || rm -rf -- "$tmp"
  [[ -z "$archive_tmp" ]] || rm -f -- "$archive_tmp"
  [[ -z "$checksum_tmp" ]] || rm -f -- "$checksum_tmp"
  if [[ "$status" -ne 0 || "$published" -ne 1 ]]; then
    rm -f -- "$output" "$output.sha256"
  fi
  exit "$status"
}
trap cleanup EXIT

mkdir -p "$output_dir"
rm -f -- "$output" "$output.sha256"

if command -v sha256sum >/dev/null 2>&1; then
  checksum_tool='sha256sum'
elif command -v shasum >/dev/null 2>&1; then
  checksum_tool='shasum'
else
  printf 'error: neither sha256sum nor shasum is available\n' >&2
  exit 1
fi

sha=$(git -C "$repo" rev-parse --verify "$rev^{commit}")
forbidden_metadata=0
while IFS= read -r -d '' tracked_path; do
  remaining_path=$tracked_path
  while true; do
    path_component=${remaining_path%%/*}
    if [[ "$path_component" = '.DS_Store' || "$path_component" = ._* ]]; then
      forbidden_metadata=1
      break 2
    fi
    [[ "$remaining_path" = */* ]] || break
    remaining_path=${remaining_path#*/}
  done
done < <(git -C "$repo" ls-tree -r -z --name-only "$sha")
if [[ "$forbidden_metadata" -eq 1 ]]; then
  printf 'error: tracked source contains forbidden macOS metadata\n' >&2
  exit 1
fi

tmp=$(mktemp -d)
archive_tmp=$(mktemp "$output_dir/.yunbay-source-archive.XXXXXX")
checksum_tmp=$(mktemp "$output_dir/.yunbay-source-checksum.XXXXXX")

mkdir -p "$tmp/source"
git -C "$repo" archive "$sha" | tar -xf - -C "$tmp/source"

if find "$tmp/source" \( -name '._*' -o -name '.DS_Store' \) -print -quit | grep -q .; then
  printf 'error: tracked source contains forbidden macOS metadata\n' >&2
  exit 1
fi

printf 'git_sha=%s\nbuilt_at_utc=%s\n' \
  "$sha" \
  "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  >"$tmp/source/.yunbay-source-manifest"

tar -C "$tmp" -czf "$archive_tmp" source

if [[ "$checksum_tool" = 'sha256sum' ]]; then
  digest=$(sha256sum "$archive_tmp" | awk '{print $1}')
else
  digest=$(shasum -a 256 "$archive_tmp" | awk '{print $1}')
fi
printf '%s  %s\n' "$digest" "$output" >"$checksum_tmp"

mv -f -- "$archive_tmp" "$output"
archive_tmp=''
mv -f -- "$checksum_tmp" "$output.sha256"
checksum_tmp=''
published=1
