// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"reflect"

	"dogego/store"
	"dogego/wire"
)

// TxIndexer resolves a confirmed transaction by RPC display txid.
type TxIndexer interface {
	Lookup(txidHex string) (blockHashLE [32]byte, txIndex uint32, err error)
}

// RawBlockGetter loads a raw P2P block payload by block id (LE uint256).
type RawBlockGetter interface {
	Get(hashLE [32]byte) ([]byte, error)
}

// ChainPrevOutView resolves prevouts from indexed blocks in rawblocks/ (Core UTXO subset).
type ChainPrevOutView struct {
	Index TxIndexer
	Raw   RawBlockGetter
}

// Lookup implements PrevOutView.
func (v *ChainPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	if v == nil || v.Index == nil || v.Raw == nil {
		return PrevOut{}, false
	}
	txid := txidDisplayFromLE(prevHash)
	var tx *wire.Tx
	if stx, ok := v.Index.(*store.TxIndex); ok {
		var err error
		tx, err = store.LoadIndexedTx(stx, v.Raw, txid)
		if err != nil {
			return PrevOut{}, false
		}
	} else {
		blockHash, pos, err := v.Index.Lookup(txid)
		if err != nil {
			return PrevOut{}, false
		}
		raw, err := v.Raw.Get(blockHash)
		if err != nil {
			return PrevOut{}, false
		}
		tx, _, err = wire.ReadTxAtIndex(raw, pos)
		if err != nil {
			return PrevOut{}, false
		}
	}
	if int(idx) >= len(tx.Vout) {
		return PrevOut{}, false
	}
	o := tx.Vout[idx]
	return PrevOut{Value: o.Value, PkScript: append([]byte(nil), o.PkScript...)}, true
}

// MultiPrevOutView tries each view in order (mempool before chain is typical for admission).
type MultiPrevOutView []PrevOutView

// Lookup implements PrevOutView.
func (m MultiPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	for _, v := range m {
		if v == nil {
			continue
		}
		if o, ok := v.Lookup(prevHash, idx); ok {
			return o, true
		}
	}
	return PrevOut{}, false
}

// AdmissionPrevOutView builds the prevout resolver used for mempool admission:
// unconfirmed parents in the pool, optional UTXO cache at tip, then txindex + rawblocks.
func AdmissionPrevOutView(pool MempoolPool, index TxIndexer, raw RawBlockGetter) PrevOutView {
	return admissionPrevOutView(pool, nil, index, raw)
}

// AdmissionPrevOutViewWithUtxo is like AdmissionPrevOutView but consults utxo before scanning blocks.
func AdmissionPrevOutViewWithUtxo(pool MempoolPool, utxo UtxoOutpointSource, index TxIndexer, raw RawBlockGetter) PrevOutView {
	var uv PrevOutView
	if utxo != nil {
		uv = UtxoPrevOutView{Source: utxo}
	}
	return admissionPrevOutView(pool, uv, index, raw)
}

func isNilIface(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

// ConnectBlockPrevOutView builds the prevout view for ConnectBlock (UTXO cache checked before tx index).
func ConnectBlockPrevOutView(index TxIndexer, raw RawBlockGetter, utxo UtxoOutpointSource) PrevOutView {
	var uv PrevOutView
	if utxo != nil {
		uv = UtxoPrevOutView{Source: utxo}
	}
	return admissionPrevOutView(nil, uv, index, raw)
}

func admissionPrevOutView(pool MempoolPool, utxo PrevOutView, index TxIndexer, raw RawBlockGetter) PrevOutView {
	var views []PrevOutView
	if !isNilIface(pool) {
		views = append(views, &MempoolPrevOutView{Pool: pool})
	}
	if utxo != nil {
		views = append(views, utxo)
	}
	if index != nil && raw != nil {
		views = append(views, &ChainPrevOutView{Index: index, Raw: raw})
	}
	if len(views) == 0 {
		return nil
	}
	return MultiPrevOutView(views)
}

// MempoolPool is the subset of mempool.Pool needed for prevout lookup (avoids importing mempool in tests).
type MempoolPool interface {
	GetRawByTxID(rpcTxid string) ([]byte, error)
}
