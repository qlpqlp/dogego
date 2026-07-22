// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// execGetTxOut implements gettxout in a Core-like shape.
// Confirmed outputs require txindex + rawblocks; spend detection walks headers from the
// funding height through the tip and requires a raw block payload for every height in that range
// (otherwise RPC returns an error - DogeGo has no UTXO set).
// Mempool-only hits are returned when include_mempool is true and the tx is only in the pool.
func execGetTxOut(ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, pool *mempool.Pool, utxo *store.UtxoCache, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 {
		return nil, -8, "gettxout: txid and vout index required"
	}
	var txidStr string
	if err := json.Unmarshal(params[0], &txidStr); err != nil {
		return nil, -8, "gettxout: bad txid"
	}
	var nFloat float64
	if err := json.Unmarshal(params[1], &nFloat); err != nil {
		return nil, -8, "gettxout: bad vout"
	}
	if nFloat < 0 || nFloat > float64(^uint32(0)) || nFloat != float64(int64(nFloat)) {
		return nil, -8, "gettxout: vout must be a non-negative integer"
	}
	vout := uint32(nFloat)

	includeMempool := true
	if len(params) > 2 {
		if err := json.Unmarshal(params[2], &includeMempool); err != nil {
			return nil, -8, "gettxout: bad include_mempool flag"
		}
	}

	txidNorm := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txidStr), "0x"))
	if len(txidNorm) != 64 {
		return nil, -8, "gettxout: txid must be 64 hex characters"
	}
	for _, c := range txidNorm {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return nil, -8, "gettxout: txid must be hex"
	}

	var (
		fundingTx *wire.Tx
		coinH     int64
		txIdx     int64
	)

	if ix != nil && raw != nil {
		if hit, err := ix.LookupHit(txidNorm); err == nil {
			var errLoad error
			fundingTx, errLoad = store.LoadIndexedTx(ix, raw, txidNorm)
			if errLoad != nil {
				return nil, -5, "gettxout: indexed transaction missing: "+errLoad.Error()
			}
			coinH, err = j.HeightByDisplayHash(pow.LEUint256DisplayHex(hit.BlockHashLE[:]))
			if err != nil {
				return nil, -8, "gettxout: block height lookup: "+err.Error()
			}
			txIdx = int64(hit.TxIndex)
		}
	}

	if fundingTx != nil {
		if int(vout) >= len(fundingTx.Vout) {
			return nil, 0, ""
		}
		if includeMempool && pool != nil && pool.SpendsOutpoint(txidNorm, vout) {
			return nil, 0, ""
		}
		spent, code, msg := outpointSpentConfirmed(j, raw, utxo, paths, coinH, txIdx, txidNorm, vout)
		if code != 0 {
			return nil, code, msg
		}
		if spent {
			return nil, 0, ""
		}
		chainTip, _, _ := activeChainFromJournal(j, raw, paths)
		best, _ := blockHashHexAt(j, chainTip)
		conf := int64(0)
		if coinH >= 0 && chainTip >= coinH {
			conf = chainTip - coinH + 1
		}
		o := &fundingTx.Vout[vout]
		spk := map[string]interface{}{
			"hex":       fmt.Sprintf("%x", o.PkScript),
			"asm":       "",
			"reqSigs":   1,
			"type":      "nonstandard",
			"addresses": []interface{}{},
		}
		cb := len(fundingTx.Vin) > 0 && isCoinbaseInput(&fundingTx.Vin[0])
		return map[string]interface{}{
			"bestblock":     best,
			"confirmations": conf,
			"value":         float64(o.Value) / 1e8,
			"scriptPubKey":  spk,
			"version":       fundingTx.Version,
			"coinbase":      cb,
			"dogego_note":   spendDetectionNote(utxo),
		}, 0, ""
	}

	// Mempool-only (or include_mempool path when tx not confirmed in our index)
	if !includeMempool || pool == nil {
		return nil, 0, ""
	}
	rawBlob, err := pool.GetRawByTxID(txidNorm)
	if err != nil {
		return nil, 0, ""
	}
	tx, err := wire.DeserializeTx(rawBlob)
	if err != nil {
		return nil, 0, ""
	}
	if int(vout) >= len(tx.Vout) {
		return nil, 0, ""
	}
	if pool.SpendsOutpoint(txidNorm, vout) {
		return nil, 0, ""
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	best, _ := blockHashHexAt(j, chainTip)
	o := &tx.Vout[vout]
	spk := map[string]interface{}{
		"hex":       fmt.Sprintf("%x", o.PkScript),
		"asm":       "",
		"reqSigs":   1,
		"type":      "nonstandard",
		"addresses": []interface{}{},
	}
	cb := len(tx.Vin) > 0 && isCoinbaseInput(&tx.Vin[0])
	return map[string]interface{}{
		"bestblock":     best,
		"confirmations": int64(0),
		"value":         float64(o.Value) / 1e8,
		"scriptPubKey":  spk,
		"version":       tx.Version,
		"coinbase":      cb,
		"dogego_note":   "unconfirmed (local mempool); another mempool tx may still spend this output",
	}, 0, ""
}

func outpointSpentConfirmed(j HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, paths *DataPaths, coinHeight, fundingTxIndex int64, rpcTxid string, vout uint32) (spent bool, errCode int, errMsg string) {
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	if utxo != nil && utxo.TipHeight() >= chainTip && utxo.TipHeight() >= coinHeight {
		_, ok := utxo.Lookup(rpcTxid, vout)
		return !ok, 0, ""
	}
	if raw == nil {
		return false, -8, "gettxout: raw block store required for spend scan"
	}
	spent, err := consensus.OutpointSpentInBlocks(j, raw, coinHeight, fundingTxIndex, rpcTxid, vout)
	if err != nil {
		return false, -8, "gettxout: " + err.Error()
	}
	return spent, 0, ""
}

func spendDetectionNote(utxo *store.UtxoCache) string {
	if utxo != nil && utxo.TipHeight() >= 0 {
		return "spend detection uses in-memory UTXO cache at chain tip when caught up"
	}
	return "spend detection scans rawblocks from coin height through tip until UTXO cache is synced"
}
