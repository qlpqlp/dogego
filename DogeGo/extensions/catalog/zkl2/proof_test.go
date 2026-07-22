// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestProofHashAndCommitment(t *testing.T) {
	p := Proof{
		TransactionID:    strings.Repeat("a", 64),
		BlockHash:        strings.Repeat("b", 64),
		BlockHeight:      100,
		TransactionIndex: 1,
		ProofData:        hex.EncodeToString(make([]byte, 64)),
		PublicInputs:     []string{strings.Repeat("c", 64)},
	}
	p, err := NormalizeProof(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateProofStructure(p); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProof(p); err != nil {
		t.Fatal(err)
	}
	commit, err := ProofCommitment(p.BlockHash, p.TransactionID, p.ProofHash)
	if err != nil || len(commit) != 64 {
		t.Fatalf("commit %q err %v", commit, err)
	}
}

func TestProofRootSortedByTxid(t *testing.T) {
	mk := func(txid string) Proof {
		p := Proof{
			TransactionID: txid,
			BlockHash:     strings.Repeat("d", 64),
			BlockHeight:   1,
			ProofData:     hex.EncodeToString([]byte(txid)),
			PublicInputs:  []string{strings.Repeat("e", 64)},
		}
		p, _ = NormalizeProof(p)
		return p
	}
	r1, err := ComputeProofRoot([]Proof{mk(strings.Repeat("2", 64)), mk(strings.Repeat("1", 64))})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ComputeProofRoot([]Proof{mk(strings.Repeat("1", 64)), mk(strings.Repeat("2", 64))})
	if err != nil {
		t.Fatal(err)
	}
	if r1 != r2 {
		t.Fatalf("roots differ %s vs %s", r1, r2)
	}
}

func TestCheckZKPRequiresPublicInputs(t *testing.T) {
	err := VerifyCheckZKP(CheckZKPModeGroth16, make([]byte, 64), nil)
	if err == nil {
		t.Fatal("expected error without public inputs")
	}
}
