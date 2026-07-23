// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// CoreParityGap is one unchecked Core-parity item from ROADMAP.md / STANDALONE_FULLNODE_ACCEPTANCE.md.
type CoreParityGap struct {
	ID      string `json:"id"`
	Area    string `json:"area"`
	Title   string `json:"title"`
	Status  string `json:"status"` // open, partial, declined
	Effort  string `json:"effort"` // S, M, L, XL
	Summary string `json:"summary"`
	Ref     string `json:"ref,omitempty"`
}

// DefaultCoreParityGaps lists what DogeGo still lacks vs Dogecoin Core (src/).
// Sync with docs/CORE_PARITY_GAPS.md and ROADMAP.md unchecked [ ] lines.
func DefaultCoreParityGaps() []CoreParityGap {
	return []CoreParityGap{
		{ID: "protocol_lock", Area: "Consensus", Title: "Mainnet protocol lock (no consensus forks)", Status: "done", Effort: "S",
			Summary: "Mainnet block/header/script/subsidy/auxpow rules follow Dogecoin Core; reorg in code means chain tip competition, not a protocol fork. Solo: GET /api/core-compare deployment.protocol_lock (getdeploymentinfo + getblockchaininfo softforks vs consensus params). Side-by-side: deployments + softforks vs Core when core_rpc_addr is set.",
			Ref:     "ROADMAP Dogecoin protocol lock"},

		// Standalone acceptance gate (milestones)
		{ID: "milestone_a_full", Area: "Certification", Title: "Consensus differential harness (full)", Status: "done", Effort: "M",
			Summary: "Mainnet field header PoW + block hex connect (heights 1-3); side-by-side via core_compare_with_core.ps1 (Core :22555, DogeGo :22557).",
			Ref:     "ROADMAP Milestone A"},
		{ID: "milestone_b_full", Area: "Certification", Title: "Crash / corruption suite (full)", Status: "done", Effort: "L",
			Summary: "Offline cert + subprocess kill green via dogego cert milestones-bde; cross-platform dogego cert live-soak; multi-hour scheduled soak needs green dogego-live runs (DOGEGO_SCHEDULED_LIVE_SOAK=1).",
			Ref:     "ROADMAP Milestone B"},
		{ID: "milestone_d_full", Area: "Certification", Title: "Mempool policy parity (full)", Status: "done", Effort: "M",
			Summary: "58-template corpus + offline stateful eval green via dogego cert milestones-bde; live 24/24 Core compare on dogego-live via dogego cert weekly-live remains operator sign-off.",
			Ref:     "ROADMAP Milestone D"},
		{ID: "milestone_e_full", Area: "Certification", Title: "Operator workflow vs Core (full)", Status: "done", Effort: "L",
			Summary: "Seventeen live web operator-cert gates (incl. mining GBT/aux + PQ format probe) + offline operator workflow via dogego cert milestones-bde; dogego cert enable-weekly → provision → weekly-live → live-soak workflow 10 for full close.",
			Ref:     "ROADMAP Milestone E"},

		// Consensus & script
		{ID: "header_consensus", Area: "Consensus", Title: "Header validation corpus", Status: "done", Effort: "M",
			Summary: "Six-hundred-header stored + forty-eight-header batch; journal_length fixtures; bad-nBits index 24+.",
			Ref:     "STANDALONE §1"},
		{ID: "block_validity", Area: "Consensus", Title: "Block validity corpus", Status: "done", Effort: "M",
			Summary: "Stored genesis-512-block connect + Core-shaped CheckTransaction/CheckBlock reject classes.",
			Ref:     "STANDALONE §1"},
		{ID: "reorg_adversarial", Area: "Consensus", Title: "Adversarial reorg election", Status: "done", Effort: "M",
			Summary: "Multi-peer election + marginal defer + precious override + fork probe + invalid header reject tests.",
			Ref:     "STANDALONE §1"},

		// P2P
		{ID: "addrman_buckets", Area: "P2P", Title: "Core bucketed addrman", Status: "done", Effort: "XL",
			Summary: "256 tried + 1024 new hash buckets with 64-deep slot indices (TriedSlot / NewRefs multi-ref ≤8), Core-style nKey (learned_addrs.json v3), occupancy maps for place/evict, dogego_addrbook_n_key_set, GET /api/p2p addrman_info, GET /api/core-addrman-probe + scripts/core_addrman_workflow.ps1, bucket-spread tried eviction, churn soak + nKey save/load invariants.",
			Ref:     "ROADMAP Phase 1"},
		{ID: "peer_management", Area: "P2P", Title: "Peer quality under churn", Status: "done", Effort: "L",
			Summary: "Inbound eviction-when-full protects addnode (flexible host:port match) + BIP152 HB; prefers crowded /16 netgroups then oldest. Offline eclipse-pressure soak (TestEclipseInboundPressureSoak) + eviction table tests. Live multi-peer eclipse soak still open.",
			Ref:     "STANDALONE §3"},
		{ID: "inbound_serving", Area: "P2P", Title: "Inbound getdata/getheaders", Status: "done", Effort: "M",
			Summary: "Black-box getheaders+getdata incl. batch caps, witness/filtered inv, getheaders 2000 cap, BIP157 filter serve/range cap, duplicate getdata block inv; marginal reorg defer + invalid header policy tests.",
			Ref:     "STANDALONE §3"},
		{ID: "forward_ibd", Area: "P2P", Title: "Forward IBD body/connect convergence", Status: "done", Effort: "M",
			Summary: "Body-IBD header pause, inline connect at frontier, lag≥512 connect boost (up to 8× passes / 128 blocks per call); lag≥8192 syncUtxo 512 passes; caught-up connect lag≥1 drains stored bodies + debounced utxo.cache save (solo mine); startup skips SyncUtxo when snapshot matches contiguous bodies; RPC `dogego_connect_catch_up_{passes,batch,interval_ms}` on getblockchaininfo / Web UI summary; GET /api/core-ibd-convergence-probe snapshot (15th live web gate); GET /api/core-addrman-probe (16th live web gate); addrman churn soak; mainnet ~10k+ with connect lag ≤15 and 15m soak gate passing.",
			Ref:     "STANDALONE §3"},
		{ID: "bip152_compact", Area: "P2P", Title: "BIP152 compact blocks", Status: "done", Effort: "L",
			Summary: "BIP152 v1 relay code-complete (HB≤3, cmpct announce/reconstruct/getblocktxn, AuxPoW→inv/full block). Offline AuxPoW/cmpct edge tests + dogego cert bip152-soak (-skip-live default). getblockchaininfo includes dogego_cmpct_reconstruct_fallback_getdata. Live probe GET /api/core-bip152-probe; extended live HB soak still operator-owned (bip152_live_soak_gate.ps1 / -skip-live=false).",
			Ref:     "wire/cmpctblock.go, node/cmpct.go, dogego cert bip152-soak"},
		{ID: "auxpow_parent_chain", Area: "P2P", Title: "Litecoin parent chain store", Status: "declined", Effort: "-",
			Summary: "Matches Core: no separate Litecoin sync. Parent header/coinbase/merkle branches are embedded in each aux block (checkAuxPow / CAuxPow::check). Not a DogeGo gap vs Core.",
			Ref:     "ROADMAP Phase 3"},

		// Storage & chainstate
		{ID: "utxo_long_ibd", Area: "Storage", Title: "UTXO correctness long IBD + reorgs", Status: "done", Effort: "L",
			Summary: "Deterministic hash_serialized snapshots every checkpoint window; cross-check vs known-good nodes.",
			Ref:     "STANDALONE §2"},
		{ID: "undo_replay", Area: "Storage", Title: "Undo / disconnect fidelity", Status: "done", Effort: "L",
			Summary: "Reorg replay yields identical chainstate as linear sync; no Core undo files yet (RebuildFromChain path).",
			Ref:     "STANDALONE §2"},
		{ID: "crash_consistency", Area: "Storage", Title: "Power-loss crash model", Status: "done", Effort: "L",
			Summary: "Random kill during write-heavy phases; startup repair converges. Offline: headers/raw/filter/txindex crash fixtures + utxo.cache.tmp purge and corrupt-snapshot quarantine. Multi-hour live soak still open.",
			Ref:     "STANDALONE §2"},
		{ID: "prune_semantics", Area: "Storage", Title: "Prune mode integration", Status: "done", Effort: "M",
			Summary: "pruneblockchain removes raw bodies, writes prune_marker.json; getblockchaininfo pruned/prune_height; verifychain level 2 + getblock tip after prune.",
			Ref:     "STANDALONE §2"},

		// Mempool
		{ID: "mempool_admission", Area: "Mempool", Title: "Policy admission corpus", Status: "done", Effort: "M",
			Summary: "58-template corpus in core_mempool_vectors.json; align policy results with Core at mainnet workflow scale.",
			Ref:     "STANDALONE §4"},
		{ID: "package_accounting", Area: "Mempool", Title: "Ancestor/descendant parity", Status: "done", Effort: "M",
			Summary: "getmempoolentry ancestor/descendant counts and fees; package limit corpus; diamond/two-parent/fan-out + seeded DAG property tests (mempool/package_dag_property_test.go). Live package-RBF vs Core still open.",
			Ref:     "STANDALONE §4"},
		{ID: "rbf_edges", Area: "Mempool", Title: "BIP125 edge cases", Status: "done", Effort: "M",
			Summary: "Corpus: insufficient/sufficient fee, not-replaceable, FullRBF accept, package-fee aggregation, descendant limit; BIP125 rule 5 (<=100 replaced txs incl. descendants, ErrRBFTooManyConflicts) + rule 2 (no new unconfirmed inputs, ErrRBFNewUnconfirmedInput) in mempool_rbf.go; corpus rows rbf_too_many_conflicts + rbf_new_unconfirmed_input; more replacement envelopes backlog.",
			Ref:     "STANDALONE §4"},

		// RPC & wallet
		{ID: "rpc_workflows", Area: "RPC", Title: "Core workflow RPC suite", Status: "done", Effort: "L",
			Summary: "Operator RPC golden error codes + recovery idempotency tests; side-by-side vs Core partial.",
			Ref:     "STANDALONE §5"},
		{ID: "dogego_live_scheduled_ci", Area: "Certification", Title: "dogego-live scheduled CI", Status: "done", Effort: "M",
			Summary: "dogego cert workflow10, provision, weekly, enable-weekly, weekly-live, live-soak, setup-parity; GET /api/core-runner-probes + GET /api/core-workflow10-probe; workflow 10 runbook.",
			Ref:     "CORE_SIDE_BY_SIDE_WORKFLOWS workflow 10"},
		{ID: "wallet_keypool", Area: "Wallet", Title: "Core keypool semantics", Status: "done", Effort: "L",
			Summary: "Same idea as Core: a receive/change keypool with keypoolrefill, iskeypool, and pool consume on spend. Stored in HD wallet.json (not wallet.dat BDB). Import from Core wallet.dat replays pool into HD (pool_indices_replayed, hd_keypool_core_index).",
			Ref:     "ROADMAP Phase 11"},
		{ID: "wallet_core_migration", Area: "Wallet", Title: "Core wallet.dat migration", Status: "done", Effort: "L",
			Summary: "dogego_probewalletdat dry-run (pool_entries, pool_keys_matched/unmatched, pool_unmatched_hint, pool_indices_replayed on HD import, keypool_hint, keypool_refill_size); native BDB import incl. encrypted decryption (CCrypter) via options.passphrase; wallet/pool_replay.go HD keypool replay; Core dumpwallet fallback; Receive tab; dogego cert wallet-import (offline superset) + dogego cert wallet-migration (live probe/import); dogego cert preflight/provision/weekly-live pool metadata notes; GET /api/core-wallet-probe + core_wallet_workflow.ps1 when DOGEGO_WALLET_DAT set.",
			Ref:     "ROADMAP Phase 11"},
		{ID: "wallet_rpc_deep", Area: "Wallet", Title: "Full wallet RPC subset", Status: "done", Effort: "L",
			Summary: "UTXO-cache fast path for getwalletinfo (wallet_index_height, needs_rescan, dogego_wallet_scan_index_ok, dogego_wallet_history_fast_path, dogego_wallet_listtransactions_utxo_walk, dogego_wallet_listtransactions_scan_pending, scanning), getbalance, listunspent (wallet_utxo_scan.cache.json refreshed on connect); FilterRowsByScriptSet for fund/send/history; BlockStep /api/blockstep/address wallet fast path; web UI defers heavy wallet polls when dogego_connect_lag>32/64 or rescan builds index on solo-miner UTXO walk (>64 UTXOs); GET /api/wallet/txs and /api/wallet/txs.csv return deferred+defer_reason during IBD/connect lag/scan build; GET /api/wallet/txs prefers WalletTxs.ListPage (listtransactions bridge) before UTXO/scan fallbacks; type=all uses wallet.db scan history when receive rows indexed (else UTXO receives + scan sends); type=sent uses wallet.db scan index with lazy hex; listtransactions light path skips UTXO receive walk when wallet.db has receive rows; dashboard auto-rescan when needs_rescan or wallet_listtransactions_utxo_walk with >64 UTXOs (summary stub + Settings); internal self-transfer send amounts via SendDisplayKoinu; rescan/catch-up skip SyncUtxo when chainActive caught up (node/wallet_catchup_test.go); startup UTXO snapshot skip when bodies match (utxo_snapshot_guard_test.go); listtransactions/gettransaction/listsinceblock fee+hex from wallet.db; core-wallet-probe wallet_history_defer_reason skips listtransactions latency gate when deferred; getwalletinfo dogego_wallet_history_defer_reason mirrors web defer; wallet_tx_hex_ok/wallet_tx_fee_ok/wallet_listtransactions_ok (<3s for 40 rows when not deferred); POST /api/wallet/rescan + StartWalletCatchUpRescan on node open; PSBT round-trip + psbt_bip32_deriv_ok; signer_cmd_configured; external signer via signer_cmd.",
			Ref:     "ROADMAP Phase 11"},
		{ID: "hardware_psbt", Area: "Wallet", Title: "External / hardware PSBT", Status: "done", Effort: "L",
			Summary: "BIP32 deriv paths + optional HWI-compatible signer_cmd transport in walletprocesspsbt (errors propagated; SignTimeout); mocksigner offline e2e; Milestone E psbt_roundtrip_ok + psbt_bip32_deriv_ok + hardware_psbt_hint (incl. local round-trip with signer_cmd) + keypool_topup_ok on core-wallet-probe; Settings POST /api/signer-test for HWI enumerate before restart; Features card warns when signer_cmd set but no HWI device; native USB/HID without HWI not in scope.",
			Ref:     "ROADMAP Phase 11"},

		// Mining & PQ
		{ID: "mining_gbt_aux", Area: "Mining", Title: "GBT + aux mining templates", Status: "done", Effort: "M",
			Summary: "getblocktemplate Digishield bits + BIP22 longpoll; createauxblock/submitauxblock; getmininginfo; GET /api/core-mining-probe + dogego cert mining + scripts/core_mining_workflow.ps1; optional Core GBT side-by-side when tips align.",
			Ref:     "rpc/getblocktemplate.go, rpc/auxpow_mining.go"},
		{ID: "pq_verify", Area: "PQC", Title: "PQ commitment verify + relay", Status: "done", Effort: "L",
			Summary: "MVP complete: off-chain OP_RETURN + TX_C/TX_R verify; Raccoon-G = vendored libdogecoin src/raccoon_g (Core PR #8; CGO -tags raccoon_g; no placeholder); web Settings pq_carrier + Send carrier mode; GET /api/core-pq-probe; dogego cert pq. No production PQ safety claim.",
			Ref:     "ROADMAP Phase 10"},

		// Docs (Phase 12 - not Core parity but tracked on roadmap)
		{ID: "rpc_cookbooks", Area: "Docs", Title: "Per-RPC cookbooks", Status: "done", Effort: "L",
			Summary: "GET /api/rpc/cookbook - curl + dogego-cli per method; HTML reference at /api/rpc/reference.html.",
			Ref:     "ROADMAP Phase 12"},
		{ID: "openrpc", Area: "Docs", Title: "OpenRPC / OpenAPI surface", Status: "done", Effort: "M",
			Summary: "GET /api/openrpc.json; integration guides at GET /api/integration/guides.",
			Ref:     "ROADMAP Phase 12"},
	}
}
