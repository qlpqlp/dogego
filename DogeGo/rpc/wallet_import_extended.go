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

func execDogegoImportMnemonic(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || paths.WalletImportMnemonic == nil {
		return nil, -1, "dogego_importmnemonic: wallet is not implemented in DogeGo"
	}
	if len(params) < 1 {
		return nil, -8, "dogego_importmnemonic: mnemonic required"
	}
	var mnemonic string
	if err := json.Unmarshal(params[0], &mnemonic); err != nil {
		return nil, -8, "dogego_importmnemonic: mnemonic must be a string"
	}
	passphrase := ""
	if len(params) > 1 && string(params[1]) != "null" {
		if err := json.Unmarshal(params[1], &passphrase); err != nil {
			return nil, -8, "dogego_importmnemonic: passphrase must be a string"
		}
	}
	rescan := true
	if len(params) > 2 {
		var code int
		var msg string
		rescan, code, msg = parseRPCBoolOpt(params[2], true, "dogego_importmnemonic", "rescan")
		if code != 0 {
			return nil, code, msg
		}
	}
	if err := paths.WalletImportMnemonic(strings.TrimSpace(mnemonic), passphrase); err != nil {
		return nil, -8, err.Error()
	}
	if rescan {
		if code, msg := walletRescanAfterImport(paths, j, raw, params, 2, "dogego_importmnemonic"); code != 0 {
			return nil, code, msg
		}
	}
	addr := ""
	if paths.WalletDefaultAddress != nil {
		addr = paths.WalletDefaultAddress()
	}
	return map[string]any{
		"ok":      true,
		"address": addr,
		"hd":      paths.WalletHDFormat != nil && paths.WalletHDFormat() == "hd",
	}, 0, ""
}

func execDogegoImportBIP38(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || paths.WalletImportBIP38 == nil {
		return nil, -1, "dogego_importbip38: wallet is not implemented in DogeGo"
	}
	if len(params) < 2 {
		return nil, -8, "dogego_importbip38: encrypted key and passphrase required"
	}
	var enc, passphrase string
	if err := json.Unmarshal(params[0], &enc); err != nil {
		return nil, -8, "dogego_importbip38: encrypted key must be a string"
	}
	if err := json.Unmarshal(params[1], &passphrase); err != nil {
		return nil, -8, "dogego_importbip38: passphrase must be a string"
	}
	rescan := true
	if len(params) > 2 {
		var code int
		var msg string
		rescan, code, msg = parseRPCBoolOpt(params[2], true, "dogego_importbip38", "rescan")
		if code != 0 {
			return nil, code, msg
		}
	}
	addr, err := paths.WalletImportBIP38(strings.TrimSpace(enc), passphrase)
	if err != nil {
		return nil, -8, err.Error()
	}
	if rescan {
		if code, msg := walletRescanAfterImport(paths, j, raw, params, 2, "dogego_importbip38"); code != 0 {
			return nil, code, msg
		}
	}
	_ = chainName
	return map[string]any{"ok": true, "address": addr}, 0, ""
}

func execDogegoListWalletAddresses(paths *DataPaths) (interface{}, int, string) {
	if paths == nil || paths.WalletListAddresses == nil {
		return []WalletAddressEntry{}, 0, ""
	}
	return paths.WalletListAddresses(), 0, ""
}
