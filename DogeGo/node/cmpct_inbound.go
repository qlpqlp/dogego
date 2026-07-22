// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

type cmpctPending struct {
	header   *wire.HeaderAndShortIDs
	blockID  [32]byte
	missing  []uint64
	shortHit map[uint64][]byte
}

// CmpctServeEnv holds services for compact-block serving and reconstruction.
type CmpctServeEnv struct {
	Raw   *store.RawBlockStore
	Pool  *mempool.Pool
	Block *BlockStoreCtx
}

func (l *peerLink) clearCmpctPending() {
	l.cmpctPending = nil
}

// requestCmpctFullBlock asks the peer for the full block after compact reconstruction fails (Core-style fallback).
func requestCmpctFullBlock(mw *MsgWriter, blockID [32]byte) {
	if mw == nil {
		return
	}
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeBlock, Hash: blockID}})
	if err != nil {
		return
	}
	cmpctMetrics.ReconstructFallback.Add(1)
	_ = mw.Write("getdata", pl)
}

// HandleInboundCmpctBlock processes a cmpctblock from a peer announcing compact blocks.
func HandleInboundCmpctBlock(mw *MsgWriter, env CmpctServeEnv, link *peerLink, payload []byte) {
	if link == nil || !link.cmpctHBFrom {
		return
	}
	cmpctMetrics.In.Add(1)
	hs, err := wire.DecodeHeaderAndShortIDs(payload)
	if err != nil {
		cmpctMetrics.ReconstructFail.Add(1)
		applog.Line("net", fmt.Sprintf("cmpctblock decode from %s: %v", link.addr, err))
		return
	}
	blockID := pow.BlockHashLE(hs.Header80[:])
	shortHit := wire.MatchCmpctShortIDsFromMempool(hs, mempoolBlobs(env.Pool))
	missing := wire.MissingCmpctIndexes(hs, shortHit)
	if len(missing) == 0 {
		raw, err := wire.ReconstructBlockFromCmpct(hs, shortHit, nil)
		if err != nil {
			cmpctMetrics.ReconstructFail.Add(1)
			applog.Line("net", fmt.Sprintf("cmpctblock reconstruct from %s: %v", link.addr, err))
			requestCmpctFullBlock(mw, blockID)
			return
		}
		cmpctMetrics.MempoolHit.Add(1)
		cmpctMetrics.ReconstructOK.Add(1)
		if env.Block != nil {
			HandleBroadcastBlock(mw, env.Block, link.addr, nil, raw)
			RelayStoredBlock(env.Block, raw, link.addr)
		}
		return
	}
	cmpctMetrics.GetBlockTxnOut.Add(1)
	link.cmpctPending = &cmpctPending{
		header:   hs,
		blockID:  blockID,
		missing:  missing,
		shortHit: shortHit,
	}
	req := &wire.BlockTransactionsRequest{BlockHash: blockID, Indexes: missing}
	pl, err := wire.EncodeBlockTransactionsRequest(req)
	if err != nil {
		link.clearCmpctPending()
		requestCmpctFullBlock(mw, blockID)
		return
	}
	_ = mw.Write("getblocktxn", pl)
}

// HandleInboundBlockTxn completes a pending cmpct reconstruction.
func HandleInboundBlockTxn(mw *MsgWriter, env CmpctServeEnv, link *peerLink, payload []byte) {
	if link == nil || link.cmpctPending == nil {
		return
	}
	cmpctMetrics.BlockTxnIn.Add(1)
	pend := link.cmpctPending
	blockID := pend.blockID
	bt, err := wire.DecodeBlockTransactions(payload)
	if err != nil {
		link.clearCmpctPending()
		cmpctMetrics.ReconstructFail.Add(1)
		requestCmpctFullBlock(mw, blockID)
		return
	}
	if bt.BlockHash != pend.blockID {
		link.clearCmpctPending()
		cmpctMetrics.ReconstructFail.Add(1)
		requestCmpctFullBlock(mw, blockID)
		return
	}
	if len(bt.Transactions) != len(pend.missing) {
		link.clearCmpctPending()
		cmpctMetrics.ReconstructFail.Add(1)
		requestCmpctFullBlock(mw, blockID)
		return
	}
	extra := make(map[uint64][]byte, len(pend.missing))
	for i, idx := range pend.missing {
		extra[idx] = bt.Transactions[i]
	}
	raw, err := wire.ReconstructBlockFromCmpct(pend.header, pend.shortHit, extra)
	link.clearCmpctPending()
	if err != nil {
		cmpctMetrics.ReconstructFail.Add(1)
		applog.Line("net", fmt.Sprintf("blocktxn reconstruct from %s: %v", link.addr, err))
		requestCmpctFullBlock(mw, blockID)
		return
	}
	cmpctMetrics.ReconstructOK.Add(1)
	if env.Block != nil {
		HandleBroadcastBlock(mw, env.Block, link.addr, nil, raw)
		RelayStoredBlock(env.Block, raw, link.addr)
	}
}

// HandleInboundGetBlockTxn serves missing transactions for a compact block request.
func HandleInboundGetBlockTxn(mw *MsgWriter, raw *store.RawBlockStore, payload []byte) error {
	if mw == nil || raw == nil {
		return nil
	}
	req, err := wire.DecodeBlockTransactionsRequest(payload)
	if err != nil {
		return err
	}
	blockRaw, err := raw.Get(req.BlockHash)
	if err != nil {
		return err
	}
	txs, err := BlockTxRawsAtIndexes(blockRaw, req.Indexes)
	if err != nil {
		return err
	}
	bt := &wire.BlockTransactions{BlockHash: req.BlockHash, Transactions: txs}
	pl, err := wire.EncodeBlockTransactions(bt)
	if err != nil {
		return err
	}
	if err := mw.Write("blocktxn", pl); err != nil {
		return err
	}
	cmpctMetrics.BlockTxnServed.Add(1)
	return nil
}

func mempoolBlobs(pool *mempool.Pool) [][]byte {
	if pool == nil {
		return nil
	}
	return pool.RawBlobs()
}
