// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync"

// utxoConnectMu serializes ConnectBlock replay (catch-up worker, syncutxo RPC, bounded SyncUtxo).
var utxoConnectMu sync.Mutex

func withUtxoConnectLock(fn func() error) error {
	if fn == nil {
		return nil
	}
	utxoConnectMu.Lock()
	defer utxoConnectMu.Unlock()
	return fn()
}

// UtxoConnectInFlight reports whether connect replay holds utxoConnectMu.
func UtxoConnectInFlight() bool {
	ok := utxoConnectMu.TryLock()
	if ok {
		utxoConnectMu.Unlock()
		return false
	}
	return true
}
