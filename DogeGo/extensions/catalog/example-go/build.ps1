# Build example.go release zip (subprocess extension).
# Output goes to dist/ so the package root stays source-only.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$binName = if ($IsWindows -or $env:OS -match "Windows") { "hello-ext.exe" } else { "hello-ext" }
$bin = Join-Path $dist $binName
go build -ldflags="-s -w" -trimpath -o $bin ./hello
if (-not (Test-Path $bin)) { throw "build failed" }

$zip = Join-Path $dist "hello.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path dogego.extension.json, icon.png, $bin -DestinationPath $zip
Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
Write-Host "Install: Settings -> Extensions -> Install zip -> select dist\hello.zip"
