// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "strings"

func walletTxRowsForTxid(rows []walletTxRow, txid string, paths *DataPaths, includeWatchonly bool) []walletTxRow {
	txid = strings.ToLower(strings.TrimSpace(txid))
	var out []walletTxRow
	for _, r := range rows {
		if !strings.EqualFold(r.txid, txid) {
			continue
		}
		if !walletRowMatchesFilter(paths, r, "", includeWatchonly) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func walletTxDetailsFromRows(paths *DataPaths, rows []walletTxRow) []interface{} {
	details := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		addr := walletRowAddress(paths, r)
		isWatch := addr != "" && paths != nil && paths.WalletIsWatchAddress != nil && paths.WalletIsWatchAddress(addr)
		d := map[string]interface{}{
			"address":      addr,
			"category":     r.category,
			"amount":       float64(r.amountKoinu) / 1e8,
			"iswatchonly":  isWatch,
		}
		if r.abandoned {
			d["abandoned"] = true
		}
		if r.category == "receive" {
			d["vout"] = r.vout
		}
		details = append(details, d)
	}
	return details
}

func walletTxInvolvesWatchonly(paths *DataPaths, rows []walletTxRow) bool {
	for _, r := range rows {
		addr := walletRowAddress(paths, r)
		if addr != "" && paths != nil && paths.WalletIsWatchAddress != nil && paths.WalletIsWatchAddress(addr) {
			return true
		}
	}
	return false
}

func walletTxEntryAmountKoinu(rows []walletTxRow) int64 {
	var sum int64
	for _, r := range rows {
		sum += r.amountKoinu
	}
	return sum
}
