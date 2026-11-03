#!/usr/bin/env bash
# macOS Tauri desktop build — Cargo -> out/desktop/cargo, bundles -> out/desktop/bundle
#
# 签名 / 公证：
#   - 设了 APPLE_SIGNING_IDENTITY（且证书已在 keychain 里）→ Developer ID 签名，
#     CI 里由 scripts/ci_import_apple_cert.sh 先导入证书。
#   - 有 APPLE_ID / APPLE_PASSWORD / APPLE_TEAM_ID 时 tauri build 会把 .app 送去公证
#     并 staple；自制 DMG 由本脚本单独公证。
#   - 都没设 → ad-hoc 签名，产物只能自己机器用。
#
# 签名顺序是硬约束：**先签嵌套二进制，最后签 app**。签名把内容封进封印，任何
# 「先签 app 再往包里塞东西」都会让开发者签名与公证票据一起作废，用户端表现为
# 「已损坏，无法打开」，而 CI 还是绿的。老版本这里是 cp sidecar 之后
# `codesign --deep --sign -` 覆盖 app，正是这个错误模式。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
APP_DISPLAY="Danmo Work"
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-$DQ_DESKTOP_CARGO}"
dq_ensure_out_layout

if [[ "$(uname -s)" != Darwin ]]; then
  echo "pack-macos-desktop must run on macOS" >&2
  exit 1
fi

SIGN_IDENTITY="${APPLE_SIGNING_IDENTITY:-}"
SIGNED=0
NOTARIZED=0
if [[ -n "$SIGN_IDENTITY" ]]; then
  SIGNED=1
  echo "==> Signing identity: $SIGN_IDENTITY"
  if [[ -n "${APPLE_ID:-}" && -n "${APPLE_PASSWORD:-}" && -n "${APPLE_TEAM_ID:-}" ]]; then
    NOTARIZED=1
    echo "==> Notarization credentials present — tauri will notarize the .app"
  else
    echo "==> No APPLE_ID/APPLE_PASSWORD/APPLE_TEAM_ID — signed but NOT notarized"
  fi
else
  echo "==> No APPLE_SIGNING_IDENTITY — ad-hoc signature only (not distributable)"
fi

ENTITLEMENTS="$DQ_ROOT/desktop/src-tauri/entitlements.plist"

# is_macho <path>：判断是不是 Mach-O 可执行文件。
# 不靠路径猜——Tauri 会把 resources/ 整体拷进 Contents/Resources/resources/，
# 多一层目录，写死 Resources/rg/rg 会漏签。
is_macho() {
  local f="$1"
  [[ -f "$f" ]] || return 1
  file -b "$f" 2>/dev/null | grep -q "Mach-O"
}

# list_nested_binaries <app>
list_nested_binaries() {
  local app="$1" f
  while IFS= read -r -d '' f; do
    if is_macho "$f"; then
      printf '%s\0' "$f"
    fi
  done < <(find "$app/Contents/MacOS" "$app/Contents/Resources" -type f -print0 2>/dev/null)
}

# sign_binary <path>：给可执行文件单独签名。--options runtime 是公证的前置条件，
# --timestamp 让签名在证书有效期内长期可验证（没有它，公证会被拒）。
sign_binary() {
  local target="$1"
  [[ -f "$target" ]] || return 0
  codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" "$target"
}

# sign_app <app>：签 app 包本身（连同 entitlements）。--deep 已废弃——它只对
# 框架和 helper 生效，且会掩盖漏签的嵌套二进制，所以这里不传。
sign_app() {
  local app="$1"
  local args=(--force --options runtime --timestamp --sign "$SIGN_IDENTITY")
  if [[ -f "$ENTITLEMENTS" ]]; then
    args+=(--entitlements "$ENTITLEMENTS")
  fi
  codesign "${args[@]}" "$app"
}

cd "$DQ_ROOT/desktop"
if [[ ! -d node_modules ]]; then
  npm install
fi

# Desktop app needs to know the backend API URL
export VITE_API_BASE_URL="http://127.0.0.1:${DQ_BACKEND_PORT:-7801}"

# Prefer local updater key when CI secrets are not set
if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" && -z "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -f "$DQ_ROOT/desktop/src-tauri/keys/updater.key" ]]; then
  export TAURI_SIGNING_PRIVATE_KEY_PATH="$DQ_ROOT/desktop/src-tauri/keys/updater.key"
  export TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"
fi

# tauri build bundler only reads TAURI_SIGNING_PRIVATE_KEY (not PATH).
# Load from PATH when KEY is unset; then drop PATH so `tauri signer` doesn't conflict.
if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" && -n "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -f "${TAURI_SIGNING_PRIVATE_KEY_PATH}" ]]; then
  TAURI_SIGNING_PRIVATE_KEY="$(tr -d '\r\n' < "$TAURI_SIGNING_PRIVATE_KEY_PATH")"
  export TAURI_SIGNING_PRIVATE_KEY
fi
unset TAURI_SIGNING_PRIVATE_KEY_PATH

has_tauri_signing_key() {
  [[ -n "${TAURI_SIGNING_PRIVATE_KEY:-}" ]]
}

# Build Go backend as Tauri sidecar binary
echo "==> Building backend sidecar..."
"$SCRIPT_DIR/build_sidecar.sh"

SIDECAR_NAME="danmo-work-backend-$(rustc -vV | sed -n 's|host: ||p')"
SIDECAR_BIN="$DQ_ROOT/desktop/src-tauri/bin/$SIDECAR_NAME"

# sidecar 必须在 Tauri 把它拷进 .app **之前**签好：先打包再补签会毁掉整个包的签名。
if [[ "$SIGNED" == 1 ]]; then
  [[ -f "$SIDECAR_BIN" ]] || { echo "sidecar not found: $SIDECAR_BIN" >&2; exit 1; }
  sign_binary "$SIDECAR_BIN"
  echo "==> Signed sidecar: $SIDECAR_NAME"
fi

echo "==> Staging macOS first-launch script..."
"$SCRIPT_DIR/stage_first_launch.sh" darwin "$HOME/.danmo-work/first_launch"
"$SCRIPT_DIR/stage_first_launch.sh" darwin "$DQ_ROOT/desktop/src-tauri/resources/first_launch"

echo "==> Staging bundled ripgrep (grep engine)..."
"$SCRIPT_DIR/fetch_ripgrep.sh" "$DQ_ROOT/desktop/src-tauri/resources/rg"

# rg 落在 Contents/Resources 下，是货真价实的 Mach-O 可执行文件：Hardened Runtime
# 下未签名的它会被内核拒绝执行（grep 引擎直接失效），公证也会因此被拒。
RG_RES="$DQ_ROOT/desktop/src-tauri/resources/rg/rg"
if [[ "$SIGNED" == 1 && -f "$RG_RES" ]]; then
  sign_binary "$RG_RES"
  echo "==> Signed bundled ripgrep"
fi

echo "==> Tauri build ($APP_NAME) -> $CARGO_TARGET_DIR"
# Build .app (+ updater artifacts when signing key is present)
if has_tauri_signing_key; then
  npm run tauri build -- -b app
else
  echo "WARNING: no Tauri signing key — building without updater artifacts"
  npm run tauri build -- -b app --config '{"bundle":{"createUpdaterArtifacts":false}}'
fi

BUNDLE_SRC=""
for candidate in \
  "$CARGO_TARGET_DIR/release/bundle" \
  "$CARGO_TARGET_DIR/aarch64-apple-darwin/release/bundle" \
  "$DQ_ROOT/desktop/src-tauri/target/release/bundle" \
  "$DQ_ROOT/desktop/src-tauri/target/aarch64-apple-darwin/release/bundle"; do
  if [[ -d "$candidate" ]]; then
    BUNDLE_SRC="$candidate"
    break
  fi
done

if [[ -z "$BUNDLE_SRC" ]]; then
  echo "Tauri bundle not found under $CARGO_TARGET_DIR" >&2
  exit 1
fi

rm -rf "$DQ_DESKTOP_BUNDLE"/*
mkdir -p "$DQ_DESKTOP_BUNDLE"
cp -R "$BUNDLE_SRC"/* "$DQ_DESKTOP_BUNDLE/"

APP_BUNDLE=$(find "$DQ_DESKTOP_BUNDLE" -name "*.app" -maxdepth 2 -type d 2>/dev/null | head -1)
NEED_RESIGN=0

# Tauri 的 externalBin 会把 bin/danmo-work-backend-<triple> 拷进来（名字保留三元组，
# 见 src-tauri/src/lib.rs 的查找顺序）。极少数情况下它没拷成功，此时补一次注入——
# 但注入会破坏已签好的包，所以下面必须重签并重新公证。
if [[ -n "$APP_BUNDLE" ]]; then
  if [[ ! -f "$APP_BUNDLE/Contents/MacOS/$SIDECAR_NAME" && ! -f "$APP_BUNDLE/Contents/MacOS/danmo-work-backend" ]]; then
    [[ -f "$SIDECAR_BIN" ]] || { echo "ERROR: sidecar missing from app and bin: $SIDECAR_BIN" >&2; exit 1; }
    cp "$SIDECAR_BIN" "$APP_BUNDLE/Contents/MacOS/"
    echo "WARNING: Tauri 没有打包 sidecar，已手动注入 $SIDECAR_NAME"
    NEED_RESIGN=1
  else
    echo "==> Sidecar present in bundle: $(basename "$(find "$APP_BUNDLE/Contents/MacOS" -maxdepth 1 -name 'danmo-work-backend*' | head -1)")"
  fi
fi

if [[ "$SIGNED" == 1 && -n "$APP_BUNDLE" ]]; then
  # 无论 Tauri 有没有把嵌套二进制签好，这里统一再签一遍（幂等）：Tauri 只负责
  # 它自己产出的 app 签名，resources/ 里的 rg 它不一定管；漏签的 Mach-O 在
  # Hardened Runtime 下会被内核拒绝执行，公证也会被拒。
  nested_unsigned=0
  while IFS= read -r -d '' exe; do
    rep="$(codesign -dv --verbose=2 "$exe" 2>&1 || true)"
    if [[ "$rep" != *"Authority=Developer ID Application"* ]]; then
      nested_unsigned=1
      break
    fi
  done < <(list_nested_binaries "$APP_BUNDLE")

  if [[ "$NEED_RESIGN" == 1 || "$nested_unsigned" == 1 ]]; then
    # 重签必须从内到外：先把所有嵌套可执行文件签好，再签外壳。
    echo "==> Re-signing bundle (nested binaries first, then the app)"
    while IFS= read -r -d '' exe; do
      sign_binary "$exe"
    done < <(list_nested_binaries "$APP_BUNDLE")
    sign_app "$APP_BUNDLE"
    NEED_RESIGN=0

    # 重签把 Tauri 的公证票据变成了过期票据，必须重新公证 + staple。
    if [[ "$NOTARIZED" == 1 ]]; then
      echo "==> Re-notarizing .app after re-sign (may take several minutes)"
      ZIP="$DQ_DESKTOP_BUNDLE/_notarize.zip"
      ditto -c -k --keepParent "$APP_BUNDLE" "$ZIP"
      xcrun notarytool submit "$ZIP" \
        --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
      rm -f "$ZIP"
      xcrun stapler staple "$APP_BUNDLE"
    fi
  fi

  # 签名坏了就别上传：用户看到的会是「已损坏，无法打开」，而 CI 是绿的。
  echo "==> Verifying code signature..."
  codesign --verify --deep --strict --verbose=2 "$APP_BUNDLE"
  # 不要写成 `codesign -dv | grep -q`：grep 命中即退出，codesign 收到 SIGPIPE，
  # pipefail 会把签名正常也判成失败。
  sig_report="$(codesign -dv --verbose=2 "$APP_BUNDLE" 2>&1 || true)"
  if [[ "$sig_report" != *"Authority=Developer ID Application"* ]]; then
    echo "ERROR: $APP_BUNDLE 不是 Developer ID 签名（签名身份没生效）" >&2
    printf '%s\n' "$sig_report" >&2
    exit 1
  fi
  # 每个嵌套二进制都必须是同一个 Developer ID，漏签的那个会让 app 被拒。
  while IFS= read -r -d '' exe; do
    exe_report="$(codesign -dv --verbose=2 "$exe" 2>&1 || true)"
    if [[ "$exe_report" != *"Authority=Developer ID Application"* ]]; then
      echo "ERROR: 嵌套二进制未正确签名：$exe" >&2
      printf '%s\n' "$exe_report" >&2
      exit 1
    fi
  done < <(list_nested_binaries "$APP_BUNDLE")
  echo "==> Signature OK"

  if [[ "$NOTARIZED" == 1 ]]; then
    if xcrun stapler validate "$APP_BUNDLE" >/dev/null 2>&1; then
      echo "==> App bundle notarized (ticket stapled)"
    else
      echo "WARNING: .app 没有公证票据" >&2
    fi
  fi
elif [[ -n "$APP_BUNDLE" ]]; then
  # 本地无证书：ad-hoc 重签让机器上能跑（分发不可用）。
  codesign --force --deep --sign - "$APP_BUNDLE" 2>/dev/null && echo "==> Re-signed .app bundle (ad-hoc)" || echo "WARNING: codesign failed"
  xattr -cr "$APP_BUNDLE" 2>/dev/null && echo "==> Removed quarantine attributes" || true
fi

# Rebuild updater artifact from the final .app so updates include the backend and
# carry the signing/notarization state that was actually shipped.
if [[ -n "$APP_BUNDLE" && -d "$APP_BUNDLE" ]] && has_tauri_signing_key; then
  UPDATER_DIR="$DQ_DESKTOP_BUNDLE/macos"
  mkdir -p "$UPDATER_DIR"
  APP_BASENAME="$(basename "$APP_BUNDLE")"
  APP_VERSION=$(plutil -extract CFBundleShortVersionString raw "$APP_BUNDLE/Contents/Info.plist" 2>/dev/null || echo "0.0.0")
  ARCH=$(uname -m)
  case "$ARCH" in
    arm64) ARCH_LABEL="aarch64" ;;
    x86_64) ARCH_LABEL="x86_64" ;;
    *) ARCH_LABEL="$ARCH" ;;
  esac
  # Prefer space-free names for GitHub Releases / latest.json
  TAR_NAME="Danmo.Work_${APP_VERSION}_${ARCH_LABEL}.app.tar.gz"
  TAR_PATH="$UPDATER_DIR/$TAR_NAME"
  echo "==> Creating updater archive: $TAR_NAME"
  (
    cd "$(dirname "$APP_BUNDLE")"
    tar -czf "$TAR_PATH" "$APP_BASENAME"
  )
  echo "==> Signing updater archive..."
  npx tauri signer sign "$TAR_PATH" -p "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"
  # Drop any stale updater tarballs produced before sidecar injection
  find "$UPDATER_DIR" -maxdepth 1 -name '*.app.tar.gz' ! -name "$TAR_NAME" -delete 2>/dev/null || true
  find "$UPDATER_DIR" -maxdepth 1 -name '*.app.tar.gz.sig' ! -name "${TAR_NAME}.sig" -delete 2>/dev/null || true
  echo "==> Updater artifacts: $TAR_PATH (+ .sig)"
elif ! has_tauri_signing_key; then
  echo "WARNING: no Tauri signing key — skipping updater archive resign"
fi

# Create helper files for DMG
DMG_STAGING="$DQ_DESKTOP_BUNDLE/_dmg_staging"
rm -rf "$DMG_STAGING"
mkdir -p "$DMG_STAGING"

# Copy .app into staging
if [[ -n "$APP_BUNDLE" ]] && [[ -d "$APP_BUNDLE" ]]; then
  cp -R "$APP_BUNDLE" "$DMG_STAGING/"
fi

# 说明文件按真实签名状态分三种：没公证就别告诉用户「双击即可」，
# 公证过的也别再教他右键打开（那会让人以为这包有问题）。
if [[ "$NOTARIZED" == 1 ]]; then
  cat > "$DMG_STAGING/阅读说明.txt" << README_EOF
📦 ${APP_DISPLAY} 安装说明 (macOS)

  1. 将 ${APP_DISPLAY}.app 拖入「应用程序」文件夹
  2. 双击打开即可

本应用已使用 Developer ID 签名，并通过 Apple 公证。
README_EOF
elif [[ "$SIGNED" == 1 ]]; then
  cat > "$DMG_STAGING/阅读说明.txt" << README_EOF
📦 ${APP_DISPLAY} 安装说明 (macOS)

  1. 将 ${APP_DISPLAY}.app 拖入「应用程序」文件夹
  2. 双击打开即可

本应用已使用 Developer ID 签名，但未经过 Apple 公证。
首次打开若被拦截，请到「系统设置 → 隐私与安全性」点「仍要打开」。
README_EOF
else
  cat > "$DMG_STAGING/阅读说明.txt" << README_EOF
📦 ${APP_DISPLAY} 安装说明 (macOS)

由于本应用未使用 Apple 开发者签名，首次打开需要特殊操作。

安装步骤：
  1. 将 ${APP_DISPLAY}.app 拖入「应用程序」文件夹
  2. 在 Finder 中 右键（Control+点击）app → 选择「打开」
  3. 弹窗中点击「打开」确认

注意：必须使用右键 → 打开，直接双击会被 macOS 拦截。
      右键打开一次后，后续即可正常双击启动。

如果右键打开仍然被拦截：
  前往「系统设置 → 隐私与安全性」
  点击「仍要打开」按钮

终端修复（备选）：
  xattr -cr /Applications/${APP_DISPLAY}.app
README_EOF
fi

# Copy screenshot into DMG for reference (kept for the unsigned path where the
# Privacy & Security screenshot actually applies).
SCREENSHOT_SRC="$SCRIPT_DIR/../docs/gatekeeper-privacy-security.png"
if [[ "$NOTARIZED" != 1 && -f "$SCREENSHOT_SRC" ]]; then
  cp "$SCREENSHOT_SRC" "$DMG_STAGING/"
  echo "==> Copied screenshot to DMG"
fi

# Add Applications symlink
ln -s /Applications "$DMG_STAGING/Applications"

# Create DMG from staging directory
DMG_PATH=""
DMG_NAME=""
if [[ -d "$DMG_STAGING/$APP_DISPLAY.app" ]]; then
  DMG_DIR="$DQ_DESKTOP_BUNDLE/dmg"
  rm -rf "$DMG_DIR"
  mkdir -p "$DMG_DIR"
  APP_VERSION=$(plutil -extract CFBundleShortVersionString raw "$DMG_STAGING/$APP_DISPLAY.app/Contents/Info.plist" 2>/dev/null || echo "0.0.0")
  ARCH=$(uname -m)
  # Prefer space-free names for GitHub Releases / Homebrew Cask URLs
  DMG_NAME="Danmo.Work_${APP_VERSION}_${ARCH}.dmg"
  DMG_PATH="$DMG_DIR/$DMG_NAME"
  echo "==> Creating DMG: $DMG_NAME"
  hdiutil create -volname "$APP_DISPLAY" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG_PATH" 2>/dev/null \
    && echo "==> DMG created" || echo "WARNING: DMG creation failed"
fi

rm -rf "$DMG_STAGING"

# DMG 是在 Tauri 之后自己打的，Tauri 从没公证过它；不补这一步，用户双击 dmg
# 挂载后仍会被 Gatekeeper 拦（app 本身有票，dmg 没有）。
if [[ "$NOTARIZED" == 1 && -n "$DMG_PATH" && -f "$DMG_PATH" ]]; then
  echo "==> Notarizing $(basename "$DMG_PATH") (may take several minutes)"
  xcrun notarytool submit "$DMG_PATH" \
    --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
  xcrun stapler staple "$DMG_PATH"
  xcrun stapler validate "$DMG_PATH"
elif [[ "$SIGNED" == 1 ]]; then
  echo "WARNING: 有签名身份但缺公证凭据，跳过 DMG 公证：$(basename "${DMG_PATH:-none}")" >&2
fi

echo "==> Desktop bundle -> $DQ_DESKTOP_BUNDLE"
