// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// SerializeAuxPow encodes CAuxPow (inverse of ReadAuxPow).
func SerializeAuxPow(a *AuxPow) ([]byte, error) {
	if a == nil || a.Coinbase == nil {
		return nil, fmt.Errorf("auxpow: nil")
	}
	var b bytes.Buffer
	raw, err := a.Coinbase.Serialize()
	if err != nil {
		return nil, fmt.Errorf("auxpow coinbase: %w", err)
	}
	if _, err := b.Write(raw); err != nil {
		return nil, err
	}
	if _, err := b.Write(a.HashBlock[:]); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(a.MerkleBranch))); err != nil {
		return nil, err
	}
	for _, h := range a.MerkleBranch {
		if _, err := b.Write(h[:]); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(&b, binary.LittleEndian, a.MerkleIndex); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(a.ChainBranch))); err != nil {
		return nil, err
	}
	for _, h := range a.ChainBranch {
		if _, err := b.Write(h[:]); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(&b, binary.LittleEndian, a.ChainIndex); err != nil {
		return nil, err
	}
	if _, err := io.Copy(&b, bytes.NewReader(a.ParentHeader80[:])); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
