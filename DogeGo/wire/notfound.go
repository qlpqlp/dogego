// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// EncodeNotFound builds a notfound message payload (vector<CInv>), same encoding as getdata/inv.
func EncodeNotFound(inv []InvEntry) ([]byte, error) {
	return EncodeGetData(inv)
}
