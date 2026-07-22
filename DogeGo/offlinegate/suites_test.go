// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package offlinegate

import (
	"strings"
	"testing"
)

func TestDefaultSuitesNonEmpty(t *testing.T) {
	s := DefaultSuites()
	if len(s) < 10 {
		t.Fatalf("suites=%d", len(s))
	}
}

func TestNeedlesNonEmpty(t *testing.T) {
	if len(Needles()) < 10 {
		t.Fatal("expected needles")
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

func TestBootstrapCommandLine(t *testing.T) {
	line := BootstrapCommandLine()
	if !strings.Contains(line, bootstrapTest) {
		t.Fatalf("bootstrap line=%q", line)
	}
	if !strings.HasPrefix(line, "go test ") {
		t.Fatalf("bootstrap line=%q", line)
	}
}
