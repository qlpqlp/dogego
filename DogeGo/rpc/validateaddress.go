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
)

func networkFromRPCChainName(chainName string) (chain.Network, error) {
	switch strings.ToLower(strings.TrimSpace(chainName)) {
	case "test", "testnet": // "test" matches JSON-RPC tests that use chain name "test"
		return chain.RebootTestnet, nil
	case "main", "mainnet":
		return chain.MainnetDogecoin, nil
	default:
		return 0, fmt.Errorf("unknown chain %q", chainName)
	}
}

func invalidAddressResult() map[string]interface{} {
	return map[string]interface{}{
		"isvalid":     false,
		"ismine":      false,
		"iswatchonly": false,
	}
}

func validateAddressResult(chainName string, paths *DataPaths, addr string, p chain.Params, redeemScript []byte) map[string]interface{} {
	v, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		return invalidAddressResult()
	}
	ismine := rpcWalletContainsAddress(paths, addr)
	iswatch := !ismine && rpcWalletIsWatchAddress(paths, addr)
	switch v {
	case p.PubkeyHashAddrID:
		scriptHex := "76a914" + hex.EncodeToString(h160[:]) + "88ac"
		asm := fmt.Sprintf("OP_DUP OP_HASH160 %x OP_EQUALVERIFY OP_CHECKSIG", h160)
		out := map[string]interface{}{
			"isvalid":     true,
			"address":     addr,
			"ismine":      ismine,
			"iswatchonly": iswatch,
			"scriptPubKey": map[string]interface{}{
				"asm":       asm,
				"hex":       scriptHex,
				"type":      "pubkeyhash",
				"address":   addr,
				"addresses": []interface{}{addr},
			},
			"isscript":        false,
			"iswitness":       false,
			"witness_version": nil,
			"witness_program": nil,
		}
		enrichWalletAddressMeta(chainName, paths, addr, out)
		return out
	case p.ScriptHashAddrID:
		if len(redeemScript) > 0 {
			if scriptHash160(redeemScript) != h160 {
				return invalidAddressResult()
			}
		}
		scriptHex := "a914" + hex.EncodeToString(h160[:]) + "87"
		asm := fmt.Sprintf("OP_HASH160 %x OP_EQUAL", h160)
		spk := map[string]interface{}{
			"asm":       asm,
			"hex":       scriptHex,
			"type":      "scripthash",
			"address":   addr,
			"addresses": []interface{}{addr},
		}
		if len(redeemScript) > 0 {
			if meta := consensus.RedeemScriptMeta(redeemScript); meta != nil {
				for k, v := range meta {
					spk[k] = v
				}
			}
			spk["hex"] = hex.EncodeToString(redeemScript)
			spk["asm"] = scriptToASM(redeemScript)
		}
		isScript := len(redeemScript) > 0
		out := map[string]interface{}{
			"isvalid":     true,
			"address":     addr,
			"ismine":      ismine,
			"iswatchonly": iswatch,
			"scriptPubKey": spk,
			"isscript":        isScript,
			"iswitness":       false,
			"witness_version": nil,
			"witness_program": nil,
		}
		enrichWalletAddressMeta(chainName, paths, addr, out)
		return out
	default:
		return invalidAddressResult()
	}
}

// execValidateAddress implements validateaddress for P2PKH and P2SH on the node's network.
// Invalid addresses return {"isvalid": false} with HTTP 200 / JSON-RPC success (Core-shaped).
func execValidateAddress(chainName string, paths *DataPaths, params []json.RawMessage) (map[string]interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "validateaddress: address required"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return invalidAddressResult(), 0, ""
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return invalidAddressResult(), 0, ""
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return invalidAddressResult(), 0, ""
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return invalidAddressResult(), 0, ""
	}
	var redeem []byte
	if len(params) > 1 && string(params[1]) != "null" {
		var redeemHex string
		if err := json.Unmarshal(params[1], &redeemHex); err != nil {
			return nil, -8, "validateaddress: redeemScript must be hex string"
		}
		redeemHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(redeemHex), "0x"))
		if redeemHex != "" {
			var err error
			redeem, err = hex.DecodeString(redeemHex)
			if err != nil {
				return nil, -8, "validateaddress: invalid redeemScript hex"
			}
		}
	}
	if len(redeem) == 0 {
		redeem = walletRedeemScriptForAddress(paths, addr, p)
	}
	return validateAddressResult(chainName, paths, addr, p, redeem), 0, ""
}

// ValidateAddressString runs validateaddress for one address.
func ValidateAddressString(chainName, addr string) (map[string]interface{}, int, string) {
	raw, err := json.Marshal(strings.TrimSpace(addr))
	if err != nil {
		return map[string]interface{}{"isvalid": false}, 0, ""
	}
	return execValidateAddress(chainName, nil, []json.RawMessage{raw})
}
