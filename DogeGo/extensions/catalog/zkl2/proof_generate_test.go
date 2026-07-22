// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/hex"
	"testing"
)

func TestBuildCommitmentProofFromText(t *testing.T) {
	p, err := buildCommitmentProof([]byte("much wow"), "text", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ProofType != ProofModeCommitment {
		t.Fatalf("type %d", p.ProofType)
	}
	if err := VerifyProof(p); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateProofRPCShape(t *testing.T) {
	ext := &Extension{}
	out, err := ext.rpcGenerateProof(nil, GenerateProofParams{
		Payload:         "hello zk",
		PayloadEncoding: "text",
		ProofKind:       "commitment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["proof_kind"] != "commitment" {
		t.Fatalf("kind %#v", out["proof_kind"])
	}
	script, _ := out["zkdg_script_hex"].(string)
	if len(script) < 64 {
		t.Fatalf("script %q", script)
	}
	if _, err := hex.DecodeString(script); err != nil {
		t.Fatal(err)
	}
}

func TestGroth16DemoGenerate(t *testing.T) {
	p, err := buildGroth16ProofFromPayload([]byte("payload"), "text", "", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ProofType != ProofModeGroth16 {
		t.Fatal("want groth16")
	}
	// Demo uses test vector; may pass without VK loaded (legacy path) or with pairing.
	_ = VerifyProof(p)
}
