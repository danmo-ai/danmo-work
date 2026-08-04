# Windows first-launch / post-install hooks (idempotent).
# Invoked asynchronously by the backend after startup via powershell.
# Env: DANMO_HOME (default %USERPROFILE%\.danmo-work)
#
# CodeGraph CLI is installed via the market connector (assets), not here.
$ErrorActionPreference = "Stop"

if (-not $env:DANMO_HOME -or $env:DANMO_HOME.Trim() -eq "") {
  $env:DANMO_HOME = Join-Path $env:USERPROFILE ".danmo-work"
}
New-Item -ItemType Directory -Force -Path $env:DANMO_HOME | Out-Null

function Log([string]$msg) {
  Write-Host "[first-launch:windows] $msg"
}

# Future windows-only hooks go below (PATH, Defender exclusions notes, etc.).

Log "done"
