# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Query GitHub for a newer DogeGo release (exit 2 when update available).
param(
    [switch]$Json
)
$dogego = Join-Path $PSScriptRoot "..\dogego.exe"
if (-not (Test-Path -LiteralPath $dogego)) {
    $dogego = "dogego"
}
if ($Json) {
    & $dogego version -json
    exit $LASTEXITCODE
}
& $dogego version -check
exit $LASTEXITCODE
