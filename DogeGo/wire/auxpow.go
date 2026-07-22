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

// MergedMiningHeader is the merge-mining marker in parent coinbase (pchMergedMiningHeader).
var MergedMiningHeader = []byte{0xfa, 0xbe, 'm', 'm'}

// AuxPow carries merge-mining proof after an 80-byte auxpow-version header.
type AuxPow struct {
	Coinbase       *Tx
	HashBlock      [32]byte
	MerkleBranch   [][32]byte
	MerkleIndex    int32
	ChainBranch    [][32]byte
	ChainIndex     int32
	ParentHeader80 [80]byte
}

// ReadUint256 reads 32 bytes into fixed array (wire order).
func ReadUint256(r io.Reader) ([32]byte, error) {
	var h [32]byte
	_, err := io.ReadFull(r, h[:])
	return h, err
}

// ReadAuxPow deserializes CAuxPow after the child 80-byte header.
func ReadAuxPow(r *bytes.Reader) (*AuxPow, error) {
	tx, err := ReadTx(r)
	if err != nil {
		return nil, fmt.Errorf("auxpow tx: %w", err)
	}
	a := &AuxPow{Coinbase: tx}
	a.HashBlock, err = ReadUint256(r)
	if err != nil {
		return nil, err
	}
	nMB, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nMB > 64 {
		return nil, fmt.Errorf("merkle branch too long %d", nMB)
	}
	a.MerkleBranch = make([][32]byte, nMB)
	for i := range a.MerkleBranch {
		a.MerkleBranch[i], err = ReadUint256(r)
		if err != nil {
			return nil, err
		}
	}
	if err := binary.Read(r, binary.LittleEndian, &a.MerkleIndex); err != nil {
		return nil, err
	}
	nCB, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nCB > 64 {
		return nil, fmt.Errorf("chain merkle branch too long %d", nCB)
	}
	a.ChainBranch = make([][32]byte, nCB)
	for i := range a.ChainBranch {
		a.ChainBranch[i], err = ReadUint256(r)
		if err != nil {
			return nil, err
		}
	}
	if err := binary.Read(r, binary.LittleEndian, &a.ChainIndex); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(r, a.ParentHeader80[:]); err != nil {
		return nil, err
	}
	return a, nil
}
