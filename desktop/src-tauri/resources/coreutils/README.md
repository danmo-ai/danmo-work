# Bundled Microsoft Coreutils for Windows (default install)

`coreutils.exe` is **required** for `make pack-windows-desktop`. It is downloaded by
`scripts/fetch_windows_coreutils.sh` (not committed).

Install flow:
1. NSIS post-install copies it to `%USERPROFILE%\.danmo-work\bin\coreutils\`
2. Runs `coreutils-manager refresh` to create applet hardlinks
3. App first-launch also re-prepares as a safety net

See https://github.com/microsoft/coreutils (MIT).
