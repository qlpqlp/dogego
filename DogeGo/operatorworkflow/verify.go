// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package operatorworkflow holds the canonical Milestone E standalone operator
// certification suite, shared by dogego cert operator and scripts/operator_workflow_cert.ps1.
package operatorworkflow

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"dogego/fieldevidence"
	"dogego/walletimport"
)

// Suite is one go test gate in the operator workflow certification bundle.
type Suite struct {
	Name string
	Args []string
}

var (
	testOut io.Writer = os.Stdout
	testErr io.Writer = os.Stderr
)

// SetOutput redirects go test stdout/stderr (for CLI/tests).
func SetOutput(stdout, stderr io.Writer) {
	if stdout != nil {
		testOut = stdout
	}
	if stderr != nil {
		testErr = stderr
	}
}

// DefaultCoreSuites returns cross-platform offline core gates (mirrors operator_workflow_cert.ps1).
func DefaultCoreSuites() []Suite {
	return []Suite{
		{
			Name: "consensus policy/mempool",
			Args: []string{
				"test", "./consensus", "-run",
				"Core(Mempool|Script|Block|Header|Difficulty)|TestPolicyVsConsensus|TestCoreMempoolVectorTemplatesCovered|TestEvalMempoolCorpus|TestCheckSpendConflictsReturnsRBF|TestMempoolParityRPCFixtureMatchesBuilders|TestCheckAuxPowAcceptsNonDogecoin|TestCheckAuxPowRejectsParentDogecoin|TestPrevHeightsForTx|TestUtxoPrevOutView|TestUtxoHeightFromView",
				"-count=1",
			},
		},
		{
			Name: "store corruption/recovery",
			Args: []string{
				"test", "./store", "-run",
				"TestUtxoSerializedHash|TestUtxoSerializedHashReorgStress|TestRepairTxIndexFromRawAfterIndexLoss|TestRepairTxIndexIfLag|TestTxIndexSparse|TestRepairTxIndexIfSparse|TestRepairTxIndexIfSparseAtHeightZero|TestUtxoCacheUnspentHeight|TestOpenHeaderJournalRepairsPartialTail|TestHeaderSegment|TestHeaderSegmentReload|TestMigrateMonolith|TestPurgeStaleRawBlockTemps|TestPurgeStaleBlockFilterTemps|TestPurgeStaleTxIndexTemps|TestCrashActive|TestCrashKillBeforeRawPutRename|TestSubprocessKillDuringRawPut|TestSubprocessKillDuringHeaderSegmentAppend|TestSubprocessKillDuringBlockFilterPut|TestSubprocessKillDuringTxIndexWrite|TestHeaderAuxJournalRepairsTornTail|TestProcessLock|TestRawBlockStoreBundledFourSequential|TestPickBundledAppendSlotAdvancesOffset|TestProbeBundledContiguousTipStopsAtTruncatedRecord|TestBundledTornTailReopenConvergence|TestBundledTornTailReconcilesInflatedCheckpoint|TestProbeBundledContiguousTipReconcilesInflatedCheckpoint",
				"-count=1",
			},
		},
		{
			Name: "node operator workflow",
			Args: []string{
				"test", "./node", "-run",
				"TestOperatorWorkflowStandaloneCertification|TestConnectCatchUp|TestConnectFrontier|TestIBDConnect|TestConnectPrevHeights|TestRepairTxIndexRestoresFundingTxHeight|TestMaybeRepairTxIndexDetectsSparseIndex|TestMaybeRepairTxIndexOnConnect|TestBlockStoreChainDir|TestConnectErrNeedsTxIndexRepair|TestShouldRotatePeerForStubBlock|TestFinishBatchStubPurge|TestTruncateChainReorgUtxoStress|TestMaxAlternateForkWork|TestEnsureLocalGenesis|TestShouldRedial|TestRecoverablePrimary|TestPickBlockPrimary|TestBlockPeerScoresPersist|TestMergeDiscoveryCandidatesArchival|TestIbdStallRecoverIntervalGenesis|TestIbdStallRecoverIntervalConnectCaughtUp|TestNoteStubBlockHeavyPenalty|TestAutoRecoverSweepEnsuresLocalGenesis|TestAutoRecoverHeaders|TestAutoRecoverPostRewind|TestResetInFlightForHeaderRewind|TestCrashActivePutRestartConvergence|TestCrashKillBeforeRawPutSweepConvergence|TestCrashKillHeaderSegmentSweepConvergence|TestCrashIndexFilterTmpSweepRecovery|TestCrashHeaderAuxTornTailSweepRecovery|TestCrashHeaders|TestCrashHeaderSegments|TestCrashRaw|TestCrashRawBlockMidWrite|TestCrashBlockFilterMidWrite|TestStartupRecoveryConvergence|TestAutoRecoverPurge|TestMaybeRewindOnBadNBitsEachRewind|TestBadNBitsRecoveryDecision|TestMaybeResetStuckAncient|TestAutoRecoverSweepResetsStuckAncient|TestRecordWatchdog|PostAux|HeaderSyncHint|TestShouldAutoRecover|TestMaybeClampBundledContiguousFromDiskAfterTornTail|TestAutoRecoverSweepClampsBundledContiguousAfterTornTail",
				"-count=1",
			},
		},
		{
			Name: "rpc operator workflow",
			Args: []string{
				"test", "./rpc", "-run",
				"TestExecTruncateToHeight|TestExecDogegoRecoverHeaders|TestExecVerifyChainLevel4|TestExecGetTxOutSetInfo|TestExecTestMempoolAcceptDifferential|TestExecReindexTxRebuildsIndex|TestExecReindexBlockFiltersRebuilds|TestHandlerGetBlockchainInfoAuxpow|TestHeaderSyncDiagnostics",
				"-count=1", "-timeout", "120s",
			},
		},
		{
			Name: "consensus script/field",
			Args: []string{
				"test", "./consensus", "-run",
				"TestCoreSighashDifferentialHarness|TestCoreScriptTestsJSONCatalog|TestCoreScriptTestsRunnerSubset|TestCoreScriptTestsCHECKSIGVectors|TestScriptTestChecksigRoundtrip|TestVerifyScriptEval|TestEvalCheckLockTimeVerify|TestEvalCheckSequenceVerify|TestParseScriptASM|TestCoreMainnetField|TestMainnetFieldBlocksMatch|TestMainnetFieldHeadersMatch|TestCoreMainnetFieldSparse|TestCrashActiveHeaderSegment_MainnetFieldHeaders|TestCrashActiveRawPut_MainnetFieldBlock10006|TestCoreDifferentialCorpusGate",
				"-count=1",
			},
		},
		{
			Name: "consensus differential harness",
			Args: []string{
				"test", "./consensus", "-run",
				"TestCore(Block|Header|Script|Mempool)Differential|TestCoreMempoolVectorTemplatesCovered|TestCoreScriptTestsWitnessRowsIntentionallyDeclined",
				"-count=1", "-timeout", "15m",
			},
		},
	}
}

// RunRegex returns the -run filter for a suite, or "" if the suite has none.
func RunRegex(s Suite) string {
	for i := 0; i < len(s.Args); i++ {
		if s.Args[i] == "-run" && i+1 < len(s.Args) {
			return s.Args[i+1]
		}
	}
	return ""
}

// SuiteCommandLine returns the go test invocation string used in operator_workflow_cert.ps1.
func SuiteCommandLine(s Suite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that operator_workflow_cert.ps1 may use.
func SuiteScriptNeedles(s Suite) []string {
	tail := s.Args[1:]
	needles := []string{SuiteCommandLine(s)}
	for i := 0; i < len(tail); i++ {
		if tail[i] != "-run" || i+1 >= len(tail) {
			continue
		}
		quoted := append([]string(nil), tail...)
		quoted[i+1] = `""` + tail[i+1] + `""`
		needles = append(needles, "go test "+strings.Join(quoted, " "))
	}
	return needles
}

// RunCoreSuites executes the six core operator workflow go test gates.
func RunCoreSuites(root string) error {
	for _, s := range DefaultCoreSuites() {
		cmd := exec.Command("go", s.Args...)
		cmd.Dir = root
		cmd.Stdout = testOut
		cmd.Stderr = testErr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	return nil
}

// RunOffline executes full standalone operator certification (core + field evidence + wallet import).
func RunOffline(root string, skipFieldEvidence, skipWalletImport bool) error {
	if err := RunCoreSuites(root); err != nil {
		return err
	}
	if !skipFieldEvidence {
		if err := fieldevidence.RunOffline(root, testOut, testErr); err != nil {
			return fmt.Errorf("field-evidence: %w", err)
		}
	}
	if !skipWalletImport {
		walletimport.SetOutput(testOut, testErr)
		if err := walletimport.RunOffline(root); err != nil {
			return fmt.Errorf("wallet-import: %w", err)
		}
	}
	return nil
}
