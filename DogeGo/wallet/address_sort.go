// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"sort"
	"strconv"
	"strings"
)

func addressEntryTypeRank(e AddressEntry) int {
	if e.IsNodeTip {
		return 1
	}
	if e.HDPath != "" {
		if e.IsChange {
			return 2
		}
		return 0
	}
	if e.WatchOnly {
		return 3
	}
	if e.IsCosigner {
		return 4
	}
	return 5
}

func addressEntryPathIndex(path string) int {
	path = strings.TrimSpace(path)
	if path == "" {
		return 999999
	}
	i := strings.LastIndex(path, "/")
	if i < 0 || i+1 >= len(path) {
		return 999999
	}
	n, err := strconv.Atoi(path[i+1:])
	if err != nil {
		return 999999
	}
	return n
}

// SortAddressEntries orders wallet address rows Core-style: receive HD paths by
// index, then change HD paths, then watch-only, cosigner, and other imports.
func SortAddressEntries(entries []AddressEntry) {
	sort.Slice(entries, func(i, j int) bool {
		ai, aj := entries[i], entries[j]
		ti, tj := addressEntryTypeRank(ai), addressEntryTypeRank(aj)
		if ti != tj {
			return ti < tj
		}
		pi, pj := addressEntryPathIndex(ai.HDPath), addressEntryPathIndex(aj.HDPath)
		if pi != pj {
			return pi < pj
		}
		if ai.HDPath != aj.HDPath {
			return ai.HDPath < aj.HDPath
		}
		return ai.Address < aj.Address
	})
}
