// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindGoModuleRootFromDogeGo(t *testing.T) {
	root, err := findGoModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod under %q: %v", root, err)
	}
}

func TestEmitWalletMigrationReportJSON(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	outCh := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		outCh <- b
	}()

	emitWalletMigrationReport(true, map[string]any{
		"ok":      true,
		"offline": "passed",
	})
	w.Close()

	var decoded map[string]any
	if err := json.Unmarshal(<-outCh, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["ok"] != true || decoded["offline"] != "passed" {
		t.Fatalf("decoded=%v", decoded)
	}
}

func TestCertWalletMigrationWiredToPackage(t *testing.T) {
	root, err := findGoModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "dogego", "cert_wallet_migration.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"walletmigration.DefaultOfflineSuites()",
		"walletmigration.RunOffline",
		"walletmigration.WalletDatProbeOptional",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("cert_wallet_migration.go missing %q", needle)
		}
	}
}
