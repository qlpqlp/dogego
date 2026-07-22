// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync"

// headerChainWriteMu serializes header journal append, truncate, and recovery rewinds.
// Core treats headers.bin as a single writer; parallel relay top-up and dedicated sync
// must not interleave truncate with append (bad nBits rewind at ~4080 otherwise never persists).
var headerChainWriteMu sync.Mutex

func withHeaderChainWrite(fn func()) {
	headerChainWriteMu.Lock()
	defer headerChainWriteMu.Unlock()
	fn()
}

func withHeaderChainWriteErr(fn func() error) error {
	headerChainWriteMu.Lock()
	defer headerChainWriteMu.Unlock()
	return fn()
}
