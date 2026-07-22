// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// execGetPeerInfo returns Core-shaped peer objects when the node wires DataPaths.PeerInfo (embedded DogeGo).
func execGetPeerInfo(paths *DataPaths) []map[string]interface{} {
	if paths != nil && paths.PeerInfo != nil {
		return paths.PeerInfo()
	}
	return []map[string]interface{}{}
}
