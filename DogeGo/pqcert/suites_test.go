// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcert

import (
	"strings"
	"testing"
)

func TestDefaultSuitesNonEmpty(t *testing.T) {
	s := DefaultSuites()
	if len(s) != 5 {
		t.Fatalf("suites=%d", len(s))
	}
}

func TestDefaultSuitesCoverCanonicalNeedles(t *testing.T) {
	var all string
	for _, s := range DefaultSuites() {
		all += RunRegex(s) + "\n"
	}
	for _, needle := range []string{
		"TestPQCommitment",
		"TestMempoolAdmissionAcceptsPQ",
		"TestDogegoPQCarrier",
		"TestEnsurePQReadyAndNextCommitment",
		"TestWalletTxsHTTPQuantumBypassesUtxoFastPath",
	} {
		if !strings.Contains(all, needle) {
			t.Fatalf("DefaultSuites missing %q", needle)
		}
	}
}

func TestSuiteCommandLineShape(t *testing.T) {
	for _, s := range DefaultSuites() {
		line := SuiteCommandLine(s)
		if !strings.HasPrefix(line, "go test ") {
			t.Fatalf("suite %q line=%q", s.Name, line)
		}
	}
}
