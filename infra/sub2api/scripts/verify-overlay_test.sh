#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
verifier="${script_dir}/verify-overlay.sh"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
touch "$tmp/sentinel"

set +e
output=$("$verifier" "$tmp" 2>&1)
status=$?
set -e

if [[ "$status" -eq 0 ]]; then
  echo "retired overlay verifier unexpectedly succeeded" >&2
  exit 1
fi
if [[ "$output" != *"vendored sub2api source overlay is retired"* ]]; then
  echo "retired overlay verifier did not explain the failure" >&2
  exit 1
fi
if [[ ! -f "$tmp/sentinel" ]]; then
  echo "retired overlay verifier modified its target" >&2
  exit 1
fi
if grep -Eq 'rsync|rm[[:space:]]+-rf|git[[:space:]]+clone' "$verifier"; then
  echo "retired overlay verifier still contains destructive or network operations" >&2
  exit 1
fi

echo "retired overlay verifier tests passed"
