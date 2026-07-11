#!/usr/bin/env bash
set -euo pipefail

readonly UPSTREAM_URL="https://github.com/Wei-Shaw/sub2api.git"
readonly UPSTREAM_PIN="ddb1a210ce6742ebd4bd5339b9a0c8f309bcbbf0"
readonly PNPM_VERSION="10.28.2"

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd -- "${script_dir}/../../.." && pwd -P)"
overlay_root="${repo_root}/infra/sub2api"
target="${1:-/tmp/yunbay-sub2api-verify}"

if [[ $# -gt 1 || -z "${target}" || "${target}" == "/" ]]; then
  echo "usage: $0 [target-directory]" >&2
  exit 2
fi

rm -rf -- "${target}"
git clone --filter=blob:none --no-checkout "${UPSTREAM_URL}" "${target}"
git -C "${target}" checkout --detach "${UPSTREAM_PIN}"

upstream_head="$(git -C "${target}" rev-parse HEAD)"
if [[ "${upstream_head}" != "${UPSTREAM_PIN}" ]]; then
  echo "unexpected upstream HEAD: ${upstream_head}" >&2
  exit 1
fi

rsync -a --delete \
  --exclude='/go.mod' \
  --exclude='/go.sum' \
  "${overlay_root}/backend/" "${target}/backend/"

# These excludes hide generated inputs and protect matching target paths from --delete.
rsync -a --delete \
  --exclude='/package.json' \
  --exclude='/pnpm-lock.yaml' \
  --exclude='/bun.lock' \
  --exclude='/bun.lockb' \
  --exclude='/node_modules/' \
  --exclude='/dist/' \
  --exclude='/build/' \
  --exclude='/coverage/' \
  --exclude='/.cache/' \
  --exclude='/.vite/' \
  --exclude='/.rsbuild/' \
  --exclude='/.turbo/' \
  --exclude='/playwright-report/' \
  --exclude='/test-results/' \
  --exclude='*.tsbuildinfo' \
  "${overlay_root}/frontend/" "${target}/frontend/"

(
  cd -- "${target}/backend"
  GOWORK=off go test -tags=unit ./...
)

(
  cd -- "${target}/frontend"
  test -f package.json || { echo "missing pinned frontend package.json" >&2; exit 1; }
  test -f pnpm-lock.yaml || { echo "missing pinned frontend pnpm-lock.yaml" >&2; exit 1; }
  test -f pnpm-workspace.yaml || { echo "missing overlay frontend pnpm-workspace.yaml" >&2; exit 1; }
  command -v bunx >/dev/null || { echo "bunx is required" >&2; exit 1; }
  actual_pnpm_version="$(bunx "pnpm@${PNPM_VERSION}" --version)"
  if [[ "${actual_pnpm_version}" != "${PNPM_VERSION}" ]]; then
    echo "unexpected pnpm version: ${actual_pnpm_version}" >&2
    exit 1
  fi
  bunx "pnpm@${PNPM_VERSION}" install --frozen-lockfile
  bun run test:run
  bun run typecheck
  bun run build
)

overlay_head="$(git -C "${repo_root}" rev-parse HEAD)"
printf 'Pinned upstream: %s\nOverlay HEAD: %s\n' "${upstream_head}" "${overlay_head}"
