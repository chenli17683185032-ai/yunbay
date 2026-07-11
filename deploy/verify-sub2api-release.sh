#!/usr/bin/env bash
set -euo pipefail

readonly expected_image="weishaw/sub2api@sha256:2ca591c2af97eb0e2797cfc7fb7bd587194d94cebdac76f73d677eeab1d4d6c8"

base=${1:?usage: verify-sub2api-release.sh BASE_COMPOSE [ENV_FILE] [OVERRIDE_COMPOSE]}
env_file=${2:-}
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
override=${3:-$root/deploy/docker-compose.sub2api-release.yml}

args=(-f "$base")
if [[ -n "$env_file" ]]; then
  args=(--env-file "$env_file" "${args[@]}")
fi

if ! docker compose "${args[@]}" config --services | grep -Fqx sub2api; then
  echo "base compose does not define services.sub2api" >&2
  exit 1
fi

docker compose "${args[@]}" -f "$override" config --format json |
  python3 -c '
import json
import sys

expected_image = sys.argv[1]
merged = json.load(sys.stdin)
sub2api = merged.get("services", {}).get("sub2api")
if not isinstance(sub2api, dict):
    raise SystemExit("merged compose does not define services.sub2api")
actual_image = sub2api.get("image")
if actual_image != expected_image:
    raise SystemExit(
        f"merged sub2api image is {actual_image!r}, expected {expected_image!r}"
    )
if "build" in sub2api:
    raise SystemExit("merged services.sub2api must not contain build")
' "$expected_image"

echo "sub2api release override verified"
