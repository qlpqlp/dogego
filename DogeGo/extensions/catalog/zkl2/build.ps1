# Build dogego.zkl2 universal subprocess zip (preferred).
# Also writes dist/zkl2.zip for catalog download_url compatibility.
$ErrorActionPreference = "Stop"
& "$PSScriptRoot/build-universal.ps1"
