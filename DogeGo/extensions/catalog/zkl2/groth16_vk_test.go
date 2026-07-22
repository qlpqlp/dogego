// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJoinDIPVKChunksRoundTrip(t *testing.T) {
	vk := make([]byte, groth16DIPVKLen)
	for i := range vk {
		vk[i] = byte(i)
	}
	chunks, err := SplitDIPVKChunks(vk)
	if err != nil {
		t.Fatal(err)
	}
	got, err := JoinDIPVKChunks(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(vk) {
		t.Fatal("join round trip mismatch")
	}
}

func TestResolveVerifyingKeyChunksHex(t *testing.T) {
	vk := make([]byte, groth16DIPVKLen)
	chunks, err := SplitDIPVKChunks(vk)
	if err != nil {
		t.Fatal(err)
	}
	hexChunks := make([]string, len(chunks))
	for i, ch := range chunks {
		hexChunks[i] = hex.EncodeToString(ch)
	}
	got, err := resolveVerifyingKey("", hexChunks)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != groth16DIPVKLen {
		t.Fatalf("len %d", len(got))
	}
}

func TestVerifyProofInlineVerifyingKey(t *testing.T) {
	vk, proof, inputs := groth16TestVector1()
	wireBlob := buildZKPGWire(proof, [][]byte{inputs})
	p := Proof{
		TransactionID: strings.Repeat("a", 64),
		BlockHash:     strings.Repeat("b", 64),
		BlockHeight:   100,
		ProofData:     hex.EncodeToString(wireBlob),
		PublicInputs:  []string{hex.EncodeToString(inputs)},
		VerifyingKey:  hex.EncodeToString(vk),
	}
	p, err := NormalizeProof(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyProof(p); err != nil {
		t.Fatal(err)
	}
}

func TestLoadedVKSummary(t *testing.T) {
	dir := t.TempDir()
	vkDir := filepath.Join(dir, "vk")
	if err := os.MkdirAll(vkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vk := make([]byte, 432) // 1 public input + 8 header points
	if err := os.WriteFile(filepath.Join(vkDir, "default.vk"), vk, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadVKDir(vkDir); err != nil {
		t.Fatal(err)
	}
	sum := LoadedVKSummary()
	if sum["loaded"] != true {
		t.Fatalf("loaded %#v", sum)
	}
	if sum["pairing_enabled"] != true {
		t.Fatal("expected pairing enabled")
	}
}

func TestRPCInfoIncludesGroth16(t *testing.T) {
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
	out, err := e.HandleRPC("info", nil, host)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("info %#v", out)
	}
	if _, ok := m["groth16"]; !ok {
		t.Fatal("missing groth16 summary")
	}
}
