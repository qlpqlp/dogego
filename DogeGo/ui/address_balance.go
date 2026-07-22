// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/store"
)

type rpcInvoker func(method string, params []json.RawMessage) map[string]interface{}

// attachAddressUTXOBalance adds utxo_balance to target from the in-memory UTXO cache (fast path).
func attachAddressUTXOBalance(target map[string]any, invoke rpcInvoker, utxoFn func() *store.UtxoCache, pubkeyVer, scriptHashVer byte, address string) {
	if target == nil {
		return
	}
	balance := map[string]any{"available": false}
	defer func() { target["utxo_balance"] = balance }()

	address = strings.TrimSpace(address)
	if address == "" {
		return
	}

	want := strings.ToLower(address)
	if utxoFn != nil {
		utxo := utxoFn()
		if utxo != nil && utxo.TipHeight() >= 0 {
			totalKoinu, count := utxo.SumRowsMatching(func(pkScript []byte) bool {
				a := chain.ScriptPubKeyAddress(pkScript, pubkeyVer, scriptHashVer)
				return a != "" && strings.ToLower(strings.TrimSpace(a)) == want
			})
			if count > 0 {
				balance["available"] = true
				balance["total_doge"] = float64(totalKoinu) / 1e8
				balance["utxo_count"] = count
				balance["height"] = utxo.TipHeight()
				return
			}
		}
	}

	if invoke == nil {
		balance["note"] = "RPC not available"
		return
	}
	action, _ := json.Marshal("start")
	desc := "addr(" + address + ")"
	scanObjs, _ := json.Marshal([]string{desc})
	resp := invoke("scantxoutset", []json.RawMessage{action, scanObjs})
	if resp == nil {
		return
	}
	if errM, ok := resp["error"].(map[string]interface{}); ok && errM != nil {
		msg, _ := errM["message"].(string)
		if strings.Contains(strings.ToLower(msg), "not synced") {
			balance["sync_pending"] = true
			balance["note"] = "Balance will appear when the UTXO cache catches up to the chain tip."
		} else if msg != "" {
			balance["error"] = msg
		} else {
			balance["error"] = "scantxoutset failed"
		}
		return
	}
	res, ok := resp["result"].(map[string]interface{})
	if !ok || res == nil {
		return
	}
	if success, _ := res["success"].(bool); !success {
		balance["error"] = "UTXO scan did not succeed"
		return
	}
	total, _ := res["total_amount"].(float64)
	unspents, _ := res["unspents"].([]interface{})
	balance["available"] = true
	balance["total_doge"] = total
	balance["utxo_count"] = len(unspents)
	if h, ok := res["height"]; ok {
		balance["height"] = h
	}
	if sp, ok := balance["sync_pending"].(bool); ok && sp && utxoFn != nil {
		if utxo := utxoFn(); utxo != nil && utxo.TipHeight() >= 0 {
			balance["note"] = "Balance at UTXO height " + strconv.FormatInt(utxo.TipHeight(), 10) + " - chain tip may be ahead while sync finishes."
			delete(balance, "error")
		}
	}
}
