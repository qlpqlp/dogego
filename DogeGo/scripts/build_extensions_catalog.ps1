# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Windows helper: build catalog extension zips and refresh catalog.json hashes.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
$env:CGO_ENABLED = "0"
$bash = Get-Command bash -ErrorAction SilentlyContinue
if ($bash) {
  & bash ./scripts/build_extensions_catalog.sh
  exit $LASTEXITCODE
}
Write-Host "bash not found; building doginals via PowerShell only"
Set-Location (Join-Path (Get-Location) "extensions\catalog\doginals")
& .\build.ps1
$hash = (Get-FileHash .\dist\doginals.zip -Algorithm SHA256).Hash.ToLower()
Write-Host "doginals sha256=$hash (update catalog.json manually if needed)"
