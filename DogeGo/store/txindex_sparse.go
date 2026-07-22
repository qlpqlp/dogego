// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

// TxIndexSparseThrough samples heights in [0, through] and reports whether any indexed tx or
// non-coinbase parent lookup is missing from indexes/tx (file count can still match rawblocks).
func TxIndexSparseThrough(j *HeaderJournal, raw *RawBlockStore, txIx *TxIndex, net chain.Network, through, step int64) (sparse bool, err error) {
	if j == nil || raw == nil || txIx == nil || through < 0 {
		return false, nil
	}
	if step < 1 {
		step = 1
	}
	for h := int64(0); h <= through; h += step {
		if !HasStoredBodyAtHeight(j, raw, h, net) {
			continue
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return false, err
		}
		block, err := raw.Get(pow.BlockHashLE(h80))
		if err != nil {
			continue
		}
		var miss bool
		err = wire.ForEachBlockTx(block, func(i uint32, tx *wire.Tx) error {
			if !txIx.confirmedFileExists(tx.TxHash()) {
				miss = true
				return sparseStop{}
			}
			if i == 0 {
				return nil
			}
			for j := range tx.Vin {
				in := &tx.Vin[j]
				if isNullOutpoint(in) {
					continue
				}
				if !txIx.confirmedFileExists(in.PrevHash) {
					miss = true
					return sparseStop{}
				}
			}
			return nil
		})
		if miss {
			return true, nil
		}
		if err != nil && !isSparseStop(err) {
			return false, err
		}
	}
	return false, nil
}

type sparseStop struct{}

func (sparseStop) Error() string { return "sparse" }

func isSparseStop(err error) bool {
	_, ok := err.(sparseStop)
	return ok
}

func (x *TxIndex) confirmedFileExists(prevHash [32]byte) bool {
	if x == nil {
		return false
	}
	path := filepath.Join(x.root, txidRPCFileName(prevHash))
	_, err := os.Stat(path)
	return err == nil
}

// RepairTxIndexIfSparse rebuilds indexes/tx when sampled parent lookups are missing on disk.
func RepairTxIndexIfSparse(chainDir string, j *HeaderJournal, raw *RawBlockStore, txIx *TxIndex, net chain.Network, through, step int64, minRawBlocks int) (ReindexTxReport, bool, error) {
	var empty ReindexTxReport
	if chainDir == "" || j == nil || raw == nil || txIx == nil || through < 0 {
		return empty, false, nil
	}
	rawN, err := raw.Count()
	if err != nil {
		return empty, false, err
	}
	if rawN < minRawBlocks {
		return empty, false, nil
	}
	sparse, err := TxIndexSparseThrough(j, raw, txIx, net, through, step)
	if err != nil {
		return empty, false, err
	}
	if !sparse {
		return empty, false, nil
	}
	rep, err := RepairTxIndexFromRaw(chainDir)
	if err != nil {
		return empty, false, err
	}
	return rep, true, nil
}
