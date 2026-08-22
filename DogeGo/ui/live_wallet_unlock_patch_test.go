// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestPatchWalletEncryptionStatus(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Encrypt("unlock-ui-pass"); err != nil {
		t.Fatal(err)
	}
	sum := map[string]any{"tip_height": 1}
	attachWalletEncryptionStatus(sum, w)
	sumB, _ := json.Marshal(sum)
	livePayload, _ := json.Marshal(map[string]any{"ok": true, "summary": sum})
	f := &LiveFeed{summaryJSON: sumB, liveJSON: livePayload}
	storeJSONAtomic(&f.summaryAtomic, sumB)
	storeJSONAtomic(&f.liveAtomic, livePayload)

	if err := w.Unlock("unlock-ui-pass", 600); err != nil {
		t.Fatal(err)
	}
	f.PatchWalletEncryptionStatus(w)

	var got map[string]any
	if err := json.Unmarshal(f.summaryJSON, &got); err != nil {
		t.Fatal(err)
	}
	if got["wallet_unlocked"] != true {
		t.Fatalf("summary wallet_unlocked=%#v", got["wallet_unlocked"])
	}
	if got["wallet_private_keys_enabled"] != true {
		t.Fatalf("summary private_keys=%#v", got["wallet_private_keys_enabled"])
	}
	if got["tip_height"] != float64(1) {
		t.Fatalf("tip_height clobbered: %#v", got["tip_height"])
	}
}
