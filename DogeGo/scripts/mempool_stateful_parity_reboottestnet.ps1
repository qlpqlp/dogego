# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Milestone D (partial): live stateful mempool rejects on reboottestnet (wallet + mine).
# Scenarios: dust, absurd_fee, non_final, rbf_*, coinbase_immature, package_* limits,
# min_relay_fee, mempool_double_spend (Core compare when reachable).
#
#   .\scripts\mempool_stateful_parity_reboottestnet.ps1 -Scenario all
param(
    [ValidateSet("dust", "absurd_fee", "non_final", "rbf_insufficient", "rbf_sufficient", "coinbase_immature", "package_ancestor_limit", "package_descendant_limit", "min_relay_fee", "mempool_double_spend", "rbf_not_replaceable", "rbf_fullrbf", "p2pkh_roundtrip", "rbf_too_many_descendants", "p2sh_nested_p2pkh", "p2sh_multisig", "bare_multisig", "p2sh_cltv_p2pk", "p2sh_csv_p2pk", "p2pk_non_standard_input", "package_ancestor_size", "package_descendant_size", "pq_commitment_op_return", "pq_carrier_p2sh_accept", "all")]
    [string]$Scenario = "all",
    [string]$Network = "reboottestnet",
    [int]$MineBlocks = 101,
    [string]$DogeGoRpcPort = "44556"
)
$ErrorActionPreference = "Stop"
. "$PSScriptRoot\dogego_rpc.ps1"

if ($Network -ne "reboottestnet") {
    Write-Error "Stateful live probe only supports reboottestnet (RelaxedPoW + wallet mine path)."
}

$rpcPort = [int]$DogeGoRpcPort
$failed = 0
$script:coreCompareRequired = ($env:DOGEGO_CORE_COMPARE -eq "1") -or ($env:DOGEGO_CORE_COMPARE_REQUIRED -eq "1")
$script:coreCompareStrict = ($env:DOGEGO_CORE_COMPARE_REQUIRED -eq "1")
$script:coreCompareMin = if ($env:DOGEGO_CORE_COMPARE_MIN) { [int]$env:DOGEGO_CORE_COMPARE_MIN } else { 24 }
$script:coreMismatches = 0
$script:coreCompared = 0

function Compare-CoreMempoolRow {
    param(
        [string]$Name,
        [string]$TxHex,
        [bool]$DgAllowed,
        [bool]$WantAccept,
        [string]$WantReasonSubstr = ""
    )
    if (-not $script:coreCompareRequired) { return }
    try {
        $coreRow = Test-MempoolAcceptRow -TxHex $TxHex -IsCore $true
    } catch {
        if ($script:coreCompareStrict) {
            Write-Host ("FAIL Core compare required but unreachable ($Name): $_") -ForegroundColor Red
            $script:coreMismatches++
        } else {
            Write-Host "Core compare skipped ($Name): $_" -ForegroundColor DarkGray
        }
        return
    }
    if ($null -eq $coreRow) {
        if ($script:coreCompareStrict) {
            Write-Host ("FAIL Core compare required but dogecoin-cli unavailable ($Name)") -ForegroundColor Red
            $script:coreMismatches++
        } else {
            Write-Host "Core compare unavailable ($Name)" -ForegroundColor DarkGray
        }
        return
    }
    $script:coreCompared++
    $coreAllowed = [bool]$coreRow.allowed
    if ($coreAllowed -ne $DgAllowed) {
        Write-Host ("FAIL Core drift $Name`: DogeGo allowed=$DgAllowed Core allowed=$coreAllowed") -ForegroundColor Red
        $script:coreMismatches++
        return
    }
    if (-not $WantAccept -and $WantReasonSubstr) {
        $cr = "$($coreRow.'reject-reason')"
        if ($cr -and $cr -notmatch $WantReasonSubstr) {
            Write-Host ("WARN Core reject reason differs on $Name`: $cr") -ForegroundColor DarkYellow
        }
    }
    Write-Host ("Core:   allowed={0} reject={1}" -f $coreRow.allowed, $coreRow.'reject-reason') -ForegroundColor DarkGray
}

function Invoke-Dg {
    param([string]$Method, [object[]]$Params = @())
    return Invoke-DogeGoJsonRpc -Method $Method -Params $Params -RpcPort $rpcPort -WarmupRetries 5 -WarmupDelaySec 2
}

function Test-MempoolAcceptRow {
    param([string]$TxHex, [bool]$IsCore)
    if ($IsCore) {
        $coreCli = $env:DOGEGO_CORE_CLI
        if (-not $coreCli) { $coreCli = (Get-Command dogecoin-cli -ErrorAction SilentlyContinue).Source }
        if (-not $coreCli) { return $null }
        $port = if ($env:DOGEGO_CORE_RPC_PORT) { $env:DOGEGO_CORE_RPC_PORT } else { "44555" }
        $param = '["' + $TxHex + '"]'
        $args = @("-rpcport=$port", "testmempoolaccept", $param)
        if ($env:DOGEGO_CORE_RPC_USER) { $args = @("-rpcuser=$env:DOGEGO_CORE_RPC_USER", "-rpcpassword=$env:DOGEGO_CORE_RPC_PASS") + $args }
        $out = & $coreCli @args 2>&1
        if ($LASTEXITCODE -ne 0) { throw ($out | Out-String) }
        return ($out | ConvertFrom-Json)[0]
    }
    $r = Invoke-Dg testmempoolaccept -Params @(@($TxHex))
    if ($r -is [System.Array]) { return $r[0] }
    return $r
}

function Assert-Reject {
    param(
        [string]$Name,
        [string]$TxHex,
        [string]$WantReasonSubstr
    )
    Write-Host "`n--- $Name ---" -ForegroundColor Yellow
    $dgRow = Test-MempoolAcceptRow -TxHex $TxHex -IsCore $false
    Write-Host ("DogeGo: allowed={0} reject={1}" -f $dgRow.allowed, $dgRow.'reject-reason')
    if ($dgRow.allowed -eq $true) {
        Write-Host "FAIL: expected reject on DogeGo" -ForegroundColor Red
        $script:failed++
        return
    }
    $reason = "$($dgRow.'reject-reason')"
    if ($WantReasonSubstr -and $reason -notmatch $WantReasonSubstr) {
        Write-Host ("FAIL: want reject containing '$WantReasonSubstr', got '$reason'") -ForegroundColor Red
        $script:failed++
        return
    }
    Compare-CoreMempoolRow -Name $Name -TxHex $TxHex -DgAllowed ([bool]$dgRow.allowed) -WantAccept $false -WantReasonSubstr $WantReasonSubstr
    Write-Host "OK $Name" -ForegroundColor Green
}

function Assert-Accept {
    param(
        [string]$Name,
        [string]$TxHex
    )
    Write-Host "`n--- $Name ---" -ForegroundColor Yellow
    $dgRow = Test-MempoolAcceptRow -TxHex $TxHex -IsCore $false
    Write-Host ("DogeGo: allowed={0} reject={1}" -f $dgRow.allowed, $dgRow.'reject-reason')
    if ($dgRow.allowed -ne $true) {
        Write-Host "FAIL: expected accept on DogeGo" -ForegroundColor Red
        $script:failed++
        return
    }
    Compare-CoreMempoolRow -Name $Name -TxHex $TxHex -DgAllowed $true -WantAccept $true
    Write-Host "OK $Name" -ForegroundColor Green
}

function Ensure-MatureFunds {
    $addr = Invoke-Dg getnewaddress -Params @("stateful_probe")
    Write-Host "Mining $MineBlocks blocks to $addr ..." -ForegroundColor DarkGray
    $hashes = Invoke-Dg generatetoaddress -Params @($MineBlocks, $addr)
    if (-not $hashes -or @($hashes).Count -lt 1) {
        Write-Error "generatetoaddress returned no blocks"
    }
}

function Build-FundedSignedTx {
    param([hashtable]$Outputs)
    $outputsJson = $Outputs | ConvertTo-Json -Compress
    $rawPartial = Invoke-Dg createrawtransaction -Params @("[]", $outputsJson)
    $funded = Invoke-Dg fundrawtransaction -Params @($rawPartial)
    if (-not $funded.hex) { Write-Error "fundrawtransaction returned no hex" }
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($funded.hex)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "signrawtransactionwithwallet incomplete"
    }
    return $signed.hex
}

function Build-AbsurdFeeSignedTx {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    if (-not $unspent -or @($unspent).Count -eq 0) {
        Write-Error "listunspent empty after mining"
    }
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    $changeAddr = Invoke-Dg getnewaddress -Params @("absurd_change")
    # Leave ~1 DOGE as output; remainder becomes fee (>100 DOGE max absurd on mainnet policy).
    $outDOGE = 1.0
    if ([double]$pick.amount -le 2.0) {
        Write-Error "UTXO too small for absurd-fee probe (need >2 DOGE, got $($pick.amount))"
    }
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $pick.txid; vout = [int]$pick.vout }) -Compress
    $outputsJson = (@{ $changeAddr = $outDOGE } | ConvertTo-Json -Compress)
    $rawPartial = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson)
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($rawPartial)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "signrawtransactionwithwallet incomplete (absurd fee)"
    }
    return $signed.hex
}

function Build-NonFinalSignedTx {
    $height = [int64](Invoke-Dg getblockcount)
    $lockTime = [int]$height + 50
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    $dest = Invoke-Dg getnewaddress -Params @("non_final_out")
    $seq = 4294967294  # SequenceFinal - 1
    $inputsJson = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $seq
        }) -Compress
    $outputsJson = (@{ $dest = 10.0 } | ConvertTo-Json -Compress)
    $rawPartial = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson, $lockTime)
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($rawPartial)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "signrawtransactionwithwallet incomplete (non-final)"
    }
    return $signed.hex
}

function Build-RBFInsufficientHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 20.0) {
        Write-Error "UTXO too small for RBF probe (need >=20 DOGE)"
    }
    $dest = Invoke-Dg getnewaddress -Params @("rbf_parent")
    $rbfSeq = 4294967293  # MaxBIP125RBFSequence
    $inputsJson = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $rbfSeq
        }) -Compress
    # Parent: small output, large fee.
    $outParent = 1.0
    $outputsParent = (@{ $dest = $outParent } | ConvertTo-Json -Compress)
    $rawParent = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsParent)
    $signedParent = Invoke-Dg signrawtransactionwithwallet -Params @($rawParent)
    if (-not $signedParent.complete) { Write-Error "parent sign failed" }
    try {
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    } catch {
        $acc = Test-MempoolAcceptRow -TxHex $signedParent.hex -IsCore $false
        if ($acc.allowed -ne $true) { Write-Error "parent tx not accepted: $($acc.'reject-reason')" }
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    }
    # Replacement: reclaim almost all value (insufficient fee bump vs parent).
    $inVal = [double]$pick.amount
    $outReplace = [math]::Round($inVal - 0.01, 8)
    $outputsReplace = (@{ $dest = $outReplace } | ConvertTo-Json -Compress)
    $rawReplace = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsReplace)
    $signedReplace = Invoke-Dg signrawtransactionwithwallet -Params @($rawReplace)
    if (-not $signedReplace.complete) { Write-Error "replacement sign failed" }
    return $signedReplace.hex
}

function Build-RBFSufficientHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 20.0) {
        Write-Error "UTXO too small for RBF sufficient probe (need >=20 DOGE)"
    }
    $dest = Invoke-Dg getnewaddress -Params @("rbf_sufficient")
    $rbfSeq = 4294967293
    $inputsJson = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $rbfSeq
        }) -Compress
    # Parent: modest output, large fee (e.g. ~19 DOGE fee on 20 DOGE input).
    $outParent = 1.0
    $outputsParent = (@{ $dest = $outParent } | ConvertTo-Json -Compress)
    $rawParent = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsParent)
    $signedParent = Invoke-Dg signrawtransactionwithwallet -Params @($rawParent)
    if (-not $signedParent.complete) { Write-Error "parent sign failed (rbf sufficient)" }
    try {
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    } catch {
        $acc = Test-MempoolAcceptRow -TxHex $signedParent.hex -IsCore $false
        if ($acc.allowed -ne $true) { Write-Error "parent tx not accepted: $($acc.'reject-reason')" }
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    }
    # Replacement: much smaller output => higher package fee (sufficient BIP125 bump).
    $inVal = [double]$pick.amount
    $outReplace = 0.1
    $outputsReplace = (@{ $dest = $outReplace } | ConvertTo-Json -Compress)
    $rawReplace = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsReplace)
    $signedReplace = Invoke-Dg signrawtransactionwithwallet -Params @($rawReplace)
    if (-not $signedReplace.complete) { Write-Error "replacement sign failed (rbf sufficient)" }
    return $signedReplace.hex
}

function Build-CoinbaseImmatureHex {
    $addr = Invoke-Dg getnewaddress -Params @("immature_cb")
    Write-Host "Mining 1 block for coinbase_immature probe ..." -ForegroundColor DarkGray
    $hashes = Invoke-Dg generatetoaddress -Params @(1, $addr)
    if (-not $hashes -or @($hashes).Count -lt 1) {
        Write-Error "generatetoaddress(1) returned no blocks"
    }
    $unspent = Invoke-Dg listunspent -Params @(0, 1, @($addr), $true, @{})
    $pick = $unspent | Where-Object { $_.txid -eq $hashes[0] } | Select-Object -First 1
    if (-not $pick) {
        $pick = $unspent | Select-Object -First 1
    }
    if (-not $pick) {
        Write-Error "no coinbase UTXO after mining 1 block"
    }
    $dest = Invoke-Dg getnewaddress -Params @("immature_spend")
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $pick.txid; vout = [int]$pick.vout }) -Compress
    $outAmt = [math]::Round([double]$pick.amount - 0.01, 8)
    if ($outAmt -le 0) { Write-Error "coinbase output too small" }
    $outputsJson = (@{ $dest = $outAmt } | ConvertTo-Json -Compress)
    $rawPartial = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson)
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($rawPartial)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "signrawtransactionwithwallet incomplete (coinbase immature)"
    }
    return $signed.hex
}

function Submit-ChainedSignedTx {
    param(
        [string]$PrevTxid,
        [int]$PrevVout,
        [double]$InAmt
    )
    $outAddr = Invoke-Dg getnewaddress -Params @("pkg_chain")
    $fee = 0.001
    $outAmt = [math]::Round($InAmt - $fee, 8)
    if ($outAmt -le 0) { Write-Error "chained tx: insufficient input $InAmt" }
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $PrevTxid; vout = $PrevVout }) -Compress
    $outputsJson = (@{ $outAddr = $outAmt } | ConvertTo-Json -Compress)
    $rawPartial = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson)
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($rawPartial)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "chained tx sign incomplete"
    }
    $txid = Invoke-Dg sendrawtransaction -Params @($signed.hex)
    if (-not $txid) { Write-Error "sendrawtransaction returned empty txid" }
    return @{ Txid = "$txid"; Vout = 0; Amount = $outAmt }
}

function Build-PackageAncestorLimitHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 5.0) {
        Write-Error "UTXO too small for package ancestor probe (need >=5 DOGE)"
    }
    $state = @{
        Txid   = "$($pick.txid)"
        Vout   = [int]$pick.vout
        Amount = [double]$pick.amount
    }
    # 26 chained mempool txs => 27th exceeds DEFAULT_ANCESTOR_LIMIT (25).
    for ($i = 0; $i -lt 26; $i++) {
        Write-Host ("  package chain hop {0}/26 ..." -f ($i + 1)) -ForegroundColor DarkGray
        $state = Submit-ChainedSignedTx -PrevTxid $state.Txid -PrevVout $state.Vout -InAmt $state.Amount
    }
    $dest = Invoke-Dg getnewaddress -Params @("pkg_over_limit")
    $outAmt = [math]::Round($state.Amount - 0.001, 8)
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $state.Txid; vout = $state.Vout }) -Compress
    $outputsJson = (@{ $dest = $outAmt } | ConvertTo-Json -Compress)
    $rawChild = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson)
    $signedChild = Invoke-Dg signrawtransactionwithwallet -Params @($rawChild)
    if (-not $signedChild.complete -or -not $signedChild.hex) {
        Write-Error "package ancestor child sign incomplete"
    }
    return $signedChild.hex
}

function Build-MinRelayFeeHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 2.0) {
        Write-Error "UTXO too small for min-relay probe (need >=2 DOGE)"
    }
    $dest = Invoke-Dg getnewaddress -Params @("minrelay_out")
    # Fee ~0.00001 DOGE on a ~250B tx is far below DEFAULT_MIN_RELAY (0.001 DOGE/kB).
    $outAmt = [math]::Round([double]$pick.amount - 0.00001, 8)
    if ($outAmt -le 0) { Write-Error "min relay probe: output non-positive" }
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $pick.txid; vout = [int]$pick.vout }) -Compress
    $outputsJson = (@{ $dest = $outAmt } | ConvertTo-Json -Compress)
    $rawPartial = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsJson)
    $signed = Invoke-Dg signrawtransactionwithwallet -Params @($rawPartial)
    if (-not $signed.complete -or -not $signed.hex) {
        Write-Error "signrawtransactionwithwallet incomplete (min relay)"
    }
    return $signed.hex
}

function Build-MempoolDoubleSpendHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 15.0) {
        Write-Error "UTXO too small for double-spend probe (need >=15 DOGE)"
    }
    $addr1 = Invoke-Dg getnewaddress -Params @("ds_first")
    $addr2 = Invoke-Dg getnewaddress -Params @("ds_second")
    $inputsJson = ConvertTo-Json -InputObject @(@{ txid = $pick.txid; vout = [int]$pick.vout }) -Compress
    $outputs1 = (@{ $addr1 = 10.0 } | ConvertTo-Json -Compress)
    $raw1 = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputs1)
    $signed1 = Invoke-Dg signrawtransactionwithwallet -Params @($raw1)
    if (-not $signed1.complete) { Write-Error "first spend sign failed" }
    try {
        Invoke-Dg sendrawtransaction -Params @($signed1.hex) | Out-Null
    } catch {
        $acc = Test-MempoolAcceptRow -TxHex $signed1.hex -IsCore $false
        if ($acc.allowed -ne $true) { Write-Error "first spend not accepted: $($acc.'reject-reason')" }
        Invoke-Dg sendrawtransaction -Params @($signed1.hex) | Out-Null
    }
    $outputs2 = (@{ $addr2 = 9.0 } | ConvertTo-Json -Compress)
    $raw2 = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputs2)
    $signed2 = Invoke-Dg signrawtransactionwithwallet -Params @($raw2)
    if (-not $signed2.complete -or -not $signed2.hex) {
        Write-Error "conflict spend sign failed"
    }
    return $signed2.hex
}

function Build-RBFNotReplaceableHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 20.0) {
        Write-Error "UTXO too small for RBF-not-replaceable probe (need >=20 DOGE)"
    }
    $dest = Invoke-Dg getnewaddress -Params @("rbf_final_parent")
    $finalSeq = 4294967295
    $inputsJson = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $finalSeq
        }) -Compress
    $outputsParent = (@{ $dest = 1.0 } | ConvertTo-Json -Compress)
    $rawParent = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsParent)
    $signedParent = Invoke-Dg signrawtransactionwithwallet -Params @($rawParent)
    if (-not $signedParent.complete) { Write-Error "non-replaceable parent sign failed" }
    try {
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    } catch {
        $acc = Test-MempoolAcceptRow -TxHex $signedParent.hex -IsCore $false
        if ($acc.allowed -ne $true) { Write-Error "parent not accepted: $($acc.'reject-reason')" }
        Invoke-Dg sendrawtransaction -Params @($signedParent.hex) | Out-Null
    }
    $rbfSeq = 4294967293
    $inputsJson2 = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $rbfSeq
        }) -Compress
    $inVal = [double]$pick.amount
    $outputsReplace = (@{ $dest = [math]::Round($inVal - 0.1, 8) } | ConvertTo-Json -Compress)
    $rawReplace = Invoke-Dg createrawtransaction -Params @($inputsJson2, $outputsReplace)
    $signedReplace = Invoke-Dg signrawtransactionwithwallet -Params @($rawReplace)
    if (-not $signedReplace.complete -or -not $signedReplace.hex) {
        Write-Error "replacement sign failed (not replaceable)"
    }
    return $signedReplace.hex
}

function Build-P2PKHRoundtripHex {
    $dest = Invoke-Dg getnewaddress -Params @("p2pkh_roundtrip")
    return Build-FundedSignedTx -Outputs @{ $dest = 10.0 }
}

function Build-RBFTooManyDescendantsHex {
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @(), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if ([double]$pick.amount -lt 30.0) {
        Write-Error "UTXO too small for RBF-too-many-descendants probe (need >=30 DOGE)"
    }
    $dest = Invoke-Dg getnewaddress -Params @("rbf_desc_parent")
    $rbfSeq = 4294967293
    $inputsJson = ConvertTo-Json -InputObject @(@{
            txid     = $pick.txid
            vout     = [int]$pick.vout
            sequence = $rbfSeq
        }) -Compress
    $outputsParent = (@{ $dest = 1.0 } | ConvertTo-Json -Compress)
    $rawParent = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsParent)
    $signedParent = Invoke-Dg signrawtransactionwithwallet -Params @($rawParent)
    if (-not $signedParent.complete) { Write-Error "RBF parent sign failed (too many descendants)" }
    $parentTxid = $null
    try {
        $parentTxid = Invoke-Dg sendrawtransaction -Params @($signedParent.hex)
    } catch {
        $acc = Test-MempoolAcceptRow -TxHex $signedParent.hex -IsCore $false
        if ($acc.allowed -ne $true) { Write-Error "parent not accepted: $($acc.'reject-reason')" }
        $parentTxid = Invoke-Dg sendrawtransaction -Params @($signedParent.hex)
    }
    if (-not $parentTxid) { Write-Error "parent txid missing" }
    $state = @{ Txid = "$parentTxid"; Vout = 0; Amount = 1.0 }
    for ($i = 0; $i -lt 26; $i++) {
        Write-Host ("  RBF descendant chain hop {0}/26 ..." -f ($i + 1)) -ForegroundColor DarkGray
        $state = Submit-ChainedSignedTx -PrevTxid $state.Txid -PrevVout $state.Vout -InAmt $state.Amount
    }
    $inVal = [double]$pick.amount
    $outputsReplace = (@{ $dest = 0.1 } | ConvertTo-Json -Compress)
    $rawReplace = Invoke-Dg createrawtransaction -Params @($inputsJson, $outputsReplace)
    $signedReplace = Invoke-Dg signrawtransactionwithwallet -Params @($rawReplace)
    if (-not $signedReplace.complete -or -not $signedReplace.hex) {
        Write-Error "replacement sign failed (too many descendants)"
    }
    return $signedReplace.hex
}

function Get-WalletProbeFunding {
    $addr = Invoke-Dg getnewaddress -Params @("stateful_wallet_anchor")
    $wif = Invoke-Dg dumpprivkey -Params @($addr)
    $unspent = Invoke-Dg listunspent -Params @(1, 9999999, @($addr), $true, @{})
    $pick = $unspent | Sort-Object { [double]$_.amount } -Descending | Select-Object -First 1
    if (-not $pick) {
        Write-Error "no UTXO for wallet probe funding address $addr"
    }
    if ([double]$pick.amount -lt 2.0) {
        Write-Error "wallet probe UTXO too small (need >=2 DOGE)"
    }
    $amountKoinu = [int64]([decimal]$pick.amount * 100000000)
    $fund = @{
        Wif         = "$wif"
        Txid        = "$($pick.txid)"
        Vout        = [int]$pick.vout
        AmountKoinu = $amountKoinu
        MineAddr    = $addr
    }
    $script:LastWalletProbeFunding = $fund
    return $fund
}

function Build-WalletAnchoredProbe {
    param(
        [string]$Template,
        [int64]$ChainHeight = 0,
        [switch]$SubmitBlock
    )
    $fund = Get-WalletProbeFunding
    $dogegoRoot = Split-Path -Parent $PSScriptRoot
    $goArgs = @(
        "run", "./cmd/statefulprobe",
        "-template", $Template,
        "-wif", $fund.Wif,
        "-txid", $fund.Txid,
        "-vout", $fund.Vout,
        "-amount", $fund.AmountKoinu
    )
    if ($ChainHeight -gt 0) {
        $goArgs += @("-height", $ChainHeight)
    }
    if ($SubmitBlock) {
        $tip = [int64](Invoke-Dg getblockcount)
        $hash = Invoke-Dg getblockhash -Params @($tip)
        $hdr = Invoke-Dg getblockheader -Params @($hash, $false)
        $goArgs += @("-submitblock", "-prevheader", "$hdr", "-mineheight", ($tip + 1))
    }
    Push-Location $dogegoRoot
    try {
        $out = & go @goArgs 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw ($out | Out-String)
        }
        $text = ($out | Out-String).Trim()
        return ($text | ConvertFrom-Json)
    } finally {
        Pop-Location
    }
}

function Submit-PrepMempoolTxs {
    param(
        [string[]]$PrepHex,
        [string]$Name
    )
    foreach ($hex in $PrepHex) {
        if (-not $hex) { continue }
        try {
            Invoke-Dg sendrawtransaction -Params @($hex) | Out-Null
        } catch {
            if ($Name -eq "p2pk_non_standard_input") {
                Write-Host "SKIP $Name prep (P2PK funding output is non-standard for relay)" -ForegroundColor DarkGray
                return $false
            }
            throw
        }
    }
    return $true
}

function Run-WalletAnchoredScenario {
    param(
        [string]$Name,
        [string]$Template,
        [int64]$CltvHeight = 0,
        [int]$CsvMineBlocks = 0,
        [string]$RejectSubstr = "",
        [switch]$SubmitBlockPrep
    )
    $probe = Build-WalletAnchoredProbe -Template $Template -ChainHeight $CltvHeight -SubmitBlock:$SubmitBlockPrep
    if ($probe.prep_submit_block_hex) {
        Write-Host "  submitblock prep (non-standard P2PK funding) ..." -ForegroundColor DarkGray
        $null = Invoke-Dg submitblock -Params @($probe.prep_submit_block_hex)
    } elseif (-not (Submit-PrepMempoolTxs -PrepHex $probe.prep_tx_hex -Name $Name)) {
        return
    }
    $mineAddr = $script:LastWalletProbeFunding.MineAddr
    if ($CltvHeight -gt 0) {
        Write-Host "  CLTV confirm anchor + mine to height >= $CltvHeight ..." -ForegroundColor DarkGray
        Invoke-Dg generatetoaddress -Params @(1, $mineAddr) | Out-Null
        $tip = [int64](Invoke-Dg getblockcount)
        $need = [int]([math]::Max(0, $CltvHeight - $tip))
        if ($need -gt 0) {
            Invoke-Dg generatetoaddress -Params @($need, $mineAddr) | Out-Null
        }
    }
    if ($CsvMineBlocks -gt 0) {
        Write-Host "  CSV confirm anchor + mine $CsvMineBlocks blocks ..." -ForegroundColor DarkGray
        Invoke-Dg generatetoaddress -Params @(1, $mineAddr) | Out-Null
        Invoke-Dg generatetoaddress -Params @($CsvMineBlocks, $mineAddr) | Out-Null
    }
    if ($probe.want_accept) {
        Assert-Accept -Name $Name -TxHex $probe.probe_tx_hex
    } else {
        Assert-Reject -Name $Name -TxHex $probe.probe_tx_hex -WantReasonSubstr $RejectSubstr
    }
}

Write-Host "=== Stateful mempool parity (reboottestnet, scenario=$Scenario) ===" -ForegroundColor Cyan

$info = Invoke-Dg getblockchaininfo
if ($info.chain -ne "reboottestnet") {
    Write-Error "Node chain=$($info.chain) want reboottestnet"
}

try {
    $null = Invoke-Dg getwalletinfo
} catch {
    Write-Error "Wallet required (enable wallet on reboottestnet node)."
}

if ($Scenario -eq "coinbase_immature" -or $Scenario -eq "all") {
    $hex = Build-CoinbaseImmatureHex
    Assert-Reject -Name "coinbase_immature" -TxHex $hex -WantReasonSubstr "immature"
}

if ($Scenario -ne "coinbase_immature") {
    Ensure-MatureFunds
}

if ($Scenario -eq "dust" -or $Scenario -eq "all") {
    $dustAddr = Invoke-Dg getnewaddress -Params @("dust_out")
    $hex = Build-FundedSignedTx -Outputs @{ $dustAddr = 0.00000001 }
    Assert-Reject -Name "dust_output_reject" -TxHex $hex -WantReasonSubstr "dust"
}

if ($Scenario -eq "absurd_fee" -or $Scenario -eq "all") {
    $hex = Build-AbsurdFeeSignedTx
    Assert-Reject -Name "absurd_fee" -TxHex $hex -WantReasonSubstr "absurd"
}

if ($Scenario -eq "non_final" -or $Scenario -eq "all") {
    $hex = Build-NonFinalSignedTx
    Assert-Reject -Name "non_final" -TxHex $hex -WantReasonSubstr "non-final"
}

if ($Scenario -eq "rbf_sufficient" -or $Scenario -eq "all") {
    $hex = Build-RBFSufficientHex
    Assert-Accept -Name "rbf_sufficient_fee" -TxHex $hex
}

if ($Scenario -eq "rbf_insufficient" -or $Scenario -eq "all") {
    $hex = Build-RBFInsufficientHex
    Assert-Reject -Name "rbf_insufficient_fee" -TxHex $hex -WantReasonSubstr "insufficient"
}

if ($Scenario -eq "package_ancestor_limit" -or $Scenario -eq "package_descendant_limit" -or $Scenario -eq "all") {
    if ($null -eq $script:packageOverLimitHex) {
        $script:packageOverLimitHex = Build-PackageAncestorLimitHex
    }
}

if ($Scenario -eq "package_ancestor_limit" -or $Scenario -eq "all") {
    Assert-Reject -Name "package_ancestor_limit" -TxHex $script:packageOverLimitHex -WantReasonSubstr "too-long-mempool-chain"
}

if ($Scenario -eq "package_descendant_limit" -or $Scenario -eq "all") {
    Assert-Reject -Name "package_descendant_limit" -TxHex $script:packageOverLimitHex -WantReasonSubstr "descendants|too-long-mempool-chain"
}

if ($Scenario -eq "min_relay_fee" -or $Scenario -eq "all") {
    $hex = Build-MinRelayFeeHex
    Assert-Reject -Name "min_relay_fee" -TxHex $hex -WantReasonSubstr "min fee"
}

if ($Scenario -eq "mempool_double_spend" -or $Scenario -eq "all") {
    $hex = Build-MempoolDoubleSpendHex
    Assert-Reject -Name "mempool_double_spend" -TxHex $hex -WantReasonSubstr "mempool-conflict"
}

if ($Scenario -eq "rbf_not_replaceable" -or $Scenario -eq "all") {
    $hex = Build-RBFNotReplaceableHex
    Assert-Reject -Name "rbf_not_replaceable" -TxHex $hex -WantReasonSubstr "mempool-conflict"
}

if ($Scenario -eq "rbf_fullrbf" -or $Scenario -eq "all") {
    $fullRbf = $false
    try {
        $mi = Invoke-Dg getmempoolinfo
        if ($null -ne $mi.fullrbf) { $fullRbf = [bool]$mi.fullrbf }
    } catch {
        Write-Host "getmempoolinfo.fullrbf unavailable; skipping full-RBF accept path" -ForegroundColor DarkGray
    }
    $hex = Build-RBFNotReplaceableHex
    if ($fullRbf) {
        Assert-Accept -Name "rbf_fullrbf" -TxHex $hex
    } else {
        Write-Host "`n--- rbf_fullrbf (mempoolfullrbf off) ---" -ForegroundColor Yellow
        $dgRow = Test-MempoolAcceptRow -TxHex $hex -IsCore $false
        if ($dgRow.allowed -eq $true) {
            Write-Host "FAIL: expected reject without mempoolfullrbf" -ForegroundColor Red
            $script:failed++
        } else {
            Write-Host "OK rbf_fullrbf skipped (enable mempoolfullrbf for accept probe)" -ForegroundColor Green
        }
    }
}

if ($Scenario -eq "p2pkh_roundtrip" -or $Scenario -eq "all") {
    $hex = Build-P2PKHRoundtripHex
    Assert-Accept -Name "p2pkh_roundtrip" -TxHex $hex
}

if ($Scenario -eq "rbf_too_many_descendants" -or $Scenario -eq "all") {
    $hex = Build-RBFTooManyDescendantsHex
    Assert-Reject -Name "rbf_too_many_descendants" -TxHex $hex -WantReasonSubstr "replacement|descendants"
}

$walletAnchored = @(
    @{ Scenario = "p2sh_nested_p2pkh"; Template = "p2sh_nested_p2pkh"; Reject = "" },
    @{ Scenario = "p2sh_multisig"; Template = "p2sh_multisig"; Reject = "" },
    @{ Scenario = "bare_multisig"; Template = "bare_multisig"; Reject = "" },
    @{ Scenario = "package_ancestor_size"; Template = "package_ancestor_size"; Reject = "too-long-mempool-chain" },
    @{ Scenario = "package_descendant_size"; Template = "package_descendant_size"; Reject = "too-long-mempool-chain" }
)

foreach ($row in $walletAnchored) {
    if ($Scenario -eq $row.Scenario -or $Scenario -eq "all") {
        Run-WalletAnchoredScenario -Name $row.Scenario -Template $row.Template -RejectSubstr $row.Reject
    }
}

if ($Scenario -eq "p2sh_cltv_p2pk" -or $Scenario -eq "all") {
    $lockH = [int64](Invoke-Dg getblockcount) + 10
    Run-WalletAnchoredScenario -Name "p2sh_cltv_p2pk" -Template "p2sh_cltv_p2pk" -CltvHeight $lockH
}

if ($Scenario -eq "p2sh_csv_p2pk" -or $Scenario -eq "all") {
    Run-WalletAnchoredScenario -Name "p2sh_csv_p2pk" -Template "p2sh_csv_p2pk" -CsvMineBlocks 3
}

if ($Scenario -eq "p2pk_non_standard_input" -or $Scenario -eq "all") {
    Run-WalletAnchoredScenario -Name "p2pk_non_standard_input" -Template "p2pk_non_standard_input" -RejectSubstr "non-standard-input" -SubmitBlockPrep
}

$pqWalletAnchored = @(
    @{ Scenario = "pq_commitment_op_return"; Template = "pq_commitment_op_return" },
    @{ Scenario = "pq_carrier_p2sh_accept"; Template = "pq_carrier_p2sh_accept" }
)
foreach ($row in $pqWalletAnchored) {
    if ($Scenario -eq $row.Scenario -or $Scenario -eq "all") {
        Run-WalletAnchoredScenario -Name $row.Scenario -Template $row.Template
    }
}

if ($failed -gt 0) {
    Write-Host "`nStateful mempool parity failed ($failed scenario(s))." -ForegroundColor Red
    exit 1
}
if ($script:coreCompareStrict -and $script:coreCompared -eq 0) {
    Write-Host "`nStateful mempool Core compare required but no rows compared (dogecoin-cli on reboottestnet?)." -ForegroundColor Red
    exit 1
}
if ($script:coreCompareStrict -and $Scenario -eq "all" -and $script:coreCompared -lt $script:coreCompareMin) {
    Write-Host ("`nStateful mempool Core compare required but only {0}/{1} rows compared." -f $script:coreCompared, $script:coreCompareMin) -ForegroundColor Red
    exit 1
}
if ($script:coreCompareRequired -and $script:coreMismatches -gt 0) {
    Write-Host "`nStateful mempool Core compare failed ($script:coreMismatches drift(s), $script:coreCompared compared)." -ForegroundColor Red
    exit 1
}
if ($script:coreCompared -gt 0) {
    Write-Host "Core side-by-side: $script:coreCompared scenario(s), 0 drift." -ForegroundColor DarkGray
}
Write-Host "`nStateful mempool parity passed ($Scenario)." -ForegroundColor Green
exit 0
