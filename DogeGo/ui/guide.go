// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// GuideTerm is one glossary entry for the web dashboard.
type GuideTerm struct {
	Term    string `json:"term"`
	Explain string `json:"explain"`
}

// GuideSection groups educational copy for operators.
type GuideSection struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Body  string      `json:"body"`
	Terms []GuideTerm `json:"terms,omitempty"`
	Links []DocsLink  `json:"links,omitempty"`
}

// GuideManifest is returned by GET /api/guide.
type GuideManifest struct {
	Title    string         `json:"title"`
	Subtitle string         `json:"subtitle"`
	Sections []GuideSection `json:"sections"`
}

// DefaultGuideManifest explains DogeGo vs Core for new operators.
func DefaultGuideManifest() GuideManifest {
	return GuideManifest{
		Title:    "How DogeGo works",
		Subtitle: "Short explanations for the dashboard and setup wizard. DogeGo is experimental - use official Dogecoin Core for production wallets and exchanges.",
		Sections: []GuideSection{
			{
				ID:    "sync",
				Title: "Headers vs block bodies",
				Body:  "DogeGo syncs in two layers, like Bitcoin Core headers-first IBD. The header journal records the proof-of-work chain (height, difficulty, timestamps). Block bodies are full transactions stored under rawblocks/ and must connect contiguously from genesis before the node treats itself as caught up on chainActive. The Sync tab shows download rate, blocks behind, and estimated time left; mainnet uses Core-style assumevalid to skip script checks on old buried blocks (see Settings: assumevalid).",
				Terms: []GuideTerm{
					{Term: "headers.bin", Explain: "Append-only header journal for your network (mainnet/ or testnet/)."},
					{Term: "rawblocks/", Explain: "Full P2P block payloads fetched with getdata during IBD."},
					{Term: "chainActive", Explain: "Highest height with contiguous stored bodies - RPC getblockcount uses this, not the header tip while bodies lag."},
					{Term: "IBD", Explain: "Initial block download - parallel block-assist workers plus the primary peer lane (maxoutbound / block_sync_workers in config)."},
					{Term: "assumevalid", Explain: "Core -assumevalid: trust a buried block hash and skip ECDSA checks below it; recent blocks near the tip are still fully verified."},
					{Term: "verification_progress", Explain: "During IBD on mainnet, the hero % uses dogego_tx_verification_progress (Core tx curve) when initialblockdownload is true; otherwise block-body coverage vs header tip."},
				},
			},
			{
				ID:    "p2p",
				Title: "Peers & connectivity",
				Body:  "The primary TCP peer drives header sync and lane 0 block download. Extra outbound relays share transactions and can help with headers or blocks. Pick cgnat when you cannot accept inbound connections (Starlink, carrier NAT).",
				Terms: []GuideTerm{
					{Term: "classic", Explain: "Listen for inbound peers and maintain outbound relays."},
					{Term: "cgnat", Explain: "Outbound-only - no public listen; still relays txs/blocks via dialed peers."},
					{Term: "both", Explain: "Default: inbound listen when possible plus outbound relays."},
					{Term: "feefilter", Explain: "BIP133 minimum feerate your peers ask you to respect when relaying txs."},
				},
			},
			{
				ID:    "mempool",
				Title: "Mempool & relay policy",
				Body:  "The in-memory mempool holds unconfirmed transactions this node would relay. Minimum feerates come from -minrelaytxfee, peer feefilters, and the rolling floor after eviction (-incrementalrelayfee). Witness (segwit) transactions are decoded for RPC but rejected on Dogecoin - same as Core policy.",
				Terms: []GuideTerm{
					{Term: "minrelaytxfee", Explain: "Configured floor in koinu per kB (100000 ≈ 0.001 DOGE/kB default)."},
					{Term: "mempoolminfee", Explain: "Effective minimum from config, rolling eviction, and connected peers."},
					{Term: "package limits", Explain: "Ancestor/descendant count and size caps for chained unconfirmed txs."},
					{Term: "dogego_mempool.json", Explain: "Optional JSON persist - not interchangeable with Core mempool.dat."},
				},
			},
			{
				ID:    "storage",
				Title: "Storage vs Dogecoin Core",
				Body:  "DogeGo does not use Core blocks/blk*.dat or chainstate/ LevelDB. You cannot point Dogecoin Core at a DogeGo datadir. Analytics uses a separate Pebble store (dogego_analytics.db) for checkpoints only.",
				Terms: []GuideTerm{
					{Term: "UTXO cache", Explain: "In-memory set at the connected tip; optional utxo.cache snapshot - not Core chainstate."},
					{Term: "tx index", Explain: "indexes/tx maps txid → block for explorer and getrawtransaction."},
					{Term: "AuxPoW", Explain: "Merged-mining headers after the aux era; parent chain is validated without storing Litecoin blocks."},
				},
			},
			{
				ID:    "dashboard",
				Title: "Web dashboard tabs",
				Body:  "The loopback dashboard (default localhost:2013 for WebAuthn/biometrics; 127.0.0.1:2013 also works) polls /api/summary and related APIs. Overview shows sync, chain search, wallet balance, and operator cert N/17 when probes are warm. Explorer needs the tx index. Mempool and Analytics are full-node only. Send/Receive/History support optional PQ commitments and carrier sends (Settings → Wallet). Features tab runs seventeen live operator-cert probes incl. mining GBT/aux (GET /api/core-mining-probe) and PQ format (GET /api/core-pq-probe). Settings edits dogecoinconf.json and can restart services; browser display prefs live in localStorage only.",
				Terms: []GuideTerm{
					{Term: "Overview", Explain: "Sync progress (headers vs contiguous bodies), quick search, network stats, optional raw summary JSON."},
					{Term: "Send / Receive", Explain: "Built-in HD wallet: sendtoaddress, optional PQ OP_RETURN or carrier (pq_mode), QR receive address, history list."},
					{Term: "Explorer", Explain: "Height, block hash, txid, or address lookup via local tx index + rawblocks."},
					{Term: "Mempool", Explain: "Live pool size, feerates, relay policy strip, pause/resume admission."},
					{Term: "Analytics", Explain: "Charts from embedded sidecar Pebble store (optional at setup)."},
					{Term: "Features", Explain: "Core parity dashboard: summary counts, when to use Core vs DogeGo, live node flags, roadmap, backlog, capability categories, searchable RPC table (GET /api/capabilities)."},
					{Term: "Docs", Explain: "Merged documentation: sync/P2P/mempool concepts, integration, operator runbooks, embedded markdown viewer."},
					{Term: "Console", Explain: "JSON-RPC console and rolling node log."},
					{Term: "Settings", Explain: "Paths, P2P, mempool package limits, RPC auth, restart node - most fields need restart."},
				},
			},
			{
				ID:    "wallet",
				Title: "Built-in wallet",
				Body:  "wallet.json per network (mainnet or testnet): BIP44 receive/change, encryptwallet, importdescriptors subset (pkh, sh(pkh/multi), timelock sh(cltv/csv), bare multi with flag), dumpwallet descriptor= lines, fundrawtransaction, bumpfee / psbtbumpfee with CLTV/CSV. PSBT wallet RPCs (create/fund/process/finalize utilities) with BIP32 deriv paths; optional external signer via signer_cmd (enumeratesigners, signerdisplayaddress). Optional PQ: setwalletflag pq_commitments (OP_RETURN FLC1/DIL2/RCG4) and pq_carrier (TX_C/TX_R carrier sends); Settings → Wallet toggles via GET/POST /api/wallet/flags; Send tab Advanced carrier mode (pq_mode: carrier). Core wallet.dat migration via dogego_probewalletdat + dogego_importwalletdat (native BDB, pool probe + pool_unmatched_hint + pool_indices_replayed on HD import + keypool_refill_size + keypool_hint, encrypted via passphrase, or core_rpc_addr fallback); Receive tab wallet.dat card and address book (iskeypool / hd_keypool_core_index tags); GET /api/core-wallet-probe and scripts/core_wallet_workflow.ps1 report address_book_keypool_count when DOGEGO_WALLET_DAT or live wallet is wired. Balances use the UTXO cache - rescan after import. See the Docs tab (wallet section) and docs/WALLET.md for workflows.",
				Terms: []GuideTerm{
					{Term: "importdescriptors", Explain: "Watch or spendable import for supported output descriptors."},
					{Term: "deriveaddresses", Explain: "Derive display addresses from a non-range descriptor."},
					{Term: "avoid_reuse", Explain: "Wallet flag deprioritizes reused scripts in coin selection."},
					{Term: "Docs tab", Explain: "Merged documentation hub: dashboard concepts, integration, RPC workflows, and links to docs/ in the repo."},
				},
			},
			{
				ID:    "core_parity",
				Title: "DogeGo vs Dogecoin Core",
				Body:  "Dogecoin Core (src/) is the specification. DogeGo targets Core-compatible sync, mempool policy, and JSON-RPC on typical operator paths but is not production-certified until the standalone acceptance matrix is complete. Mainnet consensus rules are locked to Core (no protocol forks); chain reorg in code means competing tips, not new activations. Open the Features tab for live counts (capabilities live/partial, parity gaps open/partial/declined, RPC class breakdown) and runtime flags for this node. Intentional differences include native chain storage and no Litecoin parent-chain sync for AuxPoW. BIP152 compact blocks are partial (v1 HB relay in code; live operator soak remains open).",
				Terms: []GuideTerm{
					{Term: "Dogecoin protocol lock", Explain: "DogeGo does not change mainnet consensus rules. Block/header/script/subsidy/auxpow follow Dogecoin Core; any divergence is a bug. PQ and experimental wallet paths are optional and off-chain unless explicitly reviewed. See ROADMAP.md and docs/INTENTIONAL_DIFFERENCES.md."},
					{Term: "GET /api/capabilities", Explain: "parity_summary, core_guidance, certification, categories, core_parity_gaps, roadmap, rpc_methods, live - powers the Features tab."},
					{Term: "GET /api/core-operator-cert", Explain: "Live operator certification matrix: seventeen web probe gates (compare, maintenance, restart-resume, OS autostart, reboot testnet founder, CI runner readiness, restart-connect, Milestone D setup-parity, mempool parity, reindex, BIP152 HB, PQ format, wallet, mining GBT/aux, end-to-end, IBD convergence, addrman) plus script-only soak rows. ?matrix=1 for definitions only; ?refresh=1 bypasses the 90s probe cache."},
					{Term: "GET /api/core-probes", Explain: "Runs all live operator probes on this DogeGo node (maintenance, wallet, BIP152, mining GBT/aux, PQ format, end-to-end, …). Optional Dogecoin Core reference compare only when core_rpc_addr is set in Settings → Advanced."},
					{Term: "GET /api/core-mining-probe", Explain: "Mining cert: getmininginfo + getblocktemplate (Digishield bits, BIP22 longpoll) + createauxblock in aux era; optional Core GBT compare when tips align. Features feat-core-mining card; live operator-cert gate mining. Offline: dogego cert mining."},
					{Term: "GET /api/core-pq-probe", Explain: "PQ format/carrier offline probe: FLC1/DIL2/RCG4 OP_RETURN round-trip + TX_C/TX_R carrier pair; verifier-side only. Features feat-core-pq card; live operator-cert gate pq_format. Offline: dogego cert pq."},
					{Term: "GET /api/core-wallet-probe", Explain: "Wallet workflow when built-in wallet enabled: getwalletinfo, getbalance, getnewaddress, validateaddress, dogego_listwalletaddresses (address_book_keypool_count / address_book_core_pool_indices_stored), validateaddress/getaddressinfo iskeypool round-trip on first keypool row, pool_core_indices_stored vs address book count, setlabel round-trip; optional dogego_probewalletdat when DOGEGO_WALLET_DAT is set (Milestone E)."},
					{Term: "GET /api/core-reindex-probe", Explain: "Check-only reindex workflow: required RPC methods + getindexinfo sync (Milestone E). Optional Core getindexinfo compare when core_rpc_addr is set and the node is caught up."},
					{Term: "GET /api/core-maintenance", Explain: "Milestone E maintenance RPC bundle: verifychain levels 2 and 4, getindexinfo, getchaintxstats, getblockfilter; optional Core verifychain + getindexinfo + getchaintxstats + getblockfilter at shared tip when core_rpc_addr is set."},
					{Term: "GET /api/core-restart-resume", Explain: "Milestone E restart resume: rawblocks_sync checkpoint vs contiguous bodies, IBD assist pool, getzmqnotifications."},
					{Term: "GET /api/core-ibd-convergence-probe", Explain: "Milestone E IBD progress snapshot (headers, contiguous bodies, connect boost). Timed convergence window: dogego cert ibd-convergence or scripts/ibd_convergence_check.ps1."},
					{Term: "GET /api/core-addrman-probe", Explain: "Partial Core addrman snapshot: getaddrmaninfo bucket stats cross-checked against getblockchaininfo dogego_addrbook_* fields. Mirrors scripts/core_addrman_workflow.ps1."},
					{Term: "GET /api/core-field-evidence-probe", Explain: "Milestone A field evidence: offline committed corpus status plus live dogedata/mainnet readiness for TestCoreMainnetFieldHeaderPoW and field_disk_connect_cert.ps1. Offline cert: dogego cert field-evidence."},
					{Term: "dogego cert offline", Explain: "Cross-platform CI push/PR offline gate (offlinegate/ suites). Mirrors scripts/ci_offline_gate.ps1."},
					{Term: "dogego cert wallet-import", Explain: "Offline BIP39/BIP38 + signer + wallet.dat import cert (walletimport/). Superset of wallet-migration; mirrors wallet_import_cert.ps1."},
					{Term: "dogego cert operator", Explain: "Milestone E standalone operator cert: core consensus/store/node/rpc differential harness + field-evidence + wallet-import (~5-20 min). Mirrors operator_workflow_cert.ps1."},
					{Term: "dogego cert pq", Explain: "PQ OP_RETURN + TX_C/TX_R carrier format cert (pqcert/). Verifies current scaffolding only - not production PQ safety or consensus relay policy."},
					{Term: "dogego cert mining", Explain: "Offline mining GBT/aux cert (ui ProbeCoreMining + rpc getblocktemplate/createauxblock tests). Live: GET /api/core-mining-probe or scripts/core_mining_workflow.ps1."},
					{Term: "GET /api/core-compare", Explain: "Optional side-by-side compare when core_rpc_addr is set (chain tips, chainwork, mediantime, verificationprogress when caught up, verifychain, getmempoolinfo relay policy fullrbf/minrelaytxfee/incrementalrelayfee, getdeploymentinfo + getblockchaininfo softforks protocol-lock). Without Core, solo operators still get deployment and softfork sanity vs consensus params. Not required for normal operation."},
					{Term: "GET /api/core-status", Explain: "Cached operator cert snapshot and Core RPC config without running probes. Includes mempool_offline_corpus and mempool_parity when probe cache is warm. /api/summary also includes dogego_operator_cert_* and dogego_mempool_* when cache is warm."},
					{Term: "GET /api/core-end-to-end-probe", Explain: "Bundled operator workflow (node health, restart-resume, maintenance, reindex, Milestone D offline corpus + BIP125 rule 2/5 + live mempool parity, BIP152 HB, protocol-lock, setup-parity, wallet, mining GBT/aux) - implemented in Go inside the running node; optional scripts/core_end_to_end_workflow.ps1 is a Windows-side mirror for CI."},
					{Term: "POST /api/core-test", Explain: "Quick Core JSON-RPC reachability from Settings → Advanced (tests form values before save)."},
					{Term: "POST /api/signer-test", Explain: "Quick HWI external signer enumerate from Settings → Advanced (tests signer_cmd form value before save/restart)."},
					{Term: "GET /api/mempool/parity-probe", Explain: "Live testmempoolaccept on stateless rows; offline_stateful + stateful_live gate summary on default probe. Core side-by-side when core_rpc_addr set. ?corpus=full or ?corpus=stateful for offline eval only."},
					{Term: "GET /api/mempool/stateful-status", Explain: "Read-only Milestone D stateful offline corpus + live 24/24 reboottestnet gate hints (no testmempoolaccept rows)."},
					{Term: "GET /api/core-setup-parity", Explain: "Read-only Milestone D reboottestnet bootstrap check (dogego + Core wallet readiness for stateful 24/24 gate). CLI: dogego cert setup-parity -mine-bootstrap."},
					{Term: "GET /api/core-cert", Explain: "Certification milestones A/B/D/E, corpus sizes, offline go test names, operator script paths."},
					{Term: "live / partial / planned", Explain: "Capability row status: works today, MVP with gaps, or backlog."},
					{Term: "open / partial / declined", Explain: "Parity gap status from ROADMAP.md and STANDALONE_FULLNODE_ACCEPTANCE.md."},
					{Term: "standalone_ready", Explain: "true only when every parity gap is done (no open or partial rows). Today: partial - certification milestones A/B/D/E still in progress."},
				},
				Links: []DocsLink{
					{Label: "CORE_PARITY_GAPS.md", Path: "docs/CORE_PARITY_GAPS.md"},
					{Label: "INTENTIONAL_DIFFERENCES.md", Path: "docs/INTENTIONAL_DIFFERENCES.md"},
					{Label: "STANDALONE_FULLNODE_ACCEPTANCE.md", Path: "docs/STANDALONE_FULLNODE_ACCEPTANCE.md"},
				},
			},
			{
				ID:    "setup",
				Title: "First-run setup wizard",
				Body:  "When datadir is unset, dogego node serves a setup wizard on the web UI port. Save writes dogecoinconf.json; Save & start node launches the full node and opens the dashboard in the same browser tab (not a new window). Default: do not auto-open an extra browser tab.",
			},
			{
				ID:    "security",
				Title: "Security habits",
				Body:  "Keep RPC and this dashboard on loopback (127.0.0.1). Use rpc_cookie or strong rpc_user/password. Optional HTTPS: set webui_tls_local / rpc_tls_local in dogecoinconf.json (or Settings → Interface → Local HTTPS) to auto-generate certs under datadir/tls/; use local_tls_trust_ca or dogego tls trust-ca so browsers trust the local CA. Explicit PEM paths: webui_tls_cert/key and rpc_tls_cert/key. A reverse proxy with TLS is still recommended for remote access.",
			},
		},
	}
}
