// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocAuditRoadmapCheckedItems ensures key ROADMAP deliverables stay marked done.
func TestDocAuditRoadmapCheckedItems(t *testing.T) {
	root := repoRoot(t)
	roadmap, err := os.ReadFile(filepath.Join(root, "ROADMAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(roadmap)
	for _, needle := range []string{
		"[x] **Per-RPC cookbook**",
		"[x] **OpenAPI** (or OpenRPC)",
		"[x] **WebAuthn**",
		"[x] **Core parity:** addrman buckets",
		"[x] **Milestone A (mainnet field evidence cert, partial):**",
		"[x] **Operator UX:** **`dogego cert field-evidence`**",
		"offlinegate/bootstrap.go",
		"fieldevidence/suites.go",
		"STANDALONE_FULLNODE_ACCEPTANCE.md",
		"dogego cert offline",
		"dogego cert setup-parity",
		"dogego cert weekly-live",
		"dogego cert live-soak",
		"dogego cert enable-weekly",
		"dogego cert workflow10",
		"runner/workflow10.go",
		"GET /api/core-workflow10-probe",
		"ui/workflow10_probe.go",
		"RPC_CONSOLE_TUTORIAL.md",
		"/api/rpc/cookbook",
		"runner_readiness",
		"GET /api/core-runner-probes",
		"**17** live web gates",
		"[x] **Operator UX:** **`dogego cert weekly-live`**",
		"[x] **Operator UX:** **`dogego cert setup-parity`**",
		"[x] **MVP:** **`dogego_importmnemonic`**",
		"pool_index_min",
		"pool_indices_replayed",
		"wallet/pool_replay.go",
		"Certification exit checklist",
		"Offline prerequisites",
		"cert_offline_prerequisites",
		"dogego cert wallet-migration",
		"dogego cert wallet-import",
		"dogego cert operator",
		"dogego cert pq",
		"-offline-only",
		"pq_cert.ps1",
		"Milestone E operator (deep)",
		"[x] **MVP:** **solo background miner mempool inclusion**",
		"[x] **Web UI polish:** **Send coin control (Advanced)**",
		"[x] **Core parity:** **pruneblockchain idempotency**",
		"[x] **Operator UX:** **P2P identity + node-tip HD key**",
		"desktop/bip21.go",
		"wallet/node_tip.go",
		"isnodetip",
		"POST /api/config/uacomment-preview",
		"ui/uacomment_preview.go",
		"address_book_node_tip_count",
		"nodetip_validateaddress_ok",
		"[x] **MVP:** **DogeGo relay CGNAT (DGR phase 1)**",
		"node/dgr/discover.go",
		"substituteRPCParams",
		"rpc/cookbook_examples.go",
		"POST /api/wallet/unlock",
		"ui/setup_wallet_encrypt.go",
		"node/solo_mining_runtime.go",
		"GET /api/peers",
		"Settings wallet encryption panel",
		"POST /api/wallet/encrypt",
		"ui/wallet_encrypt_api.go",
		"POST /api/update/apply",
		"POST /api/update/download",
		"version/update_download.go",
		"cmd/dogego/restart_spawn",
		"Settings → Tools",
		"st-tools-groups",
		".github/workflows/release.yml",
		"POST /api/control/restart",
		"-replacetarget",
		"Install update",
		"[x] **Operator UX:** **auto-update**",
		"relay seed addnode",
		"DGR tunnel-first",
		"[x] **Operator UX:** **monthly wallet backup reminder**",
		"[x] **Phase 12:** **auto-update operator docs**",
		"dogego_backup_last_download",
		"OPERATOR.md) § Auto-update",
		"st-update-status",
		"POST /api/update/check",
		"dogego version -check",
		"dogego cert workflow10",
		"[x] **Operator UX:** **Settings → Interface updates panel**",
		"desktop/notify",
		"SetOnAvailable",
		"check_update.ps1",
		"schedule_update_check.ps1",
		"[x] **Operator UX:** **native OS update notification**",
		"DGR phase 2",
		"node/dgr/p2p_frame.go",
		"p2p_proxy.go",
		"dgr_tunnel_conn.go",
		"dogego_dgr_tunnel",
		"SetDGRTunnel",
		"DialP2POutbound",
		"dgr_wiring.go",
		"DGR phase 2 polish",
		"DGR phase 3",
		"node/dgr/tls_pin.go",
		"relay_tls_pins",
		"dgr_relay_book.go",
		"server_cert_sha256",
		"DGR phase 3 settings polish",
		"st-dgr-tls-pins",
		"useServerCertPin",
		"dogego cert ibd-convergence",
		"ibdconvergence/",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ROADMAP.md missing checked item: %s", needle)
		}
	}
}

func TestDocAuditWebUIOperatorCertGates(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "WEB_UI.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"N/17",
		"GET /api/core-runner-probes",
		"GET /api/core-pq-probe",
		"GET /api/core-mining-probe",
		"pq_carrier_enabled",
		"pq_mode",
		"`iskeypool` round-trip",
		"wallet_dat_keypool_refill_size",
		"runner",
		"stateful-status",
		"protocol lock",
		"cert_offline_prerequisites",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WEB_UI.md missing %q", needle)
		}
	}
}

func TestDocAuditWebUILanRemoteAuth(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "WEB_UI.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"webui_remote_auth",
		"GET /api/lan-peer-hint",
		"LAN peer pairing",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WEB_UI.md missing %q", needle)
		}
	}
}

func TestDocAuditWebUISignerTest(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "WEB_UI.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"POST /api/signer-test",
		"signer_cmd",
		"Test external signer",
		"POST /api/wallet/keypool-refill",
		"Refill keypool",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WEB_UI.md missing %q", needle)
		}
	}
}

func TestDocAuditWebUIReceiveKeypoolFields(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "WEB_UI.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"iskeypool",
		"hd_keypool_core_index",
		"keypool/core-pool tags",
		"`/api/wallet/addresses`",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WEB_UI.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreCertManifest(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "ui", "core_cert.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"seventeen live operator-cert gates",
		"dogego cert provision -preflight",
		"dogego cert weekly -require-wallet-dat",
		"dogego cert weekly-live",
		"dogego cert enable-weekly",
		"runner/enable_weekly.go",
		"dogego cert live-soak",
		"ci_milestone_b_full_gate.ps1",
		"core_reboottestnet_core_aligned_gate.ps1",
		"workflow 10",
		"wallet/pool_replay.go",
		"dogego cert pq",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ui/core_cert.go missing %q", needle)
		}
	}
}

func TestDocAuditMilestoneBCert(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "MILESTONE_B_CERT.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert enable-weekly",
		"dogego cert weekly-live",
		"dogego cert live-soak",
		"-skip-scripts",
		"workflow 10",
		"DOGEGO_SCHEDULED_LIVE_SOAK=1",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("MILESTONE_B_CERT.md missing %q", needle)
		}
	}
}

func TestDocAuditIntegrationWorkflow10(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "INTEGRATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"GET /api/core-runner-probes",
		"dogego cert enable-weekly",
		"dogego cert weekly-live",
		"dogego cert live-soak",
		"dogego cert wallet-import",
		"dogego cert operator",
		"workflow 10",
		"pool_indices_replayed",
		"address_book_keypool_count",
		"`iskeypool` round-trip",
		"pool_core_indices_stored",
		"POST /api/signer-test",
		"POST /api/wallet/keypool-refill",
		"GET /api/core-ibd-convergence-probe",
		"GET /api/core-addrman-probe",
		"GET /api/core-pq-probe",
		"POST /api/wallet/flags",
		"defer_reason",
		"core_addrman_workflow.ps1",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("INTEGRATION.md missing %q", needle)
		}
	}
}

func TestDocAuditDeveloperGuideTwelveGates(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "DEVELOPER_GUIDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"**17** live web gates",
		"GET /api/core-runner-probes",
		"dogego cert enable-weekly",
		"dogego cert operator",
		"workflow 10",
		"Offline prerequisites",
		"dogego cert wallet-import",
		"pool_indices_replayed",
		"pool_unmatched_hint",
		"keypool_refill_size",
		"wallet/pool_replay.go",
		"pool_keys_unmatched",
		"address_book_keypool_count",
		"wallet_dat_keypool_refill_size",
		"cert_offline_prerequisites",
		"ci_offline_gate.sh",
		"Dogecoin protocol lock",
		"no consensus fork",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("DEVELOPER_GUIDE.md missing %q", needle)
		}
	}
}

func TestDocAuditIntentionalDifferencesPQRelay(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "INTENTIONAL_DIFFERENCES.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, needle := range []string{
		"pq commitments (relay)",
		"isstandardtx",
		"dogego cert pq",
		"format recognition only",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("INTENTIONAL_DIFFERENCES.md missing %q", needle)
		}
	}
}

func TestDocAuditIntentionalDifferencesPoolFields(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "INTENTIONAL_DIFFERENCES.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"pool_indices_replayed",
		"pool_index_min",
		"wallet/pool_replay.go",
		"keypoolrefill",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("INTENTIONAL_DIFFERENCES.md missing %q", needle)
		}
	}
}

func TestDocAuditCapabilitiesTwelveGates(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "ui", "capabilities.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"seventeen live web gates",
		"BIP152 HB",
		"runner readiness",
		"mining GBT",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ui/capabilities.go missing %q", needle)
		}
	}
}

func TestDocAuditStandaloneNodeQuickstartCert(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "STANDALONE_NODE_QUICKSTART.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"dogego cert offline",
		"dogego cert wallet-import",
		"dogego cert operator",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("STANDALONE_NODE_QUICKSTART.md missing %q", needle)
		}
	}
}

func TestDocAuditOverviewOfflineCert(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "OVERVIEW.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"## Offline certification",
		"Protocol lock",
		"dogego cert offline",
		"dogego cert operator",
		"dogego cert wallet-import",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("OVERVIEW.md missing %q", needle)
		}
	}
}

func TestDocAuditGuideTwelveGates(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "ui", "guide.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"seventeen web probe gates",
		"PQ format",
		"GET /api/core-pq-probe",
		"BIP152 HB",
		"CI runner readiness",
		"Dogecoin protocol lock",
		"dogego cert operator",
		"dogego cert wallet-import",
		"address_book_keypool_count",
		"iskeypool round-trip",
		"pool_core_indices_stored",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("ui/guide.go missing %q", needle)
		}
	}
}

func TestDocAuditCoreSideBySideWorkflow10(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_SIDE_BY_SIDE_WORKFLOWS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"## Workflow 10: dogego-live scheduled CI",
		"Offline prerequisites",
		"dogego cert wallet-import",
		"dogego cert operator",
		"dogego cert wallet-migration",
		"dogego cert weekly-live",
		"dogego cert live-soak",
		"-skip-scripts",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_SIDE_BY_SIDE_WORKFLOWS.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreSideBySideWalletBasicsProbe(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_SIDE_BY_SIDE_WORKFLOWS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"## Wallet basics probe (Milestone E)",
		"GET /api/core-wallet-probe",
		"core_wallet_workflow.ps1",
		"address_book_keypool_count",
		"`iskeypool` round-trip",
		"pool_core_indices_stored",
		"pool_unmatched_hint",
		"wallet_history_fast_path",
		"wallet_listtransactions_utxo_walk",
		"wallet_scan_index_ok",
		"wallet_history_defer_reason",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_SIDE_BY_SIDE_WORKFLOWS.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreParityGapsWalletScanIndex(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_PARITY_GAPS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"wallet_history_fast_path",
		"wallet_listtransactions_utxo_walk",
		"wallet_listtransactions_scan_pending",
		"wallet_scan_index_ok",
		"GET /api/summary",
		"addrman_info",
		"dogego_wallet_listtransactions_utxo_walk",
		"TestExecListTransactionsWalletManyUtxosUsesScanIndex",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_PARITY_GAPS.md missing %q", needle)
		}
	}
}

func TestDocAuditIntegrationWalletScanFields(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "INTEGRATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"wallet_history_fast_path",
		"wallet_listtransactions_utxo_walk",
		"wallet_listtransactions_scan_pending",
		"wallet_scan_index_ok",
		"defer_reason",
		"wallet_history_defer_reason",
		"wallet_history_deferred",
		"txs.csv",
		"GET /api/summary",
		"addrman_info",
		"auto-starts incremental rescan",
		"addrbook_*",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("INTEGRATION.md missing %q", needle)
		}
	}
}

func TestDocAuditWebUIWalletAutoRescan(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "WEB_UI.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"wallet_listtransactions_utxo_walk",
		"wallet_listtransactions_scan_pending",
		"auto-starts incremental rescan",
		"merged into Receive",
		"History reloads automatically",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("WEB_UI.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreParityGapsDogegoLive(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_PARITY_GAPS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"**dogego-live scheduled CI**",
		"dogego cert weekly-live",
		"dogego cert enable-weekly",
		"dogego cert live-soak",
		"workflow 10",
		"GET /api/core-runner-probes",
		"GET /api/core-autostart-probe",
		"**17** live web operator-cert gates",
		"Stateful **24/24**",
		"dogego cert wallet-import",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_PARITY_GAPS.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreParityGapsKeypoolFields(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_PARITY_GAPS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"pool_unmatched_hint",
		"validateaddress",
		"address_book_keypool_count",
		"`iskeypool` round-trip",
		"pool_core_indices_stored",
		"core_wallet_workflow.ps1",
		"dogego cert preflight",
		"wallet_dat_keypool_refill_size",
		"wallet_dat_pool_unmatched_hint",
		"Why \"partial\" is not \"done\" yet",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_PARITY_GAPS.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreParityGapsPQMempool(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CORE_PARITY_GAPS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, needle := range []string{
		"pq_commitment_op_return",
		"mempool relay policy",
		"on-chain pq crypto verify",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_PARITY_GAPS.md missing %q", needle)
		}
	}
}

// TestDocAuditStandaloneOperatorCertGates guards live web gate count in acceptance matrix.
func TestDocAuditStandaloneOperatorCertGates(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "STANDALONE_FULLNODE_ACCEPTANCE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"**17** live web gates",
		"PQ format",
		"runner_readiness",
		"GET /api/core-runner-probes",
		"addrman snapshot",
		"GET /api/core-addrman-probe",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("STANDALONE_FULLNODE_ACCEPTANCE.md missing %q", needle)
		}
	}
}

func TestDocAuditStandaloneWalletMigrationPool(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "STANDALONE_FULLNODE_ACCEPTANCE.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"Protocol lock",
		"no protocol fork",
		"wallet/pool_replay.go",
		"pool_indices_replayed",
		"pool_keys_unmatched",
		"TestExecImportWalletDatNativePoolIndicesReplayed",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("STANDALONE_FULLNODE_ACCEPTANCE.md missing %q", needle)
		}
	}
}

func TestDocAuditContributingProtocolLock(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "CONTRIBUTING.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(raw))
	for _, needle := range []string{"protocol lock", "no protocol fork", "cert_offline_prerequisites"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CONTRIBUTING.md missing %q", needle)
		}
	}
}

func TestDocAuditDocumentationOfflineCert(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "DOCUMENTATION.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"Offline certification",
		"Protocol lock",
		"dogego cert offline",
		"dogego cert operator",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("DOCUMENTATION.md missing %q", needle)
		}
	}
}

// TestDocAuditCoreParityDocsExist guards standalone parity documentation paths.
func TestDocAuditCoreParityDocsExist(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"docs/CORE_PARITY_GAPS.md",
		"docs/DOCUMENTATION.md",
		"docs/STANDALONE_FULLNODE_ACCEPTANCE.md",
		"docs/INTENTIONAL_DIFFERENCES.md",
		"docs/INTEGRATION.md",
		"docs/RPC.md",
	} {
		st, err := os.Stat(filepath.Join(root, rel))
		if err != nil || st.IsDir() {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

// TestDocAuditRPCIndexMentionsIntegratorAPIs ensures RPC.md references machine-readable catalogs.
func TestDocAuditRPCIndexMentionsIntegratorAPIs(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs/RPC.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		"getaddrmaninfo",
		"/api/rpc/cookbook",
		"/api/openrpc.json",
		"RPC_CONSOLE_TUTORIAL.md",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("docs/RPC.md missing %q", needle)
		}
	}
}

// TestDocAuditWalletMdPoolFields ensures wallet migration docs mention Core keypool probe fields.
func TestDocAuditWalletMdPoolFields(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs", "WALLET.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{"pool_count", "pool_pubkeys", "pool_index_min", "pool_entries", "pool_keys_matched", "keypool_hint", "pool_indices_replayed", "iskeypool", "hd_keypool_core_index", "validateaddress", "pool_unmatched_hint", "pool_unmatched_entries", "keypool_refill_size", "pq_carrier", "GET /api/core-pq-probe"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("docs/WALLET.md missing %q", needle)
		}
	}
}

func TestDocAuditWalletMdHistoryDeferFields(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs", "WALLET.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		"dogego_wallet_history_deferred",
		"dogego_wallet_history_defer_reason",
		"GET /api/wallet/txs",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("docs/WALLET.md missing %q", needle)
		}
	}
}

// TestDocAuditOperatorMdPoolFields ensures operator docs mention Core keypool probe fields.
func TestDocAuditOperatorMdPoolFields(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs", "OPERATOR.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{"pool_keys_matched", "pool_keys_unmatched", "pool_entries", "keypool_hint", "pool_indices_replayed", "pool_unmatched_hint", "keypool_refill_size", "hd_keypool_core_index", "iskeypool", "validateaddress", "address_book_keypool_count", "pool-only", "wallet/pool_replay.go", "dogego cert wallet-import", "dogego cert operator", "dogego cert pq", "-offline-only"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("docs/OPERATOR.md missing %q", needle)
		}
	}
}

func TestDocAuditCoreOperatorRunbookWalletMigration(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs", "CORE_OPERATOR_RUNBOOK.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, needle := range []string{
		"dogego cert offline",
		"dogego cert wallet-import",
		"dogego cert operator",
		"dogego cert pq",
		"Protocol lock",
		"cert_offline_prerequisites",
		"Core wallet.dat migration",
		"pool_indices_replayed",
		"wallet/pool_replay.go",
		"pool_keys_unmatched",
		"pool_unmatched_hint",
		"keypool_refill_size",
		"hd_keypool_core_index",
		"Milestone D - mempool policy corpus",
		"core_mempool_bip125_offline_probe.ps1",
		"rbf_too_many_conflicts",
		"dogego_mempool_offline_corpus_passed",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CORE_OPERATOR_RUNBOOK.md missing %q", needle)
		}
	}
}

func TestDocAuditReadmeProtocolLock(t *testing.T) {
	root := repoRoot(t)
	paths := []string{filepath.Join(root, "README.md"), filepath.Join(filepath.Dir(root), "README.md")}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := strings.ToLower(string(b))
		if !strings.Contains(text, "protocol lock") && !strings.Contains(text, "no protocol fork") && !strings.Contains(text, "protocol fork") {
			t.Fatalf("%s missing protocol-lock mention", p)
		}
	}
}

func TestDocAuditProtocolLock(t *testing.T) {
	root := repoRoot(t)
	cases := map[string][]string{
		"ROADMAP.md":                      {"dogecoin protocol lock", "no consensus fork", "protocol fork"},
		"docs/INTENTIONAL_DIFFERENCES.md": {"consensus (locked on mainnet)", "protocol fork"},
		"docs/SECURITY.md":                {"protocol fidelity", "protocol fork"},
		"docs/CORE_PARITY_GAPS.md":        {"protocol lock", "protocol fork"},
	}
	for rel, needles := range cases {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(b))
		for _, needle := range needles {
			if !strings.Contains(text, strings.ToLower(needle)) {
				t.Fatalf("%s missing protocol-lock needle %q", rel, needle)
			}
		}
	}
}

func TestNoEmDashInUserFacingText(t *testing.T) {
	root := repoRoot(t)
	bad := []string{"\u2014", "\u2013"} // em dash, en dash
	extOK := map[string]bool{".go": true, ".md": true, ".json": true, ".html": true, ".ps1": true, ".sh": true, ".js": true, ".css": true}
	skipDir := map[string]bool{".git": true, "testdata": true, "node_modules": true, "dogedata": true, "rawblocks": true}
	var hits []string
	containsBad := func(s string) bool {
		for _, ch := range bad {
			if strings.Contains(s, ch) {
				return true
			}
		}
		return false
	}
	var walk func(string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				if skipDir[e.Name()] {
					continue
				}
				walk(p)
				continue
			}
			if !extOK[filepath.Ext(e.Name())] {
				continue
			}
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if containsBad(string(b)) {
				hits = append(hits, filepath.ToSlash(strings.TrimPrefix(p, root+string(filepath.Separator))))
			}
		}
	}
	walk(root)
	parent := filepath.Dir(root)
	for _, rel := range []string{"docs", "README.md"} {
		p := filepath.Join(parent, rel)
		if st, err := os.Stat(p); err != nil {
			continue
		} else if st.IsDir() {
			walk(p)
		} else if extOK[filepath.Ext(p)] {
			b, _ := os.ReadFile(p)
			if containsBad(string(b)) {
				hits = append(hits, filepath.ToSlash(rel))
			}
		}
	}
	if len(hits) > 0 {
		limit := 5
		if len(hits) < limit {
			limit = len(hits)
		}
		t.Fatalf("unicode em/en dash found in %d files, e.g. %v", len(hits), hits[:limit])
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
