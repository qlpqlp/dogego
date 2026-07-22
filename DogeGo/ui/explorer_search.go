// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
)

func networkFromUISlug(slug string) (chain.Network, error) {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "testnet":
		return chain.RebootTestnet, nil
	case "mainnet":
		return chain.MainnetDogecoin, nil
	default:
		return 0, fmt.Errorf("unknown network %q", slug)
	}
}

func isAllDecimalDigits(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 16 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func is64Hex(s string) bool {
	s = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x"))
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// ExplorerUniversalSearch resolves one query string to block, transaction, or address-shaped JSON
// (local node data only). Block hash is tried before transaction id when both are 64 hex.
func ExplorerUniversalSearch(q, networkSlug string, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, addrIx *store.AddrIndex, pool *mempool.Pool, addrVer byte, rpcInvoke rpcInvoker, utxoFn func() *store.UtxoCache, contiguousHint int64) (map[string]any, int, string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, 400, "missing query"
	}
	net, err := networkFromUISlug(networkSlug)
	if err != nil {
		return nil, 400, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, 400, err.Error()
	}

	out := map[string]any{
		"query":   q,
		"network": strings.ToLower(strings.TrimSpace(networkSlug)),
	}

	// Block height (decimal)
	if isAllDecimalDigits(q) {
		hv, err := strconv.ParseInt(strings.TrimSpace(q), 10, 64)
		if err == nil && hv >= 0 {
			blk, code, msg := LookupBlockForAPI(j, raw, addrVer, strconv.FormatInt(hv, 10), "", contiguousHint)
			if code == 0 {
				hdr, _, _ := LookupHeaderForAPI(j, strconv.FormatInt(hv, 10), "")
				out["kind"] = "block"
				out["block"] = blk
				out["header"] = hdr
				attachBlockTransactions(out, j, raw, txIx, out["network"].(string), hv)
				return out, 0, ""
			}
			if code == 404 {
				return map[string]any{"kind": "none", "query": q, "message": msg}, 404, msg
			}
			return nil, code, msg
		}
	}

	// Dogecoin address (base58check)
	if ver, h160, err := chain.Base58CheckDecode(q); err == nil {
		if ver != p.PubkeyHashAddrID && ver != p.ScriptHashAddrID {
			return map[string]any{"kind": "none", "query": q, "message": "address version does not match this network"}, 404, "wrong address network"
		}
		out["kind"] = "address"
		switch ver {
		case p.PubkeyHashAddrID:
			val, code, msg := rpc.ValidateAddressString(networkSlug, q)
			if code != 0 {
				return nil, code, msg
			}
			vm := make(map[string]any, len(val))
			for k, v := range val {
				vm[k] = v
			}
			out["validate"] = vm
			if addrIx != nil && addrIx.HasAny() {
				if scan, err := ScanAddressFromIndex(addrIx, q, 0, 20, 0, 20); err == nil {
					out["local_window"] = scan
				} else if j != nil && raw != nil {
					if scan, err := ScanAddressInRawWindow(j, raw, txIx, p.PubkeyHashAddrID, p.ScriptHashAddrID, q, pool, "", -1, utxoFn); err == nil {
						out["local_window"] = scan
					} else {
						out["local_window_error"] = err.Error()
					}
				}
			} else if j != nil && raw != nil {
				if scan, err := ScanAddressInRawWindow(j, raw, txIx, p.PubkeyHashAddrID, p.ScriptHashAddrID, q, pool, "", -1, utxoFn); err == nil {
					out["local_window"] = scan
				} else {
					out["local_window_error"] = err.Error()
				}
			}
			attachAddressUTXOBalance(out, rpcInvoke, utxoFn, p.PubkeyHashAddrID, p.ScriptHashAddrID, q)
		case p.ScriptHashAddrID:
			scriptHex := "a914" + hex.EncodeToString(h160[:]) + "87"
			out["validate"] = map[string]any{
				"isvalid":     true,
				"address":     q,
				"ismine":      false,
				"iswatchonly": false,
				"scriptPubKey": map[string]any{
					"hex": scriptHex, "type": "scripthash", "address": q,
				},
				"isscript": true,
			}
			if addrIx != nil && addrIx.HasAny() {
				if scan, err := ScanAddressFromIndex(addrIx, q, 0, 20, 0, 20); err == nil {
					out["local_window"] = scan
				} else if j != nil && raw != nil {
					if scan, err := ScanAddressInRawWindow(j, raw, txIx, p.PubkeyHashAddrID, p.ScriptHashAddrID, q, pool, "", -1, utxoFn); err == nil {
						out["local_window"] = scan
					} else {
						out["local_window_error"] = err.Error()
					}
				}
			} else if j != nil && raw != nil {
				if scan, err := ScanAddressInRawWindow(j, raw, txIx, p.PubkeyHashAddrID, p.ScriptHashAddrID, q, pool, "", -1, utxoFn); err == nil {
					out["local_window"] = scan
				} else {
					out["local_window_error"] = err.Error()
				}
			}
			attachAddressUTXOBalance(out, rpcInvoke, utxoFn, p.PubkeyHashAddrID, p.ScriptHashAddrID, q)
		}
		return out, 0, ""
	}

	// 64 hex: block hash first, then txid
	if is64Hex(q) {
		hexQ := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(q), "0x"))
		hdr, hCode, hMsg := LookupHeaderForAPI(j, "", hexQ)
		if hCode == 0 {
			blk, bCode, bMsg := LookupBlockForAPI(j, raw, addrVer, "", hexQ, contiguousHint)
			if bCode != 0 {
				return nil, bCode, bMsg
			}
			out["kind"] = "block"
			out["header"] = hdr
			out["block"] = blk
			if h, ok := blk["height"].(int64); ok {
				attachBlockTransactions(out, j, raw, txIx, out["network"].(string), h)
			} else if hf, ok := blk["height"].(float64); ok {
				attachBlockTransactions(out, j, raw, txIx, out["network"].(string), int64(hf))
			}
			return out, 0, ""
		}
		canChain := txIx != nil && raw != nil
		canPool := pool != nil
		if canChain || canPool {
			jm, rawTx, src, err := rpc.LookupTxExplorer(txIx, raw, pool, hexQ)
			if err == nil {
				out["kind"] = "transaction"
				out["tx"] = jm
				out["decoded"] = jm
				out["source"] = src
				if len(rawTx) > 0 {
					out["hex"] = hex.EncodeToString(rawTx)
				}
				return out, 0, ""
			}
		}
		msg := "no block or transaction matched this 64-character hex"
		if hCode == 404 {
			msg = "not a known block hash or indexed/mempool txid: " + hMsg
		}
		return map[string]any{"kind": "none", "query": q, "message": msg}, 404, msg
	}

	return map[string]any{"kind": "none", "query": q, "message": "unrecognized query (use block height, 64-hex block hash / txid, or a Dogecoin address)"}, 404, "no match"
}
