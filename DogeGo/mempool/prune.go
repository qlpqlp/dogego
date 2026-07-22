// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"time"

	"dogego/wire"
)

// PruneExpired removes transactions older than mempoolexpiry (Core CTxMemPool::Expire).
func (p *Pool) PruneExpired() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.expirySec <= 0 || len(p.addedAt) == 0 {
		return 0
	}
	cutoff := time.Now().Unix() - p.expirySec
	var expired []string
	for rpcID, at := range p.addedAt {
		if at > 0 && at < cutoff {
			expired = append(expired, rpcID)
		}
	}
	n := 0
	for _, rpcID := range expired {
		if p.removeByDisplayTxidLocked(rpcID) {
			n++
		}
	}
	return n
}

func (p *Pool) removeByDisplayTxidLocked(displayTxid string) bool {
	for id, raw := range p.raw {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		if txidDisplayHex(tx.TxHash()) != displayTxid {
			continue
		}
		p.clearFeeDeltaLocked(displayTxid)
		delete(p.raw, id)
		delete(p.addedAt, displayTxid)
		if p.addedAtHeight != nil {
			delete(p.addedAtHeight, displayTxid)
		}
		rm := p.onRemove
		p.bumpChangeSeqLocked()
		if rm != nil {
			rm(displayTxid)
		}
		return true
	}
	return false
}
