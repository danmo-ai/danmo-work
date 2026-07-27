# Windows Authenticode (SmartScreen / Smart App Control)

Danmo Work ships a Tauri NSIS installer (`Danmo.Work_*_x64-setup.exe`). The
**Tauri updater `.sig`** (minisign) only protects auto-update integrity — it does
**not** satisfy Windows code-signing trust. Without Authenticode, SmartScreen
shows “Windows protected your PC”, and Windows 11 Smart App Control may hard-block
with no “Run anyway”.

## Goal

Authenticode-sign the Windows setup.exe in CI via
[SignPath Foundation](https://signpath.org/) (free for OSS), then **re-sign**
Tauri updater artifacts so `latest.json` covers the signed PE bytes.

Order (must not reverse):

```
tauri pack (NSIS + updater .sig)
  → Authenticode (SignPath)        # rewrites PE
  → tauri signer re-sign setup.exe / nsis.zip
  → publish Release + latest.json
```

## One-time SignPath setup

1. Apply at [SignPath Foundation](https://signpath.org/) for open-source signing.
2. Create a SignPath project (suggested slug: `danmo-work`).
3. Install the **SignPath GitHub App** on `danmo-ai/danmo-work` and authorize Actions.
4. Create an Artifact Configuration slug **`windows-installer`** matching
   [`.signpath/artifact-configurations/windows-installer.xml`](../.signpath/artifact-configurations/windows-installer.xml)
   (zip wrapper → `Danmo.Work_*_x64-setup.exe` PE + Authenticode).
5. Create signing policies:
   - `test-signing` — test cert (validate pipeline first)
   - `release-signing` — Foundation release cert (after CSR becomes VALID)
6. Add **repository** secrets (not Environment secrets):

| Secret | Purpose |
|--------|---------|
| `SIGNPATH_API_TOKEN` | CI submitter token |
| `SIGNPATH_ORGANIZATION_ID` | SignPath org GUID |

7. Optional **repository variable**:
   - `SIGNPATH_RELEASE_SIGNING_READY=true` — use `release-signing` on `v*` tags;
     otherwise CI uses `test-signing` when SignPath secrets are present.

Without `SIGNPATH_*` secrets, the Windows job still builds and publishes an
**unsigned** installer (same as today).

## CI behavior

See `.github/workflows/release.yml` → `windows-desktop`:

- Upload unsigned `*setup.exe` as a short-lived Actions artifact (`archive` zip)
- `signpath/github-action-submit-signing-request@v2` signs via HSM (private key never in GitHub)
- Copy signed installer over the bundle path
- `scripts/ci_resign_windows_updater.sh` regenerates `.sig` for setup.exe and rebuilds/re-signs `*.nsis.zip`

Job needs `permissions.actions: read` so SignPath can pull the unsigned artifact.

## User-facing (until / if unsigned)

If the installer is still unsigned:

1. SmartScreen: **More info → Run anyway**
2. Or right-click → Properties → **Unblock** (Mark of the Web), then run
3. Smart App Control (Win11 clean installs): no bypass — need Authenticode or disable SAC

After SignPath release signing is live, the Digital Signatures tab on the `.exe`
should show the SignPath / Foundation publisher.

## Related

- Updater key (separate trust): [`updater-signing.md`](./updater-signing.md)
- macOS Gatekeeper / notarization: still open (ad-hoc / right-click Open)
