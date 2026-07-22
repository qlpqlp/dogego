// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestZKPGWireRoundTrip(t *testing.T) {
	pi := make([][]byte, 2)
	for i := range pi {
		pi[i] = make([]byte, 32)
		pi[i][0] = byte(i + 1)
	}
	proof := make([]byte, 64)
	for i := range proof {
		proof[i] = 0xab
	}
	var wireBlob []byte
	wireBlob = append(wireBlob, []byte(groth16WireMagic)...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(proof)))
	wireBlob = append(wireBlob, n[:]...)
	binary.LittleEndian.PutUint32(n[:], uint32(len(pi)))
	wireBlob = append(wireBlob, n[:]...)
	wireBlob = append(wireBlob, proof...)
	for _, p := range pi {
		wireBlob = append(wireBlob, p...)
	}
	if err := parseGroth16Wire(wireBlob, pi, nil); err != nil {
		t.Fatal(err)
	}
}

func TestHandleP2PGetZKProof(t *testing.T) {
	ext, err := NewExtension(DefaultManifest())
	if err != nil {
		t.Fatal(err)
	}
	e := ext.(*Extension)
	dir := t.TempDir()
	host := &fakeHost{dir: dir, network: "testnet"}
	if err := e.OnEnable(nil, host); err != nil {
		t.Fatal(err)
	}
	defer e.OnDisable()
	p := Proof{
		TransactionID: strings.Repeat("a", 64),
		BlockHash:     strings.Repeat("b", 64),
		BlockHeight:   1,
		ProofData:     strings.Repeat("c", 64),
		PublicInputs:  []string{strings.Repeat("d", 64)},
	}
	p, _ = NormalizeProof(p)
	_ = VerifyProof(p)
	_ = e.store.PutProof(p)
	req := EncodeGetZKProof([]string{p.ProofHash})
	var replied string
	err = e.HandleP2P(CmdGetZKProof, req, "peer1", func(cmd string, payload []byte) error {
		replied = cmd
		if cmd != CmdZKProof {
			t.Fatalf("cmd %q", cmd)
		}
		proofs, err := DecodeZKProof(payload)
		if err != nil || len(proofs) != 1 {
			t.Fatalf("proofs %#v err %v", proofs, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if replied == "" {
		t.Fatal("expected zkproof reply")
	}
}

func TestHandleP2PZKInvRequestsMissing(t *testing.T) {
	ext, _ := NewExtension(DefaultManifest())
	e := ext.(*Extension)
	dir := t.TempDir()
	host := &fakeHost{dir: dir, network: "testnet"}
	_ = e.OnEnable(nil, host)
	defer e.OnDisable()
	unknown := strings.Repeat("f", 64)
	inv := EncodeZKInv([]string{unknown})
	var gotCmd string
	_ = e.HandleP2P(CmdZKInv, inv, "peer2", func(cmd string, payload []byte) error {
		gotCmd = cmd
		return nil
	})
	if gotCmd != CmdGetZKProof {
		t.Fatalf("want getzkproof got %q", gotCmd)
	}
}
