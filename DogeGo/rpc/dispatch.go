// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
)

// SupportedMethods lists JSON-RPC entrypoints implemented by this build (for help).
func SupportedMethods() []string {
	base := []string{
		"addmultisigaddress",
		"addnode",
		"addwitnessaddress",
		"abandontransaction",
		"backupwallet",
		"bumpfee",
		"psbtbumpfee",
		"clearbanned",
		"combinerawtransaction",
		"createauxblock",
		"createmultisig",
		"createpsbt",
		"createrawtransaction",
		"converttopsbt",
		"decoderawtransaction",
		"decodepsbt",
		"analyzepsbt",
		"combinepsbt",
		"joinpsbts",
		"finalizepsbt",
		"utxoupdatepsbt",
		"decodescript",
		"disconnectnode",
		"dogego_probewalletdat",
		"dogego_recoverheaders",
		"dogego_verifypqcommitment",
		"dogego_createpqcarrier",
		"dogego_sendpqcarrier",
		"dogego_verifypqcarrier",
		"dogego_listextensions",
		"dogego_listextensioncatalog",
		"dogego_enableextension",
		"dogego_disableextension",
		"dogego_instextensionzip",
		"dogego_instextensionurl",
		"dogego_instextension",
		"dogego_uninstextension",
		"dogego_importmnemonic",
		"dogego_importbip38",
		"dogego_importwalletdat",
		"dogego_listwalletaddresses",
		"dogego_probewalletdat",
		"dumptxoutset",
		"loadtxoutset",
		"dumpprivkey",
		"dumpwallet",
		"echo",
		"echojson",
		"encryptwallet",
		"enumeratesigners",
		"estimatefee",
		"estimatepriority",
		"estimaterawfee",
		"estimatesmartfee",
		"estimatesmartpriority",
		"getdescriptorinfo",
		"deriveaddresses",
		"extractdescriptor",
		"fundrawtransaction",
		"generate",
		"generatetoaddress",
		"getaccount",
		"getaccountaddress",
		"getaddressinfo",
		"getaddednodeinfo",
		"getaddrmaninfo",
		"getaddressesbyaccount",
		"getaddressesbylabel",
		"getauxblock",
		"getbalance",
		"getbalances",
		"getbestblockhash",
		"getblock",
		"getblockchaininfo",
		"getblockcount",
		"getblockfilter",
		"getblockfilterheader",
		"getblockhash",
		"getblockheader",
		"getblockstats",
		"getblocktemplate",
		"getchaintips",
		"getchaintxstats",
		"getconnectioncount",
		"getdeploymentinfo",
		"getdifficulty",
		"getindexinfo",
		"getinfo",
		"getmempoolancestors",
		"getmempooldescendants",
		"getmempoolentry",
		"getmempoolinfo",
		"mempoolexists",
		"savemempool",
		"saveutxosnapshot",
		"loadmempool",
		"importmempool",
		"setmempoolpaused",
		"getmemoryinfo",
		"getmininginfo",
		"getmocktime",
		"getnetworkinfo",
		"getnodeaddresses",
		"getnetworkhashps",
		"getnewaddress",
		"getrawchangeaddress",
		"getnettotals",
		"getpeerinfo",
		"getrawmempool",
		"getrawtransaction",
		"getreceivedbyaccount",
		"getreceivedbyaddress",
		"getreceivedbylabel",
		"gettransaction",
		"gettxout",
		"gettxoutproof",
		"gettxspendingprevout",
		"gettxoutsetinfo",
		"getzmqnotifications",
		"scanblocks",
		"scantxoutset",
		"getunconfirmedbalance",
		"getwalletinfo",
		"verifytxoutproof",
		"getrpcinfo",
		"help",
		"invalidateblock",
		"importaddress",
		"importdescriptors",
		"importmulti",
		"importprivkey",
		"importprunedfunds",
		"importpubkey",
		"importwallet",
		"keypoolrefill",
		"listaccounts",
		"listaddressgroupings",
		"listbanned",
		"listdescriptors",
		"listlabels",
		"listlockunspent",
		"listreceivedbyaccount",
		"listreceivedbyaddress",
		"listreceivedbylabel",
		"liststucktransactions",
		"listsinceblock",
		"listtransactions",
		"listunspent",
		"lockunspent",
		"move",
		"ping",
		"preciousblock",
		"prioritisetransaction",
		"pruneblockchain",
		"reconsiderblock",
		"reindexblockfilters",
		"reindextx",
		"upgradetxindex",
		"removeprunedfunds",
		"rescan",
		"resendwallettransactions",
		"sendfrom",
		"sendmany",
		"sendrawtransaction",
		"submitpackage",
		"sendtoaddress",
		"setaccount",
		"setban",
		"setlabel",
		"setmaxconnections",
		"setmocktime",
		"setnetworkactive",
		"settxfee",
		"setwalletflag",
		"signmessage",
		"signmessagewithprivkey",
		"signrawtransaction",
		"signrawtransactionwithkey",
		"signrawtransactionwithwallet",
		"signerdisplayaddress",
		"simulaterawtransaction",
		"submitauxblock",
		"submitblock",
		"syncutxo",
		"syncutxocache",
		"stop",
		"testmempoolaccept",
		"truncatetoheight",
		"uptime",
		"validateaddress",
		"verifychain",
		"verifymessage",
		"descriptorprocesspsbt",
		"walletcreatefundedpsbt",
		"walletlock",
		"walletpassphrase",
		"walletpassphrasechange",
		"walletprocesspsbt",
		"waitforblock",
		"waitforblockheight",
		"waitfornewblock",
	}
	if extensionRPCCatalog != nil {
		return append(base, extensionRPCCatalog()...)
	}
	return base
}

// dispatchRequest handles one JSON-RPC call and returns a full envelope (jsonrpc + id + result or error).
func dispatchRequest(chainName string, j HeaderJournal, pool *mempool.Pool, paths *DataPaths, raw *store.RawBlockStore, txIndex *store.TxIndex, relayTx func([]byte) error, allowUnverifiedMempool bool, method string, params []json.RawMessage, id json.RawMessage) map[string]interface{} {
	out := map[string]interface{}{"jsonrpc": "1.0"}
	if len(id) > 0 {
		var idv interface{}
		if err := json.Unmarshal(id, &idv); err == nil {
			out["id"] = idv
		} else {
			out["id"] = json.RawMessage(id)
		}
	} else {
		out["id"] = nil
	}

	setResult := func(v interface{}) { out["result"] = v }
	setErr := func(code int, msg string) {
		out["error"] = map[string]interface{}{"code": code, "message": msg}
	}

	if paths != nil && !paths.RPCWhitelist.Allowed(method) {
		setErr(-32601, "Method not allowed")
		return out
	}

	switch method {
	case "":
		setErr(-32600, "invalid request: missing method")
		return out
	case "ping":
		res, code, msg := execPing(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "uptime":
		v, code, msg := execUptime(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(v)
		return out
	case "getrpcinfo":
		setResult(execGetRPCInfo(paths))
		return out
	case "help":
		v, code, msg := execHelpWithExtensions(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(v)
		return out
	case "getblockchaininfo":
		sync := computeChainIBDState(j, chainName, raw, paths)
		blocks, headers, contiguousH := sync.blocks, sync.headers, sync.contiguousH
		ibd, verProg := sync.ibd, sync.verProg
		maxTipAge, _ := ibdTimeParams(paths)
		gen, _ := j.GenesisHashHex()
		nHdr, _ := j.Count()
		statusNote := "DogeGo experimental node - not Core consensus; blocks = chainActive (UTXO/connect tip), headers = header journal tip; dogego_contiguous_raw_height = stored bodies"
		if allowUnverifiedMempool {
			statusNote += "; mempool accepts unverified transactions (not full-node safe, testing only)"
		}
		rawCount := 0
		if raw != nil {
			if n, err := raw.Count(); err == nil {
				rawCount = n
			}
		}
		best := ""
		if h, err := blockHashHexAt(j, blocks); err == nil {
			best = h
		} else {
			best, _ = j.BestBlockHashHex()
		}
		chainwork := "0"
		if w, ok := chainWorkThrough(j, blocks, paths); ok {
			chainwork = pow.ChainworkHex(w)
		} else if cw, err := cumulativeChainworkHex(j, blocks); err == nil {
			chainwork = cw
		}
		var sizeOnDisk int64
		if raw != nil {
			if b, err := raw.CachedBytesOnDisk(60 * time.Second); err == nil {
				sizeOnDisk += b
			}
		}
		if paths != nil && paths.ChainDataDir != "" {
			hdrPath := filepath.Join(paths.ChainDataDir, "headers.bin")
			if st, err := os.Stat(hdrPath); err == nil {
				sizeOnDisk += st.Size()
			}
		}
		if raw == nil && headers >= 0 {
			statusNote += "; no on-disk raw block store (SPV / headers-only this run)"
		}
		difficulty := 0.0
		if d, err := headerDifficultyAt(j, blocks); err == nil {
			difficulty = d
		}
		mediantime := int64(0)
		if mt, err := headerMedianTimePast(j, blocks); err == nil {
			mediantime = mt
		}
		net, _ := networkFromRPCChainName(chainName)
		var chainWarns []string
		if paths == nil || paths.UtxoConnectInFlight == nil || !paths.UtxoConnectInFlight() {
			chainWarns = consensus.ChainWarnings(j, net)
		}
		warnings := strings.Join(chainWarns, "; ")
		softforks, bip9 := BuildSoftforksForTip(j, blocks, net)
		res := map[string]interface{}{
			"chain":                             chainName,
			"blocks":                            blocks,
			"headers":                           headers,
			"bestblockhash":                     best,
			"difficulty":                        difficulty,
			"mediantime":                        mediantime,
			"dogego_genesis":                    gen,
			"dogego_header_count":               nHdr,
			"dogego_raw_blocks":                 rawCount,
			"dogego_contiguous_raw_height":      contiguousH,
			"dogego_tx_index":                   txIndex != nil,
			"pruned":                            corePrunedFromSummary(paths),
			"prune_height":                      pruneHeightFromSummary(paths),
			"initialblockdownload":              ibd,
			"verificationprogress":              verProg,
			"dogego_ibd_note":                   "initialblockdownload/verificationprogress use chainActive vs headers when behind; dogego_body_verification_progress is stored contiguous bodies (not script validation)",
			"chainwork":                         chainwork,
			"size_on_disk":                      sizeOnDisk,
			"dogego_size_on_disk_note":          sizeOnDiskNote(paths),
			"dogego_embedded_analytics_sidecar": paths != nil && paths.EmbeddedAnalyticsSidecar,
			"softforks":                         softforks,
			"bip9_softforks":                    bip9,
			"automatic_pruning":                 false,
			"prune_target_size":                 int64(0),
			"warnings":                          warnings,
			"dogego_status_note":                statusNote,
		}
		if av := consensus.GlobalAssumeValidSummary(); av != nil {
			for k, v := range av {
				res[k] = v
			}
		}
		for k, v := range minimumChainWorkFields(j, chainName, headers, paths) {
			res[k] = v
		}
		for k, v := range HeaderSyncDiagnostics(j, headers, blocks, paths) {
			res[k] = v
		}
		if v, ok := res["dogego_post_aux_era_header_stall"].(bool); ok && v {
			extra := "header tip stuck near height 510000 (~8% header progress) - rebuild dogego.exe if aux parent chain-id errors appear in logs"
			if warnings == "" {
				warnings = extra
			} else {
				warnings += "; " + extra
			}
			res["warnings"] = warnings
		}
		res["dogego_auxpow_parent_chain_id_core_parity"] = true
		res["dogego_checkpoints"] = consensus.HeaderCheckpointsEnabled()
		res["dogego_maxtipage"] = maxTipAge
		if paths == nil || paths.UtxoConnectInFlight == nil || !paths.UtxoConnectInFlight() {
			if tv, ok := TxVerificationProgress(chainName, j, txIndex, blocks); ok {
				res["dogego_tx_verification_progress"] = tv
			}
		}
		if pool != nil {
			res["mempool_txs"] = pool.Count()
		}
		res["storage_mode"] = store.StorageNative
		if paths != nil && paths.StorageSummary != nil {
			skipStorage := paths.UtxoConnectInFlight != nil && paths.UtxoConnectInFlight()
			if !skipStorage {
				for k, v := range paths.StorageSummary() {
					res["dogego_"+k] = v
				}
			} else {
				res["dogego_storage_summary_deferred"] = true
			}
		}
		res["dogego_chain_active_height"] = blocks
		blocksBehind := BlocksBehindHeaders(headers, blocks, contiguousH)
		if blocksBehind > 0 {
			res["dogego_blocks_behind_headers"] = blocksBehind
		}
		if contiguousH > blocks && blocks >= 0 {
			res["dogego_stored_bodies_ahead_connect"] = contiguousH - blocks
		}
		blocksPerMin := 0.0
		if paths != nil && paths.RawSyncProgress != nil {
			prog := paths.RawSyncProgress()
			res["dogego_raw_sync"] = prog
			if prog != nil {
				if v, ok := prog["blocks_per_minute"].(float64); ok {
					blocksPerMin = v
				}
				if v, ok := prog["connect_blocks_per_minute"].(float64); ok && v > 0 {
					res["dogego_connect_blocks_per_minute"] = v
				}
				if v, ok := prog["genesis_missing"].(bool); ok && v {
					res["dogego_genesis_missing"] = true
					res["dogego_genesis_note"] = "genesis block (height 0) is not in rawblocks/ yet; forward sync and getdata include height 0 when headers exist"
				}
				if v, ok := prog["lowest_missing_height"].(int64); ok && v >= 0 {
					res["dogego_lowest_missing_height"] = v
				}
				mergeDogegoRawSyncDiagnostics(res, prog)
			}
		}
		bodyProg := BodyVerificationProgress(headers, contiguousH)
		connProg := ConnectedVerificationProgress(headers, blocks)
		if headers > blocks && blocks >= 0 {
			res["dogego_headers_sync_progress"] = HeadersSyncProgress(headers, blocks)
		}
		eta := FormatSyncETA(blocksBehind, blocksPerMin)
		res["dogego_body_verification_progress"] = bodyProg
		res["dogego_connected_verification_progress"] = connProg
		res["dogego_sync_eta"] = eta
		nodeMode := "full"
		if raw == nil {
			nodeMode = "spv"
		}
		mpCount := 0
		if pool != nil {
			mpCount = pool.Count()
		}
		genesisMissing := false
		if v, ok := res["dogego_genesis_missing"].(bool); ok {
			genesisMissing = v
		}
		syncPhase := DogeGoSyncPhase(nodeMode, headers, blocks, contiguousH, genesisMissing)
		res["dogego_sync_phase"] = syncPhase
		if bodyBehind := BlocksBehindHeaders(headers, blocks, contiguousH); bodyBehind > blocksBehind {
			blocksBehind = bodyBehind
		}
		res["dogego_sync_status"] = SyncStatusLine(nodeMode, syncPhase, headers, contiguousH, bodyProg, blocksBehind, eta, mpCount)
		mergeUtxoSnapshotDiagnostics(res, paths, blocks)
		if aligned, ok := res["dogego_utxo_bodies_aligned"].(bool); ok && !aligned {
			if remain, ok := res["dogego_utxo_body_replay_remaining"].(int64); ok && remain > 0 {
				res["dogego_sync_phase"] = "snapshot_body_replay"
			}
		}
		if paths != nil && paths.FilterIndexThrough != nil {
			mergeBlockFilterDiagnostics(res, paths.FilterIndexThrough(), contiguousH)
		}
		headerCatchUp := paths != nil && paths.HeaderCatchUpPending != nil && paths.HeaderCatchUpPending()
		bodyPaused := BodyIBDOwnsPipeline(headers, contiguousH)
		if v, ok := res["dogego_body_ibd_header_paused"].(bool); ok && v {
			bodyPaused = true
		}
		effectiveHeaderCatchUp := headerCatchUp && !bodyPaused
		res["dogego_header_catch_up_pending"] = effectiveHeaderCatchUp
		res["dogego_headers_syncing"] = nodeMode != "spv" && headers >= 0 && effectiveHeaderCatchUp
		if paths != nil && paths.BlockAssistWorkersActive != nil {
			res["dogego_block_assist_active"] = paths.BlockAssistWorkersActive()
		}
		headerRecovery := ""
		if v, ok := res["dogego_header_sync_recovery"].(string); ok {
			headerRecovery = v
		}
		lastStored := int64(0)
		if paths != nil && paths.RawSyncProgress != nil {
			if prog := paths.RawSyncProgress(); prog != nil {
				if u, ok := prog["last_block_stored_at"].(int64); ok {
					lastStored = u
				}
			}
		}
		health, syncOK := SyncHealthAssessment(syncPhase, headers, blocks, blocksBehind, blocksPerMin, lastStored, headerRecovery, headerCatchUp)
		res["dogego_sync_health"] = health
		res["dogego_sync_ok"] = syncOK
		if paths != nil {
			if paths.P2PStats != nil {
				if snap := paths.P2PStats(); snap != nil {
					mergeDogegoAddrbookFromP2P(res, snap)
				}
			}
			if paths.BaseDataDir != "" {
				res["dogego_base_datadir"] = paths.BaseDataDir
			}
			if paths.ChainDataDir != "" {
				res["dogego_chain_datadir"] = paths.ChainDataDir
			}
			if paths.BlockFilterIndex != nil {
				syncH := BlockFilterSyncedHeight(j, blocks, paths.BlockFilterIndex)
				if syncH < 0 {
					syncH = 0
				}
				res["filters"] = map[string]interface{}{
					"active": []string{"basic"},
					"height": syncH,
				}
			}
		}
		setResult(res)
		return out
	case "validateaddress":
		res, code, msg := execValidateAddress(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getaddressinfo":
		res, code, msg := execGetAddressInfo(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "verifymessage":
		ok, code, msg := execVerifyMessage(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(ok)
		return out
	case "walletcreatefundedpsbt":
		res, code, msg := execWalletCreateFundedPsbt(chainName, paths, j, raw, txIndex, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "descriptorprocesspsbt":
		res, code, msg := execDescriptorProcessPsbt(chainName, paths, txIndex, raw, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "walletprocesspsbt":
		res, code, msg := execWalletProcessPsbt(chainName, paths, txIndex, raw, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "walletlock":
		var res interface{}
		var code int
		var msg string
		if paths != nil && rpcWalletIsEncrypted(paths) {
			res, code, msg = execWalletLockPaths(paths, params)
		} else if paths != nil && rpcWalletDefaultAddress(paths) != "" {
			res, code, msg = execWalletLockUnencrypted(params)
		} else {
			res, code, msg = execWalletLock(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "walletpassphrase":
		var res interface{}
		var code int
		var msg string
		if paths != nil && rpcWalletIsEncrypted(paths) {
			res, code, msg = execWalletPassphrasePaths(paths, params)
		} else if paths != nil && rpcWalletDefaultAddress(paths) != "" {
			res, code, msg = execWalletPassphraseUnencrypted(params)
		} else {
			res, code, msg = execWalletPassphrase(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "walletpassphrasechange":
		var res interface{}
		var code int
		var msg string
		if paths != nil && rpcWalletIsEncrypted(paths) {
			res, code, msg = execWalletPassphraseChangePaths(paths, params)
		} else if paths != nil && rpcWalletDefaultAddress(paths) != "" {
			res, code, msg = execWalletPassphraseChangeUnencrypted(params)
		} else {
			res, code, msg = execWalletPassphraseChange(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "waitfornewblock":
		res, code, msg := execWaitForNewBlock(paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "waitforblock":
		res, code, msg := execWaitForBlock(paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "waitforblockheight":
		res, code, msg := execWaitForBlockHeight(paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "signmessage":
		res, code, msg := execSignMessage(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "signmessagewithprivkey":
		sig, code, msg := execSignMessageWithPrivkey(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(sig)
		return out
	case "signrawtransaction":
		res, code, msg := execSignRawTransaction(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "signrawtransactionwithkey":
		res, code, msg := execSignRawTransactionWithKey(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "signrawtransactionwithwallet":
		res, code, msg := execSignRawTransactionWithWallet(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "signerdisplayaddress":
		res, code, msg := execSignerDisplayAddress(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "verifychain":
		var aux *store.HeaderAuxJournal
		if paths != nil {
			aux = paths.HeaderAux
		}
		var utxo *store.UtxoCache
		if paths != nil {
			utxo = paths.Utxo
		}
		res, code, msg := execVerifyChain(chainName, j, aux, raw, txIndex, paths, utxo, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblockcount":
		setResult(ActiveChainBlockHeight(j, raw, paths))
		return out
	case "getbestblockhash":
		blocks := ActiveChainBlockHeight(j, raw, paths)
		best, err := blockHashHexAt(j, blocks)
		if err != nil {
			setErr(-1, err.Error())
			return out
		}
		setResult(best)
		return out
	case "getaccount":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetAccountWallet(paths, chainName, params)
		} else {
			res, code, msg = execGetAccount(chainName, params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getaccountaddress":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetAccountAddressWallet(paths, params)
		} else {
			res, code, msg = execGetAccountAddress(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getbalance":
		res, code, msg := execGetBalance(paths, j, raw, txIndex, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getbalances":
		res, code, msg := execGetBalances(chainName, paths, j, raw, pool, txIndex, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getchaintxstats":
		res, code, msg := execGetChainTxStats(j, raw, txIndex, paths, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getdeploymentinfo":
		res, code, msg := execGetDeploymentInfo(j, raw, paths, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dumptxoutset":
		res, code, msg := execDumpTxOutSet(j, raw, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "loadtxoutset":
		res, code, msg := execLoadTxOutSet(j, raw, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "mempoolexists":
		res, code, msg := execMempoolExists(pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "savemempool":
		res, code, msg := execSaveMempool(pool, paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "saveutxosnapshot":
		var utxo *store.UtxoCache
		if paths != nil {
			utxo = paths.Utxo
		}
		res, code, msg := execSaveUtxoSnapshot(utxo, paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "syncutxo", "syncutxocache":
		res, code, msg := execSyncUtxo(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "loadmempool":
		res, code, msg := execLoadMempool(pool, paths, j, txIndex, raw, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importmempool":
		res, code, msg := execImportMempool(pool, paths, j, txIndex, raw, networkFromChainName(chainName), params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setmempoolpaused":
		res, code, msg := execSetMempoolPaused(pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getchaintips":
		tips, err := buildGetChainTips(j, raw, paths)
		if err != nil {
			setErr(-1, err.Error())
			return out
		}
		setResult(tips)
		return out
	case "getconnectioncount":
		n, code, msg := execGetConnectionCount(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(n)
		return out
	case "getblockhash":
		if len(params) < 1 {
			setErr(-8, "getblockhash: height required")
			return out
		}
		var h float64
		if err := json.Unmarshal(params[0], &h); err != nil || h < 0 || h > float64(math.MaxInt64) || h != float64(int64(h)) {
			setErr(-8, "getblockhash: invalid height")
			return out
		}
		hi := int64(h)
		buf, err := j.ReadHeaderAt(hi)
		if err != nil {
			setErr(-8, err.Error())
			return out
		}
		setResult(pow.BlockHashHex(buf))
		return out
	case "getdifficulty":
		blocks := ActiveChainBlockHeight(j, raw, paths)
		d, err := headerDifficultyAt(j, blocks)
		if err != nil {
			setErr(-1, err.Error())
			return out
		}
		setResult(d)
		return out
	case "getindexinfo":
		var filters *store.BlockFilterIndex
		if paths != nil {
			filters = paths.BlockFilterIndex
		}
		var utxo *store.UtxoCache
		if paths != nil {
			utxo = paths.Utxo
		}
		setResult(execGetIndexInfo(txIndex, filters, j, raw, utxo, paths))
		return out
	case "reindexblockfilters":
		var filters *store.BlockFilterIndex
		if paths != nil {
			filters = paths.BlockFilterIndex
		}
		res, code, msg := execReindexBlockFilters(paths, j, raw, txIndex, filters)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "upgradetxindex":
		res, code, msg := execUpgradeTxIndex(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "reindextx":
		res, code, msg := execReindexTx(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getinfo":
		res, code, msg := execGetInfo(chainName, j, raw, paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "estimatesmartfee":
		res, code, msg := execEstimateSmartFee(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "estimaterawfee":
		res, code, msg := execEstimateRawFee(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "estimatesmartpriority":
		res, code, msg := execEstimateSmartPriority(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "estimatefee":
		res, code, msg := execEstimateFee(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "estimatepriority":
		res, code, msg := execEstimatePriority(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getnetworkinfo":
		statusNote := "DogeGo experimental Go node - headers-first; see dogego_note fields"
		var chainWarns []string
		if net, err := networkFromRPCChainName(chainName); err == nil {
			chainWarns = consensus.ChainWarnings(j, net)
		}
		warnings := strings.Join(chainWarns, "; ")
		proto := chain.ProtocolVersion
		sub := chain.BuildSubVersion("")
		serv := chain.NodeNetwork
		if paths != nil && paths.LocalP2P != nil {
			proto, sub, serv = paths.LocalP2P()
		}
		netActive := true
		if paths != nil && paths.NetworkActive != nil {
			netActive = paths.NetworkActive()
		}
		conn, cin, cout := 0, 0, 0
		p2pMode := "both"
		if paths != nil && paths.ConnectionCount != nil && netActive {
			conn = paths.ConnectionCount()
		} else if netActive {
			conn, cout = 1, 1
		}
		if netActive && paths != nil && paths.P2PStats != nil {
			if snap := paths.P2PStats(); snap != nil {
				if v, ok := snap["connections_inbound"].(int); ok {
					cin = v
				}
				if v, ok := snap["connections_outbound"].(int); ok {
					cout = v
				}
				if paths.ConnectionCount == nil {
					if v, ok := snap["connections_total"].(int); ok {
						conn = v
					}
				}
				if v, ok := snap["p2p_connectivity"].(string); ok && v != "" {
					p2pMode = v
				}
				if v, ok := snap["health_message"].(string); ok && v != "" {
					if statusNote != "" {
						statusNote += "; "
					}
					statusNote += v
				}
			}
		}
		res := map[string]interface{}{
			"version":                1140900,
			"subversion":             sub,
			"protocolversion":        proto,
			"localservices":          FormatServicesHex(serv),
			"localservicesnames":     LocalServiceNames(serv),
			"localrelay":             true,
			"timeoffset":             medianPeerTimeOffset(paths),
			"networkactive":          netActive,
			"connections":            conn,
			"connections_in":         cin,
			"connections_out":        cout,
			"connections_onion":      0,
			"connections_unroutable": 0,
			"networks": defaultNetworksInfo(),
			"relayfee":       0.0,
			"minrelaytxfee":  0.0,
			"incrementalfee": 0.0,
			"localaddresses": []interface{}{},
			"warnings":            warnings,
			"dogego_status_note":  statusNote,
			"dogego_p2p_mode":     p2pMode,
		}
		if paths != nil && paths.P2PStats != nil {
			if snap := paths.P2PStats(); snap != nil {
				if v, ok := snap["listen_enabled"].(bool); ok {
					res["dogego_listen"] = v
				}
				if v, ok := snap["health"].(string); ok {
					res["dogego_p2p_health"] = v
				}
				if v, ok := snap["multi_peer_enabled"].(bool); ok {
					res["dogego_multi_peer"] = v
				}
				mergeDogegoAddrbookFromP2P(res, snap)
				if netActive {
					if v, ok := snap["localaddresses"].([]map[string]interface{}); ok && len(v) > 0 {
						res["localaddresses"] = v
					}
				}
				if v, ok := snap["networks"]; ok {
					res["networks"] = v
				}
				if netActive {
					if v, ok := snap["block_assist_connections"].(int); ok {
						res["dogego_block_assist_connections"] = v
					}
					if v, ok := snap["connections_total"].(int); ok {
						res["dogego_connections_with_assist"] = v
					}
					if v, ok := snap["max_outbound"].(int); ok && v > 0 {
						res["dogego_max_outbound"] = v
					}
				}
			}
		}
		relayKoinu := minRelayFeeFromPaths(paths)
		if relayKoinu == 0 {
			relayKoinu = consensus.MinRelayTxFeePerKB()
		}
		relayDOGE := float64(relayKoinu) / 1e8
		incDOGE := float64(consensus.IncrementalRelayFeePerKB()) / 1e8
		res["relayfee"] = relayDOGE
		res["minrelaytxfee"] = relayDOGE
		res["incrementalfee"] = incDOGE
		if paths != nil && paths.FeeFilter != nil {
			peerMax := paths.FeeFilter()
			res["dogego_peer_feefilter_koinuperkb"] = peerMax
			if peerMax > 0 && peerMax > relayKoinu {
				res["dogego_relayfee_note"] = "relayfee uses max(peer feefilter, mempool rolling floor, chain default)"
			}
		}
		setResult(res)
		return out
	case "getnetworkhashps":
		res, code, msg := execGetNetworkHashPS(j, raw, paths, networkFromChainName(chainName), params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getnewaddress":
		res, code, msg := execGetNewAddress(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getrawchangeaddress":
		res, code, msg := execGetRawChangeAddress(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getpeerinfo":
		setResult(execGetPeerInfo(paths))
		return out
	case "getaddednodeinfo":
		res, code, msg := execGetAddedNodeInfo(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getaddrmaninfo":
		res, code, msg := execGetAddrmanInfo(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getnodeaddresses":
		res, code, msg := execGetNodeAddresses(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getaddressesbyaccount":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetAddressesByAccountWallet(chainName, paths, params)
		} else {
			res, code, msg = execGetAddressesByAccount(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getaddressesbylabel":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetAddressesByLabelWallet(chainName, paths, params)
		} else {
			res, code, msg = execGetAddressesByLabel(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getauxblock":
		res, code, msg := execGetAuxBlock(j, pool, txIndex, raw, chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "addmultisigaddress":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execAddMultisigAddressWallet(chainName, paths, params)
		} else {
			res, code, msg = execAddMultisigAddress(chainName, params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "addnode":
		res, code, msg := execAddNode(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "addwitnessaddress":
		res, code, msg := execAddWitnessAddress(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "abandontransaction":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execAbandonTransactionWallet(chainName, paths, j, raw, pool, params)
		} else {
			res, code, msg = execAbandonTransaction(pool, params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "backupwallet":
		res, code, msg := execBackupWallet(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "bumpfee":
		res, code, msg := execBumpFee(chainName, pool, txIndex, raw, j, paths, params, relayTx, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "psbtbumpfee":
		res, code, msg := execPsbtBumpFee(chainName, paths, pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "simulaterawtransaction":
		res, code, msg := execSimulateRawTransaction(chainName, paths, pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listaccounts":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListAccountsWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execListAccounts(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listaddressgroupings":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListAddressGroupingsWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execListAddressGroupings(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listbanned":
		res, code, msg := execListBanned(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listdescriptors":
		res, code, msg := execListDescriptors(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setwalletflag":
		res, code, msg := execSetWalletFlag(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listlabels":
		res, code, msg := execListLabelsWallet(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listlockunspent":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListLockUnspentWallet(paths, params)
		} else {
			res, code, msg = execListLockUnspent(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listreceivedbyaccount":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListReceivedByAccountWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execListReceivedByAccount(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listreceivedbyaddress":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListReceivedByAddressWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execListReceivedByAddress(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listreceivedbylabel":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListReceivedByLabelWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execListReceivedByLabel(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "liststucktransactions":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListStuckTransactionsWallet(chainName, paths, j, raw, pool, txIndex, params)
		} else {
			res, code, msg = execListStuckTransactions(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listtransactions":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListTransactionsWallet(chainName, paths, j, raw, pool, txIndex, params)
		} else {
			res, code, msg = execListTransactions(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listsinceblock":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execListSinceBlockWallet(chainName, paths, j, raw, pool, txIndex, params)
		} else {
			res, code, msg = execListSinceBlock(j, raw, paths, params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "listunspent":
		res, code, msg := execListUnspent(chainName, paths, j, raw, txIndex, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "lockunspent":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execLockUnspentWallet(paths, params)
		} else {
			res, code, msg = execLockUnspent(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "move":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execMoveWallet(params)
		} else {
			res, code, msg = execMove(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "clearbanned":
		res, code, msg := execClearBanned(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getnettotals":
		res, code, msg := execGetNetTotals(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getrawtransaction":
		res, code, msg := execGetRawTransaction(txIndex, raw, j, pool, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getreceivedbyaccount":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetReceivedByAccountWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execGetReceivedByAccount(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getreceivedbyaddress":
		res, code, msg := execGetReceivedByAddress(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getreceivedbylabel":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetReceivedByLabelWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execGetReceivedByLabel(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "gettransaction":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execGetTransactionWallet(chainName, paths, j, raw, pool, txIndex, params)
		} else {
			res, code, msg = execGetTransaction(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "gettxout":
		var utxo *store.UtxoCache
		if paths != nil {
			utxo = paths.Utxo
		}
		res, code, msg := execGetTxOut(txIndex, raw, j, pool, utxo, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "gettxoutproof":
		res, code, msg := execGetTxOutProof(txIndex, raw, j, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getzmqnotifications":
		res, code, msg := execGetZMQNotifications(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "gettxoutsetinfo":
		var utxo *store.UtxoCache
		var syncUtxo func() error
		if paths != nil {
			utxo = paths.Utxo
			syncUtxo = paths.SyncUtxo
		}
		res, code, msg := execGetTxOutSetInfo(j, raw, utxo, paths, syncUtxo)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "scantxoutset":
		var utxo *store.UtxoCache
		var syncUtxo func() error
		if paths != nil {
			utxo = paths.Utxo
			syncUtxo = paths.SyncUtxo
		}
		res, code, msg := execScanTxOutSet(chainName, paths, j, raw, txIndex, utxo, syncUtxo, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "scanblocks":
		var filters *store.BlockFilterIndex
		if paths != nil {
			filters = paths.BlockFilterIndex
		}
		res, code, msg := execScanBlocks(chainName, j, raw, txIndex, filters, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getunconfirmedbalance":
		res, code, msg := execGetUnconfirmedBalance(chainName, paths, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getwalletinfo":
		res, code, msg := execGetWalletInfo(paths, j, raw, pool, txIndex, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "invalidateblock":
		res, code, msg := execInvalidateBlock(j, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importaddress":
		res, code, msg := execImportAddress(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getdescriptorinfo":
		res, code, msg := execGetDescriptorInfo(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "deriveaddresses":
		res, code, msg := execDeriveAddresses(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "extractdescriptor":
		res, code, msg := execExtractDescriptor(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importdescriptors":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execImportDescriptors(chainName, paths, j, raw, params)
		} else {
			res, code, msg = nil, -1, "importdescriptors: built-in wallet not enabled"
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importmulti":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execImportMultiWallet(chainName, paths, j, raw, params)
		} else {
			res, code, msg = execImportMulti(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importprivkey":
		res, code, msg := execImportPrivKey(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importprunedfunds":
		res, code, msg := execImportPrunedFunds(chainName, paths, j, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importpubkey":
		res, code, msg := execImportPubKey(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "importwallet":
		res, code, msg := execImportWallet(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "keypoolrefill":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execKeypoolRefillWallet(paths, params)
		} else {
			res, code, msg = execKeypoolRefill(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "preciousblock":
		res, code, msg := execPreciousBlock(j, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "reconsiderblock":
		res, code, msg := execReconsiderBlock(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "removeprunedfunds":
		res, code, msg := execRemovePrunedFunds(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "rescan":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execRescanWallet(paths, j, raw, params)
		} else {
			res, code, msg = execRescan(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "resendwallettransactions":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execResendWalletTransactionsWallet(chainName, paths, j, raw, pool, params, relayTx)
		} else {
			res, code, msg = execResendWalletTransactions(pool, params, relayTx)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "verifytxoutproof":
		res, code, msg := execVerifyTxOutProof(j, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "decoderawtransaction":
		res, code, msg := execDecodeRawTransaction(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "decodepsbt":
		res, code, msg := execDecodePsbt(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "analyzepsbt":
		res, code, msg := execAnalyzePsbt(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "combinepsbt":
		res, code, msg := execCombinePsbt(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "joinpsbts":
		res, code, msg := execJoinPsbt(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "finalizepsbt":
		res, code, msg := execFinalizePsbt(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "utxoupdatepsbt":
		res, code, msg := execUtxoUpdatePsbt(txIndex, raw, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "gettxspendingprevout":
		res, code, msg := execGetTxSpendingPrevout(pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "fundrawtransaction":
		res, code, msg := execFundRawTransaction(chainName, paths, j, raw, txIndex, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "decodescript":
		res, code, msg := execDecodeScript(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "disconnectnode":
		res, code, msg := execDisconnectNode(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_probewalletdat":
		res, code, msg := execProbeWalletDat(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_recoverheaders":
		res, code, msg := execDogegoRecoverHeaders(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_verifypqcommitment":
		res, code, msg := execDogegoVerifyPQCommitment(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_createpqcarrier":
		res, code, msg := execDogegoCreatePQCarrier(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_verifypqcarrier":
		res, code, msg := execDogegoVerifyPQCarrier(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_sendpqcarrier":
		res, code, msg := execDogegoSendPQCarrier(chainName, paths, j, raw, pool, txIndex, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_importmnemonic":
		res, code, msg := execDogegoImportMnemonic(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_importbip38":
		res, code, msg := execDogegoImportBIP38(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_importwalletdat":
		res, code, msg := execImportWalletDat(chainName, paths, j, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dogego_listwalletaddresses":
		res, code, msg := execDogegoListWalletAddresses(paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dumpprivkey":
		res, code, msg := execDumpPrivKey(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "dumpwallet":
		res, code, msg := execDumpWallet(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "echo", "echojson":
		res, code, msg := execEcho(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "encryptwallet":
		var res interface{}
		var code int
		var msg string
		if paths != nil && paths.WalletEncrypt != nil {
			res, code, msg = execEncryptWalletPaths(paths, params)
		} else if paths != nil && rpcWalletDefaultAddress(paths) != "" {
			res, code, msg = execEncryptWalletBuiltin(params)
		} else {
			res, code, msg = execEncryptWallet(params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "enumeratesigners":
		res, code, msg := execEnumerateSigners(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "createauxblock":
		res, code, msg := execCreateAuxBlock(j, pool, txIndex, raw, chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "createmultisig":
		res, code, msg := execCreateMultisig(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "createpsbt":
		res, code, msg := execCreatePsbt(chainName, txIndex, raw, pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "createrawtransaction":
		res, code, msg := execCreateRawTransaction(chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "converttopsbt":
		res, code, msg := execConvertToPsbt(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "combinerawtransaction":
		res, code, msg := execCombineRawTransaction(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblock":
		var auxGetBlock *store.HeaderAuxJournal
		if paths != nil {
			auxGetBlock = paths.HeaderAux
		}
		res, code, msg := execGetBlock(j, raw, auxGetBlock, chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblockfilter":
		var filters *store.BlockFilterIndex
		if paths != nil {
			filters = paths.BlockFilterIndex
		}
		res, code, msg := execGetBlockFilter(j, raw, txIndex, filters, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblockfilterheader":
		var filters *store.BlockFilterIndex
		if paths != nil {
			filters = paths.BlockFilterIndex
		}
		res, code, msg := execGetBlockFilterHeader(j, raw, txIndex, filters, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblockstats":
		var utxo *store.UtxoCache
		if paths != nil {
			utxo = paths.Utxo
		}
		res, code, msg := execGetBlockStats(j, raw, txIndex, utxo, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblockheader":
		verbose := true
		if len(params) > 1 {
			if err := json.Unmarshal(params[1], &verbose); err != nil {
				setErr(-8, "getblockheader: bad verbose flag")
				return out
			}
		}
		h80, height, err := resolveGetBlockHeader(j, raw, params, paths)
		if err != nil {
			setErr(-8, err.Error())
			return out
		}
		if !verbose {
			setResult(hex.EncodeToString(h80))
			return out
		}
		cw := "0"
		if s, err := cumulativeChainworkHex(j, height); err == nil {
			cw = s
		}
		var auxJ *store.HeaderAuxJournal
		if paths != nil {
			auxJ = paths.HeaderAux
		}
		chainTip := ActiveChainBlockHeight(j, raw, paths)
		setResult(blockHeaderJSON(j, h80, height, cw, auxJ, chainTip))
		return out
	case "getrawmempool":
		res, code, msg := execGetRawMempool(pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmempoolancestors":
		res, code, msg := execGetMempoolAncestors(pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmempooldescendants":
		res, code, msg := execGetMempoolDescendants(pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmempoolentry":
		res, code, msg := execGetMempoolEntry(pool, txIndex, raw, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmempoolinfo":
		orphans := 0
		maxMem := 0
		maxOrphan := 0
		rollingMin := uint64(0)
		if paths != nil {
			if paths.OrphanCount != nil {
				orphans = paths.OrphanCount()
			}
			maxMem = paths.MaxMempoolEntries
			maxOrphan = paths.MaxOrphanEntries
			if paths.MempoolMinRelayFee != nil {
				rollingMin = paths.MempoolMinRelayFee()
			}
		}
		res, code, msg := execGetMempoolInfo(pool, txIndex, raw, minRelayFeeFromPaths(paths), orphans, maxOrphan, maxMem, fullRBFFromPaths(paths), rollingMin, paths)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmemoryinfo":
		setResult(execGetMemoryInfo())
		return out
	case "getmininginfo":
		blockMaxWeight := 0
		if paths != nil {
			blockMaxWeight = paths.BlockMaxWeight
		}
		res, code, msg := execGetMiningInfo(j, pool, txIndex, raw, chainName, paths, blockMaxWeight)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getmocktime":
		res, code, msg := execGetMockTime(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "getblocktemplate":
		blockMaxWeight := 0
		if paths != nil {
			blockMaxWeight = paths.BlockMaxWeight
		}
		res, code, msg := execGetBlockTemplate(j, pool, txIndex, raw, paths, chainName, blockMaxWeight, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "generate":
		var auxJ *store.HeaderAuxJournal
		if paths != nil {
			auxJ = paths.HeaderAux
		}
		res, code, msg := execGenerate(j, auxJ, paths, raw, pool, txIndex, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "generatetoaddress":
		var auxJ *store.HeaderAuxJournal
		if paths != nil {
			auxJ = paths.HeaderAux
		}
		res, code, msg := execGenerateToAddress(j, auxJ, paths, raw, pool, txIndex, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "prioritisetransaction":
		res, code, msg := execPrioritiseTransaction(pool, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "pruneblockchain":
		res, code, msg := execPruneBlockchain(j, raw, txIndex, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "sendfrom":
		res, code, msg := execSendFrom(chainName, paths, j, pool, txIndex, raw, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "sendmany":
		res, code, msg := execSendMany(chainName, paths, j, pool, txIndex, raw, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "sendtoaddress":
		res, code, msg := execSendToAddress(chainName, paths, j, pool, txIndex, raw, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "sendrawtransaction":
		res, code, msg := execSendRawTransaction(pool, txIndex, raw, j, paths, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		applog.Line("mempool", fmt.Sprintf("RPC sendrawtransaction accepted (txid %v)", res))
		setResult(res)
		return out
	case "submitpackage":
		res, code, msg := execSubmitPackage(pool, txIndex, raw, j, paths, params, relayTx, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setaccount":
		var res interface{}
		var code int
		var msg string
		if WalletActive(paths) {
			res, code, msg = execSetAccountWallet(chainName, paths, params)
		} else {
			res, code, msg = execSetAccount(chainName, params)
		}
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setlabel":
		res, code, msg := execSetLabelWallet(chainName, paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setban":
		res, code, msg := execSetBan(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setmaxconnections":
		res, code, msg := execSetMaxConnections(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setmocktime":
		res, code, msg := execSetMockTime(params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "setnetworkactive":
		res, code, msg := execSetNetworkActive(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "settxfee":
		res, code, msg := execSetTxFee(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "testmempoolaccept":
		res, code, msg := execTestMempoolAccept(pool, txIndex, raw, j, paths, params, allowUnverifiedMempool, networkFromChainName(chainName))
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "truncatetoheight":
		res, code, msg := execTruncateToHeight(paths, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "submitauxblock":
		res, code, msg := execSubmitAuxBlock(j, paths, raw, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "submitblock":
		var auxJ *store.HeaderAuxJournal
		if paths != nil {
			auxJ = paths.HeaderAux
		}
		res, code, msg := execSubmitBlock(j, auxJ, raw, paths, chainName, params)
		if code != 0 {
			setErr(code, msg)
			return out
		}
		setResult(res)
		return out
	case "stop":
		// Core accepts one optional deprecated "detach" argument (ignored).
		if len(params) > 1 {
			setErr(-32602, "Too many arguments")
			return out
		}
		if paths == nil || paths.Shutdown == nil {
			setErr(-1, "stop: graceful shutdown is not available for this RPC session")
			return out
		}
		applog.Line("rpc", "JSON-RPC stop requested - initiating graceful shutdown")
		sh := paths.Shutdown
		go func() {
			time.Sleep(80 * time.Millisecond)
			sh()
		}()
		setResult("DogeGo stopping.")
		return out
	default:
		if isExtensionMethod(method) {
			res, code, msg := execExtensionsRPC(paths, method, params)
			if code != 0 {
				setErr(code, msg)
				return out
			}
			setResult(res)
			return out
		}
		setErr(-32601, "Method not implemented in DogeGo yet")
		return out
	}
}
