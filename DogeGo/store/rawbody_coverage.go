// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "dogego/pow"

// ContiguousRawBodyHeight returns the highest height h such that block bodies exist for every
// header in [0, h] under native rawblocks/, or -1 when genesis is missing.
func ContiguousRawBodyHeight(j *HeaderJournal, raw *RawBlockStore) (int64, error) {
	if j == nil || raw == nil {
		return -1, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return -1, err
	}
	for h := int64(0); h <= tip; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return -1, err
		}
		if !raw.Has(pow.BlockHashLE(h80)) {
			if h == 0 {
				return -1, nil
			}
			return h - 1, nil
		}
	}
	return tip, nil
}
