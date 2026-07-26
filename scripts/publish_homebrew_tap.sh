#!/usr/bin/env bash
# Publish Casks/ into danmo-ai/homebrew-tap (short: brew tap danmo-ai/tap).
#
# Requires:
#   - Public GitHub repo danmo-ai/homebrew-tap (create once, empty is fine)
#   - HOMEBREW_TAP_TOKEN: PAT (or fine-grained token) with contents:write on that repo
#
# If the token or repo is missing, exits 0 after printing a skip message.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TAP_OWNER="${HOMEBREW_TAP_OWNER:-danmo-ai}"
TAP_REPO="${HOMEBREW_TAP_REPO:-homebrew-tap}"
TAP_SLUG="${TAP_OWNER}/${TAP_REPO}"
TOKEN="${HOMEBREW_TAP_TOKEN:-}"

if [[ -z "$TOKEN" ]]; then
  echo "HOMEBREW_TAP_TOKEN unset — skip publishing ${TAP_SLUG}"
  echo "Create ${TAP_SLUG}, add repo secret HOMEBREW_TAP_TOKEN, then re-run."
  exit 0
fi

if [[ ! -f "$ROOT/Casks/danmo-work.rb" ]]; then
  echo "Missing $ROOT/Casks/danmo-work.rb" >&2
  exit 1
fi

API="https://api.github.com/repos/${TAP_SLUG}"
HTTP="$(curl -sS -o /tmp/tap-repo.json -w '%{http_code}' \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  "$API")"

if [[ "$HTTP" == "404" ]]; then
  echo "Repo ${TAP_SLUG} not found — create an empty public repo, then re-run."
  exit 0
fi
if [[ "$HTTP" != "200" ]]; then
  echo "Failed to query ${TAP_SLUG} (HTTP ${HTTP})" >&2
  cat /tmp/tap-repo.json >&2 || true
  exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

CLONE_URL="https://x-access-token:${TOKEN}@github.com/${TAP_SLUG}.git"
if git ls-remote --exit-code "$CLONE_URL" HEAD >/dev/null 2>&1; then
  git clone --depth 1 "$CLONE_URL" "$WORK/tap"
else
  mkdir -p "$WORK/tap"
  git -C "$WORK/tap" init -b main
  git -C "$WORK/tap" remote add origin "$CLONE_URL"
fi

mkdir -p "$WORK/tap/Casks"
cp "$ROOT/Casks/danmo-work.rb" "$WORK/tap/Casks/danmo-work.rb"

cat > "$WORK/tap/README.md" <<'EOF'
# danmo-ai/homebrew-tap

Homebrew tap for [Danmo Work](https://github.com/danmo-ai/danmo-work).

```bash
brew tap danmo-ai/tap
brew install --cask danmo-work
```

Apple Silicon (arm64) only for now. The app is not Apple-notarized yet — on
first launch, right-click → Open.
EOF

git -C "$WORK/tap" config user.name "github-actions[bot]"
git -C "$WORK/tap" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git -C "$WORK/tap" add -A
if git -C "$WORK/tap" diff --staged --quiet; then
  echo "Tap ${TAP_SLUG} already up to date"
  exit 0
fi

VERSION="$(sed -nE 's/^  version "([^"]+)".*/\1/p' "$ROOT/Casks/danmo-work.rb" | head -1)"
git -C "$WORK/tap" commit -m "danmo-work ${VERSION}"
git -C "$WORK/tap" push -u origin HEAD:main

echo "Published ${TAP_SLUG} (danmo-work ${VERSION})"
