# Build dogego.bbpow release zip (subprocess research extension).
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$binName = if ($IsWindows -or $env:OS -match "Windows") { "bbpow-ext.exe" } else { "bbpow-ext" }
$bin = Join-Path $dist $binName
go build -ldflags="-s -w" -trimpath -o $bin ./cmd/bbpow-ext
if (-not (Test-Path $bin)) { throw "build failed" }

$zip = Join-Path $dist "bbpow.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path dogego.extension.json, icon.png, docs, $bin -DestinationPath $zip
Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
Write-Host "Install on TESTNET: Settings -> Extensions -> Install zip -> dist\bbpow.zip then Enable"
