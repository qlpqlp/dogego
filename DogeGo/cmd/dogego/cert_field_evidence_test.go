// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/fieldevidence"
)

func TestCertFieldEvidenceWired(t *testing.T) {
	root, err := findGoModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cert.go", "cert_field_evidence.go"} {
		raw, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "field-evidence") {
			t.Fatalf("%s missing field-evidence wiring", name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", "cert_field_evidence.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "fieldevidence.DefaultSuites()") {
		t.Fatal("cert_field_evidence.go missing fieldevidence.DefaultSuites()")
	}
}

func TestCertFieldEvidenceSuiteNeedles(t *testing.T) {
	all := ""
	for _, s := range fieldevidence.DefaultSuites() {
		all += fieldevidence.SuiteCommandLine(s) + "\n"
	}
	for _, needle := range []string{
		"TestCoreMainnetFieldMultiTxBlock15504",
		"TestMainnetFieldMultiTxBlock15504Committed",
		"TestCoreDifferentialCorpusGate/mainnet_field",
	} {
		if !strings.Contains(all, needle) {
			t.Fatalf("fieldevidence.DefaultSuites missing %q", needle)
		}
	}
}
