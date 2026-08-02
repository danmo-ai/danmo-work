#!/usr/bin/env bash
# Publish Tauri updater latest.json as a Gitee Release attachment on a rolling
# tag "updater" (stable download URL). Do NOT use a git branch — Gitee Pull
# mirror from GitHub deletes branches that do not exist upstream.
#
# Stable URL (tauri.conf.json):
#   https://gitee.com/<owner>/<repo>/releases/download/updater/latest.json
#
# Platform download URLs stay on GitHub Releases unless MIRROR_BASE_URL is set.
#
# Required env: GITEE_TOKEN, GH_TOKEN|GITHUB_TOKEN
# Optional: TAG, GITEE_OWNER, GITEE_REPO, GITHUB_REPOSITORY, MIRROR_BASE_URL
set -euo pipefail

GITEE_OWNER="${GITEE_OWNER:-danmo-ai}"
GITEE_REPO="${GITEE_REPO:-danmo-work}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-danmo-ai/danmo-work}"
GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
TAG="${TAG:-}"
MIRROR_BASE_URL="${MIRROR_BASE_URL:-}"
UPDATER_TAG="${UPDATER_TAG:-updater}"

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
API="https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}"

if [[ -z "$TAG" ]]; then
  TAG="$(curl -fsSL --connect-timeout 20 --max-time 60 \
    -H "Authorization: Bearer ${GH_TOKEN}" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/latest" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')"
fi

echo "Publishing updater latest.json for app ${TAG} → Gitee release tag ${UPDATER_TAG}"

curl -fsSL --connect-timeout 20 --max-time 60 \
  -H "Authorization: Bearer ${GH_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -o "$TMP/latest.github.json" \
  "https://github.com/${GITHUB_REPOSITORY}/releases/download/${TAG}/latest.json"

python3 - "$TMP/latest.github.json" "$TMP/latest.json" "$MIRROR_BASE_URL" <<'PY'
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
print("wrote", dst)
PY

DEFAULT_BRANCH="$(curl -fsSL --connect-timeout 20 --max-time 60 \
  "${API}?access_token=${GITEE_TOKEN}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("default_branch") or "master")')"

EXISTING="$(curl -sS --connect-timeout 20 --max-time 60 \
  "${API}/releases/tags/${UPDATER_TAG}?access_token=${GITEE_TOKEN}" || true)"
RELEASE_ID="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read() or "{}")
  print(d.get("id") or "")
except Exception:
  print("")
' <<<"$EXISTING")"

BODY="Tauri updater manifest (auto-updated).
App version reflected in this file: ${TAG}
Do not delete this rolling release — desktop clients fetch:
https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/download/${UPDATER_TAG}/latest.json
"

if [[ -z "$RELEASE_ID" ]]; then
  echo "Creating rolling Gitee release tag=${UPDATER_TAG}"
  python3 - "$UPDATER_TAG" "$BODY" "$DEFAULT_BRANCH" <<'PY' >"$TMP/payload.json"
import json, os, sys
tag, body, branch = sys.argv[1:4]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "tag_name": tag,
  "name": "Updater manifest",
  "body": body,
  "target_commitish": branch,
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  RESP="$(curl -fsSL --connect-timeout 20 --max-time 60 -X POST "${API}/releases" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/payload.json")"
  RELEASE_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$RESP")"
else
  echo "Updating rolling Gitee release id=${RELEASE_ID}"
  python3 - "$UPDATER_TAG" "$BODY" <<'PY' >"$TMP/payload.json"
import json, os, sys
tag, body = sys.argv[1:3]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "tag_name": tag,
  "name": "Updater manifest",
  "body": body,
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  curl -fsSL --connect-timeout 20 --max-time 60 -X PATCH "${API}/releases/${RELEASE_ID}" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/payload.json" >/dev/null
fi

curl -fsSL --connect-timeout 20 --max-time 60 \
  "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
  -o "$TMP/attach.json" || echo '[]' >"$TMP/attach.json"

EXISTING_ID="$(python3 - "$TMP/attach.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
items = data if isinstance(data, list) else []
for a in items:
    if a.get("name") == "latest.json":
        print(a.get("id") or "")
        break
PY
)"

if [[ -n "${EXISTING_ID}" ]]; then
  echo "Removing previous latest.json attach id=${EXISTING_ID}"
  curl -fsSL --connect-timeout 20 --max-time 60 -X DELETE \
    "${API}/releases/${RELEASE_ID}/attach_files/${EXISTING_ID}?access_token=${GITEE_TOKEN}" \
    >/dev/null || true
fi

echo "Uploading latest.json ($(wc -c <"$TMP/latest.json") bytes)"
curl -fS --connect-timeout 20 --max-time 120 \
  --speed-time 20 --speed-limit 100 \
  -X POST \
  "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}" \
  -F "access_token=${GITEE_TOKEN}" \
  -F "file=@${TMP}/latest.json;filename=latest.json" \
  -o "$TMP/upload.json" \
  -w "http=%{http_code} time=%{time_total}\n"

STABLE="https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/download/${UPDATER_TAG}/latest.json"
echo "Stable updater endpoint:"
echo "  ${STABLE}"

# Verify publicly fetchable
code="$(curl -sS -o /tmp/verify.json -w "%{http_code}" -L --connect-timeout 20 --max-time 30 "$STABLE" || true)"
echo "verify HTTP ${code}"
if [[ "$code" != "200" ]]; then
  echo "ERROR: stable URL not publicly fetchable yet (HTTP ${code})" >&2
  head -c 300 /tmp/verify.json 2>/dev/null >&2 || true
  echo >&2
  exit 1
fi
python3 -c 'import json; json.load(open("/tmp/verify.json")); print("verify JSON ok")'
