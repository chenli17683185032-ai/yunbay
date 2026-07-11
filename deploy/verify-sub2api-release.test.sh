#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
verifier=$root/deploy/verify-sub2api-release.sh
expected_image="weishaw/sub2api@sha256:2ca591c2af97eb0e2797cfc7fb7bd587194d94cebdac76f73d677eeab1d4d6c8"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT

cat >"$tmp/base.yml" <<'YAML'
services:
  new-api:
    image: yunbay-new-api:test
  sub2api:
    image: yunbay-sub2api:stale
    build:
      context: ./stale-sub2api
      dockerfile: Dockerfile
YAML

"$verifier" "$tmp/base.yml"

merged=$(docker compose \
  -f "$tmp/base.yml" \
  -f "$root/deploy/docker-compose.sub2api-release.yml" \
  config --format json)
python3 - "$expected_image" "$merged" <<'PY'
import json
import sys

expected_image, raw = sys.argv[1:]
sub2api = json.loads(raw)["services"]["sub2api"]
assert sub2api["image"] == expected_image
assert "build" not in sub2api
PY

cat >"$tmp/wrong-image.yml" <<'YAML'
services:
  sub2api:
    image: ghcr.io/wei-shaw/sub2api:latest
    build: !reset null
YAML
if "$verifier" "$tmp/base.yml" "" "$tmp/wrong-image.yml" >/dev/null 2>&1; then
  echo "verifier accepted a mutable sub2api image tag" >&2
  exit 1
fi

cat >"$tmp/build-not-reset.yml" <<YAML
services:
  sub2api:
    image: $expected_image
YAML
if "$verifier" "$tmp/base.yml" "" "$tmp/build-not-reset.yml" >/dev/null 2>&1; then
  echo "verifier accepted a merged sub2api build" >&2
  exit 1
fi

cat >"$tmp/no-sub2api.yml" <<'YAML'
services:
  new-api:
    image: yunbay-new-api:test
YAML
if "$verifier" "$tmp/no-sub2api.yml" >/dev/null 2>&1; then
  echo "verifier accepted a base compose without sub2api" >&2
  exit 1
fi

echo "sub2api release override tests passed"
