// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package pqcert holds the canonical offline post-quantum format/carrier
// certification suite. It does not certify production PQ safety.
package pqcert

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Suite is one go test gate in the PQ certification bundle.
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

// DefaultSuites returns offline tests for the current PQ format/carrier scope.
func DefaultSuites() []Suite {
	return []Suite{
		{Name: "consensus PQ commitment/carrier", Args: []string{"test", "./consensus", "-run", "TestPQCommitment|TestBuildPQCommitment|TestDetectPQCommitment|TestVerifyPQCommitment|TestPQCarrier|TestVerifyPQCarrierPair|TestMempoolAdmissionAcceptsPQ", "-count=1"}},
		{Name: "rpc PQ commitment/carrier", Args: []string{"test", "./rpc", "-run", "TestExecDogegoVerifyPQCommitment|TestScriptPubKeyDecodePQCommitment|TestDogegoPQCarrier|TestExecDogegoVerifyPQCarrier|TestSetWalletFlagPqCommitments|TestPeelPQCommitRequiresFlag|TestWalletPQTagFromTxHex|TestWalletEnrichTxKindSentPQ", "-count=1"}},
		{Name: "pqcrypto PQ backends", Args: []string{"test", "./pqcrypto", "-run", "TestFalconRoundTrip|TestDilithiumRoundTrip|TestRaccoonRoundTrip", "-count=1"}},
		{Name: "wallet PQ key material", Args: []string{"test", "./wallet", "-run", "TestEnsurePQReadyAndNextCommitment", "-count=1"}},
		{Name: "ui PQ history classification", Args: []string{"test", "./ui", "-run", "TestWalletTxsHTTPQuantumBypassesUtxoFastPath|TestProbeCorePQOK", "-count=1"}},
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

// SuiteCommandLine returns the go test invocation string for this suite.
func SuiteCommandLine(s Suite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that pq_cert.ps1 may use when inlining suites.
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

// RunOffline executes PQ certification suites from module root.
func RunOffline(root string) error {
	for _, s := range DefaultSuites() {
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
