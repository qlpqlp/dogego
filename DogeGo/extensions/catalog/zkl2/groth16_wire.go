// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/binary"
	"fmt"
)

const groth16WireMagic = "ZKPG"

// parseGroth16Wire checks the ZKPG v1 proof blob layout (crypto verify is separate).
// Layout: magic(4) | proof_len(u32 LE) | pi_count(u32 LE) | proof_bytes | pi_count * 32 bytes
func parseGroth16Wire(proofData []byte, publicInputs [][]byte, vk []byte) error {
	if len(proofData) < 12 {
		return fmt.Errorf("groth16: wire too short")
	}
	if string(proofData[:4]) != groth16WireMagic {
		return fmt.Errorf("groth16: bad magic (want ZKPG)")
	}
	proofLen := binary.LittleEndian.Uint32(proofData[4:8])
	piCount := binary.LittleEndian.Uint32(proofData[8:12])
	want := 12 + int(proofLen) + int(piCount)*32
	if want > len(proofData) {
		return fmt.Errorf("groth16: truncated wire (want %d bytes)", want)
	}
	if int(piCount) != len(publicInputs) {
		return fmt.Errorf("groth16: public input count mismatch")
	}
	off := 12 + proofLen
	for i := uint32(0); i < piCount; i++ {
		if len(publicInputs[i]) != 32 {
			return fmt.Errorf("groth16: public input %d want 32 bytes", i)
		}
		embedded := proofData[off : off+32]
		off += 32
		match := true
		for j := 0; j < 32; j++ {
			if embedded[j] != publicInputs[i][j] {
				match = false
				break
			}
		}
		if !match {
			return fmt.Errorf("groth16: public input %d mismatch vs wire", i)
		}
	}
	if proofLen < 32 {
		return fmt.Errorf("groth16: proof component too short")
	}
	proofBody := proofData[12 : 12+proofLen]
	if proofLen == groth16DIPProofLen {
		if err := validateGroth16DIPProof(proofBody); err != nil {
			return err
		}
		if err := tryDIPGroth16PairingVerify(proofBody, publicInputs, vk); err != nil {
			return err
		}
	}
	if proofLen == groth16CompressedProofLen {
		if err := tryGroth16PairingVerify(proofBody, publicInputs, vk); err != nil {
			return err
		}
	}
	return nil
}
