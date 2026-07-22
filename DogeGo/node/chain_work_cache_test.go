// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"math/big"
	"testing"
)

func TestChainWorkCacheExtend(t *testing.T) {
	c := NewChainWorkCache()
	c.mu.Lock()
	c.ready = true
	c.height = 0
	c.work = big.NewInt(100)
	c.mu.Unlock()
	delta := big.NewInt(7)
	c.Extend(0, 1, delta)
	w, ok := c.WorkThrough(1)
	if !ok || w.Cmp(big.NewInt(107)) != 0 {
		t.Fatalf("work=%v ok=%v", w, ok)
	}
}

func TestChainWorkCacheLookupSkipsWhileWarming(t *testing.T) {
	c := NewChainWorkCache()
	if _, ok := c.LookupThrough(nil, 534_000); ok {
		t.Fatal("expected miss while cache warming")
	}
}

func TestChainWorkCacheExtendPartialByOne(t *testing.T) {
	c := NewChainWorkCache()
	c.RememberPartial(0, big.NewInt(10))
	if w, ok := c.extendPartialByOne(nil, 1); ok {
		t.Fatalf("unexpected ok with nil journal: %v", w)
	}
}
