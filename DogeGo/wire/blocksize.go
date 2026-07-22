// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// BlockTxPayloadBytesRaw sums serialized tx sizes in a block payload without retaining all txs.
func BlockTxPayloadBytesRaw(blockRaw []byte) (int, error) {
	var n int
	err := ForEachBlockTx(blockRaw, func(_ uint32, tx *Tx) error {
		if tx == nil {
			return nil
		}
		b, err := tx.Serialize()
		if err != nil {
			return err
		}
		n += len(b)
		return nil
	})
	return n, err
}

// BlockTxPayloadBytes returns the sum of on-wire transaction sizes in a parsed block (witness included when present).
func BlockTxPayloadBytes(pb *ParsedBlock) int {
	if pb == nil {
		return 0
	}
	var n int
	for _, tx := range pb.Txs {
		if tx == nil {
			continue
		}
		if b, err := tx.Serialize(); err == nil {
			n += len(b)
			continue
		}
		n += len(tx.SerializeForHash())
	}
	return n
}
