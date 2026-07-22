# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Register or run a daily GitHub release check (Task Scheduler on Windows).
param(
    [switch]$Register,
    [switch]$Unregister,
    [switch]$Json
)
$ErrorActionPreference = "Stop"
$taskName = "DogeGo Update Check"
$scriptPath = Join-Path $PSScriptRoot "check_update.ps1"
if (-not (Test-Path -LiteralPath $scriptPath)) {
    Write-Error "missing $scriptPath"
}
$tr = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`""
if ($Json) { $tr += " -Json" }

if ($Unregister) {
    schtasks /Delete /TN $taskName /F 2>$null | Out-Null
    Write-Output "Removed scheduled task '$taskName' (if it existed)."
    exit 0
}

if ($Register) {
    schtasks /Create /TN $taskName /TR $tr /SC DAILY /ST 09:00 /F | Out-Null
    Write-Output "Registered '$taskName' daily at 09:00 (runs check_update.ps1; exit 2 when update available)."
    exit 0
}

& $scriptPath @PSBoundParameters
exit $LASTEXITCODE
