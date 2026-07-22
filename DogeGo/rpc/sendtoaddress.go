// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// walletBuildSignBroadcast creates, funds, signs, and sends a P2PKH payment from the built-in wallet.
func walletBuildSignBroadcast(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	outputs map[string]float64,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
	errPrefix string,
	subtractFeeFromVouts []int,
	extraFundOpts map[string]interface{},
) (interface{}, int, string) {
	walletAddr := rpcWalletDefaultAddress(paths)
	if walletAddr == "" {
		return nil, -1, errPrefix + ": wallet is not implemented in DogeGo"
	}
	if code, msg := rpcWalletRequireMainnetEncrypted(chainName, paths); code != 0 {
		return nil, code, msg
	}
	if paths != nil && paths.WalletIsEncrypted != nil && paths.WalletIsEncrypted() {
		if paths.WalletIsUnlocked == nil || !paths.WalletIsUnlocked() {
			return nil, -13, walletLockedRPCMsg
		}
	}
	if len(outputs) == 0 {
		return nil, -8, errPrefix+": no outputs"
	}
	if walletShouldUsePQCarrier(paths, outputs, extraFundOpts) {
		if res, code, msg := walletBroadcastPQCarrierPayment(chainName, paths, j, pool, txIndex, raw, outputs, relayTx, allowUnverified, net, errPrefix, extraFundOpts); code == 0 {
			return res.TxcTxid, code, msg
		} else if code != 0 && code != -8 {
			return nil, code, msg
		}
		// Fall back to OP_RETURN-only when carrier build fails (e.g. policy limits).
	}
	signedHex, code, msg := walletBuildSign(chainName, paths, j, raw, txIndex, outputs, errPrefix, subtractFeeFromVouts, extraFundOpts)
	if code != 0 {
		return nil, code, msg
	}
	signedParam, err := json.Marshal(signedHex)
	if err != nil {
		return nil, -8, errPrefix + ": internal error"
	}
	result, code, msg := execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{signedParam}, relayTx, allowUnverified, net)
	if code == 0 {
		if txid, ok := result.(string); ok {
			walletRecordTxHex(paths, txid, signedHex)
		}
	}
	return result, code, msg
}

// walletBuildSign creates, funds, and signs a wallet payment; returns signed raw hex.
func walletBuildSign(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	raw *store.RawBlockStore,
	txIndex *store.TxIndex,
	outputs map[string]float64,
	errPrefix string,
	subtractFeeFromVouts []int,
	extraFundOpts map[string]interface{},
) (signedHex string, errCode int, errMsg string) {
	walletAddr := rpcWalletDefaultAddress(paths)
	if walletAddr == "" {
		return "", -1, errPrefix + ": wallet is not implemented in DogeGo"
	}
	if paths != nil && paths.WalletIsEncrypted != nil && paths.WalletIsEncrypted() {
		if paths.WalletIsUnlocked == nil || !paths.WalletIsUnlocked() {
			return "", -13, walletLockedRPCMsg
		}
	}
	changeAddr := rpcWalletDefaultChangeAddress(paths)
	if changeAddr == "" {
		changeAddr = walletAddr
	}
	if len(outputs) == 0 {
		return "", -8, errPrefix+": no outputs"
	}
	scripts := rpcWalletTrackedScripts(paths)
	if len(scripts) > 0 {
		if _, ok := tryLoadWalletUtxoCacheRows(paths, scripts); !ok {
			RefreshWalletUtxoCache(paths, scripts)
		}
	}
	sendFund := cloneSendFundOptions(extraFundOpts)
	subtractVouts := subtractFeeFromVouts
	if len(subtractVouts) == 0 {
		if v, ok := sendFund["subtractFeeFromAmount"]; ok {
			if b, _ := v.(bool); b {
				subtractVouts = []int{0}
			}
			delete(sendFund, "subtractFeeFromAmount")
		}
	}
	pqSpec, code, msg := peelPQCommitFromSendOptions(sendFund, paths, errPrefix)
	if code != 0 {
		return "", code, msg
	}
	skipPQ := false
	if v, ok := sendFund["skip_pq_commitment"].(bool); ok && v {
		skipPQ = true
		delete(sendFund, "skip_pq_commitment")
	}
	if !skipPQ && pqSpec == nil && rpcWalletPqCommitmentsEnabled(paths) && paths.WalletNextPQCommit != nil {
		tag, commit, err := paths.WalletNextPQCommit()
		if err != nil {
			return "", -8, errPrefix+": pq commitment: "+err.Error()
		}
		if commit != "" {
			pqSpec = map[string]interface{}{"tag": tag, "commitment": commit}
		}
	}
	outJSON, err := marshalWalletOutputs(outputs, pqSpec)
	if err != nil {
		return "", -8, errPrefix + ": internal error"
	}
	inputsParam := json.RawMessage(`[]`)
	if v, ok := sendFund["inputs"]; ok {
		b, err := json.Marshal(v)
		if err != nil {
			return "", -8, errPrefix + ": invalid inputs"
		}
		if len(b) > 2 && string(b) != "[]" && string(b) != "null" {
			inputsParam = b
			sendFund["add_inputs"] = false
		}
		delete(sendFund, "inputs")
	}
	hexUnsigned, code, msg := execCreateRawTransaction(chainName, []json.RawMessage{inputsParam, outJSON})
	if code != 0 {
		return "", code, msg
	}
	unsignedHex, _ := hexUnsigned.(string)
	fundOptsMap := map[string]interface{}{"changeAddress": changeAddr}
	if len(subtractVouts) > 0 {
		fundOptsMap["subtractFeeFromOutputs"] = subtractVouts
	}
	for k, v := range sendFund {
		fundOptsMap[k] = v
	}
	fundOptsJSON, err := json.Marshal(fundOptsMap)
	if err != nil {
		return "", -8, errPrefix + ": internal error"
	}
	unsignedParam, err := json.Marshal(unsignedHex)
	if err != nil {
		return "", -8, errPrefix + ": internal error"
	}
	fundRes, code, msg := execFundRawTransaction(chainName, paths, j, raw, txIndex, []json.RawMessage{unsignedParam, fundOptsJSON})
	if code != 0 {
		return "", code, msg
	}
	fundMap, ok := fundRes.(map[string]interface{})
	if !ok {
		return "", -8, errPrefix + ": fundrawtransaction failed"
	}
	fundedHex, _ := fundMap["hex"].(string)
	if fundedHex == "" {
		return "", -8, errPrefix + ": fundrawtransaction returned no hex"
	}
	fundedParam, err := json.Marshal(fundedHex)
	if err != nil {
		return "", -8, errPrefix + ": internal error"
	}
	signRes, code, msg := execSignRawTransaction(chainName, paths, []json.RawMessage{
		fundedParam,
		json.RawMessage(`null`),
		json.RawMessage(`null`),
		json.RawMessage(`null`),
	})
	if code != 0 {
		return "", code, msg
	}
	if complete, _ := signRes["complete"].(bool); !complete {
		return "", -8, errPrefix+": signing incomplete"
	}
	signedHex, _ = signRes["hex"].(string)
	if strings.TrimSpace(signedHex) == "" {
		return "", -8, errPrefix + ": signed tx missing hex"
	}
	return signedHex, 0, ""
}

// walletFundUnsignedPayment funds a wallet payment and returns unsigned raw hex (no sign/broadcast).
func walletFundUnsignedPayment(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	raw *store.RawBlockStore,
	txIndex *store.TxIndex,
	outputs map[string]float64,
	errPrefix string,
	extraFundOpts map[string]interface{},
) (string, int, string) {
	walletAddr := rpcWalletDefaultAddress(paths)
	if walletAddr == "" {
		return "", -1, errPrefix + ": wallet is not implemented in DogeGo"
	}
	if paths != nil && paths.WalletIsEncrypted != nil && paths.WalletIsEncrypted() {
		if paths.WalletIsUnlocked == nil || !paths.WalletIsUnlocked() {
			return "", -13, walletLockedRPCMsg
		}
	}
	changeAddr := rpcWalletDefaultChangeAddress(paths)
	if changeAddr == "" {
		changeAddr = walletAddr
	}
	sendFund := cloneSendFundOptions(extraFundOpts)
	pqSpec, code, msg := peelPQCommitFromSendOptions(sendFund, paths, errPrefix)
	if code != 0 {
		return "", code, msg
	}
	if pqSpec != nil {
		return "", -8, errPrefix+": pqcommit not supported for fund-only path"
	}
	outJSON, err := marshalWalletOutputs(outputs, nil)
	if err != nil {
		return "", -8, errPrefix + ": internal error"
	}
	inputsParam := json.RawMessage(`[]`)
	hexUnsigned, code, msg := execCreateRawTransaction(chainName, []json.RawMessage{inputsParam, outJSON})
	if code != 0 {
		return "", code, msg
	}
	unsignedHex, _ := hexUnsigned.(string)
	fundOptsMap := map[string]interface{}{"changeAddress": changeAddr}
	for k, v := range sendFund {
		fundOptsMap[k] = v
	}
	fundOptsJSON, _ := json.Marshal(fundOptsMap)
	unsignedParam, _ := json.Marshal(unsignedHex)
	fundRes, code, msg := execFundRawTransaction(chainName, paths, j, raw, txIndex, []json.RawMessage{unsignedParam, fundOptsJSON})
	if code != 0 {
		return "", code, msg
	}
	fundMap, ok := fundRes.(map[string]interface{})
	if !ok {
		return "", -8, errPrefix + ": fundrawtransaction failed"
	}
	fundedHex, _ := fundMap["hex"].(string)
	if fundedHex == "" {
		return "", -8, errPrefix + ": fundrawtransaction returned no hex"
	}
	return fundedHex, 0, ""
}

func walletSignRawHex(chainName string, paths *DataPaths, unsignedHex string) (string, int, string) {
	param, _ := json.Marshal(unsignedHex)
	signRes, code, msg := execSignRawTransaction(chainName, paths, []json.RawMessage{
		param, json.RawMessage(`null`), json.RawMessage(`null`), json.RawMessage(`null`),
	})
	if code != 0 {
		return "", code, msg
	}
	if complete, _ := signRes["complete"].(bool); !complete {
		return "", -8, "signing incomplete"
	}
	signedHex, _ := signRes["hex"].(string)
	if signedHex == "" {
		return "", -8, "signed tx missing hex"
	}
	return signedHex, 0, ""
}

func marshalSortedOutputs(outputs map[string]float64) ([]byte, error) {
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	m := make(map[string]float64, len(keys))
	for _, k := range keys {
		m[k] = outputs[k]
	}
	return json.Marshal(m)
}

func voutIndicesForSubtractFee(chainName string, sortedAddrs []string, subtractFrom []string) ([]int, int, string) {
	if len(subtractFrom) == 0 {
		return nil, 0, ""
	}
	var idxs []int
	for _, a := range subtractFrom {
		a = strings.TrimSpace(a)
		vis, _, _ := ValidateAddressString(chainName, a)
		if ok, _ := vis["isvalid"].(bool); !ok {
			return nil, -5, "Invalid Dogecoin address: " + a
		}
		found := false
		for i, k := range sortedAddrs {
			if k == a {
				idxs = append(idxs, i)
				found = true
				break
			}
		}
		if !found {
			return nil, -8, "Invalid parameter, address not in outputs: " + a
		}
	}
	return idxs, 0, ""
}

func sortedOutputAddresses(outputs map[string]float64) []string {
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// execSendToAddress builds, funds, signs, and broadcasts a P2PKH payment from the built-in testnet wallet.
func execSendToAddress(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	params []json.RawMessage,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 6 {
		return nil, -32602, "Wrong number of arguments"
	}
	var dest string
	if err := json.Unmarshal(params[0], &dest); err != nil {
		return nil, -8, "sendtoaddress: address must be a string"
	}
	dest = strings.TrimSpace(dest)
	vis, _, _ := ValidateAddressString(chainName, dest)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	amt, code, msg := parseRPCAmountField(params[1], "sendtoaddress", "amount")
	if code != 0 {
		return nil, code, msg
	}
	if amt <= 0 {
		return nil, -3, "Invalid amount for send"
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var c string
		if err := json.Unmarshal(params[2], &c); err != nil {
			return nil, -8, "sendtoaddress: comment must be a string"
		}
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var c string
		if err := json.Unmarshal(params[3], &c); err != nil {
			return nil, -8, "sendtoaddress: comment_to must be a string"
		}
	}
	var subtractVouts []int
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		subtractFee, code, msg := parseRPCBoolOpt(params[4], false, "sendtoaddress", "subtractfeefromamount")
		if code != 0 {
			return nil, code, msg
		}
		if subtractFee {
			subtractVouts = []int{0}
		}
	}
	var extraFund map[string]interface{}
	if len(params) > 5 && strings.TrimSpace(string(params[5])) != "null" {
		if err := json.Unmarshal(params[5], &extraFund); err != nil {
			return nil, -8, "sendtoaddress: options must be a JSON object"
		}
	}
	return walletBuildSignBroadcast(chainName, paths, j, pool, txIndex, raw,
		map[string]float64{dest: amt}, relayTx, allowUnverified, net, "sendtoaddress", subtractVouts, extraFund)
}

// WalletSendDOGE builds, signs, and broadcasts from the built-in wallet (web UI and other non-RPC callers).
// fundOpts may include fee_rate, conf_target, pqcommit (requires pq_commitments wallet flag), etc.
func WalletSendDOGE(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	dest string,
	amountDOGE float64,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
	fundOpts map[string]interface{},
) (txid string, code int, msg string) {
	dest = strings.TrimSpace(dest)
	vis, _, _ := ValidateAddressString(chainName, dest)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return "", -5, "Invalid Dogecoin address"
	}
	if amountDOGE <= 0 {
		return "", -3, "Invalid amount for send"
	}
	if rpcWalletAddress(paths) == "" {
		return "", -1, "sendtoaddress: wallet is not implemented in DogeGo"
	}
	res, code, msg := walletBuildSignBroadcast(chainName, paths, j, pool, txIndex, raw,
		map[string]float64{dest: amountDOGE}, relayTx, allowUnverified, net, "sendtoaddress", nil, fundOpts)
	if code != 0 {
		return "", code, msg
	}
	txid, _ = res.(string)
	return txid, 0, ""
}

// WalletSendDOGEDetailed signs, broadcasts, and returns txid + signed hex for web flight tracking.
func WalletSendDOGEDetailed(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	dest string,
	amountDOGE float64,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
	fundOpts map[string]interface{},
) (txid, signedHex, status, broadcastErr string, code int, msg string) {
	dest = strings.TrimSpace(dest)
	vis, _, _ := ValidateAddressString(chainName, dest)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return "", "", "", "", -5, "Invalid Dogecoin address"
	}
	if amountDOGE <= 0 {
		return "", "", "", "", -3, "Invalid amount for send"
	}
	if rpcWalletAddress(paths) == "" {
		return "", "", "", "", -1, "sendtoaddress: wallet is not implemented in DogeGo"
	}
	if walletShouldUsePQCarrier(paths, map[string]float64{dest: amountDOGE}, fundOpts) {
		if res, code, msg := walletBroadcastPQCarrierPayment(chainName, paths, j, pool, txIndex, raw, map[string]float64{dest: amountDOGE}, relayTx, allowUnverified, net, "sendtoaddress", fundOpts); code == 0 {
			return res.TxcTxid, res.TxcHex, "broadcasting", "", 0, ""
		} else if code != 0 && code != -8 {
			return "", "", "", "", code, msg
		}
	}
	hexStr, code, msg := walletBuildSign(chainName, paths, j, raw, txIndex,
		map[string]float64{dest: amountDOGE}, "sendtoaddress", nil, fundOpts)
	if code != 0 {
		return "", "", "", "", code, msg
	}
	rawBytes, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	if err != nil || len(rawBytes) == 0 {
		return "", "", "", "", -8, "sendtoaddress: signed tx decode failed"
	}
	tx, err := wire.DeserializeTx(rawBytes)
	if err != nil {
		return "", hexStr, "broadcasting", "", -8, "sendtoaddress: signed tx decode failed"
	}
	txid = mempool.TxIDDisplayHex(tx.TxHash())
	walletRecordTxHex(paths, txid, hexStr)
	signedParam, err := json.Marshal(hexStr)
	if err != nil {
		return txid, hexStr, "broadcasting", "", -8, "sendtoaddress: internal error"
	}
	res, bcode, bmsg := execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{signedParam}, relayTx, allowUnverified, net)
	if bcode != 0 {
		lower := strings.ToLower(bmsg)
		if strings.Contains(lower, "already") || strings.Contains(lower, "known") {
			return txid, hexStr, "mempool", bmsg, 0, ""
		}
		return txid, hexStr, "broadcasting", bmsg, 0, ""
	}
	if s, ok := res.(string); ok && s != "" {
		txid = s
	}
	status = "mempool"
	if pool == nil || !pool.ContainsTxID(txid) {
		status = "broadcasting"
	}
	return txid, hexStr, status, "", 0, ""
}

// execSendFrom matches Core sendfrom; built-in wallet ignores fromaccount and minconf.
func execSendFrom(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	params []json.RawMessage,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
) (interface{}, int, string) {
	if len(params) < 3 || len(params) > 7 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "sendfrom", "fromaccount"); code != 0 {
		return nil, code, msg
	}
	var addr string
	if err := json.Unmarshal(params[1], &addr); err != nil {
		return nil, -8, "sendfrom: toaddress must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid Dogecoin address"
	}
	amt, code, msg := parseRPCAmountField(params[2], "sendfrom", "amount")
	if code != 0 {
		return nil, code, msg
	}
	if amt <= 0 {
		return nil, -3, "Invalid amount for send"
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[3], &n); err != nil {
			return nil, -8, "sendfrom: minconf must be a number"
		}
		if _, err := n.Int64(); err != nil {
			return nil, -8, "sendfrom: minconf must be a number"
		}
	}
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var c string
		if err := json.Unmarshal(params[4], &c); err != nil {
			return nil, -8, "sendfrom: comment must be a string"
		}
	}
	if len(params) > 5 && strings.TrimSpace(string(params[5])) != "null" {
		var c string
		if err := json.Unmarshal(params[5], &c); err != nil {
			return nil, -8, "sendfrom: comment_to must be a string"
		}
	}
	var extraFund map[string]interface{}
	if len(params) > 6 && strings.TrimSpace(string(params[6])) != "null" {
		var code int
		var msg string
		extraFund, code, msg = parseSendFundOptionsJSON(params[6], "sendfrom")
		if code != 0 {
			return nil, code, msg
		}
	}
	return walletBuildSignBroadcast(chainName, paths, j, pool, txIndex, raw,
		map[string]float64{addr: amt}, relayTx, allowUnverified, net, "sendfrom", nil, extraFund)
}

// execSendMany matches Core sendmany; one transaction with multiple outputs.
func execSendMany(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	params []json.RawMessage,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 6 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "sendmany", "fromaccount"); code != 0 {
		return nil, code, msg
	}
	if strings.TrimSpace(string(params[1])) == "null" {
		return nil, -8, "sendmany: amounts must be a JSON object"
	}
	var amounts map[string]json.RawMessage
	if err := json.Unmarshal(params[1], &amounts); err != nil {
		return nil, -8, "sendmany: amounts must be a JSON object"
	}
	if len(amounts) == 0 {
		return nil, -8, "sendmany: amounts must not be empty"
	}
	outputs := make(map[string]float64, len(amounts))
	seen := make(map[string]struct{})
	for payTo, rawAmt := range amounts {
		payTo = strings.TrimSpace(payTo)
		vis, _, _ := ValidateAddressString(chainName, payTo)
		if ok, _ := vis["isvalid"].(bool); !ok {
			return nil, -5, "Invalid Dogecoin address: " + payTo
		}
		if _, dup := seen[payTo]; dup {
			return nil, -8, "Invalid parameter, duplicated address: " + payTo
		}
		seen[payTo] = struct{}{}
		a, code, msg := parseRPCAmountField(rawAmt, "sendmany", "amount")
		if code != 0 {
			return nil, code, msg
		}
		if a <= 0 {
			return nil, -3, "Invalid amount for send"
		}
		outputs[payTo] = a
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[2], &n); err != nil {
			return nil, -8, "sendmany: minconf must be a number"
		}
		if _, err := n.Int64(); err != nil {
			return nil, -8, "sendmany: minconf must be a number"
		}
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var c string
		if err := json.Unmarshal(params[3], &c); err != nil {
			return nil, -8, "sendmany: comment must be a string"
		}
	}
	var subtractVouts []int
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var subtractFrom []string
		if err := json.Unmarshal(params[4], &subtractFrom); err != nil {
			return nil, -8, "sendmany: subtractfeefrom must be a JSON array"
		}
		if len(subtractFrom) > 0 {
			sorted := sortedOutputAddresses(outputs)
			var code int
			var msg string
			subtractVouts, code, msg = voutIndicesForSubtractFee(chainName, sorted, subtractFrom)
			if code != 0 {
				return nil, code, msg
			}
		}
	}
	var extraFund map[string]interface{}
	if len(params) > 5 && strings.TrimSpace(string(params[5])) != "null" {
		var code int
		var msg string
		extraFund, code, msg = parseSendFundOptionsJSON(params[5], "sendmany")
		if code != 0 {
			return nil, code, msg
		}
	}
	return walletBuildSignBroadcast(chainName, paths, j, pool, txIndex, raw,
		outputs, relayTx, allowUnverified, net, "sendmany", subtractVouts, extraFund)
}
