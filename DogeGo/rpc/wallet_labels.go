// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
)

func walletApplyLabel(chainName string, paths *DataPaths, pkScript []byte, label string) {
	label = strings.TrimSpace(label)
	if label == "" || paths == nil || paths.WalletSetLabel == nil || len(pkScript) == 0 {
		return
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return
	}
	addr := chain.ScriptPubKeyAddress(pkScript, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	if addr != "" {
		_ = paths.WalletSetLabel(addr, label)
	}
}

func walletLabelFromImportMultiReq(req map[string]json.RawMessage) string {
	raw, ok := req["label"]
	if !ok || strings.TrimSpace(string(raw)) == "null" {
		return ""
	}
	var label string
	if err := json.Unmarshal(raw, &label); err != nil {
		return ""
	}
	return strings.TrimSpace(label)
}

func rpcWalletGetLabel(paths *DataPaths, addr string) string {
	if paths == nil || paths.WalletGetLabel == nil {
		return ""
	}
	return paths.WalletGetLabel(strings.TrimSpace(addr))
}

// execListLabelsWallet returns unique address labels from wallet.json.
func execListLabelsWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		var purpose string
		if err := json.Unmarshal(params[0], &purpose); err != nil {
			return nil, -8, "listlabels: purpose must be a string"
		}
	}
	if rpcWalletAddress(paths) == "" {
		return []interface{}{}, 0, ""
	}
	if paths.WalletListLabels == nil {
		return []interface{}{}, 0, ""
	}
	labels := paths.WalletListLabels()
	out := make([]interface{}, len(labels))
	for i, lbl := range labels {
		out[i] = lbl
	}
	return out, 0, ""
}

// execSetLabelWallet assigns a label to a tracked wallet address.
func execSetLabelWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr, label string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "setlabel: address must be a string"
	}
	if err := json.Unmarshal(params[1], &label); err != nil {
		return nil, -8, "setlabel: label must be a string"
	}
	addr = strings.TrimSpace(addr)
	vis, _, _ := ValidateAddressString(chainName, addr)
	if ok, _ := vis["isvalid"].(bool); !ok {
		return nil, -5, "Invalid address"
	}
	if paths == nil || paths.WalletSetLabel == nil || rpcWalletAddress(paths) == "" {
		return nil, -1, "setlabel: wallet is not implemented in DogeGo"
	}
	if !walletAddressIsTracked(paths, chainName, addr) {
		return nil, -4, "Address not found in wallet"
	}
	if err := paths.WalletSetLabel(addr, label); err != nil {
		return nil, -1, "setlabel: " + err.Error()
	}
	return nil, 0, ""
}

// execGetAddressesByLabelWallet lists tracked addresses with the given label.
func execGetAddressesByLabelWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var label string
	if err := json.Unmarshal(params[0], &label); err != nil {
		return nil, -8, "getaddressesbylabel: label must be a string"
	}
	label = strings.TrimSpace(label)
	if rpcWalletAddress(paths) == "" {
		return map[string]interface{}{}, 0, ""
	}
	addrs, code, msg := rpcWalletTrackedAddresses(paths, chainName)
	if code != 0 {
		return nil, code, msg
	}
	out := make(map[string]interface{})
	for _, addr := range addrs {
		if rpcWalletGetLabel(paths, addr) == label {
			out[addr] = map[string]interface{}{"purpose": walletAddressLabelPurpose(paths, addr)}
		}
	}
	return out, 0, ""
}
