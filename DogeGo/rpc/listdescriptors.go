// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// execListDescriptors returns active descriptor strings for the built-in wallet (Core subset).
func execListDescriptors(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if paths == nil || rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	if paths.WalletListDescriptors == nil {
		return []interface{}{}, 0, ""
	}
	rows := paths.WalletListDescriptors(chainName)
	out := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"desc":       r.Desc,
			"timestamp":  r.Timestamp,
			"active":     r.Active,
			"internal":   r.Internal,
			"range":      nil,
			"next":       nil,
			"next_index": 0,
		})
	}
	return out, 0, ""
}

// WalletDescriptorRow is one listdescriptors entry.
type WalletDescriptorRow struct {
	Desc      string
	Timestamp int64
	Active    bool
	Internal  bool
}

func walletCommitChangeAddress(paths *DataPaths, addr string) {
	if paths == nil || paths.WalletCommitChangeAddress == nil {
		return
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}
	_ = paths.WalletCommitChangeAddress(addr)
}
