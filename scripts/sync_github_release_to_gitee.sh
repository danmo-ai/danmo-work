#!/usr/bin/env bash
# Sync small desktop installers + Tauri updater payloads from one GitHub Release
# to Gitee, then publish a Gitee-rewritten latest.json.
#
# Included (~15–20MB each):
#   *.dmg  *.exe  *.deb  *.app.tar.gz  matching *.sig  latest.json
# Excluded (too large for Gitee attachments):
#   *.AppImage  danmo-work-env-*.tar
#
# Required env: GITEE_TOKEN, GH_TOKEN|GITHUB_TOKEN
# Optional: TAG, GITEE_OWNER, GITEE_REPO, GITHUB_REPOSITORY
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

echo "==> Syncing desktop/updater assets for ${TAG} → gitee.com/${GITEE_OWNER}/${GITEE_REPO}"

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

echo "Downloading small desktop + updater assets (by name allowlist)"
mkdir -p "$TMP/assets"
gh api "repos/${GITHUB_REPOSITORY}/releases/tags/${TAG}" >"$TMP/gh-assets.json"
python3 - "$TMP/gh-assets.json" "$TMP/assets" "$GH_TOKEN" <<'PY'
import json, os, sys, urllib.request
from pathlib import Path

rel = json.load(open(sys.argv[1], encoding="utf-8"))
dest = Path(sys.argv[2])
token = sys.argv[3]

def allowed(name: str) -> bool:
    n = name
    return (
        n.endswith(".dmg")
        or n.endswith(".deb")
        or n.endswith("-setup.exe")
        or n.endswith("-setup.exe.sig")
        or n.endswith(".app.tar.gz")
        or n.endswith(".app.tar.gz.sig")
        or n == "latest.json"
    )

kept = 0
for a in rel.get("assets") or []:
    name = a.get("name") or ""
    # Public browser URL is more reliable than the API asset endpoint here.
    url = a.get("browser_download_url") or a.get("url")
    if not name or not url or not allowed(name):
        if name:
            print(f"skip {name}", flush=True)
        continue
    out = dest / name
    print(f"download {name}", flush=True)
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/octet-stream",
            "User-Agent": "danmo-work-gitee-sync",
        },
    )
    with urllib.request.urlopen(req, timeout=120) as resp, open(out, "wb") as fh:
        while True:
            chunk = resp.read(1024 * 1024)
            if not chunk:
                break
            fh.write(chunk)
    print(f"saved {name} ({out.stat().st_size} bytes)", flush=True)
    kept += 1
print(f"kept {kept} assets", flush=True)
if kept == 0:
    raise SystemExit("no desktop/updater assets to sync")
PY

curl -fsSL "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
  -o "$TMP/gitee-attach.json" || echo '[]' >"$TMP/gitee-attach.json"

python3 - "$TMP/assets" "$TMP/upload-list.txt" <<'PY'
from pathlib import Path
import sys
root = Path(sys.argv[1])
out = Path(sys.argv[2])
files = sorted(p for p in root.iterdir() if p.is_file())
# Installers first, then updater archive/sigs, then latest.json
def rank(p: Path):
    n = p.name.lower()
    if n.endswith((".dmg", ".exe", ".deb")):
        return (0, n)
    if ".app.tar.gz" in n:
        return (1, n)
    if n.endswith(".sig"):
        return (2, n)
    if n == "latest.json":
        return (3, n)
    return (4, n)
files.sort(key=rank)
out.write_text("\n".join(str(p) for p in files) + ("\n" if files else ""), encoding="utf-8")
print(f"queued {len(files)}")
PY

UPLOAD_OK=0
UPLOAD_FAIL=0

while IFS= read -r f; do
  [[ -z "$f" || ! -f "$f" ]] && continue
  name="$(basename "$f")"
  size_mb="$(python3 -c 'import os,sys; print(f"{os.path.getsize(sys.argv[1])/(1024*1024):.1f}")' "$f")"

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
  if [[ -n "$ATT_ID" && "$name" != "latest.json" ]]; then
    echo "EXISTS ${name} — skip"
    UPLOAD_OK=$((UPLOAD_OK + 1))
    continue
  fi
  if [[ -n "$ATT_ID" && "$name" == "latest.json" ]]; then
    curl -fsSL --max-time 60 -X DELETE \
      "${API}/releases/${RELEASE_ID}/attach_files/${ATT_ID}?access_token=${GITEE_TOKEN}" \
      >/dev/null || true
  fi

  echo "Uploading ${name} (${size_mb}MB) at $(date -u +%H:%M:%S)"
  # Token in query + form; abort stalled uploads (GH runners → Gitee can hang).
  if curl -fS \
    --connect-timeout 30 \
    --speed-time 30 \
    --speed-limit 1000 \
    --max-time 180 \
    -X POST \
    "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}" \
    -F "access_token=${GITEE_TOKEN}" \
    -F "file=@${f};filename=${name}" \
    -o "$TMP/upload-resp.json" \
    -w "http=%{http_code} bytes=%{size_upload} time=%{time_total}\n"; then
    echo "OK ${name}"
    UPLOAD_OK=$((UPLOAD_OK + 1))
    curl -fsSL --max-time 60 \
      "${API}/releases/${RELEASE_ID}/attach_files?access_token=${GITEE_TOKEN}&per_page=100" \
      -o "$TMP/gitee-attach.json" || true
  else
    echo "FAIL ${name} (curl exit $?) — response:" >&2
    head -c 500 "$TMP/upload-resp.json" 2>/dev/null >&2 || true
    echo >&2
    UPLOAD_FAIL=$((UPLOAD_FAIL + 1))
  fi
done <"$TMP/upload-list.txt"

echo "Upload summary: ok=${UPLOAD_OK} fail=${UPLOAD_FAIL}"
if [[ "$UPLOAD_FAIL" -gt 0 || "$UPLOAD_OK" -eq 0 ]]; then
  echo "ERROR: desktop/updater sync incomplete" >&2
  exit 1
fi

export TAG
"${SCRIPT_DIR}/publish_gitee_updater_manifest.sh"

echo "Done. Gitee release: https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/tag/${TAG}"
