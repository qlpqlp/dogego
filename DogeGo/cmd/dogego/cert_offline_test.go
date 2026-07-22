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

	"dogego/offlinegate"
)

func TestCertOfflineWiredToOfflinegate(t *testing.T) {
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
		if !strings.Contains(text, "offlinegate.RunBootstrap") {
			t.Fatalf("%s missing offlinegate.RunBootstrap", name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", "cert.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "offlinegate.DefaultSuites()") {
		t.Fatal("cert.go missing offlinegate.DefaultSuites()")
	}
}

func TestCertOfflineSuiteCountMatchesPackage(t *testing.T) {
	if len(offlinegate.DefaultSuites()) < 10 {
		t.Fatalf("offlinegate suites=%d", len(offlinegate.DefaultSuites()))
	}
}
