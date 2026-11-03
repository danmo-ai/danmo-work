#!/usr/bin/env bash
# 把 APPLE_CERTIFICATE（p12 的 base64）导入到 runner 上的临时 keychain，供 codesign /
# notarytool 使用；并把后续步骤需要的 APPLE_* 写进 GITHUB_ENV。
#
# 用法（GitHub Actions，macOS job）：
#   env:
#     APPLE_CERTIFICATE: ${{ secrets.APPLE_CERTIFICATE }}
#     APPLE_CERTIFICATE_PASSWORD: ${{ secrets.APPLE_CERTIFICATE_PASSWORD }}
#     APPLE_SIGNING_IDENTITY: ${{ secrets.APPLE_SIGNING_IDENTITY }}
#     APPLE_ID: ${{ secrets.APPLE_ID }}
#     APPLE_PASSWORD: ${{ secrets.APPLE_PASSWORD }}
#     APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
#   run: scripts/ci_import_apple_cert.sh
#
# 没设 APPLE_CERTIFICATE 时直接跳过：产物走 ad-hoc 签名，仍然能打包、能出更新包，
# 只是第一次打开需要右键「打开」。
set -euo pipefail

if [[ -z "${APPLE_CERTIFICATE:-}" ]]; then
  echo "No APPLE_CERTIFICATE secret — building unsigned (ad-hoc) desktop bundles"
  exit 0
fi

fail() { echo "::error::$*" >&2; exit 1; }

[[ -n "${APPLE_CERTIFICATE_PASSWORD:-}" ]] || fail "APPLE_CERTIFICATE 已设置，但缺 APPLE_CERTIFICATE_PASSWORD"
[[ -n "${APPLE_SIGNING_IDENTITY:-}" ]] || fail "APPLE_CERTIFICATE 已设置，但缺 APPLE_SIGNING_IDENTITY"
[[ -n "${RUNNER_TEMP:-}" ]] || fail "这个脚本只在 GitHub Actions 的 macOS runner 上用（缺 RUNNER_TEMP）"

KEYCHAIN="$RUNNER_TEMP/app-signing.keychain-db"
P12="$RUNNER_TEMP/app-signing.p12"
# 临时 keychain 的密码由 runner 临时生成，用完随机器销毁，不需要也不能由人工维护。
KEYCHAIN_PW="$(openssl rand -hex 24)"

# base64 进来时必须能解回 p12：粘贴截断是这一步最常见的失败，早报早改。
if ! printf '%s' "$APPLE_CERTIFICATE" | base64 --decode > "$P12" 2>/dev/null; then
  fail "APPLE_CERTIFICATE 不是合法 base64（通常是复制时被截断）"
fi
chmod 600 "$P12"

# macOS 的 base64 对非法字符很宽容，乱码也能"解码"成功，所以还要看是不是 PKCS#12：
# DER 编码的 p12 一定以 0x30 开头。导出成 .cer/.pem 是第二常见的错。
if [[ "$(od -An -tx1 -N1 "$P12" | tr -d ' \n')" != "30" ]]; then
  rm -f "$P12"
  fail "APPLE_CERTIFICATE 解码后不是 PKCS#12（DER 应以 0x30 开头）—— 导出时要选带私钥的 .p12"
fi

security create-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
security set-keychain-settings -lut 21600 "$KEYCHAIN"
security unlock-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"

# -T 预授权 codesign/security 使用这把私钥，后面 set-key-partition-list 才能真正免交互；
# 少了这两步，sign 时会弹钥匙串确认框，而 CI 里没人能点。
if ! security import "$P12" -k "$KEYCHAIN" -P "$APPLE_CERTIFICATE_PASSWORD" \
  -T /usr/bin/codesign -T /usr/bin/security >/dev/null 2>&1; then
  rm -f "$P12"
  security delete-keychain "$KEYCHAIN" 2>/dev/null || true
  fail "导入 p12 失败：APPLE_CERTIFICATE_PASSWORD 不对，或文件不是带私钥的 .p12"
fi
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PW" "$KEYCHAIN" >/dev/null
rm -f "$P12"

# 把临时 keychain 放进搜索列表并设为默认：codesign 只会在这里找到 Developer ID。
security list-keychains -d user -s "$KEYCHAIN" $(security list-keychains -d user | tr -d '"')
security default-keychain -s "$KEYCHAIN"

# 导入成功不等于身份可用：名字拼错、导入到别的 keychain 都要在这里暴露。
# 先查"有没有"（不带 -v：未受信任的证书也会列出来，报错文本才说得清问题在哪），
# 再查"可不可信"。不要用 `| grep -q`：grep 提前退出会让上游收到 SIGPIPE，
# 配合 pipefail 会把"找到身份"判成失败。
identities="$(security find-identity -p codesigning "$KEYCHAIN" 2>&1 || true)"
if [[ "$identities" != *"$APPLE_SIGNING_IDENTITY"* ]]; then
  rm -f "$P12"
  security delete-keychain "$KEYCHAIN" 2>/dev/null || true
  printf '%s\n' "$identities" >&2
  fail "导入后找不到签名身份：${APPLE_SIGNING_IDENTITY}（检查 APPLE_SIGNING_IDENTITY 是否与证书 CN 完全一致）"
fi

# 证书不在信任链里（缺 Developer ID 中间证书）时 -v 会把它过滤掉。签名仍然可能成功，
# 但产物装到别人机器上验不过，所以至少要让日志里留一句话。
valid="$(security find-identity -v -p codesigning "$KEYCHAIN" 2>&1 || true)"
if [[ "$valid" != *"$APPLE_SIGNING_IDENTITY"* ]]; then
  echo "::warning::证书已导入但不在信任链上（可能缺 Developer ID 中间证书），签名可能不被系统认可"
fi
echo "Signing identity ready: $APPLE_SIGNING_IDENTITY"

if [[ -z "${GITHUB_ENV:-}" ]]; then
  echo "APPLE_CERTIFICATE imported (not in Actions — nothing to export)"
  exit 0
fi

{
  echo "APPLE_SIGNING_IDENTITY=$APPLE_SIGNING_IDENTITY"
  # 公证凭据只在真的给了的时候才导出：导出空字符串会让 tauri 误以为可以公证。
  # 用 if 而不是 `[[ ]] && echo`：后者在条件不成立时返回 1，遇到 set -e 会直接退出。
  if [[ -n "${APPLE_ID:-}" ]]; then echo "APPLE_ID=$APPLE_ID"; fi
  if [[ -n "${APPLE_PASSWORD:-}" ]]; then echo "APPLE_PASSWORD=$APPLE_PASSWORD"; fi
  if [[ -n "${APPLE_TEAM_ID:-}" ]]; then echo "APPLE_TEAM_ID=$APPLE_TEAM_ID"; fi
} >> "$GITHUB_ENV"

if [[ -z "${APPLE_ID:-}" || -z "${APPLE_PASSWORD:-}" || -z "${APPLE_TEAM_ID:-}" ]]; then
  echo "::warning::缺公证凭据（APPLE_ID/APPLE_PASSWORD/APPLE_TEAM_ID），产物会签名但不公证"
fi
