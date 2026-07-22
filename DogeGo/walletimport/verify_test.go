// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletimport

import (
	"strings"
	"testing"
)

func TestDefaultOfflineSuitesNonEmpty(t *testing.T) {
	s := DefaultOfflineSuites()
	if len(s) != 5 {
		t.Fatalf("suites=%d", len(s))
	}
}

func TestSuiteCommandLineShape(t *testing.T) {
	for _, s := range DefaultOfflineSuites() {
		line := SuiteCommandLine(s)
		if !strings.HasPrefix(line, "go test ") {
			t.Fatalf("suite %q line=%q", s.Name, line)
		}
	}
}

func TestSuiteScriptNeedlesCoverMixedPool(t *testing.T) {
	found := false
	for _, s := range DefaultOfflineSuites() {
		if !strings.Contains(RunRegex(s), "MixedPool") {
			continue
		}
		for _, needle := range SuiteScriptNeedles(s) {
			if strings.Contains(needle, "MixedPool") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected MixedPool in rpc suite needles")
	}
}
