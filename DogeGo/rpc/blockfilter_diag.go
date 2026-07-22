// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// mergeBlockFilterDiagnostics adds BIP158 filter catch-up fields to getblockchaininfo.
func mergeBlockFilterDiagnostics(res map[string]interface{}, filterThrough int64, contiguousH int64) {
	if res == nil || filterThrough < 0 {
		return
	}
	res["dogego_filter_index_through"] = filterThrough
	if contiguousH > filterThrough {
		res["dogego_filter_index_lag"] = contiguousH - filterThrough
	}
}
