// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execBumpFee replaces a mempool transaction via BIP125 (options.rawtx or built-in wallet auto-bump).
func execBumpFee(chainName string, pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, params []json.RawMessage, relayTx func([]byte) error, net chain.Network) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "bumpfee: mempool not available"
	}
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	rpcTxid, code, msg := validateWalletTxIDParam(params[0], "bumpfee", "txid")
	if code != 0 {
		return nil, code, msg
	}
	oldRaw, err := pool.GetRawByTxID(rpcTxid)
	if err != nil {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	oldTx, err := wire.DeserializeTx(oldRaw)
	if err != nil {
		return nil, -5, "Invalid or non-wallet transaction id"
	}
	if len(params) < 2 || strings.TrimSpace(string(params[1])) == "null" {
		return tryWalletAutoBumpFee(chainName, paths, pool, txIndex, blocks, j, oldTx, oldRaw, nil, relayTx, net)
	}
	var opts map[string]json.RawMessage
	if err := json.Unmarshal(params[1], &opts); err != nil {
		return nil, -8, "bumpfee: options must be a JSON object"
	}
	rawField, hasRaw := opts["rawtx"]
	if !hasRaw || strings.TrimSpace(string(rawField)) == "null" {
		return tryWalletAutoBumpFee(chainName, paths, pool, txIndex, blocks, j, oldTx, oldRaw, opts, relayTx, net)
	}
	var rawHex string
	if err := json.Unmarshal(rawField, &rawHex); err != nil {
		return nil, -8, "bumpfee: rawtx must be a hex string"
	}
	rawHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(rawHex), "0x"))
	newRaw, err := hex.DecodeString(rawHex)
	if err != nil || len(newRaw) == 0 {
		return nil, -22, "TX decode failed"
	}
	if len(newRaw) > maxRawTxBytes {
		return nil, -8, fmt.Sprintf("bumpfee: transaction too large (%d bytes)", len(newRaw))
	}
	newTx, err := wire.DeserializeTx(newRaw)
	if err != nil {
		return nil, -22, "TX decode failed"
	}
	return submitBumpFeeReplacement(oldTx, oldRaw, newTx, newRaw, pool, txIndex, blocks, j, paths, relayTx, net)
}

func bumpFeePrevOutView(pool *mempool.Pool, paths *DataPaths, txIndex *store.TxIndex, blocks *store.RawBlockStore) consensus.PrevOutView {
	var utxo consensus.UtxoOutpointSource
	if paths != nil && paths.Utxo != nil {
		utxo = paths.Utxo
	}
	return consensus.AdmissionPrevOutViewWithUtxo(pool, utxo, txIndex, blocks)
}

func txDoubleSpendsSameInputs(a, b *wire.Tx) bool {
	if a == nil || b == nil {
		return false
	}
	for i := range a.Vin {
		ain := &a.Vin[i]
		if consensus.IsNullOutpoint(ain) {
			continue
		}
		for j := range b.Vin {
			bin := &b.Vin[j]
			if consensus.IsNullOutpoint(bin) {
				continue
			}
			if ain.PrevHash == bin.PrevHash && ain.PrevIdx == bin.PrevIdx {
				return true
			}
		}
	}
	return false
}
