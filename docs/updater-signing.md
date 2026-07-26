# Desktop updater signing

Generate once (password empty is fine):

```bash
cd desktop
npx tauri signer generate -w src-tauri/keys/updater.key -p ""
```

- Put the **public** key into `desktop/src-tauri/tauri.conf.json` → `plugins.updater.pubkey`
- Store the **private** key as a **repository** Actions secret (not Environment secret):

```bash
# From repo root — preferred (avoids paste corruption / Invalid padding)
gh secret set TAURI_SIGNING_PRIVATE_KEY < desktop/src-tauri/keys/updater.key

# Only if the key was generated WITH a password:
gh secret set TAURI_SIGNING_PRIVATE_KEY_PASSWORD --body 'your-password'
```

Common failure: `failed to decode base64 secret key: Invalid padding`

- Secret was truncated, quoted, or is the `.pub` file
- Paste into the GitHub UI mangled the one-line key — use `gh secret set ... < file` instead
- Secret must be the full contents of `updater.key` (~350 bytes), not double-base64-encoded

Optional China mirror secrets:

- `UPDATE_MIRROR_BASE_URL` — public HTTPS base (e.g. `https://releases.danmo.ai/danmo-work`); must match the first `plugins.updater.endpoints` entry host/path
- `UPDATE_MIRROR_S3_URI` — S3/OSS URI for `aws s3 sync` (e.g. `s3://bucket/danmo-work`)
- or `UPDATE_MIRROR_RCLONE_REMOTE` — rclone destination
- `UPDATE_MIRROR_AWS_ACCESS_KEY_ID` / `UPDATE_MIRROR_AWS_SECRET_ACCESS_KEY` / `UPDATE_MIRROR_AWS_REGION` / `UPDATE_MIRROR_AWS_ENDPOINT_URL` as needed for S3-compatible storage

On each `v*` release, CI syncs updater payloads **and** the macOS `.dmg` (Homebrew cask URL) to that mirror. App updater endpoints try the mirror first, then GitHub Releases `latest.json`. The Homebrew cask downloads from the same mirror (`Casks/danmo-work.rb`).

To backfill an older tag (e.g. after enabling mirror secrets), run workflow **Sync China Mirror** with the tag.

Local pack with updater artifacts:

```bash
export TAURI_SIGNING_PRIVATE_KEY_PATH="$PWD/desktop/src-tauri/keys/updater.key"
export TAURI_SIGNING_PRIVATE_KEY_PASSWORD=""
make pack-macos-desktop
```

Private keys under `desktop/src-tauri/keys/` are gitignored.
