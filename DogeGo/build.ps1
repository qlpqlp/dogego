# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Build DogeGo CLI for Windows (run from DogeGo repo root).
# Output: .\dogego.exe (ignored by repo *.exe rule)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
# Pebble (wallet.db + analytics) is pure Go when CGO is off; with CGO_ENABLED=1 the
# optional zstd dependency needs a C toolchain and go build fails on many Windows setups.
$env:CGO_ENABLED = "0"
go build -trimpath -buildvcs=true -o dogego.exe ./cmd/dogego
Write-Host "OK: $PSScriptRoot\dogego.exe"
& .\dogego.exe version
