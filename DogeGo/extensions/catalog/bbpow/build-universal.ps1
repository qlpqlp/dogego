# Build dogego.bbpow universal zip (all platform binaries + icon + manifest).
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
$binRoot = Join-Path $dist "bin"
if (Test-Path $binRoot) { Remove-Item $binRoot -Recurse -Force }
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$platforms = @(
    @{ Key = "windows-amd64"; GOOS = "windows"; GOARCH = "amd64"; Out = "bbpow-ext.exe" },
    @{ Key = "windows-arm64"; GOOS = "windows"; GOARCH = "arm64"; Out = "bbpow-ext.exe" },
    @{ Key = "linux-amd64";   GOOS = "linux";   GOARCH = "amd64"; Out = "bbpow-ext" },
    @{ Key = "linux-arm64";   GOOS = "linux";   GOARCH = "arm64"; Out = "bbpow-ext" },
    @{ Key = "darwin-amd64";  GOOS = "darwin";  GOARCH = "amd64"; Out = "bbpow-ext" },
    @{ Key = "darwin-arm64";  GOOS = "darwin";  GOARCH = "arm64"; Out = "bbpow-ext" }
)

foreach ($p in $platforms) {
    $dir = Join-Path $binRoot $p.Key
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $out = Join-Path $dir $p.Out
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -trimpath -o $out ./cmd/bbpow-ext
    if (-not (Test-Path $out)) { throw "build failed for $($p.Key)" }
}

$manifestPath = Join-Path $dist "dogego.extension.json"
$base = Get-Content -Raw dogego.extension.json | ConvertFrom-Json
$base.entry | Add-Member -NotePropertyName "binaries" -NotePropertyValue ([ordered]@{
    "windows-amd64" = "bin/windows-amd64/bbpow-ext.exe"
    "windows-arm64" = "bin/windows-arm64/bbpow-ext.exe"
    "linux-amd64"   = "bin/linux-amd64/bbpow-ext"
    "linux-arm64"   = "bin/linux-arm64/bbpow-ext"
    "darwin-amd64"  = "bin/darwin-amd64/bbpow-ext"
    "darwin-arm64"  = "bin/darwin-arm64/bbpow-ext"
}) -Force
$json = $base | ConvertTo-Json -Depth 10 -Compress
[System.IO.File]::WriteAllText($manifestPath, $json, [System.Text.UTF8Encoding]::new($false))

$stage = Join-Path $dist "stage"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item $manifestPath (Join-Path $stage "dogego.extension.json") -Force
Copy-Item (Join-Path $PSScriptRoot "icon.png") (Join-Path $stage "icon.png") -Force
Copy-Item $binRoot (Join-Path $stage "bin") -Recurse -Force

$zip = Join-Path $dist "bbpow-universal.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
Remove-Item $manifestPath -Force
Remove-Item $stage -Recurse -Force

$legacy = Join-Path $dist "bbpow.zip"
Copy-Item $zip $legacy -Force
Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
