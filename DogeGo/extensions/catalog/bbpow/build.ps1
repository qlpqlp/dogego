# Build dogego.bbpow release zip (subprocess research extension).
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
$stage = Join-Path $dist "stage"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

$binName = if ($IsWindows -or $env:OS -match "Windows") { "bbpow-ext.exe" } else { "bbpow-ext" }
$bin = Join-Path $dist $binName
go build -ldflags="-s -w" -trimpath -o $bin ./cmd/bbpow-ext
if (-not (Test-Path $bin)) { throw "build failed" }

Copy-Item dogego.extension.json (Join-Path $stage "dogego.extension.json") -Force
Copy-Item icon.png (Join-Path $stage "icon.png") -Force
Copy-Item docs (Join-Path $stage "docs") -Recurse -Force
Copy-Item $bin (Join-Path $stage $binName) -Force

$zip = Join-Path $dist "bbpow.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
# Stage first so Compress-Archive never stores "../icon.png" or nested dist paths.
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
Remove-Item $stage -Recurse -Force

Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
Write-Host "Install on TESTNET: Settings -> Extensions -> Install zip -> dist\bbpow.zip then Enable"
