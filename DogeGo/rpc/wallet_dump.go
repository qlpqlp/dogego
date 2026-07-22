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
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/chain"
	"dogego/wallet"
)

// execDumpWallet writes a Core-inspired text dump (WIF key + watch scripts + descriptors).
func execDumpWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var dest string
	if err := json.Unmarshal(params[0], &dest); err != nil {
		return nil, -8, "dumpwallet: filename must be a string"
	}
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return nil, -8, "dumpwallet: invalid filename"
	}
	if code, msg := rpcWalletRequireMainnetEncrypted(chainName, paths); code != 0 {
		return nil, code, "dumpwallet: "+msg
	}
	if paths == nil || paths.WalletPath == nil {
		return nil, -1, "dumpwallet: wallet is not implemented in DogeGo"
	}
	if paths.WalletWIF == nil && (paths.WalletWIFForAddress == nil || paths.WalletKnownAddresses == nil) {
		return nil, -1, "dumpwallet: wallet is not implemented in DogeGo"
	}
	if paths.WalletIsEncrypted != nil && paths.WalletIsEncrypted() {
		if paths.WalletIsUnlocked == nil || !paths.WalletIsUnlocked() {
			code, msg := rpcWalletLockedErr(wallet.ErrWalletLocked)
			return nil, code, "dumpwallet: "+msg
		}
	}
	dest, err := ValidateFilePath(dataPathRoots(paths), dest, false)
	if err != nil {
		return nil, -8, "dumpwallet: "+err.Error()
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil && filepath.Dir(dest) != "." {
		return nil, -1, "dumpwallet: " + err.Error()
	}
	var b strings.Builder
	now := time.Now().UTC()
	fmt.Fprintf(&b, "# Wallet dump created by DogeGo %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "# * Created on %s\n\n", now.Format("2006-01-02 15:04:05"))
	writtenWIF := make(map[string]struct{})
	writeKey := func(addr, wif string) {
		wif = strings.TrimSpace(wif)
		addr = strings.TrimSpace(addr)
		if wif == "" {
			return
		}
		if _, ok := writtenWIF[wif]; ok {
			return
		}
		writtenWIF[wif] = struct{}{}
		if addr == "" {
			fmt.Fprintf(&b, "%d,%s label= # cosigner extra_privkeys_hex\n", now.Unix(), wif)
			return
		}
		fmt.Fprintf(&b, "%d,%s label= # addr=%s\n", now.Unix(), wif, addr)
	}
	if paths.WalletWIFForAddress != nil && paths.WalletKnownAddresses != nil {
		for _, addr := range paths.WalletKnownAddresses() {
			wif, err := paths.WalletWIFForAddress(addr)
			if err != nil {
				continue
			}
			writeKey(addr, wif)
		}
	} else {
		wif := strings.TrimSpace(paths.WalletWIF())
		addr := rpcWalletAddress(paths)
		if wif == "" || addr == "" {
			return nil, -1, "dumpwallet: wallet is not implemented in DogeGo"
		}
		writeKey(addr, wif)
	}
	for _, extra := range rpcWalletWIFs(paths) {
		writeKey("", extra)
	}
	net, _ := networkFromRPCChainName(chainName)
	p, _ := chain.ParamsFor(net)
	writtenDesc := make(map[string]struct{})
	for _, pk := range rpcWalletWatchScripts(paths) {
		addr := ""
		if p.PubkeyHashAddrID != 0 || p.ScriptHashAddrID != 0 {
			addr = chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		}
		if addr != "" {
			if desc := walletDescriptorForAddress(chainName, paths, addr); desc != "" {
				if _, ok := writtenDesc[desc]; !ok {
					writtenDesc[desc] = struct{}{}
					fmt.Fprintf(&b, "descriptor=1 %s label= # addr=%s\n", desc, addr)
					continue
				}
			}
		}
		if paths.WalletWatchRedeemScript != nil {
			if redeem := paths.WalletWatchRedeemScript(pk); len(redeem) > 0 {
				fmt.Fprintf(&b, "redeem=1 %s %s label= # watchonly p2sh redeem\n",
					hex.EncodeToString(pk), hex.EncodeToString(redeem))
				continue
			}
		}
		fmt.Fprintf(&b, "script=1 %s label= # watchonly hex=%s\n", hex.EncodeToString(pk), hex.EncodeToString(pk))
	}
	if err := os.WriteFile(dest, []byte(b.String()), 0o600); err != nil {
		return nil, -1, "dumpwallet: " + err.Error()
	}
	return nil, 0, ""
}
