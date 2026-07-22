// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"
	"path/filepath"

	"dogego/chain"
	"dogego/pow"
)

// RepairTxIndexFromRaw re-applies IndexBlock for every raw block (idempotent repair).
func RepairTxIndexFromRaw(chainDir string) (ReindexTxReport, error) {
	return ReindexTxFromRawBlocks(chainDir, false)
}

// RepairTxIndexIfLag runs a full re-index when tx files are far below raw block count.
func RepairTxIndexIfLag(chainDir string, minRawBlocks int) (ReindexTxReport, bool, error) {
	var empty ReindexTxReport
	raw, err := OpenRawBlockStore(chainDir)
	if err != nil {
		return empty, false, err
	}
	rawN, err := raw.Count()
	if err != nil {
		return empty, false, err
	}
	if rawN < minRawBlocks {
		return empty, false, nil
	}
	txIx, err := OpenTxIndex(chainDir)
	if err != nil {
		return empty, false, err
	}
	txN, _, err := txIx.Stats()
	if err != nil {
		return empty, false, err
	}
	// Heuristic: most blocks have >=1 tx; expect tx files to trail raw count when index is stale.
	if txN >= rawN {
		return empty, false, nil
	}
	rep, err := RepairTxIndexFromRaw(chainDir)
	if err != nil {
		return empty, false, err
	}
	return rep, true, nil
}

// LowestMissingBlockHeight returns the first height in [start, tip] without a raw block, or -1 if none.
func LowestMissingBlockHeight(j *HeaderJournal, raw *RawBlockStore, start, tip int64, net chain.Network) (int64, error) {
	if j == nil || raw == nil {
		return -1, nil
	}
	if start < 0 {
		start = 0
	}
	for h := start; h <= tip; h++ {
		if !HasStoredBodyAtHeight(j, raw, h, net) {
			return h, nil
		}
	}
	return -1, nil
}

// IndexRawBlockFile indexes one raw block file by filename (for repair of single entries).
func IndexRawBlockFile(txIx *TxIndex, dir, name string) error {
	if txIx == nil {
		return fmt.Errorf("nil tx index")
	}
	if filepath.Ext(name) != ".bin" || len(name) != 68 {
		return fmt.Errorf("not a block file name")
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	if len(b) < 80 {
		return fmt.Errorf("block too short")
	}
	return txIx.IndexBlock(pow.BlockHashLE(b[:80]), b)
}
