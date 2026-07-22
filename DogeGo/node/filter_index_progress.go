// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync/atomic"

var filterIndexThrough atomic.Int64

func init() {
	filterIndexThrough.Store(-1)
}

// SetFilterIndexThrough records the highest height indexed for BIP158 basic filters (catch-up worker).
func SetFilterIndexThrough(h int64) {
	if h < 0 {
		return
	}
	for {
		cur := filterIndexThrough.Load()
		if cur >= h {
			return
		}
		if filterIndexThrough.CompareAndSwap(cur, h) {
			return
		}
	}
}

// FilterIndexThrough returns the tracked filter index tip (-1 if unknown).
func FilterIndexThrough() int64 {
	return filterIndexThrough.Load()
}
