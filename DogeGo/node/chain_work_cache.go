// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"math/big"
	"sync"

	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

// ChainWorkCache holds cumulative chain work through the journal tip without rereading all of headers.bin.
type ChainWorkCache struct {
	mu     sync.RWMutex
	height int64
	work   *big.Int
	ready  bool

	partialMu sync.RWMutex
	partialH  int64
	partialW  *big.Int
}

// NewChainWorkCache returns an empty cache; call Warm after the header journal is opened.
func NewChainWorkCache() *ChainWorkCache {
	return &ChainWorkCache{work: big.NewInt(0), height: -1}
}

// Warm rebuilds the cache in the background (one full pass at startup).
func (c *ChainWorkCache) Warm(j *store.HeaderJournal) {
	if c == nil || j == nil {
		return
	}
	go func() {
		tip, err := j.TipHeight()
		if err != nil || tip < 0 {
			return
		}
		sum, err := rpc.CumulativeChainWorkBig(j, tip)
		if err != nil {
			return
		}
		c.mu.Lock()
		c.height = tip
		c.work = sum
		c.ready = true
		c.mu.Unlock()
	}()
}

// Invalidate clears the cache (header reorg / truncate); Warm should be called again.
func (c *ChainWorkCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.ready = false
	c.height = -1
	c.work = big.NewInt(0)
	c.mu.Unlock()
	c.partialMu.Lock()
	c.partialH = -1
	c.partialW = nil
	c.partialMu.Unlock()
}

// Extend adds work for a batch appended at the new journal tip.
func (c *ChainWorkCache) Extend(prevTip, newTip int64, batchWork *big.Int) {
	if c == nil || batchWork == nil || newTip < prevTip {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready {
		return
	}
	if prevTip != c.height {
		c.ready = false
		return
	}
	if newTip < c.height {
		return
	}
	c.work = new(big.Int).Add(new(big.Int).Set(c.work), batchWork)
	c.height = newTip
}

// WorkThrough returns cached cumulative work when height matches the cached tip or a recent partial query.
func (c *ChainWorkCache) WorkThrough(through int64) (*big.Int, bool) {
	if c == nil || through < 0 {
		return nil, false
	}
	c.mu.RLock()
	if c.ready && through == c.height {
		w := new(big.Int).Set(c.work)
		c.mu.RUnlock()
		return w, true
	}
	c.mu.RUnlock()
	c.partialMu.RLock()
	if c.partialH == through && c.partialW != nil {
		w := new(big.Int).Set(c.partialW)
		c.partialMu.RUnlock()
		return w, true
	}
	c.partialMu.RUnlock()
	return nil, false
}

// RememberPartial caches chain work through a height below the warmed header tip (getblockchaininfo chainActive).
func (c *ChainWorkCache) RememberPartial(through int64, work *big.Int) {
	if c == nil || through < 0 || work == nil {
		return
	}
	c.partialMu.Lock()
	c.partialH = through
	c.partialW = new(big.Int).Set(work)
	c.partialMu.Unlock()
}

// LookupThrough returns cached chain work or computes through height and remembers the result.
func (c *ChainWorkCache) LookupThrough(j *store.HeaderJournal, through int64) (*big.Int, bool) {
	if w, ok := c.WorkThrough(through); ok {
		return w, true
	}
	if j == nil || through < 0 {
		return nil, false
	}
	c.mu.RLock()
	ready := c.ready
	c.mu.RUnlock()
	if !ready && through > 50_000 {
		return nil, false
	}
	if w, ok := c.extendPartialByOne(j, through); ok {
		return w, true
	}
	w, err := rpc.CumulativeChainWorkBig(j, through)
	if err != nil {
		return nil, false
	}
	c.RememberPartial(through, w)
	return w, true
}

func (c *ChainWorkCache) extendPartialByOne(j *store.HeaderJournal, through int64) (*big.Int, bool) {
	if c == nil || j == nil || through < 0 {
		return nil, false
	}
	c.partialMu.RLock()
	prevH := c.partialH
	prevW := c.partialW
	c.partialMu.RUnlock()
	if prevH < 0 || prevW == nil || through != prevH+1 {
		return nil, false
	}
	buf, err := j.ReadHeaderAt(through)
	if err != nil || len(buf) < 80 {
		return nil, false
	}
	bits := binary.LittleEndian.Uint32(buf[72:76])
	hw, err := pow.BlockProofFromBits(bits)
	if err != nil {
		return nil, false
	}
	w := new(big.Int).Add(new(big.Int).Set(prevW), hw)
	c.RememberPartial(through, w)
	return w, true
}

// Ready reports whether the cache has completed at least one warm pass.
func (c *ChainWorkCache) Ready() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}
