// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"testing"

	"dogego/chain"
	"dogego/consensus"
)

func TestScriptPubKeyDecodePQCommitment(t *testing.T) {
	commit := make([]byte, 32)
	for i := range commit {
		commit[i] = byte(i)
	}
	script, err := consensus.BuildPQCommitmentScript(consensus.PQTagFalcon, commit)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	out := scriptPubKeyDecode(script, p)
	if out["type"] != "nulldata" {
		t.Fatalf("type=%v", out["type"])
	}
	if out["dogego_pqc_tag"] != consensus.PQTagFalcon {
		t.Fatalf("tag=%v", out["dogego_pqc_tag"])
	}
	if out["dogego_pqc_scheme"] != "falcon-512" {
		t.Fatalf("scheme=%v", out["dogego_pqc_scheme"])
	}
	wantHex := hex.EncodeToString(commit)
	if out["dogego_pqc_commitment"] != wantHex {
		t.Fatalf("commit=%v want %s", out["dogego_pqc_commitment"], wantHex)
	}
}
