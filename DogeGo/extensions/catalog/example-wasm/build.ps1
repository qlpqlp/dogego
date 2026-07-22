# Build example.wasm release zip.
# Run from DogeGo/extensions/catalog/example-wasm
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path ping.wasm)) { throw "ping.wasm missing (see ping.wat)" }
if (Test-Path ping.zip) { Remove-Item ping.zip -Force }
Compress-Archive -Path dogego.extension.json, icon.png, ping.wasm -DestinationPath ping.zip
Write-Host "Wrote ping.zip ($((Get-Item ping.zip).Length) bytes)"
$hash = (Get-FileHash ping.zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
Write-Host "Update extensions/catalog/catalog.json example.wasm sha256 if publishing."
