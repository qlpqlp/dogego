// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"strings"

	"dogego/wallet/txdb"
)

// RememberTxHex persists signed tx hex for wallet RPC gettransaction (compact tx index).
func (w *Disk) RememberTxHex(txid, hexStr string) error {
	if w == nil {
		return nil
	}
	txid = normalizeWalletTxID(txid)
	hexStr = strings.TrimSpace(strings.ToLower(hexStr))
	if len(txid) != 64 || hexStr == "" {
		return nil
	}
	return w.withTxDB(func(db *txdb.DB) error {
		return db.PutTxHex(txid, hexStr)
	})
}

// LookupTxHex returns persisted wallet tx hex when present.
func (w *Disk) LookupTxHex(txid string) (string, bool) {
	if w == nil {
		return "", false
	}
	txid = normalizeWalletTxID(txid)
	if len(txid) != 64 {
		return "", false
	}
	var hexStr string
	var ok bool
	_ = w.withTxDB(func(db *txdb.DB) error {
		hexStr, ok = db.GetTxHex(txid)
		return nil
	})
	return hexStr, ok
}

func (w *Disk) rememberTxHexBatch(hexByID map[string]string) {
	if w == nil || len(hexByID) == 0 {
		return
	}
	_ = w.withTxDB(func(db *txdb.DB) error {
		for id, h := range hexByID {
			if err := db.PutTxHex(id, h); err != nil {
				return err
			}
		}
		return nil
	})
}
