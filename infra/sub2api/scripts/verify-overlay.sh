#!/usr/bin/env bash
set -euo pipefail

cat >&2 <<'EOF'
error: the vendored sub2api source overlay is retired because it can overwrite newer upstream files.
Use deploy/docker-compose.sub2api-release.yml and deploy/verify-sub2api-release.sh to deploy the pinned official image digest.
EOF
exit 1
