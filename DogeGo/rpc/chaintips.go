// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/pow"
	"dogego/store"
)

// buildGetChainTips returns Core-shaped chain tips (active chain + header journal ahead during IBD).
func buildGetChainTips(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths) ([]map[string]interface{}, error) {
	blocks, headerTip, contiguousH := activeChainFromJournal(j, raw, paths)
	h80, err := j.ReadHeaderAt(blocks)
	if err != nil {
		return nil, err
	}
	tips := []map[string]interface{}{
		{
			"height":    blocks,
			"hash":      pow.BlockHashHex(h80),
			"branchlen": 0,
			"status":    "active",
			"forkpoint": nil,
		},
	}
	if headerTip > blocks {
		hHdr, err := j.ReadHeaderAt(headerTip)
		if err != nil {
			return tips, nil
		}
		status := chainTipAheadStatus(blocks, headerTip, contiguousH, raw != nil)
		if status == "" {
			status = "headers-only"
		}
		tips = append(tips, map[string]interface{}{
			"height":    headerTip,
			"hash":      pow.BlockHashHex(hHdr),
			"branchlen": headerTip - blocks,
			"status":    status,
			"forkpoint": pow.BlockHashHex(h80),
		})
	}
	return tips, nil
}
