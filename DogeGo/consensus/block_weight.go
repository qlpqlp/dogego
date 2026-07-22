// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/wire"
)

// MaxBlockWeight is consensus.h MAX_BLOCK_WEIGHT (BIP141).
const MaxBlockWeight = 4_000_000

// BlockWeightRaw returns total block weight by scanning serialized block bytes (no full ParsedBlock).
func BlockWeightRaw(blockRaw []byte) (int, error) {
	var total int
	err := wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		w, err := TransactionWeight(tx)
		if err != nil {
			return err
		}
		total += w
		return nil
	})
	return total, err
}

// CheckBlockWeightRaw rejects blocks above MaxBlockWeight using a streaming scan.
func CheckBlockWeightRaw(blockRaw []byte) error {
	w, err := BlockWeightRaw(blockRaw)
	if err != nil {
		return err
	}
	if w > MaxBlockWeight {
		return fmt.Errorf("bad-blk-weight: %d > %d", w, MaxBlockWeight)
	}
	return nil
}

// BlockWeight returns total block weight for legacy (no-witness) blocks.
func BlockWeight(pb *wire.ParsedBlock) (int, error) {
	if pb == nil {
		return 0, nil
	}
	var total int
	for _, tx := range pb.Txs {
		w, err := TransactionWeight(tx)
		if err != nil {
			return 0, err
		}
		total += w
	}
	return total, nil
}

// CheckBlockWeight rejects blocks above MaxBlockWeight (Core ConnectBlock).
func CheckBlockWeight(pb *wire.ParsedBlock) error {
	w, err := BlockWeight(pb)
	if err != nil {
		return err
	}
	if w > MaxBlockWeight {
		return fmt.Errorf("bad-blk-weight: %d > %d", w, MaxBlockWeight)
	}
	return nil
}
