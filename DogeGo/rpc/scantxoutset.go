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
	"dogego/store"
)

// execScanTxOutSet scans the UTXO set for output-descriptor matches (native cache or Core chainstate).
func execScanTxOutSet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, utxo *store.UtxoCache, syncUtxo func() error, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var action string
	if err := json.Unmarshal(params[0], &action); err != nil {
		return nil, -8, "scantxoutset: action must be a string"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "abort":
		return false, 0, ""
	case "status":
		return nil, 0, ""
	case "start":
		if j == nil {
			return nil, -1, "scantxoutset: header journal not available"
		}
		if len(params) < 2 {
			return nil, -32602, "scantxoutset: scanobjects required for start"
		}
		objects, code, msg := parseScanObjectsParam(params[1])
		if code != 0 {
			return nil, code, msg
		}
		matchers, code, msg := buildScanTxOutMatchers(chainName, objects)
		if code != 0 {
			return nil, code, msg
		}
		chainTip, _, _ := activeChainFromJournal(j, raw, paths)
		if utxo == nil {
			return nil, -1, "scantxoutset: UTXO cache not available"
		}
		if syncUtxo != nil {
			_ = syncUtxo()
		}
		if utxo.TipHeight() != chainTip {
			return nil, -1, "scantxoutset: UTXO cache is not synced to chainActive tip"
		}
		return runScanTxOutSet(chainName, j, raw, txIx, utxo, chainTip, matchers)
	default:
		return nil, -8, "scantxoutset: unknown action " + action
	}
}

func parseScanObjectsParam(raw json.RawMessage) ([]scanObjectDesc, int, string) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, -8, "scantxoutset: scanobjects must be a JSON array"
	}
	if len(arr) == 0 {
		return nil, -8, "scantxoutset: scanobjects array is empty"
	}
	out := make([]scanObjectDesc, 0, len(arr))
	for i, el := range arr {
		var s string
		if err := json.Unmarshal(el, &s); err == nil && strings.TrimSpace(s) != "" {
			out = append(out, scanObjectDesc{desc: s})
			continue
		}
		var obj struct {
			Desc  string          `json:"desc"`
			Range json.RawMessage `json:"range"`
		}
		if err := json.Unmarshal(el, &obj); err != nil || strings.TrimSpace(obj.Desc) == "" {
			return nil, -8, "scantxoutset: invalid scan object at index " + strconv.Itoa(i)
		}
		hasRange := len(obj.Range) > 0 && string(obj.Range) != "null"
		out = append(out, scanObjectDesc{desc: obj.Desc, hasRange: hasRange})
	}
	return out, 0, ""
}

func runScanTxOutSet(chainName string, j HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, utxo *store.UtxoCache, chainTip int64, matchers []scanTxOutMatcher) (interface{}, int, string) {
	best, err := blockHashHexAt(j, chainTip)
	if err != nil {
		return nil, -1, "scantxoutset: " + err.Error()
	}
	var unspents []map[string]interface{}
	var totalKoinu int64
	var txoutCount int
	utxo.ForEachRow(func(row store.UtxoDumpRow) bool {
		txoutCount++
		var matchedDesc string
		for _, m := range matchers {
			if m.Match(row.PkScript) {
				matchedDesc = m.Desc
				break
			}
		}
		if matchedDesc == "" {
			return true
		}
		conf := int64(0)
		if chainTip >= 0 && row.Height >= 0 {
			conf = chainTip - row.Height + 1
		}
		blockHash := ""
		if row.Height >= 0 {
			blockHash = walletBlockHashAt(j, row.Height)
		}
		entry := map[string]interface{}{
			"txid":          row.TxID,
			"vout":          row.Vout,
			"scriptPubKey":  hex.EncodeToString(row.PkScript),
			"desc":          matchedDesc,
			"amount":        float64(row.Value) / 1e8,
			"height":        row.Height,
			"confirmations": conf,
		}
		if blockHash != "" {
			entry["blockhash"] = blockHash
		}
		if isCoinbaseUTXO(txIx, raw, row.TxID) {
			entry["coinbase"] = true
		} else {
			entry["coinbase"] = false
		}
		unspents = append(unspents, entry)
		totalKoinu += row.Value
		return true
	})
	return map[string]interface{}{
		"success":      true,
		"txouts":       txoutCount,
		"height":       chainTip,
		"bestblock":    best,
		"unspents":     unspents,
		"total_amount": float64(totalKoinu) / 1e8,
	}, 0, ""
}

func isCoinbaseUTXO(txIx *store.TxIndex, raw *store.RawBlockStore, txid string) bool {
	if txIx == nil || raw == nil {
		return false
	}
	txidNorm := strings.ToLower(strings.TrimPrefix(txid, "0x"))
	hit, err := txIx.LookupHit(txidNorm)
	if err != nil {
		return false
	}
	tx, err := store.LoadIndexedTx(txIx, raw, txidNorm)
	if err != nil {
		return false
	}
	return hit.TxIndex == 0 && consensus.IsCoinbaseTx(tx)
}
