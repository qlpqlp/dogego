// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const (
	auxCacheMaxScripts   = 256
	auxCacheMempoolStale = 60 * time.Second
)

// auxBlockCache stores merge-mining templates by payout script and block hash (Core CAuxBlockCache).
type auxBlockCache struct {
	mu sync.Mutex

	prevTipHash string
	lastMempool uint64

	byScript map[string]*pendingAuxBlock
	byHash   map[string]*pendingAuxBlock
	created  map[string]time.Time // script key -> template build time
}

var globalAuxCache auxBlockCache

func scriptKeyFromH160(h160 [20]byte) string {
	return hex.EncodeToString(h160[:])
}

func (c *auxBlockCache) onTipChange(tipHash string) {
	if c.prevTipHash != "" && c.prevTipHash != tipHash {
		c.resetLocked()
	}
	c.prevTipHash = tipHash
}

func (c *auxBlockCache) resetLocked() {
	c.byScript = nil
	c.byHash = nil
	c.created = nil
}

func (c *auxBlockCache) getByScript(scriptKey string, mempoolSeq uint64) (*pendingAuxBlock, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byScript == nil {
		return nil, false
	}
	p, ok := c.byScript[scriptKey]
	if !ok || p == nil {
		return nil, false
	}
	if c.lastMempool != mempoolSeq {
		if t, ok := c.created[scriptKey]; ok && time.Since(t) > auxCacheMempoolStale {
			return nil, false
		}
	}
	return p, true
}

func (c *auxBlockCache) getByHash(displayHash string) (*pendingAuxBlock, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byHash == nil {
		return nil, false
	}
	p, ok := c.byHash[strings.ToLower(displayHash)]
	return p, ok
}

func (c *auxBlockCache) put(scriptKey, displayHash string, mempoolSeq uint64, p *pendingAuxBlock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byScript == nil {
		c.byScript = make(map[string]*pendingAuxBlock)
		c.byHash = make(map[string]*pendingAuxBlock)
		c.created = make(map[string]time.Time)
	}
	if len(c.byScript) >= auxCacheMaxScripts {
		var oldestKey string
		var oldest time.Time
		for k, t := range c.created {
			if oldestKey == "" || t.Before(oldest) {
				oldestKey, oldest = k, t
			}
		}
		if oldestKey != "" {
			if old := c.byScript[oldestKey]; old != nil && old.displayHash != "" {
				delete(c.byHash, strings.ToLower(old.displayHash))
			}
			delete(c.byScript, oldestKey)
			delete(c.created, oldestKey)
		}
	}
	p.displayHash = displayHash
	c.byScript[scriptKey] = p
	c.byHash[strings.ToLower(displayHash)] = p
	c.created[scriptKey] = time.Now()
	c.lastMempool = mempoolSeq
}
