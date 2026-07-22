// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package txdb

import (
	"strings"

	"github.com/cockroachdb/pebble"
)

var prefixHex = []byte("h/")

func txHexKey(txid string) []byte {
	return append(append([]byte(nil), prefixHex...), strings.ToLower(strings.TrimSpace(txid))...)
}

// PutTxHex stores serialized tx hex for wallet gettransaction (compact tx index fast path).
func (w *DB) PutTxHex(txid, hexStr string) error {
	txid = strings.ToLower(strings.TrimSpace(txid))
	hexStr = strings.TrimSpace(strings.ToLower(hexStr))
	if len(txid) != 64 || hexStr == "" {
		return nil
	}
	return w.db.Set(txHexKey(txid), []byte(hexStr), pebble.Sync)
}

// GetTxHex returns persisted wallet tx hex when present.
func (w *DB) GetTxHex(txid string) (string, bool) {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if len(txid) != 64 {
		return "", false
	}
	val, closer, err := w.db.Get(txHexKey(txid))
	if err == pebble.ErrNotFound {
		return "", false
	}
	if err != nil {
		return "", false
	}
	defer closer.Close()
	if len(val) == 0 {
		return "", false
	}
	return string(val), true
}
