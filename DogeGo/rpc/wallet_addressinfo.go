// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/secp256k1"

	"dogego/chain"
)

// execGetAddressInfo returns Core-shaped metadata for an address on the RPC chain.
// Optional second param redeemScript (hex) matches validateaddress P2SH multisig checks.
func execGetAddressInfo(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "getaddressinfo: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, -5, "Invalid address"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -5, "Invalid address"
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -5, "Invalid address"
	}
	var redeem []byte
	if len(params) > 1 && string(params[1]) != "null" {
		var redeemHex string
		if err := json.Unmarshal(params[1], &redeemHex); err != nil {
			return nil, -8, "getaddressinfo: redeemScript must be hex string"
		}
		redeemHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(redeemHex), "0x"))
		if redeemHex != "" {
			redeem, err = hex.DecodeString(redeemHex)
			if err != nil {
				return nil, -8, "getaddressinfo: invalid redeemScript hex"
			}
		}
	}
	if len(redeem) == 0 {
		redeem = walletRedeemScriptForAddress(paths, addr, p)
	}
	base := validateAddressResult(chainName, paths, addr, p, redeem)
	if ok, _ := base["isvalid"].(bool); !ok {
		return nil, -5, "Invalid address"
	}
	return addressInfoResult(chainName, paths, addr, p, base), 0, ""
}

func addressInfoResult(chainName string, paths *DataPaths, addr string, p chain.Params, base map[string]interface{}) map[string]interface{} {
	ismine, _ := base["ismine"].(bool)
	iswatch, _ := base["iswatchonly"].(bool)
	spk, _ := base["scriptPubKey"].(map[string]interface{})
	spkType, _ := spk["type"].(string)
	isscript := spkType == "scripthash"

	out := map[string]interface{}{
		"address":         addr,
		"scriptPubKey":    spk,
		"ismine":          ismine,
		"iswatchonly":     iswatch,
		"isscript":        isscript,
		"iswitness":       false,
		"witness_version": nil,
		"witness_program": nil,
		"ischange":        false,
		"timestamp":       int64(0),
		"label":           "",
		"labels":          []interface{}{},
		"solvable":        ismine,
	}
	if iswatch && !ismine {
		out["solvable"] = false
	}
	if ismine {
		out["iscompressed"] = true
	}
	if isscript && iswatch && !ismine {
		redeem := walletRedeemScriptForAddress(paths, addr, p)
		if len(redeem) == 0 {
			if redeemHex, ok := spk["hex"].(string); ok {
				redeem, _ = hex.DecodeString(redeemHex)
			}
		}
		if walletSolvableFromP2SHRedeem(chainName, paths, redeem) {
			out["solvable"] = true
		}
	}
	enrichWalletAddressMeta(chainName, paths, addr, out)
	if lbl := rpcWalletGetLabel(paths, addr); lbl != "" {
		out["label"] = lbl
		out["labels"] = []interface{}{
			map[string]interface{}{"name": lbl, "purpose": walletAddressLabelPurpose(paths, addr)},
		}
	}
	return out
}

func walletPubKeyHexForAddress(chainName string, paths *DataPaths, addr string) string {
	var wif string
	if paths != nil && paths.WalletWIFForAddress != nil {
		w, err := paths.WalletWIFForAddress(addr)
		if err == nil {
			wif = w
		}
	}
	if wif == "" {
		wif = rpcWalletWIF(paths)
	}
	if wif == "" {
		return ""
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return ""
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return ""
	}
	sec, compressed, err := chain.DecodeWIF(wif, p.PrivKeyWIFVersion)
	if err != nil || len(sec) != 32 {
		return ""
	}
	priv, _ := secp256k1.PrivKeyFromBytes(sec)
	pub := priv.PubKey()
	if compressed {
		return hex.EncodeToString(pub.SerializeCompressed())
	}
	return hex.EncodeToString(pub.SerializeUncompressed())
}
