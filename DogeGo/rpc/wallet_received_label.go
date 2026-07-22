// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"slices"
	"strings"

	"dogego/store"
)

type walletReceivedAgg struct {
	amount  int64
	minConf int64
	txidSet map[string]struct{}
}

func (a *walletReceivedAgg) addMatch(m walletUtxoMatch) {
	a.amount += m.row.Value
	conf := m.confirmations
	if a.minConf == 0 || (conf > 0 && conf < a.minConf) {
		a.minConf = conf
	}
	txid := strings.ToLower(strings.TrimSpace(m.row.TxID))
	if txid == "" {
		return
	}
	if a.txidSet == nil {
		a.txidSet = make(map[string]struct{})
	}
	a.txidSet[txid] = struct{}{}
}

func walletReceivedAggTxids(a *walletReceivedAgg) []interface{} {
	if a == nil || len(a.txidSet) == 0 {
		return []interface{}{}
	}
	ids := make([]string, 0, len(a.txidSet))
	for id := range a.txidSet {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func walletReceivedByAddress(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, minConf int64, includeEmpty, includeWatchonly bool) (map[string]*walletReceivedAgg, int, string) {
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return nil, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	byAddr := make(map[string]*walletReceivedAgg)
	for _, m := range matches {
		if !includeWatchonly && !m.spendable {
			continue
		}
		a := byAddr[m.address]
		if a == nil {
			a = &walletReceivedAgg{}
			byAddr[m.address] = a
		}
		a.addMatch(m)
	}
	if includeEmpty {
		addrs, c, m := rpcWalletTrackedAddresses(paths, chainName)
		if c != 0 {
			return nil, c, m
		}
		for _, addr := range addrs {
			if !includeWatchonly && addr != rpcWalletAddress(paths) {
				continue
			}
			if _, ok := byAddr[addr]; !ok {
				byAddr[addr] = &walletReceivedAgg{}
			}
		}
	}
	return byAddr, 0, ""
}

func walletReceivedByLabel(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, minConf int64, includeEmpty, includeWatchonly bool) (map[string]*walletReceivedAgg, int, string) {
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return nil, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	byLabel := make(map[string]*walletReceivedAgg)
	for _, m := range matches {
		if !includeWatchonly && !m.spendable {
			continue
		}
		lbl := rpcWalletGetLabel(paths, m.address)
		a := byLabel[lbl]
		if a == nil {
			a = &walletReceivedAgg{}
			byLabel[lbl] = a
		}
		a.addMatch(m)
	}
	if includeEmpty {
		addrs, c, m := rpcWalletTrackedAddresses(paths, chainName)
		if c != 0 {
			return nil, c, m
		}
		for _, addr := range addrs {
			if !includeWatchonly && addr != rpcWalletAddress(paths) {
				continue
			}
			lbl := rpcWalletGetLabel(paths, addr)
			if _, ok := byLabel[lbl]; !ok {
				byLabel[lbl] = &walletReceivedAgg{}
			}
		}
	}
	return byLabel, 0, ""
}

func parseListReceivedParams(params []json.RawMessage, method string) (minConf int64, includeEmpty, includeWatchonly bool, code int, msg string) {
	minConf = 1
	if len(params) > 3 {
		return 0, false, false, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return 0, false, false, -8, method + ": minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return 0, false, false, -8, method + ": minconf out of range"
		}
		minConf = mi
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		includeEmpty, code, msg = parseRPCBoolOpt(params[1], false, method, "include_empty")
		if code != 0 {
			return 0, false, false, code, msg
		}
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		includeWatchonly, code, msg = parseRPCBoolOpt(params[2], false, method, "include_watchonly")
		if code != 0 {
			return 0, false, false, code, msg
		}
	}
	return minConf, includeEmpty, includeWatchonly, 0, ""
}

// execListReceivedByLabelWallet groups UTXO receives by address label.
func execListReceivedByLabelWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	minConf, includeEmpty, includeWatchonly, code, msg := parseListReceivedParams(params, "listreceivedbylabel")
	if code != 0 {
		return nil, code, msg
	}
	byLabel, code, msg := walletReceivedByLabel(chainName, paths, j, raw, minConf, includeEmpty, includeWatchonly)
	if code != 0 {
		return nil, code, msg
	}
	if len(byLabel) == 0 {
		return []interface{}{}, 0, ""
	}
	labels := make([]string, 0, len(byLabel))
	for lbl := range byLabel {
		labels = append(labels, lbl)
	}
	slices.Sort(labels)
	out := make([]interface{}, 0, len(labels))
	for _, lbl := range labels {
		a := byLabel[lbl]
		out = append(out, map[string]interface{}{
			"account":       "",
			"amount":        float64(a.amount) / 1e8,
			"confirmations": a.minConf,
			"label":         lbl,
			"txids":         walletReceivedAggTxids(a),
		})
	}
	return out, 0, ""
}

// execGetReceivedByLabelWallet returns total received for a label.
func execGetReceivedByLabelWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var label string
	if err := json.Unmarshal(params[0], &label); err != nil {
		return nil, -8, "getreceivedbylabel: label must be a string"
	}
	label = strings.TrimSpace(label)
	minConf := int64(1)
	includeWatchonly := false
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getreceivedbylabel: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "getreceivedbylabel: minconf out of range"
		}
		minConf = mi
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var code int
		var msg string
		includeWatchonly, code, msg = parseRPCBoolOpt(params[2], false, "getreceivedbylabel", "include_watchonly")
		if code != 0 {
			return nil, code, msg
		}
	}
	byLabel, code, msg := walletReceivedByLabel(chainName, paths, j, raw, minConf, false, includeWatchonly)
	if code != 0 {
		return nil, code, msg
	}
	if a, ok := byLabel[label]; ok {
		return float64(a.amount) / 1e8, 0, ""
	}
	return 0.0, 0, ""
}
