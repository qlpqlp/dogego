// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"container/list"
	"sync"
)

const rawBlockReadCacheMax = 48 // hot blocks for RPC getblock / explorer (tip window)

type rawBlockReadCache struct {
	mu    sync.Mutex
	ll    list.List
	items map[[32]byte]*list.Element
	max   int
}

type cacheEntry struct {
	hash [32]byte
	raw  []byte
}

func newRawBlockReadCache(max int) *rawBlockReadCache {
	if max < 1 {
		max = rawBlockReadCacheMax
	}
	return &rawBlockReadCache{
		items: make(map[[32]byte]*list.Element),
		max:   max,
	}
}

func (c *rawBlockReadCache) get(hash [32]byte) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[hash]; ok {
		c.ll.MoveToFront(el)
		cp := append([]byte(nil), el.Value.(cacheEntry).raw...)
		return cp, true
	}
	return nil, false
}

func (c *rawBlockReadCache) put(hash [32]byte, raw []byte) {
	if c == nil || len(raw) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[hash]; ok {
		el.Value = cacheEntry{hash: hash, raw: append([]byte(nil), raw...)}
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(cacheEntry{hash: hash, raw: append([]byte(nil), raw...)})
	c.items[hash] = el
	for c.ll.Len() > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		ent := back.Value.(cacheEntry)
		delete(c.items, ent.hash)
		c.ll.Remove(back)
	}
}

func (c *rawBlockReadCache) drop(hash [32]byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[hash]; ok {
		delete(c.items, hash)
		c.ll.Remove(el)
	}
}
