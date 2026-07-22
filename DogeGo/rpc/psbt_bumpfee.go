// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execPsbtBumpFee returns a signed PSBT for a fee-bumped replacement (Core psbtbumpfee; does not broadcast).
func execPsbtBumpFee(chainName string, paths *DataPaths, pool *mempool.Pool, txIndex *store.TxIndex, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "psbtbumpfee: mempool not available"
	}
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "psbtbumpfee: built-in wallet is not available"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	rpcTxid, code, msg := validateWalletTxIDParam(params[0], "psbtbumpfee", "txid")
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
	var opts map[string]json.RawMessage
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "psbtbumpfee: options must be a JSON object"
		}
	}
	newTx, origFee, newFee, code, msg := buildWalletBumpFeeTx(chainName, paths, pool, txIndex, raw, oldTx, oldRaw, opts, "psbtbumpfee")
	if code != 0 {
		return nil, code, msg
	}
	psbt, err := wire.NewPsbtFromTx(newTx)
	if err != nil {
		return nil, -8, "psbtbumpfee: " + err.Error()
	}
	fillPsbtPrevouts(psbt, txIndex, raw, pool)
	applyWalletSpendTimelocks(psbt.UnsignedTx, paths)
	signPsbtWithWallet(chainName, paths, psbt, wire.SigHashAll)
	b64, code, msg := encodePSBTBase64(psbt)
	if code != 0 {
		return nil, code, "psbtbumpfee: " + msg
	}
	return map[string]interface{}{
		"psbt":    b64,
		"origfee": float64(origFee) / 1e8,
		"fee":     float64(newFee) / 1e8,
		"errors":  []interface{}{},
	}, 0, ""
}

// execSimulateRawTransaction estimates wallet balance change from broadcasting raw txs (Core subset).
func execSimulateRawTransaction(chainName string, paths *DataPaths, pool *mempool.Pool, txIndex *store.TxIndex, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "simulaterawtransaction: built-in wallet is not available"
	}
	var hexList []string
	if err := json.Unmarshal(params[0], &hexList); err != nil {
		return nil, -8, "simulaterawtransaction: expected array of hex-encoded raw transactions"
	}
	if len(hexList) == 0 {
		return nil, -8, "simulaterawtransaction: expected array of hex-encoded raw transactions"
	}
	includeWatch := false
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "simulaterawtransaction: options must be a JSON object"
		}
		if v, ok := opts["include_watchonly"]; ok {
			includeWatch, _, _ = parseRPCBoolOpt(v, false, "simulaterawtransaction", "include_watchonly")
		}
	}
	scripts := rpcWalletSpendScripts(paths)
	if includeWatch {
		seen := walletScriptSet(scripts)
		for _, pk := range rpcWalletWatchScripts(paths) {
			if _, ok := seen[hex.EncodeToString(pk)]; !ok {
				scripts = append(scripts, pk)
			}
		}
	}
	spendSet := walletScriptSet(scripts)
	view := bumpFeePrevOutView(pool, paths, txIndex, raw)
	var deltaKoinu int64
	for i, hx := range hexList {
		hx = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hx), "0x"))
		rawTx, err := hex.DecodeString(hx)
		if err != nil || len(rawTx) == 0 {
			return nil, -22, "simulaterawtransaction: invalid tx at index "+strconv.Itoa(i)
		}
		if len(rawTx) > maxRawTxBytes {
			return nil, -8, "simulaterawtransaction: transaction too large at index "+strconv.Itoa(i)
		}
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			return nil, -22, "TX decode failed"
		}
		d, err := walletTxBalanceDeltaKoinu(tx, spendSet, view)
		if err != nil {
			return nil, -8, "simulaterawtransaction: " + err.Error()
		}
		deltaKoinu += d
	}
	return float64(deltaKoinu) / 1e8, 0, ""
}

func walletTxBalanceDeltaKoinu(tx *wire.Tx, spendSet map[string]struct{}, view consensus.PrevOutView) (int64, error) {
	if tx == nil {
		return 0, nil
	}
	var walletIn, walletOut int64
	for _, in := range tx.Vin {
		if consensus.IsNullOutpoint(&in) {
			continue
		}
		prev, ok := view.Lookup(in.PrevHash, in.PrevIdx)
		if !ok {
			return 0, consensus.ErrMissingPrevout
		}
		val, spk := prev.Value, prev.PkScript
		if _, isWallet := spendSet[hex.EncodeToString(spk)]; isWallet {
			walletIn += val
		}
	}
	for _, out := range tx.Vout {
		if _, isWallet := spendSet[hex.EncodeToString(out.PkScript)]; isWallet {
			walletOut += out.Value
		}
	}
	return walletOut - walletIn, nil
}
