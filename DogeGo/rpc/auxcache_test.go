// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"
	"time"
)

func TestAuxBlockCacheByScriptAndHash(t *testing.T) {
	var c auxBlockCache
	p := &pendingAuxBlock{displayHash: "abc123", height: 1}
	c.put("script1", "abc123", 1, p)
	if got, ok := c.getByHash("ABC123"); !ok || got != p {
		t.Fatalf("by hash")
	}
	if got, ok := c.getByScript("script1", 1); !ok || got != p {
		t.Fatalf("by script")
	}
	// Core reuses templates for up to 60s after mempool changes.
	if got, ok := c.getByScript("script1", 2); !ok || got != p {
		t.Fatalf("expected fresh template within stale window")
	}
}

func TestAuxBlockCacheTipReset(t *testing.T) {
	var c auxBlockCache
	c.onTipChange("tipA")
	c.put("s", "h1", 0, &pendingAuxBlock{displayHash: "h1"})
	c.onTipChange("tipB")
	if _, ok := c.getByScript("s", 0); ok {
		t.Fatal("cache should reset on tip change")
	}
}

func TestAuxBlockCacheMempoolStaleWindow(t *testing.T) {
	var c auxBlockCache
	c.put("s", "h1", 1, &pendingAuxBlock{displayHash: "h1"})
	c.created["s"] = time.Now().Add(-auxCacheMempoolStale - time.Second)
	if _, ok := c.getByScript("s", 2); ok {
		t.Fatal("expected stale template after 60s with mempool change")
	}
}
