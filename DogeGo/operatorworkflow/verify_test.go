// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package operatorworkflow

import (
	"strings"
	"testing"
)

func TestDefaultCoreSuitesNonEmpty(t *testing.T) {
	s := DefaultCoreSuites()
	if len(s) != 6 {
		t.Fatalf("suites=%d", len(s))
	}
}

func TestSuiteCommandLineShape(t *testing.T) {
	for _, s := range DefaultCoreSuites() {
		line := SuiteCommandLine(s)
		if !strings.HasPrefix(line, "go test ") {
			t.Fatalf("suite %q line=%q", s.Name, line)
		}
	}
}

func TestDefaultCoreSuitesCoverOperatorWorkflow(t *testing.T) {
	var all string
	for _, s := range DefaultCoreSuites() {
		all += RunRegex(s) + "\n"
	}
	for _, needle := range []string{
		"TestOperatorWorkflowStandaloneCertification",
		"TestCore(Block|Header|Script|Mempool)Differential",
		"TestEvalMempoolCorpus",
	} {
		if !strings.Contains(all, needle) {
			t.Fatalf("DefaultCoreSuites missing %q", needle)
		}
	}
}
