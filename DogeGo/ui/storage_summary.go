// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// mergeStorageSummary copies Core/native layout fields into a dashboard summary map.
// storage_mode is not double-prefixed; layout_warning is exposed as storage_layout_warning.
func mergeStorageSummary(summary map[string]any, sm map[string]interface{}) {
	if summary == nil || sm == nil {
		return
	}
	for k, v := range sm {
		switch k {
		case "storage_mode":
			summary["storage_mode"] = v
		case "layout_warning":
			summary["storage_layout_warning"] = v
		case "layout", "native_headers", "native_rawblocks", "native_raw_block_count",
			"native_tx_index", "native_txindex_legacy_files", "native_txindex_v2_files",
			"native_contiguous_body_height",
			"chain_bytes_total", "headers_bytes", "rawblocks_bytes", "txindex_bytes",
			"blocks_logical_bytes", "blocks_stored_payload_bytes",
			"compression_ratio", "compression_savings_pct",
			"block_layout", "block_zstd":
			summary[k] = v
		default:
			summary["storage_"+k] = v
		}
	}
}
