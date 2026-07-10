#!/usr/bin/env bash
set -euo pipefail

base=${1:?usage: verify-caddy-init.sh BASE_COMPOSE [ENV_FILE]}
env_file=${2:-}
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
override=$root/deploy/docker-compose.caddy-init.yml

args=(-f "$base" -f "$override")
if [[ -n "$env_file" ]]; then
  args=(--env-file "$env_file" "${args[@]}")
fi

docker compose "${args[@]}" config --format json | python3 -c '
import json
import sys

d = json.load(sys.stdin)
caddy = d.get("services", {}).get("caddy", {})
if caddy.get("init") is not True:
    raise SystemExit("merged config does not set services.caddy.init to true")
'

echo "caddy init override verified"
