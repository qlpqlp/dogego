# Build example.go source zip (manifest + icon + hello/ compiles at install if Go is on PATH).
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$zip = Join-Path $dist "hello-source.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path dogego.extension.json, icon.png, hello -DestinationPath $zip
Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
