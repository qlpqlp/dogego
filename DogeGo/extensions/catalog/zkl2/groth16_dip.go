// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"fmt"

	"github.com/cloudflare/circl/ecc/bls12381"
	"github.com/cloudflare/circl/ecc/bls12381/ff"
)

// groth16DIPProofLen is OP_CHECKZKP mode-0 stack layout: 8 proof scalars × 48-byte Fp.
const groth16DIPProofLen = 384

// validateGroth16DIPProof checks #3869 DIP proof scalars decode to valid G1/G2 points.
func validateGroth16DIPProof(proof []byte) error {
	_, err := parseDIPProof(proof)
	return err
}

// parseDIPProof decodes DIP mode-0 affine layout (8 × 48-byte Fp) into curve points.
func parseDIPProof(proof []byte) (compressedProof, error) {
	if len(proof) != groth16DIPProofLen {
		return compressedProof{}, fmt.Errorf("groth16: dip proof want %d bytes got %d", groth16DIPProofLen, len(proof))
	}
	chunks := make([][]byte, 8)
	for i := 0; i < 8; i++ {
		off := i * ff.FpSize
		chunks[i] = proof[off : off+ff.FpSize]
		if err := validateFpChunk(chunks[i]); err != nil {
			return compressedProof{}, fmt.Errorf("groth16: proof chunk %d: %w", i, err)
		}
	}
	var a bls12381.G1
	if err := g1FromAffine(chunks[0], chunks[1], &a); err != nil {
		return compressedProof{}, fmt.Errorf("groth16: pi_A: %w", err)
	}
	var b bls12381.G2
	if err := g2FromAffine(chunks[2], chunks[3], chunks[4], chunks[5], &b); err != nil {
		return compressedProof{}, fmt.Errorf("groth16: pi_B: %w", err)
	}
	var c bls12381.G1
	if err := g1FromAffine(chunks[6], chunks[7], &c); err != nil {
		return compressedProof{}, fmt.Errorf("groth16: pi_C: %w", err)
	}
	return compressedProof{A: a, B: b, C: c}, nil
}

// verifyGroth16DIP runs Groth16 pairing verify on DIP affine proof bytes + snarkjs VK.
func verifyGroth16DIP(vkBytes, proofBytes []byte, publicInputs [][]byte) error {
	proof, err := parseDIPProof(proofBytes)
	if err != nil {
		return err
	}
	vk, err := parseCompressedVK(vkBytes, len(publicInputs))
	if err != nil {
		return err
	}
	return verifyGroth16Pairing(vk, proof, publicInputs)
}

func tryDIPGroth16PairingVerify(proofBytes []byte, publicInputs [][]byte, vkOverride ...[]byte) error {
	var vk []byte
	if len(vkOverride) > 0 {
		vk = groth16VKOrDefault(vkOverride[0])
	} else {
		vk = vkBytes("")
	}
	if len(vk) == 0 {
		return nil
	}
	if len(proofBytes) != groth16DIPProofLen {
		return nil
	}
	return verifyGroth16DIP(vk, proofBytes, publicInputs)
}

func validateFpChunk(b []byte) error {
	var fp ff.Fp
	if err := fp.UnmarshalBinary(b); err != nil {
		return err
	}
	return nil
}

func g1FromAffine(xBytes, yBytes []byte, out *bls12381.G1) error {
	raw := make([]byte, bls12381.G1Size)
	copy(raw[:ff.FpSize], xBytes)
	copy(raw[ff.FpSize:], yBytes)
	raw[0] &= 0x1F
	return out.SetBytes(raw)
}

func g2FromAffine(x0, x1, y0, y1 []byte, out *bls12381.G2) error {
	xEnc, err := fp2Marshal(x0, x1)
	if err != nil {
		return err
	}
	yEnc, err := fp2Marshal(y0, y1)
	if err != nil {
		return err
	}
	raw := make([]byte, bls12381.G2Size)
	copy(raw[:ff.Fp2Size], xEnc)
	copy(raw[ff.Fp2Size:], yEnc)
	raw[0] &= 0x1F
	return out.SetBytes(raw)
}

func fp2Marshal(c0, c1 []byte) ([]byte, error) {
	var f2 ff.Fp2
	if err := f2[0].UnmarshalBinary(c0); err != nil {
		return nil, err
	}
	if err := f2[1].UnmarshalBinary(c1); err != nil {
		return nil, err
	}
	return f2.MarshalBinary()
}
