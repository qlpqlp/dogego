// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"sync"
)

var utxoSnapshotSaveGate struct {
	mu      sync.Mutex
	running bool
}

// SaveSnapshotAsync writes utxo.cache in a background goroutine (at most one in flight).
func (u *UtxoCache) SaveSnapshotAsync(path string) (started bool, err error) {
	if u == nil {
		return false, fmt.Errorf("utxo snapshot: nil cache")
	}
	utxoSnapshotSaveGate.mu.Lock()
	if utxoSnapshotSaveGate.running {
		utxoSnapshotSaveGate.mu.Unlock()
		return false, nil
	}
	utxoSnapshotSaveGate.running = true
	utxoSnapshotSaveGate.mu.Unlock()
	go func() {
		defer func() {
			utxoSnapshotSaveGate.mu.Lock()
			utxoSnapshotSaveGate.running = false
			utxoSnapshotSaveGate.mu.Unlock()
		}()
		_ = u.SaveSnapshot(path)
	}()
	return true, nil
}

// UtxoSnapshotSaveInFlight reports whether a background snapshot write is active.
func UtxoSnapshotSaveInFlight() bool {
	utxoSnapshotSaveGate.mu.Lock()
	defer utxoSnapshotSaveGate.mu.Unlock()
	return utxoSnapshotSaveGate.running
}
