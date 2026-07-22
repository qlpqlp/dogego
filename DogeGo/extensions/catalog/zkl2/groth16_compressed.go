// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"bytes"
	"fmt"
	"io"

	"github.com/cloudflare/circl/ecc/bls12381"
)

type compressedProof struct {
	A bls12381.G1
	B bls12381.G2
	C bls12381.G1
}

type compressedVK struct {
	Alpha bls12381.G1
	Beta  bls12381.G2
	Gamma bls12381.G2
	Delta bls12381.G2
	IC    []bls12381.G1
}

func parseCompressedG1(b []byte) (bls12381.G1, error) {
	var g bls12381.G1
	if len(b) != 48 {
		return g, fmt.Errorf("g1 compressed want 48 bytes")
	}
	if err := g.SetBytes(b); err != nil {
		return g, err
	}
	return g, nil
}

func parseCompressedG2(b []byte) (bls12381.G2, error) {
	var g bls12381.G2
	if len(b) != 96 {
		return g, fmt.Errorf("g2 compressed want 96 bytes")
	}
	if err := g.SetBytes(b); err != nil {
		return g, err
	}
	return g, nil
}

func parseCompressedProof(proof []byte) (compressedProof, error) {
	if len(proof) != groth16CompressedProofLen {
		return compressedProof{}, fmt.Errorf("groth16: compressed proof want %d bytes got %d", groth16CompressedProofLen, len(proof))
	}
	a, err := parseCompressedG1(proof[:48])
	if err != nil {
		return compressedProof{}, fmt.Errorf("pi_A: %w", err)
	}
	b, err := parseCompressedG2(proof[48:144])
	if err != nil {
		return compressedProof{}, fmt.Errorf("pi_B: %w", err)
	}
	c, err := parseCompressedG1(proof[144:192])
	if err != nil {
		return compressedProof{}, fmt.Errorf("pi_C: %w", err)
	}
	return compressedProof{A: a, B: b, C: c}, nil
}

func parseCompressedVK(vk []byte, publicInputCount int) (compressedVK, error) {
	if err := validateVKSize(vk, publicInputCount); err != nil {
		return compressedVK{}, err
	}
	r := bytes.NewReader(vk)
	readG1 := func() (bls12381.G1, error) {
		buf := make([]byte, 48)
		if _, err := io.ReadFull(r, buf); err != nil {
			return bls12381.G1{}, err
		}
		return parseCompressedG1(buf)
	}
	readG2 := func() (bls12381.G2, error) {
		buf := make([]byte, 96)
		if _, err := io.ReadFull(r, buf); err != nil {
			return bls12381.G2{}, err
		}
		return parseCompressedG2(buf)
	}
	alpha, err := readG1()
	if err != nil {
		return compressedVK{}, fmt.Errorf("vk alpha: %w", err)
	}
	beta, err := readG2()
	if err != nil {
		return compressedVK{}, fmt.Errorf("vk beta: %w", err)
	}
	gamma, err := readG2()
	if err != nil {
		return compressedVK{}, fmt.Errorf("vk gamma: %w", err)
	}
	delta, err := readG2()
	if err != nil {
		return compressedVK{}, fmt.Errorf("vk delta: %w", err)
	}
	var ic []bls12381.G1
	for r.Len() > 0 {
		g, err := readG1()
		if err != nil {
			return compressedVK{}, fmt.Errorf("vk ic: %w", err)
		}
		ic = append(ic, g)
	}
	if len(ic) != publicInputCount+1 {
		return compressedVK{}, fmt.Errorf("groth16: vk ic want %d points got %d", publicInputCount+1, len(ic))
	}
	return compressedVK{Alpha: alpha, Beta: beta, Gamma: gamma, Delta: delta, IC: ic}, nil
}

func scalarFromPublicInput(b []byte) (bls12381.Scalar, error) {
	var sc bls12381.Scalar
	in := append([]byte(nil), b...)
	if len(in) > 32 {
		in = in[len(in)-32:]
	}
	padded := make([]byte, 32)
	copy(padded[32-len(in):], in)
	if err := sc.UnmarshalBinary(padded); err != nil {
		return sc, err
	}
	return sc, nil
}

func verifyGroth16Compressed(vkBytes, proofBytes []byte, publicInputs [][]byte) error {
	proof, err := parseCompressedProof(proofBytes)
	if err != nil {
		return err
	}
	vk, err := parseCompressedVK(vkBytes, len(publicInputs))
	if err != nil {
		return err
	}
	return verifyGroth16Pairing(vk, proof, publicInputs)
}

func verifyGroth16Pairing(vk compressedVK, proof compressedProof, publicInputs [][]byte) error {
	var vkX bls12381.G1
	vkX = vk.IC[0]
	for i, in := range publicInputs {
		sc, err := scalarFromPublicInput(in)
		if err != nil {
			return fmt.Errorf("public input %d: %w", i, err)
		}
		var term bls12381.G1
		term.ScalarMult(&sc, &vk.IC[i+1])
		vkX.Add(&vkX, &term)
	}
	ps := []*bls12381.G1{&proof.A, &vk.Alpha, &vkX, &proof.C}
	qs := []*bls12381.G2{&proof.B, &vk.Beta, &vk.Gamma, &vk.Delta}
	gt := bls12381.ProdPairFrac(ps, qs, []int{-1, 1, 1, 1})
	if !gt.IsIdentity() {
		return fmt.Errorf("groth16: pairing verify failed")
	}
	return nil
}

func groth16VKOrDefault(override []byte) []byte {
	if len(override) > 0 {
		return override
	}
	return vkBytes("")
}

func tryGroth16PairingVerify(proofBytes []byte, publicInputs [][]byte, vkOverride ...[]byte) error {
	var vk []byte
	if len(vkOverride) > 0 {
		vk = groth16VKOrDefault(vkOverride[0])
	} else {
		vk = vkBytes("")
	}
	if len(vk) == 0 {
		return nil
	}
	if len(proofBytes) != groth16CompressedProofLen {
		return nil
	}
	return verifyGroth16Compressed(vk, proofBytes, publicInputs)
}
