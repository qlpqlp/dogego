# Build example.go universal zip (all platform binaries + icon).
# Output: dist/hello-universal.zip works on any OS without Go at install time.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
$binRoot = Join-Path $dist "bin"
if (Test-Path $binRoot) { Remove-Item $binRoot -Recurse -Force }
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$platforms = @(
    @{ Key = "windows-amd64"; GOOS = "windows"; GOARCH = "amd64"; Out = "hello-ext.exe" },
    @{ Key = "windows-arm64"; GOOS = "windows"; GOARCH = "arm64"; Out = "hello-ext.exe" },
    @{ Key = "linux-amd64";   GOOS = "linux";   GOARCH = "amd64"; Out = "hello-ext" },
    @{ Key = "linux-arm64";   GOOS = "linux";   GOARCH = "arm64"; Out = "hello-ext" },
    @{ Key = "darwin-amd64";  GOOS = "darwin";  GOARCH = "amd64"; Out = "hello-ext" },
    @{ Key = "darwin-arm64";  GOOS = "darwin";  GOARCH = "arm64"; Out = "hello-ext" }
)

foreach ($p in $platforms) {
    $dir = Join-Path $binRoot $p.Key
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $out = Join-Path $dir $p.Out
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    go build -ldflags="-s -w" -trimpath -o $out ./hello
    if (-not (Test-Path $out)) { throw "build failed for $($p.Key)" }
}

$manifestPath = Join-Path $dist "dogego.extension.json"
$base = Get-Content -Raw dogego.extension.json | ConvertFrom-Json
$base.entry | Add-Member -NotePropertyName "binaries" -NotePropertyValue ([ordered]@{
    "windows-amd64" = "bin/windows-amd64/hello-ext.exe"
    "windows-arm64" = "bin/windows-arm64/hello-ext.exe"
    "linux-amd64"   = "bin/linux-amd64/hello-ext"
    "linux-arm64"   = "bin/linux-arm64/hello-ext"
    "darwin-amd64"  = "bin/darwin-amd64/hello-ext"
    "darwin-arm64"  = "bin/darwin-arm64/hello-ext"
}) -Force
$json = $base | ConvertTo-Json -Depth 10 -Compress
[System.IO.File]::WriteAllText($manifestPath, $json, [System.Text.UTF8Encoding]::new($false))

$zip = Join-Path $dist "hello-universal.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path $manifestPath, (Join-Path $PSScriptRoot "icon.png"), $binRoot -DestinationPath $zip
Remove-Item $manifestPath -Force

Write-Host "Wrote $zip ($((Get-Item $zip).Length) bytes)"
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "sha256=$hash"
