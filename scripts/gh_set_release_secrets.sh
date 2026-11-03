#!/usr/bin/env bash
# 写入发布用的 GitHub repository secrets。
#
#   scripts/gh_set_release_secrets.sh                 证书直接从钥匙串导出
#   scripts/gh_set_release_secrets.sh -R owner/repo
#   scripts/gh_set_release_secrets.sh --p12 path.p12  从指定 p12 读（钥匙串里没有时）
#
# 证书、签名身份、Team ID 都能从本机钥匙串里拿到，所以不用准备 p12：
# 脚本用 security export 把 Developer ID 身份导出成临时 p12，转单行 base64 后写入
# APPLE_CERTIFICATE，导出密码随机生成，用户既不用记也不用输。
#
# 值都从 stdin 喂给 gh（不进 ps、不回显），临时文件在退出时删除。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

P12=""
REPO=""
KEY_PATH=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -R|--repo) REPO="${2:-}"; shift 2 ;;
    --p12) P12="${2:-}"; shift 2 ;;
    --key) KEY_PATH="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,12p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "未知参数：$1" >&2; exit 1 ;;
    *) P12="$1"; shift ;;
  esac
done

command -v gh >/dev/null || { echo "找不到 gh：brew install gh" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "先跑 gh auth login" >&2; exit 1; }

ask() {
  local def="${2:-}" v=""
  if [[ -n "$def" ]]; then printf '%s [%s]: ' "$1" "$def" >&2; else printf '%s: ' "$1" >&2; fi
  read -r v || true
  printf '%s' "${v:-$def}"
}
ask_secret() { printf '%s' "$1" >&2; local v=""; read -rs v || true; printf '\n' >&2; printf '%s' "$v"; }

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ghsecrets.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

put() { # put <名称> <文件>
  printf '  → %s ... ' "$1" >&2
  if gh secret set "$1" --repo "$REPO" < "$2" 2>/dev/null; then echo "ok" >&2; else echo "失败" >&2; fi
}

# 默认取 origin 的 OWNER/REPO。
if [[ -z "$REPO" ]]; then
  REPO="$(git -C "$ROOT" remote get-url origin 2>/dev/null \
    | sed -e 's#^git@[^:]*:##' -e 's#^https\{0,1\}://[^/]*/##' -e 's#\.git$##')"
fi

echo
REPO="$(ask "仓库 OWNER/REPO" "$REPO")"
echo "目标：$REPO"

# ---------------------------------------------------------------------------
# 证书：优先用钥匙串里现成的 Developer ID 身份
# ---------------------------------------------------------------------------

echo
echo "[1/2] macOS 签名证书"

# 列出可用的 Developer ID 身份。名字就是要写进 APPLE_SIGNING_IDENTITY 的完整值。
identities="$(security find-identity -v -p codesigning 2>/dev/null \
  | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p')"

if [[ -z "$identities" && -z "$P12" ]]; then
  echo "  钥匙串里没有 Developer ID Application 证书。" >&2
  echo "  它应该在「钥匙串访问」里（从 Apple Developer 后台下载的 .cer 双击导入）。" >&2
  P12="$(ask "也可以给一个 p12 路径（回车退出）" "")"
  P12="${P12/#\~/$HOME}"
fi

DEVELOPER_ID=""
if [[ -n "$P12" ]]; then
  P12="${P12/#\~/$HOME}"
  [[ -f "$P12" ]] || { echo "找不到 ${P12}" >&2; exit 1; }

  # 从文件读时才有密码这一步；空密码合法。
  p12_pw="$(ask_secret "p12 导出密码（没设密码就直接回车）：")"
  printf '%s\n' "$p12_pw" > "$TMP/pwfile"

  # 第 1/2 种读法：openssl。先默认（RC2/3DES/AES 都能读），再退 -legacy
  # （OpenSSL 3 读 RC2 需要它；LibreSSL 不认这个开关，所以只能放在后面）。
  read_subject() {
    local pem
    pem="$(openssl pkcs12 -in "$P12" -passin "file:$TMP/pwfile" -nokeys -clcerts "$@" 2>/dev/null)" || return 1
    [[ -n "$pem" ]] || return 1
    pem="$(printf '%s' "$pem" | openssl x509 -noout -subject -nameopt sep_multiline 2>/dev/null)" || return 1
    # 用 sep_multiline 而不是 RFC2253：公司名带逗号时 RFC2253 会转义成
    # `Danmo\, Inc.`，按逗号切就把身份截断了；多行格式每项一行，不会串。
    DEVELOPER_ID="$(printf '%s\n' "$pem" | sed -n 's/^[[:space:]]*CN[[:space:]]*=[[:space:]]*//p' \
      | sed -n '/^Developer ID Application/p' | head -1)"
    TEAM_FROM_OU="$(printf '%s\n' "$pem" | sed -n 's/^[[:space:]]*OU[[:space:]]*=[[:space:]]*//p' | head -1)"
    [[ -n "$DEVELOPER_ID" ]]
  }
  if ! read_subject; then
    read_subject -legacy || true
  fi

  # 第 3 种读法：交给系统导入一次。LibreSSL 读不了的 p12（新式 pbkdf2/AES）
  # security 往往没问题，而且它同时是"密码对不对"的判据。
  if [[ -z "$DEVELOPER_ID" ]]; then
    probe="$TMP/probe.keychain-db"
    probe_pw="probe-$$-$RANDOM"
    if security create-keychain -p "$probe_pw" "$probe" >/dev/null 2>&1 \
      && security unlock-keychain -p "$probe_pw" "$probe" >/dev/null 2>&1 \
      && security import "$P12" -k "$probe" -P "$p12_pw" -T /usr/bin/codesign >/dev/null 2>&1; then
      DEVELOPER_ID="$(security find-identity -p codesigning "$probe" 2>/dev/null \
        | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -1)"
    fi
    security delete-keychain "$probe" >/dev/null 2>&1 || true
  fi

  [[ -n "$DEVELOPER_ID" ]] || {
    echo "读不出证书里的 Developer ID 身份：密码不对，或导出的不是含私钥的 .p12。" >&2
    exit 1
  }

  base64 < "$P12" | tr -d '\n' > "$TMP/p12"
else
  # 多个身份时让作者选：签名用哪个是可验证的选择，不该由脚本猜。
  count="$(printf '%s\n' "$identities" | grep -c . || true)"
  if [[ "$count" -gt 1 ]]; then
    printf '可用签名身份：\n' >&2
    printf '%s\n' "$identities" | nl -w2 -s') ' >&2
    pick="$(ask "用第几个" "1")"
    DEVELOPER_ID="$(printf '%s\n' "$identities" | sed -n "${pick}p")"
  else
    DEVELOPER_ID="$identities"
  fi
  [[ -n "$DEVELOPER_ID" ]] || { echo "没有可用的签名身份" >&2; exit 1; }

  # 用随机密码把这个身份导出成临时 p12——这是 CI 需要的格式，密码只活在本次运行里。
  export_pw="$(openssl rand -hex 24)"
  if ! security export -k login.keychain-db -t identities -f pkcs12 -P "$export_pw" -o "$TMP/p12" >/dev/null 2>&1; then
    echo "从钥匙串导出证书失败。可以手动导出 p12 后重跑：--p12 <路径>" >&2
    exit 1
  fi
  base64 < "$TMP/p12" | tr -d '\n' > "$TMP/p12b64"
  mv "$TMP/p12b64" "$TMP/p12"
  printf '%s' "$export_pw" > "$TMP/pwfile"
fi

put APPLE_CERTIFICATE "$TMP/p12"

# 文件读入时用作者给的密码；钥匙串导出时用刚生成的随机密码。
if [[ -n "${export_pw:-}" ]]; then
  printf '%s' "$export_pw" > "$TMP/p12pw"
else
  printf '%s' "${p12_pw:-}" > "$TMP/p12pw"
fi
put APPLE_CERTIFICATE_PASSWORD "$TMP/p12pw"

# 身份串就直接用钥匙串/证书里的 CN，不需要作者抄。
printf '%s' "$(ask "签名身份" "$DEVELOPER_ID")" > "$TMP/id"
[[ -s "$TMP/id" ]] || { echo "签名身份不能为空" >&2; exit 1; }
# 身份串会原样进 codesign --sign；抄错一个字符，CI 要到打包那步才报错。
# 只提醒不拦：Apple 以后可能改名，硬拦会让脚本先于现实失效。
case "$(cat "$TMP/id")" in
  "Developer ID Application: "*) ;;
  *) echo "  ! 这个身份不是「Developer ID Application: …」，公证会失败" >&2 ;;
esac
put APPLE_SIGNING_IDENTITY "$TMP/id"

# ---------------------------------------------------------------------------
# 公证：Apple ID + App 专用密码。Team ID 从身份串/(OU) 里取。
# ---------------------------------------------------------------------------

echo
echo "[2/2] 公证"
printf '%s' "$(ask "Apple ID" "")" > "$TMP/appleid"
put APPLE_ID "$TMP/appleid"

printf '%s' "$(ask_secret "App 专用密码（abcd-efgh-ijkl-mnop）：")" > "$TMP/applepw"
put APPLE_PASSWORD "$TMP/applepw"

TEAM_ID="${TEAM_FROM_OU:-}"
if [[ -z "$TEAM_ID" ]]; then
  team_src="$(cat "$TMP/id")"
  TEAM_ID="$(printf '%s' "$team_src" | sed -n 's/.*(\([A-Z0-9]\{10\}\))[[:space:]]*$/\1/p')"
fi
printf '%s' "$(ask "Team ID" "$TEAM_ID")" > "$TMP/team"
[[ -s "$TMP/team" ]] || { echo "Team ID 不能为空" >&2; exit 1; }
put APPLE_TEAM_ID "$TMP/team"

# ---------------------------------------------------------------------------
# Tauri 更新包签名：release.yml 真正读的两个
# ---------------------------------------------------------------------------

echo
echo "[附加] Tauri 更新包签名"
DEFAULT_KEY="$ROOT/desktop/src-tauri/keys/updater.key"
if [[ -z "$KEY_PATH" ]]; then
  # 私钥就在仓库里的固定位置（已 gitignore），给成默认值，回车即可。
  # 目录里没有就给空默认——回车等于跳过这项。
  def_key=""
  [[ -f "$DEFAULT_KEY" ]] && def_key="desktop/src-tauri/keys/updater.key"
  KEY_PATH="$(ask "updater.key 路径（回车跳过）" "$def_key")"
  # 先展开 ~，再按仓库根解析相对路径：顺序反了会把 ~/x 拼成 <repo>/~/x。
  KEY_PATH="${KEY_PATH/#\~/$HOME}"
  [[ -z "$KEY_PATH" || "$KEY_PATH" == /* ]] || KEY_PATH="$ROOT/$KEY_PATH"
fi
KEY_PATH="${KEY_PATH/#\~/$HOME}"
if [[ -f "$KEY_PATH" ]]; then
  # 私钥必须和 tauri.conf.json 里的 updater.pubkey 配对，否则客户端收到的更新包
  # 验签必然失败——而这要等用户端点「更新」时才发现。生成的 .pub 就在私钥旁边。
  pub_file="$KEY_PATH.pub"
  conf_pub="$(sed -n 's/.*"pubkey":[[:space:]]*"\([^"]*\)".*/\1/p' "$ROOT/desktop/src-tauri/tauri.conf.json" 2>/dev/null | head -1)"
  if [[ -f "$pub_file" && -n "$conf_pub" ]]; then
    if [[ "$(tr -d '\r\n' < "$pub_file")" != "$conf_pub" ]]; then
      echo "  ! 这把私钥的公钥与 tauri.conf.json 的 updater.pubkey 不一致" >&2
      echo "  ! 用它签出的更新包，客户端会验签失败。确认没拿错 key 再继续。" >&2
    fi
  fi
  tr -d '\r' < "$KEY_PATH" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"//' -e 's/"$//' > "$TMP/key"
  put TAURI_SIGNING_PRIVATE_KEY "$TMP/key"
  pw="$(ask_secret "密钥密码（没设就直接回车）：")"
  if [[ -n "$pw" ]]; then
    printf '%s' "$pw" > "$TMP/pw"
    put TAURI_SIGNING_PRIVATE_KEY_PASSWORD "$TMP/pw"
  else
    echo "  → TAURI_SIGNING_PRIVATE_KEY_PASSWORD 跳过（空密码）" >&2
  fi
else
  echo "  跳过" >&2
fi

echo
gh secret list --repo "$REPO" 2>/dev/null | sed 's/^/  /'
echo
echo "触发一次发布：git tag v0.1.0 && git push origin v0.1.0"
