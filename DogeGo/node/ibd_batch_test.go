// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/store"
)

func TestEffectiveProgressiveBatchSize(t *testing.T) {
	if EffectiveProgressiveBatchSize(1) != 16 {
		t.Fatalf("single lane %d want 16 (Core MAX_BLOCKS_IN_TRANSIT_PER_PEER)", EffectiveProgressiveBatchSize(1))
	}
	if EffectiveProgressiveBatchSize(6) != 32 {
		t.Fatalf("six lanes %d want 32 cap", EffectiveProgressiveBatchSize(6))
	}
}

func TestEffectiveProgressiveBatchSizeForIBDEarlyCap(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 0
	bs.contiguousMu.Unlock()
	if n := EffectiveProgressiveBatchSizeForIBD(bs, 4); n != 4 {
		t.Fatalf("genesis frontier batch=%d want 4", n)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 500
	bs.contiguousMu.Unlock()
	if n := EffectiveProgressiveBatchSizeForIBD(bs, 4); n != 8 {
		t.Fatalf("early IBD batch=%d want 8", n)
	}
}

func TestMinRawBlockBytesMainnetEarly(t *testing.T) {
	if got := store.MinRawBlockBytes(chain.MainnetDogecoin, 1); got != 140 {
		t.Fatalf("height 1 min=%d want 140", got)
	}
	if got := store.MinRawBlockBytes(chain.MainnetDogecoin, 50_000); got != 140 {
		t.Fatalf("height 50k min=%d want 140 (consensus rejects invalid stubs)", got)
	}
}
