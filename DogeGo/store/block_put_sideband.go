// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"dogego/chain"
	"dogego/wire"
)

// MempoolConfirmFeeSample is a mempool tx confirmed in a block (feerate + blocks waited).
type MempoolConfirmFeeSample struct {
	TxID         string
	FeeratePerKB uint64
	BlocksWaited int
}

// MempoolBlockPruner removes transactions superseded by a newly stored block.
type MempoolBlockPruner interface {
	RemoveForBlockRaw(raw []byte) []string
}

// BlockPutSideband runs after each successful RawBlockStore.Put (P2P fetch, inv, broadcast).
type BlockPutSideband struct {
	Journal         *HeaderJournal
	Aux             *HeaderAuxJournal
	Network         chain.Network
	ContiguousHeight func() int64
	Pool            MempoolBlockPruner
	// CollectMempoolConfirmed samples pooled txs in the block before mempool prune (optional).
	CollectMempoolConfirmed func(blockRaw []byte, blockHeight int64) []MempoolConfirmFeeSample
	RecordMempoolConfirmed  func(blockHeight int64, samples []MempoolConfirmFeeSample)
	AfterBlockStored func(blockRaw []byte)
	// IndexBlockFilter builds/persists a BIP158 basic filter after each block store (optional).
	IndexBlockFilter func(hashLE [32]byte, blockRaw []byte) error
}

// AfterPut patches empty auxpow slots and prunes the mempool when configured.
func (b *BlockPutSideband) AfterPut(hashLE [32]byte, raw []byte) {
	if b == nil || len(raw) < 80 {
		return
	}
	height := int64(-1)
	if b.Journal != nil {
		if h, err := b.Journal.HeightByBlockHashLE(hashLE); err == nil {
			height = h
		}
	}
	if b.Aux != nil && b.Journal != nil && height >= 0 {
		cont := int64(-1)
		if b.ContiguousHeight != nil {
			cont = b.ContiguousHeight()
		}
		_, _ = PatchAuxFromBlockAtHeight(b.Journal, b.Aux, b.Network, height, cont, raw)
	}
	if err := wire.ValidateBlockPayload(raw, hashLE); err != nil {
		return
	}
	if b.RecordMempoolConfirmed != nil {
		var samples []MempoolConfirmFeeSample
		if b.CollectMempoolConfirmed != nil {
			samples = b.CollectMempoolConfirmed(raw, height)
		}
		if height >= 0 || len(samples) > 0 {
			b.RecordMempoolConfirmed(height, samples)
		}
	}
	if b.AfterBlockStored != nil {
		b.AfterBlockStored(raw)
	}
	if b.Pool != nil {
		b.Pool.RemoveForBlockRaw(raw)
	}
	if b.IndexBlockFilter != nil {
		_ = b.IndexBlockFilter(hashLE, raw)
	}
}
