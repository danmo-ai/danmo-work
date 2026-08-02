#!/usr/bin/env bash
# Publish Tauri updater latest.json to Gitee branch "updater" via Contents API
# (small JSON only — no Release binary attachments).
#
# Stable URL used by desktop/src-tauri/tauri.conf.json:
#   https://gitee.com/<owner>/<repo>/raw/updater/latest.json
#
# Platform download URLs stay on GitHub Releases (real artifact host).
# danmo.work is the marketing site (GitHub Pages), not a release CDN.
# Optional UPDATE_MIRROR_BASE_URL may rewrite urls when a real object mirror exists.
#
# Required env: GITEE_TOKEN, GH_TOKEN|GITHUB_TOKEN
# Optional: TAG, GITEE_OWNER, GITEE_REPO, GITHUB_REPOSITORY, MIRROR_BASE_URL
set -euo pipefail

GITEE_OWNER="${GITEE_OWNER:-danmo-ai}"
GITEE_REPO="${GITEE_REPO:-danmo-work}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-danmo-ai/danmo-work}"
GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
TAG="${TAG:-}"
# Empty by default — do not invent a dead host like releases.danmo.ai.
MIRROR_BASE_URL="${MIRROR_BASE_URL:-}"

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
API_BASE="https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}"

if [[ -z "$TAG" ]]; then
  TAG="$(curl -fsSL --connect-timeout 20 --max-time 60 \
    -H "Authorization: Bearer ${GH_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/latest" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')"
fi

echo "Publishing updater latest.json for ${TAG} → gitee raw/updater/"

curl -fsSL --connect-timeout 20 --max-time 60 \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -o "$TMP/latest.github.json" \
  "https://github.com/${GITHUB_REPOSITORY}/releases/download/${TAG}/latest.json"

python3 - "$TMP/latest.github.json" "$TMP/latest.gitee.json" "$MIRROR_BASE_URL" <<'PY'
import json, sys
from urllib.parse import unquote, urlparse

src, dst, mirror = sys.argv[1:4]
mirror = (mirror or "").rstrip("/")
data = json.load(open(src, encoding="utf-8"))
if mirror:
    for plat in data.get("platforms", {}).values():
        url = plat.get("url") or ""
        name = unquote(urlparse(url).path.rsplit("/", 1)[-1])
        if name:
            plat["url"] = f"{mirror}/{name}"
    print("Rewrote platform URLs →", mirror)
else:
    print("Keeping GitHub Release platform URLs")
json.dump(data, open(dst, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
PY

CONTENT_B64="$(base64 <"$TMP/latest.gitee.json" | tr -d '\n')"

ensure_updater_branch() {
  if curl -fsSL --connect-timeout 20 --max-time 60 \
    "${API_BASE}/branches/updater?access_token=${GITEE_TOKEN}" >/dev/null 2>&1; then
    return 0
  fi
  DEFAULT_BRANCH="$(curl -fsSL --connect-timeout 20 --max-time 60 \
    "${API_BASE}?access_token=${GITEE_TOKEN}" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin).get("default_branch") or "master")')"
  echo "Creating branch updater from ${DEFAULT_BRANCH}"
  curl -fsSL --connect-timeout 20 --max-time 60 -X POST "${API_BASE}/branches" \
    -H "Content-Type: application/json" \
    -d "{\"access_token\":\"${GITEE_TOKEN}\",\"branch_name\":\"updater\",\"refs\":\"${DEFAULT_BRANCH}\"}" \
    >/dev/null
}

ensure_updater_branch

FILE_META="$(curl -sS --connect-timeout 20 --max-time 60 \
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
  python3 - "$CONTENT_B64" "$MSG" "$FILE_SHA" <<'PY' >"$TMP/contents.json"
import json, os, sys
content_b64, message, sha = sys.argv[1:4]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "content": content_b64,
  "message": message,
  "branch": "updater",
  "sha": sha,
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  curl -fsSL --connect-timeout 20 --max-time 60 -X PUT "${API_BASE}/contents/latest.json" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/contents.json" >/dev/null
else
  echo "Creating raw/updater/latest.json"
  python3 - "$CONTENT_B64" "$MSG" <<'PY' >"$TMP/contents.json"
import json, os, sys
content_b64, message = sys.argv[1:3]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "content": content_b64,
  "message": message,
  "branch": "updater",
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  curl -fsSL --connect-timeout 20 --max-time 60 -X POST "${API_BASE}/contents/latest.json" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/contents.json" >/dev/null
fi

echo "Stable updater endpoint:"
echo "  https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/raw/updater/latest.json"
