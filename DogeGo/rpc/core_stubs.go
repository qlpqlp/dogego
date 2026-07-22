// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "runtime"

// execGetMemoryInfo returns a Core-shaped memory summary (DogeGo uses Go runtime stats where available).
func execGetMemoryInfo() map[string]interface{} {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return map[string]interface{}{
		"locked": map[string]interface{}{
			"used":        ms.HeapInuse,
			"free":        ms.HeapIdle,
			"total":       ms.HeapSys,
			"locked":      ms.HeapInuse,
			"chunks_used": 0,
			"chunks_free": 0,
		},
		"dogego_note":      "Go runtime MemStats (not Core locked_pool); heap_inuse bytes in locked.used",
		"dogego_heap_sys":  ms.HeapSys,
		"dogego_heap_alloc": ms.HeapAlloc,
	}
}

