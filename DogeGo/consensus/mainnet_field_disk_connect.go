// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"os"
	"sort"
	"strconv"
)

// MainnetFieldDiskConnectCase is one contiguous connect certification range [0, End].
type MainnetFieldDiskConnectCase struct {
	Name string
	End  int64
}

// mainnetFieldDiskConnectTierEnds are certification upper bounds (inclusive) when bundled bodies exist.
var mainnetFieldDiskConnectTierEnds = []int64{3, 100, 272, 500, 1000, 3368}

// MainnetFieldDiskConnectMaxEnd returns optional cap from DOGEGO_FIELD_DISK_CONNECT_MAX (0 = no cap).
func MainnetFieldDiskConnectMaxEnd() int64 {
	s := os.Getenv("DOGEGO_FIELD_DISK_CONNECT_MAX")
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// MainnetFieldDiskConnectCases picks connect tiers covered by bundledContiguous on disk.
func MainnetFieldDiskConnectCases(bundledContiguous int64) []MainnetFieldDiskConnectCase {
	if bundledContiguous < 3 {
		return nil
	}
	maxEnd := MainnetFieldDiskConnectMaxEnd()
	seen := map[int64]struct{}{}
	var ends []int64
	for _, end := range mainnetFieldDiskConnectTierEnds {
		if end > bundledContiguous {
			continue
		}
		if maxEnd > 0 && end > maxEnd {
			continue
		}
		if _, ok := seen[end]; ok {
			continue
		}
		seen[end] = struct{}{}
		ends = append(ends, end)
	}
	if len(ends) == 0 {
		return nil
	}
	sort.Slice(ends, func(i, j int) bool { return ends[i] < ends[j] })
	out := make([]MainnetFieldDiskConnectCase, 0, len(ends))
	for _, end := range ends {
		name := fmt.Sprintf("contiguous_0_%d", end)
		out = append(out, MainnetFieldDiskConnectCase{Name: name, End: end})
	}
	return out
}
