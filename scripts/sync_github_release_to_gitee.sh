#!/usr/bin/env bash
# Publish a Gitee Release for one GitHub tag WITHOUT uploading binary attachments.
#
# Why: GitHub-hosted runners hang when multipart-uploading installers to Gitee.
# Instead we:
#   1) create/update the Gitee Release with notes + GitHub download links
#   2) publish a tiny latest.json on rolling Gitee release tag "updater"
#      (Tauri: .../releases/download/updater/latest.json)
#      Do not use a git branch — Pull mirror deletes non-GitHub branches.
#
# Artifact host is GitHub Releases. danmo.work is the marketing site (Pages),
# not a binary CDN. Optional MIRROR_BASE_URL rewrites links when a real mirror exists.
#
# Required env: GITEE_TOKEN, GH_TOKEN|GITHUB_TOKEN
# Optional: TAG, GITEE_OWNER, GITEE_REPO, GITHUB_REPOSITORY, MIRROR_BASE_URL
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GITEE_OWNER="${GITEE_OWNER:-danmo-ai}"
GITEE_REPO="${GITEE_REPO:-danmo-work}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-danmo-ai/danmo-work}"
export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
TAG="${TAG:-}"
MIRROR_BASE_URL="${MIRROR_BASE_URL:-}"

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

echo "==> Syncing Gitee release metadata for ${TAG} (no binary upload)"

gh api "repos/${GITHUB_REPOSITORY}/releases/tags/${TAG}" >"$TMP/gh-release.json"

python3 - "$TMP/gh-release.json" "$TMP" "$MIRROR_BASE_URL" "$GITHUB_REPOSITORY" "$GITEE_OWNER" "$GITEE_REPO" <<'PY'
import json, sys
from pathlib import Path

rel = json.load(open(sys.argv[1], encoding="utf-8"))
out = Path(sys.argv[2])
mirror = (sys.argv[3] or "").rstrip("/")
gh_repo = sys.argv[4]
gitee_owner, gitee_repo = sys.argv[5:7]
tag = rel["tag_name"]
name = rel.get("name") or tag
body = (rel.get("body") or "").rstrip()

def is_desktop(n: str) -> bool:
    return (
        n.endswith(".dmg")
        or n.endswith(".deb")
        or n.endswith("-setup.exe")
        or n.endswith(".AppImage")
        or n.endswith(".app.tar.gz")
    )

assets = [a for a in (rel.get("assets") or []) if a.get("name") and is_desktop(a["name"])]
assets.sort(key=lambda a: a["name"].lower())

lines = [
    body,
    "",
    "---",
    "",
    "## 下载 / Downloads",
    "",
    "安装包托管在 GitHub Releases（Gitee 不存放附件）。官网 https://danmo.work 为产品站，不是下载 CDN。",
    "",
]

if mirror:
    lines += [
        "| 文件 | 镜像 | GitHub |",
        "| --- | --- | --- |",
    ]
    for a in assets:
        n = a["name"]
        gh = a.get("browser_download_url") or f"https://github.com/{gh_repo}/releases/download/{tag}/{n}"
        lines.append(f"| `{n}` | [镜像]({mirror}/{n}) | [GitHub]({gh}) |")
else:
    lines += [
        "| 文件 | GitHub |",
        "| --- | --- |",
    ]
    for a in assets:
        n = a["name"]
        gh = a.get("browser_download_url") or f"https://github.com/{gh_repo}/releases/download/{tag}/{n}"
        lines.append(f"| `{n}` | [下载]({gh}) |")

lines += [
    "",
    f"- GitHub Release: https://github.com/{gh_repo}/releases/tag/{tag}",
    f"- 自动更新清单: https://gitee.com/{gitee_owner}/{gitee_repo}/releases/download/updater/latest.json",
    f"- 官网: https://danmo.work",
    "",
]
(out / "name").write_text(name, encoding="utf-8")
(out / "body").write_text("\n".join(lines).strip() + "\n", encoding="utf-8")
(out / "tag").write_text(tag, encoding="utf-8")
print(f"desktop assets linked: {len(assets)}")
PY

NAME="$(cat "$TMP/name")"
TAG="$(cat "$TMP/tag")"

DEFAULT_BRANCH="$(curl -fsSL --connect-timeout 20 --max-time 60 \
  "${API}?access_token=${GITEE_TOKEN}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("default_branch") or "master")')"

EXISTING="$(curl -sS --connect-timeout 20 --max-time 60 \
  "${API}/releases/tags/${TAG}?access_token=${GITEE_TOKEN}" || true)"
RELEASE_ID="$(python3 -c 'import json,sys
try:
  d=json.loads(sys.stdin.read() or "{}")
  print(d.get("id") or "")
except Exception:
  print("")
' <<<"$EXISTING")"

if [[ -z "$RELEASE_ID" ]]; then
  echo "Creating Gitee release ${TAG}"
  python3 - "$TAG" "$NAME" "$TMP/body" "$DEFAULT_BRANCH" <<'PY' >"$TMP/payload.json"
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
  RESP="$(curl -fsSL --connect-timeout 20 --max-time 60 -X POST "${API}/releases" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/payload.json")"
  RELEASE_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$RESP")"
else
  echo "Updating Gitee release id=${RELEASE_ID}"
  python3 - "$TAG" "$NAME" "$TMP/body" <<'PY' >"$TMP/payload.json"
import json, os, sys
tag, name, body_path = sys.argv[1:4]
json.dump({
  "access_token": os.environ["GITEE_TOKEN"],
  "tag_name": tag,
  "name": name,
  "body": open(body_path, encoding="utf-8").read(),
}, open("/dev/stdout", "w", encoding="utf-8"))
PY
  curl -fsSL --connect-timeout 20 --max-time 60 -X PATCH "${API}/releases/${RELEASE_ID}" \
    -H "Content-Type: application/json" \
    --data-binary @"$TMP/payload.json" >/dev/null
fi

export TAG MIRROR_BASE_URL
"${SCRIPT_DIR}/publish_gitee_updater_manifest.sh"

echo "Done."
echo "  Release: https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/tag/${TAG}"
echo "  Updater: https://gitee.com/${GITEE_OWNER}/${GITEE_REPO}/releases/download/updater/latest.json"
