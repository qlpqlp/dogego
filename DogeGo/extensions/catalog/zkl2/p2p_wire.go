// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// P2P command names for zkproof-v1 overlay.
const (
	CmdZKInv        = "zkinv"
	CmdGetZKProof   = "getzkproof"
	CmdZKProof      = "zkproof"
	CmdGetZKHeaders = "getzkheaders"
	CmdZKHeaders    = "zkheaders"
	CmdGetZKBlockProofs = "getzkblockproofs"
)

// EncodeZKInv announces proof hashes.
func EncodeZKInv(hashes []string) []byte {
	return encodeHashList(hashes)
}

func DecodeZKInv(payload []byte) ([]string, error) {
	return decodeHashList(payload)
}

// EncodeGetZKProof requests proofs by hash.
func EncodeGetZKProof(hashes []string) []byte {
	return encodeHashList(hashes)
}

func DecodeGetZKProof(payload []byte) ([]string, error) {
	return decodeHashList(payload)
}

// EncodeZKProof carries one or more JSON-encoded proofs.
func EncodeZKProof(proofs []Proof) ([]byte, error) {
	raw, err := json.Marshal(proofs)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxProofDataBytes*4 {
		return nil, fmt.Errorf("zkproof message too large")
	}
	return raw, nil
}

func DecodeZKProof(payload []byte) ([]Proof, error) {
	var proofs []Proof
	if err := json.Unmarshal(payload, &proofs); err != nil {
		var one Proof
		if err2 := json.Unmarshal(payload, &one); err2 != nil {
			return nil, err
		}
		return []Proof{one}, nil
	}
	return proofs, nil
}

// EncodeGetZKHeaders requests proof-count summary from start height (count 0 = default window).
func EncodeGetZKHeaders(start int64, count uint32) []byte {
	var out []byte
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(start))
	out = append(out, n[:]...)
	binary.LittleEndian.PutUint32(n[:4], count)
	out = append(out, n[:4]...)
	return out
}

func DecodeGetZKHeaders(payload []byte) (start int64, count uint32, err error) {
	if len(payload) < 12 {
		return 0, 0, fmt.Errorf("getzkheaders too short")
	}
	start = int64(binary.LittleEndian.Uint64(payload[:8]))
	count = binary.LittleEndian.Uint32(payload[8:12])
	return start, count, nil
}

// EncodeGetZKBlockProofs requests all proofs for a block hash (32 bytes).
func EncodeGetZKBlockProofs(blockHash string) ([]byte, error) {
	b, err := decodeHash32(blockHash)
	if err != nil {
		return nil, err
	}
	return b[:], nil
}

func DecodeGetZKBlockProofs(payload []byte) (string, error) {
	if len(payload) != 32 {
		return "", fmt.Errorf("getzkblockproofs want 32 bytes")
	}
	return hex.EncodeToString(payload), nil
}

// EncodeZKHeaders returns block heights with proof counts.
func EncodeZKHeaders(heights []int64, counts []uint32) ([]byte, error) {
	if len(heights) != len(counts) {
		return nil, fmt.Errorf("heights/counts length mismatch")
	}
	var out []byte
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(heights)))
	out = append(out, n[:]...)
	for i, h := range heights {
		binary.LittleEndian.PutUint64(n[:], uint64(h))
		out = append(out, n[:]...)
		binary.LittleEndian.PutUint32(n[:], counts[i])
		out = append(out, n[:4]...)
	}
	return out, nil
}

func DecodeZKHeaders(payload []byte) ([]int64, []uint32, error) {
	if len(payload) < 4 {
		return nil, nil, fmt.Errorf("zkheaders too short")
	}
	n := int(binary.LittleEndian.Uint32(payload[:4]))
	off := 4
	heights := make([]int64, 0, n)
	counts := make([]uint32, 0, n)
	for i := 0; i < n; i++ {
		if off+12 > len(payload) {
			return nil, nil, fmt.Errorf("truncated zkheaders")
		}
		h := int64(binary.LittleEndian.Uint64(payload[off : off+8]))
		c := binary.LittleEndian.Uint32(payload[off+8 : off+12])
		off += 12
		heights = append(heights, h)
		counts = append(counts, c)
	}
	return heights, counts, nil
}

func encodeHashList(hashes []string) []byte {
	var out []byte
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(hashes)))
	out = append(out, n[:]...)
	for _, h := range hashes {
		b, _ := hex.DecodeString(strings.TrimSpace(h))
		if len(b) != 32 {
			continue
		}
		out = append(out, b...)
	}
	return out
}

func decodeHashList(payload []byte) ([]string, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("hash list too short")
	}
	n := binary.LittleEndian.Uint32(payload[:4])
	want := 4 + int(n)*32
	if len(payload) < want {
		return nil, fmt.Errorf("truncated hash list")
	}
	var out []string
	for i := 0; i < int(n); i++ {
		off := 4 + i*32
		out = append(out, hex.EncodeToString(payload[off:off+32]))
	}
	return out, nil
}
