// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/json"
	"fmt"
	"strings"
)

// WalletRPCCaller runs whitelisted wallet JSON-RPC methods on behalf of extensions.
type WalletRPCCaller interface {
	WalletUnlocked() bool
	Call(method string, params []json.RawMessage) (interface{}, error)
}

// WalletRPCHost is implemented by the scoped extension host when wallet_rpc is granted.
type WalletRPCHost interface {
	Host
	CallWalletRPC(method string, params []json.RawMessage) (interface{}, error)
}

// extensionWalletRPCReadOnly may be called without an unlocked wallet.
var extensionWalletRPCReadOnly = map[string]struct{}{
	"getwalletinfo":         {},
	"validateaddress":       {},
	"getaddressinfo":        {},
	"listunspent":           {},
	"listtransactions":      {},
	"listsinceblock":        {},
	"listlabels":            {},
	"getaddressesbylabel":   {},
	"listaddressgroupings":  {},
	"getbalance":            {},
	"getbalances":           {},
	"listreceivedbyaddress": {},
	"verifymessage":         {},
	"getnewaddress":         {},
	"getrawchangeaddress":   {},
	"setlabel":              {},
	"createrawtransaction":  {},
	"decoderawtransaction":  {},
}

// extensionWalletRPCRequiresUnlock needs walletpassphrase before use (no key export).
var extensionWalletRPCRequiresUnlock = map[string]struct{}{
	"signmessage":                  {},
	"signrawtransactionwithwallet": {},
	"sendtoaddress":                {},
	"sendmany":                     {},
	"sendfrom":                     {},
	"walletcreatefundedpsbt":       {},
	"fundrawtransaction":           {},
	"lockunspent":                  {},
	"settxfee":                     {},
}

// extensionWalletRPCBroadcast relays signed txs; no spend keys required.
var extensionWalletRPCBroadcast = map[string]struct{}{
	"sendrawtransaction": {},
}

// extensionWalletRPCForbidden is never exposed to extensions (direct key access or unlock).
var extensionWalletRPCForbidden = map[string]struct{}{
	"signmessagewithprivkey":     {},
	"signrawtransactionwithkey":  {},
	"dumpprivkey":                {},
	"dumpwallet":                 {},
	"importprivkey":              {},
	"importwallet":               {},
	"encryptwallet":              {},
	"walletpassphrase":           {},
	"walletpassphrasechange":     {},
	"walletlock":                 {},
	"exportwallet":               {},
	"abandontransaction":         {},
	"backupwallet":               {},
}

// CallWalletRPC invokes a whitelisted wallet RPC method via the node bridge.
func CallWalletRPC(caller WalletRPCCaller, method string, params []json.RawMessage) (interface{}, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		return nil, fmt.Errorf("wallet rpc method required")
	}
	if _, bad := extensionWalletRPCForbidden[method]; bad {
		return nil, fmt.Errorf("wallet rpc %q forbidden for extensions", method)
	}
	_, readOK := extensionWalletRPCReadOnly[method]
	_, unlockOK := extensionWalletRPCRequiresUnlock[method]
	_, broadcastOK := extensionWalletRPCBroadcast[method]
	if !readOK && !unlockOK && !broadcastOK {
		return nil, fmt.Errorf("wallet rpc %q not allowed for extensions", method)
	}
	if unlockOK {
		if caller == nil || !caller.WalletUnlocked() {
			return nil, fmt.Errorf("wallet must be unlocked (walletpassphrase RPC) before %q", method)
		}
	}
	if caller == nil {
		return nil, fmt.Errorf("wallet rpc unavailable")
	}
	return caller.Call(method, params)
}
