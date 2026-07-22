// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"

	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

// execListStuckTransactions matches Dogecoin Core arity (0-2 bool params). DogeGo has no wallet;
// there are no “stuck” wallet transactions, so the result is always an empty array (Core-shaped).
func execListStuckTransactions(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 {
		if _, code, msg := parseRPCBoolOpt(params[0], false, "liststucktransactions", "verbose"); code != 0 {
			return nil, code, msg
		}
	}
	if len(params) > 1 {
		if _, code, msg := parseRPCBoolOpt(params[1], false, "liststucktransactions", "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	return []interface{}{}, 0, ""
}

func parseRPCBoolOpt(raw json.RawMessage, def bool, method, field string) (bool, int, string) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return def, 0, ""
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return def, -8, method + ": invalid " + field
	}
	switch t := v.(type) {
	case bool:
		return t, 0, ""
	case float64:
		if t == 0 {
			return false, 0, ""
		}
		if t == 1 {
			return true, 0, ""
		}
	}
	return def, -8, method + ": invalid " + field
}

func validateHexSerializedTx(raw json.RawMessage, method string) (int, string) {
	var hexStr string
	if err := json.Unmarshal(raw, &hexStr); err != nil {
		return -8, method + ": hexstring must be a string"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	txb, err := hex.DecodeString(hexStr)
	if err != nil || len(txb) == 0 {
		return -8, method + ": TX decode failed"
	}
	if _, err := wire.DeserializeTx(txb); err != nil {
		return -8, method + ": TX decode failed"
	}
	return 0, ""
}

// execRescan matches Dogecoin wallet rescan arity (optional start height); DogeGo has no wallet DB to rescan.
func execRescan(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		var h json.Number
		if err := json.Unmarshal(params[0], &h); err != nil {
			return nil, -8, "rescan: height must be a number"
		}
		hi, err := h.Int64()
		if err != nil || hi < 0 {
			return nil, -8, "rescan: height out of range"
		}
	}
	return nil, -1, "rescan: wallet rescan is not implemented in DogeGo"
}

// execListLockUnspent matches Core listlockunspent (no params); DogeGo has no wallet locks - empty array.
func execListLockUnspent(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	return []interface{}{}, 0, ""
}

// execLockUnspent matches Core lockunspent arity and shape; without a wallet this is a validated no-op returning true.
func execLockUnspent(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCBoolOpt(params[0], false, "lockunspent", "unlock"); code != 0 {
		return nil, code, msg
	}
	if len(params) == 1 {
		return true, 0, ""
	}
	if strings.TrimSpace(string(params[1])) == "null" {
		return nil, -8, "lockunspent: transactions must be a JSON array"
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(params[1], &arr); err != nil {
		return nil, -8, "lockunspent: transactions must be a JSON array"
	}
	for _, elem := range arr {
		var o lockUnspentOut
		if err := json.Unmarshal(elem, &o); err != nil {
			return nil, -8, "lockunspent: Invalid parameter, expected object"
		}
		txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(o.Txid), "0x"))
		if len(txid) != 64 {
			return nil, -8, "lockunspent: Invalid parameter, expected hex txid"
		}
		if _, err := chain.Hash256FromDisplayHex(txid); err != nil {
			return nil, -8, "lockunspent: Invalid parameter, expected hex txid"
		}
		if o.Vout < 0 {
			return nil, -8, "lockunspent: Invalid parameter, vout must be positive"
		}
	}
	return true, 0, ""
}

// execKeypoolRefill matches Core optional newsize; DogeGo has no wallet keypool.
func execKeypoolRefill(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "keypoolrefill: newsize must be a number"
		}
		ni, err := n.Int64()
		if err != nil || ni < 0 {
			return nil, -8, "keypoolrefill: invalid newsize"
		}
	}
	return nil, -1, "keypoolrefill: wallet is not implemented in DogeGo"
}

var listUnspentQueryOptionKeys = map[string]struct{}{
	"minimumAmount":    {},
	"maximumAmount":    {},
	"minimumSumAmount": {},
	"maximumCount":     {},
}

func validateListUnspentQueryOptions(raw json.RawMessage) (int, string) {
	var opts map[string]json.RawMessage
	if err := json.Unmarshal(raw, &opts); err != nil {
		return -8, "listunspent: query_options must be a JSON object"
	}
	for k, elem := range opts {
		if _, ok := listUnspentQueryOptionKeys[k]; !ok {
			return -8, "listunspent: unknown query_options key"
		}
		if strings.TrimSpace(string(elem)) == "null" {
			return -8, "listunspent: invalid " + k
		}
		if k == "maximumCount" {
			var f float64
			if err := json.Unmarshal(elem, &f); err == nil {
				if f < 0 || f != math.Trunc(f) {
					return -8, "listunspent: invalid maximumCount"
				}
				continue
			}
			var s string
			if err := json.Unmarshal(elem, &s); err != nil || strings.TrimSpace(s) == "" {
				return -8, "listunspent: invalid maximumCount"
			}
			var n json.Number = json.Number(strings.TrimSpace(s))
			hi, err := n.Int64()
			if err != nil || hi < 0 {
				return -8, "listunspent: invalid maximumCount"
			}
			continue
		}
		var f float64
		var s string
		if err := json.Unmarshal(elem, &f); err == nil {
			if f < 0 {
				return -8, "listunspent: invalid " + k
			}
			continue
		}
		if err := json.Unmarshal(elem, &s); err == nil && strings.TrimSpace(s) != "" {
			continue
		}
		return -8, "listunspent: invalid " + k
	}
	return 0, ""
}

// execListTransactions matches Core listtransactions (0-4); empty when no wallet.
func execListTransactions(params []json.RawMessage) (interface{}, int, string) {
	if code, msg := execListTransactionsValidate(params); code != 0 {
		return nil, code, msg
	}
	return []interface{}{}, 0, ""
}

// execGetTransaction matches Core gettransaction (1-2); validates txid then reports absent wallet tx (Core-shaped -5).
func execGetTransaction(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "gettransaction: txid must be a string"
	}
	txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	if len(txid) != 64 {
		return nil, -8, "gettransaction: invalid txid"
	}
	if _, err := chain.Hash256FromDisplayHex(txid); err != nil {
		return nil, -8, "gettransaction: invalid txid"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[1], false, "gettransaction", "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	return nil, -5, "Invalid or non-wallet transaction id"
}

// execGetReceivedByAccount matches Core deprecated getreceivedbyaccount (1-2); DogeGo has no wallet - 0.
func execGetReceivedByAccount(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var account string
	if err := json.Unmarshal(params[0], &account); err != nil {
		return nil, -8, "getreceivedbyaccount: account must be a string"
	}
	_ = account
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getreceivedbyaccount: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "getreceivedbyaccount: minconf out of range"
		}
	}
	return 0.0, 0, ""
}

func parseSetTxFeeAmount(raw json.RawMessage) (float64, int, string) {
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		f, err := n.Float64()
		if err != nil || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, -8, "settxfee: invalid amount"
		}
		return f, 0, ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, -8, "settxfee: amount must be a number or string"
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, -8, "settxfee: invalid amount"
	}
	f, err := json.Number(s).Float64()
	if err != nil || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, -8, "settxfee: invalid amount"
	}
	return f, 0, ""
}

func execListReceivedCommon(params []json.RawMessage, method string) (interface{}, int, string) {
	if len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, method + ": minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, method + ": minconf out of range"
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[1], false, method, "include_empty"); code != 0 {
			return nil, code, msg
		}
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[2], false, method, "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	return []interface{}{}, 0, ""
}

// execListReceivedByAddress matches Core (0-3); DogeGo has no wallet - empty array.
func execListReceivedByAddress(params []json.RawMessage) (interface{}, int, string) {
	return execListReceivedCommon(params, "listreceivedbyaddress")
}

// execListReceivedByAccount matches Core deprecated listreceivedbyaccount (0-3); DogeGo has no wallet - empty array.
func execListReceivedByAccount(params []json.RawMessage) (interface{}, int, string) {
	return execListReceivedCommon(params, "listreceivedbyaccount")
}

// execListReceivedByLabel matches Core listreceivedbylabel (0-3); DogeGo has no wallet - empty array.
func execListReceivedByLabel(params []json.RawMessage) (interface{}, int, string) {
	return execListReceivedCommon(params, "listreceivedbylabel")
}

// execGetReceivedByLabel matches Core getreceivedbylabel (1-3); DogeGo has no wallet - zero.
func execGetReceivedByLabel(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var label string
	if err := json.Unmarshal(params[0], &label); err != nil {
		return nil, -8, "getreceivedbylabel: label must be a string"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return nil, -8, "getreceivedbylabel: minconf must be a number"
		}
		if _, err := n.Int64(); err != nil {
			return nil, -8, "getreceivedbylabel: minconf out of range"
		}
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[2], false, "getreceivedbylabel", "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	return 0.0, 0, ""
}

// execListAccounts matches Core deprecated listaccounts (0-2); DogeGo has no wallet - default account balance 0.
func execListAccounts(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "listaccounts: minconf must be a number"
		}
		mi, err := n.Int64()
		if err != nil || mi < 0 {
			return nil, -8, "listaccounts: minconf out of range"
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if _, code, msg := parseRPCBoolOpt(params[1], false, "listaccounts", "include_watchonly"); code != 0 {
			return nil, code, msg
		}
	}
	return map[string]interface{}{"": 0.0}, 0, ""
}

// execGetAccount matches Core deprecated getaccount (1); DogeGo has no address book - empty string for valid P2PKH.
func execGetAccount(chainName string, params []json.RawMessage) (interface{}, int, string) {
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
	return "", 0, ""
}

// execGetAddressesByLabel matches Core getaddressesbylabel (1); DogeGo has no wallet - empty object.
func execGetAddressesByLabel(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var label string
	if err := json.Unmarshal(params[0], &label); err != nil {
		return nil, -8, "getaddressesbylabel: label must be a string"
	}
	return map[string]interface{}{}, 0, ""
}

// execGetAddressesByAccount matches Core deprecated getaddressesbyaccount (1); DogeGo has no wallet - empty array.
func execGetAddressesByAccount(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var account string
	if err := json.Unmarshal(params[0], &account); err != nil {
		return nil, -8, "getaddressesbyaccount: account must be a string"
	}
	if strings.TrimSpace(account) == "*" {
		return nil, -8, "Invalid account name"
	}
	return []interface{}{}, 0, ""
}

// execListAddressGroupings matches Core listaddressgroupings (no params); DogeGo has no wallet - empty array.
func execListAddressGroupings(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	return []interface{}{}, 0, ""
}

// parseRPCAccountLabel unmarshals a string account name; "*" is invalid (Core AccountFromValue).
func parseRPCAccountLabel(raw json.RawMessage, method, field string) (string, int, string) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", -8, method + ": " + field + " must be a string"
	}
	if s == "*" {
		return "", -8, "Invalid account name"
	}
	return s, 0, ""
}

// execGetAccountAddress matches Core deprecated getaccountaddress (1); DogeGo has no wallet keypool.
func execGetAccountAddress(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "getaccountaddress", "account"); code != 0 {
		return nil, code, msg
	}
	return nil, -1, "getaccountaddress: wallet is not implemented in DogeGo"
}

// execWalletLock matches Core walletlock (no params); DogeGo has no encrypted wallet state.
func execWalletLock(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	return nil, -1, "walletlock: wallet is not implemented in DogeGo"
}

// execWalletPassphrase matches Core walletpassphrase (2); DogeGo has no encrypted wallet.
func execWalletPassphrase(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var phrase string
	if err := json.Unmarshal(params[0], &phrase); err != nil {
		return nil, -8, "walletpassphrase: passphrase must be a string"
	}
	if strings.TrimSpace(phrase) == "" {
		return nil, -8, "walletpassphrase: passphrase must not be empty"
	}
	var n json.Number
	if err := json.Unmarshal(params[1], &n); err != nil {
		return nil, -8, "walletpassphrase: timeout must be a number"
	}
	sec, err := n.Int64()
	if err != nil || sec < 0 {
		return nil, -8, "walletpassphrase: timeout out of range"
	}
	return nil, -1, "walletpassphrase: wallet is not implemented in DogeGo"
}

// execWalletPassphraseChange matches Core walletpassphrasechange (2); DogeGo has no encrypted wallet.
func execWalletPassphraseChange(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var oldp, newp string
	if err := json.Unmarshal(params[0], &oldp); err != nil {
		return nil, -8, "walletpassphrasechange: oldpassphrase must be a string"
	}
	if err := json.Unmarshal(params[1], &newp); err != nil {
		return nil, -8, "walletpassphrasechange: newpassphrase must be a string"
	}
	if strings.TrimSpace(oldp) == "" || strings.TrimSpace(newp) == "" {
		return nil, -8, "walletpassphrasechange: passphrases must not be empty"
	}
	return nil, -1, "walletpassphrasechange: wallet is not implemented in DogeGo"
}

// execSetAccount matches Core deprecated setaccount (1-2); DogeGo has no address book - error after validation.
func execSetAccount(chainName string, params []json.RawMessage) (interface{}, int, string) {
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
	return nil, -1, "setaccount can only be used with own address"
}

// execMove matches Core deprecated move (3-5); DogeGo has no wallet - validated error.
func execMove(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 3 || len(params) > 5 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "move", "fromaccount"); code != 0 {
		return nil, code, msg
	}
	if _, code, msg := parseRPCAccountLabel(params[1], "move", "toaccount"); code != 0 {
		return nil, code, msg
	}
	amt, code, msg := parseMoveAmount(params[2])
	if code != 0 {
		return nil, code, msg
	}
	if amt <= 0 {
		return nil, -3, "Invalid amount for send"
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[3], &n); err != nil {
			return nil, -8, "move: minconf must be a number"
		}
		if _, err := n.Int64(); err != nil {
			return nil, -8, "move: minconf must be a number"
		}
	}
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var comment string
		if err := json.Unmarshal(params[4], &comment); err != nil {
			return nil, -8, "move: comment must be a string"
		}
	}
	return nil, -1, "move: wallet is not implemented in DogeGo"
}

func parseRPCAmountField(raw json.RawMessage, method, field string) (float64, int, string) {
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		f, err := n.Float64()
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, -8, method + ": invalid " + field
		}
		return f, 0, ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, -8, method + ": " + field + " must be a number or string"
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, -8, method + ": invalid " + field
	}
	f, err := json.Number(s).Float64()
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, -8, method + ": invalid " + field
	}
	return f, 0, ""
}

func parseMoveAmount(raw json.RawMessage) (float64, int, string) {
	return parseRPCAmountField(raw, "move", "amount")
}

func isHexScriptArg(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// parseListSinceBlockParams validates listsinceblock (0-3) and returns since height, minconf, and filters.
func parseListSinceBlockParams(j HeaderJournal, params []json.RawMessage) (sinceHeight int64, hasSince bool, minConf int64, includeWatchonly bool, code int, msg string) {
	minConf = 1
	if len(params) > 3 {
		return 0, false, 1, false, -32602, "Wrong number of arguments"
	}
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var h string
		if err := json.Unmarshal(params[0], &h); err != nil {
			return 0, false, 1, false, -8, "listsinceblock: blockhash must be a string"
		}
		h = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(h), "0x"))
		if h != "" && j != nil {
			if hi, err := j.HeightByDisplayHash(h); err == nil {
				sinceHeight, hasSince = hi, true
			}
			// Core accepts unknown block hashes (no height filter).
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[1], &n); err != nil {
			return 0, false, 1, false, -8, "listsinceblock: target_confirmations must be a number"
		}
		tgt, err := n.Int64()
		if err != nil || tgt < 1 {
			return 0, false, 1, false, -8, "Invalid parameter"
		}
		minConf = tgt
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var c int
		var m string
		includeWatchonly, c, m = parseRPCBoolOpt(params[2], false, "listsinceblock", "include_watchonly")
		if c != 0 {
			return 0, false, 1, false, c, m
		}
	}
	return sinceHeight, hasSince, minConf, includeWatchonly, 0, ""
}

// execListSinceBlock matches Core listsinceblock (0-3); DogeGo has no wallet - empty transactions + chainActive lastblock.
func execListSinceBlock(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if _, _, _, _, code, msg := parseListSinceBlockParams(j, params); code != 0 {
		return nil, code, msg
	}
	last := ""
	if j != nil {
		if _, h, err := ChainActiveTip(j, raw, paths); err == nil {
			last = h
		}
	}
	return map[string]interface{}{
		"transactions": []interface{}{},
		"lastblock":    last,
	}, 0, ""
}

// execImportPrivKey matches Core importprivkey (1-4); replaces the built-in single-key wallet when wired.
func execImportPrivKey(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 4 {
		return nil, -32602, "Wrong number of arguments"
	}
	var wif string
	if err := json.Unmarshal(params[0], &wif); err != nil {
		return nil, -8, "importprivkey: privkey must be a string"
	}
	wif = strings.TrimSpace(wif)
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -1, "importprivkey: unknown chain"
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -1, "importprivkey: unknown chain"
	}
	if _, _, err := chain.DecodeWIF(wif, p.PrivKeyWIFVersion); err != nil {
		return nil, -5, "Invalid private key encoding"
	}
	var importLabel string
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var label string
		if err := json.Unmarshal(params[1], &label); err != nil {
			return nil, -8, "importprivkey: label must be a string"
		}
		importLabel = strings.TrimSpace(label)
	}
	rescan := true
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var code int
		var msg string
		rescan, code, msg = parseRPCBoolOpt(params[2], true, "importprivkey", "rescan")
		if code != 0 {
			return nil, code, msg
		}
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		if !rescan {
			// Core ignores height when rescan is false.
		} else {
			var n json.Number
			if err := json.Unmarshal(params[3], &n); err != nil {
				return nil, -8, "importprivkey: height must be a number"
			}
			hi, err := n.Int64()
			if err != nil || hi < 0 {
				return nil, -8, "importprivkey: height out of range"
			}
			if j != nil {
				if hi > ActiveChainBlockHeight(j, raw, paths) {
					return nil, -8, "Block height out of range"
				}
			}
		}
	}
	if paths == nil || paths.WalletImportPrivKey == nil {
		return nil, -1, "importprivkey: wallet is not implemented in DogeGo"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	if err := paths.WalletImportPrivKey(wif); err != nil {
		if code, msg := rpcWalletOpErr(err); code != 0 {
			if code == -13 {
				return nil, code, msg
			}
			return nil, -5, "Invalid private key encoding"
		}
		return nil, -5, "Invalid private key encoding"
	}
	if importLabel != "" && paths.WalletSetLabel != nil {
		if addr, err := addressFromWIF(chainName, wif); err == nil && addr != "" {
			_ = paths.WalletSetLabel(addr, importLabel)
		}
	}
	if rescan {
		if code, msg := walletRescanAfterImport(paths, j, raw, params, 3, "importprivkey"); code != 0 {
			return nil, code, msg
		}
	}
	return nil, 0, ""
}

func validateWalletTxIDParam(raw json.RawMessage, method, field string) (string, int, string) {
	var txid string
	if err := json.Unmarshal(raw, &txid); err != nil {
		return "", -8, method + ": " + field + " must be a string"
	}
	txid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txid), "0x"))
	if len(txid) != 64 {
		return "", -8, method + ": invalid " + field
	}
	if _, err := chain.Hash256FromDisplayHex(txid); err != nil {
		return "", -8, method + ": invalid " + field
	}
	return txid, 0, ""
}

// execSignMessage matches Core signmessage (2); uses the built-in wallet key when address matches.
func execSignMessage(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "signmessage: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -3, "Invalid address"
	}
	var msg string
	if err := json.Unmarshal(params[1], &msg); err != nil {
		return nil, -8, "signmessage: message must be a string"
	}
	if !rpcWalletContainsAddress(paths, addr) {
		return nil, -4, "Private key not available"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	var wif string
	if paths != nil && paths.WalletWIFForAddress != nil {
		w, err := paths.WalletWIFForAddress(addr)
		if err != nil {
			return nil, -4, "Private key not available"
		}
		wif = w
	} else {
		wif = rpcWalletWIF(paths)
	}
	if wif == "" {
		return nil, -4, "Private key not available"
	}
	msgJ, err := json.Marshal(msg)
	if err != nil {
		return nil, -8, "signmessage: internal error"
	}
	wifJ, err := json.Marshal(wif)
	if err != nil {
		return nil, -8, "signmessage: internal error"
	}
	sig, code, smsg := execSignMessageWithPrivkey(chainName, []json.RawMessage{msgJ, wifJ})
	if code != 0 {
		return nil, code, smsg
	}
	return sig, 0, ""
}

// execRemovePrunedFunds matches Core removeprunedfunds (1); built-in wallet drops abandoned tx records.
func execRemovePrunedFunds(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	txid, code, msg := validateWalletTxIDParam(params[0], "removeprunedfunds", "txid")
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || rpcWalletAddress(paths) == "" {
		return nil, -8, "Transaction does not exist in wallet."
	}
	removed := false
	if paths.WalletRemoveAbandoned != nil {
		removed = paths.WalletRemoveAbandoned(txid)
	}
	if !removed && paths.WalletRemovePrunedImport != nil {
		removed = paths.WalletRemovePrunedImport(txid)
	}
	if !removed {
		return nil, -8, "Transaction does not exist in wallet."
	}
	if paths.WalletRemoveReplacementsForTx != nil {
		_ = paths.WalletRemoveReplacementsForTx(txid)
	}
	return nil, 0, ""
}

// execImportMulti matches Core importmulti (1-2); DogeGo returns per-request failure objects.
func execImportMulti(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if strings.TrimSpace(string(params[0])) == "null" {
		return nil, -8, "importmulti: requests must be a JSON array"
	}
	var reqs []json.RawMessage
	if err := json.Unmarshal(params[0], &reqs); err != nil {
		return nil, -8, "importmulti: requests must be a JSON array"
	}
	if len(reqs) == 0 {
		return nil, -8, "importmulti: requests must not be empty"
	}
	for _, elem := range reqs {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(elem, &m); err != nil {
			return nil, -8, "importmulti: each request must be a JSON object"
		}
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "importmulti: options must be a JSON object"
		}
	}
	out := make([]map[string]interface{}, 0, len(reqs))
	for range reqs {
		out = append(out, map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    -1,
				"message": "importmulti: wallet is not implemented in DogeGo",
			},
		})
	}
	return out, 0, ""
}

// execAddMultisigAddress matches Core addmultisigaddress (2-3); reuses createmultisig validation, then wallet stub.
func execAddMultisigAddress(chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 3 && strings.TrimSpace(string(params[2])) != "null" {
		if _, code, msg := parseRPCAccountLabel(params[2], "addmultisigaddress", "account"); code != 0 {
			return nil, code, msg
		}
	}
	if _, code, msg := execCreateMultisig(chainName, params[:2]); code != 0 {
		return nil, code, msg
	}
	return nil, -1, "addmultisigaddress: wallet is not implemented in DogeGo"
}

// execAddWitnessAddress matches Core addwitnessaddress (1); DogeGo has no segwit wallet path.
func execAddWitnessAddress(chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "addwitnessaddress: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	return nil, -4, "Segregated witness not enabled on network"
}

// execEncryptWallet matches Core encryptwallet (1); DogeGo has no wallet file to encrypt.
func execEncryptWallet(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var phrase string
	if err := json.Unmarshal(params[0], &phrase); err != nil {
		return nil, -8, "encryptwallet: passphrase must be a string"
	}
	if strings.TrimSpace(phrase) == "" {
		return nil, -8, "encryptwallet: passphrase must not be empty"
	}
	return nil, -1, "encryptwallet: wallet is not implemented in DogeGo"
}
