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
	"strings"
	"testing"

	"dogego/chain"
	"dogego/consensus"
)

func TestEncryptedWalletHidesPQKeysOnDisk(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.EnsurePQReady(); err != nil {
		t.Fatal(err)
	}
	tag, _, err := w.NextPQCommitment()
	if err != nil || tag != consensus.PQTagFalcon {
		t.Fatalf("pq: tag=%s err=%v", tag, err)
	}
	_, _, _, err = w.PQCarrierKeyMaterial("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Encrypt("hunter2"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, `"sk_hex"`) || strings.Contains(body, `"pq_commit_seed_hex"`) {
		t.Fatalf("pq material leaked in encrypted wallet.json: %s", body)
	}
	if err := w.Unlock("hunter2", 0); err != nil {
		t.Fatal(err)
	}
	tag2, _, err := w.NextPQCommitment()
	if err != nil || tag2 == "" {
		t.Fatalf("pq after unlock: %v", err)
	}
	raw2, _ := os.ReadFile(path)
	if strings.Contains(string(raw2), `"sk_hex"`) {
		t.Fatalf("pq sk leaked after unlock save: %s", string(raw2))
	}
	var df diskFile
	if err := json.Unmarshal(raw2, &df); err != nil {
		t.Fatal(err)
	}
	if df.PrivKeyHex != "" || df.HDSeedHex != "" {
		t.Fatal("spend keys leaked")
	}
}

func TestMainnetRequiresEncryptionPolicy(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	path := filepath.Join(t.TempDir(), "wallet.json")
	w, _ := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err := w.RequireMainnetEncryption("mainnet"); err != ErrMainnetUnencrypted {
		t.Fatalf("mainnet plaintext: %v", err)
	}
	if err := w.RequireMainnetEncryption("testnet"); err != nil {
		t.Fatalf("testnet should allow: %v", err)
	}
}
