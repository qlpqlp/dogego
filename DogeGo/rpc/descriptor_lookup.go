// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"

	"dogego/chain"
)

// pkScriptAndAddressFromParsedDescriptor derives the imported script and display address for a parsed descriptor.
func pkScriptAndAddressFromParsedDescriptor(p chain.Params, parsed parsedDescriptor) (pkScript []byte, addr string, ok bool) {
	switch parsed.scriptType {
	case "pkh":
		_, h160, err := chain.Base58CheckDecode(parsed.addr)
		if err != nil {
			return nil, "", false
		}
		return chain.P2PKHScriptFromPubKeyHash(h160), parsed.addr, true
	case "p2sh-pkh":
		_, h160, err := chain.Base58CheckDecode(parsed.addr)
		if err != nil {
			return nil, "", false
		}
		inner := chain.P2PKHScriptFromPubKeyHash(h160)
		pkScript = chain.P2SHScriptFromScriptHash(scriptHash160(inner))
		addr = chain.ScriptPubKeyAddress(pkScript, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		return pkScript, addr, addr != ""
	case "p2sh-cltv-pkh", "p2sh-csv-pkh", "p2sh-multi", "p2sh-cltv-multi", "p2sh-csv-multi":
		if len(parsed.redeem) == 0 {
			return nil, "", false
		}
		pkScript = chain.P2SHScriptFromScriptHash(scriptHash160(parsed.redeem))
		addr = chain.ScriptPubKeyAddress(pkScript, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		return pkScript, addr, addr != ""
	case "bare-multi":
		if len(parsed.redeem) == 0 {
			return nil, "", false
		}
		return append([]byte(nil), parsed.redeem...), "", true
	default:
		return nil, "", false
	}
}

// addressFromDescriptorString returns the wallet address for a supported import descriptor, if any.
func addressFromDescriptorString(chainName string, desc string) (string, bool) {
	parsed, ok := parseImportDescriptor(strings.TrimSpace(desc))
	if !ok {
		return "", false
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", false
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", false
	}
	_, addr, ok := pkScriptAndAddressFromParsedDescriptor(p, parsed)
	return addr, ok && addr != ""
}

// walletDescriptorForAddress returns a Core-style descriptor when the wallet knows one for addr.
func walletDescriptorForAddress(chainName string, paths *DataPaths, addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" || paths == nil {
		return ""
	}
	if paths.WalletListDescriptors != nil {
		for _, row := range paths.WalletListDescriptors(chainName) {
			row.Desc = strings.TrimSpace(row.Desc)
			if row.Desc == "" {
				continue
			}
			if a, ok := addressFromDescriptorString(chainName, row.Desc); ok && a == addr {
				return row.Desc
			}
		}
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return ""
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return ""
	}
	redeem := walletRedeemScriptForAddress(paths, addr, p)
	if len(redeem) == 0 {
		if paths.WalletWatchScripts != nil {
			for _, pk := range paths.WalletWatchScripts() {
				if chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID) != addr {
					continue
				}
				if d := walletDescriptorForScript(chainName, pk); d != "" {
					return d
				}
			}
		}
		return ""
	}
	for _, pk := range rpcWalletWatchScripts(paths) {
		if chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID) != addr {
			continue
		}
		if d := walletDescriptorForP2SHRedeem(chainName, pk, redeem); d != "" {
			return d
		}
	}
	return walletDescriptorForP2SHRedeem(chainName, nil, redeem)
}
