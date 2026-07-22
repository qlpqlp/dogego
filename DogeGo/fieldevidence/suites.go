// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package fieldevidence holds the canonical Milestone A mainnet field-evidence
// certification suite, shared by dogego cert field-evidence and
// scripts/field_evidence_cert.ps1 so the two cannot drift.
package fieldevidence

import "strings"

// Suite is one go test gate in the field-evidence certification bundle.
type Suite struct {
	Name string
	Args []string
}

// DefaultSuites returns the offline mainnet field-evidence gates (no datadir, no RPC).
func DefaultSuites() []Suite {
	return []Suite{
		{
			Name: "field block harness",
			Args: []string{
				"test", "./consensus", "-run",
				"TestCoreMainnetFieldConnectCorpus|TestCoreMainnetFieldStoredBlockConnect|TestCoreMainnetFieldBlockHexVectors|TestCoreMainnetFieldBlockCheckBlock|TestCoreMainnetFieldMultiTxBlock15504|TestMainnetFieldMultiTxBlock15504Committed|TestCoreMainnetFieldCanonicalHeaderPoW|TestCoreMainnetFieldHeaderVectorsLegacyPoW|TestCoreMainnetCheckpointHeaderLegacyPoW|TestCoreMainnetCheckpointHeaderRejectCorpus|TestCoreMainnetFieldAuxpowHeaderCheckpoint|TestCoreMainnetFieldAuxpowOfflineValidate|TestCoreMainnetFieldSparseCoinbaseConnect|TestCoreMainnetCheckpointHeaderAccept|TestMainnetFieldBlock10006SubsidyCap|TestMainnetFieldBlocksBundledStoreRoundTrip|TestMainnetFieldBlocksBundledStoreContiguous|TestMainnetFieldBlocksBundledMeasureContiguous|TestMainnetFieldBlocksBundledValidateStoredBodies|TestMainnetFieldSparseBlocksBundledHashGet|TestCrashActiveBundledContiguous_MainnetFieldBlocks|TestMainnetFieldBlock10006BundledPutGet|TestMainnetFieldBlocksMatchCanonical|TestMainnetFieldHeadersMatchCanonical|TestMainnetFieldBlocksMatchBlockVectors|TestCrashActiveHeaderSegment_MainnetFieldHeaders|TestCrashActiveRawPut_MainnetFieldBlock10006|TestCoreDifferentialCorpusGate/mainnet_checkpoint_headers|TestCoreDifferentialCorpusGate/mainnet_field_block_vectors|TestCoreDifferentialCorpusGate/mainnet_field_auxpow_fixture",
				"-count=1",
			},
		},
		{
			Name: "field differential vectors",
			Args: []string{
				"test", "./consensus", "-run",
				"TestCoreHeaderDifferentialVectors/mainnet_field_header_|TestCoreHeaderDifferentialVectors/mainnet_checkpoint_104679|TestCoreHeaderDifferentialVectors/mainnet_checkpoint_145000|TestCoreHeaderDifferentialVectors/mainnet_checkpoint_371337|TestCoreBlockDifferentialVectors/mainnet_field_block_",
				"-count=1",
			},
		},
		{
			Name: "field corpus gate",
			Args: []string{"test", "./consensus", "-run", "TestCoreDifferentialCorpusGate/mainnet_field", "-count=1"},
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

// SuiteCommandLine returns the go test invocation string used in field_evidence_cert.ps1.
func SuiteCommandLine(s Suite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that field_evidence_cert.ps1 may use
// (PowerShell double-quotes the -run regex filter).
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
