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

# Gitee release attachments are typically capped (~50–100MB on community plans).
# Prefer desktop installers; skip huge env tars and anything over MAX_UPLOAD_MB.
MAX_UPLOAD_MB="${MAX_UPLOAD_MB:-100}"
SKIP_GLOBS="${SKIP_GLOBS:-danmo-work-env-*.tar}"

# Upload order: installers users click first, then updater payloads, then rest.
python3 - "$TMP/assets" "$TMP/upload-list.txt" <<'PY'
import os, sys
from pathlib import Path
root = Path(sys.argv[1])
out = Path(sys.argv[2])
files = [p for p in root.iterdir() if p.is_file()]

def rank(p: Path) -> tuple:
    n = p.name.lower()
    if n.endswith((".dmg", ".exe", ".deb")): return (0, n)
    if n.endswith(".appimage") or n.endswith(".appimage.sig"): return (1, n)
    if ".app.tar.gz" in n: return (2, n)
    if n.endswith(".sig"): return (3, n)
    if n == "latest.json": return (4, n)
    if n.startswith("danmo-work-env-"): return (9, n)
    return (5, n)

files.sort(key=rank)
out.write_text("\n".join(str(p) for p in files) + ("\n" if files else ""), encoding="utf-8")
print(f"queued {len(files)} assets")
PY

UPLOAD_OK=0
UPLOAD_SKIP=0
UPLOAD_FAIL=0
CRITICAL_FAIL=0

should_skip_name() {
  local name="$1"
  local g
  for g in $SKIP_GLOBS; do
    # shellcheck disable=SC2254
    case "$name" in
      $g) return 0 ;;
    esac
  done
  return 1
}

is_critical_name() {
  local name="$1"
  case "$name" in
    *.dmg|*.exe|*.deb|*.AppImage|*.app.tar.gz|*.app.tar.gz.sig|*.AppImage.sig|*-setup.exe.sig|latest.json)
      return 0 ;;
    *)
      return 1 ;;
  esac
}

while IFS= read -r f; do
  [[ -z "$f" || ! -f "$f" ]] && continue
  name="$(basename "$f")"
  size_mb="$(python3 -c 'import os,sys; print(os.path.getsize(sys.argv[1]) / (1024*1024))' "$f")"

  if should_skip_name "$name"; then
    echo "SKIP ${name} (matched SKIP_GLOBS)"
    UPLOAD_SKIP=$((UPLOAD_SKIP + 1))
    continue
  fi
  if python3 -c 'import sys; sys.exit(0 if float(sys.argv[1]) > float(sys.argv[2]) else 1)' "$size_mb" "$MAX_UPLOAD_MB"; then
    echo "SKIP ${name} (${size_mb%.*}MB > MAX_UPLOAD_MB=${MAX_UPLOAD_MB})"
    UPLOAD_SKIP=$((UPLOAD_SKIP + 1))
    continue
  fi

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
  # Skip re-upload when already present (except latest.json, rewritten later).
  if [[ -n "$ATT_ID" && "$name" != "latest.json" ]]; then
    echo "EXISTS ${name} (id=${ATT_ID}) — skip"
    UPLOAD_OK=$((UPLOAD_OK + 1))
    continue
  fi
  if [[ -n "$ATT_ID" && "$name" == "latest.json" ]]; then
    echo "Removing existing Gitee asset ${name} (id=${ATT_ID})"
    curl -fsSL --max-time 60 -X DELETE \
      "${API}/releases/${RELEASE_ID}/attach_files/${ATT_ID}?access_token=${GITEE_TOKEN}" \
      >/dev/null || true
  fi

  echo "Uploading ${name} (${size_mb}MB)"
  if curl -fS --retry 2 --retry-delay 3 --max-time 600 -X POST \
    "${API}/releases/${RELEASE_ID}/attach_files" \
    -F "access_token=${GITEE_TOKEN}" \
    -F "file=@${f};filename=${name}" \
    -o "$TMP/upload-resp.json"; then
    echo "OK ${name}"
    UPLOAD_OK=$((UPLOAD_OK + 1))
    # Refresh attach list so later EXISTS checks see new ids.
    curl -fsSL --max-time 60 \
      "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
      -o "$TMP/gitee-attach.json" || true
  else
    echo "FAIL ${name} (see curl exit; Gitee may reject oversized attachments)" >&2
    UPLOAD_FAIL=$((UPLOAD_FAIL + 1))
    if is_critical_name "$name"; then
      CRITICAL_FAIL=$((CRITICAL_FAIL + 1))
    fi
  fi
done <"$TMP/upload-list.txt"

echo "Upload summary: ok=${UPLOAD_OK} skip=${UPLOAD_SKIP} fail=${UPLOAD_FAIL} critical_fail=${CRITICAL_FAIL}"
if [[ "$CRITICAL_FAIL" -gt 0 ]]; then
  echo "ERROR: one or more desktop/updater assets failed to upload" >&2
  exit 1
fi

export TAG
"${SCRIPT_DIR}/publish_gitee_updater_manifest.sh"

echo "Done. Gitee release: https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/tag/${TAG}"
