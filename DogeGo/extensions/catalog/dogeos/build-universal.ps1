# Build dogego.dogeos universal zip (all platform binaries + icon + docs).
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
$binRoot = Join-Path $dist "bin"
if (Test-Path $binRoot) { Remove-Item $binRoot -Recurse -Force }
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$platforms = @(
    @{ Key = "windows-amd64"; GOOS = "windows"; GOARCH = "amd64"; Out = "dogeos-ext.exe" },
    @{ Key = "windows-arm64"; GOOS = "windows"; GOARCH = "arm64"; Out = "dogeos-ext.exe" },
    @{ Key = "linux-amd64";   GOOS = "linux";   GOARCH = "amd64"; Out = "dogeos-ext" },
    @{ Key = "linux-arm64";   GOOS = "linux";   GOARCH = "arm64"; Out = "dogeos-ext" },
    @{ Key = "darwin-amd64";  GOOS = "darwin";  GOARCH = "amd64"; Out = "dogeos-ext" },
    @{ Key = "darwin-arm64";  GOOS = "darwin";  GOARCH = "arm64"; Out = "dogeos-ext" }
)

$env:CGO_ENABLED = "0"
foreach ($p in $platforms) {
    $dir = Join-Path $binRoot $p.Key
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $out = Join-Path $dir $p.Out
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    go build -ldflags="-s -w" -trimpath -o $out ./cmd/dogeos-ext
    if (-not (Test-Path $out)) { throw "build failed for $($p.Key)" }
}
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

$manifestPath = Join-Path $dist "dogego.extension.json"
$base = Get-Content -Raw dogego.extension.json | ConvertFrom-Json
$base.entry | Add-Member -NotePropertyName "binaries" -NotePropertyValue ([ordered]@{
    "windows-amd64" = "bin/windows-amd64/dogeos-ext.exe"
    "windows-arm64" = "bin/windows-arm64/dogeos-ext.exe"
    "linux-amd64"   = "bin/linux-amd64/dogeos-ext"
    "linux-arm64"   = "bin/linux-arm64/dogeos-ext"
    "darwin-amd64"  = "bin/darwin-amd64/dogeos-ext"
    "darwin-arm64"  = "bin/darwin-arm64/dogeos-ext"
}) -Force
$json = $base | ConvertTo-Json -Depth 10 -Compress
[System.IO.File]::WriteAllText($manifestPath, $json, [System.Text.UTF8Encoding]::new($false))

$stage = Join-Path $dist "stage"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item $manifestPath (Join-Path $stage "dogego.extension.json") -Force
Copy-Item (Join-Path $PSScriptRoot "icon.png") (Join-Path $stage "icon.png") -Force
Copy-Item $binRoot (Join-Path $stage "bin") -Recurse -Force
Copy-Item (Join-Path $PSScriptRoot "docs") (Join-Path $stage "docs") -Recurse -Force

$zip = Join-Path $dist "dogeos-universal.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
Copy-Item $zip (Join-Path $dist "dogeos.zip") -Force
Remove-Item $manifestPath -Force
Remove-Item $stage -Recurse -Force

Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
