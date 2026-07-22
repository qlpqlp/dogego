// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/applog"
	"dogego/pow"
)

// HandleBroadcastBlock stores a P2P "block" payload when it matches a header in our journal.
func HandleBroadcastBlock(mw *MsgWriter, bs *BlockStoreCtx, fromAddr string, mb *MisbehaviorTracker, payload []byte) {
	if bs == nil || bs.Raw == nil || len(payload) < 81 {
		return
	}
	want := pow.BlockHashLE(payload[:80])
	if err := bs.StoreValidatedBlock(want, payload); err != nil {
		applog.Line("block", "broadcast block rejected: "+err.Error())
		if mb != nil && fromAddr != "" && isMisbehaviorBlockError(err) {
			mb.Note(fromAddr, misbehaviorInvalidBlock, err.Error())
			_ = RejectInvalidBlock(mw, want, trimRejectReason(err))
		}
		return
	}
	if bs.OnBlockFromPeer != nil && fromAddr != "" {
		height := int64(-1)
		if bs.Journal != nil {
			if h, err := bs.Journal.HeightByBlockHashLE(want); err == nil {
				height = h
			}
		}
		bs.OnBlockFromPeer(fromAddr, height)
	}
}

// RelayStoredBlock announces a block to peers after it was stored (cmpct or full-block inbound).
func RelayStoredBlock(bs *BlockStoreCtx, payload []byte, excludeAddr string) {
	if bs == nil || bs.Raw == nil || len(payload) < 80 {
		return
	}
	want := pow.BlockHashLE(payload[:80])
	if !bs.Raw.Has(want) {
		return
	}
	AnnounceBlockHash(bs.announce, want, payload, excludeAddr)
}
