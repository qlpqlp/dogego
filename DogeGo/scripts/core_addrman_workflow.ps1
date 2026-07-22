# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone E (partial): Core addrman summary when DogeGo P2P is active.
#
#   .\scripts\core_addrman_workflow.ps1
param(
    [switch]$Json
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

$issues = @()
$notes = @()

try {
    $info = Invoke-DogeGoJsonRpc -Method getaddrmaninfo -WarmupRetries 3 -WarmupDelaySec 1
} catch {
    if ($_.Exception.Message -match "P2P disabled|not enabled") {
        Write-Host "P2P disabled - skipping addrman workflow." -ForegroundColor DarkGray
        if ($Json) {
            @{ ok = $true; skipped = $true; reason = "p2p_disabled" } | ConvertTo-Json -Depth 6
        }
        exit 0
    }
    $issues += "getaddrmaninfo_failed"
    $info = $null
}

$chain = $null
try {
    $chain = Invoke-DogeGoJsonRpc -Method getblockchaininfo -WarmupRetries 1 -WarmupDelaySec 1
} catch {
    $notes += "getblockchaininfo_unavailable"
}

$tried = $null
$new = $null
$total = $null
$nKeySet = $null
$triedBucketsUsed = $null
$newBucketsUsed = $null
$triedBucketMaxFill = $null
$newBucketMaxFill = $null
$bucketSlotCap = $null
$bucketSchemaOK = $false

if ($info) {
    if ($info.all) {
        $tried = $info.all.tried
        $new = $info.all.new
        $total = $info.all.total
    }
    if ($info.dogego_buckets) {
        $b = $info.dogego_buckets
        $nKeySet = $b.n_key_set
        $triedBucketsUsed = $b.tried_buckets_used
        $newBucketsUsed = $b.new_buckets_used
        $triedBucketMaxFill = $b.tried_bucket_max_fill
        $newBucketMaxFill = $b.new_bucket_max_fill
        $bucketSlotCap = $b.bucket_slot_cap
        if ($b.tried_buckets_total -eq 256 -and $b.new_buckets_total -eq 1024 -and $b.bucket_slot_cap -eq 64) {
            $bucketSchemaOK = $true
        } else {
            $issues += "bucket_schema_mismatch"
        }
    } else {
        $issues += "dogego_buckets_missing"
    }
    if ($nKeySet) {
        $notes += "addrman_n_key_persisted"
    } else {
        $notes += "addrman_n_key_pending"
    }
    $notes += "addrman_partial: hash buckets + slot caps; not full Core slot tables"
}

$countsMatch = $false
if ($chain -and $null -ne $tried -and $null -ne $new) {
    if ($chain.dogego_addrbook_tried -eq $tried -and $chain.dogego_addrbook_new -eq $new) {
        $countsMatch = $true
    } else {
        $issues += "addrbook_counts_drift_vs_chaininfo"
    }
}

$ok = ($issues.Count -eq 0) -and $bucketSchemaOK -and $info

$result = @{
    ok                     = [bool]$ok
    tried                  = $tried
    new                    = $new
    total                  = $total
    n_key_set              = $nKeySet
    tried_buckets_used     = $triedBucketsUsed
    new_buckets_used       = $newBucketsUsed
    tried_bucket_max_fill  = $triedBucketMaxFill
    new_bucket_max_fill    = $newBucketMaxFill
    bucket_slot_cap        = $bucketSlotCap
    bucket_schema_ok       = $bucketSchemaOK
    counts_match_chaininfo = $countsMatch
    issues                 = $issues
    notes                  = $notes
    hint                   = "Partial Core addrman probe. Features tab: GET /api/core-addrman-probe"
}

if ($Json) {
    $result | ConvertTo-Json -Depth 8
    if (-not $ok -and $info) { exit 1 }
    exit 0
}

Write-Host ("addrman tried={0} new={1} n_key={2} buckets_used tried={3} new={4}" -f $tried, $new, $nKeySet, $triedBucketsUsed, $newBucketsUsed)
foreach ($n in $notes) { Write-Host ("  note: " + $n) -ForegroundColor DarkGray }
foreach ($i in $issues) { Write-Host ("  issue: " + $i) -ForegroundColor Yellow }
if (-not $ok -and $info) { exit 1 }
