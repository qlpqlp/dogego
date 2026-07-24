// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

// BloomScripts returns scriptPubKeys to insert into a BIP37 bloom filter.
func (w *Disk) BloomScripts(pkhVer, shVer byte) [][]byte {
	_ = pkhVer
	_ = shVer
	return w.TrackedScripts()
}
