// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package offlinegate

import "strings"

// Suite is one go test gate in the offline certification bundle.
type Suite struct {
	Name string
	Args []string
}

// DefaultSuites returns cross-platform offline gates shared by dogego cert offline
// and scripts/ci_offline_gate.{sh,ps1} (GitHub Actions push/PR job).
func DefaultSuites() []Suite {
	return []Suite{
		{Name: "docs and UI gates", Args: []string{"test", "./docs/...", "./ui/...", "-count=1"}},
		{Name: "stateful mempool coverage", Args: []string{"test", "./consensus", "-run", "TestStatefulMempool|TestEvalMempoolCorpus|TestCoreMempool|TestCoreMainnetFieldMultiTxBlock15504|TestProbeMainnetFieldDiskStatusDefaultPath", "-count=1", "-timeout", "5m"}},
		{Name: "RPC reindex and recovery", Args: []string{"test", "./rpc", "-run", "TestExecReindex|TestExecTruncate|TestExecDogegoRecover|TestExecVerifyChain|TestExecSaveMempoolIdempotent|TestExecLoadMempoolIdempotent|TestExecUpgradeTxIndexIdempotent|TestExecPruneBlockchainIdempotent", "-count=1", "-timeout", "120s"}},
		{Name: "wallet web fast path", Args: []string{"test", "./ui", "-run", "TestWallet|TestProbeCoreWallet|TestProbeCoreAddrman|TestProbeCoreMining|TestWalletTxHistoryDefer|TestApplyCoreOperatorCertAddrman|TestApplyCoreOperatorCertMining|TestEndToEndFromProbesAddrman|TestConnectLagCheckNoteBoost|TestMergeIBDProgressConnectCatchUpTuning|TestEndToEndFromProbesRestartResumeNote|TestApplyCoreOperatorCertRestartConnectNote|TestProbeCoreFieldEvidenceOfflineCorpus", "-count=1", "-timeout", "120s"}},
		{Name: "wallet RPC fast path", Args: []string{"test", "./rpc", "-run", "TestExecGetWalletInfoCompact|TestExecGetWalletInfoHistoryDefer|TestWalletUtxoImmature|TestWalletCollectTransactionsUILight|TestOperatorRPCErrorCodesGolden|TestLegacyBlockMempoolTxs|TestRebootTestnetMinedFixturesPresent|TestMergeDogegoRawSyncDiagnostics|TestExecGetBlockTemplate|TestExecCreateAuxBlock", "-count=1", "-short", "-timeout", "120s"}},
		{Name: "operator workflow certification", Args: []string{"test", "./node", "-run", "TestOperatorWorkflowStandaloneCertification|TestAddrBookChurnSoak|TestAddrBookSaveLoad|TestEclipseInboundPressureSoak|TestAttemptEvict|TestAcceptInboundOrEvict|TestConnectCatchUpPassesDeepBodyIBD|TestConnectCatchUpBlocksPerIBDCallDeepBody|TestSyncUtxoMaxConnectPassesDeepBacklog|TestEnrichIBDProgressSnapshotConnectCatchUpTuning", "-count=1", "-timeout", "5m"}},
		{Name: "store corruption recovery", Args: []string{"test", "./store", "-run", "TestBundledTornTail|TestProbeBundled|TestRepairTxIndex|TestPurgeStaleUtxoSnapshotTemps|TestLoadUtxoSnapshot_", "-count=1", "-timeout", "5m"}},
		{Name: "wallet migration fixtures", Args: []string{"test", "./wallet/bdb/...", "./wallet/corewallet/...", "./walletmigration/...", "-count=1", "-timeout", "120s"}},
		{Name: "wallet migration RPC", Args: []string{"test", "./rpc", "-run", "ImportWalletDat|ProbeWalletDat|ImportWalletMulti|ImportWalletCallsKeypool|ImportWalletDatCallsKeypool|SyntheticFixture|Berkeley|MultiPool|NativePool|MixedPool", "-count=1", "-timeout", "120s"}},
		{Name: "autostart package", Args: []string{"test", "./autostart/...", "-count=1"}},
		{Name: "founder preflight", Args: []string{"test", "./founder/...", "-count=1"}},
		{Name: "ibd convergence", Args: []string{"test", "./ibdconvergence/...", "-count=1"}},
		{Name: "runner provision", Args: []string{"test", "./runner/...", "-count=1"}},
	}
}

// SuiteCommandLine returns the go test invocation string used in ci_offline_gate scripts.
func SuiteCommandLine(s Suite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that ci_offline_gate scripts may use
// (bash/PowerShell single-quote the -run regex filter).
func SuiteScriptNeedles(s Suite) []string {
	tail := s.Args[1:]
	needles := []string{SuiteCommandLine(s)}
	for i := 0; i < len(tail); i++ {
		if tail[i] != "-run" || i+1 >= len(tail) {
			continue
		}
		quoted := append([]string(nil), tail...)
		quoted[i+1] = "'" + tail[i+1] + "'"
		needles = append(needles, "go test "+strings.Join(quoted, " "))
	}
	return needles
}

// Needles returns substrings that ci_offline_gate scripts must contain (docs guard).
func Needles() []string {
	return []string{
		"TestWallet",
		"TestProbeCoreWallet",
		"TestProbeCoreAddrman",
		"TestProbeCoreMining",
		"TestWalletTxHistoryDefer",
		"TestWalletAPIEnvelopeHistoryDeferred",
		"TestExecGetWalletInfoHistoryDefer",
		"TestWalletAPIEnvelopeHistoryDeferred",
		"TestSummaryWalletHistoryDeferred",
		"TestProbeCoreWalletHistoryDeferred",
		"TestApplyCoreOperatorCertAddrman",
		"TestApplyCoreOperatorCertMining",
		"TestEndToEndFromProbesAddrman",
		"TestExecGetWalletInfoCompact",
		"TestWalletCollectTransactionsUILight",
		"TestOperatorRPCErrorCodesGolden",
		"TestExecGetBlockTemplate",
		"TestExecCreateAuxBlock",
		"TestConnectCatchUpPassesDeepBodyIBD",
		"TestEnrichIBDProgressSnapshotConnectCatchUpTuning",
		"TestCoreMainnetFieldMultiTxBlock15504",
		"TestEndToEndFromProbesRestartResumeNote",
		"TestApplyCoreOperatorCertRestartConnectNote",
		"TestProbeCoreFieldEvidenceOfflineCorpus",
		"walletmigration",
		"SyntheticFixture",
		"ImportWalletDat",
		"ImportWalletMulti",
		"ImportWalletCallsKeypool",
		"ImportWalletDatCallsKeypool",
		"./autostart/",
		"./founder/",
		"./ibdconvergence/",
		"./runner/",
	}
}
