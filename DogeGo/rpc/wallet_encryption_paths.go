// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"errors"
	"strings"

	"dogego/wallet"
)

const walletLockedRPCMsg = "Error: Please enter the wallet passphrase with walletpassphrase first."

func rpcWalletIsEncrypted(paths *DataPaths) bool {
	if paths == nil || paths.WalletIsEncrypted == nil {
		return false
	}
	return paths.WalletIsEncrypted()
}

func rpcWalletUnlockUntil(paths *DataPaths) int64 {
	if paths == nil || paths.WalletUnlockUntil == nil {
		return 0
	}
	return paths.WalletUnlockUntil()
}

func rpcWalletLockedErr(err error) (int, string) {
	if errors.Is(err, wallet.ErrWalletLocked) {
		return -13, walletLockedRPCMsg
	}
	return -1, err.Error()
}

// rpcWalletRequireUnlocked returns Core -13 when the wallet is encrypted and locked.
func rpcWalletRequireUnlocked(paths *DataPaths) (int, string) {
	if !rpcWalletIsEncrypted(paths) {
		return 0, ""
	}
	if paths == nil || paths.WalletIsUnlocked == nil || !paths.WalletIsUnlocked() {
		return -13, walletLockedRPCMsg
	}
	return 0, ""
}

// rpcWalletOpErr maps wallet.ErrWalletLocked to Core -13 for address/key RPCs.
func rpcWalletOpErr(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	if code, msg := rpcWalletLockedErr(err); code == -13 {
		return code, msg
	}
	return -1, err.Error()
}

func execEncryptWalletPaths(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
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
	if paths == nil || paths.WalletEncrypt == nil {
		return nil, -1, "encryptwallet: wallet is not implemented in DogeGo"
	}
	if rpcWalletIsEncrypted(paths) {
		return nil, -1, "encryptwallet: wallet already encrypted"
	}
	msg, err := paths.WalletEncrypt(strings.TrimSpace(phrase))
	if err != nil {
		return nil, -1, "encryptwallet: "+err.Error()
	}
	return msg, 0, ""
}

func execWalletPassphrasePaths(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
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
	if paths == nil || paths.WalletUnlock == nil {
		return nil, -1, "walletpassphrase: wallet is not implemented in DogeGo"
	}
	if !rpcWalletIsEncrypted(paths) {
		return execWalletPassphraseUnencrypted(params)
	}
	if err := paths.WalletUnlock(strings.TrimSpace(phrase), sec); err != nil {
		if strings.Contains(err.Error(), "wrong passphrase") {
			return nil, -1, "walletpassphrase: "+err.Error()
		}
		return nil, -1, "walletpassphrase: "+err.Error()
	}
	return nil, 0, ""
}

func execWalletLockPaths(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if paths == nil || paths.WalletLock == nil {
		return nil, -1, "walletlock: wallet is not implemented in DogeGo"
	}
	if !rpcWalletIsEncrypted(paths) {
		return execWalletLockUnencrypted(params)
	}
	if err := paths.WalletLock(); err != nil {
		return nil, -1, "walletlock: "+err.Error()
	}
	return nil, 0, ""
}

func execWalletPassphraseChangePaths(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
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
	if paths == nil || paths.WalletChangePassphrase == nil {
		return nil, -1, "walletpassphrasechange: wallet is not implemented in DogeGo"
	}
	if !rpcWalletIsEncrypted(paths) {
		return execWalletPassphraseChangeUnencrypted(params)
	}
	if err := paths.WalletChangePassphrase(strings.TrimSpace(oldp), strings.TrimSpace(newp)); err != nil {
		if errors.Is(err, wallet.ErrWalletLocked) {
			return nil, -13, walletLockedRPCMsg
		}
		return nil, -1, "walletpassphrasechange: "+err.Error()
	}
	return nil, 0, ""
}
