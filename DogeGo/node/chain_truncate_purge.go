// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// shouldPurgeBodiesOnHeaderRewind reports whether to drop downloaded raw blocks above genesis
// when the header journal is rewound by a full difficulty period or more while bodies barely
// started (avoids block-assist storing payloads against headers that are about to be truncated).
func shouldPurgeBodiesOnHeaderRewind(tipBefore, keepThrough, contiguous int64) bool {
	if tipBefore <= keepThrough {
		return false
	}
	if tipBefore-keepThrough < 240 {
		return false
	}
	return contiguous <= 1
}
