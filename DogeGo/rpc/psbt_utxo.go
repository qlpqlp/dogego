// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execUtxoUpdatePsbt fills missing non_witness_utxo entries from txindex/raw blocks/mempool (Core subset).
func execUtxoUpdatePsbt(ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	p, code, msg := loadPSBTParam(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "utxoupdatepsbt:") {
			msg = "utxoupdatepsbt: " + msg
		}
		return nil, code, msg
	}
	fillPsbtPrevouts(p, ix, raw, pool)
	b64, code, msg := encodePSBTBase64(p)
	if code != 0 {
		return nil, code, "utxoupdatepsbt: " + msg
	}
	return b64, 0, ""
}

func fillPsbtPrevouts(p *wire.Psbt, ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool) {
	if p == nil {
		return
	}
	for i := range p.UnsignedTx.Vin {
		if p.InputHasUTXO(i) {
			continue
		}
		in := &p.UnsignedTx.Vin[i]
		if isCoinbaseInput(in) {
			continue
		}
		prevID := mempool.TxIDDisplayHex(in.PrevHash)
		parent, ok := lookupConfirmedOrMempoolTx(ix, raw, pool, prevID)
		if !ok {
			continue
		}
		ser, err := parent.Serialize()
		if err != nil {
			ser = parent.SerializeForHash()
		}
		p.SetInputNonWitnessUtxo(i, ser)
	}
}

func lookupConfirmedOrMempoolTx(ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, rpcTxid string) (*wire.Tx, bool) {
	if ix != nil && raw != nil {
		if tx, err := store.LoadIndexedTx(ix, raw, rpcTxid); err == nil {
			return tx, true
		}
	}
	if pool != nil {
		rawBlob, err := pool.GetRawByTxID(rpcTxid)
		if err == nil {
			tx, err := wire.DeserializeTx(rawBlob)
			if err == nil {
				return tx, true
			}
		}
	}
	return nil, false
}
