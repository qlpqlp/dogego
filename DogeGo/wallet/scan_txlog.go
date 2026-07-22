// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"

	"dogego/store"
)

type scannedTxJSON struct {
	TxID        string `json:"txid"`
	Category    string `json:"category"`
	Address     string `json:"address"`
	AmountKoinu int64  `json:"amount_koinu"`
	FeeKoinu    int64  `json:"fee_koinu,omitempty"`
	Vout        uint32 `json:"vout"`
	BlockHeight int64  `json:"block_height"`
}

// MaxScannedBlockHeight returns the highest block height in persisted scan history (-1 if none).
func (w *Disk) MaxScannedBlockHeight() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.maxScannedBlockHeightLocked()
}

func (w *Disk) maxScannedBlockHeightLocked() int64 {
	if h := w.maxScannedBlockHeightFromDB(); h >= 0 {
		return h
	}
	var max int64 = -1
	for _, r := range w.scannedTx {
		if r.BlockHeight > max {
			max = r.BlockHeight
		}
	}
	return max
}

// SeedScannedTx replaces in-memory scan rows (tests and offline fixtures).
func (w *Disk) SeedScannedTx(rows []ScannedTx) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.scannedTx = append([]ScannedTx(nil), rows...)
	w.mu.Unlock()
}

// ListScannedTx returns block-scan history persisted from rescan.
func (w *Disk) ListScannedTx() []ScannedTx {
	if rows := w.listScannedTxFromDB(); len(rows) > 0 {
		return rows
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]ScannedTx, len(w.scannedTx))
	copy(out, w.scannedTx)
	return out
}

// priorReceiveRows returns confirmed receive rows strictly below beforeHeight.
func (w *Disk) priorReceiveRows(beforeHeight int64) []ScannedTx {
	all := w.ListScannedTx()
	if len(all) == 0 {
		return nil
	}
	out := make([]ScannedTx, 0, len(all)/2)
	for _, r := range all {
		if r.Category == "receive" && r.BlockHeight >= 0 && r.BlockHeight < beforeHeight {
			out = append(out, r)
		}
	}
	return out
}

// RescanBlocks scans raw blocks from startHeight through the contiguous tip and merges into wallet history.
func (w *Disk) RescanBlocks(j *store.HeaderJournal, raw *store.RawBlockStore, pkhVer, shVer byte, startHeight int64) error {
	tracked := w.TrackedScripts()
	prior := w.priorReceiveRows(startHeight)
	hexByID := make(map[string]string)
	rows, err := ScanBlocksRange(j, raw, tracked, pkhVer, shVer, startHeight, hexByID, prior)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.mergeScannedFromHeightLocked(startHeight, rows)
	saveErr := w.saveLocked()
	w.mu.Unlock()
	if saveErr != nil {
		return saveErr
	}
	return w.persistScannedMergeToDB(startHeight, rows, hexByID)
}

func (w *Disk) loadScannedTx(rows []scannedTxJSON) {
	w.scannedTx = w.scannedTx[:0]
	for _, r := range rows {
		if r.TxID == "" {
			continue
		}
		w.scannedTx = append(w.scannedTx, ScannedTx{
			TxID: r.TxID, Category: r.Category, Address: r.Address,
			AmountKoinu: r.AmountKoinu, FeeKoinu: r.FeeKoinu, Vout: r.Vout, BlockHeight: r.BlockHeight,
		})
	}
}

func (w *Disk) scannedTxToDisk() []scannedTxJSON {
	if len(w.scannedTx) == 0 {
		return nil
	}
	out := make([]scannedTxJSON, len(w.scannedTx))
	for i, r := range w.scannedTx {
		out[i] = scannedTxJSON{
			TxID: r.TxID, Category: r.Category, Address: r.Address,
			AmountKoinu: r.AmountKoinu, FeeKoinu: r.FeeKoinu, Vout: r.Vout, BlockHeight: r.BlockHeight,
		}
	}
	return out
}

// ErrNoRawBlocks is returned when rescan cannot read block bodies.
var ErrNoRawBlocks = fmt.Errorf("no contiguous raw blocks for rescan")
