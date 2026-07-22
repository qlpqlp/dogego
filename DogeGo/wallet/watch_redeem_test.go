// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestWatchRedeemPersisted(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	redeem := []byte{0x51, 0x52, 0xae}
	h := chain.Hash160(redeem)
	var h160 [20]byte
	copy(h160[:], h)
	p2sh := chain.P2SHScriptFromScriptHash(h160)
	if err := w.AddWatchScript(p2sh); err != nil {
		t.Fatal(err)
	}
	if err := w.SetWatchRedeem(p2sh, redeem); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	got := w2.WatchRedeemScript(p2sh)
	if !bytesEq(got, redeem) {
		t.Fatalf("got %x", got)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) || !containsStr(string(b), "watch_redeems") {
		t.Fatalf("wallet json: %s", b)
	}
}

func bytesEq(a, b []byte) bool {
	return len(a) == len(b) && string(a) == string(b)
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || findSub(s, sub))
}

func findSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
