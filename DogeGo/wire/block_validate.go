// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"dogego/pow"
	"dogego/primitives"
)

var errTxFoundInBlock = errors.New("tx found in block")

// BlockHeaderAuxFromPayload decodes header and optional auxpow without decoding transactions.
func BlockHeaderAuxFromPayload(raw []byte) (primitives.BlockHeader, *AuxPow, error) {
	var hdr primitives.BlockHeader
	if len(raw) < 80 {
		return hdr, nil, fmt.Errorf("block too short %d", len(raw))
	}
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return hdr, nil, err
	}
	var aux *AuxPow
	if isAuxPowVersion(hdr.Version) {
		r := bytes.NewReader(raw[80:])
		a, err := ReadAuxPow(r)
		if err != nil {
			return hdr, nil, fmt.Errorf("auxpow: %w", err)
		}
		aux = a
	}
	return hdr, aux, nil
}

// BlockHeaderFromPayload decodes the 80-byte header from a serialized block (no tx decode).
func BlockHeaderFromPayload(raw []byte) (primitives.BlockHeader, error) {
	var hdr primitives.BlockHeader
	if len(raw) < 80 {
		return hdr, fmt.Errorf("block too short %d", len(raw))
	}
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return hdr, err
	}
	return hdr, nil
}

// TxHashRPCDisplay returns the Bitcoin-style RPC txid hex (byte-reversed vs wire hash).
func TxHashRPCDisplay(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return hex.EncodeToString(b)
}

// RPCTxidsFromPayload returns RPC-display txids for every tx in the block without retaining all txs.
func RPCTxidsFromPayload(raw []byte) ([]string, error) {
	var ids []string
	err := ForEachBlockTx(raw, func(_ uint32, tx *Tx) error {
		ids = append(ids, TxHashRPCDisplay(tx.TxHash()))
		return nil
	})
	return ids, err
}

// MerkleRootFromTxHashes builds the block merkle root from wire tx hashes (legacy witness tree).
func MerkleRootFromTxHashes(hashes [][32]byte) [32]byte {
	if len(hashes) == 0 {
		return [32]byte{}
	}
	layer := append([][32]byte(nil), hashes...)
	for len(layer) > 1 {
		if len(layer)%2 == 1 {
			layer = append(layer, layer[len(layer)-1])
		}
		next := make([][32]byte, len(layer)/2)
		for i := 0; i < len(layer); i += 2 {
			next[i/2] = HashPair(layer[i], layer[i+1])
		}
		layer = next
	}
	return layer[0]
}

// FindTxByRPCID scans a block payload for a transaction by RPC display txid without retaining all txs.
func FindTxByRPCID(raw []byte, wantRPC string) (*Tx, uint32, error) {
	var out *Tx
	var idx uint32
	found := false
	scanErr := ForEachBlockTx(raw, func(i uint32, tx *Tx) error {
		if TxHashRPCDisplay(tx.TxHash()) != wantRPC {
			return nil
		}
		out = tx
		idx = i
		found = true
		return errTxFoundInBlock
	})
	if found || errors.Is(scanErr, errTxFoundInBlock) {
		return out, idx, nil
	}
	if scanErr != nil {
		return nil, 0, scanErr
	}
	return nil, 0, fmt.Errorf("tx not in block")
}

// ValidateBlockPayload checks header hash and merkle root without retaining all decoded txs.
func ValidateBlockPayload(raw []byte, wantBlockID [32]byte) error {
	hdr, err := BlockHeaderFromPayload(raw)
	if err != nil {
		return err
	}
	h80 := hdr.EncodeWire80()
	if pow.BlockHashLE(h80[:]) != wantBlockID {
		return fmt.Errorf("block header hash mismatch")
	}
	var hashes [][32]byte
	if err := ForEachBlockTx(raw, func(_ uint32, tx *Tx) error {
		hashes = append(hashes, tx.TxHash())
		return nil
	}); err != nil {
		return err
	}
	if MerkleRootFromTxHashes(hashes) != hdr.MerkleRoot {
		return fmt.Errorf("merkle root mismatch")
	}
	return nil
}
