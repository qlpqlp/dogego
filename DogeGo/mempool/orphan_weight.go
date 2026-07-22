// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import "dogego/wire"

func legacyTxWeight(tx *wire.Tx) (int, error) {
	if tx == nil {
		return 0, nil
	}
	raw, err := tx.Serialize()
	if err != nil {
		return 0, err
	}
	return len(raw) * 4, nil
}
