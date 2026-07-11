#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd -- "${script_dir}/../../.." && pwd -P)"
verifier="${script_dir}/verify-overlay.sh"
dockerfile="${repo_root}/infra/sub2api/Dockerfile"

verifier_pnpm_version="$(sed -n 's/^readonly PNPM_VERSION="\([^"]*\)"$/\1/p' "${verifier}")"
dockerfile_pnpm_version="$(sed -n 's/^RUN corepack enable && corepack prepare pnpm@\([^ ]*\) --activate$/\1/p' "${dockerfile}")"

if [[ "${verifier_pnpm_version}" != "10.28.2" ]]; then
  echo "unexpected verifier pnpm version: ${verifier_pnpm_version}" >&2
  exit 1
fi

if [[ "${dockerfile_pnpm_version}" != "${verifier_pnpm_version}" ]]; then
  echo "Dockerfile pnpm version ${dockerfile_pnpm_version} does not match verifier ${verifier_pnpm_version}" >&2
  exit 1
fi

dependency_copy_line="$(sed -n '/^COPY frontend\/package.json frontend\/pnpm-lock.yaml frontend\/pnpm-workspace.yaml frontend\/\.npmrc \.\/$/=' "${dockerfile}")"
dependency_install_line="$(sed -n '/^RUN pnpm install --frozen-lockfile$/=' "${dockerfile}")"
if [[ ! "${dependency_copy_line}" =~ ^[0-9]+$ || ! "${dependency_install_line}" =~ ^[0-9]+$ || "${dependency_copy_line}" -ge "${dependency_install_line}" ]]; then
  echo "Dockerfile must copy package, lock, workspace, and npm configs before frozen install" >&2
  exit 1
fi

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
