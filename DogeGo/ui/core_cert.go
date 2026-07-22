// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "dogego/consensus"

// CertificationMilestone is one ROADMAP certification track item (A/B/D/E).
type CertificationMilestone struct {
	ID           string     `json:"id"`
	Phase        string     `json:"phase"`
	Title        string     `json:"title"`
	Status       string     `json:"status"` // partial, open, done
	Summary      string     `json:"summary"`
	OfflineTests []string   `json:"offline_tests,omitempty"`
	Scripts      []DocsLink `json:"scripts,omitempty"`
}

// CoreCertCorpus summarizes offline differential corpora (sync with consensus/testdata).
type CoreCertCorpus struct {
	MempoolVectors      int `json:"mempool_vectors"`
	MempoolParityProbe  int `json:"mempool_parity_probe_rows"`
	ScriptTestsLegacy   int `json:"script_tests_legacy"`
	HeaderHarnessStored int `json:"header_harness_stored"`
	BlockConnectStored  int `json:"block_connect_stored"`
}

// CoreCertManifest is returned by GET /api/core-cert (Features tab certification section).
type CoreCertManifest struct {
	Title           string                   `json:"title"`
	Disclaimer      string                   `json:"disclaimer"`
	Corpus          CoreCertCorpus           `json:"corpus"`
	Milestones      []CertificationMilestone `json:"milestones"`
	HarnessCommands []string                 `json:"harness_commands"`
	DocLinks        []DocsLink               `json:"doc_links"`
}

// DefaultCoreCertManifest documents certification milestones and operator scripts.
func DefaultCoreCertManifest() CoreCertManifest {
	probeRows := 17
	if rows, err := consensus.LoadMempoolParityRPCRows(); err == nil && len(rows) > 0 {
		probeRows = len(rows)
	}
	mempoolVectors := 58
	if vecs, err := consensus.LoadMempoolDifferentialVectors(); err == nil && len(vecs) > 0 {
		mempoolVectors = len(vecs)
	}
	return CoreCertManifest{
		Title:      "Core certification & differential harness",
		Disclaimer: "Mainnet consensus rules follow Dogecoin Core (no protocol forks). The running node self-heals (crash recovery, header repair, UTXO journal) without external scripts. Offline certification: go test or dogego cert offline (or scripts/cert_offline_prerequisites.{ps1,sh}). Live checks: web UI GET /api/core-probes (seventeen live operator-cert gates incl. Milestone D setup-parity, BIP152 HB, mining GBT/aux, PQ format probe, IBD convergence snapshot, addrman snapshot, and dogego-live runner readiness; built into the binary). Optional scripts/*.ps1 are Windows CI/operator extras when Dogecoin Core is installed side-by-side - not required to run DogeGo.",
		Corpus: CoreCertCorpus{
			MempoolVectors:      mempoolVectors,
			MempoolParityProbe:  probeRows,
			ScriptTestsLegacy:   1059,
			HeaderHarnessStored: 600,
			BlockConnectStored:  512,
		},
		Milestones: defaultCertificationMilestones(),
		HarnessCommands: []string{
			"dogego cert offline",
			"scripts/cert_offline_prerequisites.ps1",
			"dogego cert operator",
			"dogego cert field-evidence",
			"dogego cert wallet-migration",
			"dogego cert wallet-import",
			"dogego cert pq",
			"dogego cert provision -preflight",
			"dogego cert weekly -require-wallet-dat",
			"dogego cert weekly-live -mine-bootstrap -require-wallet-dat",
			"dogego cert enable-weekly -require-wallet-dat",
			"dogego cert workflow10 -mine-bootstrap -require-wallet-dat",
			"go test ./consensus/... -run TestCore -count=1",
			"go test ./rpc/... -run TestExecTestMempoolAcceptDifferential -count=1",
			"go test ./ui/... -run TestWallet -count=1",
			"go test ./rpc/... -run TestOperatorRPCErrorCodesGolden|TestOperatorRPCGoldenSubtestCount|TestExecGetWalletInfoCompact -count=1",
			"go test ./node -run TestOperatorWorkflowStandaloneCertification -count=1",
		},
		DocLinks: []DocsLink{
			{Label: "ROADMAP.md (protocol lock)", Path: "ROADMAP.md"},
			{Label: "SECURITY.md", Path: "docs/SECURITY.md"},
			{Label: "CORE_PARITY_GAPS.md", Path: "docs/CORE_PARITY_GAPS.md"},
			{Label: "STANDALONE_FULLNODE_ACCEPTANCE.md", Path: "docs/STANDALONE_FULLNODE_ACCEPTANCE.md"},
			{Label: "CORE_SIDE_BY_SIDE_WORKFLOWS.md", Path: "docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md"},
			{Label: "CORE_OPERATOR_RUNBOOK.md", Path: "docs/CORE_OPERATOR_RUNBOOK.md"},
		},
	}
}

func defaultCertificationMilestones() []CertificationMilestone {
	return []CertificationMilestone{
		{
			ID: "milestone_a", Phase: "A", Title: "Consensus differential harness",
			Status:  "partial",
			Summary: "Mainnet field header PoW + stored block connect; header/block/script/sighash offline corpora in go test ./consensus.",
			OfflineTests: []string{
				"TestCoreHeaderDifferentialHarness",
				"TestCoreBlockDifferentialHarness",
				"TestCoreScriptTestsRunnerSubset",
				"TestCoreMainnetFieldHeaderPoW",
				"TestCoreMainnetFieldStoredBlockConnect",
				"TestCoreMainnetFieldConnectCorpus",
				"TestCoreMainnetFieldMultiTxBlock15504",
				"TestProbeCoreFieldEvidenceOfflineCorpus",
				"TestProbeMainnetFieldDiskStatusDefaultPath",
				"TestCoreMainnetCheckpointHeaderAccept",
				"TestCoreMainnetFieldHeaderVectorsLegacyPoW",
				"TestCoreMainnetCheckpointHeaderLegacyPoW",
				"TestCoreMainnetCheckpointHeaderRejectCorpus",
				"TestCoreMainnetFieldAuxpowOfflineValidate",
				"TestCoreMainnetFieldDiskBundledConnect",
				"TestMainnetFieldBlocksBundledStoreContiguous",
				"TestMainnetFieldBlocksBundledMeasureContiguous",
				"TestMainnetFieldBlocksBundledValidateStoredBodies",
				"TestCoreDifferentialCorpusGate/mainnet_checkpoint_headers",
			},
			Scripts: []DocsLink{
				{Label: "dogego cert field-evidence", Path: "fieldevidence/suites.go"},
				{Label: "field_evidence_cert.ps1", Path: "scripts/field_evidence_cert.ps1"},
				{Label: "field_evidence_live_cert.ps1", Path: "scripts/field_evidence_live_cert.ps1"},
				{Label: "field_disk_connect_cert.ps1", Path: "scripts/field_disk_connect_cert.ps1"},
				{Label: "export_mainnet_field_blocks.ps1", Path: "scripts/export_mainnet_field_blocks.ps1"},
				{Label: "export_mainnet_field_headers.ps1", Path: "scripts/export_mainnet_field_headers.ps1"},
				{Label: "verify_mainnet_field_canonical.ps1", Path: "scripts/verify_mainnet_field_canonical.ps1"},
				{Label: "core_compare_with_core.ps1", Path: "scripts/core_compare_with_core.ps1"},
			},
		},
		{
			ID: "milestone_b", Phase: "B", Title: "Crash / corruption recovery",
			Status:  "partial",
			Summary: "Subprocess kill + offline corruption cert; cross-platform dogego cert live-soak (workflow 10; mirrors ci_milestone_b_full_gate.ps1); ibd_live_soak_gate timed IBD; addrman churn soak; connect catch-up boost diagnostics; corruption_extended_cert_mini timed loop gate; multi-hour scheduled soak still needs green dogego-live runs.",
			OfflineTests: []string{
				"TestStartupRecoveryConvergence",
				"TestChainTruncateUtxoReplay",
				"TestBundledTornTailReopenConvergence",
				"TestBundledTornTailReconcilesInflatedCheckpoint",
				"TestAutoRecoverSweepClampsBundledContiguousAfterTornTail",
				"TestCrashActiveBundledContiguous_MainnetFieldBlocks",
				"TestBuildRelaxedLegacyBlockWithExtraTx",
				"TestAddrBookChurnSoak",
				"TestAddrBookSaveLoadPreservesBuckets",
				"TestConnectCatchUpPassesDeepBodyIBD",
				"TestConnectCatchUpBlocksPerIBDCallDeepBody",
				"TestSyncUtxoMaxConnectPassesDeepBacklog",
				"TestEnrichIBDProgressSnapshotConnectCatchUpTuning",
			},
			Scripts: []DocsLink{
				{Label: "corruption_soak_cert.ps1", Path: "scripts/corruption_soak_cert.ps1"},
				{Label: "ibd_live_soak_gate.ps1", Path: "scripts/ibd_live_soak_gate.ps1"},
				{Label: "corruption_inject_soak.ps1", Path: "scripts/corruption_inject_soak.ps1"},
				{Label: "corruption_timed_loop.ps1", Path: "scripts/corruption_timed_loop.ps1"},
				{Label: "corruption_timed_loop_mini.ps1", Path: "scripts/corruption_timed_loop_mini.ps1"},
				{Label: "corruption_extended_cert_mini.ps1", Path: "scripts/corruption_extended_cert_mini.ps1"},
				{Label: "ci_milestone_b_full_gate.ps1", Path: "scripts/ci_milestone_b_full_gate.ps1"},
				{Label: "dogego cert live-soak", Path: "runner/live_soak.go"},
				{Label: "ci_scheduled_corruption_soak.ps1", Path: "scripts/ci_scheduled_corruption_soak.ps1"},
			},
		},
		{
			ID: "milestone_d", Phase: "D", Title: "Mempool policy parity",
			Status:  "partial",
			Summary: "58-template corpus; GET /api/mempool/parity-probe (stateless + stateful_live) and GET /api/mempool/stateful-status; offline stateful eval; live 24/24 reboottestnet via scripts; dogego cert weekly-live on dogego-live.",
			OfflineTests: []string{
				"TestCoreMempoolDifferentialVectors",
				"TestExecTestMempoolAcceptDifferentialVectors",
				"TestCoreMempoolVectorTemplatesCovered",
				"TestEvalMempoolCorpus",
				"TestEvalMempoolCorpusStateful",
				"TestStatefulMempoolLiveMapCoversKeyTemplates",
			},
			Scripts: []DocsLink{
				{Label: "core_mempool_corpus_probe.ps1", Path: "scripts/core_mempool_corpus_probe.ps1"},
				{Label: "core_mempool_parity_probe.ps1", Path: "scripts/core_mempool_parity_probe.ps1"},
				{Label: "core_mempool_bip125_offline_probe.ps1", Path: "scripts/core_mempool_bip125_offline_probe.ps1"},
				{Label: "mempool_stateful_parity_reboottestnet.ps1", Path: "scripts/mempool_stateful_parity_reboottestnet.ps1"},
				{Label: "core_compare_with_core.ps1 -MempoolProbe", Path: "scripts/core_compare_with_core.ps1"},
				{Label: "core_reboottestnet_core_aligned_gate.ps1", Path: "scripts/core_reboottestnet_core_aligned_gate.ps1"},
				{Label: "dogego cert weekly-live", Path: "runner/weekly_live.go"},
				{Label: "ci_scheduled_weekly_live.ps1", Path: "scripts/ci_scheduled_weekly_live.ps1"},
			},
		},
		{
			ID: "milestone_e", Phase: "E", Title: "Operator workflow vs Core",
			Status:  "partial",
			Summary: "GET /api/core-probes bundle (compare, maintenance, restart-resume, ibd_convergence, addrman, autostart, founder, runner, setup_parity, reindex, BIP152 HB, mining GBT/aux, pq format, mempool with stateful_live, wallet, end-to-end). Solo testnet: Core optional; IBD/sync OK. seventeen live operator-cert gates; dogego-live workflow 10.",
			OfflineTests: []string{
				"TestWalletBalanceFromUtxoCacheConfirmedImmature",
				"TestWalletUtxosHTTPUsesUtxoCacheFastPath",
				"TestWalletListUnspentFromUtxoCacheAllSpendScripts",
				"TestWalletListUnspentFromUtxoCacheEmptyReturnsOk",
				"TestWalletUtxosHTTPEmptyWhenCacheLive",
				"TestWalletUtxosHTTPAllSpendScripts",
				"TestWalletSendErrorResponseFeeHint",
				"TestWalletSendHTTPInsufficientFundsFeeHint",
				"TestWalletAPIEnvelopeZeroUtxosWhenCacheLive",
				"TestWalletCollectTransactionsUILightUsesHeightAsTime",
				"TestWalletUIRowsCacheHit",
				"TestExecGetWalletInfoHistoryDeferScanBuilding",
				"TestExecGetWalletInfoHistoryDeferConnectLag",
				"TestProbeCoreWalletOk",
				"TestProbeCoreWalletCountsTypedAddressBookRows",
				"TestProbeCoreWalletListLabelsMismatch",
				"TestProbeCoreWalletWalletDatProbe",
				"TestProbeCoreWalletWalletDatPoolUnmatchedWarning",
				"TestProbeCoreAddrmanOK",
				"TestProbeCoreWalletHistoryDeferredSkipsListtransactions",
				"TestProbeCorePQOK",
				"TestWalletTxHistoryDeferReasonScanBuilding",
				"TestWalletAPIEnvelopeHistoryDeferred",
				"TestEndToEndFromProbesAddrmanStep",
				"TestApplyCoreOperatorCertAddrman",
				"TestConnectLagCheckNoteBoost",
				"TestEndToEndFromProbesRestartResumeNote",
				"TestEndToEndFromProbesMempoolSteps",
				"TestApplyCoreOperatorCertRestartConnectNote",
				"TestWalletAddressNewAPI",
				"TestWalletAddressLabelAPI",
				"TestWalletLabelsAPI",
				"TestWalletProbeWalletDatAPI",
				"TestWalletImportWalletDatAPI",
				"TestWalletImportWalletDatPassphraseAPI",
				"TestWalletKeypoolRefillAPI",
				"TestExecImportWalletDatNativeSyntheticFixture",
				"TestExecImportWalletDatEncryptedSyntheticFixture",
				"TestExecImportWalletDatEncryptedDescriptorSyntheticFixture",
				"TestExecProbeWalletDatEncryptedDescriptorSyntheticFixture",
				"TestExecProbeWalletDatPoolSyntheticFixture",
				"TestExecProbeWalletDatMultiPoolRange",
				"TestExecImportWalletDatNativePoolMetadata",
				"TestExecImportWalletDatNativePoolIndicesReplayed",
				"TestExecImportWalletDatMixedPoolHDReplay",
				"TestExecImportWalletDatMixedPoolMetadata",
				"TestExecImportWalletDatMultiKeyNativeFixture",
				"TestImportWalletMultiKeyDump",
				"TestImportWalletCallsKeypoolRefill",
				"TestFixtureWalletDatProbeExtractImport",
				"TestOperatorRPCErrorCodesGolden",
				"TestOperatorRPCGoldenSubtestCount",
				"TestExecReindexTxIdempotentSecondPass",
				"TestExecReindexBlockFiltersIdempotentSecondPass",
				"TestExecDogegoRecoverHeadersIdempotentRestart",
				"TestExecTruncateToHeightIdempotentSecondCall",
				"TestExecSaveMempoolIdempotentSecondSave",
				"TestExecLoadMempoolIdempotentSecondLoad",
				"TestExecUpgradeTxIndexIdempotentSecondPass",
				"TestExecPruneBlockchainIdempotentSecondCall",
				"TestOperatorWorkflowStandaloneCertification",
			},
			Scripts: []DocsLink{
				{Label: "core_mainnet_side_by_side_runbook.ps1", Path: "scripts/core_mainnet_side_by_side_runbook.ps1"},
				{Label: "core_mainnet_restart_compare.ps1", Path: "scripts/core_mainnet_restart_compare.ps1"},
				{Label: "core_mainnet_reindex_compare.ps1", Path: "scripts/core_mainnet_reindex_compare.ps1"},
				{Label: "core_reboottestnet_reindex_workflow.ps1", Path: "scripts/core_reboottestnet_reindex_workflow.ps1"},
				{Label: "core_parity_probe.ps1", Path: "scripts/core_parity_probe.ps1"},
				{Label: "core_operator_workflow_cert.ps1", Path: "scripts/core_operator_workflow_cert.ps1"},
				{Label: "operator_workflow_cert.ps1", Path: "scripts/operator_workflow_cert.ps1"},
				{Label: "dogego cert operator", Path: "operatorworkflow/verify.go"},
				{Label: "core_operator_runbook_full.ps1", Path: "scripts/core_operator_runbook_full.ps1"},
				{Label: "core_e2e_reboottestnet_runbook.ps1", Path: "scripts/core_e2e_reboottestnet_runbook.ps1"},
				{Label: "core_e2e_full_runbook.ps1", Path: "scripts/core_e2e_full_runbook.ps1"},
				{Label: "core_e2e_mainnet_runbook.ps1", Path: "scripts/core_e2e_mainnet_runbook.ps1"},
				{Label: "core_recovery_workflow.ps1", Path: "scripts/core_recovery_workflow.ps1"},
				{Label: "corruption_long_soak_gate.ps1", Path: "scripts/corruption_long_soak_gate.ps1"},
				{Label: "mempool_stateful_core_gate.ps1", Path: "scripts/mempool_stateful_core_gate.ps1"},
				{Label: "core_reboottestnet_core_aligned_gate.ps1", Path: "scripts/core_reboottestnet_core_aligned_gate.ps1"},
				{Label: "core_reindex_prune_disruptive_workflow.ps1", Path: "scripts/core_reindex_prune_disruptive_workflow.ps1"},
				{Label: "ci_offline_gate.sh", Path: "scripts/ci_offline_gate.sh"},
				{Label: "ci_offline_gate.ps1", Path: "scripts/ci_offline_gate.ps1"},
				{Label: "offlinegate/suites.go", Path: "offlinegate/suites.go"},
				{Label: "offlinegate/bootstrap.go", Path: "offlinegate/bootstrap.go"},
				{Label: "ci_live_reboottestnet_gate.ps1", Path: "scripts/ci_live_reboottestnet_gate.ps1"},
				{Label: "ci_runner_preflight.ps1", Path: "scripts/ci_runner_preflight.ps1"},
				{Label: "dogego cert preflight", Path: "runner/preflight.go"},
				{Label: "dogego cert provision", Path: "runner/provision.go"},
				{Label: "dogego cert weekly", Path: "cmd/dogego/cert_weekly.go"},
				{Label: "dogego cert enable-weekly", Path: "runner/enable_weekly.go"},
				{Label: "ci_runner_provision_checklist.ps1", Path: "scripts/ci_runner_provision_checklist.ps1"},
				{Label: "setup_reboottestnet_core_parity.ps1", Path: "scripts/setup_reboottestnet_core_parity.ps1"},
				{Label: "dogego cert setup-parity", Path: "runner/setup_parity.go"},
				{Label: "ci_scheduled_weekly_live.ps1", Path: "scripts/ci_scheduled_weekly_live.ps1"},
				{Label: "dogego cert weekly-live", Path: "runner/weekly_live.go"},
				{Label: "ci_milestone_b_full_gate.ps1", Path: "scripts/ci_milestone_b_full_gate.ps1"},
				{Label: "dogego cert live-soak", Path: "runner/live_soak.go"},
				{Label: "dogego cert workflow10", Path: "runner/workflow10.go"},
				{Label: "ci_scheduled_corruption_soak.ps1", Path: "scripts/ci_scheduled_corruption_soak.ps1"},
				{Label: "gh_enable_scheduled_live.ps1", Path: "scripts/gh_enable_scheduled_live.ps1"},
				{Label: "core_mainnet_disruptive_reindex_gate.ps1", Path: "scripts/core_mainnet_disruptive_reindex_gate.ps1"},
				{Label: "core_restart_resume_check.ps1", Path: "scripts/core_restart_resume_check.ps1"},
				{Label: "core_reindex_prune_workflow.ps1", Path: "scripts/core_reindex_prune_workflow.ps1"},
				{Label: "core_restart_connect_check.ps1", Path: "scripts/core_restart_connect_check.ps1"},
				{Label: "core_wallet_workflow.ps1", Path: "scripts/core_wallet_workflow.ps1"},
				{Label: "core_bip152_probe.ps1", Path: "scripts/core_bip152_probe.ps1"},
				{Label: "bip152_live_soak_gate.ps1", Path: "scripts/bip152_live_soak_gate.ps1"},
				{Label: "bip152_timed_soak.ps1", Path: "scripts/bip152_timed_soak.ps1"},
				{Label: "wallet_import_cert.ps1", Path: "scripts/wallet_import_cert.ps1"},
				{Label: "dogego cert wallet-import", Path: "walletimport/verify.go"},
				{Label: "dogego cert wallet-migration", Path: "walletmigration/verify.go"},
				{Label: "dogego cert pq", Path: "pqcert/suites.go"},
				{Label: "pq_cert.ps1", Path: "scripts/pq_cert.ps1"},
				{Label: "ci_offline_gate.sh", Path: "scripts/ci_offline_gate.sh"},
				{Label: "wallet_import_cert.sh", Path: "scripts/wallet_import_cert.sh"},
				{Label: "wallet_migration_cert.sh", Path: "scripts/wallet_migration_cert.sh"},
				{Label: "field_evidence_cert.sh", Path: "scripts/field_evidence_cert.sh"},
				{Label: "operator_workflow_cert.sh", Path: "scripts/operator_workflow_cert.sh"},
				{Label: "cert_offline_prerequisites.ps1", Path: "scripts/cert_offline_prerequisites.ps1"},
				{Label: "cert_offline_prerequisites.sh", Path: "scripts/cert_offline_prerequisites.sh"},
				{Label: "pq_cert.sh", Path: "scripts/pq_cert.sh"},
				{Label: "wallet_migration_cert.ps1", Path: "scripts/wallet_migration_cert.ps1"},
				{Label: "wallet/pool_replay.go", Path: "wallet/pool_replay.go"},
				{Label: "provision_wallet_dat_fixture.ps1", Path: "scripts/provision_wallet_dat_fixture.ps1"},
				{Label: "CORE_SIDE_BY_SIDE_WORKFLOWS.md", Path: "docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md"},
			},
		},
	}
}
