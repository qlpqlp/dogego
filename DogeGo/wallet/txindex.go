// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"dogego/store"
	"dogego/wallet/txdb"
)

func (w *Disk) txDBPath() string {
	if w == nil || w.path == "" {
		return ""
	}
	return txdb.DefaultPath(w.path)
}

func (w *Disk) withTxDB(fn func(*txdb.DB) error) error {
	path := w.txDBPath()
	if path == "" {
		return nil
	}
	db, err := txdb.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

func (w *Disk) ensureTxDBMigrated() error {
	return w.withTxDB(func(db *txdb.DB) error {
		w.mu.Lock()
		legacy := scannedTxToTxRows(w.scannedTx)
		w.mu.Unlock()
		return db.ImportLegacy(legacy)
	})
}

func scannedTxToTxRows(rows []ScannedTx) []txdb.TxRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]txdb.TxRow, len(rows))
	for i, r := range rows {
		out[i] = txdb.TxRow{
			TxID: r.TxID, Category: r.Category, Address: r.Address,
			AmountKoinu: r.AmountKoinu, FeeKoinu: r.FeeKoinu, Vout: r.Vout, BlockHeight: r.BlockHeight,
		}
	}
	return out
}

func txRowsToScanned(rows []txdb.TxRow) []ScannedTx {
	if len(rows) == 0 {
		return nil
	}
	out := make([]ScannedTx, len(rows))
	for i, r := range rows {
		out[i] = ScannedTx{
			TxID: r.TxID, Category: r.Category, Address: r.Address,
			AmountKoinu: r.AmountKoinu, FeeKoinu: r.FeeKoinu, Vout: r.Vout, BlockHeight: r.BlockHeight,
		}
	}
	return out
}

// IndexConnectedBlock indexes wallet txs from one newly connected block (sequential catch-up).
func (w *Disk) IndexConnectedBlock(j *store.HeaderJournal, raw *store.RawBlockStore, pkhVer, shVer byte, height int64) error {
	if w == nil || j == nil || raw == nil {
		return nil
	}
	maxH := w.MaxScannedBlockHeight()
	if height != maxH+1 {
		return nil
	}
	hexByID := make(map[string]string)
	prior := w.priorReceiveRows(height)
	rows, err := ScanBlocksRange(j, raw, w.TrackedScripts(), pkhVer, shVer, height, hexByID, prior)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.mergeScannedFromHeightLocked(height, rows)
	saveErr := w.saveLocked()
	w.mu.Unlock()
	if saveErr != nil {
		return saveErr
	}
	return w.persistScannedMergeToDB(height, rows, hexByID)
}

// mergeScannedFromHeightLocked updates in-memory scan rows (caller must hold w.mu).
func (w *Disk) mergeScannedFromHeightLocked(fromHeight int64, rows []ScannedTx) {
	kept := w.scannedTx[:0]
	for _, r := range w.scannedTx {
		if r.BlockHeight < fromHeight {
			kept = append(kept, r)
		}
	}
	kept = append(kept, rows...)
	w.scannedTx = kept
	_ = w.consumeKeypoolFromScannedLocked(rows)
	w.rebuildUsedRecvScriptsLocked()
}

// persistScannedMergeToDB writes merged scan rows to wallet.db (no wallet.mu).
func (w *Disk) persistScannedMergeToDB(fromHeight int64, rows []ScannedTx, txHex map[string]string) error {
	if err := w.withTxDB(func(db *txdb.DB) error {
		return db.ReplaceFromHeight(fromHeight, scannedTxToTxRows(rows))
	}); err != nil {
		return err
	}
	w.rememberTxHexBatch(txHex)
	return nil
}

func (w *Disk) maxScannedBlockHeightFromDB() int64 {
	var h int64 = -1
	_ = w.withTxDB(func(db *txdb.DB) error {
		if cur, err := db.ScanCursor(); err == nil && cur >= 0 {
			h = cur
			return nil
		}
		if max, err := db.MaxScannedHeight(); err == nil && max >= 0 {
			h = max
		}
		return nil
	})
	return h
}

func (w *Disk) listScannedTxFromDB() []ScannedTx {
	var out []ScannedTx
	_ = w.withTxDB(func(db *txdb.DB) error {
		rows, err := db.ListTx()
		if err != nil {
			return err
		}
		out = txRowsToScanned(rows)
		return nil
	})
	return out
}
