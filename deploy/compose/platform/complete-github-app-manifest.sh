#!/usr/bin/env bash
# Exchange a GitHub App manifest conversion code for App ID + private key.
# Usage (from repo root, within one hour of registering the app):
#   ./deploy/compose/platform/complete-github-app-manifest.sh <code>
set -euo pipefail

code="${1:-}"
if [[ -z "$code" ]]; then
  echo "usage: $0 <manifest-conversion-code>" >&2
  echo "The code is the ?code= query param on the redirect after Create GitHub App." >&2
  exit 2
fi

# Resolve repo root from this script's location.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../.." && pwd)"
secrets_dir="${repo_root}/secrets"
pem_path="${secrets_dir}/github-app.pem"
meta_path="${secrets_dir}/github-app.json"

if ! command -v curl >/dev/null 2>&1; then
  echo "error: curl is required" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required" >&2
  exit 1
fi

mkdir -p "${secrets_dir}"
chmod 700 "${secrets_dir}"

tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT

http_code="$(
  curl -sS -o "${tmp}" -w '%{http_code}' -X POST \
    "https://api.github.com/app-manifests/${code}/conversions" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28"
)"

if [[ "${http_code}" != "201" && "${http_code}" != "200" ]]; then
  echo "error: GitHub returned HTTP ${http_code} exchanging manifest code" >&2
  cat "${tmp}" >&2
  echo >&2
  exit 1
fi

app_id="$(jq -r '.id // empty' "${tmp}")"
pem="$(jq -r '.pem // empty' "${tmp}")"
slug="$(jq -r '.slug // empty' "${tmp}")"
html_url="$(jq -r '.html_url // empty' "${tmp}")"

if [[ -z "${app_id}" || -z "${pem}" ]]; then
  echo "error: response missing id or pem" >&2
  cat "${tmp}" >&2
  echo >&2
  exit 1
fi

# Metadata without the private key (safe-ish to glance at; still gitignored).
jq '{
  id,
  slug,
  name,
  html_url,
  client_id,
  webhook_secret,
  created_at: (now | todateiso8601)
}' "${tmp}" > "${meta_path}"
chmod 600 "${meta_path}"

printf '%s\n' "${pem}" > "${pem_path}"
chmod 600 "${pem_path}"

cat <<EOF
GitHub App ready.

  App ID:      ${app_id}
  Slug:        ${slug}
  Settings:    ${html_url}
  Private key: ${pem_path}
  Metadata:    ${meta_path}

Next:
  1. Install the App on the account/org that owns the repos you will scan:
     ${html_url}/installations/new
  2. Copy .env.example → .env (if needed) and set:
       COACH_GITHUB_APP_ID=${app_id}
       COACH_GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
     Compose mounts host secrets/ at /secrets (see docs/pilot-local-quickstart.md Path C).
  3. Create a separate GitHub OAuth App for browser sign-in (same guide) and put
     client id/secret in .env.

Do not commit secrets/*.pem or .env (gitignored).
EOF
