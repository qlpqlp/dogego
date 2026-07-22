// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"dogego/chain"
	"dogego/store"
)

func labelFromDumpWalletLine(line string) string {
	idx := strings.Index(line, "label=")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+6:])
	if hash := strings.IndexByte(rest, '#'); hash >= 0 {
		rest = strings.TrimSpace(rest[:hash])
	}
	return strings.TrimSpace(rest)
}

// parseDumpWalletLine extracts a WIF, watch script hex, redeem= pair, or descriptor= line from a dump.
func parseDumpWalletLine(line string) (wif, scriptHex, redeemHex, desc string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", "", "", false
	}
	if strings.HasPrefix(line, "descriptor=") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "descriptor="))
		if !strings.HasPrefix(rest, "1 ") {
			return "", "", "", "", false
		}
		rest = strings.TrimSpace(rest[2:])
		if idx := strings.Index(rest, " label="); idx >= 0 {
			rest = rest[:idx]
		}
		if hash := strings.Index(rest, " #"); hash >= 0 {
			rest = rest[:hash]
		}
		desc = strings.TrimSpace(rest)
		if desc == "" {
			return "", "", "", "", false
		}
		return "", "", "", desc, true
	}
	if strings.HasPrefix(line, "redeem=") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "redeem="))
		fields := strings.Fields(rest)
		if len(fields) >= 3 && fields[0] == "1" && isHexScriptArg(fields[1]) && isHexScriptArg(fields[2]) {
			return "", fields[1], fields[2], "", true
		}
		return "", "", "", "", false
	}
	if strings.HasPrefix(line, "script=") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "script="))
		fields := strings.Fields(rest)
		for _, f := range fields {
			f = strings.TrimSpace(f)
			if len(f) >= 2 && isHexScriptArg(f) {
				return "", f, "", "", true
			}
		}
		return "", "", "", "", false
	}
	comma := strings.IndexByte(line, ',')
	if comma <= 0 {
		return "", "", "", "", false
	}
	rest := strings.TrimSpace(line[comma+1:])
	if sp := strings.IndexByte(rest, ' '); sp > 0 {
		rest = rest[:sp]
	}
	rest = strings.TrimSpace(strings.TrimSuffix(rest, ","))
	if rest == "" {
		return "", "", "", "", false
	}
	return rest, "", "", "", true
}

// execImportWallet loads keys from a dumpwallet-style text file into the built-in wallet.
func execImportWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var filename string
	if err := json.Unmarshal(params[0], &filename); err != nil {
		return nil, -8, "importwallet: filename must be a string"
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, -8, "importwallet: invalid filename"
	}
	if paths == nil || paths.WalletImportSpendKey == nil || paths.WalletImportWatch == nil {
		return nil, -1, "importwallet: wallet is not implemented in DogeGo"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	importWatch := func(spk, redeem []byte) error {
		if paths.WalletSetWatchRedeem != nil && len(redeem) > 0 {
			return rpcWalletImportWatchScript(paths, spk, redeem)
		}
		return paths.WalletImportWatch(spk)
	}
	if rpcWalletAddress(paths) == "" {
		return nil, -1, "importwallet: wallet is not implemented in DogeGo"
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, -8, "Cannot open wallet dump file"
	}
	defer f.Close()
	var importedKey bool
	var importedWatch bool
	var unlockChecked bool
	var spendKeyImported bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		wif, scriptHex, redeemHex, desc, ok := parseDumpWalletLine(line)
		if !ok {
			continue
		}
		if desc != "" {
			lbl := labelFromDumpWalletLine(line)
			elem, err := json.Marshal(map[string]interface{}{"desc": desc, "label": lbl})
			if err != nil {
				continue
			}
			if _, imported := importDescriptorOne(chainName, paths, p, elem); imported {
				importedWatch = true
			}
			continue
		}
		if scriptHex != "" {
			pk, err := hex.DecodeString(scriptHex)
			if err != nil || len(pk) == 0 {
				continue
			}
			var redeem []byte
			if redeemHex != "" {
				redeem, err = hex.DecodeString(redeemHex)
				if err != nil || len(redeem) == 0 {
					continue
				}
			}
			if err := importWatch(pk, redeem); err != nil {
				return nil, -1, "importwallet: " + err.Error()
			}
			importedWatch = true
			if lbl := labelFromDumpWalletLine(line); lbl != "" && paths.WalletSetLabel != nil {
				if addr := chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID); addr != "" {
					_ = paths.WalletSetLabel(addr, lbl)
				}
			}
			continue
		}
		if wif != "" {
			if !unlockChecked {
				if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
					return nil, code, msg
				}
				unlockChecked = true
			}
			if !spendKeyImported {
				var err error
				if paths.WalletHDFormat != nil && paths.WalletHDFormat() == "hd" && paths.WalletImportPrivKey != nil {
					err = paths.WalletImportPrivKey(wif)
				} else {
					err = paths.WalletImportSpendKey(wif)
				}
				if err != nil {
					if code, msg := rpcWalletOpErr(err); code != 0 {
						return nil, code, msg
					}
					return nil, -5, "Error adding key to wallet"
				}
				spendKeyImported = true
			} else if paths.WalletImportPrivKey != nil {
				if err := paths.WalletImportPrivKey(wif); err != nil {
					if code, msg := rpcWalletOpErr(err); code != 0 {
						return nil, code, msg
					}
					return nil, -5, "Error adding key to wallet"
				}
			} else if err := paths.WalletImportSpendKey(wif); err != nil {
				if code, msg := rpcWalletOpErr(err); code != 0 {
					return nil, code, msg
				}
				return nil, -5, "Error adding key to wallet"
			}
			importedKey = true
			if lbl := labelFromDumpWalletLine(line); lbl != "" && paths.WalletSetLabel != nil {
				if addr, err := addressFromWIF(chainName, wif); err == nil && addr != "" {
					_ = paths.WalletSetLabel(addr, lbl)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, -1, "importwallet: " + err.Error()
	}
	if !importedKey && !importedWatch {
		if len(rpcWalletWatchScripts(paths)) == 0 {
			return nil, -8, "No valid keys or scripts found in dump file"
		}
	}
	if code, msg := walletRescanAfterImport(paths, j, raw, nil, -1, "importwallet"); code != 0 {
		return nil, code, msg
	}
	if importedKey {
		walletMaybeRefillKeypool(paths)
	}
	return nil, 0, ""
}
