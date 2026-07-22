// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"strings"

	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execKeypoolRefillWallet is a no-op success for the single-key built-in wallet (Core compatibility).
func execKeypoolRefillWallet(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "keypoolrefill: wallet is not implemented in DogeGo"
	}
	newSize := 0
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "keypoolrefill: newsize must be a number"
		}
		ni, err := n.Int64()
		if err != nil || ni < 0 {
			return nil, -8, "keypoolrefill: invalid newsize"
		}
		newSize = int(ni)
	}
	if paths != nil && paths.WalletKeypoolRefill != nil {
		if err := paths.WalletKeypoolRefill(newSize); err != nil {
			if code, msg := rpcWalletOpErr(err); code != 0 {
				if code == -13 {
					return nil, code, msg
				}
				return nil, code, "keypoolrefill: "+msg
			}
			return nil, -1, "keypoolrefill: "+err.Error()
		}
	}
	return true, 0, ""
}

func walletUniqueTxCount(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool) int {
	// Fast path for getwalletinfo txcount: avoid walletCollectTransactions (header lookups per row).
	if rpcWalletAddress(paths) == "" && len(rpcWalletWatchScripts(paths)) == 0 {
		return 0
	}
	seen := make(map[string]struct{})
	if paths != nil && paths.WalletListScannedTx != nil {
		for _, st := range paths.WalletListScannedTx() {
			if st.TxID != "" {
				seen[strings.ToLower(st.TxID)] = struct{}{}
			}
		}
	}
	if pool != nil && paths != nil && paths.Utxo != nil {
		if entries, err := pool.SortedTransactions(); err == nil {
			spendScripts := rpcWalletSpendScripts(paths)
			for _, ent := range entries {
				tx, err := wire.DeserializeTx(ent.Raw)
				if err != nil {
					continue
				}
				touch := false
				for _, in := range tx.Vin {
					if _, ok := paths.Utxo.LookupOutpoint(in.PrevHash, in.PrevIdx); ok {
						touch = true
						break
					}
				}
				if !touch {
					for _, o := range tx.Vout {
						for _, pk := range spendScripts {
							if bytes.Equal(o.PkScript, pk) {
								touch = true
								break
							}
						}
						if touch {
							break
						}
					}
				}
				if touch {
					seen[strings.ToLower(mempool.TxIDDisplayHex(tx.TxHash()))] = struct{}{}
				}
			}
		}
	}
	return len(seen)
}
