# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone B (partial): offline crash/corruption convergence certification.
# Runs subprocess-kill and startup recovery Go tests - no live node required.
#
#   .\scripts\corruption_soak_cert.ps1
$ErrorActionPreference = "Stop"
$DogeGo = Split-Path -Parent $PSScriptRoot
Set-Location $DogeGo

Write-Host "=== Corruption / crash recovery certification (offline) ===" -ForegroundColor Cyan

$tests = @(
    "go test ./store -run ""TestSubprocessKillDuringRawPut|TestSubprocessKillDuringHeaderSegmentAppend|TestSubprocessKillDuringBlockFilterPut|TestSubprocessKillDuringTxIndexWrite|TestCrashActive|TestCrashKillBeforeRawPutRename|TestHeaderSegmentTailRepairOnOpen|TestHeaderSegmentCheckpointRepair|TestHeaderSegmentPurgeStaleTemps|TestCrashActiveHeaderSegment|TestReconcileRawBlockSyncCheckpoint|TestPurgeStaleRawBlockSyncTemps|TestPurgeStaleHeaderSyncTemps|TestProbeBundledContiguousTipStopsAtTruncatedRecord|TestBundledTornTailReopenConvergence|TestBundledTornTailReconcilesInflatedCheckpoint|TestProbeBundledContiguousTipReconcilesInflatedCheckpoint|TestPickBundledAppendSlotAdvancesOffset"" -count=1 -timeout 180s",
    "go test ./consensus -run ""TestCrashActiveBundledContiguous_MainnetFieldBlocks|TestCrashActiveRawPut_MainnetFieldBlock10006|TestCrashActiveHeaderSegment_MainnetFieldHeaders"" -count=1 -timeout 120s",
    "go test ./node -run ""TestStartupRecoveryConvergence|TestCrashActivePutRestartConvergence|TestCrashKillBeforeRawPutSweepConvergence|TestCrashKillHeaderSegmentSweepConvergence|TestCrashHeaderSegmentsMidWriteRecovery|TestCrashIndexFilterTmpSweepRecovery|TestCrashHeaderAuxTornTailSweepRecovery|TestAutoRecoverSweepIsIdempotentOnCorruptionFixtures|TestAutoRecoverSweepReconcilesRawBlockSyncCheckpoint|TestAutoRecoverSweepClampsBundledContiguousAfterTornTail|TestMaybeClampBundledContiguousFromDiskAfterTornTail|TestIbdStallRecoverIntervalBodyIBDPaused|TestInboundServeBlackBox|TestBadNBitsRecoveryDecision|TestLoadUtxoSnapshot|TestRampReplayContiguousFromDiskBounded|TestShouldPreserve|TestResetContiguous"" -count=1 -timeout 180s",
    "go test ./store -run ""TestMeasureContiguousBodiesOnDisk"" -count=1 -timeout 60s"
)

foreach ($cmd in $tests) {
    Write-Host "`n> $cmd" -ForegroundColor Yellow
    Invoke-Expression $cmd
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $cmd" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

Write-Host "`nCorruption soak certification passed (offline kill + recovery tests)." -ForegroundColor Green
exit 0
