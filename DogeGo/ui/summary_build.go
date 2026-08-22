// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"strings"
	"time"

	"dogego/consensus"
	"dogego/diskspace"
	"dogego/netfw"
	"dogego/rpc"
)

var p2pSummaryExtraFieldKeys = []string{
	"upnp_mapped", "upnp_external", "upnp_method", "listen_enabled",
	"bip152_hb_to", "bip152_hb_from", "bip152_hb_max",
	"dogego_cmpct_in", "dogego_cmpct_mempool_hit", "dogego_cmpct_getblocktxn_out",
	"dogego_cmpct_blocktxn_in", "dogego_cmpct_reconstruct_ok", "dogego_cmpct_reconstruct_fail",
	"dogego_cmpct_announced_out", "dogego_cmpct_served_getdata", "dogego_cmpct_fallback_full_block",
	"dogego_cmpct_blocktxn_served", "dogego_cmpct_reconstruct_fallback_getdata",
	"addrbook_tried", "addrbook_new", "addrbook_n_key_set", "addrbook_bucket_slot_cap",
	"addrbook_tried_bucket_max_fill", "addrbook_new_bucket_max_fill",
	"addrbook_tried_buckets_used", "addrbook_new_buckets_used",
}

func mergeP2PSummaryExtraFields(summary map[string]any, p2pSnap map[string]any) {
	if summary == nil || p2pSnap == nil {
		return
	}
	for _, k := range p2pSummaryExtraFieldKeys {
		if v, ok := p2pSnap[k]; ok {
			summary[k] = v
		}
	}
}

// BuildSummaryMap assembles /api/summary JSON fields (may block; use LiveFeed for HTTP handlers).
func BuildSummaryMap(cfg StartConfig) (map[string]any, error) {
	if cfg.Journal == nil {
		return nil, fmt.Errorf("no journal")
	}
	// Header sync appends on dedicated/background goroutines; always derive tip from headers.bin size.
	tip, cnt, err1 := journalTipForDashboard(cfg.Journal)
	if err1 != nil {
		return nil, fmt.Errorf("journal read: %w", err1)
	}
	best, err := journalBestHashForDashboard(cfg.Journal)
	if err != nil {
		return nil, fmt.Errorf("journal tip hash: %w", err)
	}
	gen := strings.TrimSpace(cfg.GenesisHash)
	if gen == "" {
		gen, err = cfg.Journal.GenesisHashHex()
		if err != nil {
			return nil, err
		}
	}
	mp := 0
	if cfg.Pool != nil {
		mp = cfg.Pool.Count()
	}
	peer := ""
	if cfg.PeerLabel != nil {
		peer = *cfg.PeerLabel
	}
	rb := 0
	contiguousH := contiguousHeightForAPI(cfg)
	if cfg.RawBlocks != nil {
		if n, err := cfg.RawBlocks.FastCount(); err == nil {
			rb = n
		}
		rb = maybeReconcileRawBlockCount(cfg.RawBlocks, contiguousH, rb)
	}
	verProg := 1.0
	if cfg.RawBlocks != nil && tip >= 0 {
		want := int(tip) + 1
		have := int(contiguousH) + 1
		if contiguousH < 0 {
			have = 0
		}
		if want > 0 && have < want {
			verProg = float64(have) / float64(want)
		}
	}
	outbound, inbound := 0, 0
	p2pMode := strings.TrimSpace(cfg.EffectiveFile.P2PConnectivity)
	if p2pMode == "" {
		p2pMode = "both"
	}
	p2pHealth, p2pHealthMsg := "", ""
	var p2pSnap map[string]any
	if cfg.P2PSnapshot != nil {
		p2pSnap = p2PSnapshotWithTimeout(cfg.P2PSnapshot)
	}
	if p2pSnap != nil {
		if v, ok := p2pSnap["connections_outbound"].(int); ok {
			outbound = v
		}
		if v, ok := p2pSnap["connections_inbound"].(int); ok {
			inbound = v
		}
		if v, ok := p2pSnap["p2p_connectivity"].(string); ok && v != "" {
			p2pMode = v
		}
		if v, ok := p2pSnap["health"].(string); ok {
			p2pHealth = v
		}
		if v, ok := p2pSnap["health_message"].(string); ok {
			p2pHealthMsg = v
		}
	}
	inboundHint := ""
	if p2pSnap != nil {
		if v, ok := p2pSnap["inbound_hint"].(string); ok {
			inboundHint = v
		}
	}
	if outbound == 0 && cfg.PeerLabel != nil {
		pv := strings.TrimSpace(*cfg.PeerLabel)
		if pv != "" && !strings.HasPrefix(pv, "(") && !strings.Contains(pv, "handshaking") {
			outbound = 1
		}
	}
	ibdActive := false
	var coreIBD rpc.ChainIBDSnapshot
	if p2pSnap != nil {
		if v, ok := p2pSnap["initialblockdownload"].(bool); ok {
			ibdActive = v
		}
		if v, ok := p2pSnap["verification_progress"].(float64); ok {
			verProg = v
		}
	}
	if cfg.ChainIBDSync != nil && p2pSnap == nil {
		coreIBD = cfg.ChainIBDSync()
		ibdActive = coreIBD.IBD
		verProg = coreIBD.VerificationProgress
	}
	var headerDiag map[string]interface{}
	if cfg.HeaderSyncDiag != nil {
		headerDiag = cfg.HeaderSyncDiag()
	}
	var blocksBehind int64
	var blocksPerMin float64
	var summaryLastStored int64
	var summaryLowestMissing int64 = -1
	var summaryGenesisMissing bool
	var inFlight, syncWorkers, assistPeerPool, discoveryFeedSize int
	var headerCatchUpPending bool
	var blockAssistActive bool
	var peerStartHeight int64
	var ibdProg map[string]interface{}
	if p2pSnap != nil {
		if v, ok := p2pSnap["peer_start_height"].(int32); ok && v > 0 {
			peerStartHeight = int64(v)
		} else if v, ok := p2pSnap["peer_start_height"].(int64); ok && v > 0 {
			peerStartHeight = v
		} else if v, ok := p2pSnap["peer_start_height"].(float64); ok && v > 0 {
			peerStartHeight = int64(v)
		}
		if v, ok := p2pSnap["header_catch_up_pending"].(bool); ok {
			headerCatchUpPending = v
		}
		if v, ok := p2pSnap["block_assist_active"].(bool); ok {
			blockAssistActive = v
		}
		if prog, ok := p2pSnap["ibd_progress"].(map[string]interface{}); ok {
			ibdProg = prog
			if v, ok := prog["lowest_missing_height"].(int64); ok {
				summaryLowestMissing = v
			} else if v, ok := prog["lowest_missing_height"].(float64); ok {
				summaryLowestMissing = int64(v)
			}
			if v, ok := prog["genesis_missing"].(bool); ok && v {
				summaryGenesisMissing = true
			}
			if cfg.ChainIBDSync == nil {
				if v, ok := prog["idle_full"].(bool); ok {
					ibdActive = !v
				}
			}
			if v, ok := prog["blocks_per_minute"].(float64); ok {
				blocksPerMin = v
			}
			if v, ok := prog["last_block_stored_at"].(int64); ok {
				summaryLastStored = v
			} else if v, ok := prog["last_block_stored_at"].(float64); ok {
				summaryLastStored = int64(v)
			}
			if v, ok := prog["in_flight_batches"].(int); ok {
				inFlight = v
			} else if v, ok := prog["in_flight_batches"].(float64); ok {
				inFlight = int(v)
			}
			if v, ok := prog["sync_workers"].(int); ok {
				syncWorkers = v
			} else if v, ok := prog["sync_workers"].(float64); ok {
				syncWorkers = int(v)
			}
			if v, ok := prog["assist_peer_pool"].(int); ok {
				assistPeerPool = v
			} else if v, ok := prog["assist_peer_pool"].(float64); ok {
				assistPeerPool = int(v)
			}
			if v, ok := prog["discovery_feed_size"].(int); ok {
				discoveryFeedSize = v
			} else if v, ok := prog["discovery_feed_size"].(float64); ok {
				discoveryFeedSize = int(v)
			}
		}
		if v, ok := p2pSnap["chain_active_height"].(int64); ok && tip >= 0 {
			blocksBehind = tip - v
		} else if v, ok := p2pSnap["chain_active_height"].(float64); ok && tip >= 0 {
			blocksBehind = tip - int64(v)
		} else if v, ok := p2pSnap["contiguous_block_height"].(int64); ok && tip >= 0 {
			blocksBehind = tip - v
		} else if v, ok := p2pSnap["contiguous_block_height"].(float64); ok && tip >= 0 {
			blocksBehind = tip - int64(v)
		}
	}
	if blocksBehind < 0 {
		blocksBehind = 0
	}
	chainActive := chainActiveHeightForAPI(cfg, tip)
	if tip >= 0 && chainActive >= 0 && tip > chainActive {
		blocksBehind = tip - chainActive
	}
	if tip >= 0 && chainActive >= 0 && tip > chainActive && cfg.ChainIBDSync == nil && verProg < 1 {
		ibdActive = true
	}
	if bodyBehind := rpc.BlocksBehindHeaders(tip, chainActive, contiguousH); bodyBehind > blocksBehind {
		blocksBehind = bodyBehind
	}
	nm := strings.TrimSpace(cfg.NodeMode)
	if nm == "" {
		nm = "full"
	}
	orphanRaw := 0
	if rb > 0 && contiguousH >= 0 {
		orphanRaw = rb - int(contiguousH+1)
		if orphanRaw < 0 {
			orphanRaw = 0
		}
	} else if rb > 0 && contiguousH < 0 {
		orphanRaw = rb
	}
	syncPhase := rpc.DogeGoSyncPhase(nm, tip, chainActive, contiguousH, summaryGenesisMissing)
	relayNote := p2pHealthMsg
	if relayNote == "" {
		relayNote = "DogeGo: single outbound peer; headers-first; full blocks on disk when in full-node mode"
		if nm == "spv" {
			relayNote = "DogeGo SPV: headers-only - no full block payloads on this run"
		}
	}
	ef := cfg.EffectiveFile
	var chainWarnings []string
	if !ibdActive {
		if net, err := networkFromUISlug(cfg.Network); err == nil {
			chainWarnings = consensus.ChainWarnings(cfg.Journal, net)
		}
	}
	ibdFlag := ibdActive
	if p2pSnap != nil {
		if v, ok := p2pSnap["initialblockdownload"].(bool); ok {
			ibdFlag = v
		}
	} else if cfg.ChainIBDSync != nil {
		ibdFlag = coreIBD.IBD
	}
	summary := map[string]any{
		"chain":                      cfg.ChainDisplay,
		"network":                    cfg.Network,
		"node_mode":                  nm,
		"peer":                       peer,
		"tip_height":                 tip,
		"chain_active_height":        chainActive,
		"best_hash":                  best,
		"genesis_hash":               gen,
		"header_count":               cnt,
		"raw_blocks":                 rb,
		"contiguous_raw_height":      contiguousH,
		"orphan_raw_blocks_estimate": orphanRaw,
		"sync_phase":                 syncPhase,
		"verification_progress":      verProg,
		"sync_pct":                   verProg * 100,
		"mempool_txs":                mp,
		"rpc_addr":                   cfg.RPCAddr,
		"mine_requested":             cfg.MineRequested,
		"mining_active":              cfg.MiningActive != nil && cfg.MiningActive.Load(),
		"wallet_enabled":             walletLoaded(cfg),
		"wallet_rpc_ready":           walletRPCReady(cfg),
		"wallet_address_ready":       walletAddressReady(cfg),
		"wallet_address":             walletAddr(cfg.Wallet),
	}
	if walletLoaded(cfg) {
		if confirmed, immature, utxos, ok := walletBalanceFromUtxoCache(cfg); ok {
			summary["wallet_balance"] = confirmed
			summary["wallet_immature_balance"] = immature
			summary["wallet_utxo_count"] = utxos
		}
		attachWalletEncryptionStatus(summary, cfg.Wallet)
		attachWalletRescanStatus(summary, cfg)
		attachWalletHistoryDeferStatus(summary, cfg)
	}
	summary["base_data_dir"] = cfg.BaseDataDir
	if peers := instancesForAPI(cfg.BaseDataDir, cfg.Network); len(peers) > 0 {
		summary["peer_instances"] = peers
	}
	summary["chain_data_dir"] = cfg.ChainDataDir
	summary["connections_in"] = inbound
	summary["connections_out"] = outbound
	summary["p2p_connectivity"] = p2pMode
	summary["p2p_health"] = p2pHealth
	summary["relay_note"] = relayNote
	summary["inbound_hint"] = inboundHint
	summary["embedded_analytics_sidecar"] = analyticsSidecarLive(cfg)
	summary["dogego_ibd_optimize"] = ef.IBDOptimizeEnabled()
	if ef.DBCacheMB > 0 {
		summary["dogego_dbcache_mb"] = ef.DBCacheMB
	} else {
		summary["dogego_dbcache_mb"] = 0 // 0 = auto from free RAM at node start
	}
	ibdFocus := ef.IBDOptimizeEnabled() && (ibdActive || ibdFlag)
	summary["dogego_ibd_focus"] = ibdFocus
	if ibdFocus && ef.EmbeddedAnalyticsEnabled() && !analyticsSidecarLive(cfg) {
		summary["dogego_analytics_deferred_ibd"] = true
	}
	summary["ibd_active"] = ibdActive
	summary["initialblockdownload"] = ibdFlag
	summary["blocks_behind_headers"] = blocksBehind
	summary["blocks_per_minute"] = blocksPerMin
	summary["in_flight_batches"] = inFlight
	summary["sync_workers"] = syncWorkers
	summary["assist_peer_pool"] = assistPeerPool
	summary["discovery_feed_size"] = discoveryFeedSize
	summary["relay_policy"] = RelayPolicyForAPI(ef, cfg.Pool)
	summary["chain_warnings"] = chainWarnings
	eta := rpc.FormatSyncETA(blocksBehind, blocksPerMin)
	bodyPct := rpc.BodyVerificationProgress(tip, contiguousH)
	connPct := rpc.ConnectedVerificationProgress(tip, chainActive)
	displayPct := rpc.EffectiveIBDDisplayProgress(tip, contiguousH, peerStartHeight, ibdFlag)
	if ibdFlag {
		verProg = displayPct
		summary["verification_progress"] = verProg
		summary["sync_pct"] = verProg * 100
	}
	if peerStartHeight > 0 {
		summary["peer_start_height"] = peerStartHeight
		summary["dogego_header_ibd_progress"] = rpc.HeaderIBDProgress(tip, peerStartHeight)
	}
	summary["sync_eta"] = eta
	summary["dogego_body_verification_progress"] = bodyPct
	summary["dogego_connected_verification_progress"] = connPct
	if tip > chainActive && chainActive >= 0 {
		summary["dogego_headers_sync_progress"] = rpc.HeadersSyncProgress(tip, chainActive)
	}
	if contiguousH > chainActive && chainActive >= 0 {
		summary["dogego_stored_bodies_ahead_connect"] = contiguousH - chainActive
	}
	syncLine := rpc.SyncStatusLine(nm, syncPhase, tip, contiguousH, bodyPct, blocksBehind, eta, mp)
	bodyIBDHeaderPaused := bodyIBDHeaderPausedForSummary(tip, contiguousH, headerDiag, ibdProg)
	if ibdFlag && peerStartHeight > int64(tip) && bodyPct < 0.01 && !bodyIBDHeaderPaused {
		hdrPct := int(displayPct * 100)
		if hdrPct > 100 {
			hdrPct = 100
		}
		syncLine = fmt.Sprintf("Synchronizing headers… %d%% (height %s / ~%s)", hdrPct, fmt.Sprintf("%d", tip), fmt.Sprintf("%d", peerStartHeight))
	} else if bodyIBDHeaderPaused && ibdFlag && bodyPct < 0.999 {
		bodyPctDisp := int(bodyPct * 100)
		if bodyPctDisp < 1 && bodyPct > 0 {
			bodyPctDisp = 1
		}
		syncLine = fmt.Sprintf("Downloading block bodies… %d%% (connected %s / headers %s)", bodyPctDisp, fmt.Sprintf("%d", chainActive), fmt.Sprintf("%d", tip))
	}
	summary["sync_status_line"] = syncLine
	effectiveHeaderCatchUp := headerCatchUpPending && !bodyIBDHeaderPaused
	summary["headers_syncing"] = nm != "spv" && tip >= 0 && effectiveHeaderCatchUp
	summary["dogego_header_catch_up_pending"] = effectiveHeaderCatchUp
	summary["dogego_block_assist_active"] = blockAssistActive
	headerRecovery := ""
	if v, ok := headerDiag["dogego_header_sync_recovery"].(string); ok {
		headerRecovery = v
	}
	health, syncOK := rpc.SyncHealthAssessment(syncPhase, tip, chainActive, blocksBehind, blocksPerMin, summaryLastStored, headerRecovery, effectiveHeaderCatchUp)
	summary["dogego_sync_health"] = health
	summary["dogego_sync_ok"] = syncOK
	summary["dogego_sync_phase"] = syncPhase
	summary["dogego_sync_status"] = summary["sync_status_line"]
	if summaryLowestMissing >= 0 {
		summary["lowest_missing_height"] = summaryLowestMissing
	}
	if summaryGenesisMissing {
		summary["dogego_genesis_missing"] = true
		summary["dogego_genesis_note"] = "genesis block (height 0) is not in rawblocks/ yet; forward sync and getdata include height 0 when headers exist"
	}
	mergeIBDProgressDiagnostics(summary, ibdProg)
	if !ibdActive {
		if tv, ok := rpc.TxVerificationProgress(rpcChainFromUISlug(cfg.Network), cfg.Journal, cfg.TxIndex, chainActive); ok {
			summary["dogego_tx_verification_progress"] = tv
		}
	}
	if chainActive >= 0 {
		txProcessed := chainActive + 1
		if cfg.TxIndex != nil {
			if n, _, err := cfg.TxIndex.CachedStats(60 * time.Second); err == nil && n > 0 {
				txProcessed = int64(n)
			}
		}
		summary["transactions_processed"] = txProcessed
	}
	if cfg.StorageSummary != nil {
		mergeStorageSummary(summary, cfg.StorageSummary())
	}
	if headerDiag != nil {
		for k, v := range headerDiag {
			summary[k] = v
		}
	}
	fa := netfw.UserAlertSnapshot()
	summary["dogego_firewall_alert"] = fa
	if fa.Active {
		summary["dogego_firewall_warning"] = fa.Message
	}
	dp := diskspace.Current()
	summary["dogego_disk_pressure"] = dp
	if dp.Active {
		summary["dogego_disk_pressure_warning"] = dp.Message
	}
	if p2pSnap != nil {
		if v, ok := p2pSnap["primary_peer"].(string); ok {
			if v = strings.TrimSpace(v); v != "" {
				summary["primary_peer"] = v
			}
		}
		if act, ok := p2pSnap["dogego_sync_activity"].(map[string]any); ok && act != nil {
			act = cloneStringAnyMap(act)
			patchSyncActivityForHeaderTip(act, tip, peerStartHeight, contiguousH, chainActive, blocksBehind, blocksPerMin, summaryLowestMissing, inFlight, bodyIBDHeaderPaused)
			summary["dogego_sync_activity"] = act
		} else if act, ok := p2pSnap["dogego_sync_activity"]; ok && act != nil {
			summary["dogego_sync_activity"] = act
		}
		mergeP2PSummaryExtraFields(summary, p2pSnap)
	}
	applyBodyIBDPauseOperatorFields(summary, tip, chainActive, contiguousH, peerStartHeight, ibdFlag, blocksPerMin, blocksBehind, syncPhase, summaryLastStored, headerRecovery)
	zmqOn := strings.TrimSpace(cfg.EffectiveFile.ZmqPubHashBlock) != "" ||
		strings.TrimSpace(cfg.EffectiveFile.ZmqPubHashTx) != "" ||
		strings.TrimSpace(cfg.EffectiveFile.ZmqPubRawBlock) != "" ||
		strings.TrimSpace(cfg.EffectiveFile.ZmqPubRawTx) != ""
	summary["zmq_enabled"] = zmqOn
	AnnotateRestartResumeSummary(summary, cfg.ChainDataDir, tip, contiguousH, ibdFlag, assistPeerPool)
	AnnotateCoreParitySummary(summary, cfg.Network, probeConfigFromStart(cfg))
	AnnotateCoreOperatorCertSummary(summary)
	EnrichRPCSummaryFields(summary, cfg.RPCAddr, cfg.RPCSnapshot)
	if cfg.Network == "mainnet" {
		summary["dogego_auxpow_parent_chain_id_core_parity"] = true
	}
	mergeVersionFields(summary)
	mergeUpdateFields(summary, cfg)
	ApplyUILoadingFlags(summary, false)
	return summary, nil
}

func mergeIBDProgressDiagnostics(summary map[string]any, prog map[string]interface{}) {
	if summary == nil || prog == nil {
		return
	}
	if v, ok := prog["block_stalling_timeout_sec"].(int64); ok {
		summary["dogego_block_stalling_timeout_sec"] = v
	}
	if v, ok := prog["block_download_timeout_sec"].(int64); ok {
		summary["dogego_block_download_timeout_sec"] = v
	}
	if v, ok := prog["last_block_stall_peer"].(string); ok && v != "" {
		summary["dogego_last_block_stall_peer"] = v
		if t, ok := prog["last_block_stall_at"].(int64); ok && t > 0 {
			summary["dogego_last_block_stall_at"] = t
		}
	}
	if v, ok := prog["last_block_download_timeout_peer"].(string); ok && v != "" {
		summary["dogego_last_block_download_timeout_peer"] = v
		if t, ok := prog["last_block_download_timeout_at"].(int64); ok && t > 0 {
			summary["dogego_last_block_download_timeout_at"] = t
		}
	}
	if v, ok := prog["frontier_stalling_since"].(int64); ok && v > 0 {
		summary["dogego_frontier_stalling_since"] = v
	}
	if v, ok := prog["max_blocks_in_transit_per_peer"].(int); ok && v > 0 {
		summary["dogego_max_blocks_in_transit_per_peer"] = v
	}
	if v, ok := prog["lane_in_flight"].(map[string]int); ok && len(v) > 0 {
		summary["dogego_lane_in_flight"] = v
	}
	if v, ok := prog["connect_lag"].(int64); ok && v > 0 {
		summary["dogego_connect_lag"] = v
	} else if v, ok := prog["connect_lag"].(float64); ok && v > 0 {
		summary["dogego_connect_lag"] = int64(v)
	}
	if v, ok := prog["dbcache_mb"].(int); ok && v > 0 {
		summary["dogego_dbcache_mb"] = v
	} else if v, ok := prog["dbcache_mb"].(float64); ok && v > 0 {
		summary["dogego_dbcache_mb"] = int(v)
	}
	if v, ok := prog["blocks_per_minute_lifetime"].(float64); ok && v > 0 {
		summary["dogego_blocks_per_minute_lifetime"] = v
	}
	if v, ok := prog["connect_blocks_per_minute"].(float64); ok && v > 0 {
		summary["dogego_connect_blocks_per_minute"] = v
	}
	if v, ok := prog["contiguous_raw_height"].(int64); ok && v >= 0 {
		summary["dogego_raw_sync_contiguous"] = v
	}
	if v, ok := prog["body_ibd_header_paused"].(bool); ok && v {
		summary["dogego_body_ibd_header_paused"] = true
	}
	if v, ok := prog["connect_deferred_for_download"].(bool); ok && v {
		summary["dogego_connect_deferred_for_download"] = true
	}
	if v, ok := prog["body_ibd_eta_minutes"].(int64); ok && v > 0 {
		summary["dogego_body_ibd_eta_minutes"] = v
	} else if v, ok := prog["body_ibd_eta_minutes"].(float64); ok && v > 0 {
		summary["dogego_body_ibd_eta_minutes"] = int64(v)
	}
	if v, ok := prog["connect_catch_up_min_lag"].(int64); ok && v > 0 {
		summary["dogego_connect_catch_up_min_lag"] = v
	} else if v, ok := prog["connect_catch_up_min_lag"].(float64); ok && v > 0 {
		summary["dogego_connect_catch_up_min_lag"] = int64(v)
	}
	if v, ok := prog["connect_catch_up_passes"].(int); ok && v > 0 {
		summary["dogego_connect_catch_up_passes"] = v
	} else if v, ok := prog["connect_catch_up_passes"].(int64); ok && v > 0 {
		summary["dogego_connect_catch_up_passes"] = int(v)
	}
	if v, ok := prog["connect_catch_up_batch"].(int); ok && v > 0 {
		summary["dogego_connect_catch_up_batch"] = v
	} else if v, ok := prog["connect_catch_up_batch"].(int64); ok && v > 0 {
		summary["dogego_connect_catch_up_batch"] = int(v)
	}
	if v, ok := prog["connect_catch_up_interval_ms"].(int64); ok && v > 0 {
		summary["dogego_connect_catch_up_interval_ms"] = v
	}
}

func bodyIBDHeaderPausedForSummary(tip, contiguousH int64, headerDiag map[string]interface{}, ibdProg map[string]interface{}) bool {
	if ibdProg != nil {
		if v, ok := ibdProg["body_ibd_header_paused"].(bool); ok && v {
			return true
		}
	}
	if headerDiag != nil {
		if v, ok := headerDiag["dogego_body_ibd_header_paused"].(bool); ok && v {
			return true
		}
	}
	return rpc.BodyIBDOwnsPipeline(tip, contiguousH)
}

// applyBodyIBDPauseOperatorFields re-applies sync %, status line, and header flags after ibd_prog/header_diag merges.
func applyBodyIBDPauseOperatorFields(summary map[string]any, tip, chainActive, contiguousH, peerStartHeight int64, ibdFlag bool, blocksPerMin float64, blocksBehind int64, syncPhase string, summaryLastStored int64, headerRecovery string) {
	if summary == nil {
		return
	}
	paused := false
	if v, ok := summary["dogego_body_ibd_header_paused"].(bool); ok && v {
		paused = true
	}
	if !paused {
		paused = rpc.BodyIBDOwnsPipeline(tip, contiguousH)
	}
	if !paused {
		return
	}
	summary["dogego_body_ibd_header_paused"] = true
	summary["dogego_header_catch_up_pending"] = false
	summary["headers_syncing"] = false

	bodyPct := rpc.BodyVerificationProgress(tip, contiguousH)
	summary["dogego_body_verification_progress"] = bodyPct
	if ibdFlag {
		summary["verification_progress"] = bodyPct
		summary["sync_pct"] = bodyPct * 100
	}
	if ibdFlag && bodyPct < 0.999 {
		bodyPctDisp := int(bodyPct * 100)
		if bodyPctDisp < 1 && bodyPct > 0 {
			bodyPctDisp = 1
		}
		line := fmt.Sprintf("Downloading block bodies… %d%% (connected %s / headers %s)", bodyPctDisp, fmt.Sprintf("%d", chainActive), fmt.Sprintf("%d", tip))
		summary["sync_status_line"] = line
		summary["dogego_sync_status"] = line
	}
	if act, ok := summary["dogego_sync_activity"].(map[string]any); ok && act != nil {
		lowestMissing := int64(-1)
		if v, ok := summary["lowest_missing_height"].(int64); ok {
			lowestMissing = v
		} else if v, ok := summary["lowest_missing_height"].(float64); ok {
			lowestMissing = int64(v)
		}
		inFlight := 0
		if v, ok := summary["in_flight_batches"].(int); ok {
			inFlight = v
		} else if v, ok := summary["in_flight_batches"].(float64); ok {
			inFlight = int(v)
		}
		act = cloneStringAnyMap(act)
		patchSyncActivityForHeaderTip(act, tip, peerStartHeight, contiguousH, chainActive, blocksBehind, blocksPerMin, lowestMissing, inFlight, true)
		summary["dogego_sync_activity"] = act
	}
	health, syncOK := rpc.SyncHealthAssessment(syncPhase, tip, chainActive, blocksBehind, blocksPerMin, summaryLastStored, headerRecovery, false)
	summary["dogego_sync_health"] = health
	summary["dogego_sync_ok"] = syncOK
	ApplyUILoadingFlags(summary, false)
}
