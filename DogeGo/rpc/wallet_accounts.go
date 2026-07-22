// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/store"
)

// execListAccountsWallet returns the default account balance (deprecated Core RPC).
func execListAccountsWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	minConf := int64(1)
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "listaccounts: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "listaccounts: minconf out of range"
		}
		minConf = mi
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[1], false, "listaccounts", "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	if rpcWalletAddress(paths) == "" {
		return map[string]interface{}{"": 0.0}, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	var sum int64
	for _, m := range matches {
		sum += m.row.Value
	}
	return map[string]interface{}{"": float64(sum) / 1e8}, 0, ""
}

// execGetAddressesByAccountWallet lists addresses for the default account (all tracked when HD).
func execGetAddressesByAccountWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "getaddressesbyaccount", "account"); code != 0 {
		return nil, code, msg
	}
	if rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	addrs, code, msg := rpcWalletTrackedAddresses(paths, chainName)
	if code != 0 {
		return nil, code, msg
	}
	out := make([]interface{}, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a)
	}
	return out, 0, ""
}

// execListAddressGroupingsWallet returns UTXO groupings for the spendable wallet address.
func execListAddressGroupingsWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, 1, 0)
	if code != 0 {
		return nil, code, msg
	}
	byAddr := make(map[string][]interface{})
	for _, m := range matches {
		if !m.spendable {
			continue
		}
		byAddr[m.address] = append(byAddr[m.address], []interface{}{m.address, float64(m.row.Value) / 1e8})
	}
	if len(byAddr) == 0 {
		return []interface{}{}, 0, ""
	}
	out := make([]interface{}, 0, len(byAddr))
	for _, group := range byAddr {
		out = append(out, group)
	}
	return out, 0, ""
}

// execListReceivedByAccountWallet lists amounts for the default account (deprecated Core RPC).
func execListReceivedByAccountWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	minConf := int64(1)
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "listreceivedbyaccount: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "listreceivedbyaccount: minconf out of range"
		}
		minConf = mi
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[1], false, "listreceivedbyaccount", "include_empty"); code != 0 {
			return nil, code, msg
		}
	}
	includeWatchonly := false
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var code int
		var msg string
		includeWatchonly, code, msg = parseRPCBoolOpt(params[2], false, "listreceivedbyaccount", "include_watchonly")
		if code != 0 {
			return nil, code, msg
		}
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	agg := &walletReceivedAgg{}
	for _, m := range matches {
		if !includeWatchonly && !m.spendable {
			continue
		}
		agg.addMatch(m)
	}
	if agg.amount == 0 {
		return []interface{}{}, 0, ""
	}
	return []interface{}{
		map[string]interface{}{
			"account":       "",
			"amount":        float64(agg.amount) / 1e8,
			"confirmations": agg.minConf,
			"txids":         walletReceivedAggTxids(agg),
		},
	}, 0, ""
}

// execGetReceivedByAccountWallet returns total received for the default account.
func execGetReceivedByAccountWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var account string
	if err := json.Unmarshal(params[0], &account); err != nil {
		return nil, -8, "getreceivedbyaccount: account must be a string"
	}
	account = strings.TrimSpace(account)
	if account == "*" {
		return nil, -8, "Invalid account name"
	}
	if account != "" {
		return 0.0, 0, ""
	}
	minConf := int64(1)
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getreceivedbyaccount: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "getreceivedbyaccount: minconf out of range"
		}
		minConf = mi
	}
	matches, code, msg := walletUtxoMatches(paths, j, raw, chainName, minConf, 0)
	if code != 0 {
		return nil, code, msg
	}
	var sum int64
	for _, m := range matches {
		sum += m.row.Value
	}
	return float64(sum) / 1e8, 0, ""
}

// execGetAccountAddressWallet returns the wallet P2PKH for the default account.
func execGetAccountAddressWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "getaccountaddress", "account"); code != 0 {
		return nil, code, msg
	}
	if rpcWalletAddress(paths) == "" {
		return nil, -1, "getaccountaddress: wallet is not implemented in DogeGo"
	}
	addr := rpcWalletAddress(paths)
	if paths.WalletPeekReceiveAddress != nil {
		if a := strings.TrimSpace(paths.WalletPeekReceiveAddress()); a != "" {
			addr = a
		}
	}
	return addr, 0, ""
}

// execSetAccountWallet is a no-op for tracked addresses (deprecated Core account bookkeeping).
func execSetAccountWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "setaccount: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCAccountLabel(params[1], "setaccount", "account"); code != 0 {
			return nil, code, msg
		}
	}
	if walletAddressIsTracked(paths, chainName, addr) {
		return nil, 0, ""
	}
	return nil, -1, "setaccount can only be used with own address"
}

// execGetAccountWallet returns the default account label for wallet-owned addresses.
func execGetAccountWallet(paths *DataPaths, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "getaccount: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	if rpcWalletAddress(paths) == "" {
		return "", 0, ""
	}
	if walletAddressIsTracked(paths, chainName, addr) {
		return "", 0, ""
	}
	return "", 0, ""
}
