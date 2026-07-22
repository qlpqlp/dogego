// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestEffectiveBlockDownloadTimeoutEarlyIBDCap(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 0
	bs.contiguousMu.Unlock()
	d := EffectiveBlockDownloadTimeout(bs, 4)
	if d != earlyIBDBlockDownloadTimeout {
		t.Fatalf("early IBD timeout=%v want %v", d, earlyIBDBlockDownloadTimeout)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()
	d = EffectiveBlockDownloadTimeout(bs, 4)
	if d <= earlyIBDBlockDownloadTimeout {
		t.Fatalf("caught-up timeout=%v want > %v", d, earlyIBDBlockDownloadTimeout)
	}
}
