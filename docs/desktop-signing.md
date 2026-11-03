# macOS desktop signing & notarization

The desktop package (`make pack-macos-desktop` / CI `Build Release Artifacts`) is signed
with a Developer ID Application certificate and notarized by Apple. Without the secrets
below it still builds, but the bundle is only ad-hoc signed and recipients must
right-click → Open.

## Required repo secrets

| Secret | Purpose |
|--------|---------|
| `APPLE_CERTIFICATE` | Developer ID Application `.p12`, single-line base64 |
| `APPLE_CERTIFICATE_PASSWORD` | Password used when exporting the `.p12` |
| `APPLE_SIGNING_IDENTITY` | e.g. `Developer ID Application: Foo (TEAMID)` — must match the cert CN |
| `APPLE_ID` | Apple ID email for notarization |
| `APPLE_PASSWORD` | App-specific password (`abcd-efgh-ijkl-mnop`) |
| `APPLE_TEAM_ID` | 10-character Team ID |
| `TAURI_SIGNING_PRIVATE_KEY` | Updater signature key (separate concern, see below) |

`scripts/ci_import_apple_cert.sh` imports the p12 into a throwaway keychain on the runner
and exports `APPLE_*` to later steps. It skips cleanly when `APPLE_CERTIFICATE` is unset.

## Signing order is the whole game

Signing seals the bundle contents. Touching anything inside a signed bundle afterwards
voids the Developer ID signature *and* the notarization ticket, and the failure mode is a
green CI run plus a "Danmo Work.app is damaged and can't be opened" dialog for users.

So `scripts/pack_desktop_macos.sh` signs **inner to outer**:

1. The Go sidecar (`desktop/src-tauri/bin/danmo-work-backend-<triple>`) is signed *before*
   `tauri build`, so Tauri copies a signed binary into `Contents/MacOS/`.
2. The bundled ripgrep (`desktop/src-tauri/resources/rg/rg`) is signed before the build for
   the same reason. It is a real Mach-O in `Contents/Resources/`; unsigned it is killed by
   the kernel under hardened runtime (the grep engine silently stops working) and
   notarization rejects the bundle.
3. Tauri signs and (with notarization credentials) notarizes the `.app` itself.
4. The script verifies the app and every Mach-O under `Contents/MacOS` and
   `Contents/Resources` carries a `Developer ID Application` authority, and fails the job
   otherwise.
5. Our own DMG (with the Chinese install notes) is built after Tauri's, so it is notarized
   and stapled separately — Tauri never notarizes it.

If a sidecar is somehow missing from the bundle, the script injects it, then re-signs every
nested binary, re-signs the app, and re-notarizes — never just the outer bundle.

`entitlements.plist` is intentionally empty: the app is not sandboxed, and the sidecar and
rg are signed individually rather than inheriting anything. Add a key only when
`notarytool` names it in a rejection.

## Timing

Tauri notarizes the `.app` inside `tauri build`. The build is ~4 minutes; Apple's notary
queue has been observed taking 33 minutes. The macOS job is capped at
`timeout-minutes: 90` — a long "Build macOS desktop" step is usually the queue, not a hang.

## Verifying a published build

```bash
# app carries a notarization ticket that validates offline
xcrun stapler validate "/Applications/Danmo Work.app"
spctl -a -vvv "/Applications/Danmo Work.app"   # → accepted / Notarized Developer ID
```

## Updater signatures

`TAURI_SIGNING_PRIVATE_KEY` signs the `.app.tar.gz` payload that `latest.json` points at.
It is unrelated to Apple signing: if the payload was signed with a key that does not match
`plugins.updater.pubkey` in `tauri.conf.json`, every client rejects the update. Keep
`updater.key.pub` and that `pubkey` in sync.

The updater archive is rebuilt from the **final** `.app` (after any re-sign / injection), so
what clients download carries exactly the signature that was verified above.
