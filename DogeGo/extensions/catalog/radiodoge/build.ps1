# Build dogego.radiodoge zip for this OS only.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$out = Join-Path $dist "radiodoge-ext.exe"
if ($IsLinux -or $IsMacOS) { $out = Join-Path $dist "radiodoge-ext" }

$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -trimpath -o $out ./cmd/radiodoge-ext
if (-not (Test-Path $out)) { throw "build failed" }

$stage = Join-Path $dist "stage-local"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Force -Path $stage | Out-Null
Copy-Item (Join-Path $PSScriptRoot "dogego.extension.json") (Join-Path $stage "dogego.extension.json")
Copy-Item (Join-Path $PSScriptRoot "icon.png") (Join-Path $stage "icon.png")
Copy-Item $out (Join-Path $stage (Split-Path $out -Leaf))
Copy-Item (Join-Path $PSScriptRoot "docs") (Join-Path $stage "docs") -Recurse

$zip = Join-Path $dist "radiodoge.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip
Remove-Item $stage -Recurse -Force
Write-Host "Wrote $zip"
Write-Host "sha256=$((Get-FileHash $zip -Algorithm SHA256).Hash.ToLower())"
