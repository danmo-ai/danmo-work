#!/usr/bin/env bash
# Sync a single GitHub Release (metadata + assets) to Gitee, then publish the
# Gitee-rewritten Tauri updater manifest (see publish_gitee_updater_manifest.sh).
#
# Required env:
#   GITEE_TOKEN
#   GH_TOKEN or GITHUB_TOKEN
# Optional env:
#   TAG (default: latest GitHub release)
#   GITEE_OWNER / GITEE_REPO / GITHUB_REPOSITORY
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GITEE_OWNER="${GITEE_OWNER:-danmo-ai}"
GITEE_REPO="${GITEE_REPO:-danmo-work}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-danmo-ai/danmo-work}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
TAG="${TAG:-}"

if [[ -z "${GITEE_TOKEN:-}" ]]; then
  echo "ERROR: GITEE_TOKEN unset" >&2
  exit 1
fi
if [[ -z "$GH_TOKEN" ]]; then
  echo "ERROR: GH_TOKEN or GITHUB_TOKEN unset" >&2
  exit 1
fi
if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh CLI required" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
API="https://gitee.com/api/v5/repos/${GITEE_OWNER}/${GITEE_REPO}"

if [[ -z "$TAG" ]]; then
  TAG="$(gh release view --repo "$GITHUB_REPOSITORY" --json tagName -q .tagName)"
fi

echo "==> Syncing GitHub release ${TAG} → gitee.com/${GITEE_OWNER}/${GITEE_REPO}"

gh release view "$TAG" --repo "$GITHUB_REPOSITORY" --json name,body,tagName \
  >"$TMP/gh-release.json"
python3 - "$TMP/gh-release.json" "$TMP" <<'PY'
import json, sys
from pathlib import Path
rel = json.load(open(sys.argv[1], encoding="utf-8"))
out = Path(sys.argv[2])
(out / "name").write_text(rel.get("name") or rel["tagName"], encoding="utf-8")
(out / "body").write_text(rel.get("body") or "", encoding="utf-8")
PY
NAME="$(cat "$TMP/name")"

DEFAULT_BRANCH="$(curl -fsSL "${API}?access_token=${GITEE_TOKEN}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("default_branch") or "master")')"

EXISTING="$(curl -sS "${API}/releases/tags/${TAG}?access_token=${GITEE_TOKEN}" || true)"
RELEASE_ID="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read() or "{}")
  print(d.get("id") or "")
except Exception:
  print("")
' <<<"$EXISTING")"

if [[ -z "$RELEASE_ID" ]]; then
  echo "Creating Gitee release ${TAG}"
  python3 - "$TAG" "$NAME" "$TMP/body" "$DEFAULT_BRANCH" <<'PY' >"$TMP/create-payload.json"
import json, os, sys
tag, name, body_path, branch = sys.argv[1:5]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "tag_name": tag,
  "name": name,
  "body": open(body_path, encoding="utf-8").read(),
  "target_commitish": branch,
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  RESP="$(curl -fsSL -X POST "${API}/releases" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/create-payload.json")"
  RELEASE_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$RESP")"
else
  echo "Updating Gitee release id=${RELEASE_ID}"
  python3 - "$TAG" "$NAME" "$TMP/body" <<'PY' >"$TMP/patch-payload.json"
import json, os, sys
tag, name, body_path = sys.argv[1:4]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "tag_name": tag,
  "name": name,
  "body": open(body_path, encoding="utf-8").read(),
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  curl -fsSL -X PATCH "${API}/releases/${RELEASE_ID}" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/patch-payload.json" >/dev/null
fi

echo "Downloading GitHub assets for ${TAG}"
mkdir -p "$TMP/assets"
gh release download "$TAG" --repo "$GITHUB_REPOSITORY" -D "$TMP/assets"

curl -fsSL "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
  -o "$TMP/gitee-attach.json" || echo '[]' >"$TMP/gitee-attach.json"

shopt -s nullglob
for f in "$TMP/assets"/*; do
  [[ -f "$f" ]] || continue
  name="$(basename "$f")"
  ATT_ID="$(python3 - "$TMP/gitee-attach.json" "$name" <<'PY'
import json, sys
name = sys.argv[2]
data = json.load(open(sys.argv[1], encoding="utf-8"))
items = data if isinstance(data, list) else []
for a in items:
    if a.get("name") == name:
        print(a.get("id") or "")
        break
PY
)"
  if [[ -n "$ATT_ID" ]]; then
    echo "Removing existing Gitee asset ${name} (id=${ATT_ID})"
    curl -fsSL -X DELETE \
      "${API}/releases/${RELEASE_ID}/attach_files/${ATT_ID}?access_token=${GITEE_TOKEN}" \
      >/dev/null || true
  fi
  echo "Uploading ${name}"
  curl -fsSL -X POST "${API}/releases/${RELEASE_ID}/attach_files" \
    -F "access_token=${GITEE_TOKEN}" \
    -F "file=@${f};filename=${name}" \
    >/dev/null
done

export TAG
"${SCRIPT_DIR}/publish_gitee_updater_manifest.sh"

echo "Done. Gitee release: https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/${TAG}"
