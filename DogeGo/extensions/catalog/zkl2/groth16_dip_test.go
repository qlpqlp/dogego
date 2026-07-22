// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudflare/circl/ecc/bls12381"
	"github.com/cloudflare/circl/ecc/bls12381/ff"
)

func TestGroth16DIPProofRejectsInvalidPoint(t *testing.T) {
	proof := make([]byte, groth16DIPProofLen)
	// Valid Fp encodings (zero) but not on-curve affine points for G1/G2.
	for i := 0; i < 8; i++ {
		proof[i*48+47] = 1
	}
	err := validateGroth16DIPProof(proof)
	if err == nil {
		t.Fatal("expected invalid point error")
	}
}

func TestGroth16DIPPairingFromCompressedVector(t *testing.T) {
	vk, proof, inputs := groth16TestVector1()
	dip, err := dipProofFromCompressed(proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyGroth16DIP(vk, dip, [][]byte{inputs}); err != nil {
		t.Fatal(err)
	}
}

func TestGroth16DIPPairingZKPGWire(t *testing.T) {
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
	dip, err := dipProofFromCompressed(proof)
	if err != nil {
		t.Fatal(err)
	}
	wireBlob := buildZKPGWire(dip, [][]byte{inputs})
	if err := VerifyCheckZKP(CheckZKPModeGroth16, wireBlob, [][]byte{inputs}); err != nil {
		t.Fatal(err)
	}
}

func TestGroth16WireDIP384Validates(t *testing.T) {
	pi := [][]byte{make([]byte, 32), make([]byte, 32)}
	proof := make([]byte, groth16DIPProofLen)
	for i := 0; i < 8; i++ {
		proof[i*48+47] = 1
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
	err := parseGroth16Wire(wireBlob, pi, nil)
	if err == nil {
		t.Fatal("expected dip point validation failure")
	}
}

// dipProofFromCompressed expands snarkjs compressed proof (192 B) to DIP affine layout (384 B).
func dipProofFromCompressed(proof []byte) ([]byte, error) {
	cp, err := parseCompressedProof(proof)
	if err != nil {
		return nil, err
	}
	out := make([]byte, groth16DIPProofLen)
	ax := cp.A.Bytes()
	if len(ax) != ff.FpSize*2 {
		return nil, fmt.Errorf("g1 affine want %d bytes got %d", ff.FpSize*2, len(ax))
	}
	copy(out[0:48], ax[:48])
	copy(out[48:96], ax[48:96])
	bx, by, err := g2AffineFpChunks(cp.B)
	if err != nil {
		return nil, err
	}
	copy(out[96:144], bx[0])
	copy(out[144:192], bx[1])
	copy(out[192:240], by[0])
	copy(out[240:288], by[1])
	cx := cp.C.Bytes()
	if len(cx) != ff.FpSize*2 {
		return nil, fmt.Errorf("g1 affine want %d bytes got %d", ff.FpSize*2, len(cx))
	}
	copy(out[288:336], cx[:48])
	copy(out[336:384], cx[48:96])
	return out, nil
}

func g2AffineFpChunks(g bls12381.G2) (x, y [2][]byte, err error) {
	raw := g.Bytes()
	if len(raw) != bls12381.G2Size {
		return x, y, fmt.Errorf("g2 uncompressed want %d bytes", bls12381.G2Size)
	}
	var fx, fy ff.Fp2
	if err := fx.UnmarshalBinary(raw[:ff.Fp2Size]); err != nil {
		return x, y, err
	}
	if err := fy.UnmarshalBinary(raw[ff.Fp2Size:]); err != nil {
		return x, y, err
	}
	for i := 0; i < 2; i++ {
		x[i], err = fx[i].MarshalBinary()
		if err != nil {
			return x, y, err
		}
		y[i], err = fy[i].MarshalBinary()
		if err != nil {
			return x, y, err
		}
	}
	return x, y, nil
}
