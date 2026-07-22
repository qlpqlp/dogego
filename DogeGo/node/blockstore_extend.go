// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/consensus"
	"dogego/wire"
)

// SetOnChainTruncating registers a callback at the start of TruncateChainToHeight.
func (c *BlockStoreCtx) SetOnChainTruncating(fn func(keepThrough int64)) {
	if c == nil {
		return
	}
	c.OnChainTruncating = fn
}

// SetOnChainTruncated registers a callback after TruncateChainToHeight (header rewind / operator truncate).
func (c *BlockStoreCtx) SetOnChainTruncated(fn func(keepThrough int64)) {
	if c == nil {
		return
	}
	c.OnChainTruncated = fn
}

// SetOnContiguousAdvance registers a callback when stored raw bodies extend the contiguous chain.
func (c *BlockStoreCtx) SetOnContiguousAdvance(fn func(contiguous int64)) {
	if c == nil {
		return
	}
	c.onContiguousAdvance = fn
}

// SetOnChainActiveAdvance runs after UTXO ApplyBlock advances chainActive (ConnectBlock path).
func (c *BlockStoreCtx) SetOnChainActiveAdvance(fn func(height int64)) {
	if c != nil {
		c.onChainActiveAdvance = fn
	}
}

// AppendOnChainActiveAdvance chains an additional callback after UTXO connect advances chainActive.
func (c *BlockStoreCtx) AppendOnChainActiveAdvance(fn func(height int64)) {
	if c == nil || fn == nil {
		return
	}
	prev := c.onChainActiveAdvance
	c.onChainActiveAdvance = func(h int64) {
		if prev != nil {
			prev(h)
		}
		fn(h)
	}
}

// SetOnTipChanged registers a callback after the header journal grows from an inbound block.
func (c *BlockStoreCtx) SetOnTipChanged(fn func(int64)) {
	if c == nil {
		return
	}
	c.onTipChanged = fn
}

// tryExtendChainFromPayload appends one header when the block builds on the current tip.
func (c *BlockStoreCtx) tryExtendChainFromPayload(blockRaw []byte, nowUnix int64) (int64, error) {
	if c == nil || c.Journal == nil || len(blockRaw) < 80 {
		return -1, fmt.Errorf("no header journal")
	}
	parentH, err := c.Journal.TipHeight()
	if err != nil {
		return -1, err
	}
	if c.Raw != nil {
		ch := c.ContiguousRawHeight()
		if ch >= 0 {
			parentH = ch
		} else {
			parentH = 0
		}
	}
	height, err := consensus.ExtendHeadersFromPayload(c.Journal, c.Aux, c.Params, blockRaw, parentH, nowUnix)
	if err != nil {
		return -1, err
	}
	if c.onTipChanged != nil {
		c.onTipChanged(height)
	}
	return height, nil
}

// tryExtendChainFromBlock appends one header when the block builds on the current tip.
func (c *BlockStoreCtx) tryExtendChainFromBlock(pb *wire.ParsedBlock, nowUnix int64) (int64, error) {
	if c == nil || c.Journal == nil || pb == nil {
		return -1, fmt.Errorf("no header journal")
	}
	parentH, err := c.Journal.TipHeight()
	if err != nil {
		return -1, err
	}
	if c.Raw != nil {
		ch := c.ContiguousRawHeight()
		if ch >= 0 {
			parentH = ch
		} else {
			parentH = 0
		}
	}
	height, err := consensus.ExtendHeadersFromParentHeight(c.Journal, c.Aux, c.Params, pb, parentH, nowUnix)
	if err != nil {
		return -1, err
	}
	if c.onTipChanged != nil {
		c.onTipChanged(height)
	}
	return height, nil
}
