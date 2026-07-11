#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd -- "${script_dir}/../../.." && pwd -P)"
verifier="${script_dir}/verify-overlay.sh"

test_root="$(mktemp -d "${repo_root}/.verify-overlay-safety.XXXXXX")"
trap 'rm -rf -- "${test_root}"' EXIT

mkdir -p "${test_root}/bin" "${test_root}/protected"
touch "${test_root}/protected/sentinel"

cat >"${test_root}/bin/git" <<'EOF'
#!/usr/bin/env bash
exit 99
EOF
chmod +x "${test_root}/bin/git"

if PATH="${test_root}/bin:${PATH}" "${verifier}" "${test_root}/protected" >/dev/null 2>&1; then
  echo "verifier unexpectedly accepted a target inside the repository" >&2
  exit 1
fi

if [[ ! -f "${test_root}/protected/sentinel" ]]; then
  echo "verifier deleted a target outside the disposable temp boundary" >&2
  exit 1
fi

echo "verify-overlay target safety test passed"
