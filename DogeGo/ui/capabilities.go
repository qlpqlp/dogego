// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"

	"dogego/rpc"
	"dogego/version"
)

// CapabilityFeature is one operator-facing capability row for the web UI.
type CapabilityFeature struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // live, partial, planned, stub, na
	Summary  string `json:"summary"`
	CoreNote string `json:"core_note,omitempty"`
	UITab    string `json:"ui_tab,omitempty"`
}

// CapabilityCategory groups features for the dashboard.
type CapabilityCategory struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	Blurb    string              `json:"blurb,omitempty"`
	Features []CapabilityFeature `json:"features"`
}

// RPCMethodRow is one JSON-RPC command with DogeGo help and parity class.
type RPCMethodRow struct {
	Method string `json:"method"`
	Class  string `json:"class"` // live, partial, stub
	Help   string `json:"help"`
}

// RoadmapHighlight is a short parity milestone for the Features page.
type RoadmapHighlight struct {
	Phase   string `json:"phase"`
	Title   string `json:"title"`
	Done    bool   `json:"done"`
	Summary string `json:"summary"`
}

// CoreProbeAPI documents one live operator probe endpoint for the Features tab.
type CoreProbeAPI struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	Milestone  string `json:"milestone,omitempty"`
	Bundled    bool   `json:"bundled,omitempty"`
}

// CapabilitiesManifest is returned by GET /api/capabilities.
type CapabilitiesManifest struct {
	ClientVersion  string               `json:"client_version"`
	Disclaimer     string               `json:"disclaimer"`
	ParitySummary  ParitySummary        `json:"parity_summary"`
	CoreGuidance   CoreGuidance         `json:"core_guidance"`
	Certification  CoreCertManifest     `json:"certification"`
	CoreProbeAPIs  []CoreProbeAPI       `json:"core_probe_apis"`
	Categories     []CapabilityCategory `json:"categories"`
	CoreParityGaps []CoreParityGap      `json:"core_parity_gaps"`
	RPCMethods     []RPCMethodRow       `json:"rpc_methods"`
	Roadmap        []RoadmapHighlight   `json:"roadmap"`
	Live           map[string]any       `json:"live"`
}

// DefaultCapabilitiesManifest documents what DogeGo does today vs Core targets.
func DefaultCapabilitiesManifest() CapabilitiesManifest {
	categories := capabilityCategories()
	gaps := DefaultCoreParityGaps()
	roadmap := roadmapHighlights()
	rpc := buildRPCMethodRows()
	return CapabilitiesManifest{
		ClientVersion: version.Display(),
		Disclaimer:    "DogeGo is experimental Go node software. It is not Dogecoin Core. Use official Core for production wallets, mining pools, and exchange integrations.",
		CoreGuidance:  DefaultCoreGuidance(),
		Certification: DefaultCoreCertManifest(),
		CoreProbeAPIs: defaultCoreProbeAPIs(),
		Categories:    categories,
		CoreParityGaps: gaps,
		RPCMethods:    rpc,
		Roadmap:       roadmap,
		ParitySummary: buildParitySummary(categories, gaps, roadmap, rpc),
		Live:          map[string]any{},
	}
}

func roadmapHighlights() []RoadmapHighlight {
	return []RoadmapHighlight{
		{Phase: "P2P", Title: "CGNAT / Starlink outbound relay", Done: true, Summary: "p2p_connectivity classic | cgnat | both; multi-peer relay without public inbound."},
		{Phase: "P2P", Title: "DogeGo relay CGNAT (DGR)", Done: true, Summary: "NODE_DOGEGO_RELAY_CGNAT QUIC tunnel; config dogego_relay_cgnat; metrics /api/dgr."},
		{Phase: "P2P", Title: "UPnP / NAT-PMP port mapping", Done: true, Summary: "upnp=auto|enable|disable; IGD2/IGD1 + NAT-PMP; localaddresses advertises mapped public IP."},
		{Phase: "Cert", Title: "Consensus differential harness", Done: true, Summary: "Multi-height PoW chains; Core hex block corpus; Milestone A field evidence."},
		{Phase: "Cert", Title: "Crash/corruption suite (offline)", Done: true, Summary: "Kill/repair fixtures + utxo.cache quarantine. Optional long-haul: dogego cert live-soak (self-cert of native Go storage)."},
		{Phase: "Cert", Title: "Scheduled weekly live CI", Done: true, Summary: "Offline gates green; optional dogego-live weekly-live / workflow 10 for long-haul self-cert."},
		{Phase: "Cert", Title: "Operator workflow probes", Done: true, Summary: "Seventeen live web gates + scripts; optional mainnet reindex/prune self-cert on your datadir."},
		{Phase: "P2P", Title: "Core bucketed addrman", Done: true, Summary: "256 tried + 1024 new hash buckets, 64-deep slot caps, nKey; capacity raised toward Core slot totals."},
		{Phase: "P2P", Title: "Parallel block download (IBD)", Done: true, Summary: "Multi-peer IBD: block-assist workers, relay lanes, height striping, peer scores."},
		{Phase: "Mempool", Title: "minrelay / incrementalrelay config", Done: true, Summary: "dogecoinconf.json + web Settings; BIP125, rolling eviction floor, getmempoolinfo."},
		{Phase: "Mempool", Title: "savemempool / loadmempool", Done: true, Summary: "dogego_mempool.json persist + startup restore (not Core mempool.dat)."},
		{Phase: "RPC", Title: "estimatesmartfee + mining warnings", Done: true, Summary: "Core-shaped fee/priority RPCs; estimatesmartpriority INF_PRIORITY when min relay enforced; getblockchaininfo/getnetworkinfo warnings chain-only."},
		{Phase: "RPC", Title: "ZMQ notifications", Done: true, Summary: "zmqpubhashblock/hashtx/rawblock/rawtx PUB sockets (pure-Go zmq4); Core multipart format; getzmqnotifications RPC."},
		{Phase: "Mempool", Title: "setmempoolpaused + feerate percentiles", Done: true, Summary: "Pause admission via RPC; getmempoolinfo feerate_percentiles when prevouts resolve."},
		{Phase: "RPC", Title: "getblockstats fees from UTXO/index", Done: true, Summary: "totalfee, feerate_percentiles, dustouts, utxo_size_inc when prevouts resolve."},
		{Phase: "RPC", Title: "getchaintxstats window from rawblocks", Done: true, Summary: "window_tx_count sums txs in stored blocks; window_final_block_hash."},
		{Phase: "RPC", Title: "getblocktemplate BIP22 + version bits", Done: true, Summary: "Proposal + longpoll, NextBlockBits target, GBTBlockVersion client rules, vbrequired:0 (Dogecoin Core)."},
		{Phase: "RPC", Title: "Mining GBT/aux Core-parity cert", Done: true, Summary: "GET /api/core-mining-probe + dogego cert mining; createauxblock/submitauxblock; optional Core GBT side-by-side."},
		{Phase: "Docs", Title: "Documentation hub (docs/ + web UI)", Done: true, Summary: "DOCUMENTATION, INTEGRATION, RPC, WALLET indexes; Docs tab + GET /api/docs; Features RPC catalog."},
		{Phase: "Docs", Title: "Per-RPC cookbooks + OpenRPC", Done: true, Summary: "GET /api/rpc/cookbook, /api/rpc/reference.html, /api/openrpc.json, /api/integration/guides."},
		{Phase: "Operator", Title: "GitHub auto-update", Done: true, Summary: "Daily release check, Overview banner, Settings → Interface → Updates, tray menu, SHA256 verify, Install update restart."},
		{Phase: "Operator", Title: "Settings RPC tools tab", Done: true, Summary: "GET /api/rpc/cookbook in Settings → Tools; run RPC without Console tab."},
		{Phase: "Extensions", Title: "Extension platform", Done: true, Summary: "Manifest v1, wasm/subprocess hosts, catalog+zip install, scoped permissions, dynamic dogego_ext_* RPC; Settings → Extensions tab."},
		{Phase: "Extensions", Title: "dogego.zkl2 ZK L2 overlay", Done: true, Summary: "zkproof-v1 P2P sync, Groth16 verify off-L1 (compressed + DIP + inline VK), Pebble store; no consensus fork."},
	}
}

func capabilityCategories() []CapabilityCategory {
	return []CapabilityCategory{
		{
			ID: "sync", Title: "Chain sync & storage",
			Blurb: "Headers-first sync with optional raw block bodies on disk.",
			Features: []CapabilityFeature{
				{ID: "headers", Name: "Header journal", Status: "live", Summary: "getheaders / headers loop, headers.bin per network.", CoreNote: "Core: full block tree + chainstate.", UITab: "overview"},
				{ID: "rawblocks", Name: "Raw block files", Status: "live", Summary: "Progressive getdata, contiguous body sync from genesis, assist peers.", CoreNote: "Core: blocks/ + LevelDB chainstate (not interchangeable).", UITab: "overview"},
				{ID: "reorg", Name: "Header reorg handling", Status: "live", Summary: "Journal truncate + raw/tx prune + in-memory UTXO rebuild to fork.", CoreNote: "Core: full reorg validation + persisted chainstate rewind.", UITab: "overview"},
				{ID: "txindex", Name: "Transaction index", Status: "live", Summary: "indexes/tx for confirmed txs when enabled.", CoreNote: "Core: txindex + optional filters.", UITab: "explorer"},
				{ID: "chainstate", Name: "UTXO / chainstate", Status: "live", Summary: "In-memory UTXO cache + utxo.cache snapshot; mempool/gettxout use it at tip.", CoreNote: "Core: chainstate/ LevelDB.", UITab: "features"},
				{ID: "auxpow", Name: "AuxPoW blocks", Status: "live", Summary: "checkAuxPow matches Core CAuxPow::check + CheckAuxPowProofOfWork (embedded Litecoin/parent header in each aux block; no separate parent chain sync on Core or DogeGo); headers_aux.bin; createauxblock/submitauxblock.", CoreNote: "Core: parent PoW from embedded CAuxPow only; no Litecoin P2P sync.", UITab: "features"},
			},
		},
		{
			ID: "p2p", Title: "P2P networking",
			Blurb: "Primary peer syncs headers/blocks; classic, cgnat, or both modes add relay peers (Settings → P2P connectivity).",
			Features: []CapabilityFeature{
				{ID: "p2p_single", Name: "Primary sync peer", Status: "live", Summary: "One TCP session drives header/block sync; ping/pong.", CoreNote: "Core: same role among many peers.", UITab: "overview"},
				{ID: "p2p_cgnat", Name: "CGNAT / multi-peer relay", Status: "live", Summary: "classic | cgnat | both - outbound-only relay for Starlink/CGNAT; optional inbound listen.", CoreNote: "Core: addrman, eviction, UPnP.", UITab: "settings"},
				{ID: "p2p_dgr", Name: "DogeGo relay CGNAT (DGR)", Status: "live", Summary: "NODE_DOGEGO_RELAY_CGNAT QUIC tunnel; dogego_relay_cgnat config; metrics on Overview and GET /api/dgr.", CoreNote: "Core: no equivalent service bit.", UITab: "settings"},
				{ID: "p2p_inv", Name: "inv / getdata blocks", Status: "live", Summary: "Block fetch, progressive sync, optional inv handler.", CoreNote: "Core: full inv routing.", UITab: "overview"},
				{ID: "p2p_cmpct", Name: "BIP152 compact blocks", Status: "live", Summary: "sendcmpct HB negotiate (up to 3 peers); cmpctblock announce on connect/mine/relay/inbound; inbound reconstruct + getblocktxn/blocktxn; getdata MSG_CMPCT_BLOCK (full block fallback when cmpct cannot encode, e.g. AuxPoW); scripts/core_bip152_probe.ps1 + GET /api/core-bip152-probe.", CoreNote: "Core: BIP152 v1 since 1.14.0; v2 witness not implemented.", UITab: "features"},
				{ID: "p2p_tx", Name: "Transaction relay", Status: "live", Summary: "Inbound/outbound tx when mempool policy accepts.", CoreNote: "Core: orphan pool, feerate policy.", UITab: "mempool"},
				{ID: "p2p_addnode", Name: "addnode / disconnectnode", Status: "live", Summary: "Manual peers when multi-peer P2P active; persisted to dogecoinconf.json addnode.", CoreNote: "Core: persistent addnode on disk.", UITab: "settings"},
				{ID: "p2p_ban", Name: "Peer banning", Status: "live", Summary: "setban / listbanned; banlist.json + misbehavior_scores.json persist per chain.", CoreNote: "Core: full addrman ban buckets.", UITab: "settings"},
				{ID: "p2p_multi", Name: "Multi-peer IBD", Status: "live", Summary: "Block-assist + relay lanes; peer scores in getpeerinfo; Core-scale addrman buckets.", CoreNote: "Core: parallel block download + addrman.", UITab: "features"},
			},
		},
		{
			ID: "mempool", Title: "Mempool & relay",
			Features: []CapabilityFeature{
				{ID: "mempool_pool", Name: "In-memory mempool", Status: "live", Summary: "Admission, RBF (BIP125 rule 2/5 + PaysForRBF descendant conflict fees), submitpackage CPFP package feerate, maxmempool, prioritisetransaction; 58-template offline eval + 32 live testmempoolaccept rows via GET /api/mempool/parity-probe.", CoreNote: "Core: full CBlockPolicyEstimator parity.", UITab: "mempool"},
				{ID: "relay_fees", Name: "Relay fee policy", Status: "live", Summary: "minrelaytxfee + incrementalrelayfee; package limits (-limit*); standardness (datacarrier, bare multisig); Settings + getmempoolinfo dogego_*_policy.", CoreNote: "Core: same -minrelaytxfee / -incrementalrelayfee / -limit* / -datacarrier flags.", UITab: "settings"},
				{ID: "sendraw", Name: "sendrawtransaction", Status: "live", Summary: "Decode, policy check, optional P2P relay; Missing inputs / already in chain paths.", CoreNote: "Core: same RPC; stricter wallet integration.", UITab: "mempool"},
				{ID: "feefilter", Name: "Fee filter", Status: "live", Summary: "TxConfirmStats on ConnectBlock + mempool confirms; fee_history.json persist; estimatefee nblocks=1 → -1.", CoreNote: "Core: priority-era estimator only.", UITab: "mempool"},
				{ID: "script_verify", Name: "Script verification", Status: "live", Summary: "P2PKH, P2PK, P2SH (multisig/CLTV/CSV); BIP9-aware CSV/CLTV at ConnectBlock; mempool NULLDUMMY + discourage upgradable NOPs; PUSHDATA1/2/4; decoderaw/getblock asm.", CoreNote: "Core: mempool STANDARD_SCRIPT_VERIFY_FLAGS subset.", UITab: "docs"},
				{ID: "tx_wire", Name: "Transaction wire format", Status: "live", Summary: "Read/write legacy + witness serialization; BIP141 size/weight/vsize in RPC; witness txs rejected at admission.", CoreNote: "Core: full CTransaction + segwit policy (disabled on Dogecoin).", UITab: "docs"},
			},
		},
		{
			ID: "explorer", Title: "Explorer & raw transactions",
			Features: []CapabilityFeature{
				{ID: "web_search", Name: "Web search", Status: "live", Summary: "Height, hash, txid, address scan (local data).", UITab: "explorer"},
				{ID: "getblock", Name: "getblock / getblockheader", Status: "live", Summary: "When raw block stored; default tip and confirmations at chainActive height.", CoreNote: "Core: always from chainstate.", UITab: "explorer"},
				{ID: "getrawtx", Name: "getrawtransaction", Status: "live", Summary: "Mempool + txindex + rawblocks; confirmations vs chainActive.", UITab: "explorer"},
				{ID: "gettxout", Name: "gettxout", Status: "live", Summary: "UTXO cache when synced; else scans raw blocks + mempool.", CoreNote: "Core: UTXO set lookup.", UITab: "explorer"},
				{ID: "createraw", Name: "createrawtransaction / signraw", Status: "live", Summary: "Legacy P2PKH/P2SH; limited signing.", CoreNote: "Core: full witness + wallet.", UITab: "explorer"},
				{ID: "merkle_proof", Name: "gettxoutproof", Status: "live", Summary: "CMerkleBlock subset; 80-byte headers only.", CoreNote: "Core: auxpow-aware proofs.", UITab: "explorer"},
			},
		},
		{
			ID: "rpc", Title: "JSON-RPC server",
			Features: []CapabilityFeature{
				{ID: "rpc_http", Name: "HTTP JSON-RPC", Status: "live", Summary: "chainActive-aware chain RPCs (getblockheader, waitfor*, gettxout, getnetworkhashps); verifychain, mempool, mining.", UITab: "settings"},
				{ID: "rpc_cookie", Name: "Cookie auth", Status: "live", Summary: ".cookie file when rpc_cookie set.", CoreNote: "Core: same pattern.", UITab: "settings"},
				{ID: "rpc_wallet", Name: "Wallet RPC surface", Status: "live", Summary: "Built-in HD wallet on mainnet and testnet: send, fund, PSBT subset, descriptors import; Core migration via dogego_probewalletdat + dogego_importwalletdat (pool probe + partial HD replay + keypool_hint); external signer via signer_cmd.", CoreNote: "Core: full wallet + external signer.", UITab: "features"},
				{ID: "rpc_mining", Name: "Mining / aux RPC", Status: "live", Summary: "getblocktemplate (NextBlockBits, BIP22 proposal + longpoll, vbavailable, coinbaseaux) + createauxblock/submitauxblock; GET /api/core-mining-probe + dogego cert mining + scripts/core_mining_workflow.ps1; optional Core GBT side-by-side.", CoreNote: "Core: getblocktemplate + aux mining RPCs.", UITab: "features"},
			},
		},
		{
			ID: "wallet", Title: "Built-in wallet",
			Features: []CapabilityFeature{
				{ID: "wallet_disk", Name: "Disk wallet (mainnet + testnet)", Status: "live", Summary: "BIP44 HD receive/change; encryptwallet + walletpassphrase; Core wallet.dat migration (native BDB + encrypted passphrase); PQ OP_RETURN send when pq_commitments flag on.", CoreNote: "Core: wallet.dat format.", UITab: "receive"},
				{ID: "wallet_send", Name: "Send payments", Status: "live", Summary: "sendtoaddress / sendmany / fundrawtransaction; web Send tab; avoid_reuse coin selection.", CoreNote: "Core: full accounts + external signer.", UITab: "send"},
				{ID: "wallet_psbt", Name: "PSBT utilities", Status: "live", Summary: "walletcreatefundedpsbt, walletprocesspsbt, psbtbumpfee; BIP32 deriv paths; optional external signer via signer_cmd.", CoreNote: "Core: hardware PSBT workflow.", UITab: "features"},
			},
		},
		{
			ID: "core_gaps", Title: "Self-certification",
			Blurb: "Core-compatible behavior ships; optional soaks certify DogeGo native storage resilience.",
			Features: []CapabilityFeature{
				{ID: "gap_cert", Name: "Optional long-haul soak", Status: "live", Summary: "Offline crash suite and 17 live web probes ship today. dogego cert live-soak / weekly-live / workflow 10 are optional self-cert of Go storage (headers/, rawblocks/, utxo.cache, wallet.db).", CoreNote: "Core never used this checklist; it is the reference LevelDB layout.", UITab: "features"},
				{ID: "core_live_probes", Name: "Live operator probes", Status: "live", Summary: "GET /api/core-probes bundle on this node. Optional Dogecoin Core reference compare when core_rpc_addr is set.", CoreNote: "Solo testnet: fully autonomous without Core.", UITab: "features"},
				{ID: "gap_consensus", Name: "Consensus & script", Status: "live", Summary: "Legacy script_tests 1059/1059 offline; header/block differential harnesses + mainnet field evidence.", CoreNote: "Core: src/validation.cpp, src/script/.", UITab: "docs"},
				{ID: "gap_p2p", Name: "P2P addrman & BIP152", Status: "live", Summary: "Core-scale addrman buckets + BIP152 v1 HB offline soak cert. Optional live HB soak is self-cert, not a missing feature.", CoreNote: "Core: src/net.cpp, src/net_processing.cpp.", UITab: "settings"},
				{ID: "gap_storage", Name: "Native chainstate", Status: "live", Summary: "Go-native headers/rawblocks/UTXO cache (not Core LevelDB). Crash quarantine shipped; optional multi-hour soak is self-cert.", CoreNote: "Core: blocks/ + chainstate/ LevelDB.", UITab: "features"},
				{ID: "gap_wallet", Name: "Wallet & hardware", Status: "live", Summary: "HD wallet.json keeps a Core-like receive/change keypool (not wallet.dat BDB). Core wallet.dat import + HWI signer_cmd. Native USB/HID without HWI is out of scope.", CoreNote: "Core: wallet.dat + external signer.", UITab: "features"},
			},
		},
		{
			ID: "ops", Title: "Operator & analytics",
			Features: []CapabilityFeature{
				{ID: "web_ui", Name: "Web dashboard", Status: "live", Summary: "Overview, explorer, mempool relay KPIs, Settings (dogecoinconf.json), setup wizard, update banner.", UITab: "overview"},
				{ID: "auto_update", Name: "GitHub auto-update", Status: "live", Summary: "Daily GitHub Releases check; Overview banner + Settings → Interface → Updates; tray download/install; dogego version -check.", UITab: "settings"},
				{ID: "system_tray", Name: "System tray", Status: "live", Summary: "Open Dashboard/Console, version line, check/download/install updates, native OS notification on new release.", UITab: "settings"},
				{ID: "analytics_db", Name: "Pebble analytics sidecar", Status: "live", Summary: "dogego_analytics.db (Pebble) + dogego indexer CLI.", CoreNote: "Core: no equivalent; uses own indexes.", UITab: "analytics"},
				{ID: "cli_indexer", Name: "CLI indexer", Status: "live", Summary: "dogego indexer reindex-tx, verify-bodies, scan.", UITab: "features"},
				{ID: "mining_local", Name: "Local mining", Status: "live", Summary: "generate/generatetoaddress: real scrypt PoW on reboot testnet and mainnet (RelaxedPoW=false); background mine on reboot testnet; merge-mining via createauxblock/submitauxblock; certified via GET /api/core-mining-probe.", CoreNote: "Core: generate* + GBT mining RPCs.", UITab: "settings"},
				{ID: "pqc", Name: "Post-quantum commitments", Status: "live", Summary: "OP_RETURN FLC1/DIL2/RCG4 + TX_C/TX_R carrier RPCs; wallet pq_commitments/pq_carrier flags; web Send carrier mode; GET /api/core-pq-probe; dogego cert pq. More soak testing welcome; not a consensus fork.", CoreNote: "Draft libdogecoin PQC carrier spec.", UITab: "features"},
				{ID: "extensions", Name: "Extensions platform", Status: "live", Summary: "Catalog/zip install, wasm+subprocess hosts, enable/disable; sidebar Extensions menu; dogego_listextensions + dogego_ext_* RPC.", CoreNote: "Core: no plugin model.", UITab: "extensions"},
				{ID: "zkl2", Name: "ZK L2 (dogego.zkl2)", Status: "live", Summary: "Optional zkproof-v1 overlay; tx-anchored Groth16 proofs; P2P zkinv/getzkproof; off-L1 verify (no OP_CHECKZKP fork).", CoreNote: "Inspired by Dogecoin #3869; extension-only.", UITab: "settings"},
				{ID: "bbpow", Name: "BBPoW research (dogego.bbpow)", Status: "experimental", Summary: "Testnet-only: verify Bitcoin SHA-256 commitments as a Dogecoin security signal (BBPoW/CAuxPoW). Not AuxPoW; not L1 consensus (hard-fork territory).", CoreNote: "Research extension only; Bitcoin unchanged.", UITab: "extensions"},
			},
		},
	}
}

func defaultCoreProbeAPIs() []CoreProbeAPI {
	return []CoreProbeAPI{
		{Path: "/api/core-probes", Label: "All probes (bundle)", Milestone: "E", Bundled: true},
		{Path: "/api/core-compare", Label: "getblockchaininfo + verifychain", Milestone: "E", Bundled: true},
		{Path: "/api/core-maintenance", Label: "verifychain, indexes, filters", Milestone: "E", Bundled: true},
		{Path: "/api/core-restart-resume", Label: "rawblocks_sync checkpoint", Milestone: "E", Bundled: true},
		{Path: "/api/core-ibd-convergence-probe", Label: "IBD convergence snapshot", Milestone: "E", Bundled: true},
		{Path: "/api/core-addrman-probe", Label: "Addrman snapshot", Milestone: "E", Bundled: true},
		{Path: "/api/core-autostart-probe", Label: "OS login autostart", Milestone: "E", Bundled: true},
		{Path: "/api/core-founder-probe", Label: "Reboot testnet founder", Milestone: "E", Bundled: true},
		{Path: "/api/core-runner-probes", Label: "CI runner readiness", Milestone: "E", Bundled: true},
		{Path: "/api/core-workflow10-probe", Label: "Workflow 10 preflight", Milestone: "E", Bundled: true},
		{Path: "/api/core-setup-parity", Label: "Milestone D setup parity", Milestone: "D", Bundled: false},
		{Path: "/api/mempool/parity-probe", Label: "testmempoolaccept corpus", Milestone: "D", Bundled: true},
		{Path: "/api/mempool/stateful-status", Label: "stateful mempool gate status", Milestone: "D", Bundled: false},
		{Path: "/api/core-wallet-probe", Label: "wallet workflow", Milestone: "E", Bundled: true},
		{Path: "/api/core-operator-cert", Label: "Operator cert live matrix", Milestone: "E", Bundled: false},
		{Path: "/api/core-status", Label: "Cached operator cert snapshot", Milestone: "E", Bundled: false},
		{Path: "/api/core-test", Label: "Core JSON-RPC reachability (POST)", Milestone: "E", Bundled: false},
		{Path: "/api/signer-test", Label: "HWI external signer reachability (POST)", Milestone: "E", Bundled: false},
		{Path: "/api/wallet/keypool-refill", Label: "HD keypool refill (POST)", Milestone: "E", Bundled: false},
		{Path: "/api/core-reindex-probe", Label: "reindex / index check-only", Milestone: "E", Bundled: true},
		{Path: "/api/core-bip152-probe", Label: "BIP152 getpeerinfo HB", Milestone: "E", Bundled: true},
		{Path: "/api/core-mining-probe", Label: "Mining GBT / aux templates", Milestone: "E", Bundled: true},
		{Path: "/api/core-pq-probe", Label: "PQ format/carrier offline probe", Milestone: "E", Bundled: true},
		{Path: "/api/core-field-evidence-probe", Label: "Milestone A field datadir readiness", Milestone: "A", Bundled: false},
		{Path: "/api/core-end-to-end-probe", Label: "end-to-end workflow bundle", Milestone: "E", Bundled: true},
		{Path: "/api/core-cert", Label: "Certification manifest", Milestone: "A-E"},
	}
}

func buildRPCMethodRows() []RPCMethodRow {
	methods := rpc.SupportedMethods()
	rows := make([]RPCMethodRow, 0, len(methods))
	for _, m := range methods {
		h, _ := rpc.MethodHelp(m)
		rows = append(rows, RPCMethodRow{
			Method: m,
			Class:  classifyRPCHelp(h),
			Help:   h,
		})
	}
	return rows
}

func classifyRPCHelp(help string) string {
	h := strings.ToLower(help)
	switch {
	case h == "":
		return "partial"
	case strings.Contains(h, "not implemented"):
		return "stub"
	case strings.Contains(h, "no wallet"), strings.Contains(h, "subset"), strings.Contains(h, "placeholder"),
		strings.Contains(h, "heuristic"), strings.Contains(h, "stub"), strings.Contains(h, "not available"),
		strings.Contains(h, "always 0"), strings.Contains(h, "always empty"), strings.Contains(h, "no-op"):
		return "partial"
	default:
		return "live"
	}
}

// EnrichCapabilitiesLive fills runtime flags for the manifest.
func EnrichCapabilitiesLive(m *CapabilitiesManifest, live map[string]any) {
	if m == nil {
		return
	}
	m.Live = live
}
