// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
)

// execDogegoRecoverHeaders rewinds stale header journal data without deleting headers.bin manually.
func execDogegoRecoverHeaders(paths *DataPaths) (interface{}, int, string) {
	if paths == nil || paths.RecoverHeaderJournal == nil {
		return nil, -1, "dogego_recoverheaders: header journal not available"
	}
	tipBefore, tipAfter, rewound, err := paths.RecoverHeaderJournal()
	if err != nil {
		if paths.RestartHeaderSyncIfStuck != nil && paths.RestartHeaderSyncIfStuck() {
			return map[string]interface{}{
				"rewound":                         false,
				"tip_before":                      tipBefore,
				"tip_after":                       tipAfter,
				"dogego_header_catch_up_pending":  true,
				"dogego_header_sync_restarted":    true,
				"message":                         "header journal unchanged; background header sync and block-assist restarted",
			}, 0, ""
		}
		return nil, -1, err.Error()
	}
	out := map[string]interface{}{
		"rewound":    rewound,
		"tip_before": tipBefore,
		"tip_after":  tipAfter,
	}
	if rewound {
		out["dogego_header_catch_up_pending"] = true
		out["message"] = "header journal rewound; background header sync and block-assist continue"
	} else if paths.HeaderCatchUpPending != nil && paths.HeaderCatchUpPending() {
		out["dogego_header_catch_up_pending"] = true
		out["dogego_header_sync_restarted"] = true
		out["message"] = "header journal unchanged; background header sync and block-assist restarted"
	}
	return out, 0, ""
}

// execTruncateToHeight truncates headers/rawblocks/index/UTXO to height (operator maintenance).
func execTruncateToHeight(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var height float64
	if err := json.Unmarshal(params[0], &height); err != nil {
		return nil, -8, "height must be a number"
	}
	if height < 0 || height != float64(int64(height)) {
		return nil, -8, "invalid height"
	}
	h := int64(height)
	if paths == nil || paths.TruncateToHeight == nil {
		return nil, -1, "truncatetoheight: not available"
	}
	if err := paths.TruncateToHeight(h); err != nil {
		return nil, -1, err.Error()
	}
	return map[string]interface{}{
		"truncated_to": h,
		"message":      fmt.Sprintf("chain truncated to height %d (headers, rawblocks, txindex, UTXO)", h),
	}, 0, ""
}
