// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

const unencryptedWalletPassphraseMsg = "Error: running with an unencrypted wallet, but walletpassphrase was called."

// execEncryptWalletBuiltin reports that wallet encryption is not available in DogeGo.
func execEncryptWalletBuiltin(params []json.RawMessage) (interface{}, int, string) {
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
	return nil, -1, "encryptwallet: wallet encryption is not supported in DogeGo"
}

// execWalletPassphraseUnencrypted is returned when the built-in wallet has no encryption.
func execWalletPassphraseUnencrypted(params []json.RawMessage) (interface{}, int, string) {
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
	return nil, -15, unencryptedWalletPassphraseMsg
}

// execWalletPassphraseChangeUnencrypted matches Core error when wallet is not encrypted.
func execWalletPassphraseChangeUnencrypted(params []json.RawMessage) (interface{}, int, string) {
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
	return nil, -15, "Error: running with an unencrypted wallet, but walletpassphrasechange was called."
}

// execWalletLockUnencrypted matches Core when locking an unencrypted wallet.
func execWalletLockUnencrypted(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	return nil, -15, "Error: running with an unencrypted wallet, but walletlock was called."
}
