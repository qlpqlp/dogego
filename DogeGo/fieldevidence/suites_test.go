// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package fieldevidence

import (
	"strings"
	"testing"
)

func TestDefaultSuitesNonEmpty(t *testing.T) {
	s := DefaultSuites()
	if len(s) != 3 {
		t.Fatalf("expected 3 field-evidence suites, got %d", len(s))
	}
	for _, suite := range s {
		if suite.Name == "" {
			t.Fatal("suite missing name")
		}
		if len(suite.Args) < 2 || suite.Args[0] != "test" {
			t.Fatalf("suite %q has malformed args %v", suite.Name, suite.Args)
		}
	}
}

func TestDefaultSuitesCoverCanonicalNeedles(t *testing.T) {
	var all string
	for _, s := range DefaultSuites() {
		all += SuiteCommandLine(s) + "\n"
	}
	for _, needle := range []string{
		"TestCoreMainnetFieldMultiTxBlock15504",
		"TestMainnetFieldMultiTxBlock15504Committed",
		"TestCoreDifferentialCorpusGate/mainnet_field",
	} {
		if !strings.Contains(all, needle) {
			t.Fatalf("DefaultSuites missing canonical needle %q", needle)
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
