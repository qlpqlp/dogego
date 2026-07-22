// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "slices"

// sortFundCandidates orders UTXO picks for funding: non-reused scripts first when avoid_reuse
// is enabled, then largest value within each group (Core deprioritizes reused addresses).
func sortFundCandidates(paths *DataPaths, candidates []fundPick) {
	isReused := paths != nil && paths.WalletIsScriptReused != nil && rpcWalletAvoidReuse(paths)
	slices.SortFunc(candidates, func(a, b fundPick) int {
		ri := isReused && paths.WalletIsScriptReused(a.row.PkScript)
		rj := isReused && paths.WalletIsScriptReused(b.row.PkScript)
		switch {
		case ri && !rj:
			return 1
		case !ri && rj:
			return -1
		default:
			if a.row.Value > b.row.Value {
				return -1
			}
			if a.row.Value < b.row.Value {
				return 1
			}
			return 0
		}
	})
}
