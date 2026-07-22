// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
	"dogego/store"
)

func rpcWalletWatchScripts(paths *DataPaths) [][]byte {
	if paths == nil || paths.WalletWatchScripts == nil {
		return nil
	}
	return paths.WalletWatchScripts()
}

func rpcWalletTrackedScripts(paths *DataPaths) [][]byte {
	var out [][]byte
	for _, pk := range rpcWalletSpendScripts(paths) {
		out = append(out, pk)
	}
	for _, w := range rpcWalletWatchScripts(paths) {
		out = append(out, w)
	}
	return out
}

func rpcWalletIsWatchAddress(paths *DataPaths, addr string) bool {
	if paths == nil || paths.WalletIsWatchAddress == nil {
		return false
	}
	return paths.WalletIsWatchAddress(strings.TrimSpace(addr))
}

func rpcWalletIsSpendableScript(paths *DataPaths, pkScript []byte) bool {
	return walletScriptInSet(walletRawScriptSet(rpcWalletSpendScripts(paths)), pkScript)
}

func walletRawScriptSet(scripts [][]byte) map[string]struct{} {
	set := make(map[string]struct{}, len(scripts))
	for _, pk := range scripts {
		if len(pk) > 0 {
			set[string(pk)] = struct{}{}
		}
	}
	return set
}

func walletScriptInSet(set map[string]struct{}, pkScript []byte) bool {
	if len(pkScript) == 0 || len(set) == 0 {
		return false
	}
	_, ok := set[string(pkScript)]
	return ok
}

type walletUtxoMatch struct {
	row           store.UtxoDumpRow
	address       string
	spendable     bool
	confirmations int64
}

func walletUtxoMatches(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, chainName string, minConf int64, maxResults int) ([]walletUtxoMatch, int, string) {
	scripts := rpcWalletTrackedScripts(paths)
	if len(scripts) == 0 {
		return nil, 0, ""
	}
	if paths == nil || paths.Utxo == nil {
		return nil, 0, ""
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	rows, err := walletConfirmedUTXORows(paths, scripts, maxResults)
	if err != nil {
		return nil, -1, "listunspent: " + err.Error()
	}
	spendSet := walletRawScriptSet(rpcWalletSpendScripts(paths))
	chainTip := int64(-1)
	if paths != nil && paths.Utxo != nil {
		chainTip = paths.Utxo.TipHeight()
	}
	if chainTip < 0 {
		chainTip, _, _ = activeChainFromJournal(j, raw, paths)
	}
	var out []walletUtxoMatch
	for _, row := range rows {
		var addr string
		var matched bool
		for _, pk := range scripts {
			if !bytes.Equal(row.PkScript, pk) {
				continue
			}
			addr = chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			matched = true
			break
		}
		if !matched || addr == "" {
			continue
		}
		conf := confirmationsFromTip(chainTip, row.Height)
		if conf < minConf {
			continue
		}
		spendable := walletScriptInSet(spendSet, row.PkScript) && !rpcWalletIsLockedOutpoint(paths, row.TxID, row.Vout)
		out = append(out, walletUtxoMatch{
			row:           row,
			address:       addr,
			spendable:     spendable,
			confirmations: conf,
		})
	}
	return out, 0, ""
}

// walletScanHasReceiveRows reports whether wallet.db scan history includes receive rows.
func walletScanHasReceiveRows(paths *DataPaths) bool {
	if paths == nil || paths.WalletListScannedTx == nil {
		return false
	}
	for _, r := range paths.WalletListScannedTx() {
		if r.Category == "receive" {
			return true
		}
	}
	return false
}

// importWatchScriptArg resolves importaddress argument to scriptPubKey bytes.
func importWatchScriptArg(chainName, arg string, wantP2SH bool) ([]byte, int, string) {
	arg = strings.TrimSpace(arg)
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -1, "importaddress: unknown chain"
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -1, "importaddress: unknown chain"
	}
	if v, h160, err := chain.Base58CheckDecode(arg); err == nil {
		if wantP2SH {
			return nil, -5, "Cannot use the p2sh flag with an address - use a script instead"
		}
		switch v {
		case p.PubkeyHashAddrID:
			return chain.P2PKHScriptFromPubKeyHash(h160), 0, ""
		case p.ScriptHashAddrID:
			return chain.P2SHScriptFromScriptHash(h160), 0, ""
		}
	}
	if isHexScriptArg(arg) {
		pk, err := hex.DecodeString(arg)
		if err != nil || len(pk) == 0 {
			return nil, -5, "Invalid Dogecoin address or script"
		}
		if wantP2SH {
			h := chain.Hash160(pk)
			if len(h) != 20 {
				return nil, -5, "Invalid Dogecoin address or script"
			}
			var h160 [20]byte
			copy(h160[:], h)
			return chain.P2SHScriptFromScriptHash(h160), 0, ""
		}
		return pk, 0, ""
	}
	return nil, -5, "Invalid Dogecoin address or script"
}

// execImportAddress imports a watch-only P2PKH address or hex script into the built-in wallet.
func execImportAddress(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 5 {
		return nil, -32602, "Wrong number of arguments"
	}
	var arg string
	if err := json.Unmarshal(params[0], &arg); err != nil {
		return nil, -8, "importaddress: script or address must be a string"
	}
	wantP2SH := false
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var code int
		var msg string
		wantP2SH, code, msg = parseRPCBoolOpt(params[3], false, "importaddress", "p2sh")
		if code != 0 {
			return nil, code, msg
		}
	}
	pkScript, code, msg := importWatchScriptArg(chainName, arg, wantP2SH)
	if code != 0 {
		return nil, code, msg
	}
	var importLabel string
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &importLabel); err != nil {
			return nil, -8, "importaddress: label must be a string"
		}
		importLabel = strings.TrimSpace(importLabel)
	}
	rescan := true
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var c int
		var m string
		rescan, c, m = parseRPCBoolOpt(params[2], true, "importaddress", "rescan")
		if c != 0 {
			return nil, c, m
		}
	}
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		if !rescan {
			// Core ignores height when rescan is false.
		} else {
			var n json.Number
			if err := json.Unmarshal(params[4], &n); err != nil {
				return nil, -8, "importaddress: height must be a number"
			}
			hi, err := n.Int64()
			if err != nil || hi < 0 {
				return nil, -8, "importaddress: height out of range"
			}
			if j != nil {
				chainTip, _, _ := activeChainFromJournal(j, raw, paths)
				if hi > chainTip {
					return nil, -8, "Block height out of range"
				}
			}
		}
	}
	if paths == nil || paths.WalletImportWatch == nil {
		return nil, -1, "importaddress: wallet is not implemented in DogeGo"
	}
	if err := paths.WalletImportWatch(pkScript); err != nil {
		return nil, -1, "importaddress: " + err.Error()
	}
	walletApplyLabel(chainName, paths, pkScript, importLabel)
	if rescan {
		if code, msg := walletRescanAfterImport(paths, j, raw, params, 4, "importaddress"); code != 0 {
			return nil, code, msg
		}
	}
	return nil, 0, ""
}

func rpcWalletTrackedAddresses(paths *DataPaths, chainName string) ([]string, int, string) {
	if paths != nil && paths.WalletKnownAddresses != nil {
		if addrs := paths.WalletKnownAddresses(); len(addrs) > 0 {
			return slices.Clone(addrs), 0, ""
		}
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	seen := make(map[string]struct{})
	var out []string
	if a := rpcWalletAddress(paths); a != "" {
		seen[a] = struct{}{}
		out = append(out, a)
	}
	for _, pk := range rpcWalletWatchScripts(paths) {
		a := chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out, 0, ""
}

func walletAddressIsTracked(paths *DataPaths, chainName, addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	if rpcWalletContainsAddress(paths, addr) {
		return true
	}
	return rpcWalletIsWatchAddress(paths, addr)
}

// execListReceivedByAddressWallet groups UTXO cache receives by tracked address.
func execListReceivedByAddressWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	minConf, includeEmpty, includeWatchonly, code, msg := parseListReceivedParams(params, "listreceivedbyaddress")
	if code != 0 {
		return nil, code, msg
	}
	byAddr, code, msg := walletReceivedByAddress(chainName, paths, j, raw, minConf, includeEmpty, includeWatchonly)
	if code != 0 {
		return nil, code, msg
	}
	if len(byAddr) == 0 {
		return []interface{}{}, 0, ""
	}
	addrs := make([]string, 0, len(byAddr))
	for addr := range byAddr {
		addrs = append(addrs, addr)
	}
	slices.Sort(addrs)
	out := make([]interface{}, 0, len(addrs))
	for _, addr := range addrs {
		a := byAddr[addr]
		entry := map[string]interface{}{
			"address":       addr,
			"account":       "",
			"amount":        float64(a.amount) / 1e8,
			"confirmations": a.minConf,
			"label":         rpcWalletGetLabel(paths, addr),
			"txids":         walletReceivedAggTxids(a),
		}
		isWatch := false
		if paths != nil && paths.WalletIsWatchAddress != nil {
			isWatch = paths.WalletIsWatchAddress(addr)
		}
		entry["iswatchonly"] = isWatch
		out = append(out, entry)
	}
	return out, 0, ""
}

// pubkeyHexToP2PKHScript builds standard P2PKH scriptPubKey from a hex-encoded public key.
func pubkeyHexToP2PKHScript(pubHex string) ([]byte, int, string) {
	pubHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(pubHex), "0x"))
	if !isHexScriptArg(pubHex) {
		return nil, -5, "Pubkey must be a hex string"
	}
	raw, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, -5, "Pubkey must be a hex string"
	}
	pub, err := secp256k1.ParsePubKey(raw)
	if err != nil {
		return nil, -5, "Pubkey is not a valid public key"
	}
	comp := pub.SerializeCompressed()
	h := sha256.Sum256(comp)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	h160 := r.Sum(nil)
	pk := append([]byte{0x76, 0xa9, 0x14}, h160...)
	return append(pk, 0x88, 0xac), 0, ""
}

// execImportPubKey imports a watch-only pubkey as P2PKH (importpubkey).
func execImportPubKey(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 4 {
		return nil, -32602, "Wrong number of arguments"
	}
	var pubHex string
	if err := json.Unmarshal(params[0], &pubHex); err != nil {
		return nil, -8, "importpubkey: pubkey must be a string"
	}
	pkScript, code, msg := pubkeyHexToP2PKHScript(pubHex)
	if code != 0 {
		return nil, code, msg
	}
	var importLabel string
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &importLabel); err != nil {
			return nil, -8, "importpubkey: label must be a string"
		}
		importLabel = strings.TrimSpace(importLabel)
	}
	rescan := true
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var c int
		var m string
		rescan, c, m = parseRPCBoolOpt(params[2], true, "importpubkey", "rescan")
		if c != 0 {
			return nil, c, m
		}
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		if !rescan {
			// Core ignores height when rescan is false.
		} else {
			var n json.Number
			if err := json.Unmarshal(params[3], &n); err != nil {
				return nil, -8, "importpubkey: height must be a number"
			}
			hi, err := n.Int64()
			if err != nil || hi < 0 {
				return nil, -8, "importpubkey: height out of range"
			}
			if j != nil {
				chainTip, _, _ := activeChainFromJournal(j, raw, paths)
				if hi > chainTip {
					return nil, -8, "Block height out of range"
				}
			}
		}
	}
	if paths == nil || paths.WalletImportWatch == nil {
		return nil, -1, "importpubkey: wallet is not implemented in DogeGo"
	}
	if err := paths.WalletImportWatch(pkScript); err != nil {
		return nil, -1, "importpubkey: " + err.Error()
	}
	walletApplyLabel(chainName, paths, pkScript, importLabel)
	if rescan {
		if code, msg := walletRescanAfterImport(paths, j, raw, params, 3, "importpubkey"); code != 0 {
			return nil, code, msg
		}
	}
	return nil, 0, ""
}
