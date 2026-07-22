// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// Vectors from esuwu/groth16-verifier-bls12381 (1 public input).
func TestGroth16CompressedPairingValid(t *testing.T) {
	vk, proof, inputs := groth16TestVector1()
	if len(proof) != groth16CompressedProofLen {
		t.Fatalf("proof len %d", len(proof))
	}
	if err := verifyGroth16Compressed(vk, proof, [][]byte{inputs}); err != nil {
		t.Fatal(err)
	}
}

func TestGroth16CompressedPairingInvalid(t *testing.T) {
	vk, proof, _ := groth16TestVector1()
	badInputs, _ := base64.StdEncoding.DecodeString("cmzVCcRVnckw3QUPhmG4Bkppeg4K50oDQwQ9EH+Fq1s=")
	if err := verifyGroth16Compressed(vk, proof, [][]byte{badInputs}); err == nil {
		t.Fatal("expected pairing failure")
	}
}

func TestLoadVKDirAndZKPGWirePairing(t *testing.T) {
	vk, proof, inputs := groth16TestVector1()
	dir := t.TempDir()
	vkDir := filepath.Join(dir, "vk")
	if err := os.MkdirAll(vkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vkDir, "default.vk"), vk, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadVKDir(vkDir); err != nil {
		t.Fatal(err)
	}
	wireBlob := buildZKPGWire(proof, [][]byte{inputs})
	if err := VerifyCheckZKP(CheckZKPModeGroth16, wireBlob, [][]byte{inputs}); err != nil {
		t.Fatal(err)
	}
}

func groth16TestVector1() (vk, proof, inputs []byte) {
	return groth16DemoVector()
}
