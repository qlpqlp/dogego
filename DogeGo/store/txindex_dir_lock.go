// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "sync"

// txIndexDirLocks serializes all indexes/tx file writes for one chain directory.
// Repair paths open a second *TxIndex with its own mu; without a shared dir lock
// Windows reports ".tmp: being used by another process" during parallel IBD connect.
var txIndexDirLocks sync.Map

func txIndexDirLock(root string) *sync.Mutex {
	if root == "" {
		return &sync.Mutex{}
	}
	v, _ := txIndexDirLocks.LoadOrStore(root, &sync.Mutex{})
	return v.(*sync.Mutex)
}
