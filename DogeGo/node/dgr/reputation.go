// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import "sort"

// RelayAddrBook records relay QUIC dial outcomes for target ordering (Core addrman try/success).
type RelayAddrBook interface {
	NoteTry(addr string)
	NoteSuccess(addr string)
	NoteFailure(addr string)
	RelayDialScore(addr string) int
}

// SortTargetsByReputation orders relay targets by addrman dial score (best first).
func SortTargetsByReputation(targets []string, book RelayAddrBook) {
	if len(targets) < 2 || book == nil {
		return
	}
	sort.SliceStable(targets, func(i, j int) bool {
		si := book.RelayDialScore(targets[i])
		sj := book.RelayDialScore(targets[j])
		if si != sj {
			return si > sj
		}
		return targets[i] < targets[j]
	})
}
