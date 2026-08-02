#!/usr/bin/env bash
# After GitHub→Gitee release sync: rewrite latest.json platform URLs to Gitee,
# re-upload that manifest onto the Gitee release, and publish a stable copy at
#   https://gitee.com/<owner>/<repo>/raw/updater/latest.json
# for Tauri updater endpoints (Gitee has no /releases/latest/download/).
#
# Required env:
#   GITEE_TOKEN
# Optional env:
#   GITEE_OWNER (default danmo-ai)
#   GITEE_REPO  (default danmo-work)
#   GITHUB_REPOSITORY (default danmo-ai/danmo-work)
#   GH_TOKEN / GITHUB_TOKEN
#   TAG  (default: latest GitHub release tag)
set -euo pipefail

GITEE_OWNER="${GITEE_OWNER:-danmo-ai}"
GITEE_REPO="${GITEE_REPO:-danmo-work}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-danmo-ai/danmo-work}"
GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
TAG="${TAG:-}"

if [[ -z "${GITEE_TOKEN:-}" ]]; then
  echo "ERROR: GITEE_TOKEN unset" >&2
  exit 1
fi

if [[ -z "$GH_TOKEN" ]]; then
  echo "ERROR: GH_TOKEN or GITHUB_TOKEN unset" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [[ -z "$TAG" ]]; then
  TAG="$(curl -fsSL \
    -H "Authorization: Bearer ${GH_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/latest" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')"
fi

echo "Publishing Gitee updater manifest for ${TAG}"

curl -fsSL \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -o "$TMP/latest.github.json" \
  "https://github.com/${GITHUB_REPOSITORY}/releases/download/${TAG}/latest.json"

GH_PREFIX="https://github.com/${GITHUB_REPOSITORY}/releases/download/${TAG}"
GITEE_PREFIX="https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/download/${TAG}"

python3 - "$TMP/latest.github.json" "$TMP/latest.gitee.json" "$GH_PREFIX" "$GITEE_PREFIX" <<'PY'
import json, sys
from urllib.parse import unquote, urlparse
src, dst, gh_prefix, gitee_prefix = sys.argv[1:5]
data = json.load(open(src, encoding="utf-8"))

def hosted_on_gitee(name: str) -> bool:
    # Mirror only small desktop/updater payloads; keep GitHub URLs for AppImage etc.
    n = name.lower()
    if n.endswith(".appimage") or n.endswith(".appimage.sig"):
        return False
    if n.startswith("danmo-work-env-"):
        return False
    return (
        n.endswith(".dmg")
        or n.endswith(".deb")
        or n.endswith("-setup.exe")
        or n.endswith("-setup.exe.sig")
        or n.endswith(".app.tar.gz")
        or n.endswith(".app.tar.gz.sig")
        or n.endswith(".nsis.zip")
        or n.endswith(".nsis.zip.sig")
    )

for plat in data.get("platforms", {}).values():
    url = plat.get("url", "")
    name = unquote(urlparse(url).path.rsplit("/", 1)[-1])
    if not hosted_on_gitee(name):
        continue
    if url.startswith(gh_prefix):
        plat["url"] = gitee_prefix + url[len(gh_prefix):]
    elif "github.com" in url and "/releases/download/" in url:
        plat["url"] = url.replace("https://github.com/", "https://gitee.com/", 1)
json.dump(data, open(dst, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
print("Wrote", dst)
PY

# --- Attach rewritten latest.json to the Gitee release (replace if present) ---
RELEASE_JSON="$(curl -fsSL \
  "https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}/releases/tags/${TAG}?access_token=${GITEE_TOKEN}")"
RELEASE_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$RELEASE_JSON")"

curl -fsSL \
  "https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
  -o "$TMP/gitee-attach.json" || echo '[]' >"$TMP/gitee-attach.json"
printf '%s' "$RELEASE_JSON" >"$TMP/gitee-release.json"
EXISTING_ID="$(python3 - "$TMP/gitee-attach.json" "$TMP/gitee-release.json" <<'PY'
import json, sys
for path in sys.argv[1:]:
    try:
        data = json.load(open(path, encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        continue
    items = data if isinstance(data, list) else (data.get("assets") or [])
    for a in items:
        if a.get("name") == "latest.json":
            print(a.get("id") or "")
            raise SystemExit
PY
)"

if [[ -n "${EXISTING_ID}" ]]; then
  echo "Removing existing Gitee latest.json attach_file id=${EXISTING_ID}"
  curl -fsSL -X DELETE \
    "https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}/releases/${RELEASE_ID}/attach_files/${EXISTING_ID}?access_token=${GITEE_TOKEN}" \
    >/dev/null || true
fi

echo "Uploading Gitee-rewritten latest.json to release ${TAG} (id=${RELEASE_ID})"
curl -fsSL -X POST \
  "https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}/releases/${RELEASE_ID}/attach_files" \
  -F "access_token=${GITEE_TOKEN}" \
  -F "file=@${TMP}/latest.gitee.json;filename=latest.json" \
  >/dev/null

# --- Stable raw endpoint on branch "updater" for Tauri plugins.updater.endpoints ---
CONTENT_B64="$(base64 <"$TMP/latest.gitee.json" | tr -d '\n')"
API_BASE="https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}"

ensure_updater_branch() {
  if curl -fsSL "${API_BASE}/branches/updater?access_token=${GITEE_TOKEN}" >/dev/null 2>&1; then
    return 0
  fi
  DEFAULT_BRANCH="$(curl -fsSL "${API_BASE}?access_token=${GITEE_TOKEN}" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin).get("default_branch") or "master")')"
  BASE_SHA="$(curl -fsSL "${API_BASE}/branches/${DEFAULT_BRANCH}?access_token=${GITEE_TOKEN}" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["commit"]["sha"])')"
  echo "Creating branch updater from ${DEFAULT_BRANCH} (${BASE_SHA})"
  curl -fsSL -X POST "${API_BASE}/branches" \
    -H "Content-Type: application/json" \
    -d "{\"access_token\":\"${GITEE_TOKEN}\",\"branch_name\":\"updater\",\"refs\":\"${DEFAULT_BRANCH}\"}" \
    >/dev/null
}

ensure_updater_branch

FILE_META="$(curl -sS \
  "${API_BASE}/contents/latest.json?access_token=${GITEE_TOKEN}&ref=updater" || true)"
FILE_SHA="$(python3 - "$FILE_META" <<'PY'
import json, sys
raw = sys.stdin.read().strip()
if not raw:
    raise SystemExit
try:
    data = json.loads(raw)
except json.JSONDecodeError:
    raise SystemExit
if isinstance(data, dict) and data.get("sha"):
    print(data["sha"])
PY
)"

MSG="chore(updater): publish latest.json for ${TAG}"
if [[ -n "${FILE_SHA}" ]]; then
  echo "Updating raw/updater/latest.json (sha=${FILE_SHA})"
  curl -fsSL -X PUT "${API_BASE}/contents/latest.json" \
    -H "Content-Type: application/json" \
    -d "{\"access_token\":\"${GITEE_TOKEN}\",\"content\":\"${CONTENT_B64}\",\"message\":\"${MSG}\",\"branch\":\"updater\",\"sha\":\"${FILE_SHA}\"}" \
    >/dev/null
else
  echo "Creating raw/updater/latest.json"
  curl -fsSL -X POST "${API_BASE}/contents/latest.json" \
    -H "Content-Type: application/json" \
    -d "{\"access_token\":\"${GITEE_TOKEN}\",\"content\":\"${CONTENT_B64}\",\"message\":\"${MSG}\",\"branch\":\"updater\"}" \
    >/dev/null
fi

echo "Stable updater endpoint:"
echo "  https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/raw/updater/latest.json"
