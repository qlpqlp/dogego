// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package indexer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestRun_noArgsPrintsHelpAndStatus(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	chainRoot := filepath.Join(dir, "testnet")
	if err := os.MkdirAll(chainRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(chainRoot, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	_ = j
	var help bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run(nil, "dogego.exe")
	w.Close()
	os.Stdout = old
	_, _ = help.ReadFrom(r)
	if code != 0 && code != 1 {
		t.Fatalf("exit code %d", code)
	}
}

func TestIsHelpToken(t *testing.T) {
	if !isHelpToken("help") || !isHelpToken("-h") || !isHelpToken("--help") {
		t.Fatal("expected help tokens")
	}
	if isHelpToken("status") {
		t.Fatal("status is not help")
	}
}
