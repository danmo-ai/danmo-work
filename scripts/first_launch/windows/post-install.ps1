# Windows first-launch / post-install hooks (idempotent).
# Invoked asynchronously by the backend after startup via powershell.
# Env: DANMO_HOME (default %USERPROFILE%\.danmo-work)
$ErrorActionPreference = "Stop"

if (-not $env:DANMO_HOME -or $env:DANMO_HOME.Trim() -eq "") {
  $env:DANMO_HOME = Join-Path $env:USERPROFILE ".danmo-work"
}
$BinDir = Join-Path $env:DANMO_HOME "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

function Log([string]$msg) {
  Write-Host "[first-launch:windows] $msg"
}

function Install-CodeGraph {
  $archive = Join-Path $BinDir "codegraph.zip"
  $dest = Join-Path $BinDir "codegraph.exe"
  if (-not (Test-Path -LiteralPath $archive)) {
    Log "codegraph archive not found — skip"
    return
  }
  if (Test-Path -LiteralPath $dest) {
    Log "codegraph binary already present"
    return
  }
  $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("codegraph-extract-" + [guid]::NewGuid().ToString("N"))
  New-Item -ItemType Directory -Force -Path $tmp | Out-Null
  try {
    Expand-Archive -LiteralPath $archive -DestinationPath $tmp -Force
    $found = Get-ChildItem -Path $tmp -Recurse -Filter "codegraph.exe" -File | Select-Object -First 1
    if (-not $found) {
      throw "codegraph.exe missing inside archive"
    }
    Copy-Item -LiteralPath $found.FullName -Destination $dest -Force
    Log "extracted codegraph → $dest"
  }
  finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
  }
}

Install-CodeGraph

# Future windows-only hooks go below (PATH, Defender exclusions notes, etc.).

Log "done"
