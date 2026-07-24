// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"

	"dogego/applog"
	"dogego/bloom"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

const (
	maxServeBlocksPerGetData = 16
	maxServeTxsPerGetData    = 8
)

// GetDataServeEnv holds data sources for answering inbound getdata (Core ProcessGetData subset).
type GetDataServeEnv struct {
	Raw   *store.RawBlockStore
	Pool  *mempool.Pool
	TxIx  *store.TxIndex
	Bloom *bloom.Filter // optional BIP37 filter for MSG_FILTERED_BLOCK
}

// HandleInboundGetData serves block/tx inventory from local store and mempool; sends notfound for the rest.
func HandleInboundGetData(ctx context.Context, mw *MsgWriter, env GetDataServeEnv, payload []byte) error {
	if mw == nil {
		return nil
	}
	entries, err := wire.DecodeInvPayload(payload)
	if err != nil {
		return err
	}
	var notfound []wire.InvEntry
	servedBlk, servedTx, servedFilt := 0, 0, 0
	for _, e := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		switch e.Type {
		case wire.InvTypeBlock, wire.InvTypeWitnessBlock:
			if servedBlk >= maxServeBlocksPerGetData {
				notfound = append(notfound, e)
				continue
			}
			raw, ok := serveBlock(env.Raw, e.Hash)
			if !ok {
				notfound = append(notfound, e)
				continue
			}
			if err := mw.Write("block", raw); err != nil {
				return err
			}
			servedBlk++
		case wire.InvTypeCmpctBlock:
			if servedBlk >= maxServeBlocksPerGetData {
				notfound = append(notfound, e)
				continue
			}
			blockRaw, ok := serveBlock(env.Raw, e.Hash)
			if !ok {
				notfound = append(notfound, e)
				continue
			}
			cmpct, cmpctOK := ServeCmpctBlockFromRaw(blockRaw)
			if !cmpctOK {
				// AuxPoW and other non-cmpct blocks: serve full block (Core-shaped fallback).
				cmpctMetrics.FallbackFullBlock.Add(1)
				if err := mw.Write("block", blockRaw); err != nil {
					return err
				}
				servedBlk++
				continue
			}
			if err := mw.Write("cmpctblock", cmpct); err != nil {
				return err
			}
			cmpctMetrics.ServedGetData.Add(1)
			servedBlk++
		case wire.InvTypeFilteredBlock:
			if env.Bloom == nil || env.Bloom.IsEmpty() {
				notfound = append(notfound, e)
				continue
			}
			if servedFilt >= maxServeBlocksPerGetData {
				notfound = append(notfound, e)
				continue
			}
			n, err := serveFilteredBlock(mw, env, e.Hash)
			if err != nil || n == 0 {
				notfound = append(notfound, e)
				continue
			}
			servedFilt++
			servedTx += n - 1 // merkleblock + matched txs; count txs toward batch
		case wire.InvTypeWitnessTx:
			notfound = append(notfound, e)
		case wire.InvTypeTx:
			if servedTx >= maxServeTxsPerGetData {
				notfound = append(notfound, e)
				continue
			}
			raw, ok := serveTx(env, e.Hash)
			if !ok {
				notfound = append(notfound, e)
				continue
			}
			if err := mw.Write("tx", raw); err != nil {
				return err
			}
			servedTx++
		default:
			notfound = append(notfound, e)
		}
	}
	if len(notfound) == 0 {
		if servedBlk > 0 || servedTx > 0 || servedFilt > 0 {
			applog.Line("net", fmt.Sprintf("getdata served %d block(s), %d filtered, %d tx(s)", servedBlk, servedFilt, servedTx))
		}
		return nil
	}
	pl, err := wire.EncodeNotFound(notfound)
	if err != nil {
		return err
	}
	if err := mw.Write("notfound", pl); err != nil {
		return err
	}
	if servedBlk > 0 || servedTx > 0 || servedFilt > 0 {
		applog.Line("net", fmt.Sprintf("getdata served %d block(s), %d filtered, %d tx(s); notfound %d", servedBlk, servedFilt, servedTx, len(notfound)))
	}
	return nil
}

// serveFilteredBlock sends merkleblock + matched txs. Returns number of messages written (0 = fail).
func serveFilteredBlock(mw *MsgWriter, env GetDataServeEnv, hashLE [32]byte) (int, error) {
	raw, ok := serveBlock(env.Raw, hashLE)
	if !ok || len(raw) < 80 {
		return 0, nil
	}
	pb, err := wire.ParseBlock(raw)
	if err != nil {
		return 0, err
	}
	txids := make([][32]byte, len(pb.Txs))
	match := make([]bool, len(pb.Txs))
	var matched []*wire.Tx
	for i, tx := range pb.Txs {
		txids[i] = tx.TxHash()
		if bloom.MatchRelevantTx(env.Bloom, tx) {
			match[i] = true
			matched = append(matched, tx)
		}
	}
	pmt, err := wire.NewPartialMerkleTree(txids, match)
	if err != nil {
		return 0, err
	}
	mb, err := wire.SerializeMerkleBlock(raw[:80], pmt)
	if err != nil {
		return 0, err
	}
	if err := mw.Write("merkleblock", mb); err != nil {
		return 0, err
	}
	n := 1
	for _, tx := range matched {
		rawTx, err := tx.Serialize()
		if err != nil {
			continue
		}
		if err := mw.Write("tx", rawTx); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func serveBlock(raw *store.RawBlockStore, hashLE [32]byte) ([]byte, bool) {
	if raw == nil {
		return nil, false
	}
	b, err := raw.Get(hashLE)
	if err != nil {
		return nil, false
	}
	return b, true
}

func serveTx(env GetDataServeEnv, hashLE [32]byte) ([]byte, bool) {
	txid := mempool.TxIDDisplayHex(hashLE)
	if env.Pool != nil {
		if raw, err := env.Pool.GetRawByTxID(txid); err == nil {
			return raw, true
		}
	}
	if env.TxIx != nil && env.Raw != nil {
		if raw, ok := confirmedTxRaw(env.TxIx, env.Raw, txid); ok {
			return raw, true
		}
	}
	return nil, false
}

func confirmedTxRaw(ix *store.TxIndex, raw *store.RawBlockStore, txidHex string) ([]byte, bool) {
	hit, err := ix.LookupHit(txidHex)
	if err != nil {
		return nil, false
	}
	if len(hit.TxRaw) > 0 {
		return hit.TxRaw, true
	}
	payload, err := raw.Get(hit.BlockHashLE)
	if err != nil {
		return nil, false
	}
	tx, _, err := wire.ReadTxAtIndex(payload, hit.TxIndex)
	if err != nil {
		return nil, false
	}
	ser, err := tx.Serialize()
	if err != nil {
		return nil, false
	}
	return ser, true
}
