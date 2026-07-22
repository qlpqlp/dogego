// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

const commitmentWireMagic = "ZKCM"

// commitmentDomain separates commitment public inputs from arbitrary hashes.
var commitmentDomain = []byte("dogego.zkl2.commitment.v1")

// verifyCommitmentProof checks ZKCM wire binding to public_inputs (#3869-style 32-byte inputs).
func verifyCommitmentProof(proofData []byte, publicInputs [][]byte) error {
	if len(proofData) < 4+4+1+4+32 {
		return fmt.Errorf("commitment: proof too short")
	}
	if string(proofData[:4]) != commitmentWireMagic {
		return fmt.Errorf("commitment: bad magic (want ZKCM)")
	}
	if len(publicInputs) < 2 {
		return fmt.Errorf("commitment: want 2 public inputs")
	}
	for i, in := range publicInputs[:2] {
		if len(in) != 32 {
			return fmt.Errorf("commitment: public input %d want 32 bytes", i)
		}
	}
	payloadHash := proofData[13 : 13+32]
	if !bytesEqual(publicInputs[0], payloadHash) {
		return fmt.Errorf("commitment: payload hash mismatch")
	}
	tag := sha256.Sum256(commitmentDomain)
	if !bytesEqual(publicInputs[1], tag[:]) {
		return fmt.Errorf("commitment: commitment domain tag mismatch")
	}
	return nil
}

func buildZKCMWire(payloadKind string, payloadLen int, payloadHash, tagHash []byte) []byte {
	kindByte := byte('t')
	switch payloadKind {
	case "file", "base64":
		kindByte = 'f'
	case "hex":
		kindByte = 'h'
	}
	var out []byte
	out = append(out, []byte(commitmentWireMagic)...)
	var ver [4]byte
	binary.LittleEndian.PutUint32(ver[:], 1)
	out = append(out, ver[:]...)
	out = append(out, kindByte)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(payloadLen))
	out = append(out, n[:]...)
	out = append(out, payloadHash...)
	out = append(out, tagHash...)
	return out
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func decodePayloadBytes(payload, encoding string) ([]byte, string, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, "", fmt.Errorf("payload required")
	}
	switch encoding {
	case "", "text":
		return []byte(payload), "text", nil
	case "base64", "file":
		b, err := decodeBase64Flexible(payload)
		if err != nil {
			return nil, "", fmt.Errorf("base64 payload: %w", err)
		}
		return b, "file", nil
	case "hex":
		b, err := hex.DecodeString(payload)
		if err != nil {
			return nil, "", fmt.Errorf("hex payload: %w", err)
		}
		return b, "hex", nil
	default:
		return nil, "", fmt.Errorf("unsupported payload_encoding %q", encoding)
	}
}

func decodeBase64Flexible(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
