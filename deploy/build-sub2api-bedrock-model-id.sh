#!/usr/bin/env bash
set -euo pipefail

readonly upstream_url=https://github.com/Wei-Shaw/sub2api.git
readonly upstream_tag=v0.1.166
readonly upstream_commit=dc893dd0b8eab41df5be595ae9fcd1aa74a062b8
readonly default_image=yunbay/sub2api:0.1.166-bedrock-model-id-20260729

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
patch_bundle=$root/deploy/sub2api-v0.1.166-bedrock-model-id.patch.b64
image=${1:-$default_image}
worktree=$(mktemp -d)
patch_file=$worktree/bedrock-model-id.patch

cleanup() {
  rm -rf -- "$worktree"
}
trap cleanup EXIT

git clone --quiet --depth 1 --branch "$upstream_tag" "$upstream_url" "$worktree/sub2api"
actual_commit=$(git -C "$worktree/sub2api" rev-parse HEAD)
if [[ "$actual_commit" != "$upstream_commit" ]]; then
  printf 'error: upstream %s resolved to %s, expected %s\n' \
    "$upstream_tag" "$actual_commit" "$upstream_commit" >&2
  exit 1
fi

if base64 --decode "$patch_bundle" > "$patch_file" 2>/dev/null; then
  :
elif base64 -D -i "$patch_bundle" > "$patch_file"; then
  :
else
  printf 'error: failed to decode %s\n' "$patch_bundle" >&2
  exit 1
fi

git -C "$worktree/sub2api" apply --check "$patch_file"
git -C "$worktree/sub2api" apply "$patch_file"
git -C "$worktree/sub2api" diff --check

docker buildx build \
  --platform linux/amd64 \
  --load \
  --tag "$image" \
  --build-arg VERSION=0.1.166-bedrock-model-id.1 \
  --build-arg COMMIT=dc893dd0-bedrock-model-id \
  "$worktree/sub2api"

docker image inspect --format 'image={{.RepoTags}} id={{.Id}} size={{.Size}}' "$image"
